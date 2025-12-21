package response

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	Data any `json:"data"`
}

// WriteJSON writes v as JSON with the given status code.
// It sets Content-Type to application/json; charset=utf-8 if not already set.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// OK writes a 200 response with {"data": ...}.
func OK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, Envelope{Data: data})
}

// Created writes a 201 response with {"data": ...}.
func Created(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, Envelope{Data: data})
}

// NoContent writes a 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
