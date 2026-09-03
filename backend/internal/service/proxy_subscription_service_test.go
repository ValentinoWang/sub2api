package service

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProxySubscriptionFetchIgnoresEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("subscription"))
		require.NoError(t, err)
	}))
	defer server.Close()

	service := NewProxySubscriptionService(nil, nil)
	transport, ok := service.fetchClient.Transport.(*http.Transport)
	require.True(t, ok)
	require.Nil(t, transport.Proxy)

	sourceURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	body, err := service.fetch(context.Background(), sourceURL)
	require.NoError(t, err)
	require.Equal(t, []byte("subscription"), body)
}

func TestProxySubscriptionFetchRetriesTruncatedResponse(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("Content-Length", "20")
			_, err := w.Write([]byte("partial"))
			require.NoError(t, err)
			return
		}
		_, err := w.Write([]byte("subscription"))
		require.NoError(t, err)
	}))
	defer server.Close()

	service := NewProxySubscriptionService(nil, nil)
	sourceURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	body, err := service.fetch(context.Background(), sourceURL)
	require.NoError(t, err)
	require.Equal(t, []byte("subscription"), body)
	require.EqualValues(t, 2, requests.Load())
}

func TestProxySubscriptionFetchDoesNotRetryClientError(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "invalid subscription", http.StatusBadRequest)
	}))
	defer server.Close()

	service := NewProxySubscriptionService(nil, nil)
	sourceURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	_, err = service.fetch(context.Background(), sourceURL)
	require.EqualError(t, err, "subscription server returned HTTP 400")
	require.EqualValues(t, 1, requests.Load())
}

func TestProxySubscriptionFetchStopsRetryingWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests atomic.Int32
	service := NewProxySubscriptionService(nil, nil)
	service.fetchClient = &http.Client{Transport: proxySubscriptionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		cancel()
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})}
	sourceURL, err := url.Parse("https://subscription.example.test/list")
	require.NoError(t, err)
	_, err = service.fetch(ctx, sourceURL)
	require.ErrorIs(t, err, context.Canceled)
	require.EqualValues(t, 1, requests.Load())
}

type proxySubscriptionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f proxySubscriptionRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestReservedProxySubscriptionPortsPreventImmediateReuse(t *testing.T) {
	state := &proxySubscriptionRuntimeState{Subscriptions: []proxySubscriptionRuntimeEntry{
		{Nodes: []proxySubscriptionRuntimeNode{{Port: 20000}, {Port: 20001}}},
	}}

	usedPorts := reservedProxySubscriptionPorts(state)
	port, err := allocateProxySubscriptionPort(20000, 20002, usedPorts)
	require.NoError(t, err)
	require.Equal(t, 20002, port)
}

func TestRenderProxySubscriptionMihomoConfig(t *testing.T) {
	state := proxySubscriptionTestState(t)
	raw, err := renderProxySubscriptionMihomoConfig(state)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &config))
	require.Equal(t, "controller-test-secret", config["secret"])
	require.Len(t, config["proxies"], 3)
	require.Len(t, config["listeners"], 3)

	listeners := config["listeners"].([]any)
	first := listeners[0].(map[string]any)
	require.Equal(t, 22000, first["port"])
	require.Equal(t, "0.0.0.0", first["listen"])
}

func TestAtomicWriteProxySubscriptionFileUsesPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	require.NoError(t, atomicWriteProxySubscriptionFile(path, []byte("secret\n"), 0o600))

	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestProxySubscriptionControllerSecretUsesPersistedRuntimeConfig(t *testing.T) {
	secret, err := proxySubscriptionControllerSecret([]byte("secret: runtime-secret\nlisteners: []\n"))
	require.NoError(t, err)
	require.Equal(t, "runtime-secret", secret)
}

func TestRenderedProxySubscriptionConfigAcceptedByMihomo(t *testing.T) {
	bin := os.Getenv("MIHOMO_BIN")
	if bin == "" {
		t.Skip("MIHOMO_BIN is not set")
	}
	raw, err := renderProxySubscriptionMihomoConfig(proxySubscriptionTestState(t))
	require.NoError(t, err)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, raw, 0o600))
	output, err := exec.Command(bin, "-t", "-d", dir, "-f", configPath).CombinedOutput()
	require.NoError(t, err, "mihomo rejected generated config: %s", output)
}

func TestRenderedProxySubscriptionFixtureAcceptedByMihomo(t *testing.T) {
	bin := os.Getenv("MIHOMO_BIN")
	fixturePath := os.Getenv("PROXY_SUBSCRIPTION_FIXTURE_FILE")
	if bin == "" || fixturePath == "" {
		t.Skip("MIHOMO_BIN and PROXY_SUBSCRIPTION_FIXTURE_FILE are required")
	}
	raw, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	parsed, err := ParseProxySubscription(raw)
	require.NoError(t, err)

	nodes := make([]proxySubscriptionRuntimeNode, 0, len(parsed))
	for i, node := range parsed {
		nodes = append(nodes, proxySubscriptionRuntimeNode{
			Fingerprint: node.Fingerprint,
			Name:        node.Name,
			SourceURI:   node.SourceURI,
			ProxyID:     int64(i + 1),
			Port:        22000 + i,
		})
	}
	state := &proxySubscriptionRuntimeState{
		Version:          proxySubscriptionStateVersion,
		ControllerSecret: "controller-test-secret",
		Subscriptions: []proxySubscriptionRuntimeEntry{{
			ID:    "fixture-subscription",
			Name:  "fixture",
			Nodes: nodes,
		}},
	}
	config, err := renderProxySubscriptionMihomoConfig(state)
	require.NoError(t, err)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, config, 0o600))
	output, err := exec.Command(bin, "-t", "-d", dir, "-f", configPath).CombinedOutput()
	require.NoError(t, err, "mihomo rejected fixture config: %s", output)
}

func proxySubscriptionTestState(t *testing.T) *proxySubscriptionRuntimeState {
	t.Helper()
	lines := []string{
		"vless://11111111-1111-1111-1111-111111111111@ws.example.test:443?security=tls&type=ws&sni=edge.example.test&host=edge.example.test&path=%2Fws&ech=dns.example.test#WS",
		"hysteria2://fake-password@hy2.example.test:8443?sni=hy2.example.test&insecure=1&mport=20000-30000#HY2",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:fake-password")) + "@ss.example.test:8388#SS",
	}
	nodes := make([]proxySubscriptionRuntimeNode, 0, len(lines))
	for i, line := range lines {
		parsed, err := parseProxySubscriptionURI(line)
		require.NoError(t, err)
		nodes = append(nodes, proxySubscriptionRuntimeNode{
			Fingerprint: parsed.Fingerprint,
			Name:        parsed.Name,
			SourceURI:   parsed.SourceURI,
			ProxyID:     int64(i + 1),
			Port:        22000 + i,
		})
	}
	return &proxySubscriptionRuntimeState{
		Version:          proxySubscriptionStateVersion,
		ControllerSecret: "controller-test-secret",
		Subscriptions: []proxySubscriptionRuntimeEntry{{
			ID:    "test-subscription",
			Name:  "test",
			Nodes: nodes,
		}},
	}
}
