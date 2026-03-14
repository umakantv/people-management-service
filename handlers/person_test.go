package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/umakantv/people/models"
	"github.com/umakantv/people/repository"
	"github.com/umakantv/people/testhelpers"
)

// setupTestHandler creates a handler with in-memory DB for testing
func setupTestHandler(t *testing.T) (*PersonHandler, *repository.PersonRepository) {
	t.Helper()
	db := testhelpers.SetupTestDB(t)
	t.Cleanup(func() { testhelpers.CloseDB(t, db) })
	repo := repository.NewPersonRepository(db)
	return NewPersonHandler(repo), repo
}

func TestCreate_Success(t *testing.T) {
	handler, _ := setupTestHandler(t)

	body := `{"name":"John Doe","email":"john@example.com","joined_date":"2024-01-15"}`
	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var person models.Person
	if err := json.Unmarshal(rec.Body.Bytes(), &person); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if person.ID == 0 {
		t.Error("expected ID to be set")
	}
	if person.Name != "John Doe" {
		t.Errorf("expected name 'John Doe', got '%s'", person.Name)
	}
	if person.IsActive != 1 {
		t.Errorf("expected is_active=1, got %d", person.IsActive)
	}
}

func TestCreate_MissingFields(t *testing.T) {
	handler, _ := setupTestHandler(t)

	testCases := []struct {
		name string
		body string
	}{
		{"missing all", `{}`},
		{"missing email", `{"name":"John","joined_date":"2024-01-15"}`},
		{"missing name", `{"email":"john@example.com","joined_date":"2024-01-15"}`},
		{"missing joined_date", `{"name":"John","email":"john@example.com"}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Create(context.Background(), rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestCreate_InvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSearch_WithQuery(t *testing.T) {
	handler, _ := setupTestHandler(t)

	// Create test data
	handler.repo.Create(models.CreatePersonRequest{Name: "John Doe", Email: "john@example.com", JoinedDate: "2024-01-01"})
	handler.repo.Create(models.CreatePersonRequest{Name: "Jane Smith", Email: "jane@example.com", JoinedDate: "2024-01-01"})
	handler.repo.Create(models.CreatePersonRequest{Name: "Bob Johnson", Email: "bob@test.com", JoinedDate: "2024-01-01"})

	// Search by name substring
	req := httptest.NewRequest(http.MethodGet, "/people?q=john", nil)
	rec := httptest.NewRecorder()

	handler.Search(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var people []models.Person
	if err := json.Unmarshal(rec.Body.Bytes(), &people); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Should find "John Doe" (name) and "Bob Johnson" (name substring)
	if len(people) != 2 {
		t.Errorf("expected 2 results, got %d", len(people))
	}
}

func TestSearch_ByEmail(t *testing.T) {
	handler, _ := setupTestHandler(t)

	handler.repo.Create(models.CreatePersonRequest{Name: "John", Email: "john@example.com", JoinedDate: "2024-01-01"})
	handler.repo.Create(models.CreatePersonRequest{Name: "Jane", Email: "jane@test.org", JoinedDate: "2024-01-01"})

	req := httptest.NewRequest(http.MethodGet, "/people?q=example.com", nil)
	rec := httptest.NewRecorder()

	handler.Search(context.Background(), rec, req)

	var people []models.Person
	json.Unmarshal(rec.Body.Bytes(), &people)

	if len(people) != 1 {
		t.Errorf("expected 1 result for email search, got %d", len(people))
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	handler, _ := setupTestHandler(t)

	handler.repo.Create(models.CreatePersonRequest{Name: "JOHN DOE", Email: "john@example.com", JoinedDate: "2024-01-01"})

	req := httptest.NewRequest(http.MethodGet, "/people?q=john", nil)
	rec := httptest.NewRecorder()

	handler.Search(context.Background(), rec, req)

	var people []models.Person
	json.Unmarshal(rec.Body.Bytes(), &people)

	if len(people) != 1 {
		t.Errorf("expected 1 result for case-insensitive search, got %d", len(people))
	}
}

func TestSearch_EmptyQueryListsAll(t *testing.T) {
	handler, _ := setupTestHandler(t)

	handler.repo.Create(models.CreatePersonRequest{Name: "A", Email: "a@test.com", JoinedDate: "2024-01-01"})
	handler.repo.Create(models.CreatePersonRequest{Name: "B", Email: "b@test.com", JoinedDate: "2024-01-01"})

	req := httptest.NewRequest(http.MethodGet, "/people", nil)
	rec := httptest.NewRecorder()

	handler.Search(context.Background(), rec, req)

	var people []models.Person
	json.Unmarshal(rec.Body.Bytes(), &people)

	if len(people) != 2 {
		t.Errorf("expected 2 results when listing all, got %d", len(people))
	}
}

func TestUpdate_Success(t *testing.T) {
	handler, _ := setupTestHandler(t)

	// Create a person
	created, _ := handler.repo.Create(models.CreatePersonRequest{Name: "John", Email: "john@test.com", JoinedDate: "2024-01-01"})

	// Update
	body := `{"name":"John Updated","email":"john.updated@test.com"}`
	req := httptest.NewRequest(http.MethodPut, "/people/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()

	handler.Update(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var person models.Person
	json.Unmarshal(rec.Body.Bytes(), &person)

	if person.Name != "John Updated" {
		t.Errorf("expected name 'John Updated', got '%s'", person.Name)
	}
	if person.ID != created.ID {
		t.Errorf("ID should not change")
	}
}

func TestUpdate_PartialUpdate(t *testing.T) {
	handler, _ := setupTestHandler(t)

	handler.repo.Create(models.CreatePersonRequest{Name: "John", Email: "john@test.com", JoinedDate: "2024-01-01"})

	// Only update name
	body := `{"name":"John New"}`
	req := httptest.NewRequest(http.MethodPut, "/people/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()

	handler.Update(context.Background(), rec, req)

	var person models.Person
	json.Unmarshal(rec.Body.Bytes(), &person)

	if person.Name != "John New" {
		t.Errorf("name should be updated")
	}
	if person.Email != "john@test.com" {
		t.Errorf("email should remain unchanged, got '%s'", person.Email)
	}
}

func TestUpdate_NonExisting(t *testing.T) {
	handler, _ := setupTestHandler(t)

	body := `{"name":"Test"}`
	req := httptest.NewRequest(http.MethodPut, "/people/999", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()

	handler.Update(context.Background(), rec, req)

	// Should fail because GetByID returns error for non-existing
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for non-existing, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestUpdate_InvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	body := `{"name":"Test"}`
	req := httptest.NewRequest(http.MethodPut, "/people/abc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	rec := httptest.NewRecorder()

	handler.Update(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid id, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestDeactivate_Success(t *testing.T) {
	handler, _ := setupTestHandler(t)

	handler.repo.Create(models.CreatePersonRequest{Name: "John", Email: "john@test.com", JoinedDate: "2024-01-01"})

	req := httptest.NewRequest(http.MethodPost, "/people/1/deactivate", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()

	handler.Deactivate(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var person models.Person
	json.Unmarshal(rec.Body.Bytes(), &person)

	if person.IsActive != 0 {
		t.Errorf("expected is_active=0, got %d", person.IsActive)
	}
	if person.DeactivedAt == nil {
		t.Error("expected deactived_at to be set")
	}
}

func TestDeactivate_NonExisting(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/people/999/deactivate", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()

	handler.Deactivate(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for non-existing, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestDeactivate_InvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/people/xyz/deactivate", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "xyz"})
	rec := httptest.NewRecorder()

	handler.Deactivate(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid id, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestReactivate_Success(t *testing.T) {
	handler, _ := setupTestHandler(t)

	// Create and deactivate first
	handler.repo.Create(models.CreatePersonRequest{Name: "John", Email: "john@test.com", JoinedDate: "2024-01-01"})
	handler.repo.Deactivate(1)

	req := httptest.NewRequest(http.MethodPost, "/people/1/reactivate", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()

	handler.Reactivate(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var person models.Person
	json.Unmarshal(rec.Body.Bytes(), &person)

	if person.IsActive != 1 {
		t.Errorf("expected is_active=1, got %d", person.IsActive)
	}
	if person.ActivatedAt == nil {
		t.Error("expected activated_at to be set")
	}
}

func TestReactivate_NonExisting(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/people/999/reactivate", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()

	handler.Reactivate(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for non-existing, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestReactivate_InvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/people/abc/reactivate", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	rec := httptest.NewRecorder()

	handler.Reactivate(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid id, got %d", http.StatusBadRequest, rec.Code)
	}
}
