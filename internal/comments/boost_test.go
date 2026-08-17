package comments

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func newBoostHandlerTest(t *testing.T, authorIP string) (*gin.Engine, *store.Store, int64) {
	return newBoostHandlerTestAs(t, authorIP, nil)
}

func newBoostHandlerTestAs(t *testing.T, authorIP string, viewer *store.CurrentUser) (*gin.Engine, *store.Store, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st, err := store.New(filepath.Join(t.TempDir(), "boost.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	id, err := st.CreateComment(&store.Comment{
		PageKey: "/post/boost", Nick: "author", Email: "author@example.com",
		ContentMD: "hello", Status: store.CommentApproved, IP: authorIP,
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	var site atomic.Pointer[tmpl.Site]
	site.Store(&tmpl.Site{Comments: tmpl.CommentsConfig{Enabled: true, GravatarMirror: "https://avatar.test"}})
	h := New(st, &site)
	r := gin.New()
	if viewer != nil {
		r.Use(func(c *gin.Context) {
			c.Set(ctxKeyCurrentUser, viewer)
			c.Next()
		})
	}
	routes := h.PublicRoutes()
	comments := r.Group("/api/comments")
	comments.GET("", routes.List)
	comments.POST("", routes.Create)
	comments.POST("/:id/like", routes.Like)
	comments.POST("/:id/boost", routes.Boost)
	comments.DELETE("/:id/boost", routes.Unboost)
	comments.POST("/upload", routes.UploadImage)
	return r, st, id
}

func boostRequest(r *gin.Engine, method, path string, payload any, remote string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.RemoteAddr = remote + ":1234"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestBoostAPIValidationAndSelfGuard(t *testing.T) {
	r, _, id := newBoostHandlerTestAs(t, "198.51.100.8", &store.CurrentUser{ID: 2, Nickname: "reader", Email: "reader@example.com"})
	path := "/api/comments/" + itoa(id) + "/boost"
	for _, emoji := range []string{"💣", "🤝🏽", "🇨🇳", "👨‍👩‍👧‍👦", "1️⃣", "©️"} {
		if w := boostRequest(r, http.MethodPost, path, map[string]any{"emoji": emoji, "nick": "n", "email": "n@example.com"}, "198.51.100.9"); w.Code != http.StatusOK {
			t.Fatalf("valid emoji %q status=%d body=%s", emoji, w.Code, w.Body.String())
		}
	}
	for _, emoji := range []string{"", "hello", "😀😀", "⌂", "♘"} {
		if w := boostRequest(r, http.MethodPost, path, map[string]any{"emoji": emoji, "nick": "n", "email": "n@example.com"}, "198.51.100.9"); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid emoji %q status=%d body=%s", emoji, w.Code, w.Body.String())
		}
	}
	if w := boostRequest(r, http.MethodPost, path, map[string]any{"emoji": "👍", "text": strings.Repeat("字", 17), "nick": "n", "email": "n@example.com"}, "198.51.100.9"); w.Code != http.StatusBadRequest {
		t.Fatalf("long text status=%d body=%s", w.Code, w.Body.String())
	}
	if w := boostRequest(r, http.MethodPost, path, map[string]any{"emoji": "👍", "nick": "author", "email": "author@example.com"}, "198.51.100.8"); w.Code != http.StatusForbidden {
		t.Fatalf("self boost status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBoostAPIRequiresLogin(t *testing.T) {
	r, _, id := newBoostHandlerTest(t, "198.51.100.8")
	path := "/api/comments/" + itoa(id) + "/boost"
	if w := boostRequest(r, http.MethodPost, path, map[string]any{"emoji": "👍", "nick": "reader", "email": "reader@example.com"}, "198.51.100.9"); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous create status=%d body=%s", w.Code, w.Body.String())
	}
	if w := boostRequest(r, http.MethodDelete, path, nil, "198.51.100.9"); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBoostAPILoggedInUpdateAndDelete(t *testing.T) {
	r, st, id := newBoostHandlerTestAs(t, "198.51.100.8", &store.CurrentUser{ID: 2, Nickname: "reader", Email: "reader@example.com"})
	path := "/api/comments/" + itoa(id) + "/boost"
	w := boostRequest(r, http.MethodPost, path, map[string]any{"emoji": "👍", "text": "第一版", "nick": "reader", "email": "reader@example.com"}, "198.51.100.9")
	if w.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	w = boostRequest(r, http.MethodPost, path, map[string]any{"emoji": "🎉", "text": "更新版", "nick": "reader", "email": "reader@example.com"}, "198.51.100.9")
	if w.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Boost struct {
			Emoji string `json:"emoji"`
			Text  string `json:"text"`
		} `json:"boost"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Boost.Emoji != "🎉" || resp.Boost.Text != "更新版" {
		t.Fatalf("update response=%s err=%v", w.Body.String(), err)
	}
	if w = boostRequest(r, http.MethodDelete, path, nil, "198.51.100.9"); w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}
	if n, err := st.CountBoostsByComment(id); err != nil || n != 0 {
		t.Fatalf("remaining boosts=%d err=%v", n, err)
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
