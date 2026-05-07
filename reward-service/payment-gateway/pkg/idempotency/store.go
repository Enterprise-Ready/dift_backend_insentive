package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix  = "idempotency:"
	defaultTTL = 24 * time.Hour
)

// Store manages idempotency keys using Redis
type Store struct {
	client redis.UniversalClient
	ttl    time.Duration
}

type IdempotencyRecord struct {
	Key       string          `json:"key"`
	Status    string          `json:"status"` // "processing" | "completed"
	Response  json.RawMessage `json:"response,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func NewStore(client redis.UniversalClient, ttl time.Duration) *Store {
	if ttl == 0 {
		ttl = defaultTTL
	}
	return &Store{client: client, ttl: ttl}
}

// Lock acquires idempotency lock and returns existing result if found
func (s *Store) Lock(ctx context.Context, merchantID, key string) (*IdempotencyRecord, bool, error) {
	redisKey := s.buildKey(merchantID, key)

	// Try to get existing record
	existing, err := s.client.Get(ctx, redisKey).Bytes()
	if err == nil {
		var record IdempotencyRecord
		if err := json.Unmarshal(existing, &record); err == nil {
			if record.Status == "completed" {
				return &record, true, nil // Already processed
			}
			// Still processing - conflict
			return nil, false, fmt.Errorf("request with idempotency key %s is still processing", key)
		}
	}

	// Create new lock record
	record := &IdempotencyRecord{
		Key:       key,
		Status:    "processing",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, _ := json.Marshal(record)
	// Use NX (set if not exists)
	set, err := s.client.SetNX(ctx, redisKey, data, s.ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !set {
		return nil, false, fmt.Errorf("concurrent request with key %s", key)
	}

	return record, false, nil
}

// Complete saves the final response for an idempotency key
func (s *Store) Complete(ctx context.Context, merchantID, key string, response interface{}) error {
	redisKey := s.buildKey(merchantID, key)

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	record := &IdempotencyRecord{
		Key:       key,
		Status:    "completed",
		Response:  responseBytes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, _ := json.Marshal(record)
	return s.client.Set(ctx, redisKey, data, s.ttl).Err()
}

// Release removes a processing lock (on error)
func (s *Store) Release(ctx context.Context, merchantID, key string) error {
	redisKey := s.buildKey(merchantID, key)
	return s.client.Del(ctx, redisKey).Err()
}

func (s *Store) buildKey(merchantID, key string) string {
	return fmt.Sprintf("%s%s:%s", keyPrefix, merchantID, key)
}
