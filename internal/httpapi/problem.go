// Package httpapi contains HTTP response contracts shared by both modules.
package httpapi

// Problem is the RFC 9457-compatible error shape used by JSON endpoints.
// The legacy error field is retained for existing clients.
type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Error    string `json:"error"`
}
