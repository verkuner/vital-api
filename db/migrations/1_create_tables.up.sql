-- Users (identity managed by auth provider, cached locally)
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     VARCHAR(255) NOT NULL UNIQUE,
    email           VARCHAR(255) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    date_of_birth   DATE,
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_provider_id ON users (provider_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- Vital Readings (standard table locally; TimescaleDB hypertable in production)
CREATE TABLE IF NOT EXISTS vital_readings (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vital_type      VARCHAR(50) NOT NULL,
    value           DOUBLE PRECISION NOT NULL,
    unit            VARCHAR(20) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'normal',
    device_id       UUID,
    notes           TEXT,
    measured_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, measured_at)
);

CREATE INDEX IF NOT EXISTS idx_vital_readings_user_type_time
    ON vital_readings (user_id, vital_type, measured_at DESC);

-- Alert Thresholds
CREATE TABLE IF NOT EXISTS alert_thresholds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vital_type      VARCHAR(50) NOT NULL,
    low_value       DOUBLE PRECISION,
    high_value      DOUBLE PRECISION,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(user_id, vital_type)
);

CREATE INDEX IF NOT EXISTS idx_alert_thresholds_user_id ON alert_thresholds (user_id);

-- Alerts
CREATE TABLE IF NOT EXISTS alerts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vital_type          VARCHAR(50) NOT NULL,
    value               DOUBLE PRECISION NOT NULL,
    threshold_breached  VARCHAR(10) NOT NULL,
    threshold_value     DOUBLE PRECISION NOT NULL,
    severity            VARCHAR(20) NOT NULL DEFAULT 'warning',
    acknowledged        BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_at     TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_user_id ON alerts (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_unacked ON alerts (user_id, acknowledged) WHERE acknowledged = FALSE;

-- Devices
CREATE TABLE IF NOT EXISTS devices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    type            VARCHAR(50) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices (user_id);

-- Provider-Patient assignments (for provider role access)
CREATE TABLE IF NOT EXISTS provider_patients (
    provider_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    patient_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider_id, patient_id)
);
