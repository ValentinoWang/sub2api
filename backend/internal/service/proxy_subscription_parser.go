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
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid URI: %w", err)
	}
	if strings.EqualFold(u.Scheme, "ss") {
		return parseShadowsocksSubscriptionURI(raw, u)
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
		err = fmt.Errorf("unsupported protocol %q; supported protocols are vless, hysteria2, and shadowsocks", u.Scheme)
	}
	if err != nil {
		return ProxySubscriptionNode{}, err
	}
	return ProxySubscriptionNode{Name: name, Fingerprint: fingerprint, SourceURI: strings.TrimSpace(raw), MihomoConfig: config}, nil
}

func parseShadowsocksSubscriptionURI(raw string, parsed *url.URL) (ProxySubscriptionNode, error) {
	schemeSeparator := strings.Index(raw, "://")
	if schemeSeparator < 0 {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid shadowsocks URI scheme separator")
	}
	payload := raw[schemeSeparator+3:]
	if fragmentAt := strings.IndexByte(payload, '#'); fragmentAt >= 0 {
		payload = payload[:fragmentAt]
	}
	if queryAt := strings.IndexByte(payload, '?'); queryAt >= 0 {
		payload = payload[:queryAt]
	}

	var credentials, endpoint string
	if separatorAt := strings.LastIndexByte(payload, '@'); separatorAt >= 0 {
		decoded, err := decodeShadowsocksBase64(payload[:separatorAt])
		if err != nil {
			return ProxySubscriptionNode{}, fmt.Errorf("invalid shadowsocks credentials: %w", err)
		}
		credentials = decoded
		endpoint = payload[separatorAt+1:]
	} else {
		decoded, err := decodeShadowsocksBase64(payload)
		if err != nil {
			return ProxySubscriptionNode{}, fmt.Errorf("invalid legacy shadowsocks URI: %w", err)
		}
		separatorAt := strings.LastIndexByte(decoded, '@')
		if separatorAt < 0 {
			return ProxySubscriptionNode{}, fmt.Errorf("legacy shadowsocks URI is missing server endpoint")
		}
		credentials = decoded[:separatorAt]
		endpoint = decoded[separatorAt+1:]
	}

	credentialParts := strings.SplitN(credentials, ":", 2)
	if len(credentialParts) != 2 || strings.TrimSpace(credentialParts[0]) == "" || credentialParts[1] == "" {
		return ProxySubscriptionNode{}, fmt.Errorf("shadowsocks cipher or password is missing")
	}
	endpointURL, err := url.Parse("//" + endpoint)
	if err != nil {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid shadowsocks server endpoint: %w", err)
	}
	if endpointURL.Hostname() == "" {
		return ProxySubscriptionNode{}, fmt.Errorf("shadowsocks server host is missing")
	}
	if net.ParseIP(endpointURL.Hostname()) == nil && strings.ContainsAny(endpointURL.Hostname(), " /\\") {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid shadowsocks server host")
	}
	port, err := strconv.Atoi(endpointURL.Port())
	if err != nil || port < 1 || port > 65535 {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid shadowsocks server port")
	}
	if plugin := strings.TrimSpace(parsed.Query().Get("plugin")); plugin != "" {
		return ProxySubscriptionNode{}, fmt.Errorf("unsupported shadowsocks plugin")
	}

	name, err := url.PathUnescape(parsed.Fragment)
	if err != nil {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid node name encoding")
	}
	name = sanitizeProxySubscriptionNodeName(name)
	if name == "" {
		name = fmt.Sprintf("ss-%s-%d", endpointURL.Hostname(), port)
	}
	hash := sha256.Sum256([]byte(raw))
	return ProxySubscriptionNode{
		Name:        name,
		Fingerprint: hex.EncodeToString(hash[:]),
		SourceURI:   raw,
		MihomoConfig: map[string]any{
			"type":     "ss",
			"server":   endpointURL.Hostname(),
			"port":     port,
			"cipher":   strings.TrimSpace(credentialParts[0]),
			"password": credentialParts[1],
			"udp":      true,
		},
	}, nil
}

func decodeShadowsocksBase64(encoded string) (string, error) {
	encoded, err := url.PathUnescape(strings.TrimSpace(encoded))
	if err != nil {
		return "", fmt.Errorf("invalid Base64 escaping")
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, decodeErr := encoding.DecodeString(encoded)
		if decodeErr == nil && len(decoded) > 0 {
			return string(decoded), nil
		}
	}
	return "", fmt.Errorf("credentials are not valid Base64")
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
