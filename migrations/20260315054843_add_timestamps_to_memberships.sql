-- Migration: add_timestamps_to_memberships
-- Generated: 20260315054843 UTC

-- Add added_at timestamp to track when member was added
ALTER TABLE person_group_memberships ADD COLUMN added_at DATETIME NULL;

-- Add removed_at timestamp for soft delete (tracks when member was removed)
ALTER TABLE person_group_memberships ADD COLUMN removed_at DATETIME NULL;

-- Add removed_by to track who removed the member
ALTER TABLE person_group_memberships ADD COLUMN removed_by INTEGER NULL;

-- Set added_at for existing records to current timestamp
UPDATE person_group_memberships SET added_at = datetime('now') WHERE added_at IS NULL;

-- Create index for efficient querying of recent activities
CREATE INDEX IF NOT EXISTS idx_pgm_added_at ON person_group_memberships(added_at);
CREATE INDEX IF NOT EXISTS idx_pgm_removed_at ON person_group_memberships(removed_at);
CREATE INDEX IF NOT EXISTS idx_pgm_removed_by ON person_group_memberships(removed_by);
