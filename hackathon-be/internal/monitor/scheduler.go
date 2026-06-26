// Package monitor runs the background scheduler that periodically triggers
// Night.check() for active nights that are due.
package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/t-kaijifu/hackathon-be/internal/service"
)

// Scheduler periodically runs due checks across all active nights.
type Scheduler struct {
	svc  *service.Service
	tick time.Duration
	log  *slog.Logger
}

// New constructs a Scheduler. A non-positive tick defaults to 30s.
func New(svc *service.Service, tick time.Duration, log *slog.Logger) *Scheduler {
	if tick <= 0 {
		tick = 30 * time.Second
	}
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{svc: svc, tick: tick, log: log}
}

// Run blocks until ctx is cancelled, running due checks on every tick.
func (s *Scheduler) Run(ctx context.Context) {
	s.log.Info("monitor scheduler started", "tick", s.tick.String())
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("monitor scheduler stopped")
			return
		case <-ticker.C:
			n, err := s.svc.CheckAllDue(ctx)
			if err != nil {
				s.log.Error("monitor tick failed", "error", err)
				continue
			}
			if n > 0 {
				s.log.Debug("monitor tick complete", "nightsChecked", n)
			}
		}
	}
}
