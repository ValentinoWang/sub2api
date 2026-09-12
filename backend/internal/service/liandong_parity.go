package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// CommerceParity exposes only the effective business configuration, never credentials or codes.
func (s *LiandongRestockService) CommerceParity(ctx context.Context) (map[string]any, error) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	state, err := s.loadState(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.settingRepo.GetMultiple(ctx, []string{SettingBalanceRechargeMult, SettingKeyPurchaseSubscriptionEnabled, SettingKeyPurchaseSubscriptionURL})
	if err != nil {
		return nil, err
	}
	multiplier := normalizeBalanceRechargeMultiplier(pcParseFloat(settings[SettingBalanceRechargeMult], defaultBalanceRechargeMultiplier))
	purchaseURL, urlErr := url.Parse(strings.TrimSpace(settings[SettingKeyPurchaseSubscriptionURL]))
	purchaseURLConfigured := urlErr == nil && purchaseURL.Scheme == "https" && purchaseURL.Hostname() != "" && purchaseURL.User == nil
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	digest := ""
	if len(s.codeSecret) >= 32 {
		sum := sha256.Sum256(s.codeSecret)
		digest = hex.EncodeToString(sum[:])
	}
	products := make([]map[string]any, 0, len(state.Products))
	for _, product := range state.Products {
		products = append(products, map[string]any{
			// The durable mapping includes goods ID; parity compares the denomination across independent shops.
			"mapping_key": fmt.Sprintf("balance-cny-%d", product.CNYAmount),
			"version":     product.Version, "goods_id": product.GoodsID,
			"cny_amount": product.CNYAmount, "usd_credit": strconv.FormatFloat(product.USDCredit, 'f', -1, 64),
			"grant_type": product.GrantType, "target_stock": product.TargetStock,
			"threshold": product.Threshold, "restock_count": product.RestockCount, "enabled": product.Enabled,
		})
	}
	// A build without the durable refund guard must never advertise the unused-only policy.
	if s.db == nil {
		return nil, errors.New("commerce parity requires persistent storage")
	}
	var databaseIdentity string
	if err := s.db.QueryRowContext(ctx, "SELECT json_build_array(system_identifier::text,current_database())::text FROM pg_control_system()").Scan(&databaseIdentity); err != nil {
		return nil, err
	}
	databaseDigest := sha256.Sum256([]byte(strings.TrimSpace(databaseIdentity)))
	var refundGuard bool
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = '239_liandong_unused_code_refunds.sql')").Scan(&refundGuard); err != nil {
		return nil, err
	}
	refundPolicy := "unavailable"
	if refundGuard {
		refundPolicy = "unused_only"
	}
	return map[string]any{
		"database_identity":      hex.EncodeToString(databaseDigest[:]),
		"merchant_configured":    strings.TrimSpace(s.token) != "",
		"code_secret_configured": len(s.codeSecret) >= 32, "code_secret_digest": digest,
		"purchase_enabled":        settings[SettingKeyPurchaseSubscriptionEnabled] == "true",
		"purchase_url_configured": purchaseURLConfigured,
		"restock_enabled":         state.Enabled,
		"reconciliation_required": state.ReconciliationRequired || state.PendingBatch != nil || state.LastError != "",
		"products":                products,
		"business_rules": map[string]any{
			"credit_currency": "USD", "goods_currency": "CNY", "goods_to_credit_ratio": "1",
			"native_balance_multiplier": strconv.FormatFloat(multiplier, 'f', -1, 64),
			"refund_policy":             refundPolicy, "restock_interval_seconds": int(s.interval.Seconds()),
		},
	}, nil
}
