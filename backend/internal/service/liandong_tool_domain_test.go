package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiandongToolkitListGoodsUsesNonProxyQueryAndFiltersNonCardGoods(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/merchantApi/Goods/list", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		require.Equal(t, "token", r.Header.Get("Merchant-Token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"data":{"total":3,"pageSize":1000,"records":[{"id":42,"name":"Balance","type":"card","current_stock":7},{"id":42,"name":"Duplicate","type":"card","current_stock":8},{"id":99,"name":"Membership","type":"subscription","current_stock":11}]}}`))
	}))
	defer server.Close()

	svc := &LiandongRestockService{
		baseURL:    server.URL,
		token:      "token",
		httpClient: server.Client(),
	}
	result, err := svc.ListGoods(context.Background())
	require.NoError(t, err)
	require.Equal(t, float64(0), received["is_proxy"])
	require.Len(t, result.Goods, 1)
	require.Equal(t, int64(42), result.Goods[0].GoodsID)
	require.Equal(t, "Balance", result.Goods[0].Name)
	require.Equal(t, 7, result.Goods[0].CurrentStock)
}

func TestLiandongToolkitTestConfigurationIsReadOnlyAndDoesNotExposeFailure(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"merchant-token-secret"}`))
	}))
	defer server.Close()

	svc := &LiandongRestockService{
		baseURL:    server.URL,
		token:      "merchant-token-secret",
		httpClient: server.Client(),
	}
	result, err := svc.TestConfiguration(context.Background())
	require.NoError(t, err)
	require.True(t, result.Configured)
	require.False(t, result.Reachable)
	require.True(t, result.ReadOnly)
	require.NotContains(t, result.Message, "merchant-token-secret")
	require.Equal(t, float64(0), received["is_proxy"])
}

func TestLiandongToolkitListGoodsCountsUnfilteredRowsAcrossPages(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Current int `json:"current"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch request.Current {
		case 1:
			_, _ = w.Write([]byte(`{"code":1,"data":{"total":3,"pageSize":2,"records":[{"id":42,"name":"Balance","type":"card","current_stock":7},{"id":99,"name":"Membership","type":"subscription","current_stock":11}]}}`))
		case 2:
			_, _ = w.Write([]byte(`{"code":1,"data":{"total":3,"pageSize":2,"records":[{"id":99,"name":"Membership","type":"subscription","current_stock":11}]}}`))
		default:
			_, _ = w.Write([]byte(`{"code":1,"data":{"total":3,"pageSize":2,"records":[]}}`))
		}
	}))
	defer server.Close()

	svc := &LiandongRestockService{
		baseURL:    server.URL,
		token:      "token",
		httpClient: server.Client(),
	}
	result, err := svc.ListGoods(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, requests)
	require.Len(t, result.Goods, 1)
	require.Equal(t, int64(42), result.Goods[0].GoodsID)
}
