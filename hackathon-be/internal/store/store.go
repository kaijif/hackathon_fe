// Package store defines the persistence interface and its Postgres implementation.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// Store is the persistence boundary for the application. It is an interface so
// the service layer can be unit-tested against an in-memory or mock store.
type Store interface {
	// Users
	CreateUser(ctx context.Context, u *models.User) error
	GetUser(ctx context.Context, id uuid.UUID) (*models.User, error)
	ListUsers(ctx context.Context) ([]models.User, error)
	UpdateUserLocation(ctx context.Context, id uuid.UUID, lat, lng float64, battery *int, at time.Time) error
	UpdateUserBattery(ctx context.Context, id uuid.UUID, battery int, at time.Time) error

	// Agents
	CreateAgent(ctx context.Context, a *models.Agent) error
	GetAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	ListAgents(ctx context.Context) ([]models.Agent, error)

	// Groups
	CreateGroup(ctx context.Context, g *models.Group) error
	GetGroup(ctx context.Context, id uuid.UUID) (*models.Group, error)
	ListGroups(ctx context.Context) ([]models.Group, error)
	DeleteGroup(ctx context.Context, id uuid.UUID) error
	SetGroupNight(ctx context.Context, groupID uuid.UUID, nightID *uuid.UUID, active bool) error
	ListGroupsForUser(ctx context.Context, userID uuid.UUID, onlyActiveNight bool) ([]models.Group, error)

	// Membership
	AddMember(ctx context.Context, groupID, userID uuid.UUID, isAdmin bool) error
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	SetAdmin(ctx context.Context, groupID, userID uuid.UUID, isAdmin bool) error
	IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error)
	ListMembers(ctx context.Context, groupID uuid.UUID) ([]models.Member, error)
	ListAdmins(ctx context.Context, groupID uuid.UUID) ([]models.Member, error)

	// Nights
	CreateNight(ctx context.Context, n *models.Night) error
	GetNight(ctx context.Context, id uuid.UUID) (*models.Night, error)
	ListNightsByGroup(ctx context.Context, groupID uuid.UUID) ([]models.Night, error)
	ListActiveNights(ctx context.Context) ([]models.Night, error)
	UpdateNightLifecycle(ctx context.Context, id uuid.UUID, status models.NightStatus, startedAt, endedAt *time.Time) error
	UpdateNightRange(ctx context.Context, id uuid.UUID, maxRangeM int) error
	UpdateNightCenter(ctx context.Context, id uuid.UUID, lat, lng float64) error
	SetNightLastChecked(ctx context.Context, id uuid.UUID, at time.Time) error
	DeleteNight(ctx context.Context, id uuid.UUID) error

	// Night locations & statuses
	UpsertNightLocation(ctx context.Context, loc *models.NightLocation) error
	GetNightLocation(ctx context.Context, nightID, userID uuid.UUID) (*models.NightLocation, error)
	ListNightLocations(ctx context.Context, nightID uuid.UUID) ([]models.NightLocation, error)
	PropagateUserLocationToActiveNights(ctx context.Context, userID uuid.UUID, lat, lng float64, battery *int, at time.Time) error
	PropagateUserBatteryToActiveNights(ctx context.Context, userID uuid.UUID, battery int, at time.Time) error
	UpsertParticipantStatus(ctx context.Context, st *models.ParticipantState) error
	GetParticipantStatus(ctx context.Context, nightID, userID uuid.UUID) (*models.ParticipantState, error)
	SetParticipantCheckin(ctx context.Context, nightID, userID uuid.UUID, at time.Time) error
	ListParticipantStatuses(ctx context.Context, nightID uuid.UUID) ([]models.ParticipantState, error)

	// Messages
	CreateMessage(ctx context.Context, m *models.Message) error
	ListMessagesByNight(ctx context.Context, nightID uuid.UUID) ([]models.Message, error)

	// Device tokens (push notifications)
	AddDeviceToken(ctx context.Context, dt *models.DeviceToken) error
	RemoveDeviceToken(ctx context.Context, userID uuid.UUID, token string) error
	ListDeviceTokensForUser(ctx context.Context, userID uuid.UUID) ([]models.DeviceToken, error)
}

// PgStore is the Postgres-backed implementation of Store.
type PgStore struct {
	pool *pgxpool.Pool
}

// New returns a PgStore backed by the given pool.
func New(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}
