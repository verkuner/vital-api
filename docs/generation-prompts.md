Prompt 1 — Foundation + Encore Setup
Generate the project scaffold for vital-api: encore.app, go.mod, Makefile, all empty directories for services (auth/, vital/, user/, alert/, websocket/), shared packages (authhandler/, authprovider/, middleware/, observability/, database/, apperror/), and initial files: apperror/errors.go, apperror/http_errors.go, database/redis.go, observability/logger.go.

Follow CLAUDE.md for Encore service-based structure. Follow docs/code-conventions.md for error patterns and docs/api-best-practices.md for HTTP status codes. Set up encore.app configuration.


Prompt 2 — Database layer
Generate database declarations (vital/db.go, user/db.go, alert/db.go, auth/db.go), all SQL migrations in each service's migrations/ directory, sqlc query files in db/queries/, and db/sqlc.yaml.

Follow docs/database.md entirely — schema, Encore sqldb declarations, TimescaleDB-aware migrations (with fallbacks for local/dev), sqlc config, and query patterns. Queries must cover all operations needed by endpoints in docs/features.md. Note: TimescaleDB features (hypertable, continuous aggregates) should be conditional for production only.


Prompt 3 — Auth Handler + Middleware
Generate authhandler/authhandler.go (Encore //encore:authhandler — provider-agnostic JWT validation), authprovider/provider.go (AuthProvider interface), authprovider/supabase.go (Supabase Auth implementation), authprovider/clerk.go (Clerk implementation), authprovider/keycloak.go (Keycloak implementation), and all files in middleware/ (ratelimit.go, cors.go).

Follow docs/security.md for auth architecture: Supabase Auth/Clerk as Vault for local/dev, Keycloak as Vault for production, Encore auth handler as Gatekeeper in all environments. The auth handler validates JWTs from any OIDC-compliant provider via configurable JWKS URL. Follow docs/code-conventions.md middleware chain.


Prompt 4 — Observability
Generate observability/otel.go (production OTel setup), observability/meter.go (custom metrics), observability/logger.go (slog config).

Follow docs/observability.md entirely. OTel initialization should be environment-aware: no-op in local/dev (Encore handles tracing), full OTel pipeline in production (traces, metrics, logs -> SigNoz). Follow PHI scrubbing rules.


Prompt 5 — Feature services
Generate all files in vital/, alert/, user/, auth/ — Encore API endpoints ({service}.go with //encore:api annotations), service.go, repository.go, model.go for each service.

Follow docs/features.md for all endpoints, request/response shapes, and business rules. Follow docs/code-conventions.md Encore service struct pattern. Follow docs/security.md for ownership checks and role-based access. Auth service delegates to AuthProvider interface (Supabase Auth/Clerk for local/dev, Keycloak for production). Follow docs/testing.md for unit test structure alongside each package.


Prompt 6 — WebSocket, Server Wiring & Tests
Generate websocket/ (websocket.go with //encore:api raw endpoint, hub.go, client.go, message.go), and all unit/integration tests.

Follow docs/features.md WebSocket section for message types and limits. WebSocket uses Encore's raw endpoint pattern. Follow docs/testing.md for Encore test runner patterns, mock structures, and repository integration tests.


Prompt 7 — Production Deployment
Generate deployments/docker/docker-compose.yml (Postgres+TimescaleDB, Redis, Keycloak, SigNoz, OTel Collector), deployments/docker/otel-collector-config.yaml, scripts/migrate.sh, and scripts/seed.sh.

Follow CLAUDE.md for production standalone infra. Follow docs/observability.md for OTel Collector config (production only). docker-compose.yml includes Keycloak for production auth, but NOT the API itself (that runs via `encore run` locally or `encore build docker` for production images). Local/dev uses Supabase Auth or Clerk (managed SaaS) instead of Keycloak.
