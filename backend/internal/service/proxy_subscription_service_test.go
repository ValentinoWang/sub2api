package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRenderProxySubscriptionMihomoConfig(t *testing.T) {
	state := proxySubscriptionTestState(t)
	raw, err := renderProxySubscriptionMihomoConfig(state)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &config))
	require.Equal(t, "controller-test-secret", config["secret"])
	require.Len(t, config["proxies"], 2)
	require.Len(t, config["listeners"], 2)

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
