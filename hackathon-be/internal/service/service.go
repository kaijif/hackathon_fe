// Package service holds the application business logic. It depends only on the
// store.Store and notify.Notifier interfaces so it can be unit-tested in
// isolation from Postgres.
package service

import (
	"errors"
	"log/slog"
	"time"

	"github.com/t-kaijifu/hackathon-be/internal/notify"
	"github.com/t-kaijifu/hackathon-be/internal/store"
)

// Sentinel errors returned by the service layer. ErrNotFound is shared with the
// store so callers can use a single check.
var (
	ErrNotFound   = store.ErrNotFound
	ErrValidation = errors.New("validation error")
	ErrConflict   = errors.New("conflict")
)

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Service implements the application's use cases.
type Service struct {
	store             store.Store
	notifier          notify.Notifier
	clock             Clock
	log               *slog.Logger
	defaultLowBattery int
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the clock (used in tests).
func WithClock(c Clock) Option { return func(s *Service) { s.clock = c } }

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option { return func(s *Service) { s.log = l } }

// WithDefaultLowBattery sets the default low-battery threshold for new nights.
func WithDefaultLowBattery(n int) Option {
	return func(s *Service) { s.defaultLowBattery = n }
}

// New constructs a Service.
func New(st store.Store, nf notify.Notifier, opts ...Option) *Service {
	s := &Service{
		store:             st,
		notifier:          nf,
		clock:             realClock{},
		log:               slog.Default(),
		defaultLowBattery: 20,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
