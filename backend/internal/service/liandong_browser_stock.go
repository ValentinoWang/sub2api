package service

import (
	"context"
	"encoding/json"
)

const LiandongBrowserMaxTargetStock = 999

type LiandongBrowserStockTargetRequest struct {
	GoodsIDs    []int64 `json:"goods_ids"`
	TargetStock int     `json:"target_stock"`
}

// BrowserSetStockTarget lets a device adjust only its authorized inventory target.
// Product identity, credit amounts and the administrator's master switch stay server-owned.
func (s *LiandongRestockService) BrowserSetStockTarget(ctx context.Context, id string, req LiandongBrowserStockTargetRequest) ([]LiandongBrowserProduct, error) {
	if req.TargetStock < 1 || req.TargetStock > LiandongBrowserMaxTargetStock || len(req.GoodsIDs) < 1 || len(req.GoodsIDs) > 30 {
		return nil, errBrowserInvalid
	}
	seen := make(map[int64]bool, len(req.GoodsIDs))
	for _, goodsID := range req.GoodsIDs {
		if goodsID <= 0 || seen[goodsID] {
			return nil, errBrowserInvalid
		}
		seen[goodsID] = true
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
	config, err := browserReadConfig(ctx, tx)
	if err != nil {
		return nil, err
	}
	selected := make([]LiandongBrowserProduct, 0, len(req.GoodsIDs))
	for _, goodsID := range req.GoodsIDs {
		if !browserDeviceAllows(device, goodsID) {
			return nil, errBrowserAuth
		}
		product, err := browserProduct(config, goodsID)
		if err != nil {
			return nil, err
		}
		if req.TargetStock < product.TargetStock {
			var pending bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM liandong_restock_batches WHERE goods_id=$1 AND status IN ('pending','needs_reconciliation'))`, goodsID).Scan(&pending); err != nil {
				return nil, err
			}
			if pending {
				return nil, ErrLiandongNeedsReconciliation
			}
		}
		product.TargetStock = req.TargetStock
		selected = append(selected, *product)
	}
	raw, err := json.Marshal(config.Products)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE liandong_browser_config SET products=$1::jsonb,updated_at=NOW() WHERE id=TRUE`, string(raw)); err != nil {
		return nil, err
	}
	for _, product := range selected {
		if _, err = tx.ExecContext(ctx, `UPDATE liandong_product_mappings SET target_stock=$2 WHERE goods_id=$1 AND mapping_key LIKE 'browser-%'`, product.GoodsID, product.TargetStock); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return selected, nil
}
