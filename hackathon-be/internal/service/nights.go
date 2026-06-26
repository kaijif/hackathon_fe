package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
	"github.com/t-kaijifu/hackathon-be/internal/notify"
)

// CreateNightInput is the payload for creating a night. Nil fields fall back to
// sensible defaults.
type CreateNightInput struct {
	AgentID             *uuid.UUID
	Center              *models.Coords
	TimeLimitMin        *int
	CheckInLimitMin     *int
	CheckInEveryMin     *int
	MaxRangeM           *int
	LowBatteryThreshold *int
}

// NightView is a night plus its current locations and participant statuses.
type NightView struct {
	models.Night
	CurrentLocations    []models.NightLocation    `json:"currentLocations"`
	ParticipantStatuses []models.ParticipantState `json:"participantStatuses"`
}

// CheckResult summarizes a single check() run.
type CheckResult struct {
	NightID  uuid.UUID                 `json:"nightId"`
	Ended    bool                      `json:"ended"`
	Alerts   int                       `json:"alerts"`
	Statuses []models.ParticipantState `json:"statuses"`
}

// CreateNight creates a pending night for a group.
func (s *Service) CreateNight(ctx context.Context, groupID uuid.UUID, in CreateNightInput) (*models.Night, error) {
	if _, err := s.store.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	if in.AgentID != nil {
		if _, err := s.store.GetAgent(ctx, *in.AgentID); err != nil {
			return nil, err
		}
	}
	n := &models.Night{
		ID:                  uuid.New(),
		GroupID:             groupID,
		AgentID:             in.AgentID,
		Status:              models.NightPending,
		TimeLimitMin:        derefInt(in.TimeLimitMin, 480),
		CheckInLimitMin:     derefInt(in.CheckInLimitMin, 60),
		CheckInEveryMin:     derefInt(in.CheckInEveryMin, 15),
		MaxRangeM:           derefInt(in.MaxRangeM, 1000),
		LowBatteryThreshold: derefInt(in.LowBatteryThreshold, s.defaultLowBattery),
	}
	if in.Center != nil {
		n.CenterLat = &in.Center.Lat
		n.CenterLng = &in.Center.Lng
	}
	if err := validateNight(n); err != nil {
		return nil, err
	}
	if err := s.store.CreateNight(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

// GetNight returns a night by id.
func (s *Service) GetNight(ctx context.Context, id uuid.UUID) (*models.Night, error) {
	return s.store.GetNight(ctx, id)
}

// GetNightView returns a night with its locations and participant statuses.
func (s *Service) GetNightView(ctx context.Context, id uuid.UUID) (*NightView, error) {
	n, err := s.store.GetNight(ctx, id)
	if err != nil {
		return nil, err
	}
	locs, err := s.store.ListNightLocations(ctx, id)
	if err != nil {
		return nil, err
	}
	statuses, err := s.store.ListParticipantStatuses(ctx, id)
	if err != nil {
		return nil, err
	}
	return &NightView{Night: *n, CurrentLocations: locs, ParticipantStatuses: statuses}, nil
}

// ListNightsByGroup returns all nights for a group.
func (s *Service) ListNightsByGroup(ctx context.Context, groupID uuid.UUID) ([]models.Night, error) {
	if _, err := s.store.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	return s.store.ListNightsByGroup(ctx, groupID)
}

// StartNight transitions a pending night to active.
func (s *Service) StartNight(ctx context.Context, id uuid.UUID) (*models.Night, error) {
	n, err := s.store.GetNight(ctx, id)
	if err != nil {
		return nil, err
	}
	switch n.Status {
	case models.NightActive:
		return n, nil
	case models.NightEnded:
		return nil, fmt.Errorf("%w: night has already ended", ErrConflict)
	}
	now := s.clock.Now()
	if err := s.store.UpdateNightLifecycle(ctx, id, models.NightActive, &now, nil); err != nil {
		return nil, err
	}
	if err := s.store.SetGroupNight(ctx, n.GroupID, &id, true); err != nil {
		return nil, err
	}
	return s.store.GetNight(ctx, id)
}

// EndNight transitions a night to ended.
func (s *Service) EndNight(ctx context.Context, id uuid.UUID) (*models.Night, error) {
	n, err := s.store.GetNight(ctx, id)
	if err != nil {
		return nil, err
	}
	if n.Status == models.NightEnded {
		return n, nil
	}
	return s.endNight(ctx, n)
}

func (s *Service) endNight(ctx context.Context, n *models.Night) (*models.Night, error) {
	now := s.clock.Now()
	if err := s.store.UpdateNightLifecycle(ctx, n.ID, models.NightEnded, nil, &now); err != nil {
		return nil, err
	}
	if err := s.clearGroupNight(ctx, n); err != nil {
		return nil, err
	}
	return s.store.GetNight(ctx, n.ID)
}

// DeleteNight removes a night entirely.
func (s *Service) DeleteNight(ctx context.Context, id uuid.UUID) error {
	n, err := s.store.GetNight(ctx, id)
	if err != nil {
		return err
	}
	if err := s.clearGroupNight(ctx, n); err != nil {
		return err
	}
	return s.store.DeleteNight(ctx, id)
}

func (s *Service) clearGroupNight(ctx context.Context, n *models.Night) error {
	g, err := s.store.GetGroup(ctx, n.GroupID)
	if err != nil {
		return err
	}
	if g.CurrNightID != nil && *g.CurrNightID == n.ID {
		return s.store.SetGroupNight(ctx, n.GroupID, nil, false)
	}
	return nil
}

// SetRange updates a night's maximum range from the center.
func (s *Service) SetRange(ctx context.Context, id uuid.UUID, maxRangeM int) (*models.Night, error) {
	if maxRangeM <= 0 {
		return nil, fmt.Errorf("%w: maxRange must be positive", ErrValidation)
	}
	if err := s.store.UpdateNightRange(ctx, id, maxRangeM); err != nil {
		return nil, err
	}
	return s.store.GetNight(ctx, id)
}

// SetCenter updates a night's center point.
func (s *Service) SetCenter(ctx context.Context, id uuid.UUID, lat, lng float64) (*models.Night, error) {
	if err := validateCoords(lat, lng); err != nil {
		return nil, err
	}
	if err := s.store.UpdateNightCenter(ctx, id, lat, lng); err != nil {
		return nil, err
	}
	return s.store.GetNight(ctx, id)
}

// ReportNightLocation records a participant's location within a night and also
// updates their global location.
func (s *Service) ReportNightLocation(ctx context.Context, nightID, userID uuid.UUID, lat, lng float64, battery *int) (*models.NightLocation, error) {
	if err := validateBattery(battery); err != nil {
		return nil, err
	}
	n, err := s.store.GetNight(ctx, nightID)
	if err != nil {
		return nil, err
	}
	isMember, err := s.store.IsMember(ctx, n.GroupID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, fmt.Errorf("%w: user is not a participant in this night's group", ErrValidation)
	}
	now := s.clock.Now()
	loc := &models.NightLocation{
		NightID: nightID, UserID: userID, Lat: lat, Lng: lng,
		BatteryLevel: battery, ReportedAt: now,
	}
	if err := s.store.UpsertNightLocation(ctx, loc); err != nil {
		return nil, err
	}
	if err := s.store.UpdateUserLocation(ctx, userID, lat, lng, battery, now); err != nil {
		return nil, err
	}
	return loc, nil
}

// GetLocationOf returns a participant's latest location within a night.
func (s *Service) GetLocationOf(ctx context.Context, nightID, userID uuid.UUID) (*models.NightLocation, error) {
	if _, err := s.store.GetNight(ctx, nightID); err != nil {
		return nil, err
	}
	return s.store.GetNightLocation(ctx, nightID, userID)
}

// GetBatteryLevelOf returns a participant's battery level, preferring the
// night-scoped report and falling back to the user's global battery.
func (s *Service) GetBatteryLevelOf(ctx context.Context, nightID, userID uuid.UUID) (int, error) {
	if _, err := s.store.GetNight(ctx, nightID); err != nil {
		return 0, err
	}
	if loc, err := s.store.GetNightLocation(ctx, nightID, userID); err == nil && loc.BatteryLevel != nil {
		return *loc.BatteryLevel, nil
	}
	u, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	if u.BatteryLevel == nil {
		return 0, fmt.Errorf("%w: no battery level known for user", ErrNotFound)
	}
	return *u.BatteryLevel, nil
}

// ListNightLocations returns all participant locations for a night.
func (s *Service) ListNightLocations(ctx context.Context, nightID uuid.UUID) ([]models.NightLocation, error) {
	if _, err := s.store.GetNight(ctx, nightID); err != nil {
		return nil, err
	}
	return s.store.ListNightLocations(ctx, nightID)
}

// ListParticipantStatuses returns all participant statuses for a night.
func (s *Service) ListParticipantStatuses(ctx context.Context, nightID uuid.UUID) ([]models.ParticipantState, error) {
	if _, err := s.store.GetNight(ctx, nightID); err != nil {
		return nil, err
	}
	return s.store.ListParticipantStatuses(ctx, nightID)
}

// Message sends a directed message to specific participants of a night.
func (s *Service) Message(ctx context.Context, nightID uuid.UUID, userIDs []uuid.UUID, body string) ([]models.Message, error) {
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("%w: message body is required", ErrValidation)
	}
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one recipient is required", ErrValidation)
	}
	n, err := s.store.GetNight(ctx, nightID)
	if err != nil {
		return nil, err
	}
	sender := s.senderName(ctx, n)
	out := make([]models.Message, 0, len(userIDs))
	for i := range userIDs {
		uid := userIDs[i]
		msg, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &nightID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: notify.KindMessage, Body: body, Sender: sender,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, *msg)
	}
	return out, nil
}

// NotifyAll broadcasts a notification to every participant of a night.
func (s *Service) NotifyAll(ctx context.Context, nightID uuid.UUID, body string) ([]models.Message, error) {
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("%w: message body is required", ErrValidation)
	}
	n, err := s.store.GetNight(ctx, nightID)
	if err != nil {
		return nil, err
	}
	members, err := s.store.ListMembers(ctx, n.GroupID)
	if err != nil {
		return nil, err
	}
	sender := s.senderName(ctx, n)
	out := make([]models.Message, 0, len(members))
	for i := range members {
		uid := members[i].UserID
		msg, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &nightID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: notify.KindNotify, Body: body, Sender: sender,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, *msg)
	}
	return out, nil
}

// ListMessages returns the messages associated with a night.
func (s *Service) ListMessages(ctx context.Context, nightID uuid.UUID) ([]models.Message, error) {
	if _, err := s.store.GetNight(ctx, nightID); err != nil {
		return nil, err
	}
	return s.store.ListMessagesByNight(ctx, nightID)
}

// Check is the core monitoring loop for a single night. It evaluates every
// participant's range, battery, and check-in freshness, records their status,
// alerts those with problems (and their trusted contacts), and auto-ends the
// night when its time limit has elapsed.
func (s *Service) Check(ctx context.Context, nightID uuid.UUID) (*CheckResult, error) {
	n, err := s.store.GetNight(ctx, nightID)
	if err != nil {
		return nil, err
	}
	if n.Status != models.NightActive {
		return nil, fmt.Errorf("%w: night is not active", ErrConflict)
	}
	now := s.clock.Now()
	res := &CheckResult{NightID: nightID}

	if n.StartedAt != nil && n.TimeLimitMin > 0 &&
		now.Sub(*n.StartedAt) >= time.Duration(n.TimeLimitMin)*time.Minute {
		if _, err := s.endNight(ctx, n); err != nil {
			return nil, err
		}
		res.Ended = true
		s.notifyAdmins(ctx, n, fmt.Sprintf("Night ended: %d-minute time limit reached.", n.TimeLimitMin), notify.KindNotify)
		return res, nil
	}

	members, err := s.store.ListMembers(ctx, n.GroupID)
	if err != nil {
		return nil, err
	}
	prevStatuses, err := s.store.ListParticipantStatuses(ctx, nightID)
	if err != nil {
		return nil, err
	}
	prevByUser := make(map[uuid.UUID]models.ParticipantState, len(prevStatuses))
	for _, ps := range prevStatuses {
		prevByUser[ps.UserID] = ps
	}
	sender := s.senderName(ctx, n)
	transitions := 0

	for _, m := range members {
		var loc *models.NightLocation
		if l, err := s.store.GetNightLocation(ctx, nightID, m.UserID); err == nil {
			loc = l
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}

		st := evaluateParticipant(*n, loc, prevByUser[m.UserID].LastCheckinAt, prevByUser[m.UserID].Status, now)
		st.NightID = nightID
		st.UserID = m.UserID
		st.UpdatedAt = now
		if err := s.store.UpsertParticipantStatus(ctx, &st); err != nil {
			return nil, err
		}
		res.Statuses = append(res.Statuses, st)

		// A participant who is OK — or who is out of range but has confirmed
		// they are safe — needs no nudge or guardian alert this tick.
		if st.Status == models.StatusOK || st.Status == models.StatusOutOfRangeSafe {
			continue
		}
		res.Alerts++

		// Only notify on a transition into a new problem state. While a
		// participant's status is unchanged across checks we stay silent so the
		// same alert isn't repeated on every check.
		if prevByUser[m.UserID].Status == st.Status {
			continue
		}
		transitions++

		// Nudge the flagged participant directly (once, on transition).
		uid := m.UserID
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &nightID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: notify.KindAlert, Sender: sender,
			Body:     fmt.Sprintf("Safety check: you are flagged as %s (%s). Please check in.", st.Status, st.Detail),
			Category: notify.CategoryAlert,
			Data:     participantAlertData(nightID, m.UserID, st.Status),
		}); err != nil {
			return nil, err
		}

		// Alert every OTHER group member (the guardians) and the affected
		// user's trusted contact.
		s.alertGuardians(ctx, n, members, m, st, sender)
		if st.Status == models.StatusMissing || st.Status == models.StatusOutOfRange {
			if u, err := s.store.GetUser(ctx, m.UserID); err == nil && strings.TrimSpace(u.TrustedContact) != "" {
				if _, err := s.notifier.Send(ctx, notify.Outbound{
					NightID: &nightID, GroupID: &n.GroupID, RecipientContact: u.TrustedContact,
					Kind: notify.KindAlert, Sender: sender,
					Body: fmt.Sprintf("Alert regarding %s: %s (%s).", u.Name, st.Status, st.Detail),
				}); err != nil {
					return nil, err
				}
			}
		}
	}

	// Only summarize to admins when something changed this check; an unchanged
	// roster of flagged participants must not re-notify on every check.
	if transitions > 0 {
		s.notifyAdmins(ctx, n, fmt.Sprintf("%d participant(s) need attention during the night.", res.Alerts), notify.KindAlert)
	}
	if err := s.store.SetNightLastChecked(ctx, nightID, now); err != nil {
		return nil, err
	}
	return res, nil
}

// CheckAllDue runs Check on every active night whose check-in interval has
// elapsed. It is invoked by the background scheduler.
func (s *Service) CheckAllDue(ctx context.Context) (int, error) {
	nights, err := s.store.ListActiveNights(ctx)
	if err != nil {
		return 0, err
	}
	now := s.clock.Now()
	count := 0
	for _, n := range nights {
		if !s.isDue(n, now) {
			continue
		}
		res, err := s.Check(ctx, n.ID)
		if err != nil {
			s.log.Error("scheduled check failed", "nightId", n.ID, "error", err)
			continue
		}
		// Prompt participants for a fresh check-in each interval, but not for a
		// night that this check just auto-ended.
		if !res.Ended {
			night := n
			if err := s.SendCheckinPrompts(ctx, &night); err != nil {
				s.log.Error("scheduled check-in prompts failed", "nightId", n.ID, "error", err)
			}
		}
		count++
	}
	return count, nil
}

func (s *Service) isDue(n models.Night, now time.Time) bool {
	if n.CheckInEveryMin <= 0 || n.LastCheckedAt == nil {
		return true
	}
	return now.Sub(*n.LastCheckedAt) >= time.Duration(n.CheckInEveryMin)*time.Minute
}

// CheckinInput is the payload for a participant's check-in acknowledgement.
type CheckinInput struct {
	OK      bool
	Lat     *float64
	Lng     *float64
	Battery *int
}

// Checkin records a participant's response to a periodic "are you OK?" prompt.
// A truthy OK resets the participant's check-in freshness timer; a falsy OK is
// treated as a distress signal that flags the participant and alerts every other
// group member. Optional coordinates/battery are persisted to the user and the
// active night. It returns the participant's updated status.
func (s *Service) Checkin(ctx context.Context, nightID, userID uuid.UUID, in CheckinInput) (*models.ParticipantState, error) {
	if (in.Lat == nil) != (in.Lng == nil) {
		return nil, fmt.Errorf("%w: lat and lng must be provided together", ErrValidation)
	}
	if in.Lat != nil {
		if err := validateCoords(*in.Lat, *in.Lng); err != nil {
			return nil, err
		}
	}
	if err := validateBattery(in.Battery); err != nil {
		return nil, err
	}

	n, err := s.store.GetNight(ctx, nightID)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	isMember, err := s.store.IsMember(ctx, n.GroupID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, fmt.Errorf("%w: user is not a participant in this night", ErrNotFound)
	}
	if n.Status != models.NightActive {
		return nil, fmt.Errorf("%w: night is not active", ErrConflict)
	}

	now := s.clock.Now()

	// Persist any reported location/battery into the user and active nights.
	switch {
	case in.Lat != nil && in.Lng != nil:
		loc := &models.NightLocation{
			NightID: nightID, UserID: userID, Lat: *in.Lat, Lng: *in.Lng,
			BatteryLevel: in.Battery, ReportedAt: now,
		}
		if err := s.store.UpsertNightLocation(ctx, loc); err != nil {
			return nil, err
		}
		if err := s.store.UpdateUserLocation(ctx, userID, *in.Lat, *in.Lng, in.Battery, now); err != nil {
			return nil, err
		}
		if err := s.store.PropagateUserLocationToActiveNights(ctx, userID, *in.Lat, *in.Lng, in.Battery, now); err != nil {
			return nil, err
		}
	case in.Battery != nil:
		if err := s.store.UpdateUserBattery(ctx, userID, *in.Battery, now); err != nil {
			return nil, err
		}
		if err := s.store.PropagateUserBatteryToActiveNights(ctx, userID, *in.Battery, now); err != nil {
			return nil, err
		}
	}

	// Record the acknowledgement so the monitoring loop treats them as fresh.
	if err := s.store.SetParticipantCheckin(ctx, nightID, userID, now); err != nil {
		return nil, err
	}

	// Recompute status from the freshest known data. The prior status lets us
	// honor the out_of_range_safe grace window and detect a transition into it.
	var loc *models.NightLocation
	if l, err := s.store.GetNightLocation(ctx, nightID, userID); err == nil {
		loc = l
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	prevStatus := models.StatusUnknown
	if prev, err := s.store.GetParticipantStatus(ctx, nightID, userID); err == nil {
		prevStatus = prev.Status
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	st := evaluateParticipant(*n, loc, &now, prevStatus, now)
	st.NightID = nightID
	st.UserID = userID
	st.UpdatedAt = now
	st.LastCheckinAt = &now

	if in.OK {
		switch st.Status {
		case models.StatusOK:
			st.Detail = fmt.Sprintf("Checked in at %s", now.UTC().Format(time.RFC3339))
		case models.StatusOutOfRange:
			// Out of range, but the participant has confirmed they are safe.
			st.Status = models.StatusOutOfRangeSafe
			st.Detail = fmt.Sprintf("Out of range but confirmed safe at %s", now.UTC().Format(time.RFC3339))
		case models.StatusOutOfRangeSafe:
			// Re-confirmation within the window; the timer resets via the
			// check-in timestamp recorded above.
			st.Detail = fmt.Sprintf("Out of range but confirmed safe at %s", now.UTC().Format(time.RFC3339))
		}
	} else {
		// The status enum has no dedicated "distress" value, so flag the
		// participant as missing with an explicit detail.
		st.Status = models.StatusMissing
		st.Detail = "User responded not OK; distress alert dispatched"
	}

	if err := s.store.UpsertParticipantStatus(ctx, &st); err != nil {
		return nil, err
	}

	switch {
	case !in.OK:
		s.dispatchDistress(ctx, n, userID, st)
	case st.Status == models.StatusOutOfRangeSafe && prevStatus != models.StatusOutOfRangeSafe:
		s.dispatchSafe(ctx, n, userID, st)
	}
	return &st, nil
}

// dispatchDistress alerts every other group member (and the affected user's
// trusted contact) that a participant has responded "not OK".
func (s *Service) dispatchDistress(ctx context.Context, n *models.Night, affectedUserID uuid.UUID, st models.ParticipantState) {
	sender := s.senderName(ctx, n)
	name := "A participant"
	affected, err := s.store.GetUser(ctx, affectedUserID)
	if err == nil {
		name = affected.Name
	}
	body := fmt.Sprintf("%s responded NOT OK. Please check on them.", name)
	data := map[string]any{
		"type":           "alert",
		"nightId":        n.ID.String(),
		"affectedUserId": affectedUserID.String(),
		"status":         string(st.Status),
	}

	members, err := s.store.ListMembers(ctx, n.GroupID)
	if err != nil {
		s.log.Error("distress: list members failed", "groupId", n.GroupID, "error", err)
		return
	}
	for _, m := range members {
		if m.UserID == affectedUserID {
			continue
		}
		uid := m.UserID
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &n.ID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: notify.KindAlert, Sender: sender, Body: body,
			Category: notify.CategoryAlert, Data: data,
		}); err != nil {
			s.log.Error("distress alert failed", "userId", uid, "error", err)
		}
	}

	if affected != nil && strings.TrimSpace(affected.TrustedContact) != "" {
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &n.ID, GroupID: &n.GroupID, RecipientContact: affected.TrustedContact,
			Kind: notify.KindAlert, Sender: sender,
			Body: fmt.Sprintf("Distress alert: %s responded not OK during the night.", name),
		}); err != nil {
			s.log.Error("distress trusted-contact alert failed", "error", err)
		}
	}
}

// dispatchSafe notifies every other group member (and the affected user's
// trusted contact) that a participant who is out of range has confirmed they
// are safe, mirroring who is alerted when they first went out of range.
func (s *Service) dispatchSafe(ctx context.Context, n *models.Night, affectedUserID uuid.UUID, st models.ParticipantState) {
	sender := s.senderName(ctx, n)
	name := "A participant"
	affected, err := s.store.GetUser(ctx, affectedUserID)
	if err == nil {
		name = affected.Name
	}
	body := fmt.Sprintf("%s is out of range but has confirmed they are safe.", name)
	data := map[string]any{
		"type":           "safe",
		"nightId":        n.ID.String(),
		"affectedUserId": affectedUserID.String(),
		"status":         string(st.Status),
	}

	members, err := s.store.ListMembers(ctx, n.GroupID)
	if err != nil {
		s.log.Error("safe: list members failed", "groupId", n.GroupID, "error", err)
		return
	}
	for _, m := range members {
		if m.UserID == affectedUserID {
			continue
		}
		uid := m.UserID
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &n.ID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: notify.KindNotify, Sender: sender, Body: body, Data: data,
		}); err != nil {
			s.log.Error("safe notification failed", "userId", uid, "error", err)
		}
	}

	if affected != nil && strings.TrimSpace(affected.TrustedContact) != "" {
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &n.ID, GroupID: &n.GroupID, RecipientContact: affected.TrustedContact,
			Kind: notify.KindNotify, Sender: sender, Body: body,
		}); err != nil {
			s.log.Error("safe trusted-contact notification failed", "error", err)
		}
	}
}

// SendCheckinPrompts pushes an "are you OK?" prompt to every participant of an
// active night. The scheduler invokes it once per check-in interval.
func (s *Service) SendCheckinPrompts(ctx context.Context, n *models.Night) error {
	members, err := s.store.ListMembers(ctx, n.GroupID)
	if err != nil {
		return err
	}
	sender := s.senderName(ctx, n)
	for _, m := range members {
		uid := m.UserID
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &n.ID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: notify.KindNotify, Sender: sender, Body: "Are you OK?",
			Category: notify.CategoryCheckin,
			Data: map[string]any{
				"type":    "checkin",
				"nightId": n.ID.String(),
				"userId":  uid.String(),
			},
		}); err != nil {
			s.log.Error("check-in prompt failed", "userId", uid, "error", err)
		}
	}
	return nil
}

func (s *Service) senderName(ctx context.Context, n *models.Night) string {
	if n.AgentID != nil {
		if a, err := s.store.GetAgent(ctx, *n.AgentID); err == nil {
			return a.Name
		}
	}
	return "NightWatch"
}

func (s *Service) notifyAdmins(ctx context.Context, n *models.Night, body, kind string) {
	admins, err := s.store.ListAdmins(ctx, n.GroupID)
	if err != nil {
		s.log.Error("list admins failed", "groupId", n.GroupID, "error", err)
		return
	}
	sender := s.senderName(ctx, n)
	for _, a := range admins {
		uid := a.UserID
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &n.ID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: kind, Body: body, Sender: sender,
		}); err != nil {
			s.log.Error("notify admin failed", "userId", uid, "error", err)
		}
	}
}

// alertGuardians notifies every group member except the affected participant
// that they have been flagged, deep-linking the push to the Guardian screen.
func (s *Service) alertGuardians(ctx context.Context, n *models.Night, members []models.Member, affected models.Member, st models.ParticipantState, sender string) {
	name := strings.TrimSpace(affected.Name)
	if name == "" {
		name = "A participant"
	}
	body := fmt.Sprintf("%s is flagged as %s (%s).", name, st.Status, st.Detail)
	data := map[string]any{
		"type":           "alert",
		"nightId":        n.ID.String(),
		"affectedUserId": affected.UserID.String(),
		"status":         string(st.Status),
	}
	for _, other := range members {
		if other.UserID == affected.UserID {
			continue
		}
		uid := other.UserID
		if _, err := s.notifier.Send(ctx, notify.Outbound{
			NightID: &n.ID, GroupID: &n.GroupID, RecipientUserID: &uid,
			Kind: notify.KindAlert, Sender: sender, Body: body,
			Category: notify.CategoryAlert, Data: data,
		}); err != nil {
			s.log.Error("guardian alert failed", "userId", uid, "error", err)
		}
	}
}

// participantAlertData builds the custom push fields for an alert addressed to
// the affected participant themselves.
func participantAlertData(nightID, userID uuid.UUID, status models.ParticipantStatus) map[string]any {
	return map[string]any{
		"type":    "alert",
		"nightId": nightID.String(),
		"userId":  userID.String(),
		"status":  string(status),
	}
}

// outOfRangeSafeWindow is how long an out_of_range participant who has confirmed
// they are safe stays out_of_range_safe before reverting to out_of_range. Each
// fresh check-in resets the window.
const outOfRangeSafeWindow = 30 * time.Minute

// evaluateParticipant computes a participant's status from their latest
// night-scoped location and most recent check-in acknowledgement. It is pure to
// keep the decision logic unit-testable.
func evaluateParticipant(n models.Night, loc *models.NightLocation, lastCheckin *time.Time, prevStatus models.ParticipantStatus, now time.Time) models.ParticipantState {
	st := models.ParticipantState{Status: models.StatusOK}

	// Freshness is the most recent of a reported location or an explicit
	// "I'm OK" check-in acknowledgement.
	var lastSeen time.Time
	if loc != nil {
		lastSeen = loc.ReportedAt
	}
	if lastCheckin != nil && lastCheckin.After(lastSeen) {
		lastSeen = *lastCheckin
	}

	if loc == nil && lastCheckin == nil {
		st.Status = models.StatusMissing
		st.Detail = "no location reported"
		return st
	}
	if n.CheckInLimitMin > 0 && now.Sub(lastSeen) > time.Duration(n.CheckInLimitMin)*time.Minute {
		st.Status = models.StatusMissing
		st.Detail = fmt.Sprintf("no check-in for over %d minutes", n.CheckInLimitMin)
		return st
	}
	if loc != nil && n.HasCenter() {
		d := DistanceMeters(*n.CenterLat, *n.CenterLng, loc.Lat, loc.Lng)
		dist := d
		st.DistanceM = &dist
		if d > float64(n.MaxRangeM) {
			// A participant who explicitly confirmed they are safe stays
			// out_of_range_safe until the grace window lapses; each fresh
			// check-in resets it.
			if prevStatus == models.StatusOutOfRangeSafe && lastCheckin != nil &&
				now.Sub(*lastCheckin) < outOfRangeSafeWindow {
				st.Status = models.StatusOutOfRangeSafe
				st.Detail = fmt.Sprintf("%.0f m from center but confirmed safe", d)
				return st
			}
			st.Status = models.StatusOutOfRange
			st.Detail = fmt.Sprintf("%.0f m from center (max %d m)", d, n.MaxRangeM)
			return st
		}
	}
	if loc != nil && loc.BatteryLevel != nil && *loc.BatteryLevel < n.LowBatteryThreshold {
		st.Status = models.StatusLowBattery
		st.Detail = fmt.Sprintf("battery at %d%% (min %d%%)", *loc.BatteryLevel, n.LowBatteryThreshold)
		return st
	}
	return st
}

func validateNight(n *models.Night) error {
	if n.TimeLimitMin < 0 || n.CheckInLimitMin < 0 || n.CheckInEveryMin < 0 {
		return fmt.Errorf("%w: time values must not be negative", ErrValidation)
	}
	if n.MaxRangeM <= 0 {
		return fmt.Errorf("%w: maxRange must be positive", ErrValidation)
	}
	if n.LowBatteryThreshold < 0 || n.LowBatteryThreshold > 100 {
		return fmt.Errorf("%w: lowBatteryThreshold must be between 0 and 100", ErrValidation)
	}
	return nil
}

// validateCoords ensures a latitude/longitude pair is within valid geographic
// bounds.
func validateCoords(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return fmt.Errorf("%w: lat must be between -90 and 90", ErrValidation)
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("%w: lng must be between -180 and 180", ErrValidation)
	}
	return nil
}

func derefInt(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}
