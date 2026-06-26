# NightWatch — backend

A Life360-style group safety monitor with messaging and monitoring, written in
**Go** with **PostgreSQL**.

A `Group` of `User`s runs a `Night` — a monitored session with a center point,
a maximum range, battery thresholds, and check-in intervals. A background
scheduler (and an on-demand endpoint) runs `check()`, which evaluates every
participant's location, battery, and check-in freshness, records a status, and
dispatches alerts to flagged participants, their trusted contacts, and group
admins.

> Prototype scope: **no authentication** (clients are trusted), and messages are
> delivered by a `Notifier` that **persists them to the database and logs them**.
> A real SMS/push provider can be dropped in behind the same interface.

## Quick start

### Option A — Docker (everything)

```bash
docker compose up --build
# API on http://localhost:8080
```

### Option B — Local Go + Postgres in Docker

```bash
cp .env.example .env
docker compose up -d db          # Postgres on :5432
go run ./cmd/server              # migrations run automatically on startup
```

### Option C — Local Go + your own Postgres

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/nightwatch?sslmode=disable"
go run ./cmd/server
```

Health check: `curl localhost:8080/healthz`

## Configuration

All configuration is via environment variables (see `.env.example`):

| Variable                        | Default                              | Description                                   |
| ------------------------------- | ------------------------------------ | --------------------------------------------- |
| `HTTP_ADDR`                     | `:8080`                              | HTTP listen address                           |
| `DATABASE_URL`                  | `postgres://nightwatch:...@localhost`| Postgres connection string                    |
| `MONITOR_ENABLED`               | `true`                               | Enable the background `check()` scheduler     |
| `MONITOR_TICK`                  | `30s`                                | How often the scheduler scans for due nights  |
| `DEFAULT_LOW_BATTERY_THRESHOLD` | `20`                                 | Default low-battery percent for new nights    |
| `LOG_LEVEL`                     | `info`                               | `debug` \| `info` \| `warn` \| `error`        |
| `APNS_ENABLED`                  | `false`                              | Enable Apple push notifications               |
| `APNS_KEY_PATH`                 | —                                    | Path to the APNs `.p8` auth key               |
| `APNS_KEY_ID`                   | —                                    | Apple Key ID (`kid`)                          |
| `APNS_TEAM_ID`                  | —                                    | Apple Team ID (`iss`)                         |
| `APNS_TOPIC`                    | —                                    | App bundle ID (`apns-topic`)                  |
| `APNS_PRODUCTION`               | `false`                              | `true` = prod APNs host, `false` = sandbox    |

## Architecture

```
cmd/server            entrypoint: config, db, wiring, HTTP server, scheduler
internal/
  config              env-based configuration
  models              domain types (User, Group, Night, Agent, ...)
  db                  pgx pool + embedded migration runner
  store               Store interface + Postgres implementation
  notify              Notifier interface + DB/logging implementation
  service             business logic (incl. the check() monitoring loop)
  monitor             background scheduler that runs due checks
  api                 net/http router (Go 1.22 routing) + handlers
api/openapi.yaml      OpenAPI 3 specification
```

The `service` layer depends only on the `store.Store` and `notify.Notifier`
**interfaces**, so the core monitoring logic is unit-tested with an in-memory
store, a fake clock, and a capturing notifier — no database required.

## Model → API mapping

| Model operation                       | Endpoint                                   |
| ------------------------------------- | ------------------------------------------ |
| `User.formGroup()`                    | `POST /groups`                             |
| `User.joinGroup()` / `leaveGroup()`   | `POST /groups/{id}/join` / `DELETE /groups/{id}/members/{userId}` |
| `User.listGroups()`                   | `GET /users/{id}/groups`                   |
| `User.listGroupsWithActiveNight()`    | `GET /users/{id}/groups?activeNight=true`  |
| `User.getLocation()` / `setLocation()`| `GET` / `PUT /users/{id}/location`         |
| `User.getBatteryLevel()` / `set...()` | `GET` / `PUT /users/{id}/battery`          |
| register / list / remove push device  | `POST` / `GET /users/{id}/devices`, `DELETE /users/{id}/devices/{token}` |
| `Group.getAdmin()` / `setAdmin()`     | `GET` / `PUT /groups/{id}/admins`          |
| `Group.currNight`                     | `GET /groups/{id}/night`                   |
| create `Night`                        | `POST /groups/{id}/nights`                 |
| `Night.start()` / `end()` / `delete()`| `POST .../start`, `.../end`, `DELETE`      |
| `Night.setRange()`                    | `PUT /nights/{id}/range`                   |
| `Night.setCenter()`                   | `PUT /nights/{id}/center`                  |
| `Night.check()`                       | `POST /nights/{id}/check` (+ background)   |
| acknowledge a check-in                | `POST /nights/{id}/checkin/{userId}`       |
| `Night.message()`                     | `POST /nights/{id}/messages`               |
| `Night.notifyAll()`                   | `POST /nights/{id}/notify`                 |
| `Night.getLocationOf()`               | `GET /nights/{id}/locations/{userId}`      |
| `Night.getBatteryLevelOf()`           | `GET /nights/{id}/battery/{userId}`        |
| `Night.currentLocations`              | `GET /nights/{id}/locations`               |
| `Night.participantStatuses`           | `GET /nights/{id}/statuses`                |

See `api/openapi.yaml` for the full specification.

## The monitoring loop (`check()`)

For each participant of an active night, `check()` evaluates (in priority order):

1. **missing** — no location reported, or no location/check-in within
   `checkInLimitMin`.
2. **out_of_range** — farther than `maxRangeM` from the night's center
   (great-circle / haversine distance).
3. **low_battery** — battery below `lowBatteryThreshold`.
4. **ok** — otherwise.

A participant flagged `out_of_range` who acknowledges a check-in with `ok: true`
is moved to **out_of_range_safe** — they have confirmed they are safe despite
being outside the range. This holds for a 30-minute grace window (each fresh
`ok` check-in resets it), and the other group members and the participant's
`trustedContact` are notified of the update. If they are still out of range when
the window lapses they revert to `out_of_range` (re-alerting the guardians);
returning within range clears them back to `ok`.

Flagged participants are nudged directly each check. On a **transition** into a
new problem state (`out_of_range`, `low_battery`, `missing`) every **other**
group member — the guardians — is alerted, and `missing`/`out_of_range`
participants also trigger an alert to their `trustedContact`; unchanged statuses
are throttled to avoid repeated alert spam. A participant who is `ok` or
`out_of_range_safe` is not nudged. When `timeLimitMin` elapses since
`startedAt`, the night auto-ends.

A participant's freshness is the most recent of a reported location **or** an
explicit check-in acknowledgement (`POST /nights/{id}/checkin/{userId}` with
`ok: true`), so acknowledging a prompt resets the `missing` timer even without a
new location. Responding `ok: false` is treated as distress: the participant is
flagged and every other group member is alerted immediately.

The background scheduler runs `check()` for each active night no more often than
its `checkInEveryMin`, and also pushes each participant an "are you OK?" check-in
prompt on the same cadence. `POST /nights/{id}/check` triggers a check on demand.

## Notifications & push (APNs)

Messages are delivered through the `notify.Notifier` interface. By default a
`DBNotifier` persists every message to the `messages` table and logs it.

To also deliver **native iOS push notifications** (no Twilio / third party), set
`APNS_ENABLED=true` and provide an Apple `.p8` auth key. The server then composes
a `MultiNotifier` that persists to the DB **and** pushes to Apple:

- **Auth** is token-based — a short-lived **ES256 JWT** signed with your `.p8`
  key (`KeyID` + `TeamID`), implemented with the **standard library only**
  (`crypto/ecdsa` + `net/http`'s built-in HTTP/2). Tokens are cached and
  refreshed well within Apple's 1-hour limit.
- **Devices register their token** with `POST /users/{id}/devices`
  `{ "platform": "ios", "token": "<APNs device token>" }`. Pushes target a
  user's registered iOS devices.
- **Categories** — check-in prompts push with the `CHECKIN` category (the app
  shows an "I'm OK" action that calls `/nights/{id}/checkin/{userId}`) and alerts
  push with the `ALERT` category (tapping deep-links to the Guardian screen).
  Each push carries custom fields (`type`, `nightId`, `userId`/`affectedUserId`,
  `status`) so the app can route it.
- **Caveats**: push only reaches devices running your app. A `trustedContact`
  that is just a phone number can't receive a push — that path still needs SMS
  (it remains persisted/logged and is flagged). Android can be added later
  behind the same interface via FCM.

```bash
# Register a device, then alerts from check()/message()/notifyAll() push to it.
curl -s $BASE/users/$ALEX/devices -d '{"platform":"ios","token":"abc123..."}'
```

## Example flow

```bash
BASE=http://localhost:8080

# 1. Create two users (one with a trusted contact)
ALEX=$(curl -s $BASE/users -d '{"name":"Alex","trustedContact":"+15551112222"}' | jq -r .id)
SAM=$(curl -s  $BASE/users -d '{"name":"Sam"}' | jq -r .id)

# 2. Alex forms a group and Sam joins
GID=$(curl -s $BASE/groups -d "{\"name\":\"Night Crew\",\"creatorUserId\":\"$ALEX\"}" | jq -r .id)
curl -s $BASE/groups/$GID/join -d "{\"userId\":\"$SAM\"}"

# 3. Create a night centered downtown, 1km range, and start it
NID=$(curl -s $BASE/groups/$GID/nights \
  -d '{"center":{"lat":40.7128,"lng":-74.0060},"maxRangeM":1000,"checkInEveryMin":5}' | jq -r .id)
curl -s -X POST $BASE/nights/$NID/start

# 4. Participants report locations
curl -s -X PUT $BASE/nights/$NID/locations/$ALEX -d '{"lat":40.7129,"lng":-74.0061,"batteryLevel":90}'
curl -s -X PUT $BASE/nights/$NID/locations/$SAM  -d '{"lat":40.8000,"lng":-74.0000,"batteryLevel":15}'

# 5. Run a check — Sam is out of range and low battery
curl -s -X POST $BASE/nights/$NID/check | jq

# 6. Inspect statuses and dispatched messages
curl -s $BASE/nights/$NID/statuses | jq
curl -s $BASE/nights/$NID/messages | jq
```

## Development

```bash
go test ./...     # unit tests (no DB needed)
go vet ./...
gofmt -l .        # should print nothing
```
