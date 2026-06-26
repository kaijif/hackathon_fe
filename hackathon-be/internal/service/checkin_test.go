package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
	"github.com/t-kaijifu/hackathon-be/internal/notify"
)

func ptrFloat(f float64) *float64 { return &f }

// activeNightFixture creates a started night centered at (0,0) whose creator is
// the first (admin) member, plus any extra joined members.
func activeNightFixture(t *testing.T, svc *Service, ctx context.Context, extra ...*models.User) (*models.Group, *models.Night) {
	t.Helper()
	g, err := svc.FormGroup(ctx, "Crew", extra[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range extra[1:] {
		if err := svc.JoinGroup(ctx, g.ID, u.ID); err != nil {
			t.Fatal(err)
		}
	}
	night, err := svc.CreateNight(ctx, g.ID, CreateNightInput{
		Center:              &models.Coords{Lat: 0, Lng: 0},
		MaxRangeM:           ptrInt(1000),
		CheckInLimitMin:     ptrInt(30),
		LowBatteryThreshold: ptrInt(20),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartNight(ctx, night.ID); err != nil {
		t.Fatal(err)
	}
	return g, night
}

func TestCheckinOKPersistsLocationAndStatus(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, st, _ := newTestService(clock)
	ctx := context.Background()

	u, err := svc.CreateUser(ctx, CreateUserInput{Name: "Alex"})
	if err != nil {
		t.Fatal(err)
	}
	_, night := activeNightFixture(t, svc, ctx, u)

	state, err := svc.Checkin(ctx, night.ID, u.ID, CheckinInput{
		OK: true, Lat: ptrFloat(0.0001), Lng: ptrFloat(0), Battery: ptrInt(80),
	})
	if err != nil {
		t.Fatalf("checkin: %v", err)
	}
	if state.Status != models.StatusOK {
		t.Fatalf("status = %q, want ok", state.Status)
	}
	if state.LastCheckinAt == nil {
		t.Fatal("expected lastCheckinAt to be set")
	}

	// Night location was persisted.
	loc, err := st.GetNightLocation(ctx, night.ID, u.ID)
	if err != nil {
		t.Fatalf("night location not persisted: %v", err)
	}
	if loc.BatteryLevel == nil || *loc.BatteryLevel != 80 {
		t.Fatalf("battery not persisted into night location: %+v", loc)
	}

	// Global user location/battery were updated.
	got, _ := st.GetUser(ctx, u.ID)
	if got.Lat == nil || *got.Lat != 0.0001 || got.BatteryLevel == nil || *got.BatteryLevel != 80 {
		t.Fatalf("user location/battery not updated: %+v", got)
	}
}

func TestCheckinDistressAlertsOtherMembers(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, _, nf := newTestService(clock)
	ctx := context.Background()

	affected, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Affected", TrustedContact: "+15550001111"})
	o1, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Guardian One"})
	o2, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Guardian Two"})
	_, night := activeNightFixture(t, svc, ctx, affected, o1, o2)

	state, err := svc.Checkin(ctx, night.ID, affected.ID, CheckinInput{OK: false})
	if err != nil {
		t.Fatalf("checkin: %v", err)
	}
	if state.Status != models.StatusMissing {
		t.Fatalf("distress status = %q, want missing", state.Status)
	}

	alertedUsers := map[uuid.UUID]bool{}
	trustedAlerted := false
	for _, m := range nf.sent {
		if m.RecipientUserID != nil && m.Kind == notify.KindAlert {
			alertedUsers[*m.RecipientUserID] = true
		}
		if m.RecipientContact == "+15550001111" {
			trustedAlerted = true
		}
	}
	if !alertedUsers[o1.ID] || !alertedUsers[o2.ID] {
		t.Fatalf("expected both guardians alerted, got %v", alertedUsers)
	}
	if alertedUsers[affected.ID] {
		t.Fatal("the affected user must not receive the group distress alert")
	}
	if !trustedAlerted {
		t.Fatal("expected the affected user's trusted contact to be alerted")
	}
}

func TestCheckinResetsMissingTimer(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, _, _ := newTestService(clock)
	ctx := context.Background()

	u, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Solo"})
	_, night := activeNightFixture(t, svc, ctx, u)

	// Report an in-range, healthy location, then let it go stale.
	if _, err := svc.ReportNightLocation(ctx, night.ID, u.ID, 0.0001, 0, ptrInt(90)); err != nil {
		t.Fatal(err)
	}
	clock.Advance(31 * time.Minute) // exceeds checkInLimitMin (30)

	res, err := svc.Check(ctx, night.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Statuses[0].Status != models.StatusMissing {
		t.Fatalf("expected missing before check-in, got %q", res.Statuses[0].Status)
	}

	// A bare check-in (no coordinates) must revive freshness.
	state, err := svc.Checkin(ctx, night.ID, u.ID, CheckinInput{OK: true})
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != models.StatusOK {
		t.Fatalf("expected ok right after check-in, got %q", state.Status)
	}

	res2, err := svc.Check(ctx, night.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Statuses[0].Status != models.StatusOK {
		t.Fatalf("expected ok after a fresh check-in, got %q", res2.Statuses[0].Status)
	}
}

// statusOf returns the status recorded for userID, or "" if absent.
func statusOf(statuses []models.ParticipantState, userID uuid.UUID) models.ParticipantStatus {
	for _, st := range statuses {
		if st.UserID == userID {
			return st.Status
		}
	}
	return ""
}

func TestCheckinSafeWhileOutOfRange(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, _, nf := newTestService(clock)
	ctx := context.Background()

	affected, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Affected", TrustedContact: "+15550002222"})
	guardian, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Guardian"})
	_, night := activeNightFixture(t, svc, ctx, affected, guardian)

	// ~5.5 km north of the (0,0) center => out of range, but confirms safe.
	state, err := svc.Checkin(ctx, night.ID, affected.ID, CheckinInput{
		OK: true, Lat: ptrFloat(0.05), Lng: ptrFloat(0), Battery: ptrInt(90),
	})
	if err != nil {
		t.Fatalf("checkin: %v", err)
	}
	if state.Status != models.StatusOutOfRangeSafe {
		t.Fatalf("status = %q, want out_of_range_safe", state.Status)
	}

	guardianNotified, trustedNotified := false, false
	for _, m := range nf.sent {
		if m.RecipientUserID != nil && *m.RecipientUserID == guardian.ID && m.Kind == notify.KindNotify {
			guardianNotified = true
		}
		if m.RecipientContact == "+15550002222" {
			trustedNotified = true
		}
	}
	if !guardianNotified {
		t.Fatal("expected the guardian to be notified of the safe update")
	}
	if !trustedNotified {
		t.Fatal("expected the trusted contact to be notified of the safe update")
	}
}

func TestOutOfRangeSafeExpiresAfter30Min(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, _, _ := newTestService(clock)
	ctx := context.Background()

	affected, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Affected"})
	g, err := svc.FormGroup(ctx, "Crew", affected.ID)
	if err != nil {
		t.Fatal(err)
	}
	// checkInLimitMin must exceed the 30-min safe window so the stale "missing"
	// check does not preempt the expiry back to out_of_range.
	night, err := svc.CreateNight(ctx, g.ID, CreateNightInput{
		Center:          &models.Coords{Lat: 0, Lng: 0},
		MaxRangeM:       ptrInt(1000),
		CheckInLimitMin: ptrInt(120),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartNight(ctx, night.ID); err != nil {
		t.Fatal(err)
	}

	state, err := svc.Checkin(ctx, night.ID, affected.ID, CheckinInput{
		OK: true, Lat: ptrFloat(0.05), Lng: ptrFloat(0), Battery: ptrInt(90),
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != models.StatusOutOfRangeSafe {
		t.Fatalf("status = %q, want out_of_range_safe", state.Status)
	}

	// Within the window the safe status holds.
	clock.Advance(20 * time.Minute)
	res, err := svc.Check(ctx, night.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(res.Statuses, affected.ID); got != models.StatusOutOfRangeSafe {
		t.Fatalf("at 20m: status = %q, want out_of_range_safe", got)
	}

	// Past the 30-min window, still out of range => reverts to out_of_range.
	clock.Advance(15 * time.Minute)
	res, err = svc.Check(ctx, night.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(res.Statuses, affected.ID); got != models.StatusOutOfRange {
		t.Fatalf("at 35m: status = %q, want out_of_range", got)
	}
}

func TestCheckinValidationAndState(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, _, _ := newTestService(clock)
	ctx := context.Background()

	member, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Member"})
	outsider, _ := svc.CreateUser(ctx, CreateUserInput{Name: "Outsider"})
	_, night := activeNightFixture(t, svc, ctx, member)

	cases := []struct {
		name    string
		nightID uuid.UUID
		userID  uuid.UUID
		in      CheckinInput
		wantErr error
	}{
		{"only lat", night.ID, member.ID, CheckinInput{OK: true, Lat: ptrFloat(1)}, ErrValidation},
		{"bad lat", night.ID, member.ID, CheckinInput{OK: true, Lat: ptrFloat(200), Lng: ptrFloat(0)}, ErrValidation},
		{"bad battery", night.ID, member.ID, CheckinInput{OK: true, Battery: ptrInt(150)}, ErrValidation},
		{"unknown night", uuid.New(), member.ID, CheckinInput{OK: true}, ErrNotFound},
		{"unknown user", night.ID, uuid.New(), CheckinInput{OK: true}, ErrNotFound},
		{"non-member", night.ID, outsider.ID, CheckinInput{OK: true}, ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Checkin(ctx, tc.nightID, tc.userID, tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}

	// A pending (not started) night must reject check-ins with a conflict.
	pendingNight, _ := svc.CreateNight(ctx, night.GroupID, CreateNightInput{})
	if _, err := svc.Checkin(ctx, pendingNight.ID, member.ID, CheckinInput{OK: true}); !errors.Is(err, ErrConflict) {
		t.Fatalf("pending-night check-in err = %v, want conflict", err)
	}
}

func TestScheduledCheckSendsCheckinPrompts(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)}
	svc, _, nf := newTestService(clock)
	ctx := context.Background()

	a, _ := svc.CreateUser(ctx, CreateUserInput{Name: "A"})
	b, _ := svc.CreateUser(ctx, CreateUserInput{Name: "B"})
	_, _ = activeNightFixture(t, svc, ctx, a, b)

	n, err := svc.CheckAllDue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 night checked, got %d", n)
	}

	prompts := 0
	for _, m := range nf.sent {
		if m.Category == notify.CategoryCheckin {
			prompts++
			if m.Kind != notify.KindNotify {
				t.Fatalf("check-in prompt kind = %q, want notify", m.Kind)
			}
			if m.Data["type"] != "checkin" {
				t.Fatalf("check-in prompt missing type=checkin: %+v", m.Data)
			}
		}
	}
	if prompts != 2 {
		t.Fatalf("expected an 'are you OK?' prompt per member (2), got %d", prompts)
	}
}
