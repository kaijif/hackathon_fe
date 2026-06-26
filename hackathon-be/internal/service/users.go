package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// CreateUserInput is the payload for creating a user.
type CreateUserInput struct {
	Name           string
	TrustedContact string
	Lat            *float64
	Lng            *float64
	Battery        *int
}

// CreateUser registers a new user (client app).
func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (*models.User, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if err := validateBattery(in.Battery); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	u := &models.User{
		ID:             uuid.New(),
		Name:           strings.TrimSpace(in.Name),
		TrustedContact: strings.TrimSpace(in.TrustedContact),
		Lat:            in.Lat,
		Lng:            in.Lng,
		BatteryLevel:   in.Battery,
	}
	if in.Lat != nil && in.Lng != nil {
		u.LocationUpdatedAt = &now
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// GetUser returns a user by id.
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.store.GetUser(ctx, id)
}

// ListUsers returns all users.
func (s *Service) ListUsers(ctx context.Context) ([]models.User, error) {
	return s.store.ListUsers(ctx)
}

// SetLocation updates a user's location (and optional battery) and propagates
// it into every active night the user participates in.
func (s *Service) SetLocation(ctx context.Context, id uuid.UUID, lat, lng float64, battery *int) (*models.User, error) {
	if err := validateBattery(battery); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if err := s.store.UpdateUserLocation(ctx, id, lat, lng, battery, now); err != nil {
		return nil, err
	}
	if err := s.store.PropagateUserLocationToActiveNights(ctx, id, lat, lng, battery, now); err != nil {
		return nil, err
	}
	return s.store.GetUser(ctx, id)
}

// SetBattery updates only a user's battery level, propagating it into active nights.
func (s *Service) SetBattery(ctx context.Context, id uuid.UUID, battery int) (*models.User, error) {
	if err := validateBattery(&battery); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if err := s.store.UpdateUserBattery(ctx, id, battery, now); err != nil {
		return nil, err
	}
	if err := s.store.PropagateUserBatteryToActiveNights(ctx, id, battery, now); err != nil {
		return nil, err
	}
	return s.store.GetUser(ctx, id)
}

// ListGroupsForUser returns the groups a user belongs to, optionally filtered to
// only those with an active night.
func (s *Service) ListGroupsForUser(ctx context.Context, userID uuid.UUID, onlyActiveNight bool) ([]models.Group, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.store.ListGroupsForUser(ctx, userID, onlyActiveNight)
}

func validateBattery(b *int) error {
	if b != nil && (*b < 0 || *b > 100) {
		return fmt.Errorf("%w: battery must be between 0 and 100", ErrValidation)
	}
	return nil
}
