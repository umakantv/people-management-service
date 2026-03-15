-- Migration: add_fts5_search_tables
-- Generated: 20260315072311 UTC
-- Full-text search using SQLite FTS5 for people and groups
--
-- REQUIREMENT: SQLite FTS5 extension must be available.
-- For go-sqlite3: compile with `go run --tags "fts5" main.go`
-- If unavailable: this migration fails. Set USE_FTS=false in .env to skip FTS5.
-- Search will automatically fallback to LIKE queries when FTS5 is disabled/unavailable.

-- Create FTS5 virtual table for people (indexes name and email)
CREATE VIRTUAL TABLE IF NOT EXISTS people_fts USING fts5(
    name,
    email,
    content='people',
    content_rowid='id'
);

-- Create FTS5 virtual table for groups (indexes name and description)
CREATE VIRTUAL TABLE IF NOT EXISTS groups_fts USING fts5(
    name,
    description,
    content='groups',
    content_rowid='id'
);

-- Create triggers to keep FTS tables in sync with people table
CREATE TRIGGER IF NOT EXISTS people_fts_insert AFTER INSERT ON people BEGIN
    INSERT INTO people_fts(rowid, name, email) VALUES (new.id, new.name, new.email);
END;

CREATE TRIGGER IF NOT EXISTS people_fts_delete AFTER DELETE ON people BEGIN
    INSERT INTO people_fts(people_fts, rowid, name, email) VALUES ('delete', old.id, old.name, old.email);
END;

CREATE TRIGGER IF NOT EXISTS people_fts_update AFTER UPDATE ON people BEGIN
    INSERT INTO people_fts(people_fts, rowid, name, email) VALUES ('delete', old.id, old.name, old.email);
    INSERT INTO people_fts(rowid, name, email) VALUES (new.id, new.name, new.email);
END;

-- Create triggers to keep FTS tables in sync with groups table
CREATE TRIGGER IF NOT EXISTS groups_fts_insert AFTER INSERT ON groups BEGIN
    INSERT INTO groups_fts(rowid, name, description) VALUES (new.id, new.name, new.description);
END;

CREATE TRIGGER IF NOT EXISTS groups_fts_delete AFTER DELETE ON groups BEGIN
    INSERT INTO groups_fts(groups_fts, rowid, name, description) VALUES ('delete', old.id, old.name, old.description);
END;

CREATE TRIGGER IF NOT EXISTS groups_fts_update AFTER UPDATE ON groups BEGIN
    INSERT INTO groups_fts(groups_fts, rowid, name, description) VALUES ('delete', old.id, old.name, old.description);
    INSERT INTO groups_fts(rowid, name, description) VALUES (new.id, new.name, new.description);
END;

-- Initial population of FTS tables (for existing data)
INSERT INTO people_fts(rowid, name, email) SELECT id, name, email FROM people WHERE NOT EXISTS (SELECT 1 FROM people_fts WHERE people_fts.rowid = people.id);
INSERT INTO groups_fts(rowid, name, description) SELECT id, name, description FROM groups WHERE NOT EXISTS (SELECT 1 FROM groups_fts WHERE groups_fts.rowid = groups.id);
