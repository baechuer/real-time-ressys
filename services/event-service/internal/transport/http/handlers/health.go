package handlers

import (
	"net/http"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/response"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	response.Data(w, http.StatusOK, map[string]string{"status": "ok"})
}
