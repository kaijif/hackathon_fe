// Package notify defines the Notifier abstraction used to deliver messages and
// alerts. The default implementation persists each message to the database and
// logs it; a real SMS/push provider can be dropped in behind the same interface.
package notify

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// Kind values for outbound messages.
const (
	KindMessage = "message" // a directed message to specific users
	KindNotify  = "notify"  // a broadcast notification
	KindAlert   = "alert"   // an automated safety alert from check()
)

// APNs notification categories. These line up with the iOS notification
// categories/actions so the app can render the right action buttons and
// deep-links for a push.
const (
	CategoryCheckin = "CHECKIN" // "are you OK?" prompt with an "I'm OK" action
	CategoryAlert   = "ALERT"   // safety alert that deep-links to the Guardian screen
)

// Outbound is a message to be delivered.
type Outbound struct {
	NightID          *uuid.UUID
	GroupID          *uuid.UUID
	RecipientUserID  *uuid.UUID
	RecipientContact string
	Kind             string
	Body             string
	Sender           string

	// Category is the APNs notification category (e.g. CHECKIN, ALERT). It is
	// ignored by the DB notifier and only affects push payloads.
	Category string
	// Data carries extra top-level custom fields merged into the push payload
	// (e.g. type, nightId, userId, status) so the iOS app can route the
	// notification. It is ignored by the DB notifier.
	Data map[string]any
}

// Notifier delivers outbound messages.
type Notifier interface {
	Send(ctx context.Context, msg Outbound) (*models.Message, error)
}

// MessageSink persists messages. *store.PgStore satisfies this interface.
type MessageSink interface {
	CreateMessage(ctx context.Context, m *models.Message) error
}

// DBNotifier persists messages to a MessageSink and logs them.
type DBNotifier struct {
	sink MessageSink
	log  *slog.Logger
}

// NewDBNotifier constructs a DBNotifier.
func NewDBNotifier(sink MessageSink, log *slog.Logger) *DBNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &DBNotifier{sink: sink, log: log}
}

// Send persists and logs the message.
func (n *DBNotifier) Send(ctx context.Context, msg Outbound) (*models.Message, error) {
	m := &models.Message{
		ID:               uuid.New(),
		NightID:          msg.NightID,
		GroupID:          msg.GroupID,
		RecipientUserID:  msg.RecipientUserID,
		RecipientContact: msg.RecipientContact,
		Kind:             msg.Kind,
		Body:             msg.Body,
		Sender:           msg.Sender,
	}
	if err := n.sink.CreateMessage(ctx, m); err != nil {
		return nil, err
	}

	recipient := msg.RecipientContact
	if msg.RecipientUserID != nil {
		recipient = msg.RecipientUserID.String()
	}
	n.log.Info("message dispatched",
		"kind", m.Kind,
		"recipient", recipient,
		"sender", m.Sender,
		"body", m.Body,
	)
	return m, nil
}
