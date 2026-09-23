package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// A Codex_degrade-style Mihomo profile with fake credentials.
const codexDegradeStyleProfile = `
mixed-port: 0
proxies:
- name: 🇺🇸美国-洛杉矶-ZgoCloud-NetLab机房-Vless
  type: vless
  server: dc.example.test
  port: 443
  uuid: 11111111-1111-1111-1111-111111111111
  network: tcp
  tls: true
  servername: dc.example.test
  dialer-proxy: other
- name: 🇺🇸美国-克利夫兰-Charter静态住宅IP
  type: hysteria2
  server: isp.example.test
  port: 1443
  password: fake-one
- name: 🇺🇸美国-雪城-SurfAir静态住宅IP
  type: hysteria2
  server: isp.example.test
  port: 2443
  password: fake-two
- name: 剩余流量：12.5 GB
  type: ss
  server: info.example.test
  port: 1
  cipher: aes-128-gcm
  password: x
proxy-groups:
- name: ♻️ 自动选择
  type: url-test
  proxies: [🇺🇸美国-洛杉矶-ZgoCloud-NetLab机房-Vless]
- name: 💼 业务出口
  type: select
  proxies: [♻️ 自动选择, 🇺🇸美国-洛杉矶-ZgoCloud-NetLab机房-Vless, DIRECT]
- name: 🎫 打票出口
  type: select
  proxies: [🇺🇸美国-克利夫兰-Charter静态住宅IP, 🇺🇸美国-雪城-SurfAir静态住宅IP]
- name: 空组
  type: select
  proxies: [DIRECT, REJECT]
`

func TestParseProxySubscriptionDocument_MihomoProfileKeepsProviderGroups(t *testing.T) {
	doc, err := ParseProxySubscriptionDocument([]byte(codexDegradeStyleProfile))
	require.NoError(t, err)
	require.Equal(t, ProxySubscriptionFormatMihomoYAML, doc.Format)
	require.Len(t, doc.Nodes, 3)
	require.Equal(t, []string{"剩余流量：12.5 GB"}, doc.Info)
	require.NotContains(t, doc.Nodes[0].MihomoConfig, "dialer-proxy", "chained dialers are stripped")
	require.Equal(t, "vless", doc.Nodes[0].MihomoConfig["type"])
	require.Empty(t, doc.Nodes[0].SourceURI)

	names := map[string][]string{}
	for _, group := range doc.Groups {
		names[group.Name] = group.Fingerprints
	}
	require.NotContains(t, names, "空组", "groups with only built-in policies are dropped")
	require.Equal(t, []string{doc.Nodes[0].Fingerprint}, names["💼 业务出口"], "nested url-test groups flatten to their nodes")
	require.Equal(t, []string{doc.Nodes[1].Fingerprint, doc.Nodes[2].Fingerprint}, names["🎫 打票出口"])
}

func TestParseProxySubscriptionDocument_RejectsUnsafeOrEmptyProfiles(t *testing.T) {
	for name, body := range map[string]string{
		"providers only":   "proxy-providers:\n  remote:\n    type: http\n    url: https://example.test/p.yaml\n",
		"unsupported type": "proxies:\n- {name: a, type: direct, server: x.test, port: 1}\n",
		"bad port":         "proxies:\n- {name: a, type: ss, server: x.test, port: 0, cipher: aes-128-gcm, password: p}\n",
		"only info":        "proxies:\n- {name: 套餐到期：2026-10-01, type: ss, server: x.test, port: 1, cipher: aes-128-gcm, password: p}\n",
	} {
		_, err := ParseProxySubscriptionDocument([]byte(body))
		require.Error(t, err, name)
	}
}

func TestParseProxySubscriptionDocument_URIListSeparatesInfoEntries(t *testing.T) {
	lines := []string{
		"vless://11111111-1111-1111-1111-111111111111@a.example.test:443?security=tls&sni=a.example.test#" + url.PathEscape("剩余流量：138.17 GB"),
		"vless://11111111-1111-1111-1111-111111111111@b.example.test:443?security=tls&sni=b.example.test#" + url.PathEscape("优秀|【3x】中转|香港家宽🇭🇰"),
		"hysteria2://fake@c.example.test:443?sni=c.example.test#" + url.PathEscape("【2】新加坡高速节点🇸🇬hy2"),
	}
	raw := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
	doc, err := ParseProxySubscriptionDocument([]byte(raw))
	require.NoError(t, err)
	require.Equal(t, ProxySubscriptionFormatURIList, doc.Format)
	require.Len(t, doc.Nodes, 2)
	require.Equal(t, []string{"剩余流量：138.17 GB"}, doc.Info)
}

func TestClassifyProxySubscriptionNode_CodexDegradeNaming(t *testing.T) {
	cases := []struct {
		name, protocol string
		want           ProxySubscriptionNodeMeta
	}{
		{"优秀|【3x】中转|香港家宽🇭🇰", "vless", ProxySubscriptionNodeMeta{Region: "HK", Country: "香港", Flag: "🇭🇰", Residential: true, Multiplier: "3×", Route: "中转", Protocol: "vless", DisplayName: "🇭🇰香港-中转-香港家宽-住宅IP"}},
		{"【2】新加坡高速节点🇸🇬hy2", "hysteria2", ProxySubscriptionNodeMeta{Region: "SG", Country: "新加坡", Flag: "🇸🇬", Multiplier: "1×", Route: "直连", Protocol: "hysteria2", DisplayName: "🇸🇬新加坡-【2】新加坡高速节点-机房-Hysteria2"}},
		{"[01]台湾hinet家宽🇨🇳hy2", "hysteria2", ProxySubscriptionNodeMeta{Region: "TW", Country: "台湾", Flag: "🇹🇼", Residential: true, Multiplier: "1×", Route: "直连", Protocol: "hysteria2", DisplayName: "🇹🇼台湾-[01]台湾hinet家宽-住宅IP"}},
		{"cf加速|美国圣何塞", "vless", ProxySubscriptionNodeMeta{Region: "US", Country: "美国", Flag: "🇺🇸", Multiplier: "1×", Route: "CF", Protocol: "vless", DisplayName: "🇺🇸美国-cf加速-美国圣何塞-机房-Vless"}},
		{"【10x】日本KDDI家宽🇯🇵ss|不可直连！小白不要连接！", "ss", ProxySubscriptionNodeMeta{Region: "JP", Country: "日本", Flag: "🇯🇵", Residential: true, Multiplier: "10×", Route: "直连", Protocol: "ss", DisplayName: "🇯🇵日本-日本KDDI家宽-不可直连！小白不要连接！-住宅IP"}},
		{"🇺🇸美国-克利夫兰-Charter静态住宅IP", "hysteria2", ProxySubscriptionNodeMeta{Region: "US", Country: "美国", Flag: "🇺🇸", Residential: true, Multiplier: "1×", Route: "直连", Protocol: "hysteria2", DisplayName: "🇺🇸美国-美国-克利夫兰-Charter静态住宅IP-住宅IP"}},
		{"Mystery node", "vless", ProxySubscriptionNodeMeta{Multiplier: "1×", Route: "直连", Protocol: "vless", DisplayName: "🌐未知-Mystery node-机房-Vless"}},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, ClassifyProxySubscriptionNode(tc.name, tc.protocol), tc.name)
	}
	for _, info := range []string{"剩余流量：138.17+GB", "距离下次重置剩余：5+天", "套餐到期：2026-09-14", "苏菲家宽官网地址：sufe.us/ai.sufe.pro", "官网地址防失联发布页：sufe.uk"} {
		require.True(t, isProxySubscriptionInfoEntry(info), info)
	}
	for _, node := range []string{"联通移动用中转", "【3x】中转|美国一🇺🇸", "香港家宽hy2🇭🇰"} {
		require.False(t, isProxySubscriptionInfoEntry(node), node)
	}
}

func TestBuildProxySubscriptionGroups_ProviderThenCodexDegradeGroups(t *testing.T) {
	nodes := []proxySubscriptionRuntimeNode{
		{Fingerprint: "a", Name: "优秀|【3x】中转|香港家宽🇭🇰", ProxyID: 1},
		{Fingerprint: "b", Name: "【3x】中转|日本NTT家宽", ProxyID: 2},
		{Fingerprint: "c", Name: "【2】新加坡高速节点🇸🇬hy2", ProxyID: 3},
	}
	metas := map[string]ProxySubscriptionNodeMeta{}
	for _, node := range nodes {
		metas[node.Fingerprint] = ClassifyProxySubscriptionNode(node.Name, "vless")
	}
	source := []ProxySubscriptionSourceGroup{{Name: "🎫 打票出口", Type: "select", Fingerprints: []string{"b"}}, {Name: "gone", Fingerprints: []string{"zzz"}}}
	groups := buildProxySubscriptionGroups(nodes, metas, source)
	byName := map[string]ProxySubscriptionGroup{}
	order := []string{}
	for _, group := range groups {
		byName[group.Name] = group
		order = append(order, group.Name)
	}
	require.Equal(t, "subscription", byName["🎫 打票出口"].Source, "a provider group wins over the derived group of the same name")
	require.Equal(t, []int64{2}, byName["🎫 打票出口"].ProxyIDs)
	require.NotContains(t, byName, "gone")
	require.Equal(t, []int64{3}, byName["💼 业务出口"].ProxyIDs)
	require.Equal(t, []int64{1, 2}, byName["家宽住宅 · 3×"].ProxyIDs)
	require.Equal(t, []int64{3}, byName["机房 · 1×"].ProxyIDs)
	require.Equal(t, []int64{1}, byName["🇭🇰 香港 · 住宅"].ProxyIDs)
	require.Equal(t, []int64{2}, byName["🇯🇵 日本 · 住宅"].ProxyIDs)
	require.Equal(t, "🎫 打票出口", order[0])
}

func TestParseProxySubscriptionUsage(t *testing.T) {
	h := http.Header{}
	require.Nil(t, ParseProxySubscriptionUsage(h))
	h.Set("Subscription-Userinfo", "upload=100; download=2048; total=10737418240; expire=1790000000")
	usage := ParseProxySubscriptionUsage(h)
	require.NotNil(t, usage)
	require.Equal(t, int64(100), usage.Upload)
	require.Equal(t, int64(2048), usage.Download)
	require.Equal(t, int64(10737418240), usage.Total)
	require.Equal(t, time.Unix(1790000000, 0).UTC(), *usage.ExpireAt)
	h.Set("Subscription-Userinfo", "garbage")
	require.Nil(t, ParseProxySubscriptionUsage(h))
}

func TestProxySubscriptionErrorTextNeverLeaksURL(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://sub.example.test/api/v1/client/subscribe?token=secret-token", Err: errors.New("connection reset")}
	require.NotContains(t, proxySubscriptionErrorText(err), "secret-token")
	require.Equal(t, "subscription request failed", proxySubscriptionErrorText(err))
	require.Equal(t, "subscription server returned HTTP 404", proxySubscriptionErrorText(errors.New("subscription server returned HTTP 404")))
}

func TestValidateProxySubscriptionRefreshInterval(t *testing.T) {
	for _, ok := range []int{0, 15, 60, 1440} {
		require.NoError(t, validateProxySubscriptionRefreshInterval(ok))
	}
	for _, bad := range []int{-1, 1, 14, 1441} {
		require.True(t, infraerrors.IsBadRequest(validateProxySubscriptionRefreshInterval(bad)))
	}
	last := time.Now().Add(-61 * time.Minute)
	require.True(t, proxySubscriptionRefreshDue(proxySubscriptionRuntimeEntry{URLCiphertext: "c", RefreshIntervalMinutes: 60, LastRefreshAt: &last}, time.Now()))
	require.False(t, proxySubscriptionRefreshDue(proxySubscriptionRuntimeEntry{URLCiphertext: "c", RefreshIntervalMinutes: 0, LastRefreshAt: &last}, time.Now()))
	require.False(t, proxySubscriptionRefreshDue(proxySubscriptionRuntimeEntry{RefreshIntervalMinutes: 60}, time.Now()), "no stored URL, nothing to refresh")
}

type subscriptionAdminStub struct {
	AdminService
	mu      sync.Mutex
	nextID  int64
	proxies map[int64]*Proxy
}

func (a *subscriptionAdminStub) CreateProxy(_ context.Context, input *CreateProxyInput) (*Proxy, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextID++
	p := &Proxy{ID: a.nextID, Name: input.Name, Protocol: input.Protocol, Host: input.Host, Port: input.Port, Status: StatusActive}
	a.proxies[p.ID] = p
	return p, nil
}

func (a *subscriptionAdminStub) GetProxy(_ context.Context, id int64) (*Proxy, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.proxies[id]
	if !ok {
		return nil, errors.New("not found")
	}
	copyProxy := *p
	return &copyProxy, nil
}

func (a *subscriptionAdminStub) UpdateProxy(_ context.Context, id int64, input *UpdateProxyInput) (*Proxy, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.proxies[id]
	if input.Status != "" {
		p.Status = input.Status
	}
	return p, nil
}

type reversingEncryptor struct{}

func (reversingEncryptor) Encrypt(plain string) (string, error) {
	return "enc:" + base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

func (reversingEncryptor) Decrypt(cipher string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(cipher, "enc:"))
	return string(raw), err
}

func TestProxySubscriptionImportRefreshAndList(t *testing.T) {
	var mu sync.Mutex
	body := codexDegradeStyleProfile
	subscription := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if !strings.Contains(r.Header.Get("User-Agent"), "clash.meta") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3; expire=1790000000")
		_, _ = w.Write([]byte(body))
	}))
	defer subscription.Close()
	var reloads atomic.Int64
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		reloads.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer controller.Close()
	dir := t.TempDir()
	admin := &subscriptionAdminStub{proxies: map[int64]*Proxy{}}
	svc := NewProxySubscriptionService(admin, &config.Config{ProxySubscription: config.ProxySubscriptionConfig{
		Enabled: true, StatePath: filepath.Join(dir, "subscriptions.json"), ConfigPath: filepath.Join(dir, "config.yaml"),
		ControllerConfigPath: "/etc/mihomo/config.yaml", ControllerURL: controller.URL, ProxyHost: "mihomo", PortStart: 20000, PortEnd: 20100,
	}})
	svc.encryptor = reversingEncryptor{}
	svc.validateURL = func(context.Context, *url.URL) error { return nil }
	subscriptionURL := subscription.URL + "/sub?token=secret-token"

	result, err := svc.Import(context.Background(), ProxySubscriptionImportInput{Name: "VPS2ISP", URL: subscriptionURL})
	require.NoError(t, err)
	require.Equal(t, ProxySubscriptionFormatMihomoYAML, result.Format)
	require.Equal(t, 3, result.NodeCount)
	require.Equal(t, 1, result.InfoCount)
	require.Equal(t, 3, result.Created)

	stateRaw, err := os.ReadFile(filepath.Join(dir, "subscriptions.json"))
	require.NoError(t, err)
	require.NotContains(t, string(stateRaw), "secret-token", "the URL is stored encrypted")
	configRaw, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	var rendered map[string]any
	require.NoError(t, yaml.Unmarshal(configRaw, &rendered))
	require.Len(t, rendered["proxies"], 3)
	require.NotContains(t, string(configRaw), "dialer-proxy")

	views, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 1)
	view := views[0]
	require.True(t, view.HasURL)
	require.Equal(t, proxySubscriptionDefaultRefreshMinutes, view.RefreshIntervalMinutes)
	require.NotNil(t, view.NextRefreshAt)
	require.Equal(t, int64(3), view.Usage.Total)
	require.Equal(t, []string{"剩余流量：12.5 GB"}, view.Info)
	groups := map[string]ProxySubscriptionGroup{}
	for _, group := range view.Groups {
		groups[group.Name] = group
	}
	require.Equal(t, "subscription", groups["🎫 打票出口"].Source)
	require.Len(t, groups["🎫 打票出口"].ProxyIDs, 2)
	require.Equal(t, "subscription", groups["💼 业务出口"].Source)
	require.Contains(t, groups, "🇺🇸 美国 · 住宅")
	viewJSON, err := json.Marshal(views)
	require.NoError(t, err)
	for _, secret := range []string{"secret-token", "fake-one", "11111111-1111"} {
		require.NotContains(t, string(viewJSON), secret)
	}

	// The provider drops one residential node: refresh keeps IDs and deactivates the removed proxy.
	mu.Lock()
	body = strings.Replace(codexDegradeStyleProfile, "- name: 🇺🇸美国-雪城-SurfAir静态住宅IP\n  type: hysteria2\n  server: isp.example.test\n  port: 2443\n  password: fake-two\n", "", 1)
	mu.Unlock()
	refreshed, err := svc.Refresh(context.Background(), result.SubscriptionID)
	require.NoError(t, err)
	require.Equal(t, 2, refreshed.Reused)
	require.Equal(t, 0, refreshed.Created)
	require.Equal(t, 1, refreshed.Deactivated)
	require.Equal(t, int64(2), reloads.Load())
	inactive := 0
	for _, p := range admin.proxies {
		if p.Status == "inactive" {
			inactive++
		}
	}
	require.Equal(t, 1, inactive)

	require.NoError(t, svc.UpdateRefreshInterval(context.Background(), result.SubscriptionID, 0))
	require.True(t, infraerrors.IsBadRequest(svc.UpdateRefreshInterval(context.Background(), result.SubscriptionID, 5)))
	views, err = svc.List(context.Background())
	require.NoError(t, err)
	require.Zero(t, views[0].RefreshIntervalMinutes)
	require.Nil(t, views[0].NextRefreshAt)

	// A failed refresh is recorded without the URL and keeps the nodes.
	subscription.Close()
	_, err = svc.Refresh(context.Background(), result.SubscriptionID)
	require.Error(t, err)
	views, err = svc.List(context.Background())
	require.NoError(t, err)
	require.Equal(t, "failed", views[0].LastRefreshStatus)
	require.NotContains(t, views[0].LastRefreshError, "secret-token")
	require.Equal(t, 2, views[0].NodeCount)
}

func TestProxySubscriptionListGroupsLegacyEntriesAndFlagsInfoNodes(t *testing.T) {
	dir := t.TempDir()
	state := proxySubscriptionTestState(t)
	info, err := parseProxySubscriptionURI("vless://11111111-1111-1111-1111-111111111111@info.example.test:443?security=tls&sni=info.example.test#" + url.PathEscape("套餐到期：2026-09-14"))
	require.NoError(t, err)
	state.Subscriptions[0].Nodes = append(state.Subscriptions[0].Nodes, proxySubscriptionRuntimeNode{Fingerprint: info.Fingerprint, Name: info.Name, SourceURI: info.SourceURI, ProxyID: 9, Port: 22009})
	svc := NewProxySubscriptionService(nil, &config.Config{ProxySubscription: config.ProxySubscriptionConfig{Enabled: true, StatePath: filepath.Join(dir, "subscriptions.json")}})
	require.NoError(t, svc.saveState(state))
	views, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.False(t, views[0].HasURL)
	require.Equal(t, ProxySubscriptionFormatURIList, views[0].Format)
	require.Equal(t, 3, views[0].NodeCount)
	require.True(t, views[0].Nodes[3].Info)
	require.NotEmpty(t, views[0].Groups)
	for _, group := range views[0].Groups {
		require.NotContains(t, group.ProxyIDs, int64(9), "info entries never join a group")
	}
	_, err = svc.Refresh(context.Background(), views[0].ID)
	require.ErrorContains(t, err, "not stored")
}

func TestRenderedMihomoProfileSubscriptionAcceptedByMihomo(t *testing.T) {
	bin := os.Getenv("MIHOMO_BIN")
	if bin == "" {
		t.Skip("MIHOMO_BIN is not set")
	}
	doc, err := ParseProxySubscriptionDocument([]byte(codexDegradeStyleProfile))
	require.NoError(t, err)
	nodes := make([]proxySubscriptionRuntimeNode, 0, len(doc.Nodes))
	for i, node := range doc.Nodes {
		nodes = append(nodes, newProxySubscriptionRuntimeNode(node, int64(i+1), 23000+i))
	}
	state := &proxySubscriptionRuntimeState{Version: proxySubscriptionStateVersion, ControllerSecret: "controller-test-secret",
		Subscriptions: []proxySubscriptionRuntimeEntry{{ID: "yaml", Name: "yaml", Nodes: nodes}}}
	raw, err := renderProxySubscriptionMihomoConfig(state)
	require.NoError(t, err)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, raw, 0o600))
	bin, err = validatedMihomoTestBinary(bin)
	require.NoError(t, err)
	// #nosec G702 -- MIHOMO_BIN is selected by the test operator and resolved to a regular executable by validatedMihomoTestBinary; no subscription value controls the program, and no shell is used.
	output, err := exec.Command(bin, "-t", "-d", dir, "-f", configPath).CombinedOutput()
	require.NoError(t, err, "mihomo rejected generated config: %s", output)
}
