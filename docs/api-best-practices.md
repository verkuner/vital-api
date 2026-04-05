# API Best Practices

Guidelines for designing, implementing, and maintaining the Vital Signs REST API.

## URL Structure

All endpoints follow the pattern: `/{prefix}/v{major}/{resource}/{id?}/{subresource?}`

Endpoints are defined using Encore `//encore:api` annotations:

```go
//encore:api auth method=GET path=/api/v1/vitals/:id
func (s *VitalService) GetVital(ctx context.Context, id string) (*VitalResponse, error)
```

```
/api/v1/users/{id}/vitals
/api/v1/vitals/{id}
/api/v1/alerts
/api/v1/auth/login
```

### Rules

- Lowercase, hyphen-separated path segments: `/alert-thresholds`, not `/alertThresholds`
- Nouns for resources, not verbs: `GET /vitals`, not `GET /getVitals`
- IDs always in path params (`:id` in Encore), never in query string for primary resource lookup
- Use plural resource names: `/vitals`, `/alerts`, `/users`
- Nest sub-resources up to one level deep; avoid `/users/{id}/vitals/{id}/notes/{id}`

## HTTP Methods

| Method | Usage | Idempotent | Body |
|--------|-------|-----------|------|
| `GET` | Read resource(s) | Yes | No |
| `POST` | Create resource | No | Yes |
| `PUT` | Full replace | Yes | Yes |
| `PATCH` | Partial update | Yes | Yes (partial) |
| `DELETE` | Remove resource | Yes | No |

- Use `POST /vitals` to create, not `PUT /vitals` (no client-assigned ID)
- Use `PATCH /users/{id}` for profile updates (only changed fields)
- Use `POST /alerts/{id}/acknowledge` for actions that don't fit CRUD

## Request & Response Format

### Content Type

All requests and responses use `application/json`.

```
Content-Type: application/json
Accept: application/json
```

### Request Bodies

```go
// Always validate with go-playground/validator
type RecordVitalRequest struct {
    VitalType  string    `json:"vital_type"  validate:"required,oneof=heart_rate blood_pressure_systolic blood_pressure_diastolic spo2 temperature respiratory_rate"`
    Value      float64   `json:"value"       validate:"required,gt=0"`
    Unit       string    `json:"unit"        validate:"required"`
    DeviceID   *string   `json:"device_id"   validate:"omitempty,uuid4"`
    Notes      *string   `json:"notes"       validate:"omitempty,max=500"`
    MeasuredAt time.Time `json:"measured_at" validate:"required"`
}
```

### Response Envelope

Single resource:
```json
{
  "data": { "id": "...", "vital_type": "heart_rate", "value": 72 }
}
```

Collection:
```json
{
  "data": [...],
  "meta": {
    "total": 100,
    "page": 1,
    "per_page": 20,
    "pages": 5
  }
}
```

Error:
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": [
      { "field": "value", "message": "must be greater than 0" }
    ]
  }
}
```

## HTTP Status Codes

| Code | Meaning | When to Use |
|------|---------|-------------|
| `200` | OK | Successful GET, PATCH, DELETE |
| `201` | Created | Successful POST that creates a resource |
| `204` | No Content | DELETE with no body, or action with no result |
| `400` | Bad Request | Malformed JSON, missing required fields |
| `401` | Unauthorized | Missing or invalid JWT |
| `403` | Forbidden | Valid JWT but insufficient permissions |
| `404` | Not Found | Resource does not exist |
| `409` | Conflict | Duplicate resource (unique constraint) |
| `422` | Unprocessable Entity | Valid JSON but business rule violation |
| `429` | Too Many Requests | Rate limit exceeded |
| `500` | Internal Server Error | Unexpected server error |

Never return `200` for errors. Never expose stack traces in error responses.

## Pagination

Use cursor-based pagination for time-series data (vitals), offset for bounded sets (alerts).

### Cursor Pagination (Vitals)

```
GET /api/v1/vitals?vital_type=heart_rate&limit=20&before=2026-01-15T10:00:00Z
```

Response includes:
```json
{
  "data": [...],
  "meta": {
    "limit": 20,
    "has_more": true,
    "next_cursor": "2026-01-10T08:30:00Z"
  }
}
```

### Offset Pagination (Alerts)

```
GET /api/v1/alerts?page=2&per_page=20&sort=created_at&order=desc
```

### Rules

- Default `per_page`/`limit`: 20
- Maximum `per_page`/`limit`: 100
- Always include `has_more` or `total` so clients can render pagination UI

## Filtering & Sorting

```
GET /api/v1/vitals?vital_type=heart_rate&from=2026-01-01T00:00:00Z&to=2026-01-31T23:59:59Z
GET /api/v1/alerts?severity=critical&acknowledged=false
GET /api/v1/alerts?sort=created_at&order=desc
```

### Rules

- All filter params are optional — no filter returns all authorized records
- Date ranges use ISO 8601 with timezone: `2026-01-01T00:00:00Z`
- Sorting uses `sort={field}&order={asc|desc}`
- Unknown filter params return `400 Bad Request` (prevents silent bugs)

## Versioning

```
/api/v1/...   # current stable
/api/v2/...   # introduced when breaking changes are required
```

- Minor, additive changes (new optional fields, new endpoints) do not require a version bump
- Breaking changes (removed fields, changed semantics, required field additions) require `/v2/`
- Old version gets `Sunset` header 90 days before removal: `Sunset: Sat, 01 Jan 2027 00:00:00 GMT`
- Document the migration guide in `docs/migration/v1-to-v2.md`

## Response Headers

Every response should include:

```
X-Request-ID: <uuid>         # echoed from request or generated
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1735689600
Content-Type: application/json
```

## Idempotency

For POST endpoints that should be idempotent (e.g. recording a vital from a device that may retry):

```
POST /api/v1/vitals
Idempotency-Key: <client-generated-uuid>
```

- Store key in Redis with 24h TTL
- On duplicate key: return the original response with `200` instead of `201`
- Document which endpoints support idempotency keys

## Handler Pattern (Encore)

Encore handles JSON serialization, deserialization, and HTTP status codes automatically. API handlers are typed Go functions:

```go
// vital/vital.go
//encore:api auth method=POST path=/api/v1/vitals
func (s *VitalService) RecordVital(ctx context.Context, req *RecordVitalRequest) (*VitalReadingResponse, error) {
    userID := auth.UserID()

    if errs := s.validator.Validate(req); errs != nil {
        return nil, &errs.Error{Code: errs.InvalidArgument, Message: "validation failed"}
    }

    reading, err := s.svc.RecordVital(ctx, uuid.MustParse(string(userID)), *req)
    if err != nil {
        return nil, mapDomainError(err)
    }

    return &VitalReadingResponse{Data: reading}, nil
}
```

Encore automatically:
- Parses JSON request body into the `req` parameter
- Serializes the return struct to JSON
- Maps `error` returns to appropriate HTTP status codes
- Handles content-type headers

## Rate Limiting Strategy

| Endpoint Group | Limit | Window | Key |
|---------------|-------|--------|-----|
| `POST /auth/*` | 10 | 1 minute | IP |
| `GET /vitals/*` | 100 | 1 minute | User ID |
| `POST /vitals` | 60 | 1 minute | User ID |
| `WebSocket /ws` | 5 connections | — | User ID |

Rate limit state is stored in Redis using the token bucket algorithm.

## API Documentation

### Local/Dev (Encore)

Encore automatically generates API documentation from the `//encore:api` annotations and Go type definitions. Available in the Encore dev dashboard:

```bash
encore run
open http://localhost:9400   # Service Catalog + API Explorer
```

The API Explorer allows testing endpoints directly from the browser with auto-generated forms based on request types.

### Production (Swagger/OpenAPI)

For production API documentation, use swaggo annotations alongside Encore annotations:

```go
// RecordVital godoc
// @Summary     Record a vital reading
// @Description Records a new vital sign reading for the authenticated user
// @Tags        vitals
// @Accept      json
// @Produce     json
// @Param       body body RecordVitalRequest true "Vital reading"
// @Success     201 {object} VitalReadingResponse
// @Failure     400 {object} ErrorResponse
// @Router      /vitals [post]
// @Security    BearerAuth
//encore:api auth method=POST path=/api/v1/vitals
func (s *VitalService) RecordVital(ctx context.Context, req *RecordVitalRequest) (*VitalReadingResponse, error) {
```

Generate docs: `make swagger` → served at `/swagger/index.html`.
