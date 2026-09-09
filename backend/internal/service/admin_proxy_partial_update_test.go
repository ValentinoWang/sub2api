//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdminProxyPartialUpdatePreservesOmittedSettings(t *testing.T) {
	for _, input := range []*UpdateProxyInput{{Status: "inactive"}, {Name: "renamed"}, {Host: "new.example"}} {
		expiry := time.Now().Add(24 * time.Hour)
		backup := int64(10)
		original := &Proxy{ID: 9, Name: "original", Host: "old.example", Status: StatusActive, ExpiresAt: &expiry, FallbackMode: FallbackModeProxy, BackupProxyID: &backup, ExpiryWarnDays: 7}
		repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: original}
		svc := &adminServiceImpl{proxyRepo: repo}
		got, err := svc.UpdateProxy(context.Background(), 9, input)
		require.NoError(t, err)
		require.Equal(t, original.ExpiresAt, got.ExpiresAt)
		require.Equal(t, original.FallbackMode, got.FallbackMode)
		require.Equal(t, original.BackupProxyID, got.BackupProxyID)
		require.Equal(t, original.ExpiryWarnDays, got.ExpiryWarnDays)
		require.Equal(t, 1, repo.updateCalls)
	}
}

func TestAdminProxyPartialUpdateClearsAndSetsSettings(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	backup := int64(10)
	zero := 0
	repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: &Proxy{ID: 9, ExpiresAt: &expiry, FallbackMode: FallbackModeProxy, BackupProxyID: &backup, ExpiryWarnDays: 7}}
	svc := &adminServiceImpl{proxyRepo: repo}
	got, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
		ClearExpiresAt: true, FallbackMode: FallbackModeNone, ClearBackupID: true, ExpiryWarnDays: &zero,
	})
	require.NoError(t, err)
	require.Nil(t, got.ExpiresAt)
	require.Nil(t, got.BackupProxyID)
	require.Equal(t, FallbackModeNone, got.FallbackMode)
	require.Zero(t, got.ExpiryWarnDays)
	days := 3
	got, err = svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
		ExpiresAt: &expiry, FallbackMode: FallbackModeProxy, BackupProxyID: &backup, ExpiryWarnDays: &days,
	})
	require.NoError(t, err)
	require.Equal(t, &expiry, got.ExpiresAt)
	require.Equal(t, &backup, got.BackupProxyID)
	require.Equal(t, FallbackModeProxy, got.FallbackMode)
	require.Equal(t, days, got.ExpiryWarnDays)
}

func TestAdminProxyPartialUpdateValidatesMergedFallback(t *testing.T) {
	backup := int64(10)
	self := int64(9)
	negative := -1
	for _, tc := range []struct {
		name      string
		input     UpdateProxyInput
		wantError bool
	}{
		{name: "reuse existing backup", input: UpdateProxyInput{FallbackMode: FallbackModeProxy}},
		{name: "clear required backup", input: UpdateProxyInput{ClearBackupID: true}, wantError: true},
		{name: "self backup", input: UpdateProxyInput{BackupProxyID: &self}, wantError: true},
		{name: "negative warning", input: UpdateProxyInput{ExpiryWarnDays: &negative}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: &Proxy{ID: 9, FallbackMode: FallbackModeProxy, BackupProxyID: &backup}}
			svc := &adminServiceImpl{proxyRepo: repo}
			_, err := svc.UpdateProxy(context.Background(), 9, &tc.input)
			if tc.wantError {
				require.Error(t, err)
				require.Zero(t, repo.updateCalls)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAdminProxyPartialUpdatePreservesCredentialFieldPresence(t *testing.T) {
	for _, tc := range []struct {
		name         string
		input        UpdateProxyInput
		wantUsername string
		wantPassword string
	}{
		{name: "omitted", wantUsername: "old-user", wantPassword: "old-pass"},
		{name: "null clears", input: UpdateProxyInput{ClearUsername: true, ClearPassword: true}},
		{name: "strings set", input: UpdateProxyInput{Username: "new-user", UsernameSet: true, Password: "new-pass", PasswordSet: true}, wantUsername: "new-user", wantPassword: "new-pass"},
		{name: "empty strings clear", input: UpdateProxyInput{UsernameSet: true, PasswordSet: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &updatingProxyRepoStub{
				proxyRepoStub: &proxyRepoStub{},
				proxy:         &Proxy{ID: 9, Username: "old-user", Password: "old-pass", FallbackMode: FallbackModeNone},
			}
			svc := &adminServiceImpl{proxyRepo: repo}

			got, err := svc.UpdateProxy(context.Background(), 9, &tc.input)

			require.NoError(t, err)
			require.Equal(t, tc.wantUsername, got.Username)
			require.Equal(t, tc.wantPassword, got.Password)
		})
	}
}

type proxyLatencyCacheStub struct {
	setCalls   int
	deletedIDs []int64
	latencies  map[int64]*ProxyLatencyInfo
}

func (s *proxyLatencyCacheStub) GetProxyLatencies(_ context.Context, _ []int64) (map[int64]*ProxyLatencyInfo, error) {
	return s.latencies, nil
}

func (s *proxyLatencyCacheStub) SetProxyLatency(_ context.Context, id int64, info *ProxyLatencyInfo) error {
	s.setCalls++
	if s.latencies == nil {
		s.latencies = make(map[int64]*ProxyLatencyInfo)
	}
	copy := *info
	s.latencies[id] = &copy
	return nil
}

func (s *proxyLatencyCacheStub) DeleteProxyLatency(_ context.Context, id int64) error {
	s.deletedIDs = append(s.deletedIDs, id)
	delete(s.latencies, id)
	return nil
}

func TestAdminProxyNetworkIdentityChangeInvalidatesLatencyCache(t *testing.T) {
	repo := &updatingProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		proxy:         &Proxy{ID: 9, Protocol: "http", Host: "old.example", Port: 3128, Status: StatusActive, FallbackMode: FallbackModeNone},
	}
	cache := &proxyLatencyCacheStub{latencies: map[int64]*ProxyLatencyInfo{9: {Success: true}}}
	svc := &adminServiceImpl{proxyRepo: repo, proxyLatencyCache: cache}

	_, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{Host: "new.example"})

	require.NoError(t, err)
	require.Equal(t, []int64{9}, cache.deletedIDs)
}

func TestAdminProxyStatusChangeKeepsLatencyCache(t *testing.T) {
	repo := &updatingProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		proxy:         &Proxy{ID: 9, Protocol: "http", Host: "same.example", Port: 3128, Status: StatusActive, FallbackMode: FallbackModeNone},
	}
	cache := &proxyLatencyCacheStub{}
	svc := &adminServiceImpl{proxyRepo: repo, proxyLatencyCache: cache}

	_, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{Status: "inactive"})

	require.NoError(t, err)
	require.Empty(t, cache.deletedIDs)
}

type blockingProxyProber struct {
	started chan struct{}
	release chan struct{}
}

func (p *blockingProxyProber) ProbeProxy(context.Context, string) (*ProxyExitInfo, int64, error) {
	close(p.started)
	<-p.release
	return &ProxyExitInfo{IP: "203.0.113.10"}, 25, nil
}

func TestAsyncProxyProbeDoesNotWriteAfterNetworkIdentityChanges(t *testing.T) {
	stale := &Proxy{ID: 9, Protocol: "http", Host: "old.example", Port: 3128, Status: StatusActive}
	repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: stale}
	cache := &proxyLatencyCacheStub{}
	prober := &blockingProxyProber{started: make(chan struct{}), release: make(chan struct{})}
	svc := &adminServiceImpl{proxyRepo: repo, proxyLatencyCache: cache, proxyProber: prober}
	done := make(chan struct{})
	go func() {
		svc.probeProxyLatency(context.Background(), stale)
		close(done)
	}()
	<-prober.started
	repo.proxy = &Proxy{ID: 9, Protocol: "http", Host: "new.example", Port: 3128, Status: StatusActive}
	close(prober.release)
	<-done

	require.Zero(t, cache.setCalls)
}
