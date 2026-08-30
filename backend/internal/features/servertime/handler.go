package servertime

import (
	"encoding/json"
	"net/http"
	"time"
	_ "time/tzdata"

	"rwandafreespace.com/blog/backend/internal/platform/requestmeta"
)

type Clock func() time.Time

type Handler struct {
	now      Clock
	location *time.Location
}

func NewHandler(now Clock) *Handler {
	if now == nil {
		now = time.Now
	}

	location, err := time.LoadLocation(TimeZone)
	if err != nil {
		panic("load embedded Kigali time zone: " + err.Error())
	}

	return &Handler{now: now, location: location}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: APIError{
			Code:      "method_not_allowed",
			Message:   "only GET is supported",
			RequestID: requestmeta.ID(r.Context()),
		}})
		return
	}

	now := h.now().In(h.location)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, Response{
		Data: Data{
			ISO:              now.Format(time.RFC3339Nano),
			Display:          now.Format("Monday, 2 January 2006 at 15:04:05 MST"),
			TimeZone:         TimeZone,
			UnixMilliseconds: now.UnixMilli(),
		},
		Meta: Meta{RequestID: requestmeta.ID(r.Context())},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
