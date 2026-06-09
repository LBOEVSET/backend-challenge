// Package port defines the interfaces (ports) the application layer depends on.
// Adapters (MongoDB, in-memory, etc.) implement these interfaces.
package port

import (
	"context"

	"github.com/lboevset/backend-challenge/internal/domain"
)

// UserRepository is the persistence port for User entities.
// Any database adapter must satisfy this interface.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindAll(ctx context.Context) ([]*domain.User, error)
	Update(ctx context.Context, id string, fields domain.UpdateFields) (*domain.User, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}
