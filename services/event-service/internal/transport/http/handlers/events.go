package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/application/event"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/dto"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/middleware"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/response"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/validate"
)

type Clock interface{ Now() time.Time }

type EventsHandler struct {
	svc   *event.Service
	clock Clock
}

func NewEventsHandler(svc *event.Service, clock Clock) *EventsHandler {
	return &EventsHandler{svc: svc, clock: clock}
}

// Public
func (h *EventsHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	sort := q.Get("sort")
	switch sort {
	case "", "time", "popularity":
	default:
		response.Err(w, domain.ErrValidationMeta("invalid query param", map[string]string{
			"sort": "must be one of: time, popularity",
		}))
		return
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

	var fromPtr, toPtr *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			response.Err(w, domain.ErrValidationMeta("invalid query param", map[string]string{
				"from": "must be RFC3339 timestamp",
			}))
			return
		}
		tt := t.UTC()
		fromPtr = &tt
	}

	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			response.Err(w, domain.ErrValidationMeta(
				"invalid query param",
				map[string]string{"to": "must be RFC3339"},
			))
			return
		}
		tt := t.UTC()
		toPtr = &tt
	}

	filter := event.ListFilter{
		City:     q.Get("city"),
		Query:    q.Get("q"),
		Category: q.Get("category"),
		From:     fromPtr,
		To:       toPtr,
		Page:     page,
		PageSize: pageSize,
		Sort:     sort,
	}

	items, total, err := h.svc.ListPublic(r.Context(), filter)
	if err != nil {
		response.Err(w, err)
		return
	}
	now := h.clock.Now().UTC()
	out := make([]dto.EventResp, 0, len(items))
	for _, it := range items {
		out = append(out, dto.ToEventResp(it, now))
	}

	response.Data(w, http.StatusOK, dto.PageResp[dto.EventResp]{
		Items:    out,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	})
}

func (h *EventsHandler) GetPublic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "event_id")
	if !validate.IsUUID(id) {
		response.Err(w, domain.ErrValidationMeta("invalid path param", map[string]string{
			"event_id": "must be uuid",
		}))
		return
	}
	ev, err := h.svc.GetPublic(r.Context(), id)
	if err != nil {
		response.Err(w, err)
		return
	}
	now := h.clock.Now().UTC()
	response.Data(w, http.StatusOK, dto.ToEventResp(ev, now))
}

// Organizer
func (h *EventsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateEventReq
	if err := validate.DecodeJSON(r, &req); err != nil {
		// If validate.DecodeJSON returns plain error, wrap it as validation_error
		response.Err(w, domain.ErrValidationMeta("invalid json body", map[string]string{
			"body": "malformed JSON or invalid fields",
		}))
		return
	}
	cmd := event.CreateCmd{
		ActorID:     middleware.UserID(r),
		ActorRole:   middleware.Role(r),
		Title:       req.Title,
		Description: req.Description,
		City:        req.City,
		Category:    req.Category,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Capacity:    req.Capacity,
	}
	ev, err := h.svc.Create(r.Context(), cmd)
	if err != nil {
		response.Err(w, err)
		return
	}
	now := h.clock.Now().UTC()
	response.Data(w, http.StatusCreated, dto.ToEventResp(ev, now))
}

func (h *EventsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "event_id")
	if !validate.IsUUID(id) {
		response.Err(w, domain.ErrValidationMeta("invalid path param", map[string]string{
			"event_id": "must be uuid",
		}))
		return
	}

	var req dto.UpdateEventReq
	if err := validate.DecodeJSON(r, &req); err != nil {
		response.Err(w, domain.ErrValidationMeta("invalid json body", map[string]string{
			"body": "malformed JSON or invalid fields",
		}))
		return
	}

	cmd := event.UpdateCmd{
		ActorID:     middleware.UserID(r),
		ActorRole:   middleware.Role(r),
		EventID:     id,
		Title:       req.Title,
		Description: req.Description,
		City:        req.City,
		Category:    req.Category,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Capacity:    req.Capacity,
	}

	ev, err := h.svc.Update(r.Context(), cmd)
	if err != nil {
		response.Err(w, err)
		return
	}

	now := h.clock.Now().UTC()
	response.Data(w, http.StatusOK, dto.ToEventResp(ev, now))
}

func (h *EventsHandler) Publish(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "event_id")
	if !validate.IsUUID(id) {
		response.Err(w, domain.ErrValidationMeta("invalid path param", map[string]string{
			"event_id": "must be uuid",
		}))
		return
	}

	ev, err := h.svc.Publish(r.Context(), id, middleware.UserID(r), middleware.Role(r))
	if err != nil {
		response.Err(w, err)
		return
	}

	now := h.clock.Now().UTC()
	response.Data(w, http.StatusOK, dto.ToEventResp(ev, now))
}

func (h *EventsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "event_id")
	if !validate.IsUUID(id) {
		response.Err(w, domain.ErrValidationMeta("invalid path param", map[string]string{
			"event_id": "must be uuid",
		}))
		return
	}

	ev, err := h.svc.Cancel(r.Context(), id, middleware.UserID(r), middleware.Role(r))
	if err != nil {
		response.Err(w, err)
		return
	}

	now := h.clock.Now().UTC()
	response.Data(w, http.StatusOK, dto.ToEventResp(ev, now))
}

func (h *EventsHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, total, err := h.svc.ListMyEvents(r.Context(), middleware.UserID(r), middleware.Role(r), page, pageSize)
	if err != nil {
		response.Err(w, err)
		return
	}
	now := h.clock.Now().UTC()
	out := make([]dto.EventResp, 0, len(items))
	for _, it := range items {
		out = append(out, dto.ToEventResp(it, now))
	}

	response.Data(w, http.StatusOK, dto.PageResp[dto.EventResp]{
		Items:    out,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}
