//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type updatingProxyRepoStub struct {
	*proxyRepoStub
	proxy       *Proxy
	updateCalls int
}

type atomicUpdatingProxyRepoStub struct {
	*updatingProxyRepoStub
	atomicCalls int
}

func (s *atomicUpdatingProxyRepoStub) UpdateFields(_ context.Context, _ int64, apply func(*Proxy) error) (*Proxy, error) {
	s.atomicCalls++
	copy := *s.proxy
	if err := apply(&copy); err != nil {
		return nil, err
	}
	s.proxy = &copy
	return &copy, nil
}

func (s *updatingProxyRepoStub) GetByID(context.Context, int64) (*Proxy, error) {
	copy := *s.proxy
	return &copy, nil
}

func TestAdminProxyUpdateUsesAtomicRepositoryMergeWhenAvailable(t *testing.T) {
	repo := &atomicUpdatingProxyRepoStub{updatingProxyRepoStub: &updatingProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		proxy:         &Proxy{ID: 9, Protocol: "http", Host: "old.example", Port: 3128, FallbackMode: FallbackModeNone},
	}}
	svc := &adminServiceImpl{proxyRepo: repo}

	got, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{Name: "renamed"})

	require.NoError(t, err)
	require.Equal(t, "renamed", got.Name)
	require.Equal(t, 1, repo.atomicCalls)
	require.Zero(t, repo.updateCalls)
}

func (s *updatingProxyRepoStub) Update(_ context.Context, proxy *Proxy) error {
	s.updateCalls++
	copy := *proxy
	s.proxy = &copy
	return nil
}

func TestBothProxyUpdateServicesUseRepositoryUpdateBoundary(t *testing.T) {
	t.Run("ProxyService", func(t *testing.T) {
		repo := &updatingProxyRepoStub{
			proxyRepoStub: &proxyRepoStub{},
			proxy:         &Proxy{ID: 9, Protocol: "http", Host: "old.example", Port: 8080, Status: StatusActive},
		}
		svc := NewProxyService(repo)
		host := "new.example"

		_, err := svc.Update(context.Background(), 9, UpdateProxyRequest{Host: &host})

		require.NoError(t, err)
		require.Equal(t, 1, repo.updateCalls)
		require.Equal(t, host, repo.proxy.Host)
	})

	t.Run("adminService", func(t *testing.T) {
		repo := &updatingProxyRepoStub{
			proxyRepoStub: &proxyRepoStub{},
			proxy: &Proxy{
				ID:             9,
				Protocol:       "http",
				Host:           "old.example",
				Port:           8080,
				Status:         StatusActive,
				FallbackMode:   FallbackModeNone,
				ExpiryWarnDays: 7,
			},
		}
		svc := &adminServiceImpl{proxyRepo: repo}
		warnDays := 7

		_, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
			Host:           "new.example",
			FallbackMode:   FallbackModeNone,
			ExpiryWarnDays: &warnDays,
		})

		require.NoError(t, err)
		require.Equal(t, 1, repo.updateCalls)
		require.Equal(t, "new.example", repo.proxy.Host)
	})
}
