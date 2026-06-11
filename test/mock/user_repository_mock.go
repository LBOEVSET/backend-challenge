// Package mock provides test doubles for port.UserRepository.
package mock

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/lboevset/backend-challenge/internal/domain"
)

// UserRepository is a mock that satisfies port.UserRepository.
// Generated with testify/mock conventions.
type UserRepository struct {
	mock.Mock
}

func (m *UserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *UserRepository) FindAll(ctx context.Context, limit, offset int64) ([]*domain.User, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *UserRepository) Update(ctx context.Context, id string, fields domain.UpdateFields) (*domain.User, error) {
	args := m.Called(ctx, id, fields)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *UserRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *UserRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
