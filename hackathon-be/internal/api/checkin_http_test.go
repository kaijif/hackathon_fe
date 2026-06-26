package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
	"github.com/t-kaijifu/hackathon-be/internal/notify"
	"github.com/t-kaijifu/hackathon-be/internal/service"
	"github.com/t-kaijifu/hackathon-be/internal/store"
)

// stubStore embeds store.Store so only the methods exercised by the check-in
// HTTP path need implementing; any unexpected call panics on the nil interface.
type stubStore struct {
	store.Store
	night  *models.Night
	user   *models.User
	member bool
}

func (s *stubStore) GetNight(_ context.Context, id uuid.UUID) (*models.Night, error) {
	if s.night == nil || s.night.ID != id {
		return nil, store.ErrNotFound
	}
	n := *s.night
	return &n, nil
}

func (s *stubStore) GetUser(_ context.Context, id uuid.UUID) (*models.User, error) {
	if s.user == nil || s.user.ID != id {
		return nil, store.ErrNotFound
	}
	u := *s.user
	return &u, nil
}

func (s *stubStore) IsMember(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return s.member, nil
}

func (s *stubStore) SetParticipantCheckin(_ context.Context, _, _ uuid.UUID, _ time.Time) error {
	return nil
}

func (s *stubStore) GetNightLocation(_ context.Context, _, _ uuid.UUID) (*models.NightLocation, error) {
	return nil, store.ErrNotFound
}

func (s *stubStore) UpsertParticipantStatus(_ context.Context, _ *models.ParticipantState) error {
	return nil
}

func (s *stubStore) GetParticipantStatus(_ context.Context, _, _ uuid.UUID) (*models.ParticipantState, error) {
	return nil, store.ErrNotFound
}

type noopNotifier struct{}

func (noopNotifier) Send(_ context.Context, msg notify.Outbound) (*models.Message, error) {
	return &models.Message{ID: uuid.New(), Kind: msg.Kind, Body: msg.Body}, nil
}

func newCheckinServer(st store.Store) *httptest.Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(st, noopNotifier{}, service.WithLogger(logger))
	return httptest.NewServer(NewServer(svc, logger).Handler())
}

func postJSON(t *testing.T, url string, body any) (int, []byte) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestCheckinEndpoint(t *testing.T) {
	nightID := uuid.New()
	userID := uuid.New()
	groupID := uuid.New()
	activeNight := &models.Night{ID: nightID, GroupID: groupID, Status: models.NightActive, MaxRangeM: 1000, CheckInLimitMin: 30}

	t.Run("ok returns 200 with participant status", func(t *testing.T) {
		ts := newCheckinServer(&stubStore{
			night:  activeNight,
			user:   &models.User{ID: userID, Name: "Alex"},
			member: true,
		})
		defer ts.Close()

		code, data := postJSON(t, ts.URL+"/nights/"+nightID.String()+"/checkin/"+userID.String(), map[string]any{"ok": true})
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", code, data)
		}
		var st models.ParticipantState
		if err := json.Unmarshal(data, &st); err != nil {
			t.Fatalf("decode: %v (%s)", err, data)
		}
		if st.Status != models.StatusOK {
			t.Fatalf("status = %q, want ok", st.Status)
		}
		if st.UserID != userID || st.NightID != nightID {
			t.Fatalf("unexpected ids in response: %+v", st)
		}
	})

	t.Run("missing ok returns 400", func(t *testing.T) {
		ts := newCheckinServer(&stubStore{night: activeNight, user: &models.User{ID: userID}, member: true})
		defer ts.Close()
		code, _ := postJSON(t, ts.URL+"/nights/"+nightID.String()+"/checkin/"+userID.String(), map[string]any{"lat": 1.0, "lng": 2.0})
		if code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", code)
		}
	})

	t.Run("unknown night returns 404", func(t *testing.T) {
		ts := newCheckinServer(&stubStore{member: true})
		defer ts.Close()
		code, _ := postJSON(t, ts.URL+"/nights/"+uuid.New().String()+"/checkin/"+userID.String(), map[string]any{"ok": true})
		if code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", code)
		}
	})

	t.Run("non-active night returns 409", func(t *testing.T) {
		pending := &models.Night{ID: nightID, GroupID: groupID, Status: models.NightPending, MaxRangeM: 1000}
		ts := newCheckinServer(&stubStore{night: pending, user: &models.User{ID: userID}, member: true})
		defer ts.Close()
		code, _ := postJSON(t, ts.URL+"/nights/"+nightID.String()+"/checkin/"+userID.String(), map[string]any{"ok": true})
		if code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", code)
		}
	})
}
