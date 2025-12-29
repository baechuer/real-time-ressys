package response

import (
	"encoding/json"
	"net/http"
)

// Envelope is the success envelope:
// {"data": ...}
type Envelope struct {
	Data any `json:"data,omitempty"`
}

// ErrorBody matches auth-service style:
// {"error":{"code":"...","message":"...","meta":{...},"request_id":"..."}}
type ErrorBody struct {
	Error ErrorPayload `json:"error"`
}

type ErrorPayload struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Meta      map[string]string `json:"meta,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

// JSON writes raw JSON with Content-Type.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Data wraps payload with {"data": ...}
func Data(w http.ResponseWriter, status int, payload any) {
	JSON(w, status, Envelope{Data: payload})
}

// Fail writes error body:
// {"error":{"code":"...","message":"...","meta":{...},"request_id":"..."}}
func Fail(w http.ResponseWriter, status int, code, message string, meta map[string]string, requestID string) {
	JSON(w, status, ErrorBody{
		Error: ErrorPayload{
			Code:      code,
			Message:   message,
			Meta:      meta,
			RequestID: requestID,
		},
	})
}
