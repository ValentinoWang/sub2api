package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxProxySubscriptionBytes = 2 << 20
	maxProxySubscriptionNodes = 500
)

// ProxySubscriptionNode is a validated outbound that can be rendered into a
// Mihomo runtime configuration. MihomoConfig contains credentials and must not
// be logged or returned by an API handler.
type ProxySubscriptionNode struct {
	Name         string
	Fingerprint  string
	SourceURI    string
	MihomoConfig map[string]any
}

// ParseProxySubscription accepts a Base64 subscription or a plain URI list.
// Unsupported or malformed lines fail the entire import so the UI cannot
// report success for a partially usable subscription.
func ParseProxySubscription(raw []byte) ([]ProxySubscriptionNode, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("proxy subscription is empty")
	}
	if len(raw) > maxProxySubscriptionBytes {
		return nil, fmt.Errorf("proxy subscription exceeds %d bytes", maxProxySubscriptionBytes)
	}

	text, err := decodeProxySubscription(raw)
	if err != nil {
		return nil, err
	}
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	if len(lines) == 0 {
		return nil, fmt.Errorf("proxy subscription contains no nodes")
	}
	if len(lines) > maxProxySubscriptionNodes {
		return nil, fmt.Errorf("proxy subscription contains %d nodes; maximum is %d", len(lines), maxProxySubscriptionNodes)
	}

	nodes := make([]ProxySubscriptionNode, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		node, err := parseProxySubscriptionURI(line)
		if err != nil {
			return nil, fmt.Errorf("proxy subscription node %d: %w", i+1, err)
		}
		if _, exists := seen[node.Fingerprint]; exists {
			continue
		}
		seen[node.Fingerprint] = struct{}{}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("proxy subscription contains no unique nodes")
	}
	return nodes, nil
}

func decodeProxySubscription(raw []byte) (string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
	if strings.Contains(trimmed, "://") {
		return trimmed, nil
	}

	compact := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, trimmed)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(compact)
		if err == nil {
			text := strings.TrimSpace(string(decoded))
			if strings.Contains(text, "://") {
				return text, nil
			}
		}
	}
	return "", fmt.Errorf("proxy subscription is neither a URI list nor valid Base64")
}

func parseProxySubscriptionURI(raw string) (ProxySubscriptionNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid URI: %w", err)
	}
	if u.Hostname() == "" {
		return ProxySubscriptionNode{}, fmt.Errorf("missing server host")
	}
	if net.ParseIP(u.Hostname()) == nil && strings.ContainsAny(u.Hostname(), " /\\") {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid server host")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid server port")
	}

	hash := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	fingerprint := hex.EncodeToString(hash[:])
	name, err := url.PathUnescape(u.Fragment)
	if err != nil {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid node name encoding")
	}
	name = sanitizeProxySubscriptionNodeName(name)
	if name == "" {
		name = fmt.Sprintf("%s-%s-%d", strings.ToLower(u.Scheme), u.Hostname(), port)
	}

	var config map[string]any
	switch strings.ToLower(u.Scheme) {
	case "vless":
		config, err = parseVLESSMihomoConfig(u)
	case "hysteria2", "hy2":
		config, err = parseHysteria2MihomoConfig(u)
	default:
		err = fmt.Errorf("unsupported protocol %q; supported protocols are vless and hysteria2", u.Scheme)
	}
	if err != nil {
		return ProxySubscriptionNode{}, err
	}
	return ProxySubscriptionNode{Name: name, Fingerprint: fingerprint, SourceURI: strings.TrimSpace(raw), MihomoConfig: config}, nil
}

func parseVLESSMihomoConfig(u *url.URL) (map[string]any, error) {
	if u.User == nil || strings.TrimSpace(u.User.Username()) == "" {
		return nil, fmt.Errorf("vless UUID is missing")
	}
	q := u.Query()
	security := strings.ToLower(strings.TrimSpace(q.Get("security")))
	if security != "tls" && security != "reality" {
		return nil, fmt.Errorf("unsupported vless security %q", security)
	}
	network := strings.ToLower(strings.TrimSpace(q.Get("type")))
	if network == "" {
		network = "tcp"
	}
	if network != "tcp" && network != "ws" && network != "xhttp" {
		return nil, fmt.Errorf("unsupported vless transport %q", network)
	}

	serverName := firstProxySubscriptionValue(q.Get("sni"), q.Get("servername"))
	if serverName == "" {
		return nil, fmt.Errorf("vless TLS server name is missing")
	}
	config := map[string]any{
		"type":             "vless",
		"server":           u.Hostname(),
		"port":             mustProxySubscriptionPort(u),
		"uuid":             u.User.Username(),
		"network":          network,
		"tls":              true,
		"servername":       serverName,
		"skip-cert-verify": queryBool(q, "insecure", "allowInsecure"),
		"udp":              true,
	}
	if fingerprint := strings.TrimSpace(q.Get("fp")); fingerprint != "" {
		config["client-fingerprint"] = fingerprint
	}

	if security == "reality" {
		publicKey := strings.TrimSpace(q.Get("pbk"))
		if publicKey == "" {
			return nil, fmt.Errorf("vless reality public key is missing")
		}
		config["reality-opts"] = map[string]any{
			"public-key": publicKey,
			"short-id":   strings.TrimSpace(q.Get("sid")),
		}
		if flow := strings.TrimSpace(q.Get("flow")); flow != "" {
			config["flow"] = flow
		}
	}

	switch network {
	case "ws":
		ws := map[string]any{"path": firstProxySubscriptionValue(q.Get("path"), "/")}
		if host := strings.TrimSpace(q.Get("host")); host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		config["ws-opts"] = ws
	case "xhttp":
		xhttp := map[string]any{
			"mode": firstProxySubscriptionValue(q.Get("mode"), "auto"),
			"path": firstProxySubscriptionValue(q.Get("path"), "/"),
		}
		if host := strings.TrimSpace(q.Get("host")); host != "" {
			xhttp["host"] = host
		}
		config["xhttp-opts"] = xhttp
	}
	if ech := strings.TrimSpace(q.Get("ech")); ech != "" {
		config["ech-opts"] = map[string]any{
			"enable":            true,
			"query-server-name": ech,
		}
	}
	return config, nil
}

func parseHysteria2MihomoConfig(u *url.URL) (map[string]any, error) {
	if u.User == nil || strings.TrimSpace(u.User.String()) == "" {
		return nil, fmt.Errorf("hysteria2 password is missing")
	}
	password, err := url.PathUnescape(u.User.String())
	if err != nil {
		return nil, fmt.Errorf("invalid hysteria2 password encoding")
	}
	q := u.Query()
	config := map[string]any{
		"type":             "hysteria2",
		"server":           u.Hostname(),
		"port":             mustProxySubscriptionPort(u),
		"password":         password,
		"skip-cert-verify": queryBool(q, "insecure", "allowInsecure"),
		"udp":              true,
	}
	if sni := strings.TrimSpace(q.Get("sni")); sni != "" {
		config["sni"] = sni
	}
	if ports := strings.TrimSpace(q.Get("mport")); ports != "" {
		config["mport"] = ports
		config["ports"] = ports
	}
	if obfs := strings.TrimSpace(q.Get("obfs")); obfs != "" {
		config["obfs"] = obfs
		if password := strings.TrimSpace(firstProxySubscriptionValue(q.Get("obfs-password"), q.Get("obfsPassword"))); password != "" {
			config["obfs-password"] = password
		}
	}
	return config, nil
}

func mustProxySubscriptionPort(u *url.URL) int {
	port, _ := strconv.Atoi(u.Port())
	return port
}

func queryBool(values url.Values, keys ...string) bool {
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(values.Get(key))) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func firstProxySubscriptionValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func sanitizeProxySubscriptionNodeName(name string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name))
	const maxRunes = 72
	runes := []rune(name)
	if len(runes) > maxRunes {
		name = string(runes[:maxRunes])
	}
	return name
}
