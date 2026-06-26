package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"

	"github.com/t-kaijifu/hackathon-be/internal/api"
	"github.com/t-kaijifu/hackathon-be/internal/db"
	"github.com/t-kaijifu/hackathon-be/internal/notify"
	"github.com/t-kaijifu/hackathon-be/internal/service"
	"github.com/t-kaijifu/hackathon-be/internal/store"
)

const dsn = "postgres://nightwatch:nightwatch@localhost:54329/nightwatch?sslmode=disable"

var baseURL string

func TestMain(m *testing.M) {
	pg := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Username("nightwatch").
			Password("nightwatch").
			Database("nightwatch").
			Port(54329).
			Version(embeddedpostgres.V16).
			StartTimeout(90 * time.Second),
	)
	if err := pg.Start(); err != nil {
		fmt.Println("failed to start embedded postgres:", err)
		os.Exit(1)
	}

	code := func() int {
		ctx := context.Background()
		pool, err := db.Connect(ctx, dsn)
		if err != nil {
			fmt.Println("db connect:", err)
			return 1
		}
		defer pool.Close()
		if err := db.Migrate(ctx, pool); err != nil {
			fmt.Println("migrate:", err)
			return 1
		}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		st := store.New(pool)
		nf := notify.NewDBNotifier(st, logger)
		svc := service.New(st, nf, service.WithLogger(logger))
		ts := httptest.NewServer(api.NewServer(svc, logger).Handler())
		defer ts.Close()
		baseURL = ts.URL
		return m.Run()
	}()

	_ = pg.Stop()
	os.Exit(code)
}

func do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func obj(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode object: %v (%s)", err, string(data))
	}
	return m
}

func arr(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("decode array: %v (%s)", err, string(data))
	}
	return a
}

func requireStatus(t *testing.T, want, got int, data []byte) {
	t.Helper()
	if got != want {
		t.Fatalf("expected status %d, got %d: %s", want, got, string(data))
	}
}

func TestEndToEnd(t *testing.T) {
	// Create two users; Alex has a trusted contact.
	st, data := do(t, "POST", "/users", map[string]any{"name": "Alex", "trustedContact": "+15551112222"})
	requireStatus(t, 201, st, data)
	alex := obj(t, data)["id"].(string)

	st, data = do(t, "POST", "/users", map[string]any{"name": "Sam", "trustedContact": "+15553334444"})
	requireStatus(t, 201, st, data)
	sam := obj(t, data)["id"].(string)

	// Alex forms a group; Sam joins.
	st, data = do(t, "POST", "/groups", map[string]any{"name": "Night Crew", "creatorUserId": alex})
	requireStatus(t, 201, st, data)
	gid := obj(t, data)["id"].(string)

	st, data = do(t, "POST", "/groups/"+gid+"/join", map[string]any{"userId": sam})
	requireStatus(t, 204, st, data)

	// Alex must be an admin (formGroup makes the creator an admin).
	st, data = do(t, "GET", "/groups/"+gid+"/admins", nil)
	requireStatus(t, 200, st, data)
	admins := arr(t, data)
	if len(admins) != 1 || admins[0]["userId"].(string) != alex {
		t.Fatalf("expected Alex to be the sole admin, got %s", string(data))
	}

	// Create and start a night centered downtown with a 1km range.
	st, data = do(t, "POST", "/groups/"+gid+"/nights", map[string]any{
		"center":              map[string]any{"lat": 40.7128, "lng": -74.0060},
		"maxRangeM":           1000,
		"checkInEveryMin":     5,
		"lowBatteryThreshold": 20,
	})
	requireStatus(t, 201, st, data)
	night := obj(t, data)
	nid := night["id"].(string)
	if night["status"].(string) != "pending" {
		t.Fatalf("new night should be pending, got %v", night["status"])
	}

	st, data = do(t, "POST", "/nights/"+nid+"/start", nil)
	requireStatus(t, 200, st, data)
	if obj(t, data)["status"].(string) != "active" {
		t.Fatalf("night should be active after start")
	}

	// Report locations: Alex near center, Sam ~4km away with low battery.
	st, data = do(t, "PUT", "/nights/"+nid+"/locations/"+alex,
		map[string]any{"lat": 40.7129, "lng": -74.0061, "batteryLevel": 90})
	requireStatus(t, 200, st, data)

	st, data = do(t, "PUT", "/nights/"+nid+"/locations/"+sam,
		map[string]any{"lat": 40.7500, "lng": -74.0060, "batteryLevel": 10})
	requireStatus(t, 200, st, data)

	// Run the monitoring loop.
	st, data = do(t, "POST", "/nights/"+nid+"/check", nil)
	requireStatus(t, 200, st, data)
	result := obj(t, data)
	if result["alerts"].(float64) < 1 {
		t.Fatalf("expected at least one alert, got %v", result["alerts"])
	}

	// Statuses: Sam out_of_range, Alex ok.
	st, data = do(t, "GET", "/nights/"+nid+"/statuses", nil)
	requireStatus(t, 200, st, data)
	statusByUser := map[string]string{}
	for _, s := range arr(t, data) {
		statusByUser[s["userId"].(string)] = s["status"].(string)
	}
	if statusByUser[sam] != "out_of_range" {
		t.Fatalf("Sam should be out_of_range, got %q", statusByUser[sam])
	}
	if statusByUser[alex] != "ok" {
		t.Fatalf("Alex should be ok, got %q", statusByUser[alex])
	}

	// Messages were persisted, including a trusted-contact alert.
	st, data = do(t, "GET", "/nights/"+nid+"/messages", nil)
	requireStatus(t, 200, st, data)
	msgs := arr(t, data)
	if len(msgs) == 0 {
		t.Fatal("expected dispatched messages to be persisted")
	}
	trusted := false
	for _, m := range msgs {
		if c, ok := m["recipientContact"].(string); ok && c == "+15553334444" {
			trusted = true
		}
	}
	if !trusted {
		t.Fatal("expected an alert to the out-of-range user's trusted contact")
	}
}

func TestNightViewAndActiveNightFilter(t *testing.T) {
	// Build a fresh user/group/night and verify the aggregated night view and
	// the activeNight group filter.
	_, data := do(t, "POST", "/users", map[string]any{"name": "Riley"})
	riley := obj(t, data)["id"].(string)

	_, data = do(t, "POST", "/groups", map[string]any{"name": "Riley Crew", "creatorUserId": riley})
	gid := obj(t, data)["id"].(string)

	// No active night yet.
	_, data = do(t, "GET", "/users/"+riley+"/groups?activeNight=true", nil)
	if len(arr(t, data)) != 0 {
		t.Fatal("expected no groups with an active night yet")
	}

	_, data = do(t, "POST", "/groups/"+gid+"/nights", map[string]any{"maxRangeM": 500})
	nid := obj(t, data)["id"].(string)
	do(t, "POST", "/nights/"+nid+"/start", nil)

	// Now the group should appear in the active-night filter.
	_, data = do(t, "GET", "/users/"+riley+"/groups?activeNight=true", nil)
	if len(arr(t, data)) != 1 {
		t.Fatalf("expected one group with an active night, got %s", string(data))
	}

	// The night view should embed config and (empty) participant collections.
	status, data := do(t, "GET", "/nights/"+nid, nil)
	requireStatus(t, 200, status, data)
	view := obj(t, data)
	if _, ok := view["currentLocations"]; !ok {
		t.Fatal("night view should include currentLocations")
	}
	if _, ok := view["participantStatuses"]; !ok {
		t.Fatal("night view should include participantStatuses")
	}

	// End the night; the group becomes inactive.
	do(t, "POST", "/nights/"+nid+"/end", nil)
	_, data = do(t, "GET", "/groups/"+gid, nil)
	if obj(t, data)["active"].(bool) {
		t.Fatal("group should be inactive after the night ends")
	}
}

func TestDeviceRegistration(t *testing.T) {
	_, data := do(t, "POST", "/users", map[string]any{"name": "Devon"})
	uid := obj(t, data)["id"].(string)

	// Register an iOS device token.
	status, data := do(t, "POST", "/users/"+uid+"/devices",
		map[string]any{"platform": "ios", "token": "devtoken-abc-123"})
	requireStatus(t, 201, status, data)
	dt := obj(t, data)
	if dt["platform"].(string) != "ios" || dt["token"].(string) != "devtoken-abc-123" {
		t.Fatalf("unexpected device token: %s", string(data))
	}

	status, data = do(t, "GET", "/users/"+uid+"/devices", nil)
	requireStatus(t, 200, status, data)
	if len(arr(t, data)) != 1 {
		t.Fatalf("expected 1 device, got %s", string(data))
	}

	// Re-registering the same token is an upsert, not a duplicate.
	do(t, "POST", "/users/"+uid+"/devices", map[string]any{"platform": "ios", "token": "devtoken-abc-123"})
	_, data = do(t, "GET", "/users/"+uid+"/devices", nil)
	if len(arr(t, data)) != 1 {
		t.Fatalf("upsert should not duplicate, got %s", string(data))
	}

	// Unregister.
	status, data = do(t, "DELETE", "/users/"+uid+"/devices/devtoken-abc-123", nil)
	requireStatus(t, 204, status, data)
	_, data = do(t, "GET", "/users/"+uid+"/devices", nil)
	if len(arr(t, data)) != 0 {
		t.Fatalf("expected 0 devices after delete, got %s", string(data))
	}
}
