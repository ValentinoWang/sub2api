package service

import (
	"encoding/base64"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseProxySubscriptionSupportedNodes(t *testing.T) {
	lines := []string{
		"vless://11111111-1111-1111-1111-111111111111@ws.example.test:443?security=tls&type=ws&sni=edge.example.test&host=edge.example.test&path=%2Fws&ech=dns.example.test#WS%20node",
		"vless://22222222-2222-2222-2222-222222222222@reality.example.test:443?security=reality&type=tcp&servername=www.example.com&pbk=fake-public-key&sid=abcd&fp=chrome&flow=xtls-rprx-vision#Reality",
		"vless://33333333-3333-3333-3333-333333333333@xhttp.example.test:443?security=tls&type=xhttp&sni=xhttp.example.test&host=xhttp.example.test&path=%2Fapi&mode=auto&ech=dns.example.test#XHTTP",
		"hysteria2://fake-password@hy2.example.test:8443?sni=hy2.example.test&insecure=1&mport=20000-30000#HY2",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:fake-password")) + "@ss.example.test:8388#SS%20node",
	}
	raw := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))

	nodes, err := ParseProxySubscription([]byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 5)

	ws := nodes[0].MihomoConfig
	require.Equal(t, "vless", ws["type"])
	require.Equal(t, "ws", ws["network"])
	require.Equal(t, "WS node", nodes[0].Name)
	require.Equal(t, map[string]any{"enable": true, "query-server-name": "dns.example.test"}, ws["ech-opts"])

	reality := nodes[1].MihomoConfig
	require.Equal(t, "xtls-rprx-vision", reality["flow"])
	require.Equal(t, map[string]any{"public-key": "fake-public-key", "short-id": "abcd"}, reality["reality-opts"])

	xhttp := nodes[2].MihomoConfig
	require.Equal(t, "xhttp", xhttp["network"])
	require.Equal(t, map[string]any{"host": "xhttp.example.test", "mode": "auto", "path": "/api"}, xhttp["xhttp-opts"])

	hy2 := nodes[3].MihomoConfig
	require.Equal(t, "hysteria2", hy2["type"])
	require.Equal(t, "20000-30000", hy2["ports"])
	require.Equal(t, true, hy2["skip-cert-verify"])

	ss := nodes[4].MihomoConfig
	require.Equal(t, "SS node", nodes[4].Name)
	require.Equal(t, "ss", ss["type"])
	require.Equal(t, "aes-256-gcm", ss["cipher"])
	require.Equal(t, "fake-password", ss["password"])
	require.Equal(t, true, ss["udp"])
}

func TestParseProxySubscriptionLegacyShadowsocksURI(t *testing.T) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:legacy-password@legacy.example.test:8389"))
	nodes, err := ParseProxySubscription([]byte("ss://" + encoded + "#Legacy"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "Legacy", nodes[0].Name)
	require.Equal(t, "legacy.example.test", nodes[0].MihomoConfig["server"])
	require.Equal(t, 8389, nodes[0].MihomoConfig["port"])
}

func TestParseProxySubscriptionFixture(t *testing.T) {
	path := os.Getenv("PROXY_SUBSCRIPTION_FIXTURE_FILE")
	if path == "" {
		t.Skip("PROXY_SUBSCRIPTION_FIXTURE_FILE is not set")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	nodes, err := ParseProxySubscription(raw)
	require.NoError(t, err)
	if expectedText := os.Getenv("PROXY_SUBSCRIPTION_EXPECTED_NODES"); expectedText != "" {
		expected, convertErr := strconv.Atoi(expectedText)
		require.NoError(t, convertErr)
		require.Len(t, nodes, expected)
	}
	t.Logf("parsed %d proxy subscription nodes", len(nodes))
}

func TestParseProxySubscriptionRejectsPartialImport(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@ok.example.test:443?security=tls&type=ws&sni=ok.example.test\ntrojan://unsupported@example.test:443"
	_, err := ParseProxySubscription([]byte(raw))
	require.ErrorContains(t, err, "unsupported protocol")
}

func TestParseProxySubscriptionDeduplicatesExactURIs(t *testing.T) {
	line := "hysteria2://fake-password@hy2.example.test:8443?sni=hy2.example.test#node"
	nodes, err := ParseProxySubscription([]byte(line + "\n" + line))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
}
