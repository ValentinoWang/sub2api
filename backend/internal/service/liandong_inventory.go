package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type LiandongLocalInventory struct {
	Batches        int `json:"batches"`
	AllocatedCodes int `json:"allocated_codes"`
	CreatedCodes   int `json:"created_codes"`
	UnusedCodes    int `json:"unused_codes"`
	UsedCodes      int `json:"used_codes"`
	DisabledCodes  int `json:"disabled_codes"`
	OtherCodes     int `json:"other_codes"`
	MissingCodes   int `json:"missing_codes"`
}

type LiandongInventoryRow struct {
	GoodsID          int64                   `json:"goods_id"`
	Local            *LiandongLocalInventory `json:"local"`
	MerchantUnsold   *int                    `json:"merchant_unsold"`
	QuantityDelta    *int                    `json:"quantity_delta"`
	IdentityVerified bool                    `json:"identity_verified"`
	Comparison       string                  `json:"comparison"`
	LocalError       string                  `json:"local_error,omitempty"`
	MerchantError    string                  `json:"merchant_error,omitempty"`
	ObservedAt       string                  `json:"observed_at"`
}

type LiandongInventoryReport struct {
	Rows                   []LiandongInventoryRow `json:"rows"`
	ReconciliationRequired bool                   `json:"reconciliation_required"`
	ObservedAt             string                 `json:"observed_at"`
}

// Inventory compares quantity observations only. The verified merchant contract
// provides an unsold total, not card identities or sold-but-unredeemed orders.
func (s *LiandongRestockService) Inventory(ctx context.Context) (*LiandongInventoryReport, error) {
	s.stateMu.Lock()
	state, err := s.loadState(ctx)
	s.stateMu.Unlock()
	if err != nil {
		return nil, err
	}
	report := &LiandongInventoryReport{
		Rows:                   make([]LiandongInventoryRow, 0, len(state.Products)),
		ReconciliationRequired: state.ReconciliationRequired,
		ObservedAt:             time.Now().UTC().Format(time.RFC3339),
	}
	for _, product := range state.Products {
		row := LiandongInventoryRow{GoodsID: product.GoodsID, Comparison: "unknown"}
		row.Local, err = s.localLiandongInventory(ctx, product.GoodsID)
		if err != nil {
			row.Local = nil
			row.LocalError = "local_inventory_unavailable"
		}
		if s.configured() {
			stock, fetchErr := s.fetchUnsoldStock(ctx, product.GoodsID)
			if fetchErr != nil {
				if errors.Is(fetchErr, ErrLiandongSessionVerificationRequired) {
					return nil, s.recordLiandongSessionFailure(fetchErr)
				}
				row.MerchantError = "merchant_inventory_unavailable"
			} else {
				row.MerchantUnsold = &stock
			}
		} else {
			row.MerchantError = "merchant_not_configured"
		}
		row.ObservedAt = time.Now().UTC().Format(time.RFC3339)
		if row.Local != nil && row.MerchantUnsold != nil {
			delta := row.Local.UnusedCodes - *row.MerchantUnsold
			row.QuantityDelta = &delta
			row.Comparison = "quantity_match_identity_unknown"
			if delta != 0 || row.Local.MissingCodes != 0 {
				row.Comparison = "quantity_difference_identity_unknown"
			}
		}
		report.Rows = append(report.Rows, row)
	}
	return report, nil
}

const liandongLocalInventorySQL = `
	SELECT COUNT(DISTINCT b.batch_id), COUNT(c.code_sha256), COUNT(r.id),
	       COUNT(r.id) FILTER (WHERE r.status = 'unused' AND r.used_at IS NULL AND r.used_by IS NULL AND (r.expires_at IS NULL OR r.expires_at > NOW())),
	       COUNT(r.id) FILTER (WHERE r.status = 'used' OR r.used_at IS NOT NULL OR r.used_by IS NOT NULL),
	       COUNT(r.id) FILTER (WHERE r.status = 'disabled' AND r.used_at IS NULL AND r.used_by IS NULL)
	FROM liandong_restock_batches b
	LEFT JOIN liandong_restock_batch_codes c ON c.batch_id = b.batch_id
	LEFT JOIN redeem_codes r ON encode(sha256(convert_to(r.code, 'UTF8')), 'hex') = c.code_sha256
	WHERE b.goods_id = $1`

func (s *LiandongRestockService) localLiandongInventory(ctx context.Context, goodsID int64) (*LiandongLocalInventory, error) {
	result := &LiandongLocalInventory{}
	if s.db != nil {
		err := s.db.QueryRowContext(ctx, liandongLocalInventorySQL, goodsID).Scan(
			&result.Batches, &result.AllocatedCodes, &result.CreatedCodes,
			&result.UnusedCodes, &result.UsedCodes, &result.DisabledCodes)
		if err != nil {
			return nil, err
		}
	} else {
		s.memoryMu.Lock()
		codes := []string{}
		for _, batch := range s.memoryBatches {
			if batch.Batch.GoodsID == goodsID {
				result.Batches++
				codes = append(codes, batch.Codes...)
			}
		}
		s.memoryMu.Unlock()
		result.AllocatedCodes = len(codes)
		if len(codes) > 0 && s.redeem == nil {
			return nil, errors.New("local redeem storage unavailable")
		}
		for _, code := range codes {
			record, err := s.redeem.GetByCode(ctx, code)
			if errors.Is(err, ErrRedeemCodeNotFound) {
				continue
			}
			if err != nil || record == nil {
				return nil, errors.New("local redeem inventory unavailable")
			}
			result.CreatedCodes++
			switch {
			case record.Status == StatusUsed || record.UsedAt != nil || record.UsedBy != nil:
				result.UsedCodes++
			case record.Status == "disabled":
				result.DisabledCodes++
			case record.Status == StatusUnused && !record.IsExpired():
				result.UnusedCodes++
			}
		}
	}
	result.MissingCodes = result.AllocatedCodes - result.CreatedCodes
	result.OtherCodes = result.CreatedCodes - result.UnusedCodes - result.UsedCodes - result.DisabledCodes
	if result.MissingCodes < 0 || result.OtherCodes < 0 {
		return nil, errors.New("local inventory accounting is inconsistent")
	}
	return result, nil
}

func (s *LiandongRestockService) latchLiandongStockDiscrepancy(state *LiandongRestockState, observed *int) error {
	batch := state.PendingBatch
	state.ReconciliationRequired = true
	runErr := fmt.Errorf("%w: post-upload inventory is missing or differs from the batch baseline", ErrLiandongNeedsReconciliation)
	ctx, cancel := liandongRecoveryContext()
	defer cancel()
	var observeErr error
	if s.db == nil {
		s.memoryMu.Lock()
		if saved, ok := s.memoryBatches[batch.BatchID]; ok {
			saved.RemoteAfter = observed
		}
		s.memoryMu.Unlock()
	} else {
		_, observeErr = s.db.ExecContext(ctx, `UPDATE liandong_restock_batches SET remote_stock_after = $2, updated_at = NOW() WHERE batch_id = $1`, batch.BatchID, observed)
	}
	// Successful segment acknowledgements remain intact; quantity evidence cannot
	// invalidate or replace those acknowledgements, nor clear this batch latch.
	markerErr := s.markBatchNeedsReconciliation(ctx, batch.BatchID, runErr)
	stateErr := s.saveState(ctx, state)
	if observeErr != nil || markerErr != nil || stateErr != nil {
		return fmt.Errorf("%w: inventory reconciliation persistence failed", runErr)
	}
	return runErr
}
