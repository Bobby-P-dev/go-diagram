package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, redisURL string) (*redis.Client, error) {
	return connect(ctx, redisURL, redis.NewClient)
}

func connect(ctx context.Context, redisURL string, newClient func(*redis.Options) *redis.Client) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL")
	}
	client := newClient(options)
	pingContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := client.Ping(pingContext).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("Redis connection failed: %w", err)
	}
	log.Println("Redis connected successfully for async AI queue")
	return client, nil
}

func Close(client *redis.Client) {
	if client != nil {
		_ = client.Close()
	}
}
