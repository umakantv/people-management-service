-- Migration: add_deactivation_fields
-- Generated: 20260313000001 UTC

-- Add deactived_at and activated_at columns (nullable timestamps)
ALTER TABLE people ADD COLUMN deactived_at TEXT NULL;
ALTER TABLE people ADD COLUMN activated_at TEXT NULL;
