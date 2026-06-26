package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// FormGroup creates a group with the given creator as its first admin member.
func (s *Service) FormGroup(ctx context.Context, name string, creatorID uuid.UUID) (*models.Group, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: group name is required", ErrValidation)
	}
	if _, err := s.store.GetUser(ctx, creatorID); err != nil {
		return nil, err
	}
	g := &models.Group{ID: uuid.New(), Name: strings.TrimSpace(name)}
	if err := s.store.CreateGroup(ctx, g); err != nil {
		return nil, err
	}
	if err := s.store.AddMember(ctx, g.ID, creatorID, true); err != nil {
		return nil, err
	}
	return g, nil
}

// GetGroup returns a group by id.
func (s *Service) GetGroup(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	return s.store.GetGroup(ctx, id)
}

// ListGroups returns all groups.
func (s *Service) ListGroups(ctx context.Context) ([]models.Group, error) {
	return s.store.ListGroups(ctx)
}

// DeleteGroup removes a group.
func (s *Service) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteGroup(ctx, id)
}

// JoinGroup adds a user to a group.
func (s *Service) JoinGroup(ctx context.Context, groupID, userID uuid.UUID) error {
	if _, err := s.store.GetGroup(ctx, groupID); err != nil {
		return err
	}
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return err
	}
	return s.store.AddMember(ctx, groupID, userID, false)
}

// LeaveGroup removes a user from a group.
func (s *Service) LeaveGroup(ctx context.Context, groupID, userID uuid.UUID) error {
	return s.store.RemoveMember(ctx, groupID, userID)
}

// ListMembers returns a group's members.
func (s *Service) ListMembers(ctx context.Context, groupID uuid.UUID) ([]models.Member, error) {
	if _, err := s.store.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	return s.store.ListMembers(ctx, groupID)
}

// GetAdmins returns a group's admin members.
func (s *Service) GetAdmins(ctx context.Context, groupID uuid.UUID) ([]models.Member, error) {
	if _, err := s.store.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	return s.store.ListAdmins(ctx, groupID)
}

// SetAdmin sets or clears a member's admin flag.
func (s *Service) SetAdmin(ctx context.Context, groupID, userID uuid.UUID, isAdmin bool) error {
	return s.store.SetAdmin(ctx, groupID, userID, isAdmin)
}

// CurrentNight returns the group's current night, or ErrNotFound if none.
func (s *Service) CurrentNight(ctx context.Context, groupID uuid.UUID) (*models.Night, error) {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if g.CurrNightID == nil {
		return nil, fmt.Errorf("%w: group has no current night", ErrNotFound)
	}
	return s.store.GetNight(ctx, *g.CurrNightID)
}

// CreateAgent registers a monitoring agent persona.
func (s *Service) CreateAgent(ctx context.Context, name, description string) (*models.Agent, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: agent name is required", ErrValidation)
	}
	a := &models.Agent{ID: uuid.New(), Name: strings.TrimSpace(name), Description: strings.TrimSpace(description)}
	if err := s.store.CreateAgent(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// GetAgent returns an agent by id.
func (s *Service) GetAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	return s.store.GetAgent(ctx, id)
}

// ListAgents returns all agents.
func (s *Service) ListAgents(ctx context.Context) ([]models.Agent, error) {
	return s.store.ListAgents(ctx)
}
