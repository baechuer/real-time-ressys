package event

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	zlog "github.com/rs/zerolog/log"
)

type ListFilter struct {
	City     string
	Query    string // q
	Category string
	From     *time.Time
	To       *time.Time

	PageSize int

	Sort   string // time | relevance
	Cursor string // time: "time|uuid"; relevance: "rank|time|uuid"
}

func (f *ListFilter) Normalize() error {
	f.City = strings.TrimSpace(f.City)
	f.Query = strings.TrimSpace(f.Query)
	f.Category = strings.TrimSpace(f.Category)
	f.Sort = strings.TrimSpace(f.Sort)
	f.Cursor = strings.TrimSpace(f.Cursor)

	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}

	if f.Sort == "" {
		f.Sort = "time"
	}
	if f.Sort != "time" && f.Sort != "relevance" {
		return domain.ErrValidationMeta("invalid query param", map[string]string{
			"sort": "must be one of: time, relevance",
		})
	}
	if f.Sort == "relevance" && f.Query == "" {
		return domain.ErrValidationMeta("invalid query param", map[string]string{
			"q": "required when sort=relevance",
		})
	}
	if f.From != nil && f.To != nil && f.To.Before(*f.From) {
		return domain.ErrValidation("to must be >= from")
	}
	return nil
}

type PublicListResult struct {
	Items      []*domain.Event
	NextCursor string
}

func (s *Service) ListPublic(ctx context.Context, f ListFilter) (PublicListResult, error) {
	if err := f.Normalize(); err != nil {
		return PublicListResult{}, err
	}

	// --- Caching Strategy: Only Cache "First Page" ---
	// "First Page" definition: No cursor.
	// We ignore PageSize variations in the "cache logic" check, but include it in the key.
	isFirstPage := f.Cursor == ""
	cacheKey := ""

	if isFirstPage && s.cache != nil {
		cacheKey = cacheKeyPublicList(f)
		var cached PublicListResult
		found, err := s.cache.Get(ctx, cacheKey, &cached)
		if err != nil {
			zlog.Warn().Err(err).Str("key", cacheKey).Msg("cache list get failed")
		} else if found {
			zlog.Debug().Str("key", cacheKey).Msg("cache list hit")
			return cached, nil
		}
	}

	// --- DB Logic ---
	var res PublicListResult

	switch f.Sort {
	case "time":
		afterStart, afterID, hasCursor, err := parseTimeCursorOrEmpty(f.Cursor)
		if err != nil {
			return PublicListResult{}, err
		}
		items, err := s.repo.ListPublicTimeKeyset(ctx, f, hasCursor, afterStart, afterID)
		if err != nil {
			return PublicListResult{}, err
		}

		next := ""
		if len(items) > 0 {
			last := items[len(items)-1]
			next = formatTimeCursor(last.StartTime.UTC(), last.ID)
		}
		res = PublicListResult{Items: items, NextCursor: next}

	case "relevance":
		afterRank, afterStart, afterID, hasCursor, err := parseRelevanceCursorOrEmpty(f.Cursor)
		if err != nil {
			return PublicListResult{}, err
		}
		items, ranks, err := s.repo.ListPublicRelevanceKeyset(ctx, f, hasCursor, afterRank, afterStart, afterID)
		if err != nil {
			return PublicListResult{}, err
		}

		next := ""
		if len(items) > 0 {
			last := items[len(items)-1]
			lastRank := ranks[len(ranks)-1]
			next = formatRelevanceCursor(lastRank, last.StartTime.UTC(), last.ID)
		}
		res = PublicListResult{Items: items, NextCursor: next}

	default:
		return PublicListResult{}, domain.ErrValidation("unsupported sort")
	}

	// --- Set Cache ---
	if isFirstPage && s.cache != nil && len(res.Items) > 0 {
		if err := s.cache.Set(ctx, cacheKey, res, s.ttlList); err != nil {
			zlog.Warn().Err(err).Str("key", cacheKey).Msg("cache list set failed")
		}
	}

	return res, nil
}

func (s *Service) ListMyEvents(ctx context.Context, actorID, actorRole string, page, pageSize int) ([]*domain.Event, int, error) {
	if strings.TrimSpace(actorID) == "" {
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

// -------- cursor helpers --------

func formatTimeCursor(t time.Time, id string) string {
	return t.Format(time.RFC3339Nano) + "|" + id
}

func parseTimeCursorOrEmpty(cur string) (time.Time, string, bool, error) {
	cur = strings.TrimSpace(cur)
	if cur == "" {
		return time.Time{}, "", false, nil
	}
	parts := strings.Split(cur, "|")
	if len(parts) != 2 {
		return time.Time{}, "", false, domain.ErrValidation("invalid cursor (expected time|uuid)")
	}
	t, err := parseRFC3339OrNano(parts[0])
	if err != nil {
		return time.Time{}, "", false, domain.ErrValidation("invalid cursor (expected time|uuid)")
	}
	id := strings.TrimSpace(parts[1])
	if id == "" {
		return time.Time{}, "", false, domain.ErrValidation("invalid cursor (expected time|uuid)")
	}
	return t.UTC(), id, true, nil
}

func formatRelevanceCursor(rank float64, t time.Time, id string) string {
	// keep cursor stable (8dp)
	return strconv.FormatFloat(rank, 'f', 8, 64) + "|" + t.Format(time.RFC3339Nano) + "|" + id
}

func parseRelevanceCursorOrEmpty(cur string) (float64, time.Time, string, bool, error) {
	cur = strings.TrimSpace(cur)
	if cur == "" {
		return 0, time.Time{}, "", false, nil
	}
	parts := strings.Split(cur, "|")
	if len(parts) != 3 {
		return 0, time.Time{}, "", false, domain.ErrValidation("invalid cursor (expected rank|time|uuid)")
	}

	rk, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, time.Time{}, "", false, domain.ErrValidation("invalid cursor (expected rank|time|uuid)")
	}
	t, err := parseRFC3339OrNano(parts[1])
	if err != nil {
		return 0, time.Time{}, "", false, domain.ErrValidation("invalid cursor (expected rank|time|uuid)")
	}
	id := strings.TrimSpace(parts[2])
	if id == "" {
		return 0, time.Time{}, "", false, domain.ErrValidation("invalid cursor (expected rank|time|uuid)")
	}
	return rk, t.UTC(), id, true, nil
}

func parseRFC3339OrNano(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
