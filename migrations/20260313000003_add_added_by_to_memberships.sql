-- Migration: add_added_by_to_memberships
-- Generated: 20260313000003 UTC

-- Add added_by column to track who added a member
ALTER TABLE person_group_memberships ADD COLUMN added_by INTEGER NULL;
