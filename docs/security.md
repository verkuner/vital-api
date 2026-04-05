# Security

Security architecture, threat model, and implementation guidelines for the Vital Signs API.

## Threat Model

The API handles Protected Health Information (PHI) — vital sign readings, user identities, and medical context. Key threats:

| Threat | Mitigation |
|--------|-----------|
| Unauthenticated access | JWT validation on all protected routes |
| Token theft | Short-lived JWTs (15 min), refresh token rotation |
| Broken object-level authorization | User ID always sourced from JWT, never request body |
| PHI data leakage in logs/traces | OTel Collector attribute scrubbing |
| SQL injection | sqlc parameterized queries (no string interpolation) |
| Rate abuse / DoS | Redis token bucket rate limiter per user/IP |
| CSRF | Stateless JWT (no cookies in API) |
| XSS | Not applicable (no HTML rendering in API) |
| Dependency vulnerabilities | `govulncheck` + Dependabot in CI |

## Authentication

### Auth Architecture — Vault + Gatekeeper

The auth system separates two roles:

- **Vault** (identity provider): Stores credentials, issues JWTs, handles registration/login/MFA.
  - Local/dev: Supabase Auth or Clerk (managed SaaS — no self-hosting)
  - Production: Standalone Keycloak
- **Gatekeeper** (token validator): Encore `//encore:authhandler` validates JWTs from whichever Vault is configured.

### JWT Flow

```
Client → POST /auth/login → auth service → AuthProvider (Vault)
                                             ├─ Supabase Auth (local/dev)
                                             └─ Keycloak (production)
                          ← Access Token (15 min) + Refresh Token (7 days)

Client → GET /vitals
       Authorization: Bearer <access_token>
       → Encore //encore:authhandler (Gatekeeper) validates JWT via JWKS
       → Extract user ID from `sub` claim
       → Handler runs with auth.UID + AuthData
```

### Gatekeeper — Encore Auth Handler (authhandler/authhandler.go)

The auth handler is **provider-agnostic**. It validates JWTs from any OIDC-compliant issuer via a configurable JWKS URL:

```go
type AuthData struct {
    UserID      uuid.UUID
    Email       string
    Roles       []string
}

//encore:authhandler
func Authenticate(ctx context.Context, token string) (auth.UID, *AuthData, error) {
    // 1. Fetch JWKS from configured JWKS_URL (cache with TTL)
    //    - Local/dev: Supabase/Clerk JWKS endpoint
    //    - Production: Keycloak JWKS endpoint
    // 2. Cache JWKS with 1-hour TTL, refresh on key rotation (kid mismatch)
    // 3. Parse and validate:
    //    - Signature (RS256 with provider's public key)
    //    - exp claim (not expired)
    //    - iss claim (matches configured issuer)
    //    - aud claim (matches configured audience)
    // 4. Extract user info: sub, email, roles
    //    (claim paths vary by provider — normalize in AuthData)
    // 5. Return auth.UID (provider's user ID) and AuthData
}
```

### Claims Access in Handlers

```go
//encore:api auth method=GET path=/api/v1/vitals
func (s *VitalService) ListVitals(ctx context.Context, req *ListVitalsRequest) (*ListVitalsResponse, error) {
    userID := auth.UserID()     // provider's user ID (Supabase uid / Clerk user_id / Keycloak sub)
    data := auth.Data().(*AuthData)
    // ...
}
```

### Environment-Specific Auth

| Concern | Local / Dev | Production |
|---------|-------------|------------|
| Vault (identity provider) | Supabase Auth or Clerk | Standalone Keycloak |
| Gatekeeper (token validation) | Encore `//encore:authhandler` | Same code |
| JWKS endpoint | Supabase/Clerk `.well-known/jwks.json` | Keycloak JWKS endpoint |
| User management API | Supabase Auth REST API / Clerk Backend API | Keycloak Admin REST API |
| Token issuance | Supabase Auth / Clerk | Keycloak |
| Self-hosted? | No (managed SaaS) | Yes |

### Roles

| Role | Capabilities |
|------|-------------|
| `patient` | CRUD own vitals, own alerts, own profile |
| `provider` | Read patient vitals (assigned patients only) |
| `admin` | Full access, user management |

Role checks happen in the service layer, not the handler. Access auth data via Encore's `auth` package:

```go
func (s *Service) GetVitalsForUser(ctx context.Context, requestorID, targetUserID uuid.UUID) ([]*VitalReading, error) {
    data := auth.Data().(*AuthData)
    hasRole := func(role string) bool {
        for _, r := range data.Roles {
            if r == role { return true }
        }
        return false
    }
    if requestorID != targetUserID && !hasRole("provider") && !hasRole("admin") {
        return nil, fmt.Errorf("get vitals: %w", apperror.ErrForbidden)
    }
    // ...
}
```

## Authorization

### Object-Level Authorization (BOLA/IDOR Prevention)

Every DB query for a user-owned resource MUST include `user_id` from the JWT:

```sql
-- CORRECT: user_id is from JWT claim, not request param
SELECT * FROM vital_readings
WHERE id = $1 AND user_id = $2;  -- $2 = JWT sub

-- WRONG: fetching by ID alone then checking ownership in Go
SELECT * FROM vital_readings WHERE id = $1;
```

This is enforced by passing `userID` from the middleware context to every repository call — the repository never derives user ownership from the request body.

### Provider Access

Providers may only access vitals for patients they are assigned to. This is enforced via a `provider_patient` join table:

```sql
SELECT vr.* FROM vital_readings vr
JOIN provider_patients pp ON pp.patient_id = vr.user_id
WHERE pp.provider_id = $1  -- from JWT
  AND vr.user_id = $2      -- requested patient
  AND vr.measured_at BETWEEN $3 AND $4;
```

## Transport Security

- **TLS**: Terminated at the load balancer/reverse proxy (Nginx/Traefik). Internal traffic between services may be plaintext on a private network.
- **HSTS**: Set by reverse proxy for public endpoints.
- **Minimum TLS**: 1.2 in staging, 1.3 preferred in production.
- **Certificate**: Auto-renewed via Let's Encrypt (Certbot or Traefik ACME).

## Input Validation

All request bodies are validated before reaching service logic:

```go
// 1. JSON decode (shape validation)
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    respondError(w, 400, "INVALID_JSON", ...)
    return
}

// 2. Struct validation (field rules)
if errs := h.validator.Validate(req); len(errs) > 0 {
    respondValidationError(w, errs)
    return
}

// 3. Business rule validation in service layer
if req.MeasuredAt.After(time.Now().Add(5 * time.Minute)) {
    return nil, fmt.Errorf("measured_at cannot be in the future: %w", apperror.ErrValidation)
}
```

Validation rules for vital types:

```go
type RecordVitalRequest struct {
    VitalType  string    `validate:"required,oneof=heart_rate spo2 temperature systolic diastolic respiratory_rate"`
    Value      float64   `validate:"required,gt=0,lte=1000"`
    Unit       string    `validate:"required,oneof=bpm % degC degF mmHg breaths_per_min"`
    MeasuredAt time.Time `validate:"required"`
}
```

## PHI Protection

### Data at Rest

- PostgreSQL database encrypted at filesystem level (LUKS on Linux, EBS encryption on AWS)
- No unencrypted backups; pgdump output encrypted with GPG before upload to S3

### Data in Transit

- All external traffic over TLS
- Database connections use `sslmode=require` in staging/production

### PHI in Logs and Traces

The OTel Collector `attributes/sanitize` processor strips sensitive data before export to SigNoz:

```yaml
# deployments/docker/otel-collector-config.yaml
processors:
  attributes/sanitize:
    actions:
      - key: db.statement       # strip raw SQL (may contain values)
        action: delete
      - key: vital.value        # strip raw vital readings
        action: delete
      - key: http.request.body  # never log request bodies
        action: delete
      - key: user.email
        action: hash            # hash instead of delete for debugging
```

`slog` structured logging rules:
- Never log `vital_value`, raw JWT tokens, passwords, or email addresses
- Use `user_id` (UUID) in logs, not email or name
- Redact sensitive fields in error messages: `"failed to process reading for user %s"` not `"...for john@example.com"`

### Data Retention

- Vital readings: retained indefinitely (patient medical history)
- Sessions/tokens: Redis TTL matches JWT expiry
- Audit logs: 7 years (HIPAA requirement)
- Error logs: 90 days

## Secrets Management

```bash
# Never committed to git
.env                      # local dev only
.env.staging              # injected by CI/CD

# Production
# Secrets stored in:
# - AWS Secrets Manager / HashiCorp Vault
# - Injected as environment variables at container start
# - Never written to disk in container
```

Required secrets and their rotation policy:

| Secret | Rotation | Storage |
|--------|----------|---------|
| `DATABASE_URL` password | 90 days | Vault |
| Auth provider secret (Keycloak/Supabase/Clerk) | 90 days | Vault / Encore secrets |
| JWT signing key (provider-managed) | On compromise | Auth provider |

## Rate Limiting

Implemented in `internal/middleware/ratelimit.go` using Redis token bucket:

```
Key pattern:
  ratelimit:auth:{ip}          → 10 requests / 1 minute
  ratelimit:api:{user_id}      → 100 requests / 1 minute
  ratelimit:vitals:{user_id}   → 60 POST requests / 1 minute
```

On limit exceeded:
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 42
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1735689600

{"error": {"code": "RATE_LIMITED", "message": "Too many requests. Retry after 42 seconds."}}
```

## Security Headers

Set in `internal/middleware/security_headers.go`:

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("Referrer-Policy", "no-referrer")
w.Header().Set("Content-Security-Policy", "default-src 'none'")
// HSTS set by reverse proxy, not the Go app
```

## Dependency Security

```bash
# Check for known vulnerabilities
govulncheck ./...

# Audit go.sum
go mod verify

# In CI (GitHub Actions)
- uses: golang/govulncheck-action@v1
  with:
    go-version-input: "1.24"
    check-latest: true
```

Dependencies are pinned to exact versions in `go.sum`. Dependabot PRs for security patches are auto-merged if CI passes.

## Penetration Testing Checklist

Before each major release, verify:

- [ ] All protected routes return `401` without a valid token
- [ ] User A cannot read/modify User B's vitals (IDOR test)
- [ ] SQL injection attempt on filter params returns `400`, not `500`
- [ ] Rate limiter triggers correctly and returns `429`
- [ ] JWT with expired `exp` returns `401`
- [ ] JWT signed with a different key returns `401`
- [ ] No stack traces in `500` responses
- [ ] PHI absent from application logs
- [ ] `govulncheck` passes with no HIGH/CRITICAL findings
