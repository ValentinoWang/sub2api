package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	liandongRestockStateKey  = "liandong_auto_restock_state_v1"
	liandongRestockConfigKey = "liandong_auto_restock_config_v1"
)

type LiandongRestockProduct struct {
	CNYAmount    int     `json:"cny_amount"`
	USDCredit    float64 `json:"usd_credit"`
	GoodsID      int64   `json:"goods_id"`
	GrantType    string  `json:"grant_type,omitempty"`
	ExternalURL  string  `json:"external_url,omitempty"`
	Version      int     `json:"version,omitempty"`
	Threshold    int     `json:"threshold"`
	RestockCount int     `json:"restock_count"`
	TargetStock  int     `json:"target_stock"`
	Enabled      bool    `json:"enabled"`
	CurrentStock *int    `json:"current_stock,omitempty"`
	LastError    string  `json:"last_error,omitempty"`
	LastRunAt    string  `json:"last_run_at,omitempty"`
}

type liandongRestockPendingBatch struct {
	BatchID           string  `json:"batch_id"`
	JobID             string  `json:"job_id,omitempty"`
	GoodsID           int64   `json:"goods_id"`
	CNYAmount         int     `json:"cny_amount"`
	USDCredit         float64 `json:"usd_credit"`
	GrantType         string  `json:"grant_type"`
	ExternalURL       string  `json:"external_url,omitempty"`
	Version           int     `json:"version"`
	MappingKey        string  `json:"mapping_key"`
	TargetStock       int     `json:"target_stock"`
	Count             int     `json:"count"`
	CreatedAt         string  `json:"created_at"`
	RemoteStockBefore *int    `json:"remote_stock_before,omitempty"`
}

type LiandongRestockState struct {
	Enabled      bool                         `json:"enabled"`
	Products     []LiandongRestockProduct     `json:"products"`
	LastRunAt    string                       `json:"last_run_at,omitempty"`
	LastError    string                       `json:"last_error,omitempty"`
	PendingBatch *liandongRestockPendingBatch `json:"pending_batch,omitempty"`
}

type LiandongRestockStatus struct {
	IntegrationMode         string                       `json:"integration_mode"`
	PaymentReadiness        string                       `json:"payment_readiness"`
	Configured              bool                         `json:"configured"`
	MerchantTokenConfigured bool                         `json:"merchant_token_configured"`
	CodeSecretConfigured    bool                         `json:"code_secret_configured"`
	Enabled                 bool                         `json:"enabled"`
	Running                 bool                         `json:"running"`
	IntervalSeconds         int                          `json:"interval_seconds"`
	LastRunAt               string                       `json:"last_run_at,omitempty"`
	LastError               string                       `json:"last_error,omitempty"`
	PendingBatch            bool                         `json:"pending_batch"`
	Products                []LiandongRestockProduct     `json:"products"`
	Batches                 []LiandongRestockBatchStatus `json:"batches,omitempty"`
	CurrentJob              *LiandongRestockJobSummary   `json:"current_job,omitempty"`
	Jobs                    []LiandongRestockJobSummary  `json:"jobs,omitempty"`
}

// LiandongRestockBatchStatus is a non-secret operational summary of a restock batch.
type LiandongRestockBatchStatus struct {
	BatchID           string  `json:"batch_id"`
	JobID             string  `json:"job_id,omitempty"`
	GoodsID           int64   `json:"goods_id"`
	CNYAmount         float64 `json:"cny_amount"`
	CodeCount         int     `json:"code_count"`
	Status            string  `json:"status"`
	MappingKey        string  `json:"mapping_key,omitempty"`
	MappingVersion    int     `json:"mapping_version,omitempty"`
	TargetStock       int     `json:"target_stock,omitempty"`
	SegmentsTotal     int     `json:"segments_total,omitempty"`
	SegmentsUploaded  int     `json:"segments_uploaded,omitempty"`
	RemoteStockBefore *int    `json:"remote_stock_before,omitempty"`
	RemoteStockAfter  *int    `json:"remote_stock_after,omitempty"`
	Error             string  `json:"error,omitempty"`
	CreatedAt         string  `json:"created_at"`
	UploadedAt        string  `json:"uploaded_at,omitempty"`
	UpdatedAt         string  `json:"updated_at"`
}

type LiandongRestockConfigurationUpdate struct {
	MerchantToken      string                   `json:"merchant_token"`
	GenerateCodeSecret bool                     `json:"generate_code_secret"`
	Products           []LiandongRestockProduct `json:"products"`
}

type liandongRestockStoredConfig struct {
	MerchantToken string                   `json:"merchant_token"`
	CodeSecret    string                   `json:"code_secret"`
	Products      []LiandongRestockProduct `json:"products"`
}

type LiandongRestockPolicyUpdate struct {
	CNYAmount    int  `json:"cny_amount"`
	Threshold    int  `json:"threshold"`
	RestockCount int  `json:"restock_count"`
	TargetStock  int  `json:"target_stock"`
	Enabled      bool `json:"enabled"`
}

type liandongRedeemStore interface {
	GetByCode(context.Context, string) (*RedeemCode, error)
	CreateCode(context.Context, *RedeemCode) error
}

type LiandongRestockService struct {
	settingRepo SettingRepository
	redeem      liandongRedeemStore
	encryptor   SecretEncryptor
	db          *sql.DB
	baseURL     string
	token       string
	codeSecret  []byte
	products    []LiandongRestockProduct
	interval    time.Duration
	httpClient  *http.Client

	configMu      sync.RWMutex
	mu            sync.Mutex
	stateMu       sync.Mutex
	memoryMu      sync.Mutex
	manualJobMu   sync.Mutex
	manualJobWG   sync.WaitGroup
	running       bool
	runCancel     context.CancelFunc
	manualContext context.Context
	manualCancel  context.CancelFunc
	manualJobs    map[string]struct{}
	stop          chan struct{}
	stopOnce      sync.Once
	memoryBatches map[string]*liandongMemoryBatch
	memoryJobs    map[string]*LiandongRestockJobSummary
}

// ProvideLiandongRestockService starts the durable background worker. The
// persisted enabled flag remains authoritative across process restarts.
func ProvideLiandongRestockService(settingRepo SettingRepository, redeem *RedeemService, cfg *config.Config, encryptor SecretEncryptor, db *sql.DB) *LiandongRestockService {
	svc := NewLiandongRestockService(settingRepo, redeem, cfg, encryptor, db)
	if err := svc.loadStoredConfig(context.Background()); err != nil {
		logger.LegacyPrintf("service.liandong_restock", "[LiandongRestock] load stored configuration failed: %v", err)
	}
	svc.StartWorker()
	return svc
}

func NewLiandongRestockService(settingRepo SettingRepository, redeem *RedeemService, cfg *config.Config, encryptor SecretEncryptor, db ...*sql.DB) *LiandongRestockService {
	interval := 5 * time.Minute
	if cfg != nil && cfg.LiandongRestock.IntervalSecs >= 30 {
		interval = time.Duration(cfg.LiandongRestock.IntervalSecs) * time.Second
	}
	var sqlDB *sql.DB
	if len(db) > 0 {
		sqlDB = db[0]
	}
	manualContext, manualCancel := context.WithCancel(context.Background())
	s := &LiandongRestockService{
		settingRepo:   settingRepo,
		redeem:        redeem,
		encryptor:     encryptor,
		db:            sqlDB,
		interval:      interval,
		httpClient:    &http.Client{Timeout: 20 * time.Second},
		stop:          make(chan struct{}),
		manualContext: manualContext,
		manualCancel:  manualCancel,
		manualJobs:    make(map[string]struct{}),
	}
	if cfg != nil {
		s.baseURL = strings.TrimRight(strings.TrimSpace(cfg.LiandongRestock.BaseURL), "/")
		s.token = strings.TrimSpace(cfg.LiandongRestock.MerchantToken)
		s.codeSecret = []byte(cfg.LiandongRestock.CodeSecret)
		if err := json.Unmarshal([]byte(cfg.LiandongRestock.ProductsJSON), &s.products); err == nil {
			s.products = normalizeLiandongProducts(s.products)
		}
	}
	if s.baseURL == "" {
		s.baseURL = "https://ldxp.cn"
	}
	if parsed, err := url.Parse(s.baseURL); err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		s.baseURL = ""
	}
	for i := range s.products {
		if s.products[i].Threshold < 0 {
			s.products[i].Threshold = 0
		}
		if s.products[i].RestockCount <= 0 {
			s.products[i].RestockCount = 10
		}
	}
	return s
}

func (s *LiandongRestockService) StartWorker() {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), s.interval)
				if err := s.RunOnce(ctx, false); err != nil && !errors.Is(err, context.Canceled) {
					logger.LegacyPrintf("service.liandong_restock", "[LiandongRestock] cycle failed: %v", err)
				}
				cancel()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *LiandongRestockService) StopWorker() {
	s.stopOnce.Do(func() { close(s.stop) })
	s.mu.Lock()
	if s.runCancel != nil {
		s.runCancel()
	}
	s.mu.Unlock()
	s.manualJobMu.Lock()
	if s.manualCancel != nil {
		s.manualCancel()
	}
	s.manualJobMu.Unlock()
	s.manualJobWG.Wait()
}

func (s *LiandongRestockService) configured() bool {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.configuredLocked()
}

func (s *LiandongRestockService) configuredLocked() bool {
	if s.settingRepo == nil || s.redeem == nil || s.baseURL == "" || s.token == "" || len(s.codeSecret) < 32 || len(s.products) == 0 {
		return false
	}
	seenCNY := make(map[int]struct{}, len(s.products))
	seenGoods := make(map[int64]struct{}, len(s.products))
	for _, product := range s.products {
		if product.CNYAmount <= 0 || product.USDCredit <= 0 || product.GoodsID <= 0 || product.RestockCount <= 0 {
			return false
		}
		if product.TargetStock < 0 || product.TargetStock > liandongMaxTargetStock {
			return false
		}
		if product.GrantType != "" && product.GrantType != "balance" {
			return false
		}
		if product.Version < 0 {
			return false
		}
		if product.ExternalURL != "" {
			parsed, err := url.Parse(product.ExternalURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return false
			}
		}
		if _, ok := seenCNY[product.CNYAmount]; ok {
			return false
		}
		if _, ok := seenGoods[product.GoodsID]; ok {
			return false
		}
		seenCNY[product.CNYAmount] = struct{}{}
		seenGoods[product.GoodsID] = struct{}{}
	}
	return true
}

func (s *LiandongRestockService) Configured() bool { return s.configured() }

func (s *LiandongRestockService) loadStoredConfig(ctx context.Context) error {
	if s.settingRepo == nil || s.encryptor == nil {
		return nil
	}
	raw, err := s.settingRepo.GetValue(ctx, liandongRestockConfigKey)
	if errors.Is(err, ErrSettingNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	plaintext, err := s.encryptor.Decrypt(raw)
	if err != nil {
		return fmt.Errorf("decrypt Liandong restock configuration: %w", err)
	}
	var stored liandongRestockStoredConfig
	if err := json.Unmarshal([]byte(plaintext), &stored); err != nil {
		return fmt.Errorf("decode Liandong restock configuration: %w", err)
	}
	s.applyStoredConfig(stored)
	return nil
}

func (s *LiandongRestockService) applyStoredConfig(stored liandongRestockStoredConfig) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	s.token = strings.TrimSpace(stored.MerchantToken)
	s.codeSecret = []byte(stored.CodeSecret)
	s.products = normalizeStoredLiandongProducts(stored.Products)
}

func normalizeLiandongProducts(products []LiandongRestockProduct) []LiandongRestockProduct {
	out := cloneLiandongProducts(products)
	for i := range out {
		out[i].CurrentStock = nil
		out[i].LastError = ""
		out[i].LastRunAt = ""
		if out[i].GrantType == "" {
			out[i].GrantType = "balance"
		}
		if out[i].Version <= 0 {
			out[i].Version = 1
		}
		if out[i].Threshold < 0 {
			out[i].Threshold = 0
		}
		if out[i].RestockCount <= 0 {
			out[i].RestockCount = 10
		}
		if out[i].TargetStock <= 0 {
			out[i].TargetStock = liandongDefaultTargetStock
		}
	}
	return out
}

func (s *LiandongRestockService) loadState(ctx context.Context) (*LiandongRestockState, error) {
	if s.settingRepo == nil {
		return nil, errors.New("Liandong settings storage is unavailable")
	}
	state := &LiandongRestockState{Products: s.configuredProducts()}
	raw, err := s.settingRepo.GetValue(ctx, liandongRestockStateKey)
	if errors.Is(err, ErrSettingNotFound) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), state); err != nil {
		return nil, fmt.Errorf("decode liandong restock state: %w", err)
	}
	state.Products = s.mergePolicies(state.Products)
	state.Products = visibleLiandongProducts(state.Products)
	if state.PendingBatch != nil {
		state.PendingBatch = hydrateLiandongPendingBatch(state.PendingBatch, state.Products)
	}
	return state, nil
}

func cloneLiandongProducts(in []LiandongRestockProduct) []LiandongRestockProduct {
	out := make([]LiandongRestockProduct, len(in))
	copy(out, in)
	return out
}

func (s *LiandongRestockService) configuredProducts() []LiandongRestockProduct {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return cloneLiandongProducts(s.products)
}

func (s *LiandongRestockService) mergePolicies(saved []LiandongRestockProduct) []LiandongRestockProduct {
	byCNY := make(map[int]LiandongRestockProduct, len(saved))
	for _, product := range saved {
		byCNY[product.CNYAmount] = product
	}
	out := s.configuredProducts()
	for i := range out {
		if policy, ok := byCNY[out[i].CNYAmount]; ok {
			out[i].Enabled = policy.Enabled
			out[i].Threshold = policy.Threshold
			out[i].RestockCount = policy.RestockCount
			if policy.TargetStock > 0 {
				out[i].TargetStock = policy.TargetStock
			}
			out[i].CurrentStock = policy.CurrentStock
			out[i].LastError = policy.LastError
			out[i].LastRunAt = policy.LastRunAt
		}
	}
	return out
}

func (s *LiandongRestockService) saveState(ctx context.Context, state *LiandongRestockState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, liandongRestockStateKey, string(raw))
}

func (s *LiandongRestockService) Status(ctx context.Context) (*LiandongRestockStatus, error) {
	s.stateMu.Lock()
	state, err := s.loadState(ctx)
	s.stateMu.Unlock()
	if err != nil {
		return nil, err
	}
	state.Products = visibleLiandongProducts(state.Products)
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	s.configMu.RLock()
	merchantTokenConfigured := strings.TrimSpace(s.token) != ""
	codeSecretConfigured := len(s.codeSecret) >= 32
	intervalSeconds := int(s.interval.Seconds())
	s.configMu.RUnlock()
	batches, err := s.loadBatchStatuses(ctx, 20)
	if err != nil {
		return nil, err
	}
	jobs, err := s.loadLiandongJobs(ctx, 20)
	if err != nil {
		return nil, err
	}
	return &LiandongRestockStatus{
		IntegrationMode:         "sales_channel",
		PaymentReadiness:        "NOT_READY",
		Configured:              s.configured(),
		MerchantTokenConfigured: merchantTokenConfigured,
		CodeSecretConfigured:    codeSecretConfigured,
		Enabled:                 state.Enabled,
		Running:                 running,
		IntervalSeconds:         intervalSeconds,
		LastRunAt:               state.LastRunAt,
		LastError:               state.LastError,
		PendingBatch:            state.PendingBatch != nil,
		Products:                state.Products,
		Batches:                 batches,
		CurrentJob:              currentLiandongJob(jobs),
		Jobs:                    jobs,
	}, nil
}

func (s *LiandongRestockService) UpdateConfiguration(ctx context.Context, input LiandongRestockConfigurationUpdate) (*LiandongRestockStatus, error) {
	if s.settingRepo == nil || s.encryptor == nil {
		return nil, errors.New("encrypted Liandong configuration storage is unavailable")
	}

	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if running {
		return nil, errors.New("stop the active inventory check before changing configuration")
	}

	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	state, err := s.loadState(ctx)
	if err != nil {
		return nil, err
	}
	if state.Enabled {
		return nil, errors.New("stop auto restock before changing configuration")
	}
	if state.PendingBatch != nil {
		return nil, errors.New("resolve the pending batch before changing configuration")
	}

	s.configMu.RLock()
	token := s.token
	secret := string(s.codeSecret)
	s.configMu.RUnlock()
	if replacement := strings.TrimSpace(input.MerchantToken); replacement != "" {
		token = replacement
	}
	if input.GenerateCodeSecret || len(secret) < 32 {
		generated := make([]byte, 32)
		if _, err := rand.Read(generated); err != nil {
			return nil, fmt.Errorf("generate Liandong code secret: %w", err)
		}
		secret = hex.EncodeToString(generated)
	}
	products, err := validateLiandongConfiguration(token, secret, input.Products)
	if err != nil {
		return nil, err
	}
	if err := s.persistProductMappings(ctx, products); err != nil {
		return nil, fmt.Errorf("persist Liandong product mappings: %w", err)
	}

	stored := liandongRestockStoredConfig{
		MerchantToken: token,
		CodeSecret:    secret,
		Products:      products,
	}
	plaintext, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	ciphertext, err := s.encryptor.Encrypt(string(plaintext))
	if err != nil {
		return nil, fmt.Errorf("encrypt Liandong restock configuration: %w", err)
	}
	if err := s.settingRepo.Set(ctx, liandongRestockConfigKey, ciphertext); err != nil {
		return nil, err
	}
	s.applyStoredConfig(stored)
	state.Products = s.mergePolicies(state.Products)
	if err := s.saveState(ctx, state); err != nil {
		return nil, err
	}
	return s.statusWithState(state)
}

func validateLiandongConfiguration(token, secret string, products []LiandongRestockProduct) ([]LiandongRestockProduct, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("Liandong merchant token is required")
	}
	if len(secret) < 32 {
		return nil, errors.New("Liandong code secret must contain at least 32 characters")
	}
	if len(products) == 0 || len(products) > 20 {
		return nil, errors.New("configure between 1 and 20 Liandong products")
	}
	seenCNY := make(map[int]struct{}, len(products))
	seenGoods := make(map[int64]struct{}, len(products))
	normalized := normalizeLiandongProducts(products)
	for i := range normalized {
		product := &normalized[i]
		if product.CNYAmount <= 0 || product.USDCredit <= 0 || product.GoodsID <= 0 {
			return nil, errors.New("each Liandong product requires a positive CNY amount, USD credit, and numeric goods ID")
		}
		if product.GrantType == "" {
			product.GrantType = "balance"
		}
		if product.GrantType != "balance" {
			return nil, errors.New("only balance Liandong products are supported in the first release")
		}
		if product.ExternalURL != "" {
			parsed, parseErr := url.Parse(product.ExternalURL)
			if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return nil, fmt.Errorf("invalid external URL for CNY %d", product.CNYAmount)
			}
		}
		if product.Version <= 0 {
			product.Version = 1
		}
		if !liandongTargetStockIsValid(product.TargetStock) || product.Threshold < 0 || product.Threshold > 1000 || product.RestockCount < 1 || product.RestockCount > 1000 {
			return nil, fmt.Errorf("invalid inventory policy for CNY %d", product.CNYAmount)
		}
		if _, exists := seenCNY[product.CNYAmount]; exists {
			return nil, fmt.Errorf("duplicate CNY amount %d", product.CNYAmount)
		}
		if _, exists := seenGoods[product.GoodsID]; exists {
			return nil, fmt.Errorf("duplicate Liandong goods ID %d", product.GoodsID)
		}
		seenCNY[product.CNYAmount] = struct{}{}
		seenGoods[product.GoodsID] = struct{}{}
	}
	return normalized, nil
}

func (s *LiandongRestockService) statusWithState(state *LiandongRestockState) (*LiandongRestockStatus, error) {
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	s.configMu.RLock()
	status := &LiandongRestockStatus{
		IntegrationMode:         "sales_channel",
		PaymentReadiness:        "NOT_READY",
		Configured:              s.configuredLocked(),
		MerchantTokenConfigured: strings.TrimSpace(s.token) != "",
		CodeSecretConfigured:    len(s.codeSecret) >= 32,
		Enabled:                 state.Enabled,
		Running:                 running,
		IntervalSeconds:         int(s.interval.Seconds()),
		LastRunAt:               state.LastRunAt,
		LastError:               state.LastError,
		PendingBatch:            state.PendingBatch != nil,
		Products:                visibleLiandongProducts(state.Products),
	}
	s.configMu.RUnlock()
	if batches, err := s.loadBatchStatuses(context.Background(), 20); err == nil {
		status.Batches = batches
	}
	if jobs, err := s.loadLiandongJobs(context.Background(), 20); err == nil {
		status.CurrentJob = currentLiandongJob(jobs)
		status.Jobs = jobs
	}
	return status, nil
}

func (s *LiandongRestockService) UpdatePolicies(ctx context.Context, updates []LiandongRestockPolicyUpdate) (*LiandongRestockStatus, error) {
	s.stateMu.Lock()
	state, err := s.loadState(ctx)
	if err != nil {
		s.stateMu.Unlock()
		return nil, err
	}
	byCNY := make(map[int]LiandongRestockPolicyUpdate, len(updates))
	for _, update := range updates {
		if update.Threshold < 0 || update.RestockCount < 1 || update.RestockCount > 1000 || update.TargetStock < 0 || update.TargetStock > liandongMaxTargetStock {
			s.stateMu.Unlock()
			return nil, fmt.Errorf("invalid policy for CNY %d", update.CNYAmount)
		}
		byCNY[update.CNYAmount] = update
	}
	if len(updates) != len(state.Products) || len(byCNY) != len(state.Products) {
		s.stateMu.Unlock()
		return nil, errors.New("all configured Liandong products must be included exactly once")
	}
	for i := range state.Products {
		update, ok := byCNY[state.Products[i].CNYAmount]
		if !ok {
			s.stateMu.Unlock()
			return nil, fmt.Errorf("missing policy for CNY %d", state.Products[i].CNYAmount)
		}
		state.Products[i].Threshold = update.Threshold
		state.Products[i].RestockCount = update.RestockCount
		if update.TargetStock > 0 {
			state.Products[i].TargetStock = update.TargetStock
		} else {
			state.Products[i].TargetStock = effectiveLiandongTargetStock(state.Products[i])
		}
		state.Products[i].Enabled = update.Enabled
	}
	if err := s.saveState(ctx, state); err != nil {
		s.stateMu.Unlock()
		return nil, err
	}
	s.stateMu.Unlock()
	return s.Status(ctx)
}

func (s *LiandongRestockService) SetEnabled(ctx context.Context, enabled bool) (*LiandongRestockStatus, error) {
	if enabled && !s.configured() {
		return nil, errors.New("Liandong auto restock is not fully configured")
	}
	if !enabled {
		s.mu.Lock()
		if s.runCancel != nil {
			s.runCancel()
		}
		s.mu.Unlock()
	}
	s.stateMu.Lock()
	state, err := s.loadState(ctx)
	if err != nil {
		s.stateMu.Unlock()
		return nil, err
	}
	state.Enabled = enabled
	if err := s.saveState(ctx, state); err != nil {
		s.stateMu.Unlock()
		return nil, err
	}
	s.stateMu.Unlock()
	if enabled {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), s.interval)
			defer cancel()
			if err := s.RunOnce(ctx, false); err != nil && !errors.Is(err, context.Canceled) {
				logger.LegacyPrintf("service.liandong_restock", "[LiandongRestock] initial cycle failed: %v", err)
			}
		}()
	}
	return s.Status(ctx)
}

func (s *LiandongRestockService) RunOnce(parent context.Context, force bool) error {
	_, err := s.runLiandongCycle(parent, force, nil, "")
	return err
}

func (s *LiandongRestockService) recordRunError(ctx context.Context, state *LiandongRestockState, runErr error) error {
	state.LastRunAt = time.Now().UTC().Format(time.RFC3339)
	state.LastError = runErr.Error()
	if ctx.Err() == nil {
		_ = s.saveState(ctx, state)
	}
	return runErr
}

func newLiandongBatchID() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (s *LiandongRestockService) fulfillPendingBatch(ctx context.Context, state *LiandongRestockState) error {
	batch := state.PendingBatch
	if batch == nil {
		return nil
	}
	codes, err := s.deriveCodesChecked(batch)
	if err != nil {
		return err
	}
	if err := s.recordBatchPending(ctx, batch, codes); err != nil {
		return err
	}
	segments, err := s.loadLiandongSegmentStatuses(ctx, batch.BatchID)
	if err != nil {
		return err
	}
	if len(segments) != len(liandongSegmentRanges(len(codes))) {
		return errors.New("Liandong batch segment accounting is incomplete")
	}
	for _, segment := range segments {
		if segment.Status == liandongSegmentStatusUploaded {
			continue
		}
		if segment.Status == liandongSegmentStatusNeedsReconciliation {
			return fmt.Errorf("%w: batch %s segment %d", ErrLiandongNeedsReconciliation, batch.BatchID, segment.SegmentNo)
		}
		start := segment.Offset
		end := start + segment.CodeCount
		if start < 0 || end > len(codes) || start >= end {
			return errors.New("Liandong batch segment range is invalid")
		}
		segmentCodes := codes[start:end]
		if liandongCodesDigest(segmentCodes) != segment.CodeSHA256 {
			return errors.New("Liandong batch segment hash does not match deterministic codes")
		}
		if segment.Status == liandongSegmentStatusPending || segment.Status == liandongSegmentStatusFailed {
			if err := s.ensureLiandongCodes(ctx, batch, segmentCodes); err != nil {
				_ = s.markBatchFailed(ctx, batch.BatchID, err)
				return err
			}
			if err := s.markLiandongSegmentCodesCreated(ctx, batch.BatchID, segment.SegmentNo); err != nil {
				return err
			}
		}
		if err := s.uploadCodes(ctx, batch.GoodsID, segmentCodes); err != nil {
			if isLiandongOutcomeUnknown(err) {
				_ = s.markLiandongSegmentNeedsReconciliation(ctx, batch.BatchID, segment.SegmentNo, err)
				_ = s.markBatchNeedsReconciliation(ctx, batch.BatchID, err)
				return fmt.Errorf("%w: batch %s segment %d: %v", ErrLiandongNeedsReconciliation, batch.BatchID, segment.SegmentNo, err)
			}
			_ = s.markLiandongSegmentFailed(ctx, batch.BatchID, segment.SegmentNo, err)
			_ = s.markBatchFailed(ctx, batch.BatchID, err)
			return err
		}
		if err := s.markLiandongSegmentUploaded(ctx, batch.BatchID, segment.SegmentNo); err != nil {
			return err
		}
	}

	var remoteStockAfter *int
	if stock, fetchErr := s.fetchUnsoldStock(ctx, batch.GoodsID); fetchErr == nil {
		remoteStockAfter = &stock
	}
	if err := s.markBatchUploadedObserved(ctx, batch.BatchID, remoteStockAfter); err != nil {
		return err
	}
	if remoteStockAfter != nil {
		for i := range state.Products {
			if state.Products[i].GoodsID == batch.GoodsID {
				state.Products[i].CurrentStock = remoteStockAfter
				state.Products[i].LastError = ""
				state.Products[i].LastRunAt = time.Now().UTC().Format(time.RFC3339)
				break
			}
		}
	}
	state.PendingBatch = nil
	return s.saveState(ctx, state)
}

func (s *LiandongRestockService) persistProductMappings(ctx context.Context, products []LiandongRestockProduct) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, product := range products {
		mappingKey := liandongMappingKey(product)
		targetStock := effectiveLiandongTargetStock(product)
		if _, err := tx.ExecContext(ctx, `
			UPDATE liandong_product_mappings
			SET enabled = FALSE
			WHERE goods_id = $1 AND enabled = TRUE AND mapping_key <> $2`, product.GoodsID, mappingKey); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO liandong_product_mappings
			(mapping_key, goods_id, cny_amount, grant_type, grant_value, external_url, version, enabled, target_stock)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (mapping_key) DO UPDATE SET enabled = EXCLUDED.enabled`,
			mappingKey, product.GoodsID, product.CNYAmount, product.GrantType, product.USDCredit,
			product.ExternalURL, product.Version, product.Enabled, targetStock); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *LiandongRestockService) recordBatchPending(ctx context.Context, batch *liandongRestockPendingBatch, codes []string) error {
	if batch == nil || batch.BatchID == "" {
		return errors.New("Liandong batch is required")
	}
	if len(codes) == 0 || len(codes) > liandongMaxTargetStock {
		return errors.New("invalid Liandong batch code count")
	}
	codeDigest := liandongCodesDigest(codes)
	batchCopy := *batch
	if batchCopy.GrantType == "" {
		batchCopy.GrantType = "balance"
	}
	if batchCopy.Version <= 0 {
		batchCopy.Version = 1
	}
	if batchCopy.MappingKey == "" {
		batchCopy.MappingKey = liandongMappingKey(LiandongRestockProduct{
			CNYAmount: batchCopy.CNYAmount, USDCredit: batchCopy.USDCredit, GoodsID: batchCopy.GoodsID,
			GrantType: batchCopy.GrantType, ExternalURL: batchCopy.ExternalURL, Version: batchCopy.Version,
		})
	}
	if batchCopy.TargetStock <= 0 {
		batchCopy.TargetStock = effectiveLiandongTargetStock(LiandongRestockProduct{RestockCount: batchCopy.Count})
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		if s.memoryBatches == nil {
			s.memoryBatches = make(map[string]*liandongMemoryBatch)
		}
		if existing, ok := s.memoryBatches[batchCopy.BatchID]; ok {
			if existing.CodeSHA256 != codeDigest || existing.Batch.Count != len(codes) || existing.Batch.MappingKey != batchCopy.MappingKey {
				return errors.New("Liandong pending batch snapshot does not match deterministic codes")
			}
			return nil
		}
		memoryBatch := &liandongMemoryBatch{
			Batch: batchCopy, Codes: append([]string(nil), codes...), CodeSHA256: codeDigest,
			Status: liandongBatchStatusPending, CreatedAt: batchCopy.CreatedAt,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		for segmentNo, bounds := range liandongSegmentRanges(len(codes)) {
			segmentCodes := codes[bounds[0] : bounds[0]+bounds[1]]
			memoryBatch.Segments = append(memoryBatch.Segments, liandongMemorySegment{LiandongRestockSegmentStatus{
				BatchID: batchCopy.BatchID, SegmentNo: segmentNo, Offset: bounds[0], CodeCount: bounds[1],
				CodeSHA256: liandongCodesDigest(segmentCodes), Status: liandongSegmentStatusPending,
				UpdatedAt: memoryBatch.UpdatedAt,
			}})
		}
		s.memoryBatches[batchCopy.BatchID] = memoryBatch
		return nil
	}
	snapshotRaw, err := json.Marshal(LiandongRestockMappingSnapshot{
		MappingKey: batchCopy.MappingKey, Version: batchCopy.Version, GoodsID: batchCopy.GoodsID,
		CNYAmount: batchCopy.CNYAmount, GrantType: batchCopy.GrantType, USDCredit: batchCopy.USDCredit,
		ExternalURL: batchCopy.ExternalURL, TargetStock: batchCopy.TargetStock,
	})
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO liandong_restock_batches
		(batch_id, job_id, goods_id, cny_amount, grant_value, code_count, code_sha256, status,
		 remote_stock_before, created_at, mapping_key, mapping_version, grant_type, external_url,
		 target_stock, planned_count, mapping_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, $9, $10, $11, $12, $13, $14, $6, $15::jsonb)
		ON CONFLICT (batch_id) DO NOTHING`, batchCopy.BatchID, nullableLiandongString(batchCopy.JobID), batchCopy.GoodsID,
		batchCopy.CNYAmount, batchCopy.USDCredit, len(codes), codeDigest, batchCopy.RemoteStockBefore,
		batchCopy.CreatedAt, batchCopy.MappingKey, batchCopy.Version, batchCopy.GrantType, batchCopy.ExternalURL,
		batchCopy.TargetStock, string(snapshotRaw))
	if err != nil {
		return err
	}
	for segmentNo, bounds := range liandongSegmentRanges(len(codes)) {
		segmentCodes := codes[bounds[0] : bounds[0]+bounds[1]]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO liandong_restock_segments
			(batch_id, segment_no, ordinal_start, code_count, code_sha256, status)
			VALUES ($1, $2, $3, $4, $5, 'pending')
			ON CONFLICT (batch_id, segment_no) DO NOTHING`, batchCopy.BatchID, segmentNo, bounds[0], bounds[1], liandongCodesDigest(segmentCodes)); err != nil {
			return err
		}
	}
	for i, code := range codes {
		codeDigest := sha256.Sum256([]byte(code))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO liandong_restock_batch_codes (batch_id, code_sha256, code_hint, ordinal)
			VALUES ($1, $2, $3, $4) ON CONFLICT (batch_id, ordinal) DO NOTHING`,
			batchCopy.BatchID, hex.EncodeToString(codeDigest[:]), code[:minInt(len(code), 11)], i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *LiandongRestockService) markBatchUploaded(ctx context.Context, batchID string, remoteStockAfter int) error {
	value := remoteStockAfter
	return s.markBatchUploadedObserved(ctx, batchID, &value)
}

func (s *LiandongRestockService) markBatchUploadedObserved(ctx context.Context, batchID string, remoteStockAfter *int) error {
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		batch, ok := s.memoryBatches[batchID]
		if !ok {
			return nil
		}
		batch.Status = liandongBatchStatusUploaded
		batch.Error = ""
		batch.RemoteAfter = nil
		if remoteStockAfter != nil {
			value := *remoteStockAfter
			batch.RemoteAfter = &value
		}
		batch.UploadedAt = time.Now().UTC().Format(time.RFC3339)
		batch.UpdatedAt = batch.UploadedAt
		return nil
	}
	var remoteAfter any
	if remoteStockAfter != nil {
		remoteAfter = *remoteStockAfter
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE liandong_restock_batches
		SET status = 'uploaded', uploaded_at = NOW(), updated_at = NOW(), remote_stock_after = $2, error = NULL
		WHERE batch_id = $1`, batchID, remoteAfter)
	return err
}

func (s *LiandongRestockService) markBatchFailed(ctx context.Context, batchID string, runErr error) error {
	if runErr == nil {
		return errors.New("Liandong batch failure requires an error")
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		if batch, ok := s.memoryBatches[batchID]; ok {
			batch.Status = liandongBatchStatusFailed
			batch.Error = runErr.Error()
			batch.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE liandong_restock_batches
		SET status = 'failed', error = $2, updated_at = NOW()
		WHERE batch_id = $1`, batchID, runErr.Error())
	return err
}

func (s *LiandongRestockService) markBatchNeedsReconciliation(ctx context.Context, batchID string, runErr error) error {
	if runErr == nil {
		return errors.New("Liandong reconciliation state requires an error")
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		if batch, ok := s.memoryBatches[batchID]; ok {
			batch.Status = liandongBatchStatusNeedsReconciliation
			batch.Error = runErr.Error()
			batch.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE liandong_restock_batches
		SET status = 'needs_reconciliation', error = $2, updated_at = NOW()
		WHERE batch_id = $1`, batchID, runErr.Error())
	return err
}

func (s *LiandongRestockService) loadBatchStatuses(ctx context.Context, limit int) ([]LiandongRestockBatchStatus, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if s.db == nil {
		s.memoryMu.Lock()
		defer s.memoryMu.Unlock()
		result := make([]LiandongRestockBatchStatus, 0, minInt(limit, len(s.memoryBatches)))
		for _, memoryBatch := range s.memoryBatches {
			status := LiandongRestockBatchStatus{
				BatchID: memoryBatch.Batch.BatchID, JobID: memoryBatch.Batch.JobID, GoodsID: memoryBatch.Batch.GoodsID,
				CNYAmount: float64(memoryBatch.Batch.CNYAmount), CodeCount: memoryBatch.Batch.Count, Status: memoryBatch.Status,
				MappingKey: memoryBatch.Batch.MappingKey, MappingVersion: memoryBatch.Batch.Version,
				TargetStock: memoryBatch.Batch.TargetStock, Error: memoryBatch.Error,
				CreatedAt: memoryBatch.CreatedAt, UploadedAt: memoryBatch.UploadedAt, UpdatedAt: memoryBatch.UpdatedAt,
			}
			if memoryBatch.Batch.RemoteStockBefore != nil {
				value := *memoryBatch.Batch.RemoteStockBefore
				status.RemoteStockBefore = &value
			}
			if memoryBatch.RemoteAfter != nil {
				value := *memoryBatch.RemoteAfter
				status.RemoteStockAfter = &value
			}
			status.SegmentsTotal = len(memoryBatch.Segments)
			for _, segment := range memoryBatch.Segments {
				if segment.Status == liandongSegmentStatusUploaded {
					status.SegmentsUploaded++
				}
			}
			result = append(result, status)
			if len(result) >= limit {
				break
			}
		}
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT batch_id, goods_id, cny_amount::text, code_count, status,
		       remote_stock_before, remote_stock_after, error, created_at, uploaded_at, updated_at
		FROM liandong_restock_batches
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LiandongRestockBatchStatus, 0, limit)
	for rows.Next() {
		var batch LiandongRestockBatchStatus
		var cnyText string
		var remoteBefore, remoteAfter sql.NullInt64
		var failure sql.NullString
		var createdAt, updatedAt time.Time
		var uploadedAt sql.NullTime
		if err := rows.Scan(&batch.BatchID, &batch.GoodsID, &cnyText, &batch.CodeCount, &batch.Status,
			&remoteBefore, &remoteAfter, &failure, &createdAt, &uploadedAt, &updatedAt); err != nil {
			return nil, err
		}
		batch.CNYAmount, err = strconv.ParseFloat(cnyText, 64)
		if err != nil {
			return nil, fmt.Errorf("decode Liandong batch amount: %w", err)
		}
		if remoteBefore.Valid {
			value := int(remoteBefore.Int64)
			batch.RemoteStockBefore = &value
		}
		if remoteAfter.Valid {
			value := int(remoteAfter.Int64)
			batch.RemoteStockAfter = &value
		}
		if failure.Valid {
			batch.Error = failure.String
		}
		batch.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if uploadedAt.Valid {
			batch.UploadedAt = uploadedAt.Time.UTC().Format(time.RFC3339)
		}
		batch.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		result = append(result, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type liandongAPIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (s *LiandongRestockService) post(ctx context.Context, path string, payload any) (*liandongAPIResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	s.configMu.RLock()
	baseURL := s.baseURL
	token := s.token
	s.configMu.RUnlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Merchant-Token", token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, &LiandongRemoteOutcomeUnknownError{Err: err}
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 2<<20)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, &LiandongRemoteOutcomeUnknownError{Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &LiandongRemoteFailureError{StatusCode: resp.StatusCode, Message: "HTTP response rejected the request"}
	}
	var result liandongAPIResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, &LiandongRemoteOutcomeUnknownError{Err: errors.New("Liandong returned an invalid response")}
	}
	if result.Code != 1 {
		message := strings.TrimSpace(result.Msg)
		if message == "" {
			message = "request rejected"
		}
		return nil, &LiandongRemoteFailureError{Message: message}
	}
	return &result, nil
}

func (s *LiandongRestockService) fetchUnsoldStock(ctx context.Context, goodsID int64) (int, error) {
	result, err := s.post(ctx, "/merchantApi/goodsCardStorage/list", map[string]any{
		"goods_id": goodsID, "current": 1, "pageSize": 1, "status": "0", "first": "", "keywords": "",
	})
	if err != nil {
		return 0, err
	}
	var data struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil || data.Total < 0 {
		return 0, errors.New("Liandong returned invalid inventory data")
	}
	return data.Total, nil
}

func (s *LiandongRestockService) uploadCodes(ctx context.Context, goodsID int64, codes []string) error {
	if len(codes) == 0 || len(codes) > liandongSegmentSize {
		return fmt.Errorf("Liandong upload segment must contain between 1 and %d codes", liandongSegmentSize)
	}
	_, err := s.post(ctx, "/merchantApi/GoodsCardStorage/add", map[string]any{
		"goods_id":      goodsID,
		"content":       strings.Join(codes, "\n"),
		"first":         0,
		"remove_repeat": 1,
	})
	return err
}
