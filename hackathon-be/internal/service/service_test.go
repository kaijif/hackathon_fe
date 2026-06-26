package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

func newTestService(clock Clock) (*Service, *memStore, *captureNotifier) {
	st := newMemStore()
	nf := &captureNotifier{}
	svc := New(st, nf,
		WithClock(clock),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithDefaultLowBattery(20),
	)
	return svc, st, nf
}

func ptrInt(n int) *int { return &n }

func TestEvaluateParticipant(t *testing.T) {
	now := time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)
	lat, lng := 0.0, 0.0
	night := models.Night{
		CenterLat:           &lat,
		CenterLng:           &lng,
		MaxRangeM:           1000,
		CheckInLimitMin:     60,
		LowBatteryThreshold: 20,
	}
	fresh := now.Add(-1 * time.Minute)
	expired := now.Add(-31 * time.Minute)

	tests := []struct {
		name        string
		loc         *models.NightLocation
		lastCheckin *time.Time
		prev        models.ParticipantStatus
		want        models.ParticipantStatus
	}{
		{"no location", nil, nil, "", models.StatusMissing},
		{
			"stale check-in",
			&models.NightLocation{Lat: 0, Lng: 0, ReportedAt: now.Add(-2 * time.Hour), BatteryLevel: ptrInt(90)},
			nil,
			"",
			models.StatusMissing,
		},
		{
			"out of range",
			&models.NightLocation{Lat: 0.05, Lng: 0, ReportedAt: now, BatteryLevel: ptrInt(90)},
			nil,
			"",
			models.StatusOutOfRange,
		},
		{
			"low battery",
			&models.NightLocation{Lat: 0.0001, Lng: 0, ReportedAt: now, BatteryLevel: ptrInt(10)},
			nil,
			"",
			models.StatusLowBattery,
		},
		{
			"ok",
			&models.NightLocation{Lat: 0.0001, Lng: 0, ReportedAt: now, BatteryLevel: ptrInt(90)},
			nil,
			"",
			models.StatusOK,
		},
		{
			"fresh check-in revives a stale-location participant",
			&models.NightLocation{Lat: 0.0001, Lng: 0, ReportedAt: now.Add(-2 * time.Hour), BatteryLevel: ptrInt(90)},
			&fresh,
			"",
			models.StatusOK,
		},
		{"check-in only, no location", nil, &fresh, "", models.StatusOK},
		{
			"out of range but confirmed safe within window",
			&models.NightLocation{Lat: 0.05, Lng: 0, ReportedAt: now, BatteryLevel: ptrInt(90)},
			&fresh,
			models.StatusOutOfRangeSafe,
			models.StatusOutOfRangeSafe,
		},
		{
			"safe window expired reverts to out of range",
			&models.NightLocation{Lat: 0.05, Lng: 0, ReportedAt: now, BatteryLevel: ptrInt(90)},
			&expired,
			models.StatusOutOfRangeSafe,
			models.StatusOutOfRange,
		},
		{
			"returning within range clears safe to ok",
			&models.NightLocation{Lat: 0.0001, Lng: 0, ReportedAt: now, BatteryLevel: ptrInt(90)},
			&fresh,
			models.StatusOutOfRangeSafe,
			models.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateParticipant(night, tt.loc, tt.lastCheckin, tt.prev, now)
			if got.Status != tt.want {
				t.Fatalf("status = %q, want %q (detail=%q)", got.Status, tt.want, got.Detail)
			}
		})
	}
}

func TestCheckAlertsAndStatuses(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, st, nf := newTestService(clock)
	ctx := context.Background()

	admin, err := svc.CreateUser(ctx, CreateUserInput{Name: "Admin", TrustedContact: "+15551234567"})
	if err != nil {
		t.Fatal(err)
	}
	member, err := svc.CreateUser(ctx, CreateUserInput{Name: "Member"})
	if err != nil {
		t.Fatal(err)
	}

	g, err := svc.FormGroup(ctx, "Crew", admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.JoinGroup(ctx, g.ID, member.ID); err != nil {
		t.Fatal(err)
	}

	night, err := svc.CreateNight(ctx, g.ID, CreateNightInput{
		Center:       &models.Coords{Lat: 0, Lng: 0},
		MaxRangeM:    ptrInt(1000),
		TimeLimitMin: ptrInt(480),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartNight(ctx, night.ID); err != nil {
		t.Fatal(err)
	}

	// Admin is far out of range; member is comfortably inside.
	if _, err := svc.ReportNightLocation(ctx, night.ID, admin.ID, 0.05, 0, ptrInt(90)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReportNightLocation(ctx, night.ID, member.ID, 0.0001, 0, ptrInt(90)); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Check(ctx, night.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Ended {
		t.Fatal("night should not have ended")
	}
	if res.Alerts != 1 {
		t.Fatalf("expected 1 alert, got %d", res.Alerts)
	}

	statuses, _ := st.ListParticipantStatuses(ctx, night.ID)
	byUser := map[uuid.UUID]models.ParticipantStatus{}
	for _, s := range statuses {
		byUser[s.UserID] = s.Status
	}
	if byUser[admin.ID] != models.StatusOutOfRange {
		t.Fatalf("admin status = %q, want out_of_range", byUser[admin.ID])
	}
	if byUser[member.ID] != models.StatusOK {
		t.Fatalf("member status = %q, want ok", byUser[member.ID])
	}

	// The out-of-range admin's trusted contact must have been alerted.
	trustedAlerted := false
	participantAlerted := false
	for _, m := range nf.sent {
		if m.RecipientContact == "+15551234567" {
			trustedAlerted = true
		}
		if m.RecipientUserID != nil && *m.RecipientUserID == admin.ID && m.Kind == "alert" {
			participantAlerted = true
		}
	}
	if !trustedAlerted {
		t.Fatal("expected trusted contact to be alerted")
	}
	if !participantAlerted {
		t.Fatal("expected the out-of-range participant to be alerted")
	}
}

func TestCheckEndsAtTimeLimit(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, st, _ := newTestService(clock)
	ctx := context.Background()

	u, err := svc.CreateUser(ctx, CreateUserInput{Name: "Solo"})
	if err != nil {
		t.Fatal(err)
	}
	g, err := svc.FormGroup(ctx, "Solo Crew", u.ID)
	if err != nil {
		t.Fatal(err)
	}
	night, err := svc.CreateNight(ctx, g.ID, CreateNightInput{TimeLimitMin: ptrInt(60)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartNight(ctx, night.ID); err != nil {
		t.Fatal(err)
	}

	clock.Advance(61 * time.Minute)

	res, err := svc.Check(ctx, night.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Ended {
		t.Fatal("expected night to end at time limit")
	}

	got, _ := st.GetNight(ctx, night.ID)
	if got.Status != models.NightEnded {
		t.Fatalf("night status = %q, want ended", got.Status)
	}
	grp, _ := st.GetGroup(ctx, g.ID)
	if grp.Active {
		t.Fatal("group should be inactive after night ends")
	}
}

func TestCheckRequiresActiveNight(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	svc, _, _ := newTestService(clock)
	ctx := context.Background()

	u, _ := svc.CreateUser(ctx, CreateUserInput{Name: "U"})
	g, _ := svc.FormGroup(ctx, "G", u.ID)
	night, _ := svc.CreateNight(ctx, g.ID, CreateNightInput{})

	if _, err := svc.Check(ctx, night.ID); err == nil {
		t.Fatal("expected error checking a pending night")
	}
}

func TestSetCenter(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	svc, _, _ := newTestService(clock)
	ctx := context.Background()

	u, _ := svc.CreateUser(ctx, CreateUserInput{Name: "U"})
	g, _ := svc.FormGroup(ctx, "G", u.ID)
	night, _ := svc.CreateNight(ctx, g.ID, CreateNightInput{})

	updated, err := svc.SetCenter(ctx, night.ID, 37.7749, -122.4194)
	if err != nil {
		t.Fatalf("set center: %v", err)
	}
	if updated.CenterLat == nil || *updated.CenterLat != 37.7749 ||
		updated.CenterLng == nil || *updated.CenterLng != -122.4194 {
		t.Fatalf("center not updated: %+v", updated)
	}

	// Out-of-bounds coordinates are rejected.
	if _, err := svc.SetCenter(ctx, night.ID, 200, 0); !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want validation error", err)
	}
	// An unknown night yields not found.
	if _, err := svc.SetCenter(ctx, uuid.New(), 0, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}
