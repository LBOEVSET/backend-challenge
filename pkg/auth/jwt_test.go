package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/lboevset/backend-challenge/pkg/auth"
)

const secret = "test-secret"

func TestGenerateToken_ReturnsNonEmptyToken(t *testing.T) {
	token, err := auth.GenerateToken("user-123", secret)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestValidateToken_RoundTrip(t *testing.T) {
	token, err := auth.GenerateToken("user-abc", secret)
	assert.NoError(t, err)

	claims, err := auth.ValidateToken(token, secret)
	assert.NoError(t, err)
	assert.Equal(t, "user-abc", claims.UserID)
}

func TestValidateToken_WrongSecret(t *testing.T) {
	token, _ := auth.GenerateToken("user-abc", secret)
	_, err := auth.ValidateToken(token, "wrong-secret")
	assert.Error(t, err)
}

func TestValidateToken_MalformedToken(t *testing.T) {
	_, err := auth.ValidateToken("not.a.token", secret)
	assert.Error(t, err)
}

func TestValidateToken_EmptyToken(t *testing.T) {
	_, err := auth.ValidateToken("", secret)
	assert.Error(t, err)
}
