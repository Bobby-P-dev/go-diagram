package config

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func ConnectRedis() {
	opt, err := redis.ParseURL(Env.RedisURL)
	if err != nil {
		log.Printf("Warning: Invalid REDIS_URL '%s': %v. Background queue will operate in direct fallback mode.", Env.RedisURL, err)
		return
	}

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis not reachable at %s (%v). Background queue will fallback to direct API dispatch.", Env.RedisURL, err)
		return
	}

	RedisClient = client
	log.Println("Redis connected successfully for async AI queue")
}

func CloseRedis() {
	if RedisClient != nil {
		_ = RedisClient.Close()
	}
}
