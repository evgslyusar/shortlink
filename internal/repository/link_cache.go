package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/evgslyusar/shortlink/internal/domain"
)

const (
	slugKeyPrefix = "slug:"
	fieldURL      = "url"
	fieldLinkID   = "id"
)

// LinkCache implements slug→URL caching with Redis hashes.
type LinkCache struct {
	rdb *redis.Client
}

// NewLinkCache creates a new LinkCache backed by the given Redis client.
func NewLinkCache(rdb *redis.Client) *LinkCache {
	return &LinkCache{rdb: rdb}
}

// GetLink retrieves the cached original URL and link ID for a slug.
// Returns domain.ErrNotFound if the key does not exist.
func (c *LinkCache) GetLink(ctx context.Context, slug string) (originalURL, linkID string, err error) {
	key := slugKeyPrefix + slug
	vals, err := c.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return "", "", fmt.Errorf("link_cache.GetLink: %w", err)
	}
	if len(vals) == 0 {
		return "", "", domain.ErrNotFound
	}
	return vals[fieldURL], vals[fieldLinkID], nil
}

// SetLink caches the original URL and link ID for a slug with the given TTL.
// A TTL of 0 means no expiration.
func (c *LinkCache) SetLink(ctx context.Context, slug, originalURL, linkID string, ttl time.Duration) error {
	key := slugKeyPrefix + slug
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key, fieldURL, originalURL, fieldLinkID, linkID)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("link_cache.SetLink: %w", err)
	}
	return nil
}

// DeleteLink removes the cached entry for a slug.
func (c *LinkCache) DeleteLink(ctx context.Context, slug string) error {
	if err := c.rdb.Del(ctx, slugKeyPrefix+slug).Err(); err != nil {
		return fmt.Errorf("link_cache.DeleteLink: %w", err)
	}
	return nil
}
