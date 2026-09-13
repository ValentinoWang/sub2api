package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var errBrowserPaused = infraerrors.Conflict("LDXP_BROWSER_PAUSED", "Chrome 补货已暂停，请验证登录和完整库存后手动恢复")
var errBrowserInvalid = infraerrors.BadRequest("LDXP_BROWSER_INVALID", "Chrome 补货请求或商品配置无效")
var errBrowserAuth = infraerrors.Unauthorized("LDXP_DEVICE_UNAUTHORIZED", "补货设备凭据无效或已撤销")

type LiandongBrowserProduct struct {
	GoodsID          int64      `json:"goods_id"`
	CNYAmount        int        `json:"cny_amount"`
	USDCredit        float64    `json:"usd_credit"`
	ExternalURL      string     `json:"external_url"`
	Title            string     `json:"title"`
	TargetStock      int        `json:"target_stock"`
	BatchSize        int        `json:"batch_size"`
	Enabled          bool       `json:"enabled"`
	CurrentStock     int        `json:"current_stock"`
	IdentityVerified bool       `json:"identity_verified"`
	InventoryAt      *time.Time `json:"inventory_at,omitempty"`
}
type LiandongBrowserConfig struct {
	configured   bool
	Enabled      bool                     `json:"enabled"`
	PausedReason string                   `json:"paused_reason"`
	Products     []LiandongBrowserProduct `json:"products"`
}
type LiandongBrowserDevice struct {
	GoodsIDs                []int64    `json:"goods_ids"`
	ID                      string     `json:"id"`
	Name                    string     `json:"name"`
	Revoked                 bool       `json:"revoked"`
	PausedReason            string     `json:"paused_reason"`
	LastSeenAt              *time.Time `json:"last_seen_at,omitempty"`
	AuthorizationVerifiedAt *time.Time `json:"authorization_verified_at,omitempty"`
}
type LiandongBrowserBatch struct {
	BatchID    string    `json:"batch_id"`
	GoodsID    int64     `json:"goods_id"`
	Status     string    `json:"status"`
	CodeCount  int       `json:"code_count"`
	CreatedAt  time.Time `json:"created_at"`
	Codes      []string  `json:"codes,omitempty"`
	CodeHashes []string  `json:"code_hashes,omitempty"`
}
type LiandongBrowserStatus struct {
	LiandongBrowserConfig
	Devices []LiandongBrowserDevice `json:"devices"`
	Batches []LiandongBrowserBatch  `json:"batches"`
}
type LiandongBrowserInventoryReport struct {
	BatchID  string   `json:"batch_id,omitempty"`
	GoodsID  int64    `json:"goods_id"`
	Complete bool     `json:"complete"`
	Total    int      `json:"total"`
	Hashes   []string `json:"hashes"`
}
type LiandongBrowserInventoryResult struct {
	IdentityVerified bool                  `json:"identity_verified"`
	MatchedStock     int                   `json:"matched_stock"`
	Blocked          bool                  `json:"blocked"`
	PendingBatch     *LiandongBrowserBatch `json:"pending_batch"`
	BatchResolved    bool                  `json:"batch_resolved"`
}
type LiandongRechargeProduct struct {
	GoodsID     int64   `json:"goods_id"`
	CNYAmount   int     `json:"cny_amount"`
	USDCredit   float64 `json:"usd_credit"`
	ExternalURL string  `json:"external_url"`
	Title       string  `json:"title"`
}

func browserRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func browserHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// The same PostgreSQL advisory lock serializes both transports, across processes.
func (s *LiandongRestockService) browserTx(ctx context.Context) (*sql.Tx, error) {
	if s.db == nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_BROWSER_UNAVAILABLE", "持久存储不可用")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	var ok bool
	if err = tx.QueryRowContext(ctx, "SELECT pg_try_advisory_xact_lock($1)", hashAdvisoryLockID(liandongRestockLeaseKey)).Scan(&ok); err != nil || !ok {
		_ = tx.Rollback()
		if err != nil {
			return nil, err
		}
		return nil, ErrLiandongRunBusy
	}
	return tx, nil
}
func browserReadConfig(ctx context.Context, tx *sql.Tx) (*LiandongBrowserConfig, error) {
	c := &LiandongBrowserConfig{Products: []LiandongBrowserProduct{}}
	var raw []byte
	err := tx.QueryRowContext(ctx, "SELECT enabled,products FROM liandong_browser_config WHERE id=TRUE").Scan(&c.Enabled, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &c.Products); err != nil {
		return nil, err
	}
	c.configured = true
	return c, nil
}
func browserDevice(ctx context.Context, tx *sql.Tx, id string) (*LiandongBrowserDevice, error) {
	d := &LiandongBrowserDevice{}
	var scope []byte
	err := tx.QueryRowContext(ctx, `SELECT id,name,revoked,paused_reason,last_seen_at,authorization_verified_at,goods_ids FROM liandong_browser_devices WHERE id=$1 FOR UPDATE`, id).Scan(&d.ID, &d.Name, &d.Revoked, &d.PausedReason, &d.LastSeenAt, &d.AuthorizationVerifiedAt, &scope)
	if errors.Is(err, sql.ErrNoRows) || d.Revoked {
		return nil, errBrowserAuth
	}
	if err == nil {
		err = json.Unmarshal(scope, &d.GoodsIDs)
	}
	return d, err
}
func browserProduct(c *LiandongBrowserConfig, id int64) (*LiandongBrowserProduct, error) {
	for i := range c.Products {
		if c.Products[i].GoodsID == id && c.Products[i].Enabled {
			return &c.Products[i], nil
		}
	}
	return nil, errBrowserInvalid
}
func validateBrowserProducts(products []LiandongBrowserProduct) error {
	if len(products) > 30 {
		return errBrowserInvalid
	}
	seen := map[int64]bool{}
	amounts := map[int]bool{}
	for i := range products {
		p := &products[i]
		u, e := url.Parse(p.ExternalURL)
		if p.GoodsID <= 0 || p.CNYAmount <= 0 || p.CNYAmount > 10000 || p.USDCredit != float64(p.CNYAmount) || p.TargetStock < 1 || p.TargetStock > 100 || p.BatchSize < 1 || p.BatchSize > 20 || seen[p.GoodsID] || amounts[p.CNYAmount] || e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !(u.Hostname() == "ldxp.cn" || strings.HasSuffix(u.Hostname(), ".ldxp.cn") || (u.Hostname() == "wzyp.cn" && strings.HasPrefix(u.Path, "/item/"))) {
			return errBrowserInvalid
		}
		seen[p.GoodsID] = true
		amounts[p.CNYAmount] = true
		p.Title = fmt.Sprintf("%d元额度", p.CNYAmount)
		p.CurrentStock = 0
		p.IdentityVerified = false
		p.InventoryAt = nil
	}
	return nil
}
func (s *LiandongRestockService) BrowserSaveConfig(ctx context.Context, c LiandongBrowserConfig) (*LiandongBrowserStatus, error) {
	if err := validateBrowserProducts(c.Products); err != nil {
		return nil, err
	}
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	old, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	// Configuration cannot retarget codes already handed to a browser or legacy worker.
	var legacyOpen bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_restock_batches WHERE browser_device_id IS NULL AND status IN ('pending','needs_reconciliation'))`).Scan(&legacyOpen); err != nil {
		return nil, err
	}
	if legacyOpen {
		return nil, ErrLiandongNeedsReconciliation
	}
	for _, p := range old.Products {
		var open bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_restock_batches WHERE goods_id=$1 AND browser_device_id IS NOT NULL AND status IN ('pending','needs_reconciliation'))`, p.GoodsID).Scan(&open); err != nil {
			return nil, err
		}
		if open {
			found := false
			for _, q := range c.Products {
				if q.GoodsID == p.GoodsID && q.CNYAmount == p.CNYAmount && q.ExternalURL == p.ExternalURL {
					found = true
				}
			}
			if !found {
				return nil, ErrLiandongNeedsReconciliation
			}
		}
	}
	raw, err := json.Marshal(c.Products)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO liandong_browser_config(id,enabled,products) VALUES(TRUE,$1,$2::jsonb) ON CONFLICT(id) DO UPDATE SET enabled=$1,products=$2::jsonb,updated_at=NOW()`, c.Enabled, string(raw)); err != nil {
		return nil, err
	}
	for _, p := range c.Products {
		key := fmt.Sprintf("browser-%d-%d", p.GoodsID, p.CNYAmount)
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_product_mappings SET enabled=FALSE WHERE goods_id=$1 AND mapping_key<>$2`, p.GoodsID, key); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO liandong_product_mappings(mapping_key,goods_id,cny_amount,grant_value,external_url,target_stock,enabled) VALUES($1,$2,$3,$3,$4,$5,$6) ON CONFLICT(mapping_key) DO UPDATE SET external_url=$4,target_stock=$5,enabled=$6`, key, p.GoodsID, p.CNYAmount, p.ExternalURL, p.TargetStock, p.Enabled); err != nil {
			return nil, err
		}
	}
	// Ownership changes invalidate only the affected product. Worker policy changes
	// retain proven sale inventory; upload admission separately checks config time.
	for _, previous := range old.Products {
		same := false
		for _, p := range c.Products {
			if p.GoodsID == previous.GoodsID && p.CNYAmount == previous.CNYAmount && p.ExternalURL == previous.ExternalURL && p.Enabled == previous.Enabled {
				same = true
				break
			}
		}
		if !same {
			if _, err = tx.ExecContext(ctx, `DELETE FROM liandong_browser_inventory WHERE goods_id=$1`, previous.GoodsID); err != nil {
				return nil, err
			}
			retained := false
			for _, p := range c.Products {
				if p.GoodsID == previous.GoodsID {
					retained = true
					break
				}
			}
			if !retained {
				if _, err = tx.ExecContext(ctx, `UPDATE liandong_product_mappings SET enabled=FALSE WHERE goods_id=$1 AND mapping_key LIKE 'browser-%'`, previous.GoodsID); err != nil {
					return nil, err
				}
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.BrowserStatus(ctx)
}
func (s *LiandongRestockService) BrowserCreateDevice(ctx context.Context, name string, goodsIDs []int64) (*LiandongBrowserDevice, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return nil, "", errBrowserInvalid
	}
	id, err := browserRandomHex(16)
	if err != nil {
		return nil, "", err
	}
	secret, err := browserRandomHex(32)
	if err != nil {
		return nil, "", err
	}
	key := "ldxpd_" + secret
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()
	config, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, "", err
	}
	if len(goodsIDs) == 0 {
		for _, p := range config.Products {
			if p.Enabled {
				goodsIDs = append(goodsIDs, p.GoodsID)
			}
		}
	}
	if len(goodsIDs) == 0 {
		return nil, "", errBrowserInvalid
	}
	seen := map[int64]bool{}
	for _, goodsID := range goodsIDs {
		if _, err = browserProduct(config, goodsID); err != nil || seen[goodsID] {
			return nil, "", errBrowserInvalid
		}
		seen[goodsID] = true
	}
	scope, err := json.Marshal(goodsIDs)
	if err != nil {
		return nil, "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO liandong_browser_devices(id,name,key_sha256,goods_ids) VALUES($1,$2,$3,$4::jsonb)`, id, name, browserHash(key), string(scope)); err != nil {
		return nil, "", err
	}
	if err = tx.Commit(); err != nil {
		return nil, "", err
	}
	return &LiandongBrowserDevice{ID: id, Name: name, GoodsIDs: goodsIDs}, key, nil
}
func (s *LiandongRestockService) BrowserRevokeDevice(ctx context.Context, id string) error {
	tx, err := s.browserTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_devices SET revoked=TRUE,paused_reason='revoked' WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE liandong_restock_batches SET status='needs_reconciliation',updated_at=NOW() WHERE browser_device_id=$1 AND status='pending'`, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *LiandongRestockService) BrowserAuthenticate(ctx context.Context, key string) (string, error) {
	if len(key) != 70 || !strings.HasPrefix(key, "ldxpd_") || s.db == nil {
		return "", errBrowserAuth
	}
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM liandong_browser_devices WHERE key_sha256=$1 AND NOT revoked`, browserHash(key)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errBrowserAuth
	}
	return id, err
}
func (s *LiandongRestockService) BrowserConfig(ctx context.Context, id string) (*LiandongBrowserConfig, *LiandongBrowserDevice, error) {
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	d, err := browserDevice(ctx, tx, id)
	if err != nil {
		return nil, nil, err
	}
	c.PausedReason = d.PausedReason
	filtered := []LiandongBrowserProduct{}
	for _, p := range c.Products {
		if browserDeviceAllows(d, p.GoodsID) {
			filtered = append(filtered, p)
		}
	}
	c.Products = filtered

	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	return c, d, nil
}
func (s *LiandongRestockService) BrowserHeartbeat(ctx context.Context, id, authorization string) (*LiandongBrowserConfig, error) {
	if authorization != "verified" && authorization != "failed" {
		return nil, errBrowserInvalid
	}
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	device, err := browserDevice(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if device.LastSeenAt != nil && time.Since(*device.LastSeenAt) > 2*time.Minute {
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_devices SET paused_reason=CASE WHEN paused_reason='' THEN 'disconnected' ELSE paused_reason END WHERE id=$1`, id); err != nil {
			return nil, err
		}
	}
	if authorization == "failed" {
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_devices SET last_seen_at=NOW(),authorization_verified_at=NULL,paused_reason='authorization_failed' WHERE id=$1`, id); err != nil {
			return nil, err
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_devices SET last_seen_at=NOW(),authorization_verified_at=NOW() WHERE id=$1`, id); err != nil {
			return nil, err
		}
	}
	c, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	d, err := browserDevice(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	c.PausedReason = d.PausedReason
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}

func browserLoadBatch(ctx context.Context, tx *sql.Tx, batchID string, goodsID int64, includeCodes bool) (*LiandongBrowserBatch, error) {
	b := &LiandongBrowserBatch{}
	var status string
	q := `SELECT batch_id,goods_id,status,code_count,created_at FROM liandong_restock_batches WHERE browser_device_id IS NOT NULL AND `
	args := []any{}
	if batchID != "" {
		q += `batch_id=$1`
		args = append(args, batchID)
	} else {
		q += `goods_id=$1 AND status IN ('pending','needs_reconciliation') ORDER BY created_at LIMIT 1`
		args = append(args, goodsID)
	}
	err := tx.QueryRowContext(ctx, q, args...).Scan(&b.BatchID, &b.GoodsID, &status, &b.CodeCount, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	switch status {
	case "pending":
		b.Status = "claimed"
	case "uploaded":
		b.Status = "verified"
	default:
		b.Status = "uncertain"
	}
	if includeCodes {
		rows, err := tx.QueryContext(ctx, `SELECT r.code,bc.code_sha256 FROM liandong_restock_batch_codes bc JOIN redeem_codes r ON r.id=bc.redeem_code_id WHERE bc.batch_id=$1 ORDER BY bc.ordinal`, b.BatchID)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var code, hash string
			if err = rows.Scan(&code, &hash); err != nil {
				return nil, err
			}
			b.Codes = append(b.Codes, code)
			b.CodeHashes = append(b.CodeHashes, hash)
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}
		if len(b.Codes) != b.CodeCount {
			return nil, ErrLiandongNeedsReconciliation
		}
	}
	return b, nil
}
func validateBrowserReport(r LiandongBrowserInventoryReport) error {
	if r.GoodsID <= 0 || !r.Complete || r.Total < 0 || r.Total > 100000 || r.Total != len(r.Hashes) {
		return errBrowserInvalid
	}
	seen := map[string]bool{}
	for _, h := range r.Hashes {
		b, e := hex.DecodeString(h)
		if e != nil || len(b) != 32 || h != strings.ToLower(h) || seen[h] {
			return errBrowserInvalid
		}
		seen[h] = true
	}
	return nil
}
func (s *LiandongRestockService) BrowserInventory(ctx context.Context, id string, r LiandongBrowserInventoryReport) (*LiandongBrowserInventoryResult, error) {
	if err := validateBrowserReport(r); err != nil {
		return nil, err
	}
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d, err := browserDevice(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if d.AuthorizationVerifiedAt == nil || time.Since(*d.AuthorizationVerifiedAt) > 2*time.Minute {
		return nil, errBrowserPaused
	}
	c, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !browserDeviceAllows(d, r.GoodsID) {
		return nil, errBrowserAuth
	}
	p, err := browserProduct(c, r.GoodsID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(r.Hashes)
	if err != nil {
		return nil, err
	}
	if r.Hashes == nil {
		raw = []byte("[]")
	}
	var matched int
	// Manual stock is eligible only if it is unused balance credit in this database.
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM redeem_codes r WHERE encode(sha256(convert_to(r.code,'UTF8')),'hex') IN (SELECT jsonb_array_elements_text($1::jsonb)) AND r.type='balance' AND r.status='unused' AND r.value=$2 AND r.group_id IS NULL AND (r.expires_at IS NULL OR r.expires_at>NOW()) AND NOT EXISTS (SELECT 1 FROM liandong_code_refunds f WHERE f.redeem_code_id=r.id)`, string(raw), p.CNYAmount).Scan(&matched)
	if err != nil {
		return nil, err
	}
	var wrongGoods bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_restock_batch_codes bc JOIN liandong_restock_batches b USING(batch_id) WHERE b.goods_id<>$2 AND bc.code_sha256 IN (SELECT jsonb_array_elements_text($1::jsonb))) OR EXISTS(SELECT 1 FROM liandong_browser_inventory i WHERE i.goods_id<>$2 AND EXISTS(SELECT 1 FROM jsonb_array_elements_text(i.code_hashes) h WHERE h IN (SELECT jsonb_array_elements_text($1::jsonb))))`, string(raw), p.GoodsID).Scan(&wrongGoods)
	if err != nil {
		return nil, err
	}
	verified := matched == r.Total && !wrongGoods
	if _, err = tx.ExecContext(ctx, `INSERT INTO liandong_browser_inventory(goods_id,device_id,identity_verified,code_hashes) VALUES($1,$2,$3,$4::jsonb) ON CONFLICT(goods_id) DO UPDATE SET device_id=$2,identity_verified=$3,code_hashes=$4::jsonb,observed_at=NOW()`, p.GoodsID, id, verified, string(raw)); err != nil {
		return nil, err
	}
	b, err := browserLoadBatch(ctx, tx, "", p.GoodsID, true)
	if err != nil {
		return nil, err
	}
	result := &LiandongBrowserInventoryResult{IdentityVerified: verified, MatchedStock: matched, Blocked: !verified, PendingBatch: b}
	if r.BatchID != "" {
		var terminal bool
		if err = tx.QueryRowContext(ctx, `SELECT status='uploaded' FROM liandong_restock_batches WHERE batch_id=$1 AND goods_id=$2 AND browser_device_id=$3`, r.BatchID, r.GoodsID, id).Scan(&terminal); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errBrowserInvalid
			}
			return nil, err
		}
		result.BatchResolved = terminal
	}
	if b != nil {
		all := verified
		seen := map[string]bool{}
		for _, h := range r.Hashes {
			seen[h] = true
		}
		for _, h := range b.CodeHashes {
			if !seen[h] {
				all = false
			}
		}
		if all {
			if _, err = tx.ExecContext(ctx, `UPDATE liandong_restock_batches SET status='uploaded',remote_stock_after=$2,uploaded_at=NOW(),updated_at=NOW(),error=NULL WHERE batch_id=$1`, b.BatchID, r.Total); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE liandong_restock_segments SET status='uploaded',remote_acknowledged=TRUE,uploaded_at=NOW(),updated_at=NOW() WHERE batch_id=$1`, b.BatchID); err != nil {
				return nil, err
			}
			result.BatchResolved = true
			result.PendingBatch = nil
		} else if b.Status == "uncertain" {
			result.Blocked = true
		}
		// Inventory reports never expose another device's issued codes.
		if result.PendingBatch != nil {
			var owner string
			if err = tx.QueryRowContext(ctx, `SELECT browser_device_id FROM liandong_restock_batches WHERE batch_id=$1`, b.BatchID).Scan(&owner); err != nil {
				return nil, err
			}
			if owner != id {
				result.Blocked = true
				result.PendingBatch.Codes = nil
				result.PendingBatch.CodeHashes = nil
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_devices SET last_seen_at=NOW() WHERE id=$1`, id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *LiandongRestockService) BrowserClaim(ctx context.Context, id string, goodsID int64) (*LiandongBrowserBatch, error) {
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d, err := browserDevice(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	c, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !browserDeviceAllows(d, goodsID) {
		return nil, errBrowserAuth
	}
	p, err := browserProduct(c, goodsID)
	if err != nil {
		return nil, err
	}
	if !c.Enabled || d.PausedReason != "" || d.LastSeenAt == nil || time.Since(*d.LastSeenAt) > 2*time.Minute || d.AuthorizationVerifiedAt == nil || time.Since(*d.AuthorizationVerifiedAt) > 2*time.Minute {
		return nil, errBrowserPaused
	}
	b, err := browserLoadBatch(ctx, tx, "", goodsID, true)
	if err != nil {
		return nil, err
	}
	if b != nil {
		var owner string
		if err = tx.QueryRowContext(ctx, `SELECT browser_device_id FROM liandong_restock_batches WHERE batch_id=$1`, b.BatchID).Scan(&owner); err != nil {
			return nil, err
		}
		if owner != id || b.Status == "uncertain" {
			return nil, ErrLiandongNeedsReconciliation
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return b, nil
	}
	var verified bool
	var observed time.Time
	var raw []byte
	var owner string
	err = tx.QueryRowContext(ctx, `SELECT identity_verified,observed_at,code_hashes,device_id FROM liandong_browser_inventory WHERE goods_id=$1`, goodsID).Scan(&verified, &observed, &raw, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errBrowserPaused
	}
	if err != nil {
		return nil, err
	}
	if !verified || time.Since(observed) > 2*time.Minute || owner != id {
		return nil, errBrowserPaused
	}
	var hashes []string
	if err = json.Unmarshal(raw, &hashes); err != nil {
		return nil, err
	}
	count := minInt(p.BatchSize, p.TargetStock-len(hashes))
	if count <= 0 {
		return nil, nil
	}
	batchID, err := browserRandomHex(16)
	if err != nil {
		return nil, err
	}
	b = &LiandongBrowserBatch{BatchID: "browser-" + batchID, GoodsID: goodsID, Status: "claimed", CodeCount: count, CreatedAt: time.Now().UTC()}
	for i := 0; i < count; i++ {
		code, err := browserRandomHex(16)
		if err != nil {
			return nil, err
		}
		b.Codes = append(b.Codes, code)
		b.CodeHashes = append(b.CodeHashes, browserHash(code))
	}
	snapshot, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("browser-%d-%d", goodsID, p.CNYAmount)
	if _, err = tx.ExecContext(ctx, `INSERT INTO liandong_restock_batches(batch_id,goods_id,cny_amount,grant_value,code_count,code_sha256,status,remote_stock_before,created_at,mapping_key,external_url,target_stock,planned_count,mapping_snapshot,browser_device_id) VALUES($1,$2,$3,$3,$4,$5,'pending',$6,$7,$8,$9,$10,$4,$11::jsonb,$12)`, b.BatchID, goodsID, p.CNYAmount, count, liandongCodesDigest(b.Codes), len(hashes), b.CreatedAt, key, p.ExternalURL, p.TargetStock, string(snapshot), id); err != nil {
		return nil, err
	}
	for i, code := range b.Codes {
		var redeemID int64
		if err = tx.QueryRowContext(ctx, `INSERT INTO redeem_codes(code,type,value,status,notes) VALUES($1,'balance',$2,'unused',$3) RETURNING id`, code, p.CNYAmount, "LDXP browser batch "+b.BatchID).Scan(&redeemID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO liandong_restock_batch_codes(batch_id,code_sha256,code_hint,ordinal,redeem_code_id) VALUES($1,$2,$3,$4,$5)`, b.BatchID, b.CodeHashes[i], code[:8], i, redeemID); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO liandong_restock_segments(batch_id,segment_no,ordinal_start,code_count,code_sha256,status) VALUES($1,0,0,$2,$3,'codes_created')`, b.BatchID, count, liandongCodesDigest(b.Codes)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}
func (s *LiandongRestockService) BrowserBatchAction(ctx context.Context, id, batchID, outcome string) (*LiandongBrowserBatch, error) {
	if outcome != "start" && outcome != "imported" && outcome != "uncertain" && outcome != "authorization_failed" {
		return nil, errBrowserInvalid
	}
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d, err := browserDevice(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	var owner, status string
	if err = tx.QueryRowContext(ctx, `SELECT browser_device_id,status FROM liandong_restock_batches WHERE batch_id=$1 AND browser_device_id IS NOT NULL FOR UPDATE`, batchID).Scan(&owner, &status); err != nil {
		return nil, err
	}
	if owner != id {
		return nil, errBrowserAuth
	}
	if outcome == "start" {
		var goodsID int64
		if err = tx.QueryRowContext(ctx, `SELECT goods_id FROM liandong_restock_batches WHERE batch_id=$1`, batchID).Scan(&goodsID); err != nil {
			return nil, err
		}
		c, err := browserReadConfig(ctx, tx)
		if err != nil {
			return nil, err
		}
		if _, err = browserProduct(c, goodsID); err != nil {
			return nil, err
		}
		if !browserDeviceAllows(d, goodsID) {
			return nil, errBrowserAuth
		}
		if !c.Enabled || d.PausedReason != "" || d.AuthorizationVerifiedAt == nil || time.Since(*d.AuthorizationVerifiedAt) > 2*time.Minute {
			return nil, errBrowserPaused
		}
		if status != "pending" {
			return nil, ErrLiandongNeedsReconciliation
		}
		var fresh bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_browser_inventory WHERE goods_id=$1 AND device_id=$2 AND identity_verified AND observed_at>NOW()-INTERVAL '2 minutes' AND observed_at >= (SELECT updated_at FROM liandong_browser_config WHERE id=TRUE))`, goodsID, id).Scan(&fresh); err != nil {
			return nil, err
		}
		if !fresh {
			return nil, errBrowserPaused
		}
		var intact bool
		if err = tx.QueryRowContext(ctx, `SELECT b.code_count=(SELECT count(*) FROM liandong_restock_batch_codes bc JOIN redeem_codes r ON r.id=bc.redeem_code_id WHERE bc.batch_id=b.batch_id AND bc.code_sha256=encode(sha256(convert_to(r.code,'UTF8')),'hex') AND r.status='unused' AND r.type='balance' AND r.value=b.grant_value AND r.group_id IS NULL AND (r.expires_at IS NULL OR r.expires_at>NOW()) AND NOT EXISTS(SELECT 1 FROM liandong_code_refunds f WHERE f.redeem_code_id=r.id)) FROM liandong_restock_batches b WHERE b.batch_id=$1`, batchID).Scan(&intact); err != nil {
			return nil, err
		}
		if !intact {
			return nil, ErrLiandongNeedsReconciliation
		}
	}
	// Persist uncertainty before the browser may perform any remote write.
	if status != "uploaded" {
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_restock_batches SET status='needs_reconciliation',updated_at=NOW() WHERE batch_id=$1`, batchID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_restock_segments SET status='needs_reconciliation',updated_at=NOW() WHERE batch_id=$1`, batchID); err != nil {
			return nil, err
		}
	}
	if outcome == "authorization_failed" {
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_devices SET paused_reason='authorization_failed',authorization_verified_at=NULL WHERE id=$1`, id); err != nil {
			return nil, err
		}
	}
	b, err := browserLoadBatch(ctx, tx, batchID, 0, true)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}
func browserCanResume(ctx context.Context, tx *sql.Tx, c *LiandongBrowserConfig, d *LiandongBrowserDevice) error {
	if d.AuthorizationVerifiedAt == nil || time.Since(*d.AuthorizationVerifiedAt) > 2*time.Minute {
		return errBrowserPaused
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_restock_batches WHERE browser_device_id=$1 AND status='needs_reconciliation')`, d.ID).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrLiandongNeedsReconciliation
	}
	var verified bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_browser_inventory WHERE device_id=$1 AND identity_verified AND observed_at>NOW()-INTERVAL '2 minutes')`, d.ID).Scan(&verified); err != nil {
		return err
	}
	if !verified {
		return errBrowserPaused
	}
	return nil
}
func (s *LiandongRestockService) BrowserResume(ctx context.Context, id string) (*LiandongBrowserConfig, error) {
	tx, err := s.browserTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	ids := []string{id}
	if id == "" {
		ids = nil
		rows, err := tx.QueryContext(ctx, `SELECT id FROM liandong_browser_devices WHERE NOT revoked AND paused_reason<>''`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var value string
			if err = rows.Scan(&value); err != nil {
				_ = rows.Close()
				return nil, err
			}
			ids = append(ids, value)
		}
		if err = rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	for _, deviceID := range ids {
		d, err := browserDevice(ctx, tx, deviceID)
		if err != nil {
			return nil, err
		}
		if err = browserCanResume(ctx, tx, c, d); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_devices SET paused_reason='' WHERE id=$1`, deviceID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *LiandongRestockService) BrowserStatus(ctx context.Context) (*LiandongBrowserStatus, error) {
	tx, err := s.browserReadTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	out := &LiandongBrowserStatus{LiandongBrowserConfig: *c, Devices: []LiandongBrowserDevice{}, Batches: []LiandongBrowserBatch{}}
	for i := range out.Products {
		p := &out.Products[i]
		var raw []byte
		var at time.Time
		var verified bool
		err = tx.QueryRowContext(ctx, `SELECT identity_verified,code_hashes,observed_at FROM liandong_browser_inventory WHERE goods_id=$1`, p.GoodsID).Scan(&verified, &raw, &at)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var hashes []string
		if err = json.Unmarshal(raw, &hashes); err != nil {
			return nil, err
		}
		p.CurrentStock = len(hashes)
		p.IdentityVerified = verified
		p.InventoryAt = &at
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,name,revoked,paused_reason,last_seen_at,authorization_verified_at,goods_ids FROM liandong_browser_devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d LiandongBrowserDevice
		var scope []byte
		if err = rows.Scan(&d.ID, &d.Name, &d.Revoked, &d.PausedReason, &d.LastSeenAt, &d.AuthorizationVerifiedAt, &scope); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err = json.Unmarshal(scope, &d.GoodsIDs); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if !d.Revoked && d.PausedReason != "" {
			out.PausedReason = d.PausedReason
		}
		out.Devices = append(out.Devices, d)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT batch_id,goods_id,status,code_count,created_at FROM liandong_restock_batches WHERE browser_device_id IS NOT NULL ORDER BY created_at DESC LIMIT 30`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b LiandongBrowserBatch
		if err = rows.Scan(&b.BatchID, &b.GoodsID, &b.Status, &b.CodeCount, &b.CreatedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		switch b.Status {
		case "pending":
			b.Status = "claimed"
		case "uploaded":
			b.Status = "verified"
		default:
			b.Status = "uncertain"
		}
		out.Batches = append(out.Batches, b)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	return out, nil
}
func (s *LiandongRestockService) BrowserPublicProducts(ctx context.Context) ([]LiandongRechargeProduct, error) {
	tx, err := s.browserReadTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !c.configured {
		return nil, infraerrors.NotFound("LDXP_BROWSER_NOT_CONFIGURED", "Chrome 商品目录尚未配置")
	}
	out := []LiandongRechargeProduct{}
	for _, p := range c.Products {
		if !p.Enabled {
			continue
		}
		var ready bool
		// Sale availability follows product ownership and redeemability, independent of worker connectivity.
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_browser_inventory i WHERE i.goods_id=$1 AND i.identity_verified AND EXISTS(SELECT 1 FROM redeem_codes r WHERE encode(sha256(convert_to(r.code,'UTF8')),'hex') IN(SELECT jsonb_array_elements_text(i.code_hashes)) AND r.status='unused' AND r.type='balance' AND r.value=$2 AND (r.expires_at IS NULL OR r.expires_at>NOW()) AND NOT EXISTS(SELECT 1 FROM liandong_code_refunds f WHERE f.redeem_code_id=r.id)))`, p.GoodsID, p.CNYAmount).Scan(&ready)
		if err != nil {
			return nil, err
		}
		if ready {
			out = append(out, LiandongRechargeProduct{GoodsID: p.GoodsID, CNYAmount: p.CNYAmount, USDCredit: p.USDCredit, ExternalURL: p.ExternalURL, Title: p.Title})
		}
	}
	return out, nil
}

func browserDeviceAllows(d *LiandongBrowserDevice, goodsID int64) bool {
	for _, id := range d.GoodsIDs {
		if id == goodsID {
			return true
		}
	}
	return false
}

func (s *LiandongRestockService) browserReadTx(ctx context.Context) (*sql.Tx, error) {
	if s.db == nil {
		return nil, infraerrors.ServiceUnavailable("LDXP_BROWSER_UNAVAILABLE", "持久存储不可用")
	}
	return s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
}
