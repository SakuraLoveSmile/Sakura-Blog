package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestCategoryAPIPersistsLegacyCategoryAssociation(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	w := do2FA(r, http.MethodPost, "/api/v1/posts", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"分类文章","slug":"category-api","category":"后端","status":"published"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	posts, err := st.ListByCategory("后端")
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("association count=%d response=%s", len(posts), w.Body.String())
	}
}

func TestCategoryAPIMultiCategoryRequest(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	w := do2FA(r, http.MethodPost, "/api/v1/posts", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"多分类文章","slug":"multi-category-api","categories":["产研/技术","产品"],"status":"published"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Category   string   `json:"category"`
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Category != "产研/技术" || len(response.Categories) != 2 || response.Categories[1] != "产品" {
		t.Fatalf("response categories = %+v", response)
	}
	post, err := st.GetBySlug("multi-category-api")
	if err != nil {
		t.Fatal(err)
	}
	if post == nil || len(post.Categories) != 2 || post.Category != "产研/技术" {
		t.Fatalf("stored categories = %#v", post)
	}
}

func TestCategoryAPITreeCRUDAndSafeDelete(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	create := func(path, body string) struct {
		ID int64 `json:"id"`
	} {
		w := do2FA(r, http.MethodPost, path, body, cookie)
		if w.Code != http.StatusCreated {
			t.Fatalf("create category status=%d body=%s", w.Code, w.Body.String())
		}
		var out struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	parent := create("/api/v1/categories", `{"name":"产研"}`)
	child := create("/api/v1/categories", `{"name":"技术","parent_id":`+categoryItoa(parent.ID)+`}`)
	if parent.ID == 0 || child.ID == 0 {
		t.Fatalf("ids = %d, %d", parent.ID, child.ID)
	}
	w := do2FA(r, http.MethodGet, "/api/v1/categories", "", cookie)
	var result struct {
		Tree []struct {
			Path     string `json:"path"`
			Children []struct {
				Path string `json:"path"`
			} `json:"children"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Tree) == 0 || len(result.Tree[0].Children) == 0 || result.Tree[0].Children[0].Path != "产研/技术" {
		t.Fatalf("tree = %+v", result.Tree)
	}
	w = do2FA(r, http.MethodDelete, "/api/v1/categories/"+categoryItoa(parent.ID), "", cookie)
	if w.Code != http.StatusConflict {
		t.Fatalf("delete non-empty parent status=%d body=%s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodPut, "/api/v1/categories/"+categoryItoa(parent.ID), `{"name":"产研","parent_id":`+categoryItoa(child.ID)+`}`, cookie)
	if w.Code != http.StatusConflict {
		t.Fatalf("cycle status=%d body=%s", w.Code, w.Body.String())
	}
}

func categoryItoa(v int64) string {
	return fmt.Sprintf("%d", v)
}
