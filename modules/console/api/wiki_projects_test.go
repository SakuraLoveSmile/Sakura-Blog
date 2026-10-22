package api

import (
	"net/http"
	"strings"
	"testing"

	"blog/internal/store"
)

func TestWikiProjectCRUDAPI(t *testing.T) {
	r, st := newWikiTestEnv(t)
	create := wikiReq(r, http.MethodPost, "/api/v1/wiki/projects", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","id":"stellar","title":"Stellar","tags":["主题","开源"],"visible":false,"hero":{"background":"https://cdn.example/hero.jpg","animation":{"type":"galaxy","params":{"density":4,"transparent":true}}}}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create project got %d: %s", create.Code, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"visible":false`) {
		t.Fatalf("create response should preserve visible=false: %s", create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"animation":{"type":"galaxy","params":{"density":4,"transparent":true}}`) {
		t.Fatalf("create response should preserve Hero animation: %s", create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"background":"https://cdn.example/hero.jpg"`) {
		t.Fatalf("create response should preserve Hero background image: %s", create.Body.String())
	}

	duplicate := wikiReq(r, http.MethodPost, "/api/v1/wiki/projects", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","id":"stellar","title":"Duplicate"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate project got %d, want 409: %s", duplicate.Code, duplicate.Body.String())
	}

	changedID := wikiReq(r, http.MethodPut, "/api/v1/wiki/projects/stellar", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","id":"other","title":"Nope"}`)
	if changedID.Code != http.StatusBadRequest {
		t.Fatalf("changed id got %d, want 400: %s", changedID.Code, changedID.Body.String())
	}

	doc := &store.WikiDoc{ProjectID: "stellar", Slug: "intro", Title: "Intro", Status: store.StatusPublished}
	if err := st.CreateWikidoc(doc); err != nil {
		t.Fatalf("create project doc: %v", err)
	}
	blocked := wikiReq(r, http.MethodDelete, "/api/v1/wiki/projects/stellar", "")
	if blocked.Code != http.StatusConflict {
		t.Fatalf("delete project with docs got %d, want 409: %s", blocked.Code, blocked.Body.String())
	}
}
