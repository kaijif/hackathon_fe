-- 0003_participant_checkins.sql
-- Track the last explicit "are you OK?" check-in acknowledgement per participant
-- so the monitoring loop can reset the "missing" timer on a check-in, not only
-- on a fresh location report.

ALTER TABLE night_participant_statuses
    ADD COLUMN IF NOT EXISTS last_checkin_at TIMESTAMPTZ;
