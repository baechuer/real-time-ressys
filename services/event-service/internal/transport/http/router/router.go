package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/handlers"
	authmw "github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/middleware"
)

func New(h *handlers.EventsHandler, auth *authmw.AuthMiddleware, z *handlers.HealthHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(authmw.AccessLog)
	r.Get("/healthz", z.Healthz)

	r.Route("/event/v1", func(r chi.Router) {
		// Public
		r.Get("/events", h.ListPublic)
		r.Get("/events/{event_id}", h.GetPublic)

		// Organizer
		r.Group(func(r chi.Router) {
			r.Use(auth.Require)
			r.Post("/events", h.Create)
			r.Patch("/events/{event_id}", h.Update)
			r.Post("/events/{event_id}/publish", h.Publish)
			r.Post("/events/{event_id}/cancel", h.Cancel)
			r.Get("/organizer/events", h.ListMine)
		})
	})

	return r
}
