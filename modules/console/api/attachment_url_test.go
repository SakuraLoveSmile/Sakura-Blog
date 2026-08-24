package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
)

func TestImportAttachmentURLRejectsUnsafeTargets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	for _, test := range []struct {
		name string
		url  string
		code int
	}{
		{name: "unsupported scheme", url: "file:///etc/passwd", code: http.StatusBadRequest},
		{name: "loopback IP", url: "http://127.0.0.1/image.png", code: http.StatusForbidden},
		{name: "private IP", url: "http://192.168.1.1/image.png", code: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"url": test.url})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/import-url", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			if response.Code != test.code {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.code, response.Body.String())
			}
		})
	}
}

func TestAttachmentURLFileInfo(t *testing.T) {
	u, err := url.Parse("https://example.com/path/photo.png?size=small")
	if err != nil {
		t.Fatal(err)
	}
	filename, mimeType, ok := attachmentURLFileInfo(u, "image/png; charset=utf-8")
	if !ok || filename != "photo.png" || mimeType != "image/png" {
		t.Fatalf("file info = %q, %q, %v", filename, mimeType, ok)
	}
}
