// Package servertime owns the server-time API contract and HTTP behavior.
package servertime

//go:generate go run ../../../cmd/gen-client -out ../../../../src/lib/api/generated.ts

const (
	Route    = "/api/v1/server-time"
	TimeZone = "Africa/Kigali"
)

type Response struct {
	Data Data `json:"data"`
	Meta Meta `json:"meta"`
}

type Data struct {
	ISO              string `json:"iso"`
	Display          string `json:"display"`
	TimeZone         string `json:"timeZone"`
	UnixMilliseconds int64  `json:"unixMilliseconds"`
}

type Meta struct {
	RequestID string `json:"requestId"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}
