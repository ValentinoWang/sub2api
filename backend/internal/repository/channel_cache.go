package repository

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	channelCachePubSubKey            = "channel_cache_updated"
	channelCacheGenerationKey        = "channel_cache_generation"
	channelCacheGenerationPollPeriod = time.Second
)

var channelCacheNotifyScript = redis.NewScript(`
local generation = redis.call("INCR", KEYS[1])
redis.call("PUBLISH", KEYS[2], tostring(generation))
return generation
`)

type channelCache struct {
	rdb *redis.Client
}

// NewChannelCache creates the Redis-backed channel cache invalidation bus.
func NewChannelCache(rdb *redis.Client) service.ChannelCachePubSub {
	return &channelCache{rdb: rdb}
}

// NotifyUpdate notifies all instances to invalidate their local channel cache.
func (c *channelCache) NotifyUpdate(ctx context.Context) error {
	return channelCacheNotifyScript.Run(ctx, c.rdb, []string{channelCacheGenerationKey, channelCachePubSubKey}).Err()
}

// SubscribeUpdates subscribes to channel cache invalidation notifications.
func (c *channelCache) SubscribeUpdates(ctx context.Context, handler func()) {
	go func() {
		lastSeen := uint64(0)
		readGeneration := func() (uint64, error) {
			value, err := c.rdb.Get(ctx, channelCacheGenerationKey).Uint64()
			if err == redis.Nil {
				return 0, nil
			}
			return value, err
		}
		observeGeneration := func(generation uint64) {
			if generation <= lastSeen {
				return
			}
			lastSeen = generation
			handler()
		}
		if generation, err := readGeneration(); err == nil {
			lastSeen = generation
		} else {
			slog.Warn("failed to read initial channel cache generation", "error", err)
		}

		pubsub := c.rdb.Subscribe(ctx, channelCachePubSubKey)
		defer func() {
			if err := pubsub.Close(); err != nil {
				slog.Warn("failed to close channel cache subscriber", "error", err)
			}
		}()

		messages := pubsub.Channel()
		ticker := time.NewTicker(channelCacheGenerationPollPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messages:
				if !ok {
					slog.Warn("channel cache subscriber stopped", "reason", "channel_closed")
					return
				}
				if message != nil {
					generation, err := strconv.ParseUint(message.Payload, 10, 64)
					if err != nil {
						if generation, readErr := readGeneration(); readErr == nil {
							observeGeneration(generation)
						}
						continue
					}
					observeGeneration(generation)
				}
			case <-ticker.C:
				generation, err := readGeneration()
				if err == nil {
					observeGeneration(generation)
				}
			}
		}
	}()
}
