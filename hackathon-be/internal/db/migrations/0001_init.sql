-- 0001_init.sql
-- Schema for the NightWatch backend (a Life360-style group safety monitor).

CREATE TABLE IF NOT EXISTS users (
    id                  UUID PRIMARY KEY,
    name                TEXT NOT NULL,
    trusted_contact     TEXT,                       -- phone number of an emergency contact
    lat                 DOUBLE PRECISION,
    lng                 DOUBLE PRECISION,
    battery_level       INT CHECK (battery_level BETWEEN 0 AND 100),
    location_updated_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS agents (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS groups (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    active        BOOLEAN NOT NULL DEFAULT false,
    curr_night_id UUID,                             -- FK to nights, added below
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id  UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_admin  BOOLEAN NOT NULL DEFAULT false,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS nights (
    id                    UUID PRIMARY KEY,
    group_id              UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    agent_id              UUID REFERENCES agents(id) ON DELETE SET NULL,
    center_lat            DOUBLE PRECISION,
    center_lng            DOUBLE PRECISION,
    time_limit_min        INT NOT NULL DEFAULT 480,
    check_in_limit_min    INT NOT NULL DEFAULT 60,
    check_in_every_min    INT NOT NULL DEFAULT 15,
    max_range_m           INT NOT NULL DEFAULT 1000,
    low_battery_threshold INT NOT NULL DEFAULT 20,
    status                TEXT NOT NULL DEFAULT 'pending',   -- pending | active | ended
    started_at            TIMESTAMPTZ,
    ended_at              TIMESTAMPTZ,
    last_checked_at       TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'groups_curr_night_fk'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_curr_night_fk
            FOREIGN KEY (curr_night_id) REFERENCES nights(id) ON DELETE SET NULL;
    END IF;
END$$;

CREATE INDEX IF NOT EXISTS idx_nights_group   ON nights(group_id);
CREATE INDEX IF NOT EXISTS idx_nights_status  ON nights(status);

-- Latest reported location per participant within a night.
CREATE TABLE IF NOT EXISTS night_locations (
    night_id      UUID NOT NULL REFERENCES nights(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lat           DOUBLE PRECISION NOT NULL,
    lng           DOUBLE PRECISION NOT NULL,
    battery_level INT,
    reported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (night_id, user_id)
);

-- Latest computed status per participant within a night.
CREATE TABLE IF NOT EXISTS night_participant_statuses (
    night_id   UUID NOT NULL REFERENCES nights(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'unknown',   -- ok | out_of_range | low_battery | missing | unknown
    detail     TEXT,
    distance_m DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (night_id, user_id)
);

-- Outbound messages / notifications persisted by the Notifier.
CREATE TABLE IF NOT EXISTS messages (
    id                UUID PRIMARY KEY,
    night_id          UUID REFERENCES nights(id) ON DELETE CASCADE,
    group_id          UUID REFERENCES groups(id) ON DELETE CASCADE,
    recipient_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    recipient_contact TEXT,
    kind              TEXT NOT NULL,               -- message | notify | alert
    body              TEXT NOT NULL,
    sender            TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_messages_night     ON messages(night_id);
CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient_user_id);
