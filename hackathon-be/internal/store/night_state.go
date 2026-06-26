package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// UpsertNightLocation inserts or replaces a participant's location in a night.
func (s *PgStore) UpsertNightLocation(ctx context.Context, loc *models.NightLocation) error {
	if loc.ReportedAt.IsZero() {
		loc.ReportedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO night_locations (night_id, user_id, lat, lng, battery_level, reported_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (night_id, user_id)
		DO UPDATE SET lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		              battery_level = EXCLUDED.battery_level, reported_at = EXCLUDED.reported_at`,
		loc.NightID, loc.UserID, loc.Lat, loc.Lng, loc.BatteryLevel, loc.ReportedAt)
	return err
}

// GetNightLocation returns a participant's latest location in a night.
func (s *PgStore) GetNightLocation(ctx context.Context, nightID, userID uuid.UUID) (*models.NightLocation, error) {
	var l models.NightLocation
	err := s.pool.QueryRow(ctx, `
		SELECT night_id, user_id, lat, lng, battery_level, reported_at
		FROM night_locations WHERE night_id = $1 AND user_id = $2`,
		nightID, userID,
	).Scan(&l.NightID, &l.UserID, &l.Lat, &l.Lng, &l.BatteryLevel, &l.ReportedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

// ListNightLocations returns all participant locations for a night.
func (s *PgStore) ListNightLocations(ctx context.Context, nightID uuid.UUID) ([]models.NightLocation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT night_id, user_id, lat, lng, battery_level, reported_at
		FROM night_locations WHERE night_id = $1 ORDER BY reported_at DESC`, nightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.NightLocation
	for rows.Next() {
		var l models.NightLocation
		if err := rows.Scan(&l.NightID, &l.UserID, &l.Lat, &l.Lng, &l.BatteryLevel, &l.ReportedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// PropagateUserLocationToActiveNights writes the user's location into every
// active night they participate in, keeping per-night snapshots current.
func (s *PgStore) PropagateUserLocationToActiveNights(ctx context.Context, userID uuid.UUID, lat, lng float64, battery *int, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO night_locations (night_id, user_id, lat, lng, battery_level, reported_at)
		SELECT n.id, $1, $2, $3, $4, $5
		FROM nights n
		JOIN group_members gm ON gm.group_id = n.group_id AND gm.user_id = $1
		WHERE n.status = 'active'
		ON CONFLICT (night_id, user_id)
		DO UPDATE SET lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		              battery_level = EXCLUDED.battery_level, reported_at = EXCLUDED.reported_at`,
		userID, lat, lng, battery, at)
	return err
}

// PropagateUserBatteryToActiveNights updates the battery level on existing
// per-night location snapshots for every active night the user participates in.
func (s *PgStore) PropagateUserBatteryToActiveNights(ctx context.Context, userID uuid.UUID, battery int, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE night_locations nl
		SET battery_level = $2
		FROM nights n
		WHERE nl.night_id = n.id AND n.status = 'active' AND nl.user_id = $1`,
		userID, battery)
	return err
}

// UpsertParticipantStatus inserts or replaces a participant's computed status.
// It deliberately leaves last_checkin_at untouched so a recompute by check()
// never clears a participant's check-in freshness.
func (s *PgStore) UpsertParticipantStatus(ctx context.Context, st *models.ParticipantState) error {
	if st.UpdatedAt.IsZero() {
		st.UpdatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO night_participant_statuses (night_id, user_id, status, detail, distance_m, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)
		ON CONFLICT (night_id, user_id)
		DO UPDATE SET status = EXCLUDED.status, detail = EXCLUDED.detail,
		              distance_m = EXCLUDED.distance_m, updated_at = EXCLUDED.updated_at`,
		st.NightID, st.UserID, string(st.Status), st.Detail, st.DistanceM, st.UpdatedAt)
	return err
}

// GetParticipantStatus returns a single participant's latest computed status.
func (s *PgStore) GetParticipantStatus(ctx context.Context, nightID, userID uuid.UUID) (*models.ParticipantState, error) {
	var st models.ParticipantState
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT night_id, user_id, status, COALESCE(detail, ''), distance_m, last_checkin_at, updated_at
		FROM night_participant_statuses WHERE night_id = $1 AND user_id = $2`,
		nightID, userID,
	).Scan(&st.NightID, &st.UserID, &status, &st.Detail, &st.DistanceM, &st.LastCheckinAt, &st.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	st.Status = models.ParticipantStatus(status)
	return &st, nil
}

// SetParticipantCheckin records the timestamp of a participant's most recent
// explicit check-in acknowledgement, creating the status row if necessary
// without disturbing an already-computed status.
func (s *PgStore) SetParticipantCheckin(ctx context.Context, nightID, userID uuid.UUID, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO night_participant_statuses (night_id, user_id, last_checkin_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (night_id, user_id)
		DO UPDATE SET last_checkin_at = EXCLUDED.last_checkin_at`,
		nightID, userID, at)
	return err
}

// ListParticipantStatuses returns all participant statuses for a night.
func (s *PgStore) ListParticipantStatuses(ctx context.Context, nightID uuid.UUID) ([]models.ParticipantState, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT night_id, user_id, status, COALESCE(detail, ''), distance_m, last_checkin_at, updated_at
		FROM night_participant_statuses WHERE night_id = $1 ORDER BY updated_at DESC`, nightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ParticipantState
	for rows.Next() {
		var st models.ParticipantState
		var status string
		if err := rows.Scan(&st.NightID, &st.UserID, &status, &st.Detail, &st.DistanceM, &st.LastCheckinAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		st.Status = models.ParticipantStatus(status)
		out = append(out, st)
	}
	return out, rows.Err()
}
