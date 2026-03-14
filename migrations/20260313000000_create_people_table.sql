-- Migration: create_people_table
-- Generated: 20260313000000 UTC

-- Create people table
CREATE TABLE IF NOT EXISTS people (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    is_active INTEGER NOT NULL DEFAULT 1,
    joined_date TEXT NOT NULL
);

-- Create index on email for lookups
CREATE INDEX IF NOT EXISTS idx_people_email ON people(email);
