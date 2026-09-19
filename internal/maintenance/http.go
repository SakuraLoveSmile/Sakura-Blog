package maintenance

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Status struct {
	Maintenance bool `json:"maintenance"`
	RetryAfter  int  `json:"retry_after"`
}

// Wrap 在认证和所有业务中间件之前阻断维护请求，不读取正在切换的数据库。
func (c *Coordinator) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/maintenance" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			status := Status{Maintenance: c.Active()}
			if status.Maintenance {
				status.RetryAfter = 5
			}
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		if r.URL.Path == "/livez" {
			next.ServeHTTP(w, r)
			return
		}
		release, err := c.Acquire()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Retry-After", "5")
			status := http.StatusServiceUnavailable
			if r.Method == http.MethodPost {
				switch r.URL.Path {
				case "/api/v1/backup/restores", "/api/v1/backup/restore", "/api/v1/backup/restore-snapshot", "/api/v1/system/update/apply":
					status = http.StatusConflict
				}
			}
			if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="dark"><meta http-equiv="refresh" content="5"><title>站点维护中</title><main><h1>站点维护中</h1><p>请稍候，页面将在维护结束后自动恢复。</p></main></html>`))
				return
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(Status{Maintenance: true, RetryAfter: 5})
			return
		}
		defer release()
		next.ServeHTTP(w, r.WithContext(WithRequestLease(r.Context(), release)))
	})
}
