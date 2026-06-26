package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
	"github.com/t-kaijifu/hackathon-be/internal/notify"
	"github.com/t-kaijifu/hackathon-be/internal/store"
)

// fakeClock is a controllable clock for tests.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// captureNotifier records every message it is asked to send.
type captureNotifier struct {
	mu   sync.Mutex
	sent []notify.Outbound
}

func (n *captureNotifier) Send(_ context.Context, msg notify.Outbound) (*models.Message, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, msg)
	m := &models.Message{ID: uuid.New(), Kind: msg.Kind, Body: msg.Body}
	return m, nil
}

func (n *captureNotifier) countByKind(kind string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	c := 0
	for _, m := range n.sent {
		if m.Kind == kind {
			c++
		}
	}
	return c
}

// memStore is a full in-memory implementation of store.Store for tests.
type memStore struct {
	mu        sync.Mutex
	users     map[uuid.UUID]models.User
	agents    map[uuid.UUID]models.Agent
	groups    map[uuid.UUID]models.Group
	members   map[uuid.UUID]map[uuid.UUID]models.Member // groupID -> userID -> member
	nights    map[uuid.UUID]models.Night
	locations map[uuid.UUID]map[uuid.UUID]models.NightLocation    // nightID -> userID -> loc
	statuses  map[uuid.UUID]map[uuid.UUID]models.ParticipantState // nightID -> userID -> status
	messages  []models.Message
	devices   map[string]models.DeviceToken // token -> device
}

func newMemStore() *memStore {
	return &memStore{
		users:     map[uuid.UUID]models.User{},
		agents:    map[uuid.UUID]models.Agent{},
		groups:    map[uuid.UUID]models.Group{},
		members:   map[uuid.UUID]map[uuid.UUID]models.Member{},
		nights:    map[uuid.UUID]models.Night{},
		locations: map[uuid.UUID]map[uuid.UUID]models.NightLocation{},
		statuses:  map[uuid.UUID]map[uuid.UUID]models.ParticipantState{},
		devices:   map[string]models.DeviceToken{},
	}
}

var _ store.Store = (*memStore)(nil)

func (m *memStore) CreateUser(_ context.Context, u *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	u.CreatedAt, u.UpdatedAt = now, now
	m.users[u.ID] = *u
	return nil
}

func (m *memStore) GetUser(_ context.Context, id uuid.UUID) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &u, nil
}

func (m *memStore) ListUsers(_ context.Context) ([]models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]models.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u)
	}
	return out, nil
}

func (m *memStore) UpdateUserLocation(_ context.Context, id uuid.UUID, lat, lng float64, battery *int, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return store.ErrNotFound
	}
	u.Lat, u.Lng = &lat, &lng
	if battery != nil {
		u.BatteryLevel = battery
	}
	u.LocationUpdatedAt = &at
	u.UpdatedAt = at
	m.users[id] = u
	return nil
}

func (m *memStore) UpdateUserBattery(_ context.Context, id uuid.UUID, battery int, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return store.ErrNotFound
	}
	u.BatteryLevel = &battery
	u.UpdatedAt = at
	m.users[id] = u
	return nil
}

func (m *memStore) CreateAgent(_ context.Context, a *models.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a.CreatedAt = time.Now()
	m.agents[a.ID] = *a
	return nil
}

func (m *memStore) GetAgent(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.agents[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &a, nil
}

func (m *memStore) ListAgents(_ context.Context) ([]models.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]models.Agent, 0, len(m.agents))
	for _, a := range m.agents {
		out = append(out, a)
	}
	return out, nil
}

func (m *memStore) CreateGroup(_ context.Context, g *models.Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	g.CreatedAt, g.UpdatedAt = now, now
	m.groups[g.ID] = *g
	m.members[g.ID] = map[uuid.UUID]models.Member{}
	return nil
}

func (m *memStore) GetGroup(_ context.Context, id uuid.UUID) (*models.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &g, nil
}

func (m *memStore) ListGroups(_ context.Context) ([]models.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]models.Group, 0, len(m.groups))
	for _, g := range m.groups {
		out = append(out, g)
	}
	return out, nil
}

func (m *memStore) DeleteGroup(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.groups[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.groups, id)
	delete(m.members, id)
	return nil
}

func (m *memStore) SetGroupNight(_ context.Context, groupID uuid.UUID, nightID *uuid.UUID, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return store.ErrNotFound
	}
	g.CurrNightID = nightID
	g.Active = active
	m.groups[groupID] = g
	return nil
}

func (m *memStore) ListGroupsForUser(_ context.Context, userID uuid.UUID, onlyActiveNight bool) ([]models.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Group
	for gid, members := range m.members {
		if _, ok := members[userID]; !ok {
			continue
		}
		g := m.groups[gid]
		if onlyActiveNight && !m.hasActiveNightLocked(gid) {
			continue
		}
		out = append(out, g)
	}
	return out, nil
}

func (m *memStore) hasActiveNightLocked(groupID uuid.UUID) bool {
	for _, n := range m.nights {
		if n.GroupID == groupID && n.Status == models.NightActive {
			return true
		}
	}
	return false
}

func (m *memStore) AddMember(_ context.Context, groupID, userID uuid.UUID, isAdmin bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.members[groupID]; !ok {
		m.members[groupID] = map[uuid.UUID]models.Member{}
	}
	existing, ok := m.members[groupID][userID]
	name := ""
	if u, uok := m.users[userID]; uok {
		name = u.Name
	}
	admin := isAdmin
	joined := time.Now()
	if ok {
		admin = existing.IsAdmin || isAdmin
		joined = existing.JoinedAt
	}
	m.members[groupID][userID] = models.Member{UserID: userID, Name: name, IsAdmin: admin, JoinedAt: joined}
	return nil
}

func (m *memStore) RemoveMember(_ context.Context, groupID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	members, ok := m.members[groupID]
	if !ok {
		return store.ErrNotFound
	}
	if _, ok := members[userID]; !ok {
		return store.ErrNotFound
	}
	delete(members, userID)
	return nil
}

func (m *memStore) SetAdmin(_ context.Context, groupID, userID uuid.UUID, isAdmin bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	members, ok := m.members[groupID]
	if !ok {
		return store.ErrNotFound
	}
	mem, ok := members[userID]
	if !ok {
		return store.ErrNotFound
	}
	mem.IsAdmin = isAdmin
	members[userID] = mem
	return nil
}

func (m *memStore) IsMember(_ context.Context, groupID, userID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	members, ok := m.members[groupID]
	if !ok {
		return false, nil
	}
	_, ok = members[userID]
	return ok, nil
}

func (m *memStore) ListMembers(_ context.Context, groupID uuid.UUID) ([]models.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Member
	for _, mem := range m.members[groupID] {
		out = append(out, mem)
	}
	return out, nil
}

func (m *memStore) ListAdmins(_ context.Context, groupID uuid.UUID) ([]models.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Member
	for _, mem := range m.members[groupID] {
		if mem.IsAdmin {
			out = append(out, mem)
		}
	}
	return out, nil
}

func (m *memStore) CreateNight(_ context.Context, n *models.Night) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	n.CreatedAt, n.UpdatedAt = now, now
	m.nights[n.ID] = *n
	return nil
}

func (m *memStore) GetNight(_ context.Context, id uuid.UUID) (*models.Night, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nights[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &n, nil
}

func (m *memStore) ListNightsByGroup(_ context.Context, groupID uuid.UUID) ([]models.Night, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Night
	for _, n := range m.nights {
		if n.GroupID == groupID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *memStore) ListActiveNights(_ context.Context) ([]models.Night, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Night
	for _, n := range m.nights {
		if n.Status == models.NightActive {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *memStore) UpdateNightLifecycle(_ context.Context, id uuid.UUID, status models.NightStatus, startedAt, endedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nights[id]
	if !ok {
		return store.ErrNotFound
	}
	n.Status = status
	if startedAt != nil {
		n.StartedAt = startedAt
	}
	if endedAt != nil {
		n.EndedAt = endedAt
	}
	n.UpdatedAt = time.Now()
	m.nights[id] = n
	return nil
}

func (m *memStore) UpdateNightRange(_ context.Context, id uuid.UUID, maxRangeM int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nights[id]
	if !ok {
		return store.ErrNotFound
	}
	n.MaxRangeM = maxRangeM
	m.nights[id] = n
	return nil
}

func (m *memStore) UpdateNightCenter(_ context.Context, id uuid.UUID, lat, lng float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nights[id]
	if !ok {
		return store.ErrNotFound
	}
	n.CenterLat = &lat
	n.CenterLng = &lng
	m.nights[id] = n
	return nil
}

func (m *memStore) SetNightLastChecked(_ context.Context, id uuid.UUID, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nights[id]
	if !ok {
		return store.ErrNotFound
	}
	n.LastCheckedAt = &at
	m.nights[id] = n
	return nil
}

func (m *memStore) DeleteNight(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nights[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.nights, id)
	return nil
}

func (m *memStore) UpsertNightLocation(_ context.Context, loc *models.NightLocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.locations[loc.NightID]; !ok {
		m.locations[loc.NightID] = map[uuid.UUID]models.NightLocation{}
	}
	if loc.ReportedAt.IsZero() {
		loc.ReportedAt = time.Now()
	}
	m.locations[loc.NightID][loc.UserID] = *loc
	return nil
}

func (m *memStore) GetNightLocation(_ context.Context, nightID, userID uuid.UUID) (*models.NightLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	byUser, ok := m.locations[nightID]
	if !ok {
		return nil, store.ErrNotFound
	}
	loc, ok := byUser[userID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &loc, nil
}

func (m *memStore) ListNightLocations(_ context.Context, nightID uuid.UUID) ([]models.NightLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.NightLocation
	for _, l := range m.locations[nightID] {
		out = append(out, l)
	}
	return out, nil
}

func (m *memStore) PropagateUserLocationToActiveNights(_ context.Context, userID uuid.UUID, lat, lng float64, battery *int, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.nights {
		if n.Status != models.NightActive {
			continue
		}
		members, ok := m.members[n.GroupID]
		if !ok {
			continue
		}
		if _, ok := members[userID]; !ok {
			continue
		}
		if _, ok := m.locations[n.ID]; !ok {
			m.locations[n.ID] = map[uuid.UUID]models.NightLocation{}
		}
		m.locations[n.ID][userID] = models.NightLocation{
			NightID: n.ID, UserID: userID, Lat: lat, Lng: lng, BatteryLevel: battery, ReportedAt: at,
		}
	}
	return nil
}

func (m *memStore) PropagateUserBatteryToActiveNights(_ context.Context, userID uuid.UUID, battery int, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for nightID, byUser := range m.locations {
		n, ok := m.nights[nightID]
		if !ok || n.Status != models.NightActive {
			continue
		}
		if loc, ok := byUser[userID]; ok {
			loc.BatteryLevel = &battery
			byUser[userID] = loc
		}
	}
	return nil
}

func (m *memStore) UpsertParticipantStatus(_ context.Context, st *models.ParticipantState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.statuses[st.NightID]; !ok {
		m.statuses[st.NightID] = map[uuid.UUID]models.ParticipantState{}
	}
	if st.UpdatedAt.IsZero() {
		st.UpdatedAt = time.Now()
	}
	// Mirror the SQL store: a status recompute must not clear check-in freshness.
	stored := *st
	if prev, ok := m.statuses[st.NightID][st.UserID]; ok && stored.LastCheckinAt == nil {
		stored.LastCheckinAt = prev.LastCheckinAt
	}
	m.statuses[st.NightID][st.UserID] = stored
	return nil
}

func (m *memStore) GetParticipantStatus(_ context.Context, nightID, userID uuid.UUID) (*models.ParticipantState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	byUser, ok := m.statuses[nightID]
	if !ok {
		return nil, store.ErrNotFound
	}
	st, ok := byUser[userID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &st, nil
}

func (m *memStore) SetParticipantCheckin(_ context.Context, nightID, userID uuid.UUID, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.statuses[nightID]; !ok {
		m.statuses[nightID] = map[uuid.UUID]models.ParticipantState{}
	}
	st, ok := m.statuses[nightID][userID]
	if !ok {
		st = models.ParticipantState{NightID: nightID, UserID: userID, Status: models.StatusUnknown}
	}
	t := at
	st.LastCheckinAt = &t
	m.statuses[nightID][userID] = st
	return nil
}

func (m *memStore) ListParticipantStatuses(_ context.Context, nightID uuid.UUID) ([]models.ParticipantState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.ParticipantState
	for _, st := range m.statuses[nightID] {
		out = append(out, st)
	}
	return out, nil
}

func (m *memStore) CreateMessage(_ context.Context, msg *models.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.CreatedAt = time.Now()
	m.messages = append(m.messages, *msg)
	return nil
}

func (m *memStore) ListMessagesByNight(_ context.Context, nightID uuid.UUID) ([]models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Message
	for _, msg := range m.messages {
		if msg.NightID != nil && *msg.NightID == nightID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *memStore) AddDeviceToken(_ context.Context, dt *models.DeviceToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	dt.CreatedAt, dt.UpdatedAt = now, now
	m.devices[dt.Token] = *dt
	return nil
}

func (m *memStore) RemoveDeviceToken(_ context.Context, userID uuid.UUID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	dt, ok := m.devices[token]
	if !ok || dt.UserID != userID {
		return store.ErrNotFound
	}
	delete(m.devices, token)
	return nil
}

func (m *memStore) ListDeviceTokensForUser(_ context.Context, userID uuid.UUID) ([]models.DeviceToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.DeviceToken
	for _, dt := range m.devices {
		if dt.UserID == userID {
			out = append(out, dt)
		}
	}
	return out, nil
}
