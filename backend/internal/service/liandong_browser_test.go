package service

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestLiandongBrowserRejectsUnsafeConfiguration(t *testing.T) {
	valid := LiandongBrowserProduct{GoodsID: 42, CNYAmount: 5, USDCredit: 5, ExternalURL: "https://wzyp.cn/item/test", TargetStock: 100, BatchSize: 20, Enabled: true}
	tests := []struct {
		name   string
		mutate func(*LiandongBrowserProduct)
	}{
		{"foreign host", func(p *LiandongBrowserProduct) { p.ExternalURL = "https://wzyp.cn.evil.invalid/item/test" }},
		{"credentials", func(p *LiandongBrowserProduct) { p.ExternalURL = "https://user:secret@wzyp.cn/item/test" }},
		{"non item", func(p *LiandongBrowserProduct) { p.ExternalURL = "https://wzyp.cn/elsewhere" }},
		{"wrong grant", func(p *LiandongBrowserProduct) { p.USDCredit = 6 }},
		{"target overflow", func(p *LiandongBrowserProduct) { p.TargetStock = 101 }},
		{"batch overflow", func(p *LiandongBrowserProduct) { p.BatchSize = 21 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.mutate(&p)
			require.Error(t, validateBrowserProducts([]LiandongBrowserProduct{p}))
		})
	}
	products := []LiandongBrowserProduct{valid}
	require.NoError(t, validateBrowserProducts(products))
	require.Equal(t, "5元额度", products[0].Title)
	require.Error(t, validateBrowserProducts([]LiandongBrowserProduct{valid, valid}))
}
func TestLiandongBrowserRejectsIncompleteOrForgedInventoryShape(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, r := range []LiandongBrowserInventoryReport{
		{GoodsID: 42, Complete: false, Total: 1, Hashes: []string{hash}},
		{GoodsID: 42, Complete: true, Total: 2, Hashes: []string{hash}},
		{GoodsID: 42, Complete: true, Total: 2, Hashes: []string{hash, hash}},
		{GoodsID: 42, Complete: true, Total: 1, Hashes: []string{strings.ToUpper(hash)}},
		{GoodsID: 42, Complete: true, Total: 1, Hashes: []string{"123"}},
	} {
		require.Error(t, validateBrowserReport(r))
	}
	require.NoError(t, validateBrowserReport(LiandongBrowserInventoryReport{GoodsID: 42, Complete: true}))
}
