package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiandongSessionAuthenticationFailurePersistsPause(t *testing.T) {
	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		for _, stage := range []string{"inventory", "upload", "after_upload"} {
			t.Run(fmt.Sprintf("%d_%s", statusCode, stage), func(t *testing.T) {
				requests, uploads := 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					if r.URL.Path == "/merchantApi/GoodsCardStorage/add" {
						uploads++
						if stage == "upload" {
							w.WriteHeader(statusCode)
							return
						}
					} else if stage == "inventory" || stage == "after_upload" && uploads > 0 {
						w.WriteHeader(statusCode)
						return
					}
					_, _ = w.Write([]byte(`{"code":1,"data":{"total":0}}`))
				}))
				defer server.Close()
				svc, settings, _ := newLiandongTestService(server.URL)
				ctx := context.Background()
				require.NoError(t, svc.saveState(ctx, &LiandongRestockState{Enabled: true, Products: svc.products}))
				err := svc.RunOnce(ctx, false)
				require.ErrorIs(t, err, ErrLiandongSessionVerificationRequired)
				if stage != "inventory" {
					require.ErrorIs(t, err, ErrLiandongNeedsReconciliation)
				}
				restarted, _, _ := newLiandongTestService(server.URL)
				restarted.settingRepo = settings
				state, err := restarted.loadState(ctx)
				require.NoError(t, err)
				require.False(t, state.Enabled)
				require.True(t, state.SessionVerificationRequired)
				require.Equal(t, stage != "inventory", state.ReconciliationRequired)
				status, err := restarted.Status(ctx)
				require.NoError(t, err)
				require.True(t, status.SessionVerificationRequired)
				require.Contains(t, status.LastError, "重新登录")
				before := requests
				require.NoError(t, restarted.RunOnce(ctx, false))
				require.ErrorIs(t, restarted.RunOnce(ctx, true), ErrLiandongSessionVerificationRequired)
				_, err = restarted.SetEnabled(ctx, true)
				require.ErrorIs(t, err, ErrLiandongSessionVerificationRequired)
				_, err = restarted.StartManualJob(ctx, nil)
				require.ErrorIs(t, err, ErrLiandongSessionVerificationRequired)
				restarted.memoryJobs = map[string]*LiandongRestockJobSummary{"existing": {JobID: "existing", Status: LiandongRestockJobFailed}}
				_, err = restarted.ResumeJob(ctx, "existing")
				require.ErrorIs(t, err, ErrLiandongSessionVerificationRequired)
				require.Equal(t, before, requests)
			})
		}
	}
}

func TestLiandongSessionInventoryFailurePreservesConcurrentBatchAndStopsRequests(t *testing.T) {
	ctx := context.Background()
	var svc *LiandongRestockService
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// Simulate an inventory cycle updating state after the report snapshot.
		svc.stateMu.Lock()
		state, err := svc.loadState(ctx)
		if err == nil {
			state.PendingBatch = &liandongRestockPendingBatch{BatchID: "concurrent-batch", GoodsID: 42, CNYAmount: 20}
			state.ReconciliationRequired = true
			err = svc.saveState(ctx, state)
		}
		svc.stateMu.Unlock()
		require.NoError(t, err)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	svc, _, _ = newLiandongTestService(server.URL)
	svc.products = append(svc.products, LiandongRestockProduct{CNYAmount: 30, USDCredit: 4, GoodsID: 43, RestockCount: 3, Enabled: true})
	require.NoError(t, svc.saveState(ctx, &LiandongRestockState{Enabled: true, Products: svc.products}))
	_, err := svc.Inventory(ctx)
	require.ErrorIs(t, err, ErrLiandongSessionVerificationRequired)
	require.Equal(t, 1, requests)
	state, err := svc.loadState(ctx)
	require.NoError(t, err)
	require.False(t, state.Enabled)
	require.True(t, state.SessionVerificationRequired)
	require.True(t, state.ReconciliationRequired)
	require.Equal(t, "concurrent-batch", state.PendingBatch.BatchID)
}

func TestLiandongSessionUnknownFailuresDoNotImplyAuthenticationFailure(t *testing.T) {
	for _, body := range []string{`{"code":401,"msg":"login expired"}`, `{"code":0,"msg":"token invalid"}`, `{"code":1,"data":{}}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			svc, _, redeem := newLiandongTestService(server.URL)
			ctx := context.Background()
			require.NoError(t, svc.saveState(ctx, &LiandongRestockState{Enabled: true, Products: svc.products}))
			require.Error(t, svc.RunOnce(ctx, false))
			state, err := svc.loadState(ctx)
			require.NoError(t, err)
			require.False(t, state.SessionVerificationRequired)
			require.True(t, state.Enabled)
			require.Empty(t, redeem.codes)
		})
	}
}

func TestLiandongSessionCredentialRecoveryRequiresInventoryVerification(t *testing.T) {
	stockKnown := false
	uploads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case liandongToolkitGoodsPath:
			_, _ = w.Write([]byte(`{"code":1,"data":{"records":[]}}`))
		case "/merchantApi/goodsCardStorage/list":
			if stockKnown {
				_, _ = w.Write([]byte(`{"code":1,"data":{"total":7}}`))
			} else {
				_, _ = w.Write([]byte(`{"code":1,"data":{}}`))
			}
		default:
			uploads++
		}
	}))
	defer server.Close()
	svc, _, _ := newLiandongTestService(server.URL)
	svc.encryptor = liandongTestEncryptor{}
	svc.products[0].TargetStock = 3
	ctx := context.Background()
	require.NoError(t, svc.saveState(ctx, &LiandongRestockState{Products: svc.products, SessionVerificationRequired: true, ReconciliationRequired: true, PendingBatch: &liandongRestockPendingBatch{BatchID: "existing", GoodsID: 42, CNYAmount: 20}}))
	status, err := svc.UpdateConfiguration(ctx, LiandongRestockConfigurationUpdate{MerchantToken: "replacement-test-token", Products: svc.products})
	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.True(t, status.SessionVerificationRequired)
	result, err := svc.TestConfiguration(ctx)
	require.NoError(t, err)
	require.False(t, result.Reachable)
	state, err := svc.loadState(ctx)
	require.NoError(t, err)
	require.True(t, state.SessionVerificationRequired)
	stockKnown = true
	result, err = svc.TestConfiguration(ctx)
	require.NoError(t, err)
	require.True(t, result.Reachable)
	state, err = svc.loadState(ctx)
	require.NoError(t, err)
	require.False(t, state.Enabled)
	require.False(t, state.SessionVerificationRequired)
	require.True(t, state.ReconciliationRequired)
	require.NotNil(t, state.PendingBatch)
	require.Equal(t, 7, *state.Products[0].CurrentStock)
	_, err = svc.SetEnabled(ctx, true)
	require.ErrorIs(t, err, ErrLiandongNeedsReconciliation)
	require.Zero(t, uploads)
}

func TestLiandongSessionReadOnlyProbesPauseOnAuthenticationFailure(t *testing.T) {
	for _, probe := range []string{"connection", "goods", "preview"} {
		t.Run(probe, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
			defer server.Close()
			svc, _, _ := newLiandongTestService(server.URL)
			ctx := context.Background()
			require.NoError(t, svc.saveState(ctx, &LiandongRestockState{Enabled: true, Products: svc.products}))
			switch probe {
			case "connection":
				result, err := svc.TestConfiguration(ctx)
				require.NoError(t, err)
				require.False(t, result.Reachable)
				require.Contains(t, result.Message, "重新登录")
			case "goods":
				_, err := svc.ListGoods(ctx)
				require.ErrorIs(t, err, ErrLiandongSessionVerificationRequired)
			case "preview":
				_, err := svc.Preview(ctx, nil)
				require.ErrorIs(t, err, ErrLiandongSessionVerificationRequired)
			}
			state, err := svc.loadState(ctx)
			require.NoError(t, err)
			require.True(t, state.SessionVerificationRequired)
			require.False(t, state.Enabled)
		})
	}
}
