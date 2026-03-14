package testhelpers

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"
	"github.com/umakantv/go-utils/logger"
)

// init ensures logger is initialized for tests
func init() {
	logger.Init(logger.LoggerConfig{
		CallerKey:  "file",
		TimeKey:    "timestamp",
		CallerSkip: 1,
	})
}

// SetupTestDB creates an in-memory SQLite database with the people schema
func SetupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping db: %v", err)
	}

	// Create schema directly (matches migrations)
	schema := `
		CREATE TABLE IF NOT EXISTS people (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			is_active INTEGER NOT NULL DEFAULT 1,
			joined_date TEXT NOT NULL,
			deactived_at TEXT NULL,
			activated_at TEXT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_people_email ON people(email);
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
		CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
		CREATE TABLE IF NOT EXISTS person_group_memberships (
			person_id INTEGER NOT NULL,
			group_id INTEGER NOT NULL,
			added_by INTEGER NULL,
			PRIMARY KEY (person_id, group_id),
			FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE,
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_pgm_person ON person_group_memberships(person_id);
		CREATE INDEX IF NOT EXISTS idx_pgm_group ON person_group_memberships(group_id);
		CREATE TABLE IF NOT EXISTS group_subgroups (
			parent_group_id INTEGER NOT NULL,
			child_group_id INTEGER NOT NULL,
			PRIMARY KEY (parent_group_id, child_group_id),
			FOREIGN KEY (parent_group_id) REFERENCES groups(id) ON DELETE CASCADE,
			FOREIGN KEY (child_group_id) REFERENCES groups(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_gs_parent ON group_subgroups(parent_group_id);
		CREATE INDEX IF NOT EXISTS idx_gs_child ON group_subgroups(child_group_id);
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

// CloseDB closes the database connection
func CloseDB(t *testing.T, db *sqlx.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("failed to close db: %v", err)
	}
}
