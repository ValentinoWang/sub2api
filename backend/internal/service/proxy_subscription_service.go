package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	proxySubscriptionStateVersion   = 1
	proxySubscriptionInactiveStatus = "inactive"
)

type ProxySubscriptionImportInput struct {
	Name string
	URL  string
}

type ProxySubscriptionImportResult struct {
	SubscriptionID string `json:"subscription_id"`
	NodeCount      int    `json:"node_count"`
	Created        int    `json:"created"`
	Reused         int    `json:"reused"`
	Deactivated    int    `json:"deactivated"`
}

type proxySubscriptionRuntimeState struct {
	Version          int                             `json:"version"`
	ControllerSecret string                          `json:"controller_secret"`
	Subscriptions    []proxySubscriptionRuntimeEntry `json:"subscriptions"`
}

type proxySubscriptionRuntimeEntry struct {
	ID        string                         `json:"id"`
	Name      string                         `json:"name"`
	UpdatedAt time.Time                      `json:"updated_at"`
	Nodes     []proxySubscriptionRuntimeNode `json:"nodes"`
}

type proxySubscriptionRuntimeNode struct {
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name"`
	SourceURI   string `json:"source_uri"`
	ProxyID     int64  `json:"proxy_id"`
	Port        int    `json:"port"`
}

// ProxySubscriptionService owns the persisted mapping between imported nodes,
// Mihomo listeners, and Sub2API proxy records.
type ProxySubscriptionService struct {
	admin            AdminService
	cfg              config.ProxySubscriptionConfig
	fetchClient      *http.Client
	controllerClient *http.Client
	mu               sync.Mutex
}

func NewProxySubscriptionService(admin AdminService, cfg *config.Config) *ProxySubscriptionService {
	runtimeCfg := config.ProxySubscriptionConfig{}
	if cfg != nil {
		runtimeCfg = cfg.ProxySubscription
	}
	service := &ProxySubscriptionService{admin: admin, cfg: runtimeCfg}
	service.fetchClient = &http.Client{
		Timeout: 25 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return fmt.Errorf("proxy subscription redirect limit exceeded")
			}
			return validateProxySubscriptionURL(req.Context(), req.URL)
		},
	}
	service.controllerClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
	return service
}

func (s *ProxySubscriptionService) Import(ctx context.Context, input ProxySubscriptionImportInput) (*ProxySubscriptionImportResult, error) {
	if s == nil || !s.cfg.Enabled {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "PROXY_SUBSCRIPTION_DISABLED", "Proxy subscription runtime is not enabled")
	}
	name := sanitizeProxySubscriptionName(input.Name)
	if name == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "PROXY_SUBSCRIPTION_NAME_REQUIRED", "Subscription name is required")
	}
	sourceURL, err := url.Parse(strings.TrimSpace(input.URL))
	if err != nil || sourceURL == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "PROXY_SUBSCRIPTION_URL_INVALID", "Subscription URL is invalid")
	}
	if err := validateProxySubscriptionURL(ctx, sourceURL); err != nil {
		return nil, infraerrors.New(http.StatusBadRequest, "PROXY_SUBSCRIPTION_URL_INVALID", err.Error())
	}

	raw, err := s.fetch(ctx, sourceURL)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "PROXY_SUBSCRIPTION_FETCH_FAILED", "Failed to fetch proxy subscription").WithCause(err)
	}
	nodes, err := ParseProxySubscription(raw)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadRequest, "PROXY_SUBSCRIPTION_PARSE_FAILED", err.Error())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.importLocked(ctx, name, sourceURL.String(), nodes)
}

func (s *ProxySubscriptionService) fetch(ctx context.Context, sourceURL *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/plain, application/octet-stream;q=0.9, */*;q=0.1")
	req.Header.Set("User-Agent", "Sub2API-Proxy-Subscription/1.0")
	resp, err := s.fetchClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription server returned HTTP %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxProxySubscriptionBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxProxySubscriptionBytes {
		return nil, fmt.Errorf("subscription response exceeds %d bytes", maxProxySubscriptionBytes)
	}
	return body, nil
}

func (s *ProxySubscriptionService) importLocked(ctx context.Context, name, sourceURL string, nodes []ProxySubscriptionNode) (*ProxySubscriptionImportResult, error) {
	state, err := s.loadState()
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_STATE_INVALID", "Proxy subscription state is invalid").WithCause(err)
	}
	previousConfig, err := os.ReadFile(s.cfg.ConfigPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_CONFIG_READ_FAILED", "Proxy runtime configuration cannot be read").WithCause(err)
	}
	controllerAuthSecret := state.ControllerSecret
	if len(previousConfig) > 0 {
		controllerAuthSecret, err = proxySubscriptionControllerSecret(previousConfig)
		if err != nil {
			return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_CONFIG_INVALID", "Proxy runtime configuration is invalid").WithCause(err)
		}
	}
	if state.ControllerSecret == "" {
		state.ControllerSecret, err = randomProxySubscriptionSecret()
		if err != nil {
			return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_SECRET_FAILED", "Failed to initialize proxy runtime").WithCause(err)
		}
	}

	sourceHash := sha256.Sum256([]byte(sourceURL))
	subscriptionID := hex.EncodeToString(sourceHash[:])[:24]
	entryIndex := -1
	for i := range state.Subscriptions {
		if state.Subscriptions[i].ID == subscriptionID {
			entryIndex = i
			break
		}
	}
	previous := proxySubscriptionRuntimeEntry{ID: subscriptionID}
	if entryIndex >= 0 {
		previous = state.Subscriptions[entryIndex]
	}
	previousByFingerprint := make(map[string]proxySubscriptionRuntimeNode, len(previous.Nodes))
	for _, node := range previous.Nodes {
		previousByFingerprint[node.Fingerprint] = node
	}

	usedPorts := make(map[int]struct{})
	for _, subscription := range state.Subscriptions {
		for _, node := range subscription.Nodes {
			usedPorts[node.Port] = struct{}{}
		}
	}
	for _, node := range previous.Nodes {
		delete(usedPorts, node.Port)
	}

	result := &ProxySubscriptionImportResult{SubscriptionID: subscriptionID, NodeCount: len(nodes)}
	createdProxies := make([]*Proxy, 0)
	reusedInactiveProxies := make([]*Proxy, 0)
	nextEntry := proxySubscriptionRuntimeEntry{ID: subscriptionID, Name: name, UpdatedAt: time.Now().UTC(), Nodes: make([]proxySubscriptionRuntimeNode, 0, len(nodes))}
	for _, parsed := range nodes {
		if old, ok := previousByFingerprint[parsed.Fingerprint]; ok {
			proxy, getErr := s.admin.GetProxy(ctx, old.ProxyID)
			if getErr == nil && proxy != nil && proxy.Host == s.cfg.ProxyHost && proxy.Port == old.Port {
				if proxy.Status == proxySubscriptionInactiveStatus {
					reusedInactiveProxies = append(reusedInactiveProxies, proxy)
				}
				nextEntry.Nodes = append(nextEntry.Nodes, proxySubscriptionRuntimeNode{Fingerprint: parsed.Fingerprint, Name: parsed.Name, SourceURI: parsed.SourceURI, ProxyID: old.ProxyID, Port: old.Port})
				usedPorts[old.Port] = struct{}{}
				result.Reused++
				continue
			}
		}

		port, allocErr := allocateProxySubscriptionPort(s.cfg.PortStart, s.cfg.PortEnd, usedPorts)
		if allocErr != nil {
			s.deactivateProxies(ctx, createdProxies)
			return nil, infraerrors.New(http.StatusConflict, "PROXY_SUBSCRIPTION_PORTS_EXHAUSTED", allocErr.Error())
		}
		created, createErr := s.admin.CreateProxy(ctx, &CreateProxyInput{
			Name:           buildProxySubscriptionProxyName(name, parsed.Name, parsed.Fingerprint),
			Protocol:       "http",
			Host:           s.cfg.ProxyHost,
			Port:           port,
			FallbackMode:   FallbackModeNone,
			ExpiryWarnDays: 0,
		})
		if createErr != nil {
			s.deactivateProxies(ctx, createdProxies)
			return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_PROXY_CREATE_FAILED", "Failed to create imported proxy records").WithCause(createErr)
		}
		createdProxies = append(createdProxies, created)
		usedPorts[port] = struct{}{}
		nextEntry.Nodes = append(nextEntry.Nodes, proxySubscriptionRuntimeNode{Fingerprint: parsed.Fingerprint, Name: parsed.Name, SourceURI: parsed.SourceURI, ProxyID: created.ID, Port: port})
		result.Created++
	}

	removed := make([]proxySubscriptionRuntimeNode, 0)
	nextFingerprints := make(map[string]struct{}, len(nextEntry.Nodes))
	for _, node := range nextEntry.Nodes {
		nextFingerprints[node.Fingerprint] = struct{}{}
	}
	for _, node := range previous.Nodes {
		if _, exists := nextFingerprints[node.Fingerprint]; !exists {
			removed = append(removed, node)
		}
	}

	if entryIndex >= 0 {
		state.Subscriptions[entryIndex] = nextEntry
	} else {
		state.Subscriptions = append(state.Subscriptions, nextEntry)
	}
	sort.Slice(state.Subscriptions, func(i, j int) bool { return state.Subscriptions[i].ID < state.Subscriptions[j].ID })
	configBytes, err := renderProxySubscriptionMihomoConfig(state)
	if err != nil {
		s.deactivateProxies(ctx, createdProxies)
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_CONFIG_FAILED", "Failed to generate proxy runtime configuration").WithCause(err)
	}
	if err := atomicWriteProxySubscriptionFile(s.cfg.ConfigPath, configBytes, 0o600); err != nil {
		s.deactivateProxies(ctx, createdProxies)
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_CONFIG_WRITE_FAILED", "Failed to persist proxy runtime configuration").WithCause(err)
	}
	if err := s.reloadMihomo(ctx, controllerAuthSecret); err != nil {
		s.restoreConfig(ctx, previousConfig, controllerAuthSecret)
		s.deactivateProxies(ctx, createdProxies)
		return nil, infraerrors.New(http.StatusBadGateway, "PROXY_SUBSCRIPTION_RUNTIME_RELOAD_FAILED", "Mihomo rejected the imported subscription").WithCause(err)
	}
	deactivatedRemoved := make([]*Proxy, 0, len(removed))
	for _, node := range removed {
		proxy, getErr := s.admin.GetProxy(ctx, node.ProxyID)
		if getErr != nil || proxy == nil {
			continue
		}
		if updateErr := s.setProxyStatus(ctx, proxy, proxySubscriptionInactiveStatus); updateErr != nil {
			s.restoreConfig(ctx, previousConfig, state.ControllerSecret)
			s.setProxyStatusesBestEffort(ctx, deactivatedRemoved, "active")
			s.deactivateProxies(ctx, createdProxies)
			return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_PROXY_DEACTIVATE_FAILED", "Imported runtime could not deactivate a removed proxy").WithCause(updateErr)
		}
		deactivatedRemoved = append(deactivatedRemoved, proxy)
	}
	reactivatedReused := make([]*Proxy, 0, len(reusedInactiveProxies))
	for _, proxy := range reusedInactiveProxies {
		if updateErr := s.setProxyStatus(ctx, proxy, "active"); updateErr != nil {
			s.restoreConfig(ctx, previousConfig, state.ControllerSecret)
			s.setProxyStatusesBestEffort(ctx, reactivatedReused, proxySubscriptionInactiveStatus)
			s.setProxyStatusesBestEffort(ctx, deactivatedRemoved, "active")
			s.deactivateProxies(ctx, createdProxies)
			return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_PROXY_REACTIVATE_FAILED", "Imported runtime could not reactivate a reused proxy").WithCause(updateErr)
		}
		reactivatedReused = append(reactivatedReused, proxy)
	}
	state.Version = proxySubscriptionStateVersion
	if err := s.saveState(state); err != nil {
		// A successful reload installs state.ControllerSecret. Restoring the old
		// config therefore has to authenticate with the newly installed secret.
		s.restoreConfig(ctx, previousConfig, state.ControllerSecret)
		s.setProxyStatusesBestEffort(ctx, reactivatedReused, proxySubscriptionInactiveStatus)
		s.setProxyStatusesBestEffort(ctx, deactivatedRemoved, "active")
		s.deactivateProxies(ctx, createdProxies)
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_STATE_WRITE_FAILED", "Failed to persist proxy subscription state").WithCause(err)
	}
	result.Deactivated = len(deactivatedRemoved)
	return result, nil
}

func proxySubscriptionControllerSecret(raw []byte) (string, error) {
	var configFile struct {
		Secret string `yaml:"secret"`
	}
	if err := yaml.Unmarshal(raw, &configFile); err != nil {
		return "", err
	}
	return configFile.Secret, nil
}

func (s *ProxySubscriptionService) loadState() (*proxySubscriptionRuntimeState, error) {
	raw, err := os.ReadFile(s.cfg.StatePath)
	if errors.Is(err, os.ErrNotExist) {
		return &proxySubscriptionRuntimeState{Version: proxySubscriptionStateVersion, Subscriptions: []proxySubscriptionRuntimeEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var state proxySubscriptionRuntimeState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if state.Version != proxySubscriptionStateVersion {
		return nil, fmt.Errorf("unsupported proxy subscription state version %d", state.Version)
	}
	return &state, nil
}

func (s *ProxySubscriptionService) saveState(state *proxySubscriptionRuntimeState) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteProxySubscriptionFile(s.cfg.StatePath, append(raw, '\n'), 0o600)
}

func renderProxySubscriptionMihomoConfig(state *proxySubscriptionRuntimeState) ([]byte, error) {
	proxies := make([]map[string]any, 0)
	listeners := make([]map[string]any, 0)
	for _, subscription := range state.Subscriptions {
		for _, runtimeNode := range subscription.Nodes {
			parsed, err := parseProxySubscriptionURI(runtimeNode.SourceURI)
			if err != nil {
				return nil, fmt.Errorf("stored node %s is invalid: %w", runtimeNode.Fingerprint, err)
			}
			outboundName := "node-" + runtimeNode.Fingerprint[:16]
			outbound := make(map[string]any, len(parsed.MihomoConfig)+1)
			for key, value := range parsed.MihomoConfig {
				outbound[key] = value
			}
			outbound["name"] = outboundName
			proxies = append(proxies, outbound)
			listeners = append(listeners, map[string]any{
				"name":   "in-" + runtimeNode.Fingerprint[:16],
				"type":   "mixed",
				"port":   runtimeNode.Port,
				"listen": "0.0.0.0",
				"proxy":  outboundName,
			})
		}
	}
	root := map[string]any{
		"allow-lan":           true,
		"bind-address":        "*",
		"mode":                "global",
		"log-level":           "info",
		"ipv6":                false,
		"external-controller": "0.0.0.0:9090",
		"secret":              state.ControllerSecret,
		"proxies":             proxies,
		"listeners":           listeners,
	}
	return yaml.Marshal(root)
}

func (s *ProxySubscriptionService) reloadMihomo(ctx context.Context, secret string) error {
	body, err := json.Marshal(map[string]any{"path": s.cfg.ControllerConfigPath})
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(s.cfg.ControllerURL, "/") + "/configs?force=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := s.controllerClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mihomo controller returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *ProxySubscriptionService) restoreConfig(ctx context.Context, previous []byte, secret string) {
	if len(previous) == 0 {
		return
	}
	if atomicWriteProxySubscriptionFile(s.cfg.ConfigPath, previous, 0o600) == nil {
		_ = s.reloadMihomo(ctx, secret)
	}
}

func (s *ProxySubscriptionService) deactivateProxies(ctx context.Context, proxies []*Proxy) {
	s.setProxyStatusesBestEffort(ctx, proxies, proxySubscriptionInactiveStatus)
}

func (s *ProxySubscriptionService) setProxyStatusesBestEffort(ctx context.Context, proxies []*Proxy, status string) {
	for _, proxy := range proxies {
		if proxy != nil {
			_ = s.setProxyStatus(ctx, proxy, status)
		}
	}
}

func (s *ProxySubscriptionService) setProxyStatus(ctx context.Context, proxy *Proxy, status string) error {
	_, err := s.admin.UpdateProxy(ctx, proxy.ID, &UpdateProxyInput{
		Name: proxy.Name, Protocol: proxy.Protocol, Host: proxy.Host, Port: proxy.Port,
		Username: proxy.Username, Password: proxy.Password, Status: status,
		ExpiresAt: proxy.ExpiresAt, FallbackMode: proxy.FallbackMode,
		BackupProxyID: proxy.BackupProxyID, ExpiryWarnDays: proxy.ExpiryWarnDays,
	})
	return err
}

func validateProxySubscriptionURL(ctx context.Context, u *url.URL) error {
	if u == nil || !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("subscription URL must use HTTPS")
	}
	if u.User != nil {
		return fmt.Errorf("subscription URL must not contain userinfo")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("subscription URL host is required")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return fmt.Errorf("subscription URL host cannot be resolved")
	}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("subscription URL resolves to a private or local address")
		}
	}
	return nil
}

func allocateProxySubscriptionPort(start, end int, used map[int]struct{}) (int, error) {
	if start < 1 || end > 65535 || start > end {
		return 0, fmt.Errorf("proxy subscription port range is invalid")
	}
	for port := start; port <= end; port++ {
		if _, exists := used[port]; !exists {
			return port, nil
		}
	}
	return 0, fmt.Errorf("proxy subscription port range %d-%d is exhausted", start, end)
}

func randomProxySubscriptionSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func atomicWriteProxySubscriptionFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".proxy-subscription-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func sanitizeProxySubscriptionName(value string) string {
	value = sanitizeProxySubscriptionNodeName(value)
	runes := []rune(value)
	if len(runes) > 48 {
		value = string(runes[:48])
	}
	return value
}

func buildProxySubscriptionProxyName(subscription, node, fingerprint string) string {
	name := subscription + " / " + node + " / " + fingerprint[:8]
	runes := []rune(name)
	if len(runes) > 100 {
		name = string(runes[:91]) + " / " + fingerprint[:8]
	}
	return name
}
