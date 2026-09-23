package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	proxySubscriptionRefreshOK             = "ok"
	proxySubscriptionRefreshFailed         = "failed"
	proxySubscriptionDefaultRefreshMinutes = 60
	proxySubscriptionMinRefreshMinutes     = 15
	proxySubscriptionMaxRefreshMinutes     = 1440
	proxySubscriptionRefreshTick           = time.Minute
)

// ProvideProxySubscriptionService wires URL encryption and starts automatic refresh.
func ProvideProxySubscriptionService(admin AdminService, cfg *config.Config, encryptor SecretEncryptor) *ProxySubscriptionService {
	svc := NewProxySubscriptionService(admin, cfg)
	svc.encryptor = encryptor
	svc.Start()
	return svc
}

func newProxySubscriptionRuntimeNode(parsed ProxySubscriptionNode, proxyID int64, port int) proxySubscriptionRuntimeNode {
	node := proxySubscriptionRuntimeNode{Fingerprint: parsed.Fingerprint, Name: parsed.Name, SourceURI: parsed.SourceURI, ProxyID: proxyID, Port: port}
	if parsed.SourceURI == "" {
		node.Config = parsed.MihomoConfig
	}
	return node
}

// parse re-validates a stored node before it is rendered into the runtime.
func (n proxySubscriptionRuntimeNode) parse() (ProxySubscriptionNode, error) {
	if n.SourceURI != "" {
		return parseProxySubscriptionURI(n.SourceURI)
	}
	item := make(map[string]any, len(n.Config)+1)
	for key, value := range n.Config {
		item[key] = value
	}
	item["name"] = n.Name
	return parseProxySubscriptionYAMLProxy(item)
}

func validateProxySubscriptionRefreshInterval(minutes int) error {
	if minutes == 0 || (minutes >= proxySubscriptionMinRefreshMinutes && minutes <= proxySubscriptionMaxRefreshMinutes) {
		return nil
	}
	return infraerrors.BadRequest("PROXY_SUBSCRIPTION_REFRESH_INTERVAL_INVALID", "refresh interval must be 0 (off) or between 15 and 1440 minutes")
}

// Refresh re-fetches a subscription with its stored URL and reconciles nodes,
// keeping proxy IDs of unchanged nodes. A failure is recorded on the entry.
func (s *ProxySubscriptionService) Refresh(ctx context.Context, id string) (*ProxySubscriptionImportResult, error) {
	if s == nil || !s.cfg.Enabled {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "PROXY_SUBSCRIPTION_DISABLED", "Proxy subscription runtime is not enabled")
	}
	s.mu.Lock()
	state, err := s.loadState()
	if err != nil {
		s.mu.Unlock()
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_STATE_INVALID", "Proxy subscription state is invalid").WithCause(err)
	}
	entry, ok := findProxySubscriptionEntry(state, id)
	s.mu.Unlock()
	if !ok {
		return nil, infraerrors.NotFound("PROXY_SUBSCRIPTION_NOT_FOUND", "Subscription not found")
	}
	if entry.URLCiphertext == "" || s.encryptor == nil {
		return nil, infraerrors.Conflict("PROXY_SUBSCRIPTION_URL_NOT_STORED", "The subscription URL is not stored; import the subscription again to enable refresh")
	}
	rawURL, err := s.encryptor.Decrypt(entry.URLCiphertext)
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_URL_DECRYPT_FAILED", "The stored subscription URL cannot be decrypted")
	}
	sourceURL, err := url.Parse(rawURL)
	if err == nil {
		err = s.validateURL(ctx, sourceURL)
	}
	var raw []byte
	var header http.Header
	if err == nil {
		raw, header, err = s.fetchWithHeader(ctx, sourceURL)
	}
	var doc ProxySubscriptionDocument
	if err == nil {
		doc, err = ParseProxySubscriptionDocument(raw)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.recordRefreshFailureLocked(id, proxySubscriptionErrorText(err))
		return nil, infraerrors.New(http.StatusBadGateway, "PROXY_SUBSCRIPTION_REFRESH_FAILED", proxySubscriptionErrorText(err))
	}
	result, err := s.importLocked(ctx, entry.Name, sourceURL.String(), doc, proxySubscriptionImportOptions{
		usage: ParseProxySubscriptionUsage(header), interval: entry.RefreshIntervalMinutes, urlCiphertext: entry.URLCiphertext,
	})
	if err != nil {
		s.recordRefreshFailureLocked(id, infraerrors.Message(err))
		return nil, err
	}
	return result, nil
}

func (s *ProxySubscriptionService) recordRefreshFailureLocked(id, message string) {
	state, err := s.loadState()
	if err != nil {
		return
	}
	for i := range state.Subscriptions {
		if state.Subscriptions[i].ID == id {
			now := time.Now().UTC()
			state.Subscriptions[i].LastRefreshAt = &now
			state.Subscriptions[i].LastRefreshStatus = proxySubscriptionRefreshFailed
			state.Subscriptions[i].LastRefreshError = clipCodexHarvestText(message, 200)
			if err := s.saveState(state); err != nil {
				logger.L().Warn("proxy subscription refresh status persist failed", zap.Error(err))
			}
			return
		}
	}
}

// proxySubscriptionErrorText never includes the subscription URL: url.Error
// embeds the full request URL, which carries the provider token.
func proxySubscriptionErrorText(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return "subscription server timed out"
		}
		var netErr net.Error
		if errors.As(urlErr.Err, &netErr) {
			return "subscription server is unreachable"
		}
		return "subscription request failed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "subscription server timed out"
	}
	return clipCodexHarvestText(err.Error(), 200)
}

func findProxySubscriptionEntry(state *proxySubscriptionRuntimeState, id string) (proxySubscriptionRuntimeEntry, bool) {
	for _, entry := range state.Subscriptions {
		if entry.ID == id {
			return entry, true
		}
	}
	return proxySubscriptionRuntimeEntry{}, false
}

// UpdateRefreshInterval changes automatic refresh; 0 turns it off.
func (s *ProxySubscriptionService) UpdateRefreshInterval(_ context.Context, id string, minutes int) error {
	if s == nil || !s.cfg.Enabled {
		return infraerrors.New(http.StatusServiceUnavailable, "PROXY_SUBSCRIPTION_DISABLED", "Proxy subscription runtime is not enabled")
	}
	if err := validateProxySubscriptionRefreshInterval(minutes); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadState()
	if err != nil {
		return infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_STATE_INVALID", "Proxy subscription state is invalid").WithCause(err)
	}
	for i := range state.Subscriptions {
		if state.Subscriptions[i].ID == id {
			state.Subscriptions[i].RefreshIntervalMinutes = minutes
			return s.saveState(state)
		}
	}
	return infraerrors.NotFound("PROXY_SUBSCRIPTION_NOT_FOUND", "Subscription not found")
}

// Start launches the automatic refresh loop; it is a no-op when the runtime is off.
func (s *ProxySubscriptionService) Start() {
	if s == nil || !s.cfg.Enabled {
		return
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopRefresh != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.stopRefresh, s.refreshDone = cancel, done
	go func() {
		defer close(done)
		ticker := time.NewTicker(proxySubscriptionRefreshTick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.refreshDue(ctx, time.Now())
			}
		}
	}()
}

func (s *ProxySubscriptionService) Stop() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	cancel, done := s.stopRefresh, s.refreshDone
	s.stopRefresh, s.refreshDone = nil, nil
	s.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (s *ProxySubscriptionService) refreshDue(ctx context.Context, now time.Time) {
	s.mu.Lock()
	state, err := s.loadState()
	s.mu.Unlock()
	if err != nil {
		logger.L().Warn("proxy subscription auto refresh skipped: state unavailable", zap.Error(err))
		return
	}
	for _, entry := range state.Subscriptions {
		if !proxySubscriptionRefreshDue(entry, now) || ctx.Err() != nil {
			continue
		}
		refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		result, err := s.Refresh(refreshCtx, entry.ID)
		cancel()
		if err != nil {
			logger.L().Warn("proxy subscription auto refresh failed", zap.String("subscription_id", entry.ID), zap.String("reason", infraerrors.Reason(err)))
			continue
		}
		logger.L().Info("proxy subscription auto refreshed", zap.String("subscription_id", entry.ID),
			zap.Int("nodes", result.NodeCount), zap.Int("created", result.Created), zap.Int("deactivated", result.Deactivated))
	}
}

func proxySubscriptionRefreshDue(entry proxySubscriptionRuntimeEntry, now time.Time) bool {
	if entry.URLCiphertext == "" || entry.RefreshIntervalMinutes <= 0 {
		return false
	}
	return entry.LastRefreshAt == nil || !now.Before(entry.LastRefreshAt.Add(time.Duration(entry.RefreshIntervalMinutes)*time.Minute))
}

// ProxySubscriptionNodeView is one imported node without credentials.
type ProxySubscriptionNodeView struct {
	ProxyID int64                     `json:"proxy_id"`
	Name    string                    `json:"name"`
	Info    bool                      `json:"info"`
	Meta    ProxySubscriptionNodeMeta `json:"meta"`
}

// ProxySubscriptionView is the admin view of a subscription; it never
// contains the URL, share links or node credentials.
type ProxySubscriptionView struct {
	ID                     string                      `json:"id"`
	Name                   string                      `json:"name"`
	Format                 string                      `json:"format"`
	UpdatedAt              time.Time                   `json:"updated_at"`
	NodeCount              int                         `json:"node_count"`
	HasURL                 bool                        `json:"has_url"`
	RefreshIntervalMinutes int                         `json:"refresh_interval_minutes"`
	LastRefreshAt          *time.Time                  `json:"last_refresh_at,omitempty"`
	LastRefreshStatus      string                      `json:"last_refresh_status,omitempty"`
	LastRefreshError       string                      `json:"last_refresh_error,omitempty"`
	NextRefreshAt          *time.Time                  `json:"next_refresh_at,omitempty"`
	Usage                  *ProxySubscriptionUsage     `json:"usage,omitempty"`
	Info                   []string                    `json:"info"`
	Groups                 []ProxySubscriptionGroup    `json:"groups"`
	Nodes                  []ProxySubscriptionNodeView `json:"nodes"`
}

// List returns subscriptions with nodes classified and grouped. Groups are
// derived from stored node names, so subscriptions imported before grouping
// existed are grouped too.
func (s *ProxySubscriptionService) List(_ context.Context) ([]ProxySubscriptionView, error) {
	if s == nil || !s.cfg.Enabled {
		return []ProxySubscriptionView{}, nil
	}
	s.mu.Lock()
	state, err := s.loadState()
	s.mu.Unlock()
	if err != nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "PROXY_SUBSCRIPTION_STATE_INVALID", "Proxy subscription state is invalid").WithCause(err)
	}
	views := make([]ProxySubscriptionView, 0, len(state.Subscriptions))
	for _, entry := range state.Subscriptions {
		format := entry.Format
		if format == "" {
			format = ProxySubscriptionFormatURIList
		}
		view := ProxySubscriptionView{
			ID: entry.ID, Name: entry.Name, Format: format, UpdatedAt: entry.UpdatedAt,
			HasURL: entry.URLCiphertext != "", RefreshIntervalMinutes: entry.RefreshIntervalMinutes,
			LastRefreshAt: entry.LastRefreshAt, LastRefreshStatus: entry.LastRefreshStatus, LastRefreshError: entry.LastRefreshError,
			Usage: entry.Usage, Info: append([]string{}, entry.Info...), Nodes: make([]ProxySubscriptionNodeView, 0, len(entry.Nodes)),
		}
		if view.HasURL && entry.RefreshIntervalMinutes > 0 && entry.LastRefreshAt != nil {
			next := entry.LastRefreshAt.Add(time.Duration(entry.RefreshIntervalMinutes) * time.Minute)
			view.NextRefreshAt = &next
		}
		grouped := make([]proxySubscriptionRuntimeNode, 0, len(entry.Nodes))
		metas := make(map[string]ProxySubscriptionNodeMeta, len(entry.Nodes))
		for _, node := range entry.Nodes {
			meta := ClassifyProxySubscriptionNode(node.Name, proxySubscriptionNodeProtocol(node))
			info := isProxySubscriptionInfoEntry(node.Name)
			view.Nodes = append(view.Nodes, ProxySubscriptionNodeView{ProxyID: node.ProxyID, Name: node.Name, Info: info, Meta: meta})
			if info {
				continue
			}
			metas[node.Fingerprint] = meta
			grouped = append(grouped, node)
		}
		view.NodeCount = len(grouped)
		view.Groups = buildProxySubscriptionGroups(grouped, metas, entry.Groups)
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Name < views[j].Name })
	return views, nil
}
