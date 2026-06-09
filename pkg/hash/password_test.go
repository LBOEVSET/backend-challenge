package hash_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/lboevset/backend-challenge/pkg/hash"
)

func TestPassword_ProducesHash(t *testing.T) {
	h, err := hash.Password("secret123")
	assert.NoError(t, err)
	assert.NotEmpty(t, h)
	assert.NotEqual(t, "secret123", h)
}

func TestCheckPassword_CorrectPassword(t *testing.T) {
	h, _ := hash.Password("secret123")
	assert.True(t, hash.CheckPassword(h, "secret123"))
}

func TestCheckPassword_WrongPassword(t *testing.T) {
	h, _ := hash.Password("secret123")
	assert.False(t, hash.CheckPassword(h, "wrongpass"))
}

func TestCheckPassword_InvalidHash(t *testing.T) {
	assert.False(t, hash.CheckPassword("not-a-hash", "secret123"))
}
