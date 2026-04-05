# Features & Endpoint Reference

Complete reference for all API features, endpoints, and data models.

All endpoints are defined using Encore `//encore:api` annotations. Endpoints marked `auth` require a valid JWT from the configured auth provider (validated by the `//encore:authhandler` Gatekeeper). Endpoints marked `public` are unauthenticated.

## Authentication (`/api/v1/auth`)

Delegates credential management to the configured auth provider (Vault): Supabase Auth or Clerk in local/dev, Keycloak in production. The API acts as a proxy for auth operations via the `AuthProvider` interface.

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/auth/register` | No | Create a new user account |
| `POST` | `/auth/login` | No | Obtain access + refresh tokens |
| `POST` | `/auth/logout` | Yes | Revoke refresh token |
| `POST` | `/auth/refresh` | No | Exchange refresh token for new access token |
| `POST` | `/auth/forgot-password` | No | Trigger password reset email |
| `POST` | `/auth/reset-password` | No | Complete password reset |
| `POST` | `/auth/mfa/enroll` | Yes | Begin TOTP MFA enrollment |
| `POST` | `/auth/mfa/verify` | Yes | Confirm MFA enrollment |
| `DELETE` | `/auth/mfa` | Yes | Remove MFA from account |

### Register

```go
//encore:api public method=POST path=/api/v1/auth/register
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error)
```

Request:
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "name": "Jane Smith",
  "date_of_birth": "1990-06-15"
}
```

Response `201`:
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "name": "Jane Smith"
  }
}
```

### Login

```go
//encore:api public method=POST path=/api/v1/auth/login
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
```

Request:
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

Response `200`:
```json
{
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "dGhpcyBp...",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

If MFA is enrolled, returns `200` with `mfa_required: true` and a `mfa_session_token`. Client must then call `POST /auth/mfa/challenge`.

---

## Vitals (`/api/v1/vitals`)

Core feature. Records and retrieves time-series vital sign readings.

### Vital Types

| `vital_type` | Unit | Normal Range |
|-------------|------|-------------|
| `heart_rate` | `bpm` | 60–100 |
| `blood_pressure_systolic` | `mmHg` | 90–120 |
| `blood_pressure_diastolic` | `mmHg` | 60–80 |
| `spo2` | `%` | 95–100 |
| `temperature` | `degC` or `degF` | 36.1–37.2°C |
| `respiratory_rate` | `breaths_per_min` | 12–20 |
| `weight` | `kg` | — |
| `blood_glucose` | `mmol_L` or `mg_dL` | 4.0–7.8 mmol/L (fasting) |

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/vitals` | List vitals (paginated, filterable) |
| `POST` | `/vitals` | Record a new vital reading |
| `GET` | `/vitals/{id}` | Get a specific reading |
| `DELETE` | `/vitals/{id}` | Delete a reading |
| `GET` | `/vitals/summary` | Aggregated stats for a time range |
| `GET` | `/vitals/latest` | Most recent reading per vital type |

### List Vitals

```go
//encore:api auth method=GET path=/api/v1/vitals
func (s *VitalService) ListVitals(ctx context.Context, req *ListVitalsRequest) (*ListVitalsResponse, error)
```

```
GET /api/v1/vitals?vital_type=heart_rate&from=2026-01-01T00:00:00Z&to=2026-01-31T23:59:59Z&limit=20
```

Query Parameters:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `vital_type` | string | — | Filter by type (see table above) |
| `from` | ISO 8601 | 7 days ago | Start of time range |
| `to` | ISO 8601 | now | End of time range |
| `limit` | int | 20 | Records per page (max 100) |
| `before` | ISO 8601 | — | Cursor for next page |

Response `200`:
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "vital_type": "heart_rate",
      "value": 72.0,
      "unit": "bpm",
      "status": "normal",
      "device_id": null,
      "notes": null,
      "measured_at": "2026-01-15T10:30:00Z",
      "created_at": "2026-01-15T10:30:05Z"
    }
  ],
  "meta": {
    "limit": 20,
    "has_more": true,
    "next_cursor": "2026-01-15T09:00:00Z"
  }
}
```

### Record Vital

```go
//encore:api auth method=POST path=/api/v1/vitals
func (s *VitalService) RecordVital(ctx context.Context, req *RecordVitalRequest) (*VitalReadingResponse, error)
```

Request:
```json
{
  "vital_type": "heart_rate",
  "value": 72.0,
  "unit": "bpm",
  "device_id": "device-uuid",
  "notes": "After morning walk",
  "measured_at": "2026-01-15T10:30:00Z"
}
```

Response `201`: Returns created `VitalReading` object.

Status is computed automatically:
- `normal` — within user's threshold (or default range if no threshold set)
- `low` — below low threshold
- `high` — above high threshold
- `critical` — > 20% outside threshold (triggers immediate alert)

### Vital Summary

```go
//encore:api auth method=GET path=/api/v1/vitals/summary
func (s *VitalService) GetSummary(ctx context.Context, req *SummaryRequest) (*SummaryResponse, error)
```

```
GET /api/v1/vitals/summary?vital_type=heart_rate&from=2026-01-01T00:00:00Z&to=2026-01-31T23:59:59Z&bucket=1h
```

Response `200`:
```json
{
  "data": {
    "vital_type": "heart_rate",
    "from": "2026-01-01T00:00:00Z",
    "to": "2026-01-31T23:59:59Z",
    "overall": {
      "avg": 74.2,
      "min": 58.0,
      "max": 112.0,
      "count": 248
    },
    "buckets": [
      {
        "bucket": "2026-01-01T00:00:00Z",
        "avg": 71.5,
        "min": 62.0,
        "max": 88.0,
        "count": 8
      }
    ]
  }
}
```

---

## Alerts (`/api/v1/alerts`)

Alerts are generated automatically when a vital reading breaches a threshold. They can also be managed manually.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/alerts` | List alerts (filterable) |
| `GET` | `/alerts/{id}` | Get a specific alert |
| `POST` | `/alerts/{id}/acknowledge` | Acknowledge an alert |
| `GET` | `/alerts/thresholds` | List user's alert thresholds |
| `PUT` | `/alerts/thresholds/{vital_type}` | Set threshold for a vital type |
| `DELETE` | `/alerts/thresholds/{vital_type}` | Remove threshold (revert to defaults) |

### List Alerts

```
GET /api/v1/alerts?severity=critical&acknowledged=false&page=1&per_page=20
```

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `severity` | `warning\|critical` | — | Filter by severity |
| `acknowledged` | bool | — | Filter by acknowledgement status |
| `vital_type` | string | — | Filter by vital type |
| `from` | ISO 8601 | 30 days ago | Start date |
| `to` | ISO 8601 | now | End date |

### Alert Object

```json
{
  "id": "alert-uuid",
  "vital_type": "heart_rate",
  "value": 145.0,
  "threshold_breached": "high",
  "threshold_value": 110.0,
  "severity": "critical",
  "acknowledged": false,
  "acknowledged_at": null,
  "created_at": "2026-01-15T10:30:00Z"
}
```

### Set Threshold

```go
//encore:api auth method=PUT path=/api/v1/alerts/thresholds/:vital_type
func (s *AlertService) SetThreshold(ctx context.Context, vital_type string, req *SetThresholdRequest) (*ThresholdResponse, error)
```

Request:
```json
{
  "low_value": 50.0,
  "high_value": 110.0,
  "enabled": true
}
```

---

## Users (`/api/v1/users`)

User profile management. Identity is owned by the auth provider (Supabase Auth/Clerk in local/dev, Keycloak in production); profile data (DOB, avatar) is stored locally.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/users/me` | Get current user's profile |
| `PATCH` | `/users/me` | Update profile fields |
| `DELETE` | `/users/me` | Delete account |
| `GET` | `/users/me/devices` | List registered devices |
| `POST` | `/users/me/devices` | Register a device |
| `DELETE` | `/users/me/devices/{id}` | Remove a device |

### User Profile

```json
{
  "id": "user-uuid",
  "email": "user@example.com",
  "name": "Jane Smith",
  "date_of_birth": "1990-06-15",
  "avatar_url": "https://...",
  "created_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-15T10:00:00Z"
}
```

### Update Profile

```go
//encore:api auth method=PATCH path=/api/v1/users/me
func (s *UserService) UpdateProfile(ctx context.Context, req *UpdateProfileRequest) (*UserProfileResponse, error)
```

All fields are optional:
```json
{
  "name": "Jane M. Smith",
  "date_of_birth": "1990-06-15",
  "avatar_url": "https://..."
}
```

---

## WebSocket (`/ws`)

Real-time vital sign streaming for connected devices and live dashboard updates.

### Connection

```go
//encore:api public raw method=GET path=/ws
func (s *WebSocketService) HandleWebSocket(w http.ResponseWriter, r *http.Request)
```

```
GET /ws?token=<access_token>
Upgrade: websocket
```

WebSocket uses Encore's raw endpoint (`raw`) since it needs direct HTTP access for the upgrade. Authentication: access token passed as query param (WebSocket clients can't set headers).

### Message Format

All messages are JSON with a `type` discriminator:

```json
{ "type": "<message_type>", "payload": { ... } }
```

### Client → Server Messages

**Subscribe to vital type:**
```json
{
  "type": "subscribe",
  "payload": { "vital_type": "heart_rate" }
}
```

**Unsubscribe:**
```json
{
  "type": "unsubscribe",
  "payload": { "vital_type": "heart_rate" }
}
```

**Ping:**
```json
{ "type": "ping" }
```

### Server → Client Messages

**New vital reading:**
```json
{
  "type": "vital_reading",
  "payload": {
    "id": "reading-uuid",
    "vital_type": "heart_rate",
    "value": 74.0,
    "unit": "bpm",
    "status": "normal",
    "measured_at": "2026-01-15T10:30:00Z"
  }
}
```

**Alert triggered:**
```json
{
  "type": "alert",
  "payload": {
    "id": "alert-uuid",
    "vital_type": "heart_rate",
    "value": 145.0,
    "severity": "critical"
  }
}
```

**Pong:**
```json
{ "type": "pong" }
```

**Error:**
```json
{
  "type": "error",
  "payload": { "code": "INVALID_MESSAGE", "message": "Unknown message type" }
}
```

### WebSocket Limits

- Max connections per user: **5**
- Idle timeout: **60 seconds** (client must send ping every 30s)
- Max message size: **4 KB**
- Reconnection: client responsibility (exponential backoff recommended)

---

## Health (`/health`)

Internal health and readiness probes. Not versioned, not authenticated.

```go
//encore:api public method=GET path=/health/live
func (s *HealthService) Liveness(ctx context.Context) (*HealthResponse, error)

//encore:api public method=GET path=/health/ready
func (s *HealthService) Readiness(ctx context.Context) (*HealthResponse, error)
```

Readiness checks:
- PostgreSQL: `SELECT 1`
- Redis: `PING`
- Auth provider: fetch JWKS endpoint

```json
{
  "status": "ok",
  "checks": {
    "postgres": "ok",
    "redis": "ok",
    "auth_provider": "ok"
  },
  "version": "1.2.3",
  "uptime_seconds": 3600
}
```

Returns `200` if healthy, `503` if any check fails.
