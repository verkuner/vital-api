#!/usr/bin/env bash
set -euo pipefail

# Seed development data into the database.
# Usage: DATABASE_URL=... ./scripts/seed.sh

DATABASE_URL="${DATABASE_URL:?DATABASE_URL environment variable required}"

echo "Seeding development data..."

psql "$DATABASE_URL" <<'SQL'
-- Test user (provider_id matches a Supabase/Keycloak test user)
INSERT INTO users (id, provider_id, email, name, date_of_birth)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'test-provider-user-001',
    'jane@example.com',
    'Jane Smith',
    '1990-06-15'
) ON CONFLICT (provider_id) DO NOTHING;

-- Default alert thresholds
INSERT INTO alert_thresholds (user_id, vital_type, low_value, high_value, enabled)
VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'heart_rate', 50, 110, true),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'spo2', 92, 100, true),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'temperature', 35.5, 37.8, true),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'blood_pressure_systolic', 80, 130, true),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'blood_pressure_diastolic', 50, 90, true)
ON CONFLICT (user_id, vital_type) DO NOTHING;

-- Sample vital readings (last 7 days)
INSERT INTO vital_readings (user_id, vital_type, value, unit, status, measured_at)
SELECT
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'heart_rate',
    60 + random() * 40,
    'bpm',
    'normal',
    NOW() - (n || ' hours')::interval
FROM generate_series(1, 168) AS n;

INSERT INTO vital_readings (user_id, vital_type, value, unit, status, measured_at)
SELECT
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'spo2',
    95 + random() * 5,
    '%',
    'normal',
    NOW() - (n || ' hours')::interval
FROM generate_series(1, 168) AS n;

-- A test device
INSERT INTO devices (user_id, name, type)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Apple Watch Series 10',
    'smartwatch'
) ON CONFLICT DO NOTHING;

SQL

echo "Seed complete."
