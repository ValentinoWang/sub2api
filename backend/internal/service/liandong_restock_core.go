package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	liandongDefaultTargetStock = 50000
	liandongMaxTargetStock     = 1000000
	liandongSegmentSize        = 1000
	liandongCodeLength         = 20
	liandongCodeAlphabet       = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	LiandongRestockJobQueued                 = "queued"
	LiandongRestockJobRunning                = "running"
	LiandongRestockJobCompleted              = "completed"
	LiandongRestockJobFailed                 = "failed"
	LiandongRestockJobNeedsReconciliation    = "needs_reconciliation"
	liandongBatchStatusPending               = "pending"
	liandongBatchStatusUploaded              = "uploaded"
	liandongBatchStatusFailed                = "failed"
	liandongBatchStatusNeedsReconciliation   = "needs_reconciliation"
	liandongSegmentStatusPending             = "pending"
	liandongSegmentStatusCodesCreated        = "codes_created"
	liandongSegmentStatusUploaded            = "uploaded"
	liandongSegmentStatusFailed              = "failed"
	liandongSegmentStatusNeedsReconciliation = "needs_reconciliation"
)

var (
	ErrLiandongJobNotFound         = errors.New("Liandong restock job not found")
	ErrLiandongRunBusy             = errors.New("Liandong restock already has an active cycle")
	ErrLiandongNeedsReconciliation = errors.New("Liandong remote write needs reconciliation before retry")
	ErrLiandongJobNotResumable     = errors.New("Liandong restock job is not resumable")
)

// LiandongRestockMappingSnapshot is the immutable product mapping attached to
// a preview item and copied into every durable restock batch.
type LiandongRestockMappingSnapshot struct {
	MappingKey  string  `json:"mapping_key"`
	Version     int     `json:"version"`
	GoodsID     int64   `json:"goods_id"`
	CNYAmount   int     `json:"cny_amount"`
	GrantType   string  `json:"grant_type"`
	USDCredit   float64 `json:"usd_credit"`
	ExternalURL string  `json:"external_url,omitempty"`
	TargetStock int     `json:"target_stock"`
}

// LiandongRestockPreviewItem contains only operational facts. It never
// contains a redeem code or a credential.
type LiandongRestockPreviewItem struct {
	Mapping      LiandongRestockMappingSnapshot `json:"mapping"`
	CurrentStock *int                           `json:"current_stock,omitempty"`
	TargetStock  int                            `json:"target_stock"`
	Planned      int                            `json:"planned"`
	Enabled      bool                           `json:"enabled"`
	Eligible     bool                           `json:"eligible"`
	Reason       string                         `json:"reason,omitempty"`
	Error        string                         `json:"error,omitempty"`
	BatchID      string                         `json:"batch_id,omitempty"`
}

// LiandongRestockPreview is the read-only result returned before a job is
// created. An empty selected-goods list means all configured products.
type LiandongRestockPreview struct {
	Products  []LiandongRestockPreviewItem `json:"products"`
	CreatedAt string                       `json:"created_at"`
}

// LiandongRestockPreviewResult is kept as a descriptive alias for callers
// that name the result rather than the operation.
type LiandongRestockPreviewResult = LiandongRestockPreview

// LiandongRestockJobSummary is a durable, secret-free job read model.
type LiandongRestockJobSummary struct {
	JobID         string                       `json:"job_id"`
	Status        string                       `json:"status"`
	SelectedGoods []int64                      `json:"selected_goods"`
	Products      []LiandongRestockPreviewItem `json:"products"`
	Batches       []LiandongRestockBatchStatus `json:"batches,omitempty"`
	TotalPlanned  int                          `json:"total_planned"`
	TotalUploaded int                          `json:"total_uploaded"`
	Error         string                       `json:"error,omitempty"`
	CreatedAt     string                       `json:"created_at"`
	UpdatedAt     string                       `json:"updated_at"`
	CompletedAt   string                       `json:"completed_at,omitempty"`
}

// LiandongRestockJob is an alias that makes the service's job read model
// convenient for integrations that use a noun for the returned type.
type LiandongRestockJob = LiandongRestockJobSummary

// LiandongRestockSegmentStatus is a non-secret segment read model.
type LiandongRestockSegmentStatus struct {
	BatchID    string `json:"batch_id"`
	SegmentNo  int    `json:"segment_no"`
	Offset     int    `json:"offset"`
	CodeCount  int    `json:"code_count"`
	CodeSHA256 string `json:"code_sha256"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	UploadedAt string `json:"uploaded_at,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

// LiandongRestockJobExport is an attachment-oriented export. The reader may
// contain complete codes, so callers must stream it as an attachment and must
// never marshal this value into a JSON response or log it.
type LiandongRestockJobExport struct {
	Filename    string
	ContentType string
	CodeCount   int
	Reader      io.ReadCloser
}

// LiandongRemoteFailureError means the remote endpoint returned a definite
// rejection. It is safe to retry the same deterministic segment.
type LiandongRemoteFailureError struct {
	StatusCode int
	Message    string
}

func (e *LiandongRemoteFailureError) Error() string {
	if e == nil {
		return "Liandong remote request failed"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("Liandong remote request failed with HTTP %d: %s", e.StatusCode, e.Message)
	}
	return "Liandong remote request failed: " + e.Message
}

// LiandongRemoteOutcomeUnknownError means the request may have reached the
// remote write endpoint but its result cannot be confirmed locally.
type LiandongRemoteOutcomeUnknownError struct {
	Err error
}

func (e *LiandongRemoteOutcomeUnknownError) Error() string {
	if e == nil || e.Err == nil {
		return "Liandong remote write outcome is unknown"
	}
	return "Liandong remote write outcome is unknown: " + e.Err.Error()
}

func (e *LiandongRemoteOutcomeUnknownError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type liandongMemorySegment struct {
	LiandongRestockSegmentStatus
}

type liandongMemoryBatch struct {
	Batch       liandongRestockPendingBatch
	Codes       []string
	CodeSHA256  string
	Status      string
	Error       string
	Segments    []liandongMemorySegment
	CreatedAt   string
	UploadedAt  string
	UpdatedAt   string
	RemoteAfter *int
}

type liandongCycleResult struct {
	Products []LiandongRestockPreviewItem
	Batches  []string
}

func deriveLegacyLiandongTargetStock(threshold, restockCount int) int {
	if threshold < 0 {
		threshold = 0
	}
	if restockCount <= 0 {
		restockCount = 10
	}
	if threshold > restockCount {
		return threshold
	}
	return restockCount
}

func normalizeStoredLiandongProducts(products []LiandongRestockProduct) []LiandongRestockProduct {
	out := cloneLiandongProducts(products)
	for i := range out {
		if out[i].TargetStock <= 0 {
			// A pre-target configuration has no durable target. Retain its old
			// safety floor instead of silently jumping to 50,000 on upgrade.
			out[i].TargetStock = deriveLegacyLiandongTargetStock(out[i].Threshold, out[i].RestockCount)
		}
	}
	return normalizeLiandongProducts(out)
}

func effectiveLiandongTargetStock(product LiandongRestockProduct) int {
	if product.TargetStock > 0 {
		return product.TargetStock
	}
	// Direct construction is used by older callers and tests. Keep that
	// compatibility path bounded by the legacy replenishment quantity.
	if product.RestockCount > 0 {
		return product.RestockCount
	}
	if product.Threshold > 0 {
		return product.Threshold
	}
	return 1
}

func liandongTargetStockIsValid(target int) bool {
	return target > 0 && target <= liandongMaxTargetStock
}

func liandongPlanAddition(current, target int) (int, error) {
	if current < 0 {
		return 0, errors.New("current Liandong stock cannot be negative")
	}
	if target <= 0 {
		return 0, errors.New("target Liandong stock must be positive")
	}
	if target <= current {
		return 0, nil
	}
	return target - current, nil
}

func liandongMappingKey(product LiandongRestockProduct) string {
	grantType := product.GrantType
	if grantType == "" {
		grantType = "balance"
	}
	version := product.Version
	if version <= 0 {
		version = 1
	}
	keyInput := fmt.Sprintf("%s:%d:%d:%.8f:%s:%d", grantType, product.GoodsID, product.CNYAmount, product.USDCredit, product.ExternalURL, version)
	digest := sha256.Sum256([]byte(keyInput))
	return hex.EncodeToString(digest[:])
}

func liandongMappingSnapshot(product LiandongRestockProduct) LiandongRestockMappingSnapshot {
	grantType := product.GrantType
	if grantType == "" {
		grantType = "balance"
	}
	version := product.Version
	if version <= 0 {
		version = 1
	}
	return LiandongRestockMappingSnapshot{
		MappingKey:  liandongMappingKey(product),
		Version:     version,
		GoodsID:     product.GoodsID,
		CNYAmount:   product.CNYAmount,
		GrantType:   grantType,
		USDCredit:   product.USDCredit,
		ExternalURL: product.ExternalURL,
		TargetStock: effectiveLiandongTargetStock(product),
	}
}

func liandongPreviewItem(product LiandongRestockProduct, current *int, planned int, reason string) LiandongRestockPreviewItem {
	target := effectiveLiandongTargetStock(product)
	item := LiandongRestockPreviewItem{
		Mapping:      liandongMappingSnapshot(product),
		CurrentStock: current,
		TargetStock:  target,
		Planned:      planned,
		Enabled:      product.Enabled,
		Eligible:     product.Enabled && planned > 0,
		Reason:       reason,
	}
	return item
}

func newLiandongPendingBatch(product LiandongRestockProduct, current, planned int, jobID string, createdAt string) *liandongRestockPendingBatch {
	mapping := liandongMappingSnapshot(product)
	currentCopy := current
	return &liandongRestockPendingBatch{
		JobID:             jobID,
		GoodsID:           mapping.GoodsID,
		CNYAmount:         mapping.CNYAmount,
		USDCredit:         mapping.USDCredit,
		GrantType:         mapping.GrantType,
		ExternalURL:       mapping.ExternalURL,
		Version:           mapping.Version,
		MappingKey:        mapping.MappingKey,
		TargetStock:       mapping.TargetStock,
		Count:             planned,
		CreatedAt:         createdAt,
		RemoteStockBefore: &currentCopy,
	}
}

func hydrateLiandongPendingBatch(batch *liandongRestockPendingBatch, products []LiandongRestockProduct) *liandongRestockPendingBatch {
	if batch == nil {
		return nil
	}
	out := *batch
	var product *LiandongRestockProduct
	for i := range products {
		if products[i].GoodsID == out.GoodsID {
			product = &products[i]
			break
		}
	}
	if product == nil {
		return &out
	}
	mapping := liandongMappingSnapshot(*product)
	if out.CNYAmount <= 0 {
		out.CNYAmount = mapping.CNYAmount
	}
	if out.USDCredit <= 0 {
		out.USDCredit = mapping.USDCredit
	}
	if out.GrantType == "" {
		out.GrantType = mapping.GrantType
	}
	if out.ExternalURL == "" {
		out.ExternalURL = mapping.ExternalURL
	}
	if out.Version <= 0 {
		out.Version = mapping.Version
	}
	if out.MappingKey == "" {
		out.MappingKey = mapping.MappingKey
	}
	if out.TargetStock <= 0 {
		out.TargetStock = mapping.TargetStock
	}
	if out.Count <= 0 {
		out.Count = product.RestockCount
	}
	return &out
}

func validateLiandongCodeSet(codes []string) error {
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if len(code) != liandongCodeLength {
			return fmt.Errorf("Liandong redeem code must contain exactly %d characters", liandongCodeLength)
		}
		for i := 0; i < len(code); i++ {
			if strings.IndexByte(liandongCodeAlphabet, code[i]) < 0 {
				return errors.New("Liandong redeem code contains a non-alphanumeric character")
			}
		}
		if _, exists := seen[code]; exists {
			return errors.New("duplicate Liandong redeem code in batch")
		}
		seen[code] = struct{}{}
	}
	return nil
}

func liandongSegmentRanges(total int) [][2]int {
	if total <= 0 {
		return nil
	}
	segments := make([][2]int, 0, (total+liandongSegmentSize-1)/liandongSegmentSize)
	for offset := 0; offset < total; offset += liandongSegmentSize {
		count := liandongSegmentSize
		if remaining := total - offset; remaining < count {
			count = remaining
		}
		segments = append(segments, [2]int{offset, count})
	}
	return segments
}

func liandongCodesDigest(codes []string) string {
	digest := sha256.Sum256([]byte(strings.Join(codes, "\n")))
	return hex.EncodeToString(digest[:])
}

func (s *LiandongRestockService) deriveCodesChecked(batch *liandongRestockPendingBatch) ([]string, error) {
	if batch == nil || batch.Count <= 0 || batch.Count > liandongMaxTargetStock {
		return nil, errors.New("invalid Liandong batch code count")
	}
	s.configMu.RLock()
	secret := append([]byte(nil), s.codeSecret...)
	s.configMu.RUnlock()
	codes := make([]string, batch.Count)
	for i := 0; i < batch.Count; i++ {
		mac := hmac.New(sha256.New, secret)
		_, _ = mac.Write([]byte(fmt.Sprintf("%s:%d", batch.BatchID, i)))
		digest := mac.Sum(nil)
		var code [liandongCodeLength]byte
		for j := range code {
			code[j] = liandongCodeAlphabet[int(digest[j])%len(liandongCodeAlphabet)]
		}
		codes[i] = string(code[:])
	}
	if err := validateLiandongCodeSet(codes); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *LiandongRestockService) deriveCodes(batch *liandongRestockPendingBatch) []string {
	codes, err := s.deriveCodesChecked(batch)
	if err != nil {
		return nil
	}
	return codes
}

func isLiandongOutcomeUnknown(err error) bool {
	var unknown *LiandongRemoteOutcomeUnknownError
	return errors.As(err, &unknown)
}

func isLiandongNeedsReconciliation(err error) bool {
	return errors.Is(err, ErrLiandongNeedsReconciliation)
}

func (s *LiandongRestockService) ensureLiandongCodes(ctx context.Context, batch *liandongRestockPendingBatch, codes []string) error {
	if s.redeem == nil {
		return errors.New("Liandong redeem storage is unavailable")
	}
	missing := make([]string, 0, len(codes))
	for _, code := range codes {
		existing, err := s.redeem.GetByCode(ctx, code)
		if err == nil {
			if existing == nil || existing.Type != RedeemTypeBalance || math.Abs(existing.Value-batch.USDCredit) > 0.000001 || existing.GroupID != nil || (existing.Status != "" && existing.Status != StatusUnused) || existing.IsExpired() {
				return errors.New("derived Liandong code conflicts with an existing redeem code")
			}
			continue
		}
		if !errors.Is(err, ErrRedeemCodeNotFound) {
			return err
		}
		missing = append(missing, code)
	}
	for _, code := range missing {
		if err := s.redeem.CreateCode(ctx, &RedeemCode{
			Code: code, Type: RedeemTypeBalance, Value: batch.USDCredit,
			Status: StatusUnused, Notes: "liandong:auto:" + batch.BatchID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *LiandongRestockService) loadLiandongSegmentStatuses(ctx context.Context, batchID string) ([]LiandongRestockSegmentStatus, error) {
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		batch, ok := s.memoryBatches[batchID]
		if !ok {
			return nil, nil
		}
		result := make([]LiandongRestockSegmentStatus, 0, len(batch.Segments))
		for _, segment := range batch.Segments {
			result = append(result, segment.LiandongRestockSegmentStatus)
		}
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT segment_no, ordinal_start, code_count, code_sha256, status, error, uploaded_at, updated_at
		FROM liandong_restock_segments WHERE batch_id = $1 ORDER BY segment_no`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LiandongRestockSegmentStatus, 0)
	for rows.Next() {
		var segment LiandongRestockSegmentStatus
		var failure sqlNullString
		var uploadedAt sqlNullTime
		var updatedAt time.Time
		if err := rows.Scan(&segment.SegmentNo, &segment.Offset, &segment.CodeCount, &segment.CodeSHA256, &segment.Status, &failure, &uploadedAt, &updatedAt); err != nil {
			return nil, err
		}
		segment.BatchID = batchID
		if failure.Valid {
			segment.Error = failure.String
		}
		if uploadedAt.Valid {
			segment.UploadedAt = uploadedAt.Time.UTC().Format(time.RFC3339)
		}
		segment.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		result = append(result, segment)
	}
	return result, rows.Err()
}

func (s *LiandongRestockService) updateLiandongSegmentStatus(ctx context.Context, batchID string, segmentNo int, status string, runErr error, acknowledged bool) error {
	if status == "" {
		return errors.New("Liandong segment status is required")
	}
	errorText := ""
	if runErr != nil {
		errorText = runErr.Error()
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		batch, ok := s.memoryBatches[batchID]
		if !ok {
			return nil
		}
		for i := range batch.Segments {
			if batch.Segments[i].SegmentNo != segmentNo {
				continue
			}
			batch.Segments[i].Status = status
			batch.Segments[i].Error = errorText
			batch.Segments[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if acknowledged {
				batch.Segments[i].UploadedAt = batch.Segments[i].UpdatedAt
			}
			return nil
		}
		return errors.New("Liandong segment not found")
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE liandong_restock_segments
		SET status = $3, error = $4, remote_acknowledged = $5,
		    uploaded_at = CASE WHEN $5 THEN NOW() ELSE uploaded_at END, updated_at = NOW()
		WHERE batch_id = $1 AND segment_no = $2`, batchID, segmentNo, status, nullableLiandongError(errorText), acknowledged)
	return err
}

func (s *LiandongRestockService) markLiandongSegmentCodesCreated(ctx context.Context, batchID string, segmentNo int) error {
	return s.updateLiandongSegmentStatus(ctx, batchID, segmentNo, liandongSegmentStatusCodesCreated, nil, false)
}

func (s *LiandongRestockService) markLiandongSegmentUploaded(ctx context.Context, batchID string, segmentNo int) error {
	return s.updateLiandongSegmentStatus(ctx, batchID, segmentNo, liandongSegmentStatusUploaded, nil, true)
}

func (s *LiandongRestockService) markLiandongSegmentFailed(ctx context.Context, batchID string, segmentNo int, runErr error) error {
	return s.updateLiandongSegmentStatus(ctx, batchID, segmentNo, liandongSegmentStatusFailed, runErr, false)
}

func (s *LiandongRestockService) markLiandongSegmentNeedsReconciliation(ctx context.Context, batchID string, segmentNo int, runErr error) error {
	return s.updateLiandongSegmentStatus(ctx, batchID, segmentNo, liandongSegmentStatusNeedsReconciliation, runErr, false)
}

func liandongJobProductsTotal(items []LiandongRestockPreviewItem) int {
	total := 0
	for _, item := range items {
		total += item.Planned
	}
	return total
}

func cloneLiandongJobSummary(in *LiandongRestockJobSummary) *LiandongRestockJobSummary {
	if in == nil {
		return nil
	}
	out := *in
	out.SelectedGoods = append([]int64(nil), in.SelectedGoods...)
	out.Products = append([]LiandongRestockPreviewItem(nil), in.Products...)
	for i := range in.Products {
		out.Products[i].CurrentStock = nil
		if in.Products[i].CurrentStock != nil {
			value := *in.Products[i].CurrentStock
			out.Products[i].CurrentStock = &value
		}
	}
	out.Batches = append([]LiandongRestockBatchStatus(nil), in.Batches...)
	return &out
}

func liandongSelectedGoods(products []LiandongRestockProduct, selected []int64) (map[int64]struct{}, []int64, error) {
	known := make(map[int64]struct{}, len(products))
	for _, product := range products {
		known[product.GoodsID] = struct{}{}
	}
	if len(selected) == 0 {
		all := make([]int64, 0, len(products))
		result := make(map[int64]struct{}, len(products))
		for _, product := range products {
			result[product.GoodsID] = struct{}{}
			all = append(all, product.GoodsID)
		}
		return result, all, nil
	}
	result := make(map[int64]struct{}, len(selected))
	ordered := make([]int64, 0, len(selected))
	for _, goodsID := range selected {
		if goodsID <= 0 {
			return nil, nil, errors.New("selected Liandong goods IDs must be positive")
		}
		if _, ok := known[goodsID]; !ok {
			return nil, nil, fmt.Errorf("Liandong goods ID %d is not configured", goodsID)
		}
		if _, duplicate := result[goodsID]; duplicate {
			return nil, nil, fmt.Errorf("Liandong goods ID %d was selected more than once", goodsID)
		}
		result[goodsID] = struct{}{}
		ordered = append(ordered, goodsID)
	}
	return result, ordered, nil
}

func (s *LiandongRestockService) Preview(ctx context.Context, selectedGoods []int64) (*LiandongRestockPreview, error) {
	if s.settingRepo == nil {
		return nil, errors.New("Liandong settings storage is unavailable")
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	state, err := s.loadState(ctx)
	if err != nil {
		return nil, err
	}
	_, ordered, err := liandongSelectedGoods(state.Products, selectedGoods)
	if err != nil {
		return nil, err
	}
	byGoods := make(map[int64]LiandongRestockProduct, len(state.Products))
	for _, product := range state.Products {
		byGoods[product.GoodsID] = product
	}
	result := &LiandongRestockPreview{Products: make([]LiandongRestockPreviewItem, 0, len(ordered)), CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, goodsID := range ordered {
		product := byGoods[goodsID]
		target := effectiveLiandongTargetStock(product)
		product.TargetStock = target
		if !product.Enabled {
			result.Products = append(result.Products, liandongPreviewItem(product, nil, 0, "disabled"))
			continue
		}
		stock, fetchErr := s.fetchUnsoldStock(ctx, goodsID)
		if fetchErr != nil {
			return nil, fetchErr
		}
		planned, planErr := liandongPlanAddition(stock, target)
		if planErr != nil {
			return nil, planErr
		}
		current := stock
		reason := "at_target"
		if planned > 0 {
			reason = "below_target"
		}
		result.Products = append(result.Products, liandongPreviewItem(product, &current, planned, reason))
	}
	return result, nil
}

func (s *LiandongRestockService) beginLiandongRun(parent context.Context) (context.Context, context.CancelFunc, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(parent)
	s.running = true
	s.runCancel = cancel
	return ctx, cancel, true
}

func (s *LiandongRestockService) finishLiandongRun(cancel context.CancelFunc) {
	cancel()
	s.mu.Lock()
	s.running = false
	s.runCancel = nil
	s.mu.Unlock()
}

func (s *LiandongRestockService) runLiandongCycle(parent context.Context, force bool, selected map[int64]struct{}, jobID string) (*liandongCycleResult, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel, acquired := s.beginLiandongRun(parent)
	if !acquired {
		if jobID != "" {
			return nil, ErrLiandongRunBusy
		}
		return &liandongCycleResult{}, nil
	}
	defer s.finishLiandongRun(cancel)

	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	state, err := s.loadState(ctx)
	if err != nil {
		return nil, err
	}
	if !force && !state.Enabled {
		return &liandongCycleResult{}, nil
	}
	if !s.configured() {
		return nil, errors.New("Liandong auto restock is not fully configured")
	}
	state.Products = visibleLiandongProducts(state.Products)
	result := &liandongCycleResult{Products: make([]LiandongRestockPreviewItem, 0, len(state.Products))}

	if state.PendingBatch != nil {
		batch := state.PendingBatch
		item := liandongPreviewItem(LiandongRestockProduct{
			GoodsID: batch.GoodsID, CNYAmount: batch.CNYAmount, USDCredit: batch.USDCredit,
			GrantType: batch.GrantType, ExternalURL: batch.ExternalURL, Version: batch.Version,
			TargetStock: batch.TargetStock, Enabled: true,
		}, batch.RemoteStockBefore, batch.Count, "pending_batch")
		item.BatchID = batch.BatchID
		if err := s.fulfillPendingBatch(ctx, state); err != nil {
			result.Products = append(result.Products, item)
			return result, s.recordRunError(ctx, state, err)
		}
		result.Products = append(result.Products, item)
		result.Batches = append(result.Batches, batch.BatchID)
		state.LastRunAt = time.Now().UTC().Format(time.RFC3339)
		state.LastError = ""
		return result, s.saveState(ctx, state)
	}

	for i := range state.Products {
		product := &state.Products[i]
		if selected != nil {
			if _, ok := selected[product.GoodsID]; !ok {
				continue
			}
		}
		target := effectiveLiandongTargetStock(*product)
		product.TargetStock = target
		if !product.Enabled {
			result.Products = append(result.Products, liandongPreviewItem(*product, nil, 0, "disabled"))
			continue
		}
		stock, fetchErr := s.fetchUnsoldStock(ctx, product.GoodsID)
		if fetchErr != nil {
			product.LastError = fetchErr.Error()
			return result, s.recordRunError(ctx, state, fetchErr)
		}
		product.CurrentStock = &stock
		product.LastRunAt = time.Now().UTC().Format(time.RFC3339)
		product.LastError = ""
		planned, planErr := liandongPlanAddition(stock, target)
		if planErr != nil {
			return result, s.recordRunError(ctx, state, planErr)
		}
		item := liandongPreviewItem(*product, &stock, planned, "at_target")
		if planned == 0 {
			result.Products = append(result.Products, item)
			continue
		}
		item.Reason = "below_target"
		batchID, idErr := newLiandongBatchID()
		if idErr != nil {
			return result, s.recordRunError(ctx, state, idErr)
		}
		batch := newLiandongPendingBatch(*product, stock, planned, jobID, time.Now().UTC().Format(time.RFC3339))
		batch.BatchID = batchID
		item.BatchID = batchID
		state.PendingBatch = batch
		if saveErr := s.saveState(ctx, state); saveErr != nil {
			return result, saveErr
		}
		if fulfillErr := s.fulfillPendingBatch(ctx, state); fulfillErr != nil {
			result.Products = append(result.Products, item)
			return result, s.recordRunError(ctx, state, fulfillErr)
		}
		result.Products = append(result.Products, item)
		result.Batches = append(result.Batches, batchID)
	}
	state.LastRunAt = time.Now().UTC().Format(time.RFC3339)
	state.LastError = ""
	return result, s.saveState(ctx, state)
}

func visibleLiandongProducts(products []LiandongRestockProduct) []LiandongRestockProduct {
	out := cloneLiandongProducts(products)
	for i := range out {
		out[i].TargetStock = effectiveLiandongTargetStock(out[i])
	}
	return out
}

func (s *LiandongRestockService) StartManualJob(ctx context.Context, selectedGoods []int64) (*LiandongRestockJobSummary, error) {
	if s.settingRepo == nil {
		return nil, errors.New("Liandong settings storage is unavailable")
	}
	s.stateMu.Lock()
	state, err := s.loadState(ctx)
	s.stateMu.Unlock()
	if err != nil {
		return nil, err
	}
	_, ordered, err := liandongSelectedGoods(state.Products, selectedGoods)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	jobID, err := newLiandongBatchID()
	if err != nil {
		return nil, err
	}
	job := &LiandongRestockJobSummary{JobID: jobID, Status: LiandongRestockJobQueued, SelectedGoods: ordered, CreatedAt: now, UpdatedAt: now}
	workerContext, err := s.reserveLiandongManualJob(job.JobID)
	if err != nil {
		return nil, err
	}
	if err := s.persistLiandongJob(ctx, job); err != nil {
		s.releaseLiandongManualJob(job.JobID)
		return nil, err
	}
	s.runReservedLiandongManualJob(workerContext, cloneLiandongJobSummary(job))
	return cloneLiandongJobSummary(job), nil
}

// reserveLiandongManualJob prevents two manual entry points from scheduling
// the same inventory cycle before the durable worker has acquired its run lease.
func (s *LiandongRestockService) reserveLiandongManualJob(jobID string) (context.Context, error) {
	s.manualJobMu.Lock()
	defer s.manualJobMu.Unlock()
	if s.manualContext == nil {
		s.manualContext, s.manualCancel = context.WithCancel(context.Background())
	}
	if s.manualContext.Err() != nil {
		return nil, errors.New("Liandong restock service is stopping")
	}
	if s.manualJobs == nil {
		s.manualJobs = make(map[string]struct{})
	}
	if _, exists := s.manualJobs[jobID]; exists || len(s.manualJobs) > 0 {
		return nil, ErrLiandongRunBusy
	}
	s.manualJobs[jobID] = struct{}{}
	s.manualJobWG.Add(1)
	return s.manualContext, nil
}

func (s *LiandongRestockService) isLiandongManualJobScheduled(jobID string) bool {
	s.manualJobMu.Lock()
	defer s.manualJobMu.Unlock()
	_, exists := s.manualJobs[jobID]
	return exists
}

func (s *LiandongRestockService) releaseLiandongManualJob(jobID string) {
	s.manualJobMu.Lock()
	delete(s.manualJobs, jobID)
	s.manualJobMu.Unlock()
	s.manualJobWG.Done()
}

func (s *LiandongRestockService) runReservedLiandongManualJob(ctx context.Context, job *LiandongRestockJobSummary) {
	go func() {
		defer s.releaseLiandongManualJob(job.JobID)
		if _, err := s.executeLiandongJob(ctx, job); err != nil {
			logger.LegacyPrintf("service.liandong_restock", "[LiandongRestock] manual job %s could not persist its final state: %v", job.JobID, err)
		}
	}()
}

func (s *LiandongRestockService) waitForLiandongManualJobs() {
	s.manualJobWG.Wait()
}

func (s *LiandongRestockService) executeLiandongJob(ctx context.Context, job *LiandongRestockJobSummary) (*LiandongRestockJobSummary, error) {
	job.Status = LiandongRestockJobRunning
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.persistLiandongJob(ctx, job); err != nil {
		return nil, err
	}
	selected := make(map[int64]struct{}, len(job.SelectedGoods))
	for _, goodsID := range job.SelectedGoods {
		selected[goodsID] = struct{}{}
	}
	result, runErr := s.runLiandongCycle(ctx, true, selected, job.JobID)
	if result != nil {
		job.Products = result.Products
		job.TotalPlanned = liandongJobProductsTotal(result.Products)
		job.Batches = s.batchStatusesForJob(ctx, job.JobID, result.Batches)
		job.TotalUploaded = countUploadedBatchCodes(job.Batches)
	}
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if runErr != nil {
		job.Error = runErr.Error()
		if isLiandongNeedsReconciliation(runErr) || isLiandongOutcomeUnknown(runErr) {
			job.Status = LiandongRestockJobNeedsReconciliation
		} else {
			job.Status = LiandongRestockJobFailed
		}
	} else {
		job.Status = LiandongRestockJobCompleted
		job.CompletedAt = job.UpdatedAt
	}
	if err := s.persistLiandongJob(liandongJobPersistenceContext(ctx), job); err != nil {
		return nil, err
	}
	return cloneLiandongJobSummary(job), nil
}

func liandongJobPersistenceContext(ctx context.Context) context.Context {
	if ctx == nil || ctx.Err() != nil {
		return context.Background()
	}
	return ctx
}

func countUploadedBatchCodes(batches []LiandongRestockBatchStatus) int {
	total := 0
	for _, batch := range batches {
		if batch.Status == liandongBatchStatusUploaded {
			total += batch.CodeCount
		}
	}
	return total
}

func (s *LiandongRestockService) GetJob(ctx context.Context, id string) (*LiandongRestockJobSummary, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrLiandongJobNotFound
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		job, ok := s.memoryJobs[id]
		if !ok {
			return nil, ErrLiandongJobNotFound
		}
		return cloneLiandongJobSummary(job), nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT job_id, status, selected_goods, summary, error, created_at, updated_at, completed_at
		FROM liandong_restock_jobs WHERE job_id = $1`, id)
	job, err := scanLiandongJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLiandongJobNotFound
		}
		return nil, err
	}
	return cloneLiandongJobSummary(job), nil
}

type liandongJobScanner interface {
	Scan(...any) error
}

func scanLiandongJob(row liandongJobScanner) (*LiandongRestockJobSummary, error) {
	var job LiandongRestockJobSummary
	var selectedRaw, summaryRaw []byte
	var failure sqlNullString
	var createdAt, updatedAt time.Time
	var completedAt sqlNullTime
	if err := row.Scan(&job.JobID, &job.Status, &selectedRaw, &summaryRaw, &failure, &createdAt, &updatedAt, &completedAt); err != nil {
		return nil, err
	}
	if len(summaryRaw) > 0 && string(summaryRaw) != "{}" {
		if err := json.Unmarshal(summaryRaw, &job); err != nil {
			return nil, fmt.Errorf("decode Liandong job summary: %w", err)
		}
	} else if len(selectedRaw) > 0 {
		if err := json.Unmarshal(selectedRaw, &job.SelectedGoods); err != nil {
			return nil, fmt.Errorf("decode Liandong job selection: %w", err)
		}
	}
	if failure.Valid {
		job.Error = failure.String
	}
	job.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	job.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if completedAt.Valid {
		job.CompletedAt = completedAt.Time.UTC().Format(time.RFC3339)
	}
	return &job, nil
}

func (s *LiandongRestockService) loadLiandongJobs(ctx context.Context, limit int) ([]LiandongRestockJobSummary, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		jobs := make([]LiandongRestockJobSummary, 0, minInt(limit, len(s.memoryJobs)))
		for _, job := range s.memoryJobs {
			jobs = append(jobs, *cloneLiandongJobSummary(job))
		}
		sort.Slice(jobs, func(i, j int) bool {
			if jobs[i].UpdatedAt == jobs[j].UpdatedAt {
				return jobs[i].JobID > jobs[j].JobID
			}
			return jobs[i].UpdatedAt > jobs[j].UpdatedAt
		})
		if len(jobs) > limit {
			jobs = jobs[:limit]
		}
		return jobs, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT job_id, status, selected_goods, summary, error, created_at, updated_at, completed_at
		FROM liandong_restock_jobs ORDER BY updated_at DESC, job_id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]LiandongRestockJobSummary, 0, limit)
	for rows.Next() {
		job, scanErr := scanLiandongJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func currentLiandongJob(jobs []LiandongRestockJobSummary) *LiandongRestockJobSummary {
	for i := range jobs {
		switch jobs[i].Status {
		case LiandongRestockJobQueued, LiandongRestockJobRunning, LiandongRestockJobNeedsReconciliation:
			return cloneLiandongJobSummary(&jobs[i])
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	return cloneLiandongJobSummary(&jobs[0])
}

func (s *LiandongRestockService) ResumeJob(ctx context.Context, id string) (*LiandongRestockJobSummary, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.Status == LiandongRestockJobNeedsReconciliation {
		return job, ErrLiandongNeedsReconciliation
	}
	if job.Status == LiandongRestockJobRunning || s.isLiandongManualJobScheduled(job.JobID) {
		return job, ErrLiandongRunBusy
	}
	if job.Status != LiandongRestockJobFailed && job.Status != LiandongRestockJobQueued {
		return job, ErrLiandongJobNotResumable
	}
	workerContext, reserveErr := s.reserveLiandongManualJob(job.JobID)
	if reserveErr != nil {
		return job, reserveErr
	}
	job.Status = LiandongRestockJobQueued
	job.Error = ""
	job.CompletedAt = ""
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.persistLiandongJob(ctx, job); err != nil {
		s.releaseLiandongManualJob(job.JobID)
		return nil, err
	}
	s.runReservedLiandongManualJob(workerContext, cloneLiandongJobSummary(job))
	return cloneLiandongJobSummary(job), nil
}

func (s *LiandongRestockService) ExportJob(ctx context.Context, id string) (*LiandongRestockJobExport, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.Status == LiandongRestockJobQueued || job.Status == LiandongRestockJobRunning {
		return nil, errors.New("Liandong restock job is still running")
	}
	batches, err := s.loadJobBatchSnapshots(ctx, id)
	if err != nil {
		return nil, err
	}
	var content bytes.Buffer
	codeCount := 0
	for _, batch := range batches {
		codes, deriveErr := s.deriveCodesChecked(&batch)
		if deriveErr != nil {
			return nil, deriveErr
		}
		for _, code := range codes {
			content.WriteString(code)
			content.WriteByte('\n')
		}
		codeCount += len(codes)
	}
	return &LiandongRestockJobExport{
		Filename:    "liandong-restock-" + id + ".txt",
		ContentType: "text/plain; charset=utf-8",
		CodeCount:   codeCount,
		Reader:      io.NopCloser(bytes.NewReader(content.Bytes())),
	}, nil
}

// sqlNullString and sqlNullTime keep the job reader independent of the
// generated Ent models while still accepting nullable PostgreSQL columns.
type sqlNullString struct {
	String string
	Valid  bool
}

func (v *sqlNullString) Scan(value any) error {
	if value == nil {
		v.String = ""
		v.Valid = false
		return nil
	}
	switch typed := value.(type) {
	case string:
		v.String, v.Valid = typed, true
	case []byte:
		v.String, v.Valid = string(typed), true
	default:
		return fmt.Errorf("cannot scan nullable string from %T", value)
	}
	return nil
}

type sqlNullTime struct {
	Time  time.Time
	Valid bool
}

func (v *sqlNullTime) Scan(value any) error {
	if value == nil {
		v.Time = time.Time{}
		v.Valid = false
		return nil
	}
	typed, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("cannot scan nullable time from %T", value)
	}
	v.Time, v.Valid = typed, true
	return nil
}

func (s *LiandongRestockService) persistLiandongJob(ctx context.Context, job *LiandongRestockJobSummary) error {
	if job == nil {
		return errors.New("Liandong job is required")
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		if s.memoryJobs == nil {
			s.memoryJobs = make(map[string]*LiandongRestockJobSummary)
		}
		s.memoryJobs[job.JobID] = cloneLiandongJobSummary(job)
		return nil
	}
	selectedRaw, err := json.Marshal(job.SelectedGoods)
	if err != nil {
		return err
	}
	summaryRaw, err := json.Marshal(job)
	if err != nil {
		return err
	}
	completedAt := any(nil)
	if job.CompletedAt != "" {
		completedAt = job.CompletedAt
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO liandong_restock_jobs
		(job_id, status, selected_goods, summary, error, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, $5, $6, $7, $8)
		ON CONFLICT (job_id) DO UPDATE SET
		status = EXCLUDED.status, selected_goods = EXCLUDED.selected_goods,
		summary = EXCLUDED.summary, error = EXCLUDED.error,
		updated_at = EXCLUDED.updated_at, completed_at = EXCLUDED.completed_at`,
		job.JobID, job.Status, string(selectedRaw), string(summaryRaw), nullableLiandongError(job.Error), job.CreatedAt, job.UpdatedAt, completedAt)
	return err
}

func nullableLiandongError(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableLiandongString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *LiandongRestockService) batchStatusesForJob(ctx context.Context, jobID string, batchIDs []string) []LiandongRestockBatchStatus {
	statuses, err := s.loadBatchStatuses(ctx, 100)
	if err != nil {
		return nil
	}
	if len(batchIDs) == 0 {
		return nil
	}
	selected := make(map[string]struct{}, len(batchIDs))
	for _, batchID := range batchIDs {
		selected[batchID] = struct{}{}
	}
	result := make([]LiandongRestockBatchStatus, 0, len(batchIDs))
	for _, status := range statuses {
		if status.JobID == jobID {
			result = append(result, status)
			continue
		}
		if _, ok := selected[status.BatchID]; ok {
			result = append(result, status)
		}
	}
	return result
}

func (s *LiandongRestockService) loadJobBatchSnapshots(ctx context.Context, jobID string) ([]liandongRestockPendingBatch, error) {
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		result := make([]liandongRestockPendingBatch, 0)
		for _, batch := range s.memoryBatches {
			if batch.Batch.JobID == jobID {
				copy := batch.Batch
				result = append(result, copy)
			}
		}
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT batch_id, job_id, goods_id, cny_amount::text, grant_value,
		       grant_type, external_url, mapping_version, mapping_key,
		       target_stock, code_count, created_at, remote_stock_before
		FROM liandong_restock_batches WHERE job_id = $1 ORDER BY created_at, batch_id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]liandongRestockPendingBatch, 0)
	for rows.Next() {
		var batch liandongRestockPendingBatch
		var cnyText string
		var createdAt time.Time
		var remoteBefore sqlNullInt64
		if err := rows.Scan(&batch.BatchID, &batch.JobID, &batch.GoodsID, &cnyText, &batch.USDCredit,
			&batch.GrantType, &batch.ExternalURL, &batch.Version, &batch.MappingKey,
			&batch.TargetStock, &batch.Count, &createdAt, &remoteBefore); err != nil {
			return nil, err
		}
		cnyValue, parseErr := strconv.ParseFloat(cnyText, 64)
		if parseErr != nil || cnyValue != math.Trunc(cnyValue) {
			return nil, fmt.Errorf("decode Liandong batch amount: %q", cnyText)
		}
		batch.CNYAmount = int(cnyValue)
		batch.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if remoteBefore.Valid {
			value := int(remoteBefore.Int64)
			batch.RemoteStockBefore = &value
		}
		result = append(result, batch)
	}
	return result, rows.Err()
}

type sqlNullInt64 struct {
	Int64 int64
	Valid bool
}

func (v *sqlNullInt64) Scan(value any) error {
	if value == nil {
		v.Int64, v.Valid = 0, false
		return nil
	}
	switch typed := value.(type) {
	case int64:
		v.Int64, v.Valid = typed, true
	case int:
		v.Int64, v.Valid = int64(typed), true
	case []byte:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return err
		}
		v.Int64, v.Valid = parsed, true
	default:
		return fmt.Errorf("cannot scan nullable int64 from %T", value)
	}
	return nil
}
