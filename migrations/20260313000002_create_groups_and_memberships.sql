-- Migration: create_groups_and_memberships
-- Generated: 20260313000002 UTC

-- Create groups table
CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    members_visible INTEGER NOT NULL DEFAULT 0,
    allow_self_add INTEGER NOT NULL DEFAULT 0,
    allow_sub_groups INTEGER NOT NULL DEFAULT 0,
    admin_group_id INTEGER NULL,
    FOREIGN KEY (admin_group_id) REFERENCES groups(id)
);

-- Create index on group name for lookups
CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);

-- Create person_group_memberships (through table for people in groups)
CREATE TABLE IF NOT EXISTS person_group_memberships (
    person_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    PRIMARY KEY (person_id, group_id),
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pgm_person ON person_group_memberships(person_id);
CREATE INDEX IF NOT EXISTS idx_pgm_group ON person_group_memberships(group_id);

-- Create group_subgroups (for nested groups when AllowSubGroups=true on parent)
CREATE TABLE IF NOT EXISTS group_subgroups (
    parent_group_id INTEGER NOT NULL,
    child_group_id INTEGER NOT NULL,
    PRIMARY KEY (parent_group_id, child_group_id),
    FOREIGN KEY (parent_group_id) REFERENCES groups(id) ON DELETE CASCADE,
    FOREIGN KEY (child_group_id) REFERENCES groups(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gs_parent ON group_subgroups(parent_group_id);
CREATE INDEX IF NOT EXISTS idx_gs_child ON group_subgroups(child_group_id);
