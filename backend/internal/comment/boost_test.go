package comment

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

	"blog/internal/store"
	tmpl "blog/internal/template"
)

func newBoostHandlerTest(t *testing.T, authorIP string) (*gin.Engine, *store.Store, int64) {
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
	h.Register(r)
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
	r, _, id := newBoostHandlerTest(t, "198.51.100.8")
	if w := boostRequest(r, http.MethodPost, "/api/comments/"+itoa(id)+"/boost", map[string]any{"emoji": "💣", "nick": "n", "email": "n@example.com"}, "198.51.100.9"); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid emoji status=%d body=%s", w.Code, w.Body.String())
	}
	if w := boostRequest(r, http.MethodPost, "/api/comments/"+itoa(id)+"/boost", map[string]any{"emoji": "👍", "text": strings.Repeat("字", 17), "nick": "n", "email": "n@example.com"}, "198.51.100.9"); w.Code != http.StatusBadRequest {
		t.Fatalf("long text status=%d body=%s", w.Code, w.Body.String())
	}
	if w := boostRequest(r, http.MethodPost, "/api/comments/"+itoa(id)+"/boost", map[string]any{"emoji": "👍", "nick": "author", "email": "author@example.com"}, "198.51.100.8"); w.Code != http.StatusForbidden {
		t.Fatalf("self boost status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBoostAPIAnonymousUpdateAndDelete(t *testing.T) {
	r, st, id := newBoostHandlerTest(t, "198.51.100.8")
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
