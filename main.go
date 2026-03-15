package main

import (
	"context"
	"encoding/json"
	"net/http"

	_ "github.com/mattn/go-sqlite3"

	"github.com/umakantv/go-utils/db"
	"github.com/umakantv/go-utils/db/migrations"
	"github.com/umakantv/go-utils/httpserver"
	"github.com/umakantv/go-utils/logger"

	"github.com/umakantv/people/config"
	"github.com/umakantv/people/handlers"
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
		panic(err)
	}
	logger.Info("Configuration loaded")

	// Establish database connection
	dbConn := db.GetDBConnection(cfg.DB)
	defer dbConn.Close()
	logger.Info("Database connected", logger.Any("driver", cfg.DB.DRIVER), logger.Any("db", cfg.DB.DB))

	// Run migrations
	if err := migrations.Migrate(dbConn, "./migrations"); err != nil {
		logger.Error("Migration failed", logger.Any("error", err))
		panic(err)
	}
	logger.Info("Migrations completed")

	// Create repositories and handlers
	personRepo := repository.NewPersonRepository(dbConn)
	personHandler := handlers.NewPersonHandler(personRepo)
	groupRepo := repository.NewGroupRepository(dbConn)
	groupHandler := handlers.NewGroupHandler(groupRepo, personRepo)

	// Create server with auth callback (nil since we only have "none" routes)
	server := httpserver.New("8080", nil)

	// Register health endpoint
	server.Register(httpserver.Route{
		Name:     "HealthCheck",
		Method:   "GET",
		Path:     "/health",
		AuthType: "none",
	}, httpserver.HandlerFunc(healthCheckHandler))

	// Register person endpoints
	server.Register(httpserver.Route{
		Name:     "CreatePerson",
		Method:   "POST",
		Path:     "/people",
		AuthType: "none",
	}, httpserver.HandlerFunc(personHandler.Create))

	server.Register(httpserver.Route{
		Name:     "SearchPeople",
		Method:   "GET",
		Path:     "/people",
		AuthType: "none",
	}, httpserver.HandlerFunc(personHandler.Search))

	server.Register(httpserver.Route{
		Name:     "UpdatePerson",
		Method:   "PUT",
		Path:     "/people/{id}",
		AuthType: "none",
	}, httpserver.HandlerFunc(personHandler.Update))

	server.Register(httpserver.Route{
		Name:     "DeactivatePerson",
		Method:   "POST",
		Path:     "/people/{id}/deactivate",
		AuthType: "none",
	}, httpserver.HandlerFunc(personHandler.Deactivate))

	server.Register(httpserver.Route{
		Name:     "ReactivatePerson",
		Method:   "POST",
		Path:     "/people/{id}/reactivate",
		AuthType: "none",
	}, httpserver.HandlerFunc(personHandler.Reactivate))

	// Register group endpoints
	server.Register(httpserver.Route{
		Name:     "CreateGroup",
		Method:   "POST",
		Path:     "/groups",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.Create))

	server.Register(httpserver.Route{
		Name:     "UpdateGroup",
		Method:   "PUT",
		Path:     "/groups/{id}",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.Update))

	server.Register(httpserver.Route{
		Name:     "AddGroupMember",
		Method:   "POST",
		Path:     "/groups/{id}/members",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.AddMember))

	server.Register(httpserver.Route{
		Name:     "GetGroupDirectMembers",
		Method:   "GET",
		Path:     "/groups/{id}/members/direct",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.GetDirectMembers))

	server.Register(httpserver.Route{
		Name:     "CheckGroupMember",
		Method:   "GET",
		Path:     "/groups/{id}/members/{personId}/check",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.IsMember))

	server.Register(httpserver.Route{
		Name:     "AddGroupSubgroup",
		Method:   "POST",
		Path:     "/groups/{id}/subgroups",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.AddSubgroup))

	server.Register(httpserver.Route{
		Name:     "RemoveGroupSubgroup",
		Method:   "DELETE",
		Path:     "/groups/{id}/subgroups/{subgroupId}",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.RemoveSubgroup))

	server.Register(httpserver.Route{
		Name:     "RemoveGroupMember",
		Method:   "DELETE",
		Path:     "/groups/{id}/members/{personId}",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.RemoveMember))

	server.Register(httpserver.Route{
		Name:     "BulkGroupMembers",
		Method:   "POST",
		Path:     "/groups/{id}/bulk-members",
		AuthType: "none",
	}, httpserver.HandlerFunc(groupHandler.BulkMembers))

	// Create search handler with access to both repos and config
	searchHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "query parameter 'q' is required"})
			return
		}

		// Sanitize query - limit length
		if len(query) > 200 {
			query = query[:200]
		}

		result, err := groupRepo.SearchAll(query, personRepo, cfg.UseFTS)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "search failed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query":  query,
			"fts":    cfg.UseFTS,
			"people": result.People,
			"groups": result.Groups,
			"total":  len(result.People) + len(result.Groups),
		})
	}

	server.Register(httpserver.Route{
		Name:     "UnifiedSearch",
		Method:   "GET",
		Path:     "/search",
		AuthType: "none",
	}, httpserver.HandlerFunc(searchHandler))

	logger.Info("Starting people service on port 8080")
	if err := server.Start(); err != nil {
		logger.Error("Server failed to start", logger.Any("error", err))
	}
}

func healthCheckHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}
