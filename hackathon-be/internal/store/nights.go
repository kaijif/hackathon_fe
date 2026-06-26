package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

const nightColumns = `id, group_id, agent_id, center_lat, center_lng,
	time_limit_min, check_in_limit_min, check_in_every_min, max_range_m,
	low_battery_threshold, status, started_at, ended_at, last_checked_at,
	created_at, updated_at`

func scanNight(row rowScanner) (models.Night, error) {
	var n models.Night
	var status string
	err := row.Scan(&n.ID, &n.GroupID, &n.AgentID, &n.CenterLat, &n.CenterLng,
		&n.TimeLimitMin, &n.CheckInLimitMin, &n.CheckInEveryMin, &n.MaxRangeM,
		&n.LowBatteryThreshold, &status, &n.StartedAt, &n.EndedAt, &n.LastCheckedAt,
		&n.CreatedAt, &n.UpdatedAt)
	n.Status = models.NightStatus(status)
	return n, err
}

// CreateNight inserts a night in its initial (pending) state.
func (s *PgStore) CreateNight(ctx context.Context, n *models.Night) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO nights (id, group_id, agent_id, center_lat, center_lng,
			time_limit_min, check_in_limit_min, check_in_every_min, max_range_m,
			low_battery_threshold, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`,
		n.ID, n.GroupID, n.AgentID, n.CenterLat, n.CenterLng,
		n.TimeLimitMin, n.CheckInLimitMin, n.CheckInEveryMin, n.MaxRangeM,
		n.LowBatteryThreshold, string(n.Status),
	).Scan(&n.CreatedAt, &n.UpdatedAt)
}

// GetNight returns a night by id, or ErrNotFound.
func (s *PgStore) GetNight(ctx context.Context, id uuid.UUID) (*models.Night, error) {
	n, err := scanNight(s.pool.QueryRow(ctx, `SELECT `+nightColumns+` FROM nights WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &n, nil
}

// ListNightsByGroup returns all nights for a group, newest first.
func (s *PgStore) ListNightsByGroup(ctx context.Context, groupID uuid.UUID) ([]models.Night, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+nightColumns+` FROM nights WHERE group_id = $1 ORDER BY created_at DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectNights(rows)
}

// ListActiveNights returns all nights currently in the active state.
func (s *PgStore) ListActiveNights(ctx context.Context) ([]models.Night, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+nightColumns+` FROM nights WHERE status = 'active' ORDER BY started_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectNights(rows)
}

// UpdateNightLifecycle transitions a night's status and timestamps. Passing a
// nil startedAt/endedAt preserves the existing value.
func (s *PgStore) UpdateNightLifecycle(ctx context.Context, id uuid.UUID, status models.NightStatus, startedAt, endedAt *time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE nights
		SET status = $2,
		    started_at = COALESCE($3, started_at),
		    ended_at = COALESCE($4, ended_at),
		    updated_at = now()
		WHERE id = $1`,
		id, string(status), startedAt, endedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateNightRange updates the maximum allowed range from the center.
func (s *PgStore) UpdateNightRange(ctx context.Context, id uuid.UUID, maxRangeM int) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE nights SET max_range_m = $2, updated_at = now() WHERE id = $1`, id, maxRangeM)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateNightCenter updates the night's center point.
func (s *PgStore) UpdateNightCenter(ctx context.Context, id uuid.UUID, lat, lng float64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE nights SET center_lat = $2, center_lng = $3, updated_at = now() WHERE id = $1`, id, lat, lng)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetNightLastChecked records the timestamp of the most recent check() run.
func (s *PgStore) SetNightLastChecked(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE nights SET last_checked_at = $2, updated_at = now() WHERE id = $1`, id, at)
	return err
}

// DeleteNight removes a night.
func (s *PgStore) DeleteNight(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM nights WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func collectNights(rows pgx.Rows) ([]models.Night, error) {
	var out []models.Night
	for rows.Next() {
		n, err := scanNight(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
