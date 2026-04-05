# Vital Signs API

Backend API for a vital signs monitoring mobile application. Record, track, and get alerts on health metrics like heart rate, blood pressure, temperature, oxygen saturation, and more.

## What It Does

- **Record vitals** from mobile devices or wearables
- **Track trends** with time-series queries and summaries
- **Set alert thresholds** per vital type (e.g., heart rate > 110 bpm)
- **Get notified** when readings breach your thresholds
- **Real-time streaming** via WebSocket for live monitoring dashboards
- **Multi-provider auth** — works with Supabase Auth, Clerk, or Keycloak

## Getting Started

### Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Encore CLI](https://encore.dev/docs/install) — `brew install encoredev/tap/encore`
- A [Supabase](https://supabase.com) project (free tier works)

### 1. Clone & configure

```bash
git clone https://github.com/verkuner/vital-api.git
cd vital-api
```

Set your Supabase credentials as Encore secrets:

```bash
encore secret set --type local,dev DatabaseURL
# Paste: postgresql://postgres:<password>@db.<ref>.supabase.co:5432/postgres

encore secret set --type local,dev SupabaseURL
# Paste: https://<ref>.supabase.co

encore secret set --type local,dev SupabaseAnonKey
# Paste: your anon key from Supabase Dashboard > Settings > API

encore secret set --type local,dev SupabaseServiceRoleKey
# Paste: your service_role key

encore secret set --type local,dev JWKSURL
# Paste: https://<ref>.supabase.co/auth/v1/.well-known/jwks.json
```

### 2. Run locally

```bash
encore run --port 5080
```

- API: http://127.0.0.1:5080
- Dashboard: http://127.0.0.1:9400

### 3. Verify

```bash
curl http://127.0.0.1:5080/vital/health
# {"status":"ok","service":"vital"}
```

### 4. Create a test user & start using the API

```bash
# Register
curl -X POST http://127.0.0.1:5080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"YourPass123!","name":"Your Name"}'

# Login (save the access_token)
curl -X POST http://127.0.0.1:5080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"YourPass123!"}'

# Provision your profile (required before recording vitals)
curl http://127.0.0.1:5080/api/v1/users/me \
  -H 'Authorization: Bearer <access_token>'

# Record a heart rate reading
curl -X POST http://127.0.0.1:5080/api/v1/vitals \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{"vital_type":"heart_rate","value":72,"unit":"bpm","measured_at":"2026-04-05T10:30:00Z"}'

# List your vitals
curl http://127.0.0.1:5080/api/v1/vitals \
  -H 'Authorization: Bearer <access_token>'
```

## API Testing with Bruno

The repo includes a [Bruno](https://www.usebruno.com/) collection with 27 pre-built requests.

```bash
brew install --cask bruno
```

Open Bruno, select **Open Collection**, and point to the `bruno/` folder. Choose the `local` or `dev` environment and run requests interactively.

CLI usage:

```bash
cd bruno
bru run --env local --sandbox=developer auth/Login.bru admin/Setup\ Profile.bru vitals/
```

## Environments

| Environment | URL | Trigger |
|-------------|-----|---------|
| Local | http://127.0.0.1:5080 | `encore run` |
| Dev (Encore Cloud) | https://staging-vital-api-cq4i.encr.app | Push to `main` |

## Supported Vital Types

| Type | Unit | Example Normal Range |
|------|------|---------------------|
| `heart_rate` | bpm | 60–100 |
| `blood_pressure_systolic` | mmHg | 90–120 |
| `blood_pressure_diastolic` | mmHg | 60–80 |
| `temperature` | °F | 97.0–99.5 |
| `oxygen_saturation` | % | 95–100 |
| `respiratory_rate` | breaths/min | 12–20 |
| `blood_glucose` | mg/dL | 70–140 |

## Auth Flow

1. **Register** or **Login** via `/api/v1/auth/login` to get a JWT access token
2. Pass the token as `Authorization: Bearer <token>` on all authenticated endpoints
3. On first authenticated request, call `GET /api/v1/users/me` to auto-provision your local profile
4. Tokens expire after 1 hour — use `/api/v1/auth/refresh` to renew

## Database Setup

Tables are auto-created via the `/admin/migrate` endpoint or on first run. For manual setup:

```bash
# Check existing tables
curl http://127.0.0.1:5080/admin/check-tables

# Run migrations (idempotent)
curl -X POST http://127.0.0.1:5080/admin/migrate
```

## Deployment

Pushes to `main` auto-deploy to Encore Cloud. For self-hosted production:

```bash
# Build Docker image
encore build docker vital-api:latest

# Start infrastructure
docker compose -f deployments/docker/docker-compose.yml up -d

# Run database migrations
./scripts/migrate.sh up
```

## Further Reading

See the [`docs/`](docs/) folder for detailed documentation on architecture, conventions, and internals.
