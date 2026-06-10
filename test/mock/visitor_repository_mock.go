package mock

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/lboevset/backend-challenge/internal/domain"
)

// VisitorRepository is a testify mock for port.VisitorRepository.
type VisitorRepository struct {
	mock.Mock
}

func (m *VisitorRepository) Upsert(ctx context.Context, v *domain.VisitorRecord) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}
