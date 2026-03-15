package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/umakantv/people/models"
)

// PersonRepository handles database operations for people
type PersonRepository struct {
	db *sqlx.DB
}

// NewPersonRepository creates a new PersonRepository
func NewPersonRepository(db *sqlx.DB) *PersonRepository {
	return &PersonRepository{db: db}
}

// Create inserts a new person and returns the created person
func (r *PersonRepository) Create(req models.CreatePersonRequest) (*models.Person, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := `
		INSERT INTO people (name, email, is_active, joined_date, activated_at)
		VALUES (?, ?, 1, ?, ?)
	`
	result, err := r.db.Exec(query, req.Name, req.Email, req.JoinedDate, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(int(id))
}

// GetByID retrieves a person by ID
func (r *PersonRepository) GetByID(id int) (*models.Person, error) {
	var person models.Person
	query := `SELECT id, name, email, is_active, joined_date, deactived_at, activated_at FROM people WHERE id = ?`
	err := r.db.Get(&person, query, id)
	if err != nil {
		return nil, err
	}
	return &person, nil
}

// Update modifies a person with partial updates
func (r *PersonRepository) Update(id int, req models.UpdatePersonRequest) (*models.Person, error) {
	// Build dynamic update query
	setParts := []string{}
	args := []interface{}{}

	if req.Name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Email != nil {
		setParts = append(setParts, "email = ?")
		args = append(args, *req.Email)
	}
	if req.JoinedDate != nil {
		setParts = append(setParts, "joined_date = ?")
		args = append(args, *req.JoinedDate)
	}

	if len(setParts) == 0 {
		return r.GetByID(id)
	}

	query := fmt.Sprintf("UPDATE people SET %s WHERE id = ?", strings.Join(setParts, ", "))
	args = append(args, id)

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

// Search finds people by substring match on name or email (LIKE fallback)
func (r *PersonRepository) Search(query string) ([]models.Person, error) {
	var people []models.Person
	searchPattern := "%" + strings.ToLower(query) + "%"
	sqlQuery := `
		SELECT id, name, email, is_active, joined_date, deactived_at, activated_at
		FROM people
		WHERE LOWER(name) LIKE ? OR LOWER(email) LIKE ?
		ORDER BY id
	`
	err := r.db.Select(&people, sqlQuery, searchPattern, searchPattern)
	if err != nil {
		return nil, err
	}
	return people, nil
}

// SearchFTS searches people using FTS5 for faster, more accurate results
// Returns ranked results with exact matches prioritized
func (r *PersonRepository) SearchFTS(query string) ([]models.Person, error) {
	var people []models.Person
	// Use FTS5 MATCH with ranking (bm25) - lower score is better match
	// Prioritize name matches over email by using column weights implicitly
	sqlQuery := `
		SELECT p.id, p.name, p.email, p.is_active, p.joined_date, p.deactived_at, p.activated_at
		FROM people p
		JOIN people_fts ON p.id = people_fts.rowid
		WHERE people_fts MATCH ?
		ORDER BY bm25(people_fts, 1.0, 0.5)
		LIMIT 50
	`
	err := r.db.Select(&people, sqlQuery, query)
	if err != nil {
		return nil, err
	}
	return people, nil
}

// SearchWithFallback tries FTS search first, falls back to LIKE if FTS unavailable
func (r *PersonRepository) SearchWithFallback(query string, useFTS bool) ([]models.Person, error) {
	if useFTS {
		people, err := r.SearchFTS(query)
		if err == nil && len(people) > 0 {
			return people, nil
		}
		// FTS failed or no results, fallback to LIKE
	}
	return r.Search(query)
}

// Deactivate sets is_active=0 and deactived_at=now
func (r *PersonRepository) Deactivate(id int) (*models.Person, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := `UPDATE people SET is_active = 0, deactived_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, now, id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

// Reactivate sets is_active=1 and activated_at=now
func (r *PersonRepository) Reactivate(id int) (*models.Person, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := `UPDATE people SET is_active = 1, activated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, now, id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

// List returns all people (for future use)
func (r *PersonRepository) List() ([]models.Person, error) {
	var people []models.Person
	query := `SELECT id, name, email, is_active, joined_date, deactived_at, activated_at FROM people ORDER BY id`
	err := r.db.Select(&people, query)
	if err != nil {
		return nil, err
	}
	return people, nil
}
