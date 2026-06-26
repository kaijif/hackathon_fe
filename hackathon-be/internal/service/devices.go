package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// RegisterDevice registers a push-notification device token for a user.
func (s *Service) RegisterDevice(ctx context.Context, userID uuid.UUID, platform, token string) (*models.DeviceToken, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	switch platform {
	case "ios", "android":
	default:
		return nil, fmt.Errorf("%w: platform must be 'ios' or 'android'", ErrValidation)
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("%w: token is required", ErrValidation)
	}
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	dt := &models.DeviceToken{
		ID:       uuid.New(),
		UserID:   userID,
		Platform: platform,
		Token:    strings.TrimSpace(token),
	}
	if err := s.store.AddDeviceToken(ctx, dt); err != nil {
		return nil, err
	}
	return dt, nil
}

// UnregisterDevice removes a device token for a user.
func (s *Service) UnregisterDevice(ctx context.Context, userID uuid.UUID, token string) error {
	return s.store.RemoveDeviceToken(ctx, userID, token)
}

// ListDevices returns a user's registered device tokens.
func (s *Service) ListDevices(ctx context.Context, userID uuid.UUID) ([]models.DeviceToken, error) {
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.store.ListDeviceTokensForUser(ctx, userID)
}
