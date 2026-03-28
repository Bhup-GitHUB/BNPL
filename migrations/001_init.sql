CREATE TABLE IF NOT EXISTS applications (
    id TEXT PRIMARY KEY,
    trace_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    pan TEXT NOT NULL,
    mobile TEXT NOT NULL,
    email TEXT NOT NULL,
    employment_type TEXT NOT NULL,
    monthly_income BIGINT NOT NULL,
    requested_amount BIGINT NOT NULL,
    consent_handle TEXT NOT NULL,
    bank_account_ref TEXT NOT NULL,
    device_fingerprint TEXT NOT NULL,
    typing_cadence_ms BIGINT NOT NULL,
    form_fill_seconds BIGINT NOT NULL,
    address_pincode TEXT NOT NULL,
    employer_name TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS decisions (
    application_id TEXT PRIMARY KEY REFERENCES applications(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    score INTEGER NOT NULL,
    approved_limit BIGINT NOT NULL,
    reasons JSONB NOT NULL,
    rule_hits JSONB NOT NULL,
    score_breakdown JSONB NOT NULL,
    feature_summary JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS provider_snapshots (
    id BIGSERIAL PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    provider_name TEXT NOT NULL,
    status TEXT NOT NULL,
    latency_ms BIGINT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
