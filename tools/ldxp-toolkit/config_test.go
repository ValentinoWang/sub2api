package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAcceptsFlatNamesAndAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	data := []byte(`{"ldxp_base_url":"http://ldxp.test","merchant_token":"merchant-secret","sub2api_base_url":"http://sub2api.test","sub2api_admin_token":"admin-secret","data_directory":"data","product_mappings":[{"goods_id":7,"cny_amount":20,"usd_credit":2.78,"enabled":true}]}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TargetStock != defaultTargetStock || cfg.UploadSegment != defaultUploadSegment {
		t.Fatalf("defaults were not applied: target=%d segment=%d", cfg.TargetStock, cfg.UploadSegment)
	}
	if cfg.LDXP.MerchantToken != "merchant-secret" || cfg.Sub2API.AdminToken != "admin-secret" {
		t.Fatal("flat credential aliases were not loaded")
	}
	if cfg.DataDir != filepath.Join(dir, "data") {
		t.Fatalf("relative data directory was not resolved beside config: %s", cfg.DataDir)
	}
	if err := validateConfig(cfg, true, true); err != nil {
		t.Fatalf("loaded config should validate: %v", err)
	}
}
