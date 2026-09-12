package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	*redis.Client
}

func NewClient(addr string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("redis address is empty")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return &Client{Client: client}, nil
}

// WriteJSON stores the latest authoritative runtime snapshot. PostgreSQL remains
// the source of truth; cache failures must not prevent a successful DB update.
func (c *Client) WriteJSON(ctx context.Context, key string, value any) error {
	if c == nil || c.Client == nil {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal cache value: %w", err)
	}
	if err := c.Set(ctx, key, payload, 0).Err(); err != nil {
		return fmt.Errorf("write cache %s: %w", key, err)
	}
	return nil
}
