package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestProxyLatencyCacheDeleteRemovesPermanentEntry(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &proxyLatencyCache{rdb: client}
	ctx := context.Background()

	require.NoError(t, cache.SetProxyLatency(ctx, 9, &service.ProxyLatencyInfo{IdentityHash: "identity", Success: true}))
	require.True(t, server.Exists(proxyLatencyKey(9)))
	require.NoError(t, cache.DeleteProxyLatency(ctx, 9))
	require.False(t, server.Exists(proxyLatencyKey(9)))
}
