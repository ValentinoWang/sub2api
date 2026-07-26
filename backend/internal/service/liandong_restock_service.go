package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
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
	Threshold    int     `json:"threshold"`
	RestockCount int     `json:"restock_count"`
	Enabled      bool    `json:"enabled"`
	CurrentStock *int    `json:"current_stock,omitempty"`
	LastError    string  `json:"last_error,omitempty"`
	LastRunAt    string  `json:"last_run_at,omitempty"`
}

type liandongRestockPendingBatch struct {
	BatchID   string  `json:"batch_id"`
	GoodsID   int64   `json:"goods_id"`
	CNYAmount int     `json:"cny_amount"`
	USDCredit float64 `json:"usd_credit"`
	Count     int     `json:"count"`
	CreatedAt string  `json:"created_at"`
}

type LiandongRestockState struct {
	Enabled      bool                         `json:"enabled"`
	Products     []LiandongRestockProduct     `json:"products"`
	LastRunAt    string                       `json:"last_run_at,omitempty"`
	LastError    string                       `json:"last_error,omitempty"`
	PendingBatch *liandongRestockPendingBatch `json:"pending_batch,omitempty"`
}

type LiandongRestockStatus struct {
	Configured              bool                     `json:"configured"`
	MerchantTokenConfigured bool                     `json:"merchant_token_configured"`
	CodeSecretConfigured    bool                     `json:"code_secret_configured"`
	Enabled                 bool                     `json:"enabled"`
	Running                 bool                     `json:"running"`
	IntervalSeconds         int                      `json:"interval_seconds"`
	LastRunAt               string                   `json:"last_run_at,omitempty"`
	LastError               string                   `json:"last_error,omitempty"`
	PendingBatch            bool                     `json:"pending_batch"`
	Products                []LiandongRestockProduct `json:"products"`
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
	baseURL     string
	token       string
	codeSecret  []byte
	products    []LiandongRestockProduct
	interval    time.Duration
	httpClient  *http.Client

	configMu  sync.RWMutex
	mu        sync.Mutex
	stateMu   sync.Mutex
	running   bool
	runCancel context.CancelFunc
	stop      chan struct{}
	stopOnce  sync.Once
}

// ProvideLiandongRestockService starts the durable background worker. The
// persisted enabled flag remains authoritative across process restarts.
func ProvideLiandongRestockService(settingRepo SettingRepository, redeem *RedeemService, cfg *config.Config, encryptor SecretEncryptor) *LiandongRestockService {
	svc := NewLiandongRestockService(settingRepo, redeem, cfg, encryptor)
	if err := svc.loadStoredConfig(context.Background()); err != nil {
		logger.LegacyPrintf("service.liandong_restock", "[LiandongRestock] load stored configuration failed: %v", err)
	}
	svc.StartWorker()
	return svc
}

func NewLiandongRestockService(settingRepo SettingRepository, redeem *RedeemService, cfg *config.Config, encryptor SecretEncryptor) *LiandongRestockService {
	interval := 5 * time.Minute
	if cfg != nil && cfg.LiandongRestock.IntervalSecs >= 30 {
		interval = time.Duration(cfg.LiandongRestock.IntervalSecs) * time.Second
	}
	s := &LiandongRestockService{
		settingRepo: settingRepo,
		redeem:      redeem,
		encryptor:   encryptor,
		interval:    interval,
		httpClient:  &http.Client{Timeout: 20 * time.Second},
		stop:        make(chan struct{}),
	}
	if cfg != nil {
		s.baseURL = strings.TrimRight(strings.TrimSpace(cfg.LiandongRestock.BaseURL), "/")
		s.token = strings.TrimSpace(cfg.LiandongRestock.MerchantToken)
		s.codeSecret = []byte(cfg.LiandongRestock.CodeSecret)
		_ = json.Unmarshal([]byte(cfg.LiandongRestock.ProductsJSON), &s.products)
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
	seen := make(map[int]struct{}, len(s.products))
	for _, product := range s.products {
		if product.CNYAmount <= 0 || product.USDCredit <= 0 || product.GoodsID <= 0 || product.RestockCount <= 0 {
			return false
		}
		if _, ok := seen[product.CNYAmount]; ok {
			return false
		}
		seen[product.CNYAmount] = struct{}{}
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
	s.products = normalizeLiandongProducts(stored.Products)
}

func normalizeLiandongProducts(products []LiandongRestockProduct) []LiandongRestockProduct {
	out := cloneLiandongProducts(products)
	for i := range out {
		out[i].CurrentStock = nil
		out[i].LastError = ""
		out[i].LastRunAt = ""
		if out[i].Threshold < 0 {
			out[i].Threshold = 0
		}
		if out[i].RestockCount <= 0 {
			out[i].RestockCount = 10
		}
	}
	return out
}

func (s *LiandongRestockService) loadState(ctx context.Context) (*LiandongRestockState, error) {
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
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	s.configMu.RLock()
	merchantTokenConfigured := strings.TrimSpace(s.token) != ""
	codeSecretConfigured := len(s.codeSecret) >= 32
	intervalSeconds := int(s.interval.Seconds())
	s.configMu.RUnlock()
	return &LiandongRestockStatus{
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
	for _, product := range normalized {
		if product.CNYAmount <= 0 || product.USDCredit <= 0 || product.GoodsID <= 0 {
			return nil, errors.New("each Liandong product requires a positive CNY amount, USD credit, and numeric goods ID")
		}
		if product.Threshold < 0 || product.Threshold > 1000 || product.RestockCount < 1 || product.RestockCount > 1000 {
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
		Configured:              s.configuredLocked(),
		MerchantTokenConfigured: strings.TrimSpace(s.token) != "",
		CodeSecretConfigured:    len(s.codeSecret) >= 32,
		Enabled:                 state.Enabled,
		Running:                 running,
		IntervalSeconds:         int(s.interval.Seconds()),
		LastRunAt:               state.LastRunAt,
		LastError:               state.LastError,
		PendingBatch:            state.PendingBatch != nil,
		Products:                cloneLiandongProducts(state.Products),
	}
	s.configMu.RUnlock()
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
		if update.Threshold < 0 || update.RestockCount < 1 || update.RestockCount > 1000 {
			s.stateMu.Unlock()
			return nil, fmt.Errorf("invalid policy for CNY %d", update.CNYAmount)
		}
		byCNY[update.CNYAmount] = update
	}
	if len(byCNY) != len(state.Products) {
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
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	s.running = true
	s.runCancel = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.running = false
		s.runCancel = nil
		s.mu.Unlock()
	}()
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	state, err := s.loadState(ctx)
	if err != nil {
		return err
	}
	if !force && !state.Enabled {
		return nil
	}
	if !s.configured() {
		return errors.New("Liandong auto restock is not fully configured")
	}
	if state.PendingBatch != nil {
		if err := s.fulfillPendingBatch(ctx, state); err != nil {
			return s.recordRunError(ctx, state, err)
		}
		state.LastRunAt = time.Now().UTC().Format(time.RFC3339)
		state.LastError = ""
		return s.saveState(ctx, state)
	}

	for i := range state.Products {
		product := &state.Products[i]
		if !product.Enabled {
			continue
		}
		stock, err := s.fetchUnsoldStock(ctx, product.GoodsID)
		if err != nil {
			product.LastError = err.Error()
			return s.recordRunError(ctx, state, err)
		}
		product.CurrentStock = &stock
		product.LastRunAt = time.Now().UTC().Format(time.RFC3339)
		product.LastError = ""
		if stock >= product.Threshold {
			continue
		}
		batchID, err := newLiandongBatchID()
		if err != nil {
			return s.recordRunError(ctx, state, err)
		}
		state.PendingBatch = &liandongRestockPendingBatch{
			BatchID: batchID, GoodsID: product.GoodsID, CNYAmount: product.CNYAmount,
			USDCredit: product.USDCredit, Count: product.RestockCount,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.saveState(ctx, state); err != nil {
			return err
		}
		if err := s.fulfillPendingBatch(ctx, state); err != nil {
			return s.recordRunError(ctx, state, err)
		}
		stock += product.RestockCount
		product.CurrentStock = &stock
	}
	state.LastRunAt = time.Now().UTC().Format(time.RFC3339)
	state.LastError = ""
	return s.saveState(ctx, state)
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

func (s *LiandongRestockService) deriveCodes(batch *liandongRestockPendingBatch) []string {
	s.configMu.RLock()
	secret := append([]byte(nil), s.codeSecret...)
	s.configMu.RUnlock()
	codes := make([]string, 0, batch.Count)
	for i := 0; i < batch.Count; i++ {
		mac := hmac.New(sha256.New, secret)
		_, _ = fmt.Fprintf(mac, "%s:%d", batch.BatchID, i)
		digest := strings.ToUpper(hex.EncodeToString(mac.Sum(nil)[:16]))
		codes = append(codes, "LD-"+digest[0:8]+"-"+digest[8:16]+"-"+digest[16:24]+"-"+digest[24:32])
	}
	return codes
}

func (s *LiandongRestockService) fulfillPendingBatch(ctx context.Context, state *LiandongRestockState) error {
	batch := state.PendingBatch
	if batch == nil {
		return nil
	}
	codes := s.deriveCodes(batch)
	for _, code := range codes {
		existing, err := s.redeem.GetByCode(ctx, code)
		if err == nil {
			if existing.Type != RedeemTypeBalance || math.Abs(existing.Value-batch.USDCredit) > 0.000001 {
				return errors.New("derived Liandong code conflicts with an existing redeem code")
			}
			continue
		}
		if !errors.Is(err, ErrRedeemCodeNotFound) {
			return err
		}
		if err := s.redeem.CreateCode(ctx, &RedeemCode{
			Code: code, Type: RedeemTypeBalance, Value: batch.USDCredit,
			Status: StatusUnused, Notes: "liandong:auto:" + batch.BatchID,
		}); err != nil {
			return err
		}
	}
	if err := s.uploadCodes(ctx, batch.GoodsID, codes); err != nil {
		return err
	}
	state.PendingBatch = nil
	return s.saveState(ctx, state)
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
		return nil, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 2<<20)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Liandong returned HTTP %d", resp.StatusCode)
	}
	var result liandongAPIResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, errors.New("Liandong returned an invalid response")
	}
	if result.Code != 1 {
		message := strings.TrimSpace(result.Msg)
		if message == "" {
			message = "request rejected"
		}
		return nil, fmt.Errorf("Liandong: %s", message)
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
	_, err := s.post(ctx, "/merchantApi/GoodsCardStorage/add", map[string]any{
		"goods_id":      goodsID,
		"content":       strings.Join(codes, "\n"),
		"first":         0,
		"remove_repeat": 1,
	})
	return err
}
