package httpapi

import (
	"net/http"
	"strconv"
)

const headPreviewLimit = 512

// HeadToGet makes HEAD requests use the corresponding GET route while keeping
// the response body suppressed. Gin only auto-registers HEAD handlers for a
// route in some configurations; doing this before Gin sees the request keeps
// route matching and middleware behavior consistent for every endpoint.
func HeadToGet(next http.Handler) http.Handler {
	if next == nil {
		return http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		req := r.Clone(r.Context())
		req.Method = http.MethodGet
		head := &headResponseWriter{dst: w}
		defer head.finish()
		next.ServeHTTP(head, req)
	})
}

// headResponseWriter buffers only response metadata and a small body prefix.
// Buffering until the handler returns lets us set the same content length as
// GET without retaining large HTML, feed, or asset responses in memory.
type headResponseWriter struct {
	dst         http.ResponseWriter
	status      int
	wroteHeader bool
	finished    bool
	bodyLen     int64
	bodyPreview []byte
}

func (w *headResponseWriter) Header() http.Header { return w.dst.Header() }

func (w *headResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
}

func (w *headResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	w.bodyLen += int64(len(p))
	if len(w.bodyPreview) < headPreviewLimit {
		remaining := headPreviewLimit - len(w.bodyPreview)
		if remaining > len(p) {
			remaining = len(p)
		}
		w.bodyPreview = append(w.bodyPreview, p[:remaining]...)
	}
	return len(p), nil
}

// Flush is intentionally a no-op: HEAD has no body to stream, and delaying
// the actual header write until the GET handler finishes preserves metadata.
func (w *headResponseWriter) Flush() {}

// Unwrap lets ResponseController inspect the original writer without making
// the usual HEAD path depend on optional server interfaces.
func (w *headResponseWriter) Unwrap() http.ResponseWriter { return w.dst }

func (w *headResponseWriter) finish() {
	if w.finished {
		return
	}
	w.finished = true
	if !w.wroteHeader {
		w.status = http.StatusOK
	}

	status := w.status
	if w.dst.Header().Get("Content-Type") == "" && w.bodyLen > 0 {
		w.dst.Header().Set("Content-Type", http.DetectContentType(w.bodyPreview))
	}
	if w.bodyLen > 0 && w.dst.Header().Get("Content-Length") == "" &&
		status != http.StatusNoContent && status != http.StatusNotModified &&
		(status < 100 || status >= 200) {
		w.dst.Header().Set("Content-Length", strconv.FormatInt(w.bodyLen, 10))
	}
	w.dst.WriteHeader(status)
}
