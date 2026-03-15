package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/umakantv/go-utils/logger"
	"github.com/umakantv/people/models"
	"github.com/umakantv/people/repository"
)

// PersonHandler handles HTTP requests for people
type PersonHandler struct {
	repo *repository.PersonRepository
}

// NewPersonHandler creates a new PersonHandler
func NewPersonHandler(repo *repository.PersonRepository) *PersonHandler {
	return &PersonHandler{repo: repo}
}

// Create handles POST /people
func (h *PersonHandler) Create(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req models.CreatePersonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.Name == "" || req.Email == "" || req.JoinedDate == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "name, email, and joined_date are required"})
		return
	}

	person, err := h.repo.Create(req)
	if err != nil {
		logger.Error("Failed to create person", logger.Any("error", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create person"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(person)
}

// Search handles GET /people?q=...
func (h *PersonHandler) Search(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var people []models.Person
	var err error

	if query == "" {
		// Return all people if no query
		people, err = h.repo.List()
	} else {
		people, err = h.repo.Search(query)
	}

	if err != nil {
		logger.Error("Failed to search people", logger.Any("error", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to search people"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(people)
}

// Update handles PUT /people/:id
func (h *PersonHandler) Update(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid id"})
		return
	}

	var req models.UpdatePersonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}

	person, err := h.repo.Update(id, req)
	if err != nil {
		logger.Error("Failed to update person", logger.Any("error", err), logger.Any("id", id))
		w.Header().Set("Content-Type", "application/json")
		if person == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "person not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to update person"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(person)
}

// Deactivate handles POST /people/:id/deactivate
func (h *PersonHandler) Deactivate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid id"})
		return
	}

	person, err := h.repo.Deactivate(id)
	if err != nil {
		logger.Error("Failed to deactivate person", logger.Any("error", err), logger.Any("id", id))
		w.Header().Set("Content-Type", "application/json")
		if person == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "person not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to deactivate person"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(person)
}

// Reactivate handles POST /people/:id/reactivate
func (h *PersonHandler) Reactivate(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid id"})
		return
	}

	person, err := h.repo.Reactivate(id)
	if err != nil {
		logger.Error("Failed to reactivate person", logger.Any("error", err), logger.Any("id", id))
		w.Header().Set("Content-Type", "application/json")
		if person == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "person not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to reactivate person"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(person)
}
