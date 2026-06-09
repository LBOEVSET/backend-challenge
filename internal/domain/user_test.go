package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/lboevset/backend-challenge/internal/domain"
)

func TestValidate_ValidUser(t *testing.T) {
	u := &domain.User{Name: "Alice", Email: "alice@example.com"}
	assert.NoError(t, u.Validate())
}

func TestValidate_MissingName(t *testing.T) {
	u := &domain.User{Email: "alice@example.com"}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestValidate_MissingEmail(t *testing.T) {
	u := &domain.User{Name: "Alice"}
	err := u.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}
