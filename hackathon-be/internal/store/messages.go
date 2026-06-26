package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// CreateMessage persists an outbound message / notification.
func (s *PgStore) CreateMessage(ctx context.Context, m *models.Message) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO messages (id, night_id, group_id, recipient_user_id, recipient_contact, kind, body, sender)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, NULLIF($8, ''))
		RETURNING created_at`,
		m.ID, m.NightID, m.GroupID, m.RecipientUserID, m.RecipientContact, m.Kind, m.Body, m.Sender,
	).Scan(&m.CreatedAt)
}

// ListMessagesByNight returns all messages associated with a night, newest first.
func (s *PgStore) ListMessagesByNight(ctx context.Context, nightID uuid.UUID) ([]models.Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, night_id, group_id, recipient_user_id, COALESCE(recipient_contact, ''),
		       kind, body, COALESCE(sender, ''), created_at
		FROM messages WHERE night_id = $1 ORDER BY created_at DESC`, nightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.NightID, &m.GroupID, &m.RecipientUserID, &m.RecipientContact,
			&m.Kind, &m.Body, &m.Sender, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
