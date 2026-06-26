package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

const userColumns = `id, name, COALESCE(trusted_contact, ''), lat, lng, battery_level, location_updated_at, created_at, updated_at`

func scanUser(row rowScanner) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Name, &u.TrustedContact, &u.Lat, &u.Lng,
		&u.BatteryLevel, &u.LocationUpdatedAt, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// CreateUser inserts a user, populating server-generated timestamps.
func (s *PgStore) CreateUser(ctx context.Context, u *models.User) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO users (id, name, trusted_contact, lat, lng, battery_level, location_updated_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7)
		RETURNING created_at, updated_at`,
		u.ID, u.Name, u.TrustedContact, u.Lat, u.Lng, u.BatteryLevel, u.LocationUpdatedAt,
	).Scan(&u.CreatedAt, &u.UpdatedAt)
}

// GetUser returns a user by id, or ErrNotFound.
func (s *PgStore) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// ListUsers returns all users ordered by creation time.
func (s *PgStore) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUserLocation updates a user's coordinates (and optional battery).
func (s *PgStore) UpdateUserLocation(ctx context.Context, id uuid.UUID, lat, lng float64, battery *int, at time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users
		SET lat = $2,
		    lng = $3,
		    battery_level = COALESCE($4, battery_level),
		    location_updated_at = $5,
		    updated_at = now()
		WHERE id = $1`,
		id, lat, lng, battery, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserBattery updates only a user's battery level.
func (s *PgStore) UpdateUserBattery(ctx context.Context, id uuid.UUID, battery int, at time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users
		SET battery_level = $2, updated_at = $3
		WHERE id = $1`,
		id, battery, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateAgent inserts a monitoring agent.
func (s *PgStore) CreateAgent(ctx context.Context, a *models.Agent) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO agents (id, name, description)
		VALUES ($1, $2, NULLIF($3, ''))
		RETURNING created_at`,
		a.ID, a.Name, a.Description,
	).Scan(&a.CreatedAt)
}

// GetAgent returns an agent by id, or ErrNotFound.
func (s *PgStore) GetAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	var a models.Agent
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(description, ''), created_at FROM agents WHERE id = $1`, id,
	).Scan(&a.ID, &a.Name, &a.Description, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// ListAgents returns all agents.
func (s *PgStore) ListAgents(ctx context.Context) ([]models.Agent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, COALESCE(description, ''), created_at FROM agents ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
