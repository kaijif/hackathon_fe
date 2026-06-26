package store

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// AddDeviceToken registers (or re-assigns) a device token. The token is unique;
// re-registering an existing token updates its owner and platform.
func (s *PgStore) AddDeviceToken(ctx context.Context, dt *models.DeviceToken) error {
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO device_tokens (id, user_id, platform, token)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token)
		DO UPDATE SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform, updated_at = now()
		RETURNING id, created_at, updated_at`,
		dt.ID, dt.UserID, dt.Platform, dt.Token,
	).Scan(&dt.ID, &dt.CreatedAt, &dt.UpdatedAt); err != nil {
		slog.Default().Error("AddDeviceToken: insert failed",
			"userId", dt.UserID, "platform", dt.Platform, "token", dt.Token, "error", err)
		return err
	}

	// Diagnostic: prove the row committed and reveal exactly which database the
	// app is writing to, so it can be compared against the DB you query with psql.
	var (
		dbName, dbUser, schema, serverAddr string
		thisToken, total                   int
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT current_database(), current_user, current_schema(),
		       COALESCE(inet_server_addr()::text, 'local'),
		       (SELECT count(*) FROM device_tokens WHERE token = $1),
		       (SELECT count(*) FROM device_tokens)`,
		dt.Token,
	).Scan(&dbName, &dbUser, &schema, &serverAddr, &thisToken, &total); err != nil {
		slog.Default().Warn("AddDeviceToken: post-insert read-back failed", "error", err)
		return nil
	}
	slog.Default().Info("AddDeviceToken: stored device token (read-back)",
		"deviceId", dt.ID, "userId", dt.UserID, "platform", dt.Platform, "token", dt.Token,
		"database", dbName, "dbUser", dbUser, "schema", schema, "serverAddr", serverAddr,
		"rowsForThisToken", thisToken, "deviceTokensTotal", total)
	return nil
}

// RemoveDeviceToken unregisters a device token for a user.
func (s *PgStore) RemoveDeviceToken(ctx context.Context, userID uuid.UUID, token string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM device_tokens WHERE user_id = $1 AND token = $2`, userID, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDeviceTokensForUser returns all device tokens registered by a user.
func (s *PgStore) ListDeviceTokensForUser(ctx context.Context, userID uuid.UUID) ([]models.DeviceToken, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, platform, token, created_at, updated_at
		FROM device_tokens WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.DeviceToken
	for rows.Next() {
		var dt models.DeviceToken
		if err := rows.Scan(&dt.ID, &dt.UserID, &dt.Platform, &dt.Token, &dt.CreatedAt, &dt.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, dt)
	}
	return out, rows.Err()
}
