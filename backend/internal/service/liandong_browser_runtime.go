package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type LiandongBrowserRuntimeReport struct {
	State                       string     `json:"state"`
	Reason                      string     `json:"reason"`
	BrowserVerificationRequired bool       `json:"browser_verification_required"`
	ExecutionMode               string     `json:"execution_mode"`
	CheckedAt                   *time.Time `json:"checked_at,omitempty"`
	NextCheckAt                 *time.Time `json:"next_check_at,omitempty"`
}

type LiandongBrowserRuntime struct {
	LiandongBrowserRuntimeReport
	ReportedAt time.Time `json:"reported_at"`
}

func validateBrowserRuntimeReport(report LiandongBrowserRuntimeReport) error {
	if report.ExecutionMode != "" && report.ExecutionMode != "http" && report.ExecutionMode != "browser" {
		return errBrowserInvalid
	}
	if report.State == "running" {
		if report.Reason != "" || report.BrowserVerificationRequired {
			return errBrowserInvalid
		}
		return nil
	}
	if report.State != "paused" {
		return errBrowserInvalid
	}
	switch report.Reason {
	case "binding_changed", "inventory_mismatch", "uncertain", "backend_error", "backend_auth",
		"busy", "manual", "setup_required", "disabled", "invalid_config", "state_invalid",
		"recovery_wait", "recovery_exhausted", "dedup_unverified", "backend_upgrade_required",
		"login_required", "login_rejected", "verification_required", "upstream_rejected",
		"upstream_unavailable", "network_error", "non_json", "invalid_response", "unsafe_session",
		"local_unavailable", "invalid_input":
	default:
		return errBrowserInvalid
	}
	if report.BrowserVerificationRequired && report.Reason != "non_json" && report.Reason != "verification_required" {
		return errBrowserInvalid
	}
	return nil
}

// BrowserReportRuntime records an observation only; it never renews authorization or resumes work.
func (s *LiandongRestockService) BrowserReportRuntime(ctx context.Context, id string, report LiandongBrowserRuntimeReport) (*LiandongBrowserRuntime, error) {
	if report.ExecutionMode == "" {
		report.ExecutionMode = "http"
	}
	if err := validateBrowserRuntimeReport(report); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_BROWSER_UNAVAILABLE", "持久存储不可用")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize against device revocation without taking the replenishment advisory lock.
	var deviceID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM liandong_browser_devices WHERE id=$1 AND NOT revoked FOR SHARE`, id).Scan(&deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errBrowserAuth
	}
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, errBrowserInvalid
	}
	out := &LiandongBrowserRuntime{LiandongBrowserRuntimeReport: report}
	err = tx.QueryRowContext(ctx, `INSERT INTO liandong_browser_runtime(device_id,report,reported_at)
        VALUES($1,$2::jsonb,NOW()) ON CONFLICT(device_id) DO UPDATE SET report=EXCLUDED.report,reported_at=NOW()
        RETURNING reported_at`, deviceID, string(raw)).Scan(&out.ReportedAt)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

type LiandongBrowserRecheck struct {
	ID          string     `json:"id"`
	DeviceID    string     `json:"device_id"`
	RequestedBy int64      `json:"requested_by"`
	State       string     `json:"state"`
	Reason      string     `json:"reason"`
	Resumed     bool       `json:"resumed"`
	RequestedAt time.Time  `json:"requested_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

type LiandongBrowserRecheckResult struct {
	State   string `json:"state"`
	Reason  string `json:"reason"`
	Resumed bool   `json:"resumed"`
}

var errBrowserRecheckConflict = infraerrors.Conflict("LDXP_RECHECK_CONFLICT", "复查请求状态冲突，请刷新后重试")
var errBrowserRecheckNotFound = infraerrors.NotFound("LDXP_RECHECK_NOT_FOUND", "复查请求不存在")

const browserRecheckColumns = `id,device_id,requested_by,state,reason,resumed,requested_at,started_at,finished_at`

func browserScanRecheck(row *sql.Row) (*LiandongBrowserRecheck, error) {
	var out LiandongBrowserRecheck
	err := row.Scan(&out.ID, &out.DeviceID, &out.RequestedBy, &out.State, &out.Reason, &out.Resumed, &out.RequestedAt, &out.StartedAt, &out.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &out, err
}

func (s *LiandongRestockService) browserRecheckTx(ctx context.Context, deviceID string) (*sql.Tx, error) {
	if s == nil || s.db == nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_BROWSER_UNAVAILABLE", "持久存储不可用")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Serialize only this device's observation requests, including concurrent revocation.
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM liandong_browser_devices WHERE id=$1 AND NOT revoked FOR UPDATE`, deviceID).Scan(&id)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errBrowserAuth
		}
		return nil, err
	}
	return tx, nil
}

func browserExpireQueuedRechecks(ctx context.Context, tx *sql.Tx, deviceID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE liandong_browser_rechecks SET state='failed',reason='state_invalid',resumed=FALSE,finished_at=NOW()
        WHERE device_id=$1 AND state='queued' AND requested_at<NOW()-INTERVAL '10 minutes'`, deviceID)
	return err
}

func browserPendingRecheck(ctx context.Context, tx *sql.Tx, deviceID string) (*LiandongBrowserRecheck, error) {
	return browserScanRecheck(tx.QueryRowContext(ctx, `SELECT `+browserRecheckColumns+` FROM liandong_browser_rechecks
        WHERE device_id=$1 AND state IN ('queued','checking') ORDER BY requested_at DESC,id DESC LIMIT 1 FOR UPDATE`, deviceID))
}

// BrowserRequestRecheck records operator intent, never an assertion of successful merchant verification.
func (s *LiandongRestockService) BrowserRequestRecheck(ctx context.Context, deviceID string, requestedBy int64) (*LiandongBrowserRecheck, error) {
	tx, err := s.browserRecheckTx(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var admin bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL)`, requestedBy).Scan(&admin); err != nil {
		return nil, err
	}
	if !admin {
		return nil, infraerrors.Forbidden("LDXP_ADMIN_REQUIRED", "需要有效管理员身份")
	}
	if err = browserExpireQueuedRechecks(ctx, tx, deviceID); err != nil {
		return nil, err
	}
	out, err := browserPendingRecheck(ctx, tx, deviceID)
	if err != nil {
		return nil, err
	}
	if out == nil {
		id, err := browserRandomHex(16)
		if err != nil {
			return nil, err
		}
		out, err = browserScanRecheck(tx.QueryRowContext(ctx, `INSERT INTO liandong_browser_rechecks(id,device_id,requested_by)
            VALUES($1,$2,$3) RETURNING `+browserRecheckColumns, id, deviceID, requestedBy))
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

// BrowserClaimRecheck replays in-progress work so a restarted worker can repeat its read-only checks.
func (s *LiandongRestockService) BrowserClaimRecheck(ctx context.Context, deviceID string) (*LiandongBrowserRecheck, error) {
	tx, err := s.browserRecheckTx(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = browserExpireQueuedRechecks(ctx, tx, deviceID); err != nil {
		return nil, err
	}
	out, err := browserPendingRecheck(ctx, tx, deviceID)
	if err != nil {
		return nil, err
	}
	if out != nil && out.State == "queued" {
		out, err = browserScanRecheck(tx.QueryRowContext(ctx, `UPDATE liandong_browser_rechecks SET state='checking',started_at=NOW()
            WHERE id=$1 AND device_id=$2 AND state='queued' RETURNING `+browserRecheckColumns, out.ID, deviceID))
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

// BrowserFinishRecheck stores the worker's observation without performing any resume or authorization action.
func (s *LiandongRestockService) BrowserFinishRecheck(ctx context.Context, deviceID, requestID string, result LiandongBrowserRecheckResult) (*LiandongBrowserRecheck, error) {
	switch result.State {
	case "passed":
		if result.Reason != "" {
			return nil, errBrowserInvalid
		}
	case "failed":
		if result.Resumed || validateBrowserRuntimeReport(LiandongBrowserRuntimeReport{State: "paused", Reason: result.Reason}) != nil {
			return nil, errBrowserInvalid
		}
	default:
		return nil, errBrowserInvalid
	}
	tx, err := s.browserRecheckTx(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	out, err := browserScanRecheck(tx.QueryRowContext(ctx, `SELECT `+browserRecheckColumns+` FROM liandong_browser_rechecks WHERE id=$1 AND device_id=$2 FOR UPDATE`, requestID, deviceID))
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, errBrowserRecheckNotFound
	}
	if out.State == "passed" || out.State == "failed" {
		if out.State != result.State || out.Reason != result.Reason || out.Resumed != result.Resumed {
			return nil, errBrowserRecheckConflict
		}
	} else {
		if out.State != "checking" {
			return nil, errBrowserRecheckConflict
		}
		out, err = browserScanRecheck(tx.QueryRowContext(ctx, `UPDATE liandong_browser_rechecks SET state=$3,reason=$4,resumed=$5,finished_at=NOW()
            WHERE id=$1 AND device_id=$2 AND state='checking' RETURNING `+browserRecheckColumns, requestID, deviceID, result.State, result.Reason, result.Resumed))
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}
