package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/evgslyusar/shortlink/internal/domain"
)

const slugKeyPrefix = "slug:"

// LinkCache implements slug→URL caching with Redis.
type LinkCache struct {
	rdb *redis.Client
}

// NewLinkCache creates a new LinkCache backed by the given Redis client.
func NewLinkCache(rdb *redis.Client) *LinkCache {
	return &LinkCache{rdb: rdb}
}

// GetOriginalURL retrieves the cached original URL for a slug.
// Returns domain.ErrNotFound if the key does not exist.
func (c *LinkCache) GetOriginalURL(ctx context.Context, slug string) (string, error) {
	val, err := c.rdb.Get(ctx, slugKeyPrefix+slug).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("link_cache.GetOriginalURL: %w", err)
	}
	return val, nil
}

// SetOriginalURL caches the original URL for a slug with the given TTL.
// A TTL of 0 means no expiration.
func (c *LinkCache) SetOriginalURL(ctx context.Context, slug, url string, ttl time.Duration) error {
	if err := c.rdb.Set(ctx, slugKeyPrefix+slug, url, ttl).Err(); err != nil {
		return fmt.Errorf("link_cache.SetOriginalURL: %w", err)
	}
	return nil
}

// DeleteOriginalURL removes the cached URL for a slug.
func (c *LinkCache) DeleteOriginalURL(ctx context.Context, slug string) error {
	if err := c.rdb.Del(ctx, slugKeyPrefix+slug).Err(); err != nil {
		return fmt.Errorf("link_cache.DeleteOriginalURL: %w", err)
	}
	return nil
}
