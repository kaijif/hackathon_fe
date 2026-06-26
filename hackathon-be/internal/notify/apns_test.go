package notify_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
	"github.com/t-kaijifu/hackathon-be/internal/notify"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func genAuthKey(t *testing.T) ([]byte, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), &priv.PublicKey
}

type stubLister struct{ tokens []models.DeviceToken }

func (s stubLister) ListDeviceTokensForUser(_ context.Context, _ uuid.UUID) ([]models.DeviceToken, error) {
	return s.tokens, nil
}

type removedDeviceToken struct {
	userID uuid.UUID
	token  string
}

type invalidatingStub struct {
	mu      sync.Mutex
	tokens  []models.DeviceToken
	removed []removedDeviceToken
}

func (s *invalidatingStub) ListDeviceTokensForUser(_ context.Context, _ uuid.UUID) ([]models.DeviceToken, error) {
	return s.tokens, nil
}

func (s *invalidatingStub) RemoveDeviceToken(_ context.Context, userID uuid.UUID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removed = append(s.removed, removedDeviceToken{userID: userID, token: token})
	return nil
}

func (s *invalidatingStub) removedTokens() []removedDeviceToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]removedDeviceToken, len(s.removed))
	copy(out, s.removed)
	return out
}

type capturedReq struct {
	path  string
	topic string
	auth  string
	body  []byte
}

func TestAPNsPushBuildsValidRequestAndJWT(t *testing.T) {
	keyPEM, pub := genAuthKey(t)

	var mu sync.Mutex
	var reqs []capturedReq
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, capturedReq{
			path:  r.URL.Path,
			topic: r.Header.Get("apns-topic"),
			auth:  r.Header.Get("authorization"),
			body:  b,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	uid := uuid.New()
	lister := stubLister{tokens: []models.DeviceToken{
		{UserID: uid, Platform: "ios", Token: "iostoken123"},
		{UserID: uid, Platform: "android", Token: "androidtoken"}, // must be ignored
	}}

	n, err := notify.NewAPNsNotifier(notify.APNsConfig{
		KeyID:      "KEY123",
		TeamID:     "TEAM456",
		Topic:      "com.example.nightwatch",
		Host:       ts.URL,
		HTTPClient: ts.Client(),
	}, keyPEM, lister, discardLogger())
	if err != nil {
		t.Fatalf("new apns notifier: %v", err)
	}

	if _, err := n.Send(context.Background(), notify.Outbound{
		RecipientUserID: &uid,
		Kind:            notify.KindAlert,
		Body:            "Please check in.",
		Sender:          "NightWatch",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	if len(reqs) != 1 {
		t.Fatalf("expected exactly 1 push (ios only), got %d", len(reqs))
	}
	got := reqs[0]
	if got.path != "/3/device/iostoken123" {
		t.Fatalf("unexpected path %q", got.path)
	}
	if got.topic != "com.example.nightwatch" {
		t.Fatalf("unexpected apns-topic %q", got.topic)
	}

	// Payload shape: aps.alert.body
	var payload struct {
		APS struct {
			Alert struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"alert"`
		} `json:"aps"`
	}
	if err := json.Unmarshal(got.body, &payload); err != nil {
		t.Fatalf("decode payload: %v (%s)", err, got.body)
	}
	if payload.APS.Alert.Body != "Please check in." {
		t.Fatalf("unexpected alert body %q", payload.APS.Alert.Body)
	}

	// The bearer token must be a valid ES256 JWT signed by our key.
	token, ok := strings.CutPrefix(got.auth, "bearer ")
	if !ok {
		t.Fatalf("authorization header missing bearer prefix: %q", got.auth)
	}
	verifyES256JWT(t, token, pub, "KEY123", "TEAM456")
}

func TestAPNsDoesNotRemoveTokenOnPermanentRejection(t *testing.T) {
	keyPEM, _ := genAuthKey(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
	}))
	defer ts.Close()

	uid := uuid.New()
	lister := &invalidatingStub{tokens: []models.DeviceToken{
		{UserID: uid, Platform: "ios", Token: "deadtoken"},
	}}
	n, err := notify.NewAPNsNotifier(notify.APNsConfig{
		KeyID: "K", TeamID: "T", Topic: "com.example.app", Host: ts.URL, HTTPClient: ts.Client(),
	}, keyPEM, lister, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	// A permanent APNs rejection (e.g. 410 Unregistered) must surface as an
	// error, but must NOT delete the device token: auto-invalidation has been
	// removed so registrations always persist.
	if _, err := n.Send(context.Background(), notify.Outbound{
		RecipientUserID: &uid,
		Kind:            notify.KindAlert,
		Body:            "x",
	}); err == nil {
		t.Fatal("expected APNs rejection to be returned as an error")
	}

	if removed := lister.removedTokens(); len(removed) != 0 {
		t.Fatalf("tokens must never be auto-removed, got %+v", removed)
	}
}

func TestAPNsDoesNotRemoveTokenOnTransientFailure(t *testing.T) {
	keyPEM, _ := genAuthKey(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"reason":"InternalServerError"}`))
	}))
	defer ts.Close()

	uid := uuid.New()
	lister := &invalidatingStub{tokens: []models.DeviceToken{
		{UserID: uid, Platform: "ios", Token: "retrytoken"},
	}}
	n, err := notify.NewAPNsNotifier(notify.APNsConfig{
		KeyID: "K", TeamID: "T", Topic: "com.example.app", Host: ts.URL, HTTPClient: ts.Client(),
	}, keyPEM, lister, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := n.Send(context.Background(), notify.Outbound{
		RecipientUserID: &uid,
		Kind:            notify.KindAlert,
		Body:            "x",
	}); err == nil {
		t.Fatal("expected transient APNs failure to be returned")
	}

	if removed := lister.removedTokens(); len(removed) != 0 {
		t.Fatalf("transient failure should not remove tokens, got %+v", removed)
	}
}

func verifyES256JWT(t *testing.T, token string, pub *ecdsa.PublicKey, wantKid, wantIss string) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}

	hdr, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var h map[string]string
	if err := json.Unmarshal(hdr, &h); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if h["alg"] != "ES256" || h["kid"] != wantKid {
		t.Fatalf("unexpected header: %v", h)
	}

	claims, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var c map[string]any
	if err := json.Unmarshal(claims, &c); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if c["iss"] != wantIss {
		t.Fatalf("unexpected iss: %v", c["iss"])
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("expected 64-byte signature, got %d", len(sig))
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(pub, digest[:], r, s) {
		t.Fatal("JWT signature verification failed")
	}
}

func TestAPNsSkipsContactOnlyMessages(t *testing.T) {
	keyPEM, _ := genAuthKey(t)
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	n, err := notify.NewAPNsNotifier(notify.APNsConfig{
		KeyID: "K", TeamID: "T", Topic: "com.example.app", Host: ts.URL, HTTPClient: ts.Client(),
	}, keyPEM, stubLister{}, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	// No RecipientUserID (a trusted-contact phone alert) => nothing to push.
	msg := notify.Outbound{RecipientContact: "+15551234567", Kind: notify.KindAlert, Body: "x"}
	if _, err := n.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("APNs should not be called for contact-only messages")
	}
}

type recNotifier struct {
	got []notify.Outbound
	err error
}

func (r *recNotifier) Send(_ context.Context, msg notify.Outbound) (*models.Message, error) {
	r.got = append(r.got, msg)
	return nil, r.err
}

type fakeSink struct{ created int }

func (f *fakeSink) CreateMessage(_ context.Context, _ *models.Message) error {
	f.created++
	return nil
}

func TestMultiNotifierFansOutAndToleratesExtraErrors(t *testing.T) {
	sink := &fakeSink{}
	primary := notify.NewDBNotifier(sink, discardLogger())
	good := &recNotifier{}
	bad := &recNotifier{err: errors.New("push failed")}

	multi := notify.NewMultiNotifier(primary, discardLogger(), good, bad)

	uid := uuid.New()
	msg := notify.Outbound{RecipientUserID: &uid, Kind: notify.KindNotify, Body: "hi"}
	res, err := multi.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("multi send should not fail on extra errors: %v", err)
	}
	if res == nil || res.Body != "hi" {
		t.Fatalf("expected the primary message to be returned, got %+v", res)
	}
	if sink.created != 1 {
		t.Fatalf("primary should persist exactly once, got %d", sink.created)
	}
	if len(good.got) != 1 || len(bad.got) != 1 {
		t.Fatalf("both extras should be invoked, got good=%d bad=%d", len(good.got), len(bad.got))
	}
}
