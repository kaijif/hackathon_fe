package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

const groupColumns = `id, name, active, curr_night_id, created_at, updated_at`

func scanGroup(row rowScanner) (models.Group, error) {
	var g models.Group
	err := row.Scan(&g.ID, &g.Name, &g.Active, &g.CurrNightID, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

// CreateGroup inserts a group.
func (s *PgStore) CreateGroup(ctx context.Context, g *models.Group) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO groups (id, name, active)
		VALUES ($1, $2, $3)
		RETURNING created_at, updated_at`,
		g.ID, g.Name, g.Active,
	).Scan(&g.CreatedAt, &g.UpdatedAt)
}

// GetGroup returns a group by id, or ErrNotFound.
func (s *PgStore) GetGroup(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	g, err := scanGroup(s.pool.QueryRow(ctx, `SELECT `+groupColumns+` FROM groups WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

// ListGroups returns all groups.
func (s *PgStore) ListGroups(ctx context.Context) ([]models.Group, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+groupColumns+` FROM groups ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGroups(rows)
}

// ListGroupsForUser returns the groups a user belongs to. When onlyActiveNight
// is true, only groups that currently have an active night are returned.
func (s *PgStore) ListGroupsForUser(ctx context.Context, userID uuid.UUID, onlyActiveNight bool) ([]models.Group, error) {
	q := `
		SELECT ` + prefixCols("g", groupColumns) + `
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = $1`
	if onlyActiveNight {
		q += ` WHERE EXISTS (SELECT 1 FROM nights n WHERE n.group_id = g.id AND n.status = 'active')`
	}
	q += ` ORDER BY g.created_at`

	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGroups(rows)
}

// DeleteGroup removes a group and (via cascade) its memberships and nights.
func (s *PgStore) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetGroupNight updates the group's current night pointer and active flag.
func (s *PgStore) SetGroupNight(ctx context.Context, groupID uuid.UUID, nightID *uuid.UUID, active bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE groups SET curr_night_id = $2, active = $3, updated_at = now() WHERE id = $1`,
		groupID, nightID, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddMember adds a user to a group, promoting to admin if requested. It never
// demotes an existing admin.
func (s *PgStore) AddMember(ctx context.Context, groupID, userID uuid.UUID, isAdmin bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id, is_admin)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, user_id)
		DO UPDATE SET is_admin = group_members.is_admin OR EXCLUDED.is_admin`,
		groupID, userID, isAdmin)
	return err
}

// RemoveMember removes a user from a group.
func (s *PgStore) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAdmin sets the admin flag for an existing member.
func (s *PgStore) SetAdmin(ctx context.Context, groupID, userID uuid.UUID, isAdmin bool) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE group_members SET is_admin = $3 WHERE group_id = $1 AND user_id = $2`,
		groupID, userID, isAdmin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// IsMember reports whether a user belongs to a group.
func (s *PgStore) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`,
		groupID, userID).Scan(&exists)
	return exists, err
}

// ListMembers returns all members of a group.
func (s *PgStore) ListMembers(ctx context.Context, groupID uuid.UUID) ([]models.Member, error) {
	return s.listMembers(ctx, groupID, false)
}

// ListAdmins returns only the admin members of a group.
func (s *PgStore) ListAdmins(ctx context.Context, groupID uuid.UUID) ([]models.Member, error) {
	return s.listMembers(ctx, groupID, true)
}

func (s *PgStore) listMembers(ctx context.Context, groupID uuid.UUID, adminsOnly bool) ([]models.Member, error) {
	q := `
		SELECT gm.user_id, u.name, gm.is_admin, gm.joined_at
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1`
	if adminsOnly {
		q += ` AND gm.is_admin = true`
	}
	q += ` ORDER BY gm.joined_at`

	rows, err := s.pool.Query(ctx, q, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Member
	for rows.Next() {
		var m models.Member
		if err := rows.Scan(&m.UserID, &m.Name, &m.IsAdmin, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func collectGroups(rows pgx.Rows) ([]models.Group, error) {
	var out []models.Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
