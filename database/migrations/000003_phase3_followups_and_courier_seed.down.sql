DROP INDEX IF EXISTS idx_follow_ups_queue;
DROP INDEX IF EXISTS idx_follow_ups_assigned_to;

ALTER TABLE follow_ups
DROP COLUMN IF EXISTS assigned_to;
