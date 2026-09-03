package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type liandongTestEncryptor struct{}

func (liandongTestEncryptor) Encrypt(plaintext string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (liandongTestEncryptor) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	return string(raw), err
}

type liandongSettingRepoStub struct {
	mu     sync.Mutex
	values map[string]string
}

func (r *liandongSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *liandongSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (r *liandongSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}
func (r *liandongSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (r *liandongSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *liandongSettingRepoStub) GetAll(context.Context) (map[string]string, error)    { return nil, nil }
func (r *liandongSettingRepoStub) Delete(context.Context, string) error                 { return nil }

type liandongRedeemStoreStub struct {
	mu    sync.Mutex
	codes map[string]*RedeemCode
}

func (r *liandongRedeemStoreStub) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.codes[code]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	copy := *value
	return &copy, nil
}
func (r *liandongRedeemStoreStub) CreateCode(_ context.Context, code *RedeemCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.codes[code.Code]; ok {
		return errors.New("duplicate code")
	}
	copy := *code
	r.codes[code.Code] = &copy
	return nil
}

func newLiandongTestService(serverURL string) (*LiandongRestockService, *liandongSettingRepoStub, *liandongRedeemStoreStub) {
	settings := &liandongSettingRepoStub{values: map[string]string{}}
	redeem := &liandongRedeemStoreStub{codes: map[string]*RedeemCode{}}
	svc := &LiandongRestockService{
		settingRepo: settings,
		redeem:      redeem,
		baseURL:     serverURL,
		token:       "test-token",
		codeSecret:  []byte(strings.Repeat("s", 32)),
		products: []LiandongRestockProduct{{
			CNYAmount: 20, USDCredit: 2.78, GoodsID: 42, Threshold: 5, RestockCount: 3, Enabled: true,
		}},
		interval:   time.Minute,
		httpClient: &http.Client{Timeout: 2 * time.Second},
		stop:       make(chan struct{}),
	}
	return svc, settings, redeem
}

func TestLiandongRestockConfigurationGeneratesAndEncryptsSecret(t *testing.T) {
	settings := &liandongSettingRepoStub{values: map[string]string{}}
	redeem := &liandongRedeemStoreStub{codes: map[string]*RedeemCode{}}
	svc := &LiandongRestockService{
		settingRepo: settings,
		redeem:      redeem,
		encryptor:   liandongTestEncryptor{},
		baseURL:     "https://ldxp.cn",
		interval:    time.Minute,
		httpClient:  &http.Client{Timeout: time.Second},
		stop:        make(chan struct{}),
	}

	status, err := svc.UpdateConfiguration(context.Background(), LiandongRestockConfigurationUpdate{
		MerchantToken: "merchant-token-from-liandong",
		Products: []LiandongRestockProduct{{
			CNYAmount: 20, USDCredit: 2.78, GoodsID: 12345,
			Threshold: 5, RestockCount: 10, Enabled: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || !status.MerchantTokenConfigured || !status.CodeSecretConfigured {
		t.Fatalf("unexpected status: %+v", status)
	}
	raw := settings.values[liandongRestockConfigKey]
	if strings.Contains(raw, "merchant-token-from-liandong") {
		t.Fatal("stored configuration exposed the merchant token")
	}
	plaintext, err := (liandongTestEncryptor{}).Decrypt(raw)
	if err != nil {
		t.Fatal(err)
	}
	var stored liandongRestockStoredConfig
	if err := json.Unmarshal([]byte(plaintext), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.MerchantToken != "merchant-token-from-liandong" {
		t.Fatal("merchant token was not persisted")
	}
	if len(stored.CodeSecret) != 64 {
		t.Fatalf("generated secret length = %d, want 64 hex characters", len(stored.CodeSecret))
	}
	if stored.Products[0].GoodsID != 12345 {
		t.Fatal("goods ID was not persisted")
	}
}

func TestLiandongRestockConfigurationRequiresRealNumericGoodsID(t *testing.T) {
	_, err := validateLiandongConfiguration("token", strings.Repeat("s", 32), []LiandongRestockProduct{{
		CNYAmount: 20, USDCredit: 2.78, GoodsID: 0, Threshold: 5, RestockCount: 10,
	}})
	if err == nil || !strings.Contains(err.Error(), "numeric goods ID") {
		t.Fatalf("got %v, want numeric goods ID validation error", err)
	}
}

func TestLiandongRestockRetryReusesPendingBatch(t *testing.T) {
	var mu sync.Mutex
	var uploads []string
	failFirstUpload := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Merchant-Token") != "test-token" {
			t.Error("missing merchant token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			_, _ = w.Write([]byte(`{"code":1,"data":{"total":0}}`))
		case "/merchantApi/GoodsCardStorage/add":
			var body struct {
				Content      string `json:"content"`
				RemoveRepeat int    `json:"remove_repeat"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.RemoveRepeat != 1 {
				t.Error("remove_repeat must be enabled")
			}
			mu.Lock()
			uploads = append(uploads, body.Content)
			shouldFail := failFirstUpload
			failFirstUpload = false
			mu.Unlock()
			if shouldFail {
				http.Error(w, "temporary", http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"code":1,"data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, settings, redeem := newLiandongTestService(server.URL)
	if err := svc.RunOnce(context.Background(), true); err == nil {
		t.Fatal("expected first upload to fail")
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.PendingBatch {
		t.Fatal("failed upload must preserve the pending batch")
	}
	if len(redeem.codes) != 3 {
		t.Fatalf("got %d codes, want 3", len(redeem.codes))
	}

	if err := svc.RunOnce(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.PendingBatch {
		t.Fatal("successful retry must clear the pending batch")
	}
	if len(redeem.codes) != 3 {
		t.Fatalf("retry created another batch: %d codes", len(redeem.codes))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(uploads) != 2 || uploads[0] != uploads[1] {
		t.Fatal("retry did not upload the identical deterministic batch")
	}
	if _, ok := settings.values[liandongRestockStateKey]; !ok {
		t.Fatal("state was not persisted")
	}
}

func TestLiandongRestockStopCancelsActiveRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseServer := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(requestStarted) })
		select {
		case <-r.Context().Done():
		case <-releaseServer:
		}
	}))
	defer func() {
		close(releaseServer)
		server.Close()
	}()

	svc, settings, _ := newLiandongTestService(server.URL)
	state := &LiandongRestockState{Enabled: true, Products: cloneLiandongProducts(svc.products)}
	if err := svc.saveState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- svc.RunOnce(context.Background(), false) }()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("inventory request did not start")
	}
	status, err := svc.SetEnabled(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled {
		t.Fatal("stop did not persist the disabled state")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("active request returned %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel the active request")
	}
	if raw := settings.values[liandongRestockStateKey]; !strings.Contains(raw, `"enabled":false`) {
		t.Fatal("disabled state was not saved")
	}
}
