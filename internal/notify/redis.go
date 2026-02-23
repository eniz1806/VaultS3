package notify

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisBackend publishes notification events to Redis via Pub/Sub or list queue.
type RedisBackend struct {
	client  *redis.Client
	channel string // pub/sub channel
	listKey string // list key for LPUSH queue mode
}

func NewRedisBackend(addr, channel, listKey string) *RedisBackend {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisBackend{client: client, channel: channel, listKey: listKey}
}

func (r *RedisBackend) Name() string {
	return "redis"
}

func (r *RedisBackend) Publish(ctx context.Context, payload []byte) error {
	if r.channel != "" {
		if err := r.client.Publish(ctx, r.channel, payload).Err(); err != nil {
			return err
		}
	}
	if r.listKey != "" {
		if err := r.client.LPush(ctx, r.listKey, payload).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (r *RedisBackend) Close() error {
	return r.client.Close()
}
