// Package domain contains the core business entity and rules for User.
// It has no external dependencies — only the Go standard library.
package domain

import (
	"errors"
	"time"
)

// Role constants for the RBAC system.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User is the core business entity.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Password  string    `json:"-"` // hashed — never serialised to JSON
	CreatedAt time.Time `json:"created_at"`
}

// UpdateFields holds the subset of User fields that can be updated.
type UpdateFields struct {
	Name  *string
	Email *string
}

// Validate checks domain-level invariants.
func (u *User) Validate() error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	if u.Email == "" {
		return errors.New("email is required")
	}
	return nil
}
