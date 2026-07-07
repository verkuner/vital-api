# Database schema

Storage design for the [backend API](api.md). Two concerns, one Postgres cluster:

- **PostgreSQL (relational)** — accounts, devices, links/cohorts, sharing, alerts,
  and the low-frequency *records* (sleep sessions, daily activity, ECG metadata,
  workouts, OTA jobs).
- **TimescaleDB** (a Postgres extension) — the high-frequency **metric readings**
  streamed/measured from rings, as hypertables with continuous aggregates that
  serve the day/week/month charts and the Avg/Min/Max summaries directly.

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS timescaledb;
```

## Enum types

```sql
CREATE TYPE user_role      AS ENUM ('wearer','family','care_staff');
CREATE TYPE device_backend AS ENUM ('mock','tk30','hr216','yuchen');
CREATE TYPE metric_type    AS ENUM ('heart_rate','spo2','temperature','blood_pressure',
                                    'blood_glucose','hrv','stress','respiratory');
CREATE TYPE reading_source AS ENUM ('stream','measurement','history');
CREATE TYPE sleep_stage    AS ENUM ('awake','rem','light','deep');
CREATE TYPE health_status  AS ENUM ('normal','warning','critical','offline');
CREATE TYPE link_status    AS ENUM ('pending','active','revoked');
```
> `sleep`, `activity` and `ecg` are their own record tables, so `metric_type`
> (the TimescaleDB scalar streams) deliberately excludes them.

---

## PostgreSQL — relational tables

### Accounts & auth
```sql
CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  email         citext UNIQUE,
  phone         text UNIQUE,
  name          text NOT NULL,
  password_hash text,                       -- null for OTP-only accounts
  date_of_birth date,
  locale        text NOT NULL DEFAULT 'en',
  theme         jsonb NOT NULL DEFAULT '{"mode":"system","palette":"blue"}',
  active_role   user_role NOT NULL DEFAULT 'wearer',
  active_device_id uuid,                     -- FK added after devices (see below)
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

-- A user may hold more than one role (e.g. wearer + family caregiver).
CREATE TABLE user_roles (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role    user_role NOT NULL,
  PRIMARY KEY (user_id, role)
);

CREATE TABLE refresh_tokens (
  id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash text NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE otp_codes (
  id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  identifier text NOT NULL,                  -- email or phone
  code_hash  text NOT NULL,
  purpose    text NOT NULL,                  -- 'verify' | 'login' | 'reset'
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON otp_codes (identifier, purpose);
```

### Devices
```sql
CREATE TABLE devices (
  id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  owner_user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  backend         device_backend NOT NULL,
  name            text NOT NULL,
  serial          text,
  ble_id          text,
  firmware        text,
  battery_percent int CHECK (battery_percent BETWEEN 0 AND 100),
  features        metric_type[] NOT NULL DEFAULT '{}',   -- capability-driven UI
  has_ecg         boolean NOT NULL DEFAULT false,
  has_sleep       boolean NOT NULL DEFAULT false,
  has_activity    boolean NOT NULL DEFAULT false,
  settings        jsonb NOT NULL DEFAULT '{}',            -- intervals, sleep schedule, LED, alarms
  last_sync_at    timestamptz,
  paired_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON devices (owner_user_id);

ALTER TABLE users
  ADD CONSTRAINT users_active_device_fk
  FOREIGN KEY (active_device_id) REFERENCES devices(id) ON DELETE SET NULL;

CREATE TABLE ota_jobs (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  device_id    uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  from_version text, to_version text,
  status       text NOT NULL DEFAULT 'queued',  -- queued|uploading|installing|done|failed
  progress     int NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
```

### Relationships — family links & care cohort
```sql
-- Family: a viewer user follows a subject user, read-only, per-metric permissions.
CREATE TABLE family_links (
  id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  viewer_user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  subject_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  relationship    text,                        -- 'father','mother',…
  status          link_status NOT NULL DEFAULT 'pending',
  permissions     text[] NOT NULL DEFAULT '{vitals,sleep,activity,alerts}',
  invite_code     text UNIQUE,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (viewer_user_id, subject_user_id)
);

-- Care: an org, its staff, and its patients (a patient may or may not be an app user).
CREATE TABLE care_orgs (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  name text NOT NULL
);
CREATE TABLE care_staff (
  care_org_id uuid NOT NULL REFERENCES care_orgs(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (care_org_id, user_id)
);
CREATE TABLE patients (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  care_org_id  uuid NOT NULL REFERENCES care_orgs(id) ON DELETE CASCADE,
  user_id      uuid REFERENCES users(id) ON DELETE SET NULL,  -- links to a real wearer if present
  name         text NOT NULL,
  room         text,
  age          int,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON patients (care_org_id);
```

### Sharing
```sql
CREATE TABLE sharing_settings (
  user_id     uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  -- per-metric visibility to linked viewers / care, e.g. {"heart_rate":"family","location":"care"}
  metric_scopes jsonb NOT NULL DEFAULT '{}',
  updated_at  timestamptz NOT NULL DEFAULT now()
);
```

### Alerts
```sql
CREATE TABLE alerts (
  id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  subject_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id       uuid REFERENCES devices(id) ON DELETE SET NULL,
  type            text NOT NULL,               -- 'high_heart_rate','low_spo2','fall',…
  severity        health_status NOT NULL,      -- warning|critical
  metric_type     metric_type,
  value           double precision,
  message         text NOT NULL,
  acknowledged    boolean NOT NULL DEFAULT false,
  acknowledged_by uuid REFERENCES users(id),
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON alerts (subject_user_id, created_at DESC);
CREATE INDEX ON alerts (subject_user_id) WHERE NOT acknowledged;
```

### Records — sleep, activity, ECG, workouts (low frequency)
```sql
CREATE TABLE sleep_sessions (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id    uuid REFERENCES devices(id) ON DELETE SET NULL,
  date         date NOT NULL,                  -- the night's date
  total_hours  numeric(4,2) NOT NULL,
  score        int,
  deep_hours   numeric(4,2), rem_hours numeric(4,2),
  light_hours  numeric(4,2), awake_hours numeric(4,2),
  segments     jsonb NOT NULL DEFAULT '[]',    -- [{stage,start_hour,duration_hours}]
  source       reading_source NOT NULL DEFAULT 'history',
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, device_id, date)
);
CREATE INDEX ON sleep_sessions (user_id, date DESC);

CREATE TABLE activity_daily (
  id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id      uuid REFERENCES devices(id) ON DELETE SET NULL,
  date           date NOT NULL,
  steps          int NOT NULL,
  goal_steps     int NOT NULL DEFAULT 8000,
  calories       int, distance_km numeric(5,2), active_minutes int,
  source         reading_source NOT NULL DEFAULT 'history',
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, device_id, date)
);
CREATE INDEX ON activity_daily (user_id, date DESC);

CREATE TABLE ecg_sessions (
  id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id      uuid REFERENCES devices(id) ON DELETE SET NULL,
  recorded_at    timestamptz NOT NULL,
  sample_rate_hz int NOT NULL DEFAULT 250,
  duration_s     int NOT NULL,
  heart_rate     int, hrv int, systolic int, diastolic int,
  verdict        text,                          -- 'sinus_rhythm','afib_suspected',…
  waveform_uri   text,                          -- compressed samples in object storage
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON ecg_sessions (user_id, recorded_at DESC);

CREATE TABLE workouts (
  id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id    uuid REFERENCES devices(id) ON DELETE SET NULL,
  type         text NOT NULL,                   -- 'walk','run','cycle',…
  started_at   timestamptz NOT NULL, ended_at timestamptz,
  duration_s   int, avg_hr int, calories int, distance_km numeric(5,2),
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON workouts (user_id, started_at DESC);
```
> **ECG waveform:** thousands of samples per recording → don't row-store them.
> Keep metadata here and the compressed sample array in object storage
> (`waveform_uri`). The app's `ecg_analysis` derives HR/HRV/verdict at ingest.

---

## TimescaleDB — metric readings (high frequency)

Every scalar vital (streamed or measured) lands in one hypertable. This is the
source for `/vitals/*/series`, `/vitals/*/latest`, and the Summary tiles.

```sql
CREATE TABLE metric_readings (
  time            timestamptz      NOT NULL,
  user_id         uuid             NOT NULL,
  device_id       uuid             NOT NULL,
  metric_type     metric_type      NOT NULL,
  value           double precision NOT NULL,   -- systolic for blood_pressure
  value_secondary double precision,            -- diastolic for blood_pressure
  source          reading_source   NOT NULL DEFAULT 'stream'
);

SELECT create_hypertable('metric_readings', 'time', chunk_time_interval => INTERVAL '7 days');

-- Series/summary always filter by (user, device, metric, time-range):
CREATE INDEX ON metric_readings (user_id, metric_type, time DESC);
CREATE INDEX ON metric_readings (device_id, metric_type, time DESC);
```
> `blood_pressure` is one row per measurement with `value`=systolic,
> `value_secondary`=diastolic — no second metric type needed.

### Continuous aggregates → the day/week/month views

**Hourly** rollup powers the **Day** chart (24 points):
```sql
CREATE MATERIALIZED VIEW metric_hourly
WITH (timescaledb.continuous) AS
SELECT user_id, device_id, metric_type,
       time_bucket('1 hour', time) AS bucket,
       avg(value) AS avg, min(value) AS min, max(value) AS max,
       avg(value_secondary) AS avg2, min(value_secondary) AS min2, max(value_secondary) AS max2,
       count(*) AS n
FROM metric_readings
GROUP BY user_id, device_id, metric_type, bucket;

SELECT add_continuous_aggregate_policy('metric_hourly',
  start_offset => INTERVAL '3 days', end_offset => INTERVAL '1 hour',
  schedule_interval => INTERVAL '30 minutes');
```

**Daily** rollup powers the **Week** (7 points) and **Month** (~30 points) charts
and the Avg/Min/Max Summary:
```sql
CREATE MATERIALIZED VIEW metric_daily
WITH (timescaledb.continuous) AS
SELECT user_id, device_id, metric_type,
       time_bucket('1 day', time) AS bucket,
       avg(value) AS avg, min(value) AS min, max(value) AS max,
       avg(value_secondary) AS avg2, min(value_secondary) AS min2, max(value_secondary) AS max2,
       count(*) AS n
FROM metric_readings
GROUP BY user_id, device_id, metric_type, bucket;

SELECT add_continuous_aggregate_policy('metric_daily',
  start_offset => INTERVAL '90 days', end_offset => INTERVAL '1 hour',
  schedule_interval => INTERVAL '1 hour');
```

`GET /vitals/heart_rate/series?granularity=day&from=…&to=…` becomes a plain
`SELECT bucket, avg, min, max FROM metric_daily WHERE user_id=… AND metric_type='heart_rate' AND bucket BETWEEN … ORDER BY bucket`,
with the enclosing summary from `min(min), max(max), avg(avg)`.

### Retention & compression
```sql
-- Compress raw readings older than 7 days (keep them queryable, cheaply).
ALTER TABLE metric_readings SET (timescaledb.compress,
  timescaledb.compress_segmentby = 'user_id, metric_type');
SELECT add_compression_policy('metric_readings', INTERVAL '7 days');

-- Drop raw readings after 180 days; the continuous aggregates live on.
SELECT add_retention_policy('metric_readings', INTERVAL '180 days');
```

---

## App model → storage map

| App concept | Storage |
|-------------|---------|
| `AppUser`, roles, theme/locale | `users`, `user_roles` |
| `RingPeripheral` / `RingDevice`, active device, features | `devices`, `users.active_device_id` |
| Live/measured scalar vitals (`Reading`, `MetricType`) | `metric_readings` (hypertable) |
| Today's vitals grid / Summary tiles | `metric_hourly` / `metric_daily` aggregates |
| `SleepSession` + stages | `sleep_sessions` (segments jsonb) |
| `Activity` | `activity_daily` |
| `EcgSession` | `ecg_sessions` + object storage waveform |
| `WorkoutSession` | `workouts` |
| `AlertEvent` | `alerts` |
| Family `LinkedPerson` | `family_links` |
| Care `Patient` / cohort | `care_orgs`, `care_staff`, `patients` |
| Sharing settings | `sharing_settings` |
| OTA | `ota_jobs` |

## Notes
- **Authorization** is enforced in the API layer against `family_links` (viewer↔subject) and `care_staff`/`patients` (staff↔cohort); optionally hardened with Postgres row-level security keyed on the JWT `user_id`.
- **Per-device scoping** (the app's `me@<deviceId>`) is native here: every reading/record carries `device_id`, so queries can scope to one ring or aggregate across all of a user's devices.
- **Idempotent ingestion:** a unique `(user_id, device_id, date)` on sleep/activity and an `Idempotency-Key`-backed dedupe on `metric_readings` let a ring safely re-sync overlapping windows.

---

## Adjusting the schema for Keycloak identity + Postgres authorization

The chosen split: **Keycloak handles identity only; Postgres is the single source
of truth for all authorization** (both coarse roles and fine-grained
relationships). Keycloak is the production "Vault" in this project — see
[CLAUDE.md](../CLAUDE.md) and [security.md](security.md).

- **What Keycloak owns** — credentials, login/OTP/MFA, sessions & token rotation,
  and JWT issuance. The token carries **identity only** (`sub`, `email`, `exp`).
- **What Postgres owns** — a local **projection** of each user, all domain data,
  *and* every authorization decision: role membership (`user_roles`) and the
  fine-grained, relationship-based permissions (`family_links`, `sharing_settings`,
  care cohorts).

**Why not put coarse roles in Keycloak too?** Because the fine-grained,
relationship-based permissions *must* live in Postgres regardless (Keycloak's RBAC
can't express "viewer A sees subject B's `heart_rate` but not `location`" across
thousands of changing edges). Splitting authorization across two systems means
every "can this user do X?" decision has to consult both the token and the DB.
Consolidating it in Postgres gives **one source of truth, one mental model**: a
permission change is a SQL `UPDATE` that takes effect immediately — no Keycloak
admin-API call, no group→role mapping, no waiting for a token refresh.

**The trade-off:** the JWT no longer self-describes what the user may do, so each
request resolves roles/permissions from Postgres rather than from token claims.
That's the same round-trip you already make to load the local user row; cache the
resolved set in **Redis** with a short TTL (this repo already uses Redis for
session cache) and invalidate on permission change for instant revocation.

The guiding principle: **the token proves *who* you are; Postgres decides *what*
you can do.** Keep a local user row so foreign keys, joins, and ownership checks
all work.

### 1. `users` becomes a projection keyed by the Keycloak `sub`

Keep a local `uuid` PK so every existing FK (`devices.owner_user_id`,
`metric_readings.user_id`, `alerts.acknowledged_by`, …) is untouched. Add the
external identity and demote authentication fields.

```sql
CREATE TABLE users (
  id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  keycloak_id    text NOT NULL UNIQUE,          -- JWT `sub` — the external identity
  -- Profile fields below are a CACHE synced from Keycloak claims on login,
  -- not the source of truth. Safe for display/joins; write-through to Keycloak.
  email          citext UNIQUE,
  phone          text UNIQUE,
  name           text NOT NULL,
  date_of_birth  date,
  -- App-only preferences Keycloak has no opinion on — these stay authoritative here:
  locale         text NOT NULL DEFAULT 'en',
  theme          jsonb NOT NULL DEFAULT '{"mode":"system","palette":"blue"}',
  active_role    user_role NOT NULL DEFAULT 'wearer',   -- UI's selected role; validated against user_roles
  active_device_id uuid,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON users (keycloak_id);
```

- **Drop `password_hash`.** Keycloak stores credentials; the API never sees them.
- **`email`/`phone`/`name` stay** but as a mirror. Sync them from token claims (or
  the Keycloak Admin API) on each login / JIT-provision. Keep the `UNIQUE`
  constraints only if you're comfortable they can transiently lag Keycloak.
- **JIT provisioning:** on the first authenticated request, upsert by
  `keycloak_id` (`INSERT … ON CONFLICT (keycloak_id) DO UPDATE`). This matches the
  existing `GET /users/me` auto-provision flow in this repo.

### 2. Drop the tables Keycloak replaces

```sql
DROP TABLE refresh_tokens;   -- Keycloak issues & rotates refresh tokens
DROP TABLE otp_codes;        -- Keycloak handles email/SMS OTP, verify & reset flows
```

Both `refresh_tokens` and `otp_codes` exist only to hand-roll auth. Keycloak's
sessions, offline tokens, "Verify Email", and "Forgot Password" flows cover all
of it. If you need server-side forced logout, call Keycloak's admin
`logout`/session-revocation endpoints instead of a local token table.

### 3. Roles stay in Postgres, authoritative

`user_role` / `user_roles` model *what a user is allowed to be*. Under this design
they are **not** synced from Keycloak — Postgres is authoritative, so the tables
stay exactly as in the base schema:

```sql
-- Unchanged. This is the source of truth for role membership.
CREATE TABLE user_roles (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role    user_role NOT NULL,
  PRIMARY KEY (user_id, role)
);
```

Request-time flow:

1. Auth handler validates the JWT signature/claims → gets `sub` → resolves the
   local `users` row by `keycloak_id`.
2. It loads that user's roles from `user_roles` (and, where relevant, the
   relationship permissions) into `AuthData`. Cache the resolved set in Redis
   keyed on `user_id` with a short TTL; invalidate the key when roles change.
3. `users.active_role` is the UI's *selected* role — validated against the user's
   `user_roles` set, never trusted from a token claim.

Granting or revoking a role is a plain `INSERT`/`DELETE` on `user_roles` that
takes effect on the next request (or immediately, if you bust the Redis key). No
Keycloak round-trip, no token refresh.

> Keycloak still authenticates these users; it just doesn't carry their roles.
> If you later need coarse gating *at the edge* (e.g. an API gateway that only
> understands JWT scopes), you can additionally mint a `care_staff` realm role in
> Keycloak — but keep `user_roles` as the authority the app enforces against.

### 4. Keep relationship-based permissions in Postgres — Keycloak can't model them

This is the key decision. Keycloak does **coarse RBAC** ("this user is
`care_staff`"). It does **not** naturally express *"viewer A may read subject B's
`heart_rate` and `sleep` but not `location`"* across arbitrary end-user pairs —
that's relationship-/resource-based access control, and there can be thousands of
edges that change constantly.

So **leave these exactly as they are:**

- `family_links` (viewer↔subject, `permissions text[]`, `status`)
- `care_orgs`, `care_staff`, `patients`
- `sharing_settings` (`metric_scopes` jsonb)

The API keeps enforcing them (the "Notes" above still hold). Roles from
`user_roles` gate *which endpoints* a user can call; these tables gate *which
subjects' rows* they can see — both resolved from Postgres against the `user_id`
behind the token's `sub`. Optional hardening: Postgres RLS keyed on that
`user_id`.

> Keycloak **Authorization Services (UMA 2.0)** *can* model resource-level
> sharing, but pushing per-user, per-metric sharing edges into Keycloak scales
> poorly and couples domain logic to the IdP. Keep sharing in the app DB.

### 5. Nothing changes for devices, records, alerts, or TimescaleDB

Every FK targets `users.id`, which we preserved. `devices`, `ota_jobs`,
`sleep_sessions`, `activity_daily`, `ecg_sessions`, `workouts`, `alerts`, and the
entire `metric_readings` hypertable + continuous aggregates are **unaffected**.

### 6. Multi-client: web + mobile

The API is a stateless OIDC **resource server** — it validates the Bearer JWT
against Keycloak's JWKS and maps `sub → users.keycloak_id`, agnostic to how the
user logged in. **The schema needs no changes to support both web and mobile:**
one human = one `keycloak_id` = one `users` row, whether they sign in from a
browser, a phone, or both at once. We dropped `refresh_tokens`/`otp_codes`, so
there's no per-session state to collide across concurrent sessions (Keycloak
tracks them), and `active_device_id` refers to the **wearable ring**, not the
login client — so a web and a mobile session never contend for it.

The web-vs-mobile differences live in Keycloak config and the frontends, not here:

**Two Keycloak clients, both public, Authorization Code + PKCE:**

| Client | Redirect URI | Notes |
|--------|--------------|-------|
| `vital-web` | `https://app.example.com/callback` | Browser SPA; CORS applies |
| `vital-mobile` | `vital://callback` (custom scheme / app link) | Native app; no CORS |

Separate clients let each set its own redirect URIs and token lifetimes. The
auth handler accepts any valid token from the realm; inspect the `azp`/`aud`
claim only if you want to *restrict* which client may call a given endpoint.

**Token transport.** Default to `Authorization: Bearer <token>` for **both**
clients — symmetric, and the auth handler already expects it. Only if the web
threat model demands XSS-proof storage, move web to a BFF/httpOnly-cookie pattern;
that would require the auth handler to also read the token from a cookie and add
CSRF protection (mobile is unaffected).

**CORS** applies to `vital-web` only — covered by `CORS_ALLOWED_ORIGINS`
(see [CLAUDE.md](../CLAUDE.md)). Mobile never triggers CORS.

**WebSocket.** Browsers can't set an `Authorization` header on the WS handshake,
so web clients pass the token via a query param or the `Sec-WebSocket-Protocol`
subprotocol; mobile WS libraries can send the header directly. The WS upgrade
handler should accept both.

### Summary of changes

Keycloak = identity; Postgres = all authorization.

| Table / column | Under this design | Why |
|----------------|-------------------|-----|
| `users.id` (uuid PK) | **Keep** | Preserves all FKs and joins |
| `users.keycloak_id` | **Add** (`UNIQUE NOT NULL`) | Maps JWT `sub` → local row |
| `users.password_hash` | **Remove** | Keycloak owns credentials |
| `users.email/phone/name` | **Keep as synced cache** | Display/joins; identity synced from Keycloak |
| `users.locale/theme/active_device_id` | **Keep** | App-only preferences |
| `users.active_role` | **Keep** as UI preference | Validated against `user_roles` |
| `user_roles` | **Keep, authoritative** | Postgres is the source of truth for roles |
| `refresh_tokens` | **Drop** | Keycloak issues & rotates tokens |
| `otp_codes` | **Drop** | Keycloak handles OTP / verify / reset |
| `family_links`, `sharing_settings` | **Keep, authoritative** | Relationship-based ACL — Postgres only |
| `care_orgs`, `care_staff`, `patients` | **Keep, authoritative** | Domain cohort & staff↔patient access |
| devices / records / alerts / metrics | **Unchanged** | FK to `users.id` intact |
