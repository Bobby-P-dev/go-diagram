package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	jobQueueName = "queue:ai_orchestration"
	jobStatusTTL = 2 * time.Hour
)

type RedisJobStore struct {
	client *redis.Client
}

func NewRedisJobStore(client *redis.Client) *RedisJobStore {
	return &RedisJobStore{client: client}
}

func (store *RedisJobStore) SetStatus(ctx context.Context, jobID, status string) error {
	return store.client.Set(ctx, jobStatusKey(jobID), status, jobStatusTTL).Err()
}

func (store *RedisJobStore) GetStatus(ctx context.Context, jobID string) (string, error) {
	return store.get(ctx, jobStatusKey(jobID))
}

func (store *RedisJobStore) GetJobProject(ctx context.Context, jobID string) (string, error) {
	return store.get(ctx, jobProjectKey(jobID))
}

func (store *RedisJobStore) Enqueue(ctx context.Context, payload []byte) error {
	return store.client.RPush(ctx, jobQueueName, payload).Err()
}

func (store *RedisJobStore) get(ctx context.Context, key string) (string, error) {
	value, err := store.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return value, err
}

func jobStatusKey(jobID string) string {
	return fmt.Sprintf("job:%s:status", jobID)
}

func jobProjectKey(jobID string) string {
	return fmt.Sprintf("job:%s:project", jobID)
}
