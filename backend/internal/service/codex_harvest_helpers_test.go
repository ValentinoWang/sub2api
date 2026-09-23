package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const codexHarvestCompletedBody = "data: {\"type\":\"response.created\",\"response\":{\"status\":\"in_progress\"}}\n\n" +
	"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"

func codexHarvestResponse(status int, state string) *http.Response {
	h := http.Header{}
	if state != "" {
		h.Set(openAICodexTurnStateHeader, state)
	}
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(codexHarvestCompletedBody))}
}

func testHarvestProxy(id int64) Proxy {
	return Proxy{ID: id, Name: fmt.Sprintf("harvest-%d", id), Protocol: "http", Host: fmt.Sprintf("harvest%d.example", id), Port: 8000 + int(id), Status: StatusActive}
}

type codexHarvestProxyRepoStub struct {
	ProxyRepository
	mu      sync.Mutex
	proxies map[int64]Proxy
	list    func(context.Context, []int64) ([]Proxy, error)
}

func newCodexHarvestProxyRepoStub(proxies ...Proxy) *codexHarvestProxyRepoStub {
	r := &codexHarvestProxyRepoStub{proxies: map[int64]Proxy{}}
	for _, p := range proxies {
		r.proxies[p.ID] = p
	}
	return r
}

func (r *codexHarvestProxyRepoStub) ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	if r.list != nil {
		return r.list(ctx, ids)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Proxy, 0, len(ids))
	for _, id := range ids {
		if p, ok := r.proxies[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

type codexHarvestNodeRepoStub struct {
	mu          sync.Mutex
	generation  int64
	records     []CodexHarvestNodeRecord
	feedback    []CodexHarvestNodeFeedback
	snapshotErr error
}

func (r *codexHarvestNodeRepoStub) Snapshot(_ context.Context, scope CodexHarvestNodeScope) (int64, []CodexHarvestNodeRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.snapshotErr != nil {
		return 0, nil, r.snapshotErr
	}
	out := []CodexHarvestNodeRecord{}
	for _, rec := range r.records {
		if rec.AccountID == scope.AccountID && rec.Identity == scope.Identity && rec.Model == scope.Model {
			out = append(out, rec)
		}
	}
	return r.generation, out, nil
}

func (r *codexHarvestNodeRepoStub) Record(_ context.Context, f CodexHarvestNodeFeedback) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.feedback = append(r.feedback, f)
	if f.Generation != r.generation {
		return false, nil
	}
	now := time.Now()
	for i := range r.records {
		rec := &r.records[i]
		if rec.ProxyID == f.ProxyID && rec.AccountID == f.Scope.AccountID && rec.Identity == f.Scope.Identity && rec.Model == f.Scope.Model {
			applyCodexHarvestStubFeedback(rec, f, now)
			return true, nil
		}
	}
	rec := CodexHarvestNodeRecord{CodexHarvestNodeScope: f.Scope, ID: int64(len(r.records) + 1), ProxyID: f.ProxyID, ProxyName: f.ProxyName}
	applyCodexHarvestStubFeedback(&rec, f, now)
	r.records = append(r.records, rec)
	return true, nil
}

func applyCodexHarvestStubFeedback(rec *CodexHarvestNodeRecord, f CodexHarvestNodeFeedback, now time.Time) {
	rec.LastResult = f.Result
	switch f.Result {
	case "success":
		rec.Successes++
		rec.ConsecutiveFailures = 0
		rec.LastSuccess = &now
		rec.CooldownUntil = nil
	case "network_error", "invalid_state", "response_incomplete_or_error", "upstream_error":
		if f.Result == "network_error" {
			rec.NetworkErrors++
		} else {
			rec.Misses++
		}
		rec.ConsecutiveFailures++
		until := now.Add(time.Duration(f.CooldownSeconds) * time.Second)
		rec.CooldownUntil = &until
	default:
		rec.AccountErrors++
	}
}

func (r *codexHarvestNodeRepoStub) List(context.Context, int, int) (CodexHarvestNodePage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return CodexHarvestNodePage{Items: append([]CodexHarvestNodeRecord(nil), r.records...), Total: int64(len(r.records))}, nil
}

func (r *codexHarvestNodeRepoStub) Reset(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generation++
	kept := r.records[:0]
	for _, rec := range r.records {
		if id != 0 && rec.ID != id {
			kept = append(kept, rec)
		}
	}
	r.records = kept
	return nil
}

type codexHarvestFlowRepoStub struct {
	mu     sync.Mutex
	events []CodexHarvestFlowEvent
}

func (r *codexHarvestFlowRepoStub) List(_ context.Context, limit int) ([]CodexHarvestFlowEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	start := max(len(r.events)-limit, 0)
	return append([]CodexHarvestFlowEvent(nil), r.events[start:]...), nil
}

func (r *codexHarvestFlowRepoStub) Append(_ context.Context, e CodexHarvestFlowEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

// newTicketHarvest builds a harvest service whose pool is the given proxies,
// with no pacing so tests do not sleep.
func newTicketHarvest(t *testing.T, proxies ...Proxy) (*CodexHarvestService, *codexHarvestNodeRepoStub) {
	t.Helper()
	nodes := &codexHarvestNodeRepoStub{generation: 1}
	h := NewCodexHarvestService(nodes, nil, &codexPolicyMigrationRepoStub{values: map[string]string{}}, newCodexHarvestProxyRepoStub(proxies...))
	controls := DefaultCodexHarvestControls()
	controls.Preset = codexHarvestPresetCustom
	controls.Speed.ProbeIntervalSeconds = 0
	for _, p := range proxies {
		controls.ProxyIDs = append(controls.ProxyIDs, p.ID)
	}
	require.NoError(t, h.SaveControls(context.Background(), controls))
	return h, nodes
}

func (r *codexTicketRefreshRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, ErrAccountNotFound
}
