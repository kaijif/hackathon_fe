// Package models defines the core domain types for the NightWatch backend.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Coords is a geographic coordinate pair.
type Coords struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// NightStatus is the lifecycle state of a Night.
type NightStatus string

const (
	NightPending NightStatus = "pending"
	NightActive  NightStatus = "active"
	NightEnded   NightStatus = "ended"
)

// ParticipantStatus is the computed safety status of a participant during a Night.
type ParticipantStatus string

const (
	StatusOK         ParticipantStatus = "ok"
	StatusOutOfRange ParticipantStatus = "out_of_range"
	// StatusOutOfRangeSafe is an out-of-range participant who has explicitly
	// confirmed they are safe; it holds for a grace window before reverting.
	StatusOutOfRangeSafe ParticipantStatus = "out_of_range_safe"
	StatusLowBattery     ParticipantStatus = "low_battery"
	StatusMissing        ParticipantStatus = "missing"
	StatusUnknown        ParticipantStatus = "unknown"
)

// User represents a client app / person being monitored.
type User struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	TrustedContact    string     `json:"trustedContact,omitempty"`
	Lat               *float64   `json:"lat,omitempty"`
	Lng               *float64   `json:"lng,omitempty"`
	BatteryLevel      *int       `json:"batteryLevel,omitempty"`
	LocationUpdatedAt *time.Time `json:"locationUpdatedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// Agent is a monitoring persona attached to a Night; used as the message sender.
type Agent struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Group is a collection of users that can run Nights together.
type Group struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Active      bool       `json:"active"`
	CurrNightID *uuid.UUID `json:"currNightId,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Member is a user's membership row within a group.
type Member struct {
	UserID   uuid.UUID `json:"userId"`
	Name     string    `json:"name"`
	IsAdmin  bool      `json:"isAdmin"`
	JoinedAt time.Time `json:"joinedAt"`
}

// Night is a monitored session for a group.
type Night struct {
	ID                  uuid.UUID   `json:"id"`
	GroupID             uuid.UUID   `json:"groupId"`
	AgentID             *uuid.UUID  `json:"agentId,omitempty"`
	CenterLat           *float64    `json:"centerLat,omitempty"`
	CenterLng           *float64    `json:"centerLng,omitempty"`
	TimeLimitMin        int         `json:"timeLimitMin"`
	CheckInLimitMin     int         `json:"checkInLimitMin"`
	CheckInEveryMin     int         `json:"checkInEveryMin"`
	MaxRangeM           int         `json:"maxRangeM"`
	LowBatteryThreshold int         `json:"lowBatteryThreshold"`
	Status              NightStatus `json:"status"`
	StartedAt           *time.Time  `json:"startedAt,omitempty"`
	EndedAt             *time.Time  `json:"endedAt,omitempty"`
	LastCheckedAt       *time.Time  `json:"lastCheckedAt,omitempty"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
}

// HasCenter reports whether the night has a defined center point.
func (n Night) HasCenter() bool {
	return n.CenterLat != nil && n.CenterLng != nil
}

// NightLocation is the latest reported location for a participant in a night.
type NightLocation struct {
	NightID      uuid.UUID `json:"nightId"`
	UserID       uuid.UUID `json:"userId"`
	Lat          float64   `json:"lat"`
	Lng          float64   `json:"lng"`
	BatteryLevel *int      `json:"batteryLevel,omitempty"`
	ReportedAt   time.Time `json:"reportedAt"`
}

// ParticipantState is the latest computed status for a participant in a night.
type ParticipantState struct {
	NightID       uuid.UUID         `json:"nightId"`
	UserID        uuid.UUID         `json:"userId"`
	Status        ParticipantStatus `json:"status"`
	Detail        string            `json:"detail,omitempty"`
	DistanceM     *float64          `json:"distanceM,omitempty"`
	LastCheckinAt *time.Time        `json:"lastCheckinAt,omitempty"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// Message is a persisted outbound message / notification.
type Message struct {
	ID               uuid.UUID  `json:"id"`
	NightID          *uuid.UUID `json:"nightId,omitempty"`
	GroupID          *uuid.UUID `json:"groupId,omitempty"`
	RecipientUserID  *uuid.UUID `json:"recipientUserId,omitempty"`
	RecipientContact string     `json:"recipientContact,omitempty"`
	Kind             string     `json:"kind"`
	Body             string     `json:"body"`
	Sender           string     `json:"sender,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// DeviceToken is a push-notification token registered by a user's device.
type DeviceToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	Platform  string    `json:"platform"` // ios | android
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
