package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/evgslyusar/shortlink/internal/domain"
)

// LinkPostgres implements link persistence against PostgreSQL.
type LinkPostgres struct {
	db *pgxpool.Pool
}

// NewLinkPostgres creates a new LinkPostgres repository.
func NewLinkPostgres(db *pgxpool.Pool) *LinkPostgres {
	return &LinkPostgres{db: db}
}

const pgUniqueViolation = "23505"

// CreateLink inserts a new link into the database.
// Returns domain.ErrAlreadyExists if the slug is already taken.
func (r *LinkPostgres) CreateLink(ctx context.Context, link *domain.Link) error {
	const query = `
		INSERT INTO links (id, slug, original_url, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.Exec(ctx, query,
		link.ID, link.Slug, link.OriginalURL, link.UserID, link.ExpiresAt, link.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("link_postgres.CreateLink: %w", domain.ErrAlreadyExists)
		}
		return fmt.Errorf("link_postgres.CreateLink: %w", err)
	}

	return nil
}

// FindBySlug retrieves a link by its slug.
// Returns domain.ErrNotFound if no link exists with the given slug.
func (r *LinkPostgres) FindBySlug(ctx context.Context, slug string) (*domain.Link, error) {
	const query = `
		SELECT id, slug, original_url, user_id, expires_at, created_at
		FROM links
		WHERE slug = $1`

	var l domain.Link
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&l.ID, &l.Slug, &l.OriginalURL, &l.UserID, &l.ExpiresAt, &l.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("link_postgres.FindBySlug: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("link_postgres.FindBySlug: %w", err)
	}

	return &l, nil
}

// ListByUser returns a paginated list of links owned by the given user,
// along with the total count. Expects already-validated page and perPage values.
func (r *LinkPostgres) ListByUser(ctx context.Context, userID string, page, perPage int) ([]domain.Link, int, error) {
	offset := (page - 1) * perPage

	// Count total.
	const countQuery = `SELECT COUNT(*) FROM links WHERE user_id = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("link_postgres.ListByUser count: %w", err)
	}

	// Fetch page.
	const listQuery = `
		SELECT id, slug, original_url, user_id, expires_at, created_at
		FROM links
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, listQuery, userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("link_postgres.ListByUser query: %w", err)
	}
	defer rows.Close()

	var links []domain.Link
	for rows.Next() {
		var l domain.Link
		if err := rows.Scan(&l.ID, &l.Slug, &l.OriginalURL, &l.UserID, &l.ExpiresAt, &l.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("link_postgres.ListByUser scan: %w", err)
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("link_postgres.ListByUser rows: %w", err)
	}

	return links, total, nil
}

// DeleteBySlugAndUser removes a link by slug, only if owned by the given user.
// Returns domain.ErrNotFound if no matching link exists, which covers both
// "slug does not exist" and "slug exists but belongs to another user".
func (r *LinkPostgres) DeleteBySlugAndUser(ctx context.Context, slug, userID string) error {
	const query = `DELETE FROM links WHERE slug = $1 AND user_id = $2`

	result, err := r.db.Exec(ctx, query, slug, userID)
	if err != nil {
		return fmt.Errorf("link_postgres.DeleteBySlugAndUser: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("link_postgres.DeleteBySlugAndUser: %w", domain.ErrNotFound)
	}

	return nil
}
