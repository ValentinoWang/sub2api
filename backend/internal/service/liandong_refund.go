package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	ErrLiandongRefundInvalid     = errors.New("invalid Liandong refund request")
	ErrLiandongRefundConflict    = errors.New("Liandong refund identity conflicts with an existing reservation")
	ErrLiandongRefundUnavailable = errors.New("Liandong refund storage is unavailable")
	ErrLiandongRefundNotEligible = errors.New("Liandong code is not eligible for an unused-rights refund")
	ErrLiandongRefundNotFound    = errors.New("Liandong refund reservation not found")
)

type LiandongRefundPrepareRequest struct {
	ExternalOrderNo string `json:"external_order_no"`
	BatchID         string `json:"batch_id"`
	Code            string `json:"code"`
}

type LiandongRefundConfirmRequest struct {
	ExternalOrderNo         string `json:"external_order_no"`
	MerchantRefundReference string `json:"merchant_refund_reference"`
}

// LiandongCodeRefund records local rights and an operator's external reference.
// Neither status asserts that the merchant actually returned money.
type LiandongCodeRefund struct {
	ExternalOrderNo         string     `json:"external_order_no"`
	BatchID                 string     `json:"batch_id"`
	Status                  string     `json:"status"`
	MerchantRefundReference *string    `json:"merchant_refund_reference,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	ConfirmedAt             *time.Time `json:"confirmed_at,omitempty"`
}

func validLiandongRefundReference(value string, limit int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= limit && !strings.ContainsAny(value, "\r\n\x00")
}

const liandongRefundSelect = `SELECT external_order_no, batch_id, status, merchant_refund_reference, created_at, confirmed_at, code_sha256 FROM liandong_code_refunds WHERE external_order_no = $1`

func scanLiandongRefund(row *sql.Row) (*LiandongCodeRefund, string, error) {
	var result LiandongCodeRefund
	var digest string
	err := row.Scan(&result.ExternalOrderNo, &result.BatchID, &result.Status, &result.MerchantRefundReference, &result.CreatedAt, &result.ConfirmedAt, &digest)
	return &result, digest, err
}

// PrepareUnusedCodeRefund freezes the entitlement atomically with its reservation.
// External order ownership must be checked by the operator in the merchant system.
func (s *LiandongRestockService) PrepareUnusedCodeRefund(ctx context.Context, req LiandongRefundPrepareRequest) (*LiandongCodeRefund, error) {
	if !validLiandongRefundReference(req.ExternalOrderNo, 128) || !validLiandongRefundReference(req.BatchID, 64) || !validLiandongRefundReference(req.Code, 32) {
		return nil, ErrLiandongRefundInvalid
	}
	if s == nil || s.db == nil {
		return nil, ErrLiandongRefundUnavailable
	}
	digestBytes := sha256.Sum256([]byte(req.Code))
	digest := hex.EncodeToString(digestBytes[:])
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	// Serializes identical orders even if competing requests name different codes.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 239))`, req.ExternalOrderNo); err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	existing, existingDigest, err := scanLiandongRefund(tx.QueryRowContext(ctx, liandongRefundSelect, req.ExternalOrderNo))
	if err == nil {
		if existing.BatchID != req.BatchID || existingDigest != digest {
			return nil, ErrLiandongRefundConflict
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLiandongRefundUnavailable
	}
	var id int64
	var status string
	var usedBy sql.NullInt64
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT r.id, r.status, r.used_by, r.used_at
		FROM redeem_codes r JOIN liandong_restock_batch_codes b ON b.code_sha256 = $2 AND b.batch_id = $3
		WHERE r.code = $1 FOR UPDATE OF r`, req.Code, digest, req.BatchID).Scan(&id, &status, &usedBy, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLiandongRefundNotEligible
	}
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	var reserved bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM liandong_code_refunds WHERE redeem_code_id = $1)`, id).Scan(&reserved); err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	if reserved {
		return nil, ErrLiandongRefundConflict
	}
	if status != StatusUnused || usedBy.Valid || usedAt.Valid {
		return nil, ErrLiandongRefundNotEligible
	}
	result, err := tx.ExecContext(ctx, `UPDATE redeem_codes SET status = 'disabled' WHERE id = $1 AND status = 'unused' AND used_by IS NULL AND used_at IS NULL`, id)
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return nil, ErrLiandongRefundNotEligible
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO liandong_code_refunds (external_order_no, batch_id, code_sha256, redeem_code_id) VALUES ($1,$2,$3,$4)`, req.ExternalOrderNo, req.BatchID, digest, id)
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	refund, _, err := scanLiandongRefund(tx.QueryRowContext(ctx, liandongRefundSelect, req.ExternalOrderNo))
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	if err = tx.Commit(); err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	return refund, nil
}

func (s *LiandongRestockService) ConfirmUnusedCodeRefund(ctx context.Context, req LiandongRefundConfirmRequest) (*LiandongCodeRefund, error) {
	if !validLiandongRefundReference(req.ExternalOrderNo, 128) || !validLiandongRefundReference(req.MerchantRefundReference, 128) {
		return nil, ErrLiandongRefundInvalid
	}
	if s == nil || s.db == nil {
		return nil, ErrLiandongRefundUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	refund, _, err := scanLiandongRefund(tx.QueryRowContext(ctx, liandongRefundSelect+` FOR UPDATE`, req.ExternalOrderNo))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLiandongRefundNotFound
	}
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	if refund.MerchantRefundReference != nil {
		if *refund.MerchantRefundReference != req.MerchantRefundReference {
			return nil, ErrLiandongRefundConflict
		}
		return refund, nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE liandong_code_refunds SET status = 'merchant_reference_recorded', merchant_refund_reference = $2, confirmed_at = NOW() WHERE external_order_no = $1`, req.ExternalOrderNo, req.MerchantRefundReference)
	if err != nil {
		var state interface{ SQLState() string }
		if errors.As(err, &state) && state.SQLState() == "23505" {
			return nil, ErrLiandongRefundConflict
		}
		return nil, ErrLiandongRefundUnavailable
	}
	refund, _, err = scanLiandongRefund(tx.QueryRowContext(ctx, liandongRefundSelect, req.ExternalOrderNo))
	if err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	if err = tx.Commit(); err != nil {
		return nil, ErrLiandongRefundUnavailable
	}
	return refund, nil
}
