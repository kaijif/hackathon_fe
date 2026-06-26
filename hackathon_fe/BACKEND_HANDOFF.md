# Backend Handoff: NightWatch iOS Safety App

## 1. Overview

NightWatch's iOS frontend is now being wired to the real NightWatch API as the source of truth for users, groups, active nights, participant locations, battery, statuses, and push-driven safety flows. The backend must implement the existing `openapi.yaml` contract plus the small check-in acknowledgement endpoint below so the app can create/start a monitored Night, continuously report background location/battery, render the Guardian map from server state, and receive/respond to periodic safety notifications.

## 2. Existing endpoints the frontend now actively uses

### Users

| Method | Path | Purpose | When the app calls it |
|---|---|---|---|
| `POST` | `/users` | Create a user from `CreateUserRequest`; returns `User`. | On onboarding/profile creation. |
| `GET` | `/users` | List all users. | Admin/debug/member lookup flows if needed. |
| `GET` | `/users/{id}` | Fetch one `User`. | On app launch/profile refresh and member detail refresh. |
| `PUT` | `/users/{id}/location` | Persist latest user coordinates via `SetLocationRequest`; response `User`. | Frequently from background location updates; includes `batteryLevel` when available. |
| `GET` | `/users/{id}/location` | Fetch latest user `Location`. | To refresh current user's last known server-side location. |
| `PUT` | `/users/{id}/battery` | Persist latest battery level `{ "batteryLevel": integer }`; response `User`. | When battery changes or location update cannot include battery. |
| `GET` | `/users/{id}/battery` | Fetch latest battery level. | To refresh current user's last known battery. |
| `POST` | `/users/{id}/devices` | Register APNs/FCM token; returns `DeviceToken`. | After iOS receives an APNs device token and user id is known. |
| `GET` | `/users/{id}/devices` | List registered device tokens. | Optional diagnostics/account state. |
| `DELETE` | `/users/{id}/devices/{token}` | Unregister a device token. | On logout, token rotation cleanup, or notification opt-out. |
| `GET` | `/users/{id}/groups` | List groups a user belongs to; supports `activeNight=true`. | Main groups list and active-night discovery. |

### Groups

| Method | Path | Purpose | When the app calls it |
|---|---|---|---|
| `POST` | `/groups` | Create a group from `CreateGroupRequest`; creator becomes admin. | Create-group flow. |
| `GET` | `/groups` | List groups. | Optional discovery/debug flow. |
| `GET` | `/groups/{id}` | Fetch one `Group`. | Group detail refresh. |
| `GET` | `/groups/{id}/members` | List group `Member` records. | Group detail and Guardian roster. |
| `POST` | `/groups/{id}/join` | Join a group with `{ "userId": uuid }`. | QR/deep-link invite join flow. |
| `GET` | `/groups/{id}/night` | Get the group's current `Night`. | Entering group detail; decides Start Night vs Open Guardian. |
| `POST` | `/groups/{id}/nights` | Create a pending `Night` from `CreateNightRequest`. | Start Night form submission before calling `/start`. |

### Nights

| Method | Path | Purpose | When the app calls it |
|---|---|---|---|
| `GET` | `/nights/{id}` | Fetch `NightView` with `currentLocations` and `participantStatuses`. | Guardian screen polling/refresh. |
| `DELETE` | `/nights/{id}` | Delete a night. | Admin cleanup if exposed. |
| `POST` | `/nights/{id}/start` | Start a pending `Night`; returns `Night`. | Immediately after creating a night. |
| `POST` | `/nights/{id}/end` | End an active `Night`; returns `Night`. | End Night action and backend auto-end finalization. |
| `PUT` | `/nights/{id}/range` | Update `maxRangeM`; returns `Night`. | If admin edits allowed range after creation. |
| `POST` | `/nights/{id}/check` | Run monitoring loop; returns `CheckResult`. | App may trigger during polling; backend must also run automatically. |
| `GET` | `/nights/{id}/locations` | List all current `NightLocation` records. | Guardian map refresh. |
| `PUT` | `/nights/{id}/locations/{userId}` | Report participant location into a night via `SetLocationRequest`; returns `NightLocation`. | While active night is known; complements `/users/{id}/location`. |
| `GET` | `/nights/{id}/statuses` | List all `ParticipantStatus` records. | Guardian status list refresh. |
| `POST` | `/nights/{id}/notify` | Broadcast a notification/message to all participants; returns `Message[]`. | Admin/manual broadcast if exposed; also useful for backend alert plumbing. |

## 3. NEW endpoints the backend must add

### `POST /nights/{id}/checkin/{userId}`

**Tag:** Add under the OpenAPI `Nights` tag.

**Purpose:** Acknowledge a periodic "are you OK?" check-in for one participant. This resets that participant's check-in freshness/missing timer for the active night. If coordinates or battery are included, persist them to the user and active-night location/battery state. If `ok` is `false`, treat it as a distress signal: flag the participant, record an alert state/message, and notify every other group member.

**Request JSON body:**

| Field | Type | Required | Notes |
|---|---:|---:|---|
| `ok` | boolean | yes | `true` means the user acknowledged they are OK; `false` means distress. |
| `lat` | number | no | Optional latest latitude. If provided, `lng` must also be provided. |
| `lng` | number | no | Optional latest longitude. If provided, `lat` must also be provided. |
| `batteryLevel` | integer | no | Optional battery percent, 0-100. |

```json
{
  "ok": true,
  "lat": 37.7749,
  "lng": -122.4194,
  "batteryLevel": 64
}
```

**Success response option A (preferred): `200 OK` with updated `ParticipantStatus`:**

```json
{
  "nightId": "11111111-1111-1111-1111-111111111111",
  "userId": "22222222-2222-2222-2222-222222222222",
  "status": "ok",
  "detail": "Checked in at 2026-06-24T04:35:00Z",
  "distanceM": 82.4,
  "updatedAt": "2026-06-24T04:35:00Z"
}
```

**Success response option B:** `204 No Content` after state is updated. If using 204, the frontend will refresh via `GET /nights/{id}/statuses` or `GET /nights/{id}`.

**Distress behavior (`ok: false`):** return `200 OK` with an updated `ParticipantStatus`. Because the existing enum has no `distress`, use `missing` or `unknown` with a clear `detail`, and send group alerts immediately.

```json
{
  "nightId": "11111111-1111-1111-1111-111111111111",
  "userId": "22222222-2222-2222-2222-222222222222",
  "status": "missing",
  "detail": "User responded not OK; distress alert dispatched",
  "distanceM": 82.4,
  "updatedAt": "2026-06-24T04:35:00Z"
}
```

**Error cases:**

| Status | When |
|---:|---|
| `400` | Invalid JSON; missing `ok`; only one of `lat`/`lng` provided; invalid coordinates; `batteryLevel` outside 0-100. |
| `404` | Night id or user id does not exist, or user is not a participant in the night. |
| `409` | Night is not `active` or has already ended. |

**Minimal OpenAPI schema suggestion:**

```yaml
/nights/{id}/checkin/{userId}:
  parameters:
    - $ref: "#/components/parameters/IdParam"
    - $ref: "#/components/parameters/UserIdParam"
  post:
    tags: [Nights]
    summary: Acknowledge a participant check-in
    requestBody:
      required: true
      content:
        application/json:
          schema:
            type: object
            required: [ok]
            properties:
              ok: { type: boolean }
              lat: { type: number, format: double }
              lng: { type: number, format: double }
              batteryLevel: { type: integer, minimum: 0, maximum: 100 }
    responses:
      "200":
        description: Updated participant status
        content:
          application/json:
            schema: { $ref: "#/components/schemas/ParticipantStatus" }
      "204": { description: Check-in accepted }
      "400": { $ref: "#/components/responses/BadRequest" }
      "404": { $ref: "#/components/responses/NotFound" }
      "409": { $ref: "#/components/responses/Conflict" }
```

No other new endpoint is required for the described iOS behavior; everything else maps to existing `openapi.yaml` paths.

## 4. Required server-side behaviors / business logic

### Location and battery persistence/propagation

- `PUT /users/{id}/location` must persist `lat`, `lng`, optional `batteryLevel`, and update `locationUpdatedAt`/`updatedAt` on the `User`.
- `PUT /users/{id}/battery` must persist `batteryLevel` and update `updatedAt` on the `User`.
- Whenever user location or battery changes, propagate the latest values into every `active` Night the user participates in so `GET /nights/{id}/locations` and `GET /nights/{id}` reflect current server truth.
- `PUT /nights/{id}/locations/{userId}` must also update the participant's `NightLocation` and should keep the corresponding `User.lat`, `User.lng`, `User.batteryLevel`, and `User.locationUpdatedAt` in sync.

### Device-token registry and push dispatch

- `POST /users/{id}/devices` stores or upserts `{ "platform": "ios", "token": "<APNs token>" }` and returns a `DeviceToken`.
- The iOS app uses APNs tokens. The schema allows `android`, but Android/FCM is optional for this handoff.
- Send push notifications to every registered device token for the target user. Remove or mark invalid tokens when APNs reports permanent invalidation.
- Alerts should also be recorded as `Message` rows where useful, using `kind: "alert"` or `kind: "notify"` according to existing schema.

### Monitoring loop

- `POST /nights/{id}/check` must run the same logic as the automatic scheduler and return `CheckResult`.
- The backend must also run this loop automatically for each active Night at least every `checkInEveryMin` minutes.
- For each participant:
  1. Compute distance in meters from `Night.centerLat`/`Night.centerLng` to the participant's latest coordinates.
  2. Set `ParticipantStatus.status` to `out_of_range` if `distanceM > maxRangeM`.
  3. Else set `low_battery` if latest `batteryLevel < lowBatteryThreshold`.
  4. Else set `missing` if there is no fresh location/check-in within `checkInLimitMin` minutes.
  5. Else set `ok`.
  6. Use `unknown` only when required data is unavailable and the participant cannot be classified.
- When a participant transitions into `out_of_range`, `low_battery`, or `missing`, send an alert push to EVERY OTHER member of the group, not to the affected user as a group alert recipient.
- Optionally message/SMS the affected user's `trustedContact` when the affected user is `out_of_range`, `low_battery`, `missing`, or responded with `ok: false`.
- Avoid duplicate alert spam: notify on status transition and then throttle repeated alerts for the same participant/status.
- Auto-end the Night after `timeLimitMin` from `startedAt`; set `status: "ended"`, `endedAt`, clear/reflect the group's `currNightId`/active state, and return `ended: true` from the check that ends it.

### Periodic check-in push scheduling

- While a Night is `active`, send each participant an "are you OK?" push every `checkInEveryMin` minutes.
- A successful `POST /nights/{id}/checkin/{userId}` with `ok: true` resets that participant's check-in freshness timer.
- Fresh background location updates may also count as freshness, but check-in acknowledgement should be the explicit signal for notification responses.
- If no `/checkin` call and no fresh location arrive within `checkInLimitMin`, set the participant to `missing` and alert every other group member.

### Push notification payload conventions

The backend should use payloads compatible with iOS notification categories/actions. Recommended custom JSON fields:

**Check-in push** — APNs category `CHECKIN`; app shows an "I'm OK" action that calls `/nights/{id}/checkin/{userId}`.

```json
{
  "aps": {
    "alert": {
      "title": "NightWatch check-in",
      "body": "Are you OK?"
    },
    "category": "CHECKIN",
    "sound": "default"
  },
  "type": "checkin",
  "nightId": "11111111-1111-1111-1111-111111111111",
  "userId": "22222222-2222-2222-2222-222222222222",
  "body": "Are you OK?"
}
```

**Alert push** — APNs category `ALERT`; tapping deep-links to the Guardian screen.

```json
{
  "aps": {
    "alert": {
      "title": "NightWatch alert",
      "body": "Alex is out of range."
    },
    "category": "ALERT",
    "sound": "default"
  },
  "type": "alert",
  "nightId": "11111111-1111-1111-1111-111111111111",
  "affectedUserId": "22222222-2222-2222-2222-222222222222",
  "status": "out_of_range",
  "body": "Alex is out of range."
}
```

These conventions are recommendations so the iOS notification categories/actions line up with backend pushes.

## 5. Data the app sends/expects per call

- Location updates use `SetLocationRequest` exactly: `lat` number, `lng` number, optional `batteryLevel` integer 0-100.

```json
{
  "lat": 37.7749,
  "lng": -122.4194,
  "batteryLevel": 64
}
```

- Battery-only updates use:

```json
{
  "batteryLevel": 64
}
```

- Device registration uses APNs token data:

```json
{
  "platform": "ios",
  "token": "<APNs token>"
}
```

- Night creation uses `CreateNightRequest`; the app sets `center` from the starter's current location and user-configured thresholds:

```json
{
  "agentId": null,
  "center": {
    "lat": 37.7749,
    "lng": -122.4194
  },
  "timeLimitMin": 480,
  "checkInLimitMin": 30,
  "checkInEveryMin": 15,
  "maxRangeM": 1000,
  "lowBatteryThreshold": 20
}
```

- Guardian screen expects `NightView.currentLocations[]` as `NightLocation` records and `NightView.participantStatuses[]` as `ParticipantStatus` records. Status enum values must remain exactly: `ok`, `out_of_range`, `out_of_range_safe`, `low_battery`, `missing`, `unknown`.
- `GET /nights/{id}/locations` should include latest `lat`, `lng`, optional `batteryLevel`, and `reportedAt` for each participant with known data.
- `GET /nights/{id}/statuses` should include `detail`, `distanceM`, and `updatedAt` so the app can display status reason and last-seen information.

## 6. Open questions / assumptions

- Distance is computed with the haversine formula or equivalent geodesic calculation and returned in meters as `distanceM`.
- Battery values are integer percentages from 0 through 100; `batteryLevel < lowBatteryThreshold` triggers `low_battery`.
- `trustedContact` is an optional phone number string stored on `User`; SMS/phone notification is optional but should be attempted if configured.
- The prototype has no authentication per `openapi.yaml`; clients are trusted, but backend should still validate user membership for night-specific calls.
- If multiple problem conditions are true, priority is `out_of_range`, then `low_battery`, then `missing`, then `ok`, matching the monitoring loop above unless backend chooses to encode additional detail text.
- Server timestamps are ISO 8601 date-time strings in UTC.
