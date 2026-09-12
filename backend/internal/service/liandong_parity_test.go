package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type paritySettings struct{ *liandongSettingRepoStub }

func (s paritySettings) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string)
	for _, key := range keys {
		out[key] = s.values[key]
	}
	return out, nil
}

func TestLiandongParityReadsEffectivePoliciesWithoutLeakingSecrets(t *testing.T) {
	svc, settings, _ := newLiandongTestService("https://merchant.invalid")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	}()
	svc.db = db
	svc.settingRepo = paritySettings{settings}
	svc.interval = 5 * time.Minute
	svc.products = []LiandongRestockProduct{{CNYAmount: 5, USDCredit: 5, GoodsID: 123, Version: 1, GrantType: "balance", ExternalURL: "https://shop.invalid/private", TargetStock: 5, Threshold: 1, RestockCount: 5}}
	settings.values[SettingBalanceRechargeMult] = "2"
	settings.values[SettingKeyPurchaseSubscriptionEnabled] = "true"
	settings.values[SettingKeyPurchaseSubscriptionURL] = "https://shop.invalid/private"
	state, err := json.Marshal(LiandongRestockState{Enabled: true, ReconciliationRequired: true, Products: []LiandongRestockProduct{{CNYAmount: 5, TargetStock: 4, Threshold: 2, RestockCount: 3, Enabled: true}}})
	require.NoError(t, err)
	settings.values[liandongRestockStateKey] = string(state)
	mock.ExpectQuery("SELECT json_build_array").WillReturnRows(sqlmock.NewRows([]string{"identity"}).AddRow(`["test-cluster", "test-db"]`))
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	result, err := svc.CommerceParity(context.Background())
	require.NoError(t, err)
	require.Equal(t, true, result["reconciliation_required"])
	require.Equal(t, true, result["purchase_enabled"])
	require.Equal(t, true, result["purchase_url_configured"])
	products, ok := result["products"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, products, 1)
	product := products[0]
	require.Equal(t, 4, product["target_stock"])
	require.Equal(t, 2, product["threshold"])
	require.Equal(t, 3, product["restock_count"])
	rules, ok := result["business_rules"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "2", rules["native_balance_multiplier"])
	require.Equal(t, "unused_only", rules["refund_policy"])
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	for _, secret := range []string{svc.token, string(svc.codeSecret), "shop.invalid", "merchant.invalid"} {
		require.False(t, strings.Contains(string(raw), secret))
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiandongParityDoesNotClaimRefundProtectionWithoutMigration(t *testing.T) {
	svc, _, _ := newLiandongTestService("")
	_, err := svc.CommerceParity(context.Background())
	require.Error(t, err)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	}()
	svc.db = db
	mock.ExpectQuery("SELECT json_build_array").WillReturnRows(sqlmock.NewRows([]string{"identity"}).AddRow(`["test-cluster", "test-db"]`))
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	result, err := svc.CommerceParity(context.Background())
	require.NoError(t, err)
	rules, ok := result["business_rules"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "unavailable", rules["refund_policy"])
	require.Equal(t, false, result["purchase_url_configured"])
	require.NoError(t, mock.ExpectationsWereMet())
}
