// Report generation script for membership activities
// Usage: go run ./cmd/report
//
// This script generates a report of all membership additions and removals
// that occurred in the past 24 hours, grouped by person_id.

package main

import (
	"encoding/json"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/umakantv/go-utils/db"
	"github.com/umakantv/go-utils/db/migrations"
	"github.com/umakantv/go-utils/logger"

	"github.com/umakantv/people/config"
	"github.com/umakantv/people/repository"
)

func main() {
	// Initialize logger
	logger.Init(logger.LoggerConfig{
		CallerKey:  "file",
		TimeKey:    "timestamp",
		CallerSkip: 1,
	})

	// Load configuration from .env
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", logger.Any("error", err))
		os.Exit(1)
	}

	// Establish database connection
	dbConn := db.GetDBConnection(cfg.DB)
	defer dbConn.Close()

	// Run migrations to ensure schema is up to date
	if err := migrations.Migrate(dbConn, "./migrations"); err != nil {
		logger.Error("Migration failed", logger.Any("error", err))
		os.Exit(1)
	}

	// Create repository
	groupRepo := repository.NewGroupRepository(dbConn)

	// Generate report
	report, err := groupRepo.GetMembershipReport()
	if err != nil {
		logger.Error("Failed to generate report", logger.Any("error", err))
		os.Exit(1)
	}

	// Output report as JSON
	output := map[string]interface{}{
		"report_type": "membership_activity_past_day",
		"total_affected_people": len(report),
		"activities_by_person": report,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal report", logger.Any("error", err))
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}
