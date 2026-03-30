package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/evgslyusar/shortlink/internal/domain"
)

// ClickPostgres implements click persistence against PostgreSQL.
type ClickPostgres struct {
	db *pgxpool.Pool
}

// NewClickPostgres creates a new ClickPostgres repository.
func NewClickPostgres(db *pgxpool.Pool) *ClickPostgres {
	return &ClickPostgres{db: db}
}

// BatchInsert inserts multiple clicks in a single query.
func (r *ClickPostgres) BatchInsert(ctx context.Context, clicks []domain.Click) error {
	if len(clicks) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("INSERT INTO clicks (id, link_id, clicked_at, country, referer, user_agent) VALUES ")

	args := make([]any, 0, len(clicks)*6)
	for i, c := range clicks {
		if i > 0 {
			b.WriteString(", ")
		}
		base := i * 6
		fmt.Fprintf(&b, "($%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6)
		args = append(args, c.ID, c.LinkID, c.ClickedAt, c.Country, c.Referer, c.UserAgent)
	}

	_, err := r.db.Exec(ctx, b.String(), args...)
	if err != nil {
		return fmt.Errorf("click_postgres.BatchInsert: %w", err)
	}
	return nil
}

// CountByLink returns the total number of clicks for a link.
func (r *ClickPostgres) CountByLink(ctx context.Context, linkID string) (int64, error) {
	const query = `SELECT COUNT(*) FROM clicks WHERE link_id = $1`

	var count int64
	if err := r.db.QueryRow(ctx, query, linkID).Scan(&count); err != nil {
		return 0, fmt.Errorf("click_postgres.CountByLink: %w", err)
	}
	return count, nil
}

// CountByDay returns daily click counts for a link over the last N days,
// ordered by date descending.
func (r *ClickPostgres) CountByDay(ctx context.Context, linkID string, days int) ([]domain.DayStat, error) {
	const query = `
		SELECT clicked_at::date AS day, COUNT(*) AS count
		FROM clicks
		WHERE link_id = $1 AND clicked_at >= now() - make_interval(days => $2)
		GROUP BY day
		ORDER BY day DESC`

	rows, err := r.db.Query(ctx, query, linkID, days)
	if err != nil {
		return nil, fmt.Errorf("click_postgres.CountByDay: %w", err)
	}
	defer rows.Close()

	var stats []domain.DayStat
	for rows.Next() {
		var s domain.DayStat
		if err := rows.Scan(&s.Date, &s.Count); err != nil {
			return nil, fmt.Errorf("click_postgres.CountByDay scan: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("click_postgres.CountByDay rows: %w", err)
	}
	return stats, nil
}

// CountByCountry returns click counts grouped by country for a link,
// ordered by count descending.
func (r *ClickPostgres) CountByCountry(ctx context.Context, linkID string) ([]domain.CountryStat, error) {
	const query = `
		SELECT COALESCE(country, 'XX') AS country, COUNT(*) AS count
		FROM clicks
		WHERE link_id = $1
		GROUP BY country
		ORDER BY count DESC`

	rows, err := r.db.Query(ctx, query, linkID)
	if err != nil {
		return nil, fmt.Errorf("click_postgres.CountByCountry: %w", err)
	}
	defer rows.Close()

	var stats []domain.CountryStat
	for rows.Next() {
		var s domain.CountryStat
		if err := rows.Scan(&s.Country, &s.Count); err != nil {
			return nil, fmt.Errorf("click_postgres.CountByCountry scan: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("click_postgres.CountByCountry rows: %w", err)
	}
	return stats, nil
}
