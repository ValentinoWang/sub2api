package service

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

type lifecycleClaimTestRepository struct {
	mu         sync.Mutex
	claims     map[string]bool
	claimCalls int
	markCalls  int
}

func (r *lifecycleClaimTestRepository) ListWelcomeCandidates(context.Context, string, time.Time, int) ([]LifecycleCandidate, error) {
	return nil, nil
}

func (r *lifecycleClaimTestRepository) ListInactiveCandidates(context.Context, string, time.Time, time.Time, int) ([]LifecycleCandidate, error) {
	return nil, nil
}

func (r *lifecycleClaimTestRepository) MarkSent(context.Context, int64, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markCalls++
	return nil
}

func (r *lifecycleClaimTestRepository) TryClaim(_ context.Context, userID int64, event string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimCalls++
	key := event + ":" + strconv.FormatInt(userID, 10)
	if r.claims[key] {
		return false, nil
	}
	r.claims[key] = true
	return true, nil
}

func (r *lifecycleClaimTestRepository) ReleaseClaim(_ context.Context, userID int64, event string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.claims, event+":"+strconv.FormatInt(userID, 10))
	return nil
}

func TestUserLifecycleServiceClaimsReservedMailboxOnlyOnce(t *testing.T) {
	repo := &lifecycleClaimTestRepository{claims: make(map[string]bool)}
	svc := NewUserLifecycleService(repo, &NotificationEmailService{}, nil, time.Minute)
	candidate := LifecycleCandidate{UserID: 42, Email: "user@linuxdo-connect.invalid"}

	svc.deliver(context.Background(), candidate, NotificationEmailEventUserWelcome, nil)
	svc.deliver(context.Background(), candidate, NotificationEmailEventUserWelcome, nil)

	if repo.claimCalls != 2 {
		t.Fatalf("claim calls = %d, want 2", repo.claimCalls)
	}
	if len(repo.claims) != 1 {
		t.Fatalf("claimed deliveries = %d, want 1", len(repo.claims))
	}
	if repo.markCalls != 0 {
		t.Fatalf("MarkSent calls = %d, want 0 when atomic claim is available", repo.markCalls)
	}
}
