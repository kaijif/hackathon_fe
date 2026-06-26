package notify

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/t-kaijifu/hackathon-be/internal/models"
)

const (
	apnsHostProduction = "https://api.push.apple.com"
	apnsHostSandbox    = "https://api.sandbox.push.apple.com"
	// Apple rejects provider tokens older than 1h; refresh comfortably before.
	apnsTokenTTL = 50 * time.Minute
)

// DeviceTokenLister resolves a user's registered device tokens. *store.PgStore
// satisfies this interface.
type DeviceTokenLister interface {
	ListDeviceTokensForUser(ctx context.Context, userID uuid.UUID) ([]models.DeviceToken, error)
}

// APNsConfig configures the APNs notifier.
type APNsConfig struct {
	KeyID      string // Apple Key ID (JWT "kid")
	TeamID     string // Apple Team ID (JWT "iss")
	Topic      string // app bundle ID (apns-topic header)
	Production bool   // true => api.push.apple.com, false => sandbox

	// Host and HTTPClient are optional overrides, primarily for testing. When
	// empty, sane production/sandbox defaults are used.
	Host       string
	HTTPClient *http.Client
}

// APNsNotifier pushes alerts to Apple devices over the APNs HTTP/2 API using
// token-based (JWT ES256) authentication. It implements Notifier but does not
// persist messages; compose it with a DBNotifier via MultiNotifier.
type APNsNotifier struct {
	cfg    APNsConfig
	host   string
	client *http.Client
	signer *apnsSigner
	tokens DeviceTokenLister
	log    *slog.Logger
}

// NewAPNsNotifier builds an APNsNotifier from an Apple .p8 auth key (PEM bytes).
func NewAPNsNotifier(cfg APNsConfig, authKeyPEM []byte, tokens DeviceTokenLister, log *slog.Logger) (*APNsNotifier, error) {
	if log == nil {
		log = slog.Default()
	}
	if cfg.KeyID == "" || cfg.TeamID == "" || cfg.Topic == "" {
		return nil, errors.New("apns: KeyID, TeamID and Topic are required")
	}
	key, err := parseP8Key(authKeyPEM)
	if err != nil {
		return nil, err
	}

	host := cfg.Host
	if host == "" {
		host = apnsHostSandbox
		if cfg.Production {
			host = apnsHostProduction
		}
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return &APNsNotifier{
		cfg:    cfg,
		host:   host,
		client: client,
		signer: &apnsSigner{keyID: cfg.KeyID, teamID: cfg.TeamID, key: key, now: time.Now},
		tokens: tokens,
		log:    log,
	}, nil
}

// Send pushes the message to all of the recipient user's iOS devices. Messages
// addressed only to a phone number (a trusted contact) cannot be pushed and are
// skipped.
func (a *APNsNotifier) Send(ctx context.Context, msg Outbound) (*models.Message, error) {
	if msg.RecipientUserID == nil {
		// Trusted-contact (phone) recipients cannot be pushed; this is expected.
		a.log.Debug("apns: skipping push for contact-only recipient",
			"kind", msg.Kind, "contact", msg.RecipientContact)
		return nil, nil
	}
	user := msg.RecipientUserID.String()

	tokens, err := a.tokens.ListDeviceTokensForUser(ctx, *msg.RecipientUserID)
	if err != nil {
		a.log.Error("apns: failed to load device tokens for user", "user", user, "error", err)
		return nil, err
	}
	if len(tokens) == 0 {
		a.log.Warn("apns: no device tokens registered for user; nothing to push",
			"user", user, "kind", msg.Kind)
		return nil, nil
	}

	jwt, err := a.signer.token()
	if err != nil {
		a.log.Error("apns: failed to sign provider JWT; check APNS_KEY_ID/APNS_TEAM_ID and the .p8 key",
			"user", user, "keyId", a.cfg.KeyID, "teamId", a.cfg.TeamID, "error", err)
		return nil, err
	}
	payload := buildAPNsPayload(msg)

	a.log.Info("apns: dispatching push",
		"user", user, "kind", msg.Kind, "category", msg.Category,
		"tokenCount", len(tokens), "host", a.host, "topic", a.cfg.Topic,
		"payloadBytes", len(payload))

	var firstErr error
	var pushed, skipped, failed int
	for _, dt := range tokens {
		if dt.Platform != "ios" {
			a.log.Warn("apns: skipping non-ios device token",
				"user", user, "platform", dt.Platform, "token", dt.Token)
			skipped++
			continue
		}
		if err := a.push(ctx, dt.Token, jwt, payload); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		pushed++
	}

	a.log.Info("apns: push dispatch complete",
		"user", user, "kind", msg.Kind,
		"pushed", pushed, "skipped", skipped, "failed", failed)
	return nil, firstErr
}

func (a *APNsNotifier) push(ctx context.Context, deviceToken, jwt string, payload []byte) error {
	url := a.host + "/3/device/" + deviceToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		a.log.Error("apns: failed to build push request", "url", url, "error", err)
		return err
	}
	req.Header.Set("authorization", "bearer "+jwt)
	req.Header.Set("apns-topic", a.cfg.Topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("content-type", "application/json")

	a.log.Debug("apns: sending push request",
		"host", a.host, "topic", a.cfg.Topic, "token", deviceToken, "payload", string(payload))

	start := time.Now()
	resp, err := a.client.Do(req)
	if err != nil {
		// Network/TLS/timeout failure: the request never reached Apple.
		a.log.Error("apns: transport error sending push to Apple",
			"host", a.host, "topic", a.cfg.Topic, "token", deviceToken,
			"latency", time.Since(start).String(), "error", err)
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	apnsID := resp.Header.Get("apns-id")
	latency := time.Since(start).String()

	if resp.StatusCode != http.StatusOK {
		reason := apnsReason(body)
		a.log.Warn("apns: Apple rejected push",
			"status", resp.StatusCode, "reason", reason, "apnsId", apnsID,
			"host", a.host, "topic", a.cfg.Topic, "token", deviceToken,
			"body", string(bytes.TrimSpace(body)), "latency", latency)
		return fmt.Errorf("apns status %d (reason %q): %s", resp.StatusCode, reason, bytes.TrimSpace(body))
	}

	a.log.Info("apns: Apple accepted push",
		"status", resp.StatusCode, "apnsId", apnsID,
		"token", deviceToken, "latency", latency)
	return nil
}

func apnsReason(body []byte) string {
	var errBody struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &errBody); err != nil {
		return ""
	}
	return errBody.Reason
}

func buildAPNsPayload(msg Outbound) []byte {
	title := msg.Sender
	if title == "" {
		title = "NightWatch"
	}
	aps := map[string]any{
		"alert": map[string]string{
			"title": title,
			"body":  msg.Body,
		},
		"sound": "default",
	}
	if msg.Category != "" {
		aps["category"] = msg.Category
	}
	payload := map[string]any{
		"aps":  aps,
		"kind": msg.Kind,
		"body": msg.Body,
	}
	// Merge caller-supplied custom fields (type, nightId, userId, status, ...)
	// without letting them clobber the reserved "aps" key.
	for k, v := range msg.Data {
		if k == "aps" {
			continue
		}
		payload[k] = v
	}
	b, _ := json.Marshal(payload)
	return b
}

// apnsSigner produces and caches APNs provider JWTs (ES256).
type apnsSigner struct {
	keyID  string
	teamID string
	key    *ecdsa.PrivateKey
	now    func() time.Time

	mu       sync.Mutex
	cached   string
	issuedAt time.Time
}

func (s *apnsSigner) token() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != "" && s.now().Sub(s.issuedAt) < apnsTokenTTL {
		return s.cached, nil
	}
	tok, err := s.sign()
	if err != nil {
		return "", err
	}
	s.cached = tok
	s.issuedAt = s.now()
	return tok, nil
}

func (s *apnsSigner) sign() (string, error) {
	header := map[string]string{"alg": "ES256", "kid": s.keyID, "typ": "JWT"}
	claims := map[string]any{"iss": s.teamID, "iat": s.now().Unix()}

	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64(hb) + "." + b64(cb)

	digest := sha256.Sum256([]byte(signingInput))
	r, sVal, err := ecdsa.Sign(rand.Reader, s.key, digest[:])
	if err != nil {
		return "", err
	}
	// JWS ES256 requires fixed 32-byte big-endian R and S.
	sig := make([]byte, 64)
	r.FillBytes(sig[0:32])
	sVal.FillBytes(sig[32:64])

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func b64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func parseP8Key(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("apns: invalid PEM auth key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns: parse auth key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apns: auth key is not an ECDSA private key")
	}
	return key, nil
}
