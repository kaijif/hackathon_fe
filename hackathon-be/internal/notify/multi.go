package notify

import (
	"context"
	"log/slog"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

// MultiNotifier fans a message out to several notifiers. The primary notifier
// is authoritative (its returned message and error propagate); the extras are
// best-effort side channels (e.g. push) whose errors are logged, not fatal.
type MultiNotifier struct {
	primary Notifier
	extra   []Notifier
	log     *slog.Logger
}

// NewMultiNotifier composes a primary notifier with zero or more extras.
func NewMultiNotifier(primary Notifier, log *slog.Logger, extra ...Notifier) *MultiNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &MultiNotifier{primary: primary, extra: extra, log: log}
}

// Send delivers via the primary notifier, then best-effort via the extras.
func (m *MultiNotifier) Send(ctx context.Context, msg Outbound) (*models.Message, error) {
	res, err := m.primary.Send(ctx, msg)
	if err != nil {
		return nil, err
	}
	for _, n := range m.extra {
		if _, err := n.Send(ctx, msg); err != nil {
			m.log.Error("secondary notifier failed", "kind", msg.Kind, "error", err)
		}
	}
	return res, nil
}
