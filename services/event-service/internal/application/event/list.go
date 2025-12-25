package event

import (
	"context"
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
)

type ListFilter struct {
	City     string
	Query    string
	Category string
	From     *time.Time
	To       *time.Time

	Page     int
	PageSize int
	Sort     string // "time" for now
}

func (f *ListFilter) Normalize() error {
	f.City = strings.TrimSpace(f.City)
	f.Query = strings.TrimSpace(f.Query)
	f.Category = strings.TrimSpace(f.Category)

	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	if f.Sort == "" {
		f.Sort = "time"
	}
	if f.Sort != "time" {
		return domain.ErrValidation("unsupported sort (use time)")
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		return domain.ErrValidation("to must be >= from")
	}
	return nil
}

func (s *Service) ListPublic(ctx context.Context, f ListFilter) ([]*domain.Event, int, error) {
	if err := f.Normalize(); err != nil {
		return nil, 0, err
	}
	return s.repo.ListPublic(ctx, f)
}

func (s *Service) ListMyEvents(ctx context.Context, actorID, actorRole string, page, pageSize int) ([]*domain.Event, int, error) {
	if actorID == "" {
		return nil, 0, domain.ErrForbidden("not allowed")
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.repo.ListByOwner(ctx, actorID, page, pageSize)
}
