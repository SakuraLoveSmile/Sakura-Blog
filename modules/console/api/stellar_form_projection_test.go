package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestStellarBasicFormProjectionUsesRawWithoutReencoding(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	raw := ` {"title":"严格原文标题","cover":"/canonical.jpg","article":{"ai_label":"reviewed"},"comments":{"options":{"large":9007199254740993,"flag":false,"nil":null}}} `
	blog := ` {"content":{"banner_image":"/banner.jpg"},"opaque":{"large":9007199254740993,"flag":false,"nil":null}} `
	for _, kind := range []string{"posts", "pages"} {
		body, _ := json.Marshal(map[string]any{"stellar_version": 2, "stellar_config": raw, "blog_extensions": blog, "slug": "raw-form-" + kind, "title": "不得优先的旧列", "cover": "/old.jpg", "content_md": "正文", "status": "published"})
		w := do2FA(r, http.MethodPost, "/api/v1/"+kind, string(body), cookie)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", kind, w.Code, w.Body.String())
		}
		w = do2FA(r, http.MethodGet, "/api/v1/"+kind+"/raw-form-"+kind, "", cookie)
		var got struct {
			StellarContentMetadata
			Title string `json:"title"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Title != "严格原文标题" || got.StellarConfig != raw || got.BlogExtensions != blog {
			t.Fatalf("reopen %s lost canonical form", kind)
		}
		if kind == "posts" {
			p, _ := st.GetBySlug("raw-form-posts")
			if p.Cover != "/canonical.jpg" || p.Banner != "/banner.jpg" || p.AILabel != "reviewed" {
				t.Fatalf("post projected wrong fields: cover=%s banner=%s ai=%s", p.Cover, p.Banner, p.AILabel)
			}
		}
	}
}

func TestStellarCollectionBasicFormProjectsNameSourceAndExtensions(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	raw := ` {"name":"集合显示名","headline":"Hero标题","tagline":"副标题","description":"严格简介","cover":"/canonical.jpg","icon":"/icon.png","source":{"repository":"owner/repo","branch":"main"},"visibility":{"listed":false,"searchable":false},"hero":{"enabled":true,"background":{"image":"/hero.jpg"}}} `
	blog := ` {"content":{"collection_order":7,"wiki_external_url":"https://example.test/wiki"},"opaque":{"large":9007199254740993,"false":false,"nil":null}} `
	body, _ := json.Marshal(map[string]any{"stellar_version": 2, "stellar_config": raw, "blog_extensions": blog, "id": "raw-wiki", "name": "旧名称", "title": "旧标题", "visible": false, "content_md": ""})
	w := do2FA(r, http.MethodPost, "/api/v1/wiki/projects", string(body), cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("wiki create: %d %s", w.Code, w.Body.String())
	}
	p, err := st.GetWikiProject("raw-wiki")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "集合显示名" || p.Title != p.Name || p.Description != "严格简介" || p.Repo != "owner/repo" || p.Branch != "main" || p.URL != "https://example.test/wiki" || p.Sort != 7 || p.Visible {
		t.Fatalf("wrong wiki projection: %+v", p)
	}
	if p.StellarConfig != raw || p.BlogExtensions != blog {
		t.Fatal("wiki raw document changed")
	}
	// listed=false 不覆盖存储权限；显式公开可与隐藏listing同时存在。
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	payload["visible"] = true
	body, _ = json.Marshal(payload)
	w = do2FA(r, http.MethodPut, "/api/v1/wiki/projects/raw-wiki", string(body), cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update wiki: %d %s", w.Code, w.Body.String())
	}
	p, _ = st.GetWikiProject("raw-wiki")
	if !p.Visible || !strings.Contains(p.StellarConfig, `"listed":false`) {
		t.Fatal("listing visibility changed publication permission")
	}
	topicRaw := ` {"name":"严格专栏","description":"专栏简介","cover":"/topic.jpg"} `
	body, _ = json.Marshal(map[string]any{"stellar_version": 2, "stellar_config": topicRaw, "blog_extensions": blog, "slug": "raw-topic", "name": "旧专栏"})
	w = do2FA(r, http.MethodPost, "/api/v1/topics", string(body), cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("topic: %d %s", w.Code, w.Body.String())
	}
	topic, _ := st.GetTopicBySlug("raw-topic")
	if topic.Name != "严格专栏" || topic.Cover != "/topic.jpg" || topic.Sort != 7 || topic.StellarConfig != topicRaw {
		t.Fatalf("wrong topic projection: %+v", topic)
	}
}
