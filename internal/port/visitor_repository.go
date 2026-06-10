package port

import (
	"context"

	"github.com/lboevset/backend-challenge/internal/domain"
)

// VisitorRepository persists visitor records.
// Upsert inserts on first visit, updates on repeat visits.
type VisitorRepository interface {
	Upsert(ctx context.Context, v *domain.VisitorRecord) error
}
