package models

// Person represents a person entity
type Person struct {
	ID          int     `json:"id" db:"id"`
	Name        string  `json:"name" db:"name"`
	Email       string  `json:"email" db:"email"`
	IsActive    int     `json:"is_active" db:"is_active"`
	JoinedDate  string  `json:"joined_date" db:"joined_date"`
	DeactivedAt *string `json:"deactived_at,omitempty" db:"deactived_at"`
	ActivatedAt *string `json:"activated_at,omitempty" db:"activated_at"`
}

// CreatePersonRequest represents the request body for creating a person
type CreatePersonRequest struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	JoinedDate string `json:"joined_date"`
}

// UpdatePersonRequest represents the request body for updating a person
type UpdatePersonRequest struct {
	Name       *string `json:"name,omitempty"`
	Email      *string `json:"email,omitempty"`
	JoinedDate *string `json:"joined_date,omitempty"`
}
