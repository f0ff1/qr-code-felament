package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	*redis.Client
}

type Options struct {
	Addr     string
	Password string
	DB       int
}

func NewClient(addr string) (*Client, error) {
	return NewClientWithOptions(Options{Addr: addr})
}

func NewClientWithOptions(opts Options) (*Client, error) {
	if opts.Addr == "" {
		return nil, fmt.Errorf("redis address is empty")
	}

	var client *redis.Client
	if strings.Contains(opts.Addr, "://") {
		parsed, err := redis.ParseURL(opts.Addr)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		if opts.Password != "" && parsed.Password == "" {
			parsed.Password = opts.Password
		}
		if opts.DB != 0 {
			parsed.DB = opts.DB
		}
		parsed.PoolSize = 64
		parsed.MinIdleConns = 8
		parsed.DialTimeout = 2 * time.Second
		parsed.ReadTimeout = 1 * time.Second
		parsed.WriteTimeout = 1 * time.Second
		parsed.PoolTimeout = 2 * time.Second
		client = redis.NewClient(parsed)
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:         opts.Addr,
			Password:     opts.Password,
			DB:           opts.DB,
			PoolSize:     64,
			MinIdleConns: 8,
			DialTimeout:  2 * time.Second,
			ReadTimeout:  1 * time.Second,
			WriteTimeout: 1 * time.Second,
			PoolTimeout:  2 * time.Second,
		})
	}

	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
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
	if err := c.Set(ctx, key, payload, 2*time.Minute).Err(); err != nil {
		return fmt.Errorf("write cache %s: %w", key, err)
	}
	return nil
}
