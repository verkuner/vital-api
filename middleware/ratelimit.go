package middleware

// Rate limiting is configured per-endpoint using Redis token buckets.
// In local/dev, Encore handles basic rate limiting.
// Production rate limit middleware is applied via the standalone HTTP server.
//
// Rate limit constants:
//   POST /auth/*        → 10 requests / 1 minute / IP
//   GET  /vitals/*      → 100 requests / 1 minute / user
//   POST /vitals        → 60 requests / 1 minute / user
//   WebSocket /ws       → 5 connections / user
//
// Redis key patterns:
//   vital-api:ratelimit:auth:{ip}
//   vital-api:ratelimit:api:{user_id}
//   vital-api:ratelimit:vitals:{user_id}
