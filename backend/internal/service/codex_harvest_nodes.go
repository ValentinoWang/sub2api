package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

const codexHarvestRecentSuccess = 7 * 24 * time.Hour

// CodexHarvestNodeScope isolates proxy learning per upstream identity and
// model: a re-authorized account starts with a clean record.
type CodexHarvestNodeScope struct {
	AccountID int64  `json:"account_id"`
	Identity  string `json:"-"`
	Model     string `json:"model"`
}

type CodexHarvestNodeRecord struct {
	CodexHarvestNodeScope
	ID                  int64      `json:"id"`
	ProxyID             int64      `json:"proxy_id"`
	ProxyName           string     `json:"proxy_name"`
	AccountName         string     `json:"account_name,omitempty"`
	Successes           int64      `json:"successes"`
	Misses              int64      `json:"misses"`
	NetworkErrors       int64      `json:"network_errors"`
	AccountErrors       int64      `json:"account_errors"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastSuccess         *time.Time `json:"last_success"`
	CooldownUntil       *time.Time `json:"cooldown_until"`
	LatencyMS           int64      `json:"latency_ms"`
	LastResult          string     `json:"last_result"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type CodexHarvestNodePage struct {
	Items []CodexHarvestNodeRecord `json:"items"`
	Total int64                    `json:"total"`
}

type CodexHarvestNodeFeedback struct {
	Scope           CodexHarvestNodeScope
	ProxyID         int64
	ProxyName       string
	Generation      int64
	Result          string
	LatencyMS       int64
	CooldownSeconds int
}

type CodexHarvestNodeRepository interface {
	Snapshot(context.Context, CodexHarvestNodeScope) (int64, []CodexHarvestNodeRecord, error)
	Record(context.Context, CodexHarvestNodeFeedback) (bool, error)
	List(context.Context, int, int) (CodexHarvestNodePage, error)
	Reset(context.Context, int64) error
}

// codexHarvestIdentity fingerprints the upstream ChatGPT identity without
// storing it, so learning never follows a token to a different ChatGPT user.
func codexHarvestIdentity(account *Account) string {
	if account == nil {
		return ""
	}
	id := strings.TrimSpace(account.GetCredential("chatgpt_account_id"))
	email := strings.ToLower(strings.TrimSpace(account.GetCredential("email")))
	if id == "" && email == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id + "\x00" + email))
	return hex.EncodeToString(sum[:16])
}

// rankCodexHarvestProxies orders untried, non-cooling proxies. Recently
// successful proxies come first; the rest rotate with the explore cursor so a
// fresh pool is sampled evenly instead of always hitting the first entry.
func rankCodexHarvestProxies(pool []Proxy, records []CodexHarvestNodeRecord, tried map[int64]bool, cursor uint64, now time.Time) []Proxy {
	if len(pool) == 0 {
		return nil
	}
	stats := make(map[int64]CodexHarvestNodeRecord, len(records))
	for _, r := range records {
		stats[r.ProxyID] = r
	}
	ordered := append([]Proxy(nil), pool...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	eligible := make([]Proxy, 0, len(ordered))
	offset := int(cursor % uint64(len(ordered)))
	for i := range ordered {
		p := ordered[(i+offset)%len(ordered)]
		r := stats[p.ID]
		if tried[p.ID] || (r.CooldownUntil != nil && r.CooldownUntil.After(now)) {
			continue
		}
		eligible = append(eligible, p)
	}
	recent := func(r CodexHarvestNodeRecord) bool {
		return r.LastSuccess != nil && r.LastSuccess.After(now.Add(-codexHarvestRecentSuccess))
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := stats[eligible[i].ID], stats[eligible[j].ID]
		if recent(a) != recent(b) {
			return recent(a)
		}
		if !recent(a) {
			return false
		}
		if a.ConsecutiveFailures != b.ConsecutiveFailures {
			return a.ConsecutiveFailures < b.ConsecutiveFailures
		}
		ar := float64(a.Successes) / float64(a.Successes+a.Misses+a.NetworkErrors+1)
		br := float64(b.Successes) / float64(b.Successes+b.Misses+b.NetworkErrors+1)
		if ar != br {
			return ar > br
		}
		if !a.LastSuccess.Equal(*b.LastSuccess) {
			return a.LastSuccess.After(*b.LastSuccess)
		}
		return a.LatencyMS < b.LatencyMS
	})
	return eligible
}
