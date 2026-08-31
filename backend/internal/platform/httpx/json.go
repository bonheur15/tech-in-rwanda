package httpx

import (
	"encoding/json"
	"net/http"
	"rwandafreespace.com/blog/backend/internal/platform/requestmeta"
)

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
	Details   any    `json:"details,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	JSONMeta(w, status, data, nil)
}
func JSONMeta(w http.ResponseWriter, status int, data any, meta map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	if meta == nil {
		meta = map[string]any{}
	}
	meta["requestId"] = w.Header().Get("X-Request-ID")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": meta})
}
func Failure(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": Error{Code: code, Message: message, RequestID: requestmeta.ID(r.Context())}})
}
