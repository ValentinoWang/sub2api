package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	ProxySubscriptionFormatURIList    = "uri-list"
	ProxySubscriptionFormatMihomoYAML = "mihomo-yaml"

	proxySubscriptionGroupSourceSubscription = "subscription"
	proxySubscriptionGroupSourceDerived      = "derived"

	proxySubscriptionHarvestGroup  = "🎫 打票出口"
	proxySubscriptionBusinessGroup = "💼 业务出口"

	maxProxySubscriptionGroups     = 200
	maxProxySubscriptionInfoLines  = 20
	maxProxySubscriptionGroupDepth = 8
)

// ProxySubscriptionDocument is a parsed subscription: runnable nodes, the
// provider's own proxy groups (YAML only) and informational entries such as
// remaining traffic or expiry that providers disguise as nodes.
type ProxySubscriptionDocument struct {
	Format string
	Nodes  []ProxySubscriptionNode
	Groups []ProxySubscriptionSourceGroup
	Info   []string
}

// ProxySubscriptionSourceGroup is a group declared by the subscription itself,
// resolved to node fingerprints (nested groups are flattened).
type ProxySubscriptionSourceGroup struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Fingerprints []string `json:"fingerprints"`
}

// ProxySubscriptionNodeMeta classifies a node following the Codex_degrade
// naming convention: residential nodes feed the harvest egress, datacenter
// nodes the business egress.
type ProxySubscriptionNodeMeta struct {
	Region      string `json:"region,omitempty"`
	Country     string `json:"country,omitempty"`
	Flag        string `json:"flag,omitempty"`
	Residential bool   `json:"residential"`
	Multiplier  string `json:"multiplier"`
	Route       string `json:"route"`
	Protocol    string `json:"protocol"`
	DisplayName string `json:"display_name"`
}

// ProxySubscriptionUsage mirrors the standard subscription-userinfo header.
type ProxySubscriptionUsage struct {
	Upload   int64      `json:"upload"`
	Download int64      `json:"download"`
	Total    int64      `json:"total"`
	ExpireAt *time.Time `json:"expire_at,omitempty"`
}

// ParseProxySubscriptionDocument accepts a Mihomo/Clash YAML profile with
// inline proxies, or a Base64/plain URI list, and separates info entries.
func ParseProxySubscriptionDocument(raw []byte) (ProxySubscriptionDocument, error) {
	if len(raw) == 0 {
		return ProxySubscriptionDocument{}, fmt.Errorf("proxy subscription is empty")
	}
	if len(raw) > maxProxySubscriptionBytes {
		return ProxySubscriptionDocument{}, fmt.Errorf("proxy subscription exceeds %d bytes", maxProxySubscriptionBytes)
	}
	if doc, ok, err := parseProxySubscriptionYAML(raw); ok || err != nil {
		if err != nil {
			return ProxySubscriptionDocument{}, err
		}
		return doc, nil
	}
	nodes, err := ParseProxySubscription(raw)
	if err != nil {
		return ProxySubscriptionDocument{}, err
	}
	doc := ProxySubscriptionDocument{Format: ProxySubscriptionFormatURIList}
	for _, node := range nodes {
		if isProxySubscriptionInfoEntry(node.Name) {
			doc.Info = appendProxySubscriptionInfo(doc.Info, node.Name)
			continue
		}
		doc.Nodes = append(doc.Nodes, node)
	}
	if len(doc.Nodes) == 0 {
		return ProxySubscriptionDocument{}, fmt.Errorf("proxy subscription contains no usable nodes")
	}
	return doc, nil
}

var proxySubscriptionYAMLTypes = map[string]bool{
	"ss": true, "ssr": true, "vmess": true, "vless": true, "trojan": true,
	"hysteria": true, "hysteria2": true, "tuic": true, "anytls": true,
	"socks5": true, "http": true, "wireguard": true, "snell": true, "mieru": true,
}

// Keys that would let a subscription reach outside its own outbound: chained
// dialers, host interfaces or policy routing marks.
var proxySubscriptionYAMLBlockedKeys = []string{"dialer-proxy", "interface-name", "routing-mark"}

func parseProxySubscriptionYAML(raw []byte) (ProxySubscriptionDocument, bool, error) {
	text := strings.TrimPrefix(string(raw), "\ufeff")
	var profile struct {
		Proxies     []map[string]any `yaml:"proxies"`
		ProxyGroups []map[string]any `yaml:"proxy-groups"`
	}
	var probe map[string]any
	if err := yaml.Unmarshal([]byte(text), &probe); err != nil || probe == nil {
		return ProxySubscriptionDocument{}, false, nil
	}
	_, hasProxies := probe["proxies"]
	_, hasProviders := probe["proxy-providers"]
	if !hasProxies && !hasProviders {
		return ProxySubscriptionDocument{}, false, nil
	}
	if err := yaml.Unmarshal([]byte(text), &profile); err != nil {
		return ProxySubscriptionDocument{}, true, fmt.Errorf("invalid Mihomo profile: %w", err)
	}
	if len(profile.Proxies) == 0 {
		if hasProviders {
			return ProxySubscriptionDocument{}, true, fmt.Errorf("proxy-providers are not supported; the profile must contain inline proxies")
		}
		return ProxySubscriptionDocument{}, true, fmt.Errorf("proxy subscription contains no nodes")
	}
	if len(profile.Proxies) > maxProxySubscriptionNodes {
		return ProxySubscriptionDocument{}, true, fmt.Errorf("proxy subscription contains %d nodes; maximum is %d", len(profile.Proxies), maxProxySubscriptionNodes)
	}
	doc := ProxySubscriptionDocument{Format: ProxySubscriptionFormatMihomoYAML}
	byName := map[string]string{}
	seen := map[string]struct{}{}
	for i, item := range profile.Proxies {
		node, err := parseProxySubscriptionYAMLProxy(item)
		if err != nil {
			return ProxySubscriptionDocument{}, true, fmt.Errorf("proxy subscription node %d: %w", i+1, err)
		}
		if isProxySubscriptionInfoEntry(node.Name) {
			doc.Info = appendProxySubscriptionInfo(doc.Info, node.Name)
			continue
		}
		byName[node.Name] = node.Fingerprint
		if _, dup := seen[node.Fingerprint]; dup {
			continue
		}
		seen[node.Fingerprint] = struct{}{}
		doc.Nodes = append(doc.Nodes, node)
	}
	if len(doc.Nodes) == 0 {
		return ProxySubscriptionDocument{}, true, fmt.Errorf("proxy subscription contains no usable nodes")
	}
	doc.Groups = resolveProxySubscriptionYAMLGroups(profile.ProxyGroups, byName)
	return doc, true, nil
}

func parseProxySubscriptionYAMLProxy(item map[string]any) (ProxySubscriptionNode, error) {
	name, _ := item["name"].(string)
	name = sanitizeProxySubscriptionNodeName(name)
	proxyType, _ := item["type"].(string)
	proxyType = strings.ToLower(strings.TrimSpace(proxyType))
	if !proxySubscriptionYAMLTypes[proxyType] {
		return ProxySubscriptionNode{}, fmt.Errorf("unsupported proxy type %q", proxyType)
	}
	server, _ := item["server"].(string)
	server = strings.TrimSpace(server)
	if server == "" || strings.ContainsAny(server, " /\\") {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid server host")
	}
	port, ok := proxySubscriptionYAMLPort(item["port"])
	if !ok {
		return ProxySubscriptionNode{}, fmt.Errorf("invalid server port")
	}
	config := make(map[string]any, len(item))
	for key, value := range item {
		if key == "name" {
			continue
		}
		config[key] = value
	}
	for _, key := range proxySubscriptionYAMLBlockedKeys {
		delete(config, key)
	}
	config["type"] = proxyType
	config["server"] = server
	config["port"] = port
	canonical, err := json.Marshal(config)
	if err != nil {
		return ProxySubscriptionNode{}, fmt.Errorf("proxy settings cannot be encoded")
	}
	hash := sha256.Sum256(canonical)
	if name == "" {
		name = fmt.Sprintf("%s-%s-%d", proxyType, server, port)
	}
	return ProxySubscriptionNode{Name: name, Fingerprint: hex.EncodeToString(hash[:]), MihomoConfig: config}, nil
}

func proxySubscriptionYAMLPort(value any) (int, bool) {
	var port int
	switch v := value.(type) {
	case int:
		port = v
	case int64:
		port = int(v)
	case float64:
		port = int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		port = parsed
	default:
		return 0, false
	}
	return port, port >= 1 && port <= 65535
}

// resolveProxySubscriptionYAMLGroups flattens nested groups to node
// fingerprints. Built-in policies (DIRECT, REJECT) and unknown names drop out.
func resolveProxySubscriptionYAMLGroups(groups []map[string]any, byName map[string]string) []ProxySubscriptionSourceGroup {
	declared := map[string][]string{}
	order := make([]string, 0, len(groups))
	types := map[string]string{}
	for _, group := range groups {
		name, _ := group["name"].(string)
		name = sanitizeProxySubscriptionNodeName(name)
		if name == "" {
			continue
		}
		members := []string{}
		if list, ok := group["proxies"].([]any); ok {
			for _, member := range list {
				if value, ok := member.(string); ok {
					members = append(members, sanitizeProxySubscriptionNodeName(value))
				}
			}
		}
		if _, exists := declared[name]; !exists {
			order = append(order, name)
		}
		declared[name] = members
		groupType, _ := group["type"].(string)
		types[name] = strings.ToLower(strings.TrimSpace(groupType))
	}
	var resolve func(name string, depth int, seen map[string]bool) []string
	resolve = func(name string, depth int, seen map[string]bool) []string {
		if fingerprint, ok := byName[name]; ok {
			return []string{fingerprint}
		}
		members, ok := declared[name]
		if !ok || depth > maxProxySubscriptionGroupDepth || seen[name] {
			return nil
		}
		seen[name] = true
		out := []string{}
		for _, member := range members {
			out = append(out, resolve(member, depth+1, seen)...)
		}
		delete(seen, name)
		return out
	}
	result := make([]ProxySubscriptionSourceGroup, 0, len(order))
	for _, name := range order {
		fingerprints := uniqueProxySubscriptionStrings(resolve(name, 0, map[string]bool{}))
		if len(fingerprints) == 0 {
			continue
		}
		result = append(result, ProxySubscriptionSourceGroup{Name: name, Type: types[name], Fingerprints: fingerprints})
		if len(result) >= maxProxySubscriptionGroups {
			break
		}
	}
	return result
}

func uniqueProxySubscriptionStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// Providers publish remaining traffic, expiry and their website as fake nodes.
var proxySubscriptionInfoPattern = regexp.MustCompile(`(?i)(剩余流量|流量剩余|已用流量|套餐到期|到期时间|过期时间|距离下次重置|下次重置|重置剩余|官网|发布页|防失联|网址|traffic|expire|官方|客服|tg群|telegram)`)

func isProxySubscriptionInfoEntry(name string) bool {
	return proxySubscriptionInfoPattern.MatchString(name)
}

func appendProxySubscriptionInfo(info []string, line string) []string {
	if len(info) >= maxProxySubscriptionInfoLines {
		return info
	}
	return append(info, line)
}

type proxySubscriptionRegion struct {
	code    string
	country string
	words   []string
}

// Keyword matches win over flags: providers often tag Taiwan nodes with 🇨🇳.
var proxySubscriptionRegions = []proxySubscriptionRegion{
	{"HK", "香港", []string{"香港", "hong kong", "hongkong"}},
	{"TW", "台湾", []string{"台湾", "臺灣", "台灣", "taiwan", "hinet", "seednet"}},
	{"JP", "日本", []string{"日本", "japan", "东京", "東京", "大阪", "tokyo", "osaka"}},
	{"SG", "新加坡", []string{"新加坡", "狮城", "singapore"}},
	{"KR", "韩国", []string{"韩国", "韓國", "korea", "首尔", "seoul"}},
	{"US", "美国", []string{"美国", "美國", "united states", "洛杉矶", "圣何塞", "硅谷", "西雅图", "纽约", "达拉斯", "芝加哥", "克利夫兰", "雪城", "斯托克顿", "布罗格登"}},
	{"GB", "英国", []string{"英国", "英國", "united kingdom", "伦敦", "london"}},
	{"DE", "德国", []string{"德国", "德國", "germany", "法兰克福", "frankfurt"}},
	{"FR", "法国", []string{"法国", "法國", "france", "巴黎"}},
	{"NL", "荷兰", []string{"荷兰", "荷蘭", "netherlands", "阿姆斯特丹"}},
	{"CA", "加拿大", []string{"加拿大", "canada", "多伦多"}},
	{"AU", "澳大利亚", []string{"澳大利亚", "澳洲", "australia", "悉尼"}},
	{"TH", "泰国", []string{"泰国", "泰國", "thailand", "曼谷"}},
	{"VN", "越南", []string{"越南", "vietnam"}},
	{"MY", "马来西亚", []string{"马来西亚", "malaysia"}},
	{"PH", "菲律宾", []string{"菲律宾", "philippines"}},
	{"IN", "印度", []string{"印度", "india"}},
	{"TR", "土耳其", []string{"土耳其", "turkey"}},
	{"RU", "俄罗斯", []string{"俄罗斯", "russia", "莫斯科"}},
	{"BR", "巴西", []string{"巴西", "brazil"}},
	{"AR", "阿根廷", []string{"阿根廷", "argentina"}},
	{"NG", "尼日利亚", []string{"尼日利亚", "nigeria"}},
	{"CN", "中国", []string{"中国", "china"}},
}

var (
	proxySubscriptionResidentialPattern = regexp.MustCompile(`(?i)(家宽|家庭宽带|住宅|原生|residential|\bisp\b|静态\s*isp)`)
	proxySubscriptionMultiplierPattern  = regexp.MustCompile(`(?i)[\[【(（]?\s*(\d+(?:\.\d+)?)\s*[x×倍]\s*[\]】)）]?`)
	proxySubscriptionNoisePattern       = regexp.MustCompile(`(?i)(优秀\s*[|｜]|[\[【(（]\s*\d+(?:\.\d+)?\s*[x×倍]\s*[\]】)）]|\bhy2\b|\bss\b)`)
	proxySubscriptionSeparatorPattern   = regexp.MustCompile(`[|｜/\\]+`)
	proxySubscriptionDashPattern        = regexp.MustCompile(`[-\s]*-[-\s]*`)
)

var proxySubscriptionProtocolTitles = map[string]string{
	"vless": "Vless", "vmess": "Vmess", "trojan": "Trojan", "ss": "Shadowsocks", "ssr": "ShadowsocksR",
	"hysteria": "Hysteria", "hysteria2": "Hysteria2", "tuic": "Tuic", "anytls": "AnyTLS",
	"socks5": "Socks5", "http": "HTTP", "wireguard": "WireGuard", "snell": "Snell", "mieru": "Mieru",
}

func proxySubscriptionNodeProtocol(node proxySubscriptionRuntimeNode) string {
	if node.SourceURI != "" {
		scheme := strings.ToLower(strings.SplitN(node.SourceURI, "://", 2)[0])
		if scheme == "hy2" {
			return "hysteria2"
		}
		return scheme
	}
	if value, ok := node.Config["type"].(string); ok {
		return strings.ToLower(value)
	}
	return ""
}

// ClassifyProxySubscriptionNode derives region, residential/datacenter,
// multiplier and route from the provider's node name, and builds a display
// name in the Codex_degrade style:
//
//	residential: <flag><country>-<description>-住宅IP
//	datacenter:  <flag><country>-<description>-机房-<Protocol>
func ClassifyProxySubscriptionNode(name, protocol string) ProxySubscriptionNodeMeta {
	meta := ProxySubscriptionNodeMeta{Protocol: protocol, Multiplier: "1×", Route: "直连"}
	lower := strings.ToLower(name)
	for _, region := range proxySubscriptionRegions {
		for _, word := range region.words {
			if strings.Contains(lower, strings.ToLower(word)) {
				meta.Region, meta.Country = region.code, region.country
				break
			}
		}
		if meta.Region != "" {
			break
		}
	}
	if meta.Region == "" {
		if code := proxySubscriptionFlagCode(name); code != "" {
			meta.Region = code
			for _, region := range proxySubscriptionRegions {
				if region.code == code {
					meta.Country = region.country
				}
			}
			if meta.Country == "" {
				meta.Country = code
			}
		}
	}
	if meta.Region != "" {
		meta.Flag = proxySubscriptionFlag(meta.Region)
	}
	meta.Residential = proxySubscriptionResidentialPattern.MatchString(name)
	if match := proxySubscriptionMultiplierPattern.FindStringSubmatch(name); match != nil {
		meta.Multiplier = strings.TrimSuffix(strings.TrimSuffix(match[1], ".0"), ".00") + "×"
	}
	switch {
	case strings.Contains(lower, "cf加速") || strings.Contains(lower, "cloudflare"):
		meta.Route = "CF"
	case strings.Contains(name, "中转"):
		meta.Route = "中转"
	}
	meta.DisplayName = proxySubscriptionDisplayName(name, meta)
	return meta
}

func proxySubscriptionDisplayName(name string, meta ProxySubscriptionNodeMeta) string {
	description := stripProxySubscriptionFlags(name)
	description = proxySubscriptionNoisePattern.ReplaceAllString(description, "")
	description = proxySubscriptionSeparatorPattern.ReplaceAllString(description, "-")
	description = proxySubscriptionDashPattern.ReplaceAllString(strings.TrimSpace(description), "-")
	description = strings.Trim(description, "- ")
	// A name already in Codex_degrade form keeps its wording: no repeated
	// country prefix and no second 住宅IP / 机房 suffix.
	if meta.Country != "" {
		description = strings.Trim(strings.TrimPrefix(description, meta.Country), "- ")
	}
	prefix := meta.Flag + meta.Country
	if prefix == "" {
		prefix = "🌐未知"
	}
	parts := []string{prefix}
	if description != "" {
		parts = append(parts, description)
	}
	switch {
	case meta.Residential:
		if !strings.Contains(description, "住宅IP") {
			parts = append(parts, "住宅IP")
		}
	case !strings.Contains(description, "机房"):
		parts = append(parts, "机房")
		title := proxySubscriptionProtocolTitles[meta.Protocol]
		if title == "" {
			title = meta.Protocol
		}
		if title != "" {
			parts = append(parts, title)
		}
	}
	return clipCodexHarvestText(strings.Join(parts, "-"), 96)
}

func stripProxySubscriptionFlags(value string) string {
	runes := []rune(value)
	out := make([]rune, 0, len(runes))
	for _, r := range runes {
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			continue
		}
		if r == 0xFE0F {
			continue
		}
		out = append(out, r)
	}
	return strings.TrimSpace(string(out))
}

func proxySubscriptionFlagCode(value string) string {
	runes := []rune(value)
	for i := 0; i+1 < len(runes); i++ {
		a, b := runes[i], runes[i+1]
		if a >= 0x1F1E6 && a <= 0x1F1FF && b >= 0x1F1E6 && b <= 0x1F1FF {
			return string([]rune{'A' + (a - 0x1F1E6), 'A' + (b - 0x1F1E6)})
		}
	}
	return ""
}

func proxySubscriptionFlag(code string) string {
	if len(code) != 2 || !unicode.IsUpper(rune(code[0])) || !unicode.IsUpper(rune(code[1])) {
		return ""
	}
	return string([]rune{0x1F1E6 + rune(code[0]-'A'), 0x1F1E6 + rune(code[1]-'A')})
}

// ProxySubscriptionGroup is one selectable group of subscription proxies.
type ProxySubscriptionGroup struct {
	Name     string  `json:"name"`
	Source   string  `json:"source"`
	Kind     string  `json:"kind"`
	ProxyIDs []int64 `json:"proxy_ids"`
}

// buildProxySubscriptionGroups returns the provider's own groups first, then
// Codex_degrade-style groups: 🎫 打票出口 (residential), 💼 业务出口
// (datacenter), "<kind> · <multiplier>" and "<flag> <country> · <kind>".
func buildProxySubscriptionGroups(nodes []proxySubscriptionRuntimeNode, metas map[string]ProxySubscriptionNodeMeta, source []ProxySubscriptionSourceGroup) []ProxySubscriptionGroup {
	idByFingerprint := make(map[string]int64, len(nodes))
	for _, node := range nodes {
		idByFingerprint[node.Fingerprint] = node.ProxyID
	}
	groups := make([]ProxySubscriptionGroup, 0)
	declared := map[string]bool{}
	for _, group := range source {
		ids := make([]int64, 0, len(group.Fingerprints))
		for _, fingerprint := range group.Fingerprints {
			if id, ok := idByFingerprint[fingerprint]; ok {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			continue
		}
		declared[group.Name] = true
		groups = append(groups, ProxySubscriptionGroup{Name: group.Name, Source: proxySubscriptionGroupSourceSubscription, Kind: "provider", ProxyIDs: ids})
	}
	purpose := map[string][]int64{}
	multiplier := map[string][]int64{}
	region := map[string][]int64{}
	regionOrder := map[string]string{}
	for _, node := range nodes {
		meta := metas[node.Fingerprint]
		kind := "机房"
		purposeName := proxySubscriptionBusinessGroup
		if meta.Residential {
			kind, purposeName = "家宽住宅", proxySubscriptionHarvestGroup
		}
		purpose[purposeName] = append(purpose[purposeName], node.ProxyID)
		multiplierName := kind + " · " + meta.Multiplier
		multiplier[multiplierName] = append(multiplier[multiplierName], node.ProxyID)
		if meta.Region != "" {
			short := "机房"
			if meta.Residential {
				short = "住宅"
			}
			regionName := strings.TrimSpace(meta.Flag+" "+meta.Country) + " · " + short
			region[regionName] = append(region[regionName], node.ProxyID)
			regionOrder[regionName] = meta.Region + short
		}
	}
	for _, name := range []string{proxySubscriptionHarvestGroup, proxySubscriptionBusinessGroup} {
		if ids := purpose[name]; len(ids) > 0 && !declared[name] {
			groups = append(groups, ProxySubscriptionGroup{Name: name, Source: proxySubscriptionGroupSourceDerived, Kind: "purpose", ProxyIDs: ids})
		}
	}
	for _, name := range sortedProxySubscriptionKeys(multiplier, nil) {
		groups = append(groups, ProxySubscriptionGroup{Name: name, Source: proxySubscriptionGroupSourceDerived, Kind: "multiplier", ProxyIDs: multiplier[name]})
	}
	for _, name := range sortedProxySubscriptionKeys(region, regionOrder) {
		groups = append(groups, ProxySubscriptionGroup{Name: name, Source: proxySubscriptionGroupSourceDerived, Kind: "region", ProxyIDs: region[name]})
	}
	if len(groups) > maxProxySubscriptionGroups {
		groups = groups[:maxProxySubscriptionGroups]
	}
	return groups
}

func sortedProxySubscriptionKeys(values map[string][]int64, order map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if order != nil {
			a, b = order[a], order[b]
		}
		if a == b {
			return keys[i] < keys[j]
		}
		return a < b
	})
	return keys
}

// ParseProxySubscriptionUsage reads "upload=..; download=..; total=..; expire=..".
func ParseProxySubscriptionUsage(header http.Header) *ProxySubscriptionUsage {
	raw := strings.TrimSpace(header.Get("Subscription-Userinfo"))
	if raw == "" {
		return nil
	}
	usage := &ProxySubscriptionUsage{}
	found := false
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || number < 0 {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "upload":
			usage.Upload, found = number, true
		case "download":
			usage.Download, found = number, true
		case "total":
			usage.Total, found = number, true
		case "expire":
			if number > 0 {
				at := time.Unix(number, 0).UTC()
				usage.ExpireAt, found = &at, true
			}
		}
	}
	if !found {
		return nil
	}
	return usage
}
