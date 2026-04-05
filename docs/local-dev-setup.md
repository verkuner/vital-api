# Local/Dev Environment Setup

## Overview

The local/dev environment uses **Encore** as the API framework and **Supabase** as the auth provider (Vault) and PostgreSQL database. No Docker is required — the database connects directly to Supabase's hosted PostgreSQL via `pgxpool`.

## Architecture

| Component | Provider |
|-----------|----------|
| **API Framework** | Encore (`encore run`) |
| **Database** | Supabase PostgreSQL (external, via `pgxpool`) |
| **Auth Vault** | Supabase Auth (issues JWTs) |
| **Auth Gatekeeper** | Encore `//encore:authhandler` (validates JWTs via JWKS) |
| **Tracing** | Encore built-in (localhost:9400 dashboard) |
| **Secrets** | Encore secrets (`encore secret set`) |

## Prerequisites

- **Encore CLI**: Install via `curl -L https://encore.dev/install.sh | bash`
- **Encore account**: `encore auth login`
- **Supabase project**: Create at https://supabase.com
- **psql**: For running migrations (`/opt/homebrew/opt/libpq/bin/psql`)

## Setup Steps

### 1. Encore App Initialization

The app is linked to Encore Cloud as `vital-api-cq4i`:

```bash
encore app init vital-api --lang=go
```

### 2. Encore Secrets

All secrets are stored via Encore's secrets system (not `.env` files):

```bash
encore secret set --type dev,local DatabaseURL
# postgresql://postgres:<password>@db.<project>.supabase.co:5432/postgres?sslmode=require

encore secret set --type dev,local SupabaseURL
# https://<project>.supabase.co

encore secret set --type dev,local SupabaseAnonKey
# sb_publishable_...

encore secret set --type dev,local SupabaseServiceRoleKey
# sbp_...

encore secret set --type dev,local JWKSURL
# https://<project>.supabase.co/auth/v1/.well-known/jwks.json
```

### 3. Database Migration

Migrations are run directly against Supabase PostgreSQL (not managed by Encore):

```bash
psql "postgresql://postgres:<password>@db.<project>.supabase.co:5432/postgres?sslmode=require" \
  -f vital/migrations/1_create_tables.up.sql
```

Tables created:
- `users` — cached identity data (provider_id references Supabase user ID)
- `vital_readings` — vital sign measurements (standard table, no TimescaleDB locally)
- `alert_thresholds` — per-user alert configuration
- `alerts` — generated alerts when thresholds are breached

### 4. Run the API

```bash
encore run
# API:       http://localhost:4000
# Dashboard: http://localhost:9400/vital-api-cq4i
```

## Services & Endpoints

### vital service (`vital/`)

| Endpoint | Auth | Method | Path | Description |
|----------|------|--------|------|-------------|
| Health | public | GET | `/vital/health` | Health check |
| Me | auth | GET | `/vital/me` | Returns authenticated user info |

### auth service (`auth/`)

| Endpoint | Auth | Method | Path | Description |
|----------|------|--------|------|-------------|
| Register | public | POST | `/auth/register` | Create user via Supabase Auth |
| Login | public | POST | `/auth/login` | Authenticate via Supabase Auth |
| Refresh | public | POST | `/auth/refresh` | Refresh token pair |

### authhandler (`authhandler/`)

The `//encore:authhandler` validates JWTs from Supabase by:
1. Fetching JWKS from the configured URL (cached with 10-minute TTL)
2. Parsing and validating JWT signature (RS256)
3. Extracting `sub` (user ID), `email`, and `role` claims
4. Returning `auth.UID` and `AuthData` to downstream endpoints

## Key Files

```
vital-api/
  encore.app                          # Encore app config (id: vital-api-cq4i)
  authhandler/authhandler.go          # JWT validation (//encore:authhandler)
  auth/auth.go                        # Auth API endpoints (//encore:service)
  auth/service.go                     # Supabase Auth REST API client
  auth/model.go                       # Request/response DTOs
  vital/vital.go                      # Vital API endpoints (//encore:service)
  vital/service.go                    # Service initialization
  vital/db.go                         # External DB connection via pgxpool
  vital/migrations/1_create_tables.up.sql  # Schema (run manually against Supabase)
  apperror/                           # Shared domain error types
```

## Database Strategy (No Docker)

Encore normally auto-provisions PostgreSQL via Docker. This project bypasses that by:

1. **Not using** `sqldb.NewDatabase()` (which requires Docker)
2. **Using** the Encore-documented external database pattern with `pgxpool`
3. **Storing** the Supabase connection string as an Encore secret (`DatabaseURL`)
4. **Running** migrations manually via `psql`

Trade-offs:
- No Encore DB dashboard or auto-migration
- No TimescaleDB locally (standard PostgreSQL via Supabase)
- Full Docker-free local development

## Makefile Targets

```bash
make encore-run     # Start Encore dev server
make encore-test    # Run tests via Encore
make encore-build   # Build Docker image via Encore
```

## Testing the Auth Flow

```bash
# 1. Register a user
curl -X POST http://localhost:4000/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123","name":"Test User"}'

# 2. Login to get tokens
curl -X POST http://localhost:4000/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123"}'

# 3. Use the access_token on a protected endpoint
curl http://localhost:4000/vital/me \
  -H 'Authorization: Bearer <access_token>'

# 4. Refresh tokens
curl -X POST http://localhost:4000/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}'
```
