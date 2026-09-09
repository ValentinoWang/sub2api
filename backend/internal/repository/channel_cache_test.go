package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestChannelCachePubSub(t *testing.T) {
	redisServer := miniredis.RunT(t)
	publisherRedis := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	subscriberRedis := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = publisherRedis.Close()
		_ = subscriberRedis.Close()
	})

	publisher := NewChannelCache(publisherRedis)
	subscriber := NewChannelCache(subscriberRedis)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	received := make(chan struct{}, 1)
	subscriber.SubscribeUpdates(ctx, func() {
		select {
		case received <- struct{}{}:
		default:
		}
	})

	require.Eventually(t, func() bool {
		return redisServer.PubSubNumSub(channelCachePubSubKey)[channelCachePubSubKey] == 1
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, publisher.NotifyUpdate(context.Background()))

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("channel cache invalidation notification was not received")
	}
}

func TestChannelCacheGenerationRepairsMissedPubSubNotification(t *testing.T) {
	redisServer := miniredis.RunT(t)
	subscriberRedis := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = subscriberRedis.Close() })

	subscriber := NewChannelCache(subscriberRedis)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	received := make(chan struct{}, 1)
	subscriber.SubscribeUpdates(ctx, func() {
		select {
		case received <- struct{}{}:
		default:
		}
	})
	require.Eventually(t, func() bool {
		return redisServer.PubSubNumSub(channelCachePubSubKey)[channelCachePubSubKey] == 1
	}, time.Second, 10*time.Millisecond)

	// Drop the subscriber connection, restart Redis with its durable keys, then
	// commit a generation without PubSub. The periodic read must repair the miss.
	redisServer.Close()
	require.NoError(t, redisServer.Restart())
	require.Eventually(t, func() bool {
		return subscriberRedis.Ping(context.Background()).Err() == nil
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, subscriberRedis.Incr(context.Background(), channelCacheGenerationKey).Err())
	select {
	case <-received:
	case <-time.After(2 * channelCacheGenerationPollPeriod):
		t.Fatal("durable channel cache generation was not observed")
	}
}
