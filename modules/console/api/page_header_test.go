package api

import (
	"encoding/json"
	"net/http"
	"testing"

	tmpl "blog/internal/settings"
)

func TestPageHeaderAPICreateUpdateAndEcho(t *testing.T) {
	r, _, user, h := newFullStellarEnv(t)
	h.site.Load().Authors = map[string]tmpl.AuthorDef{"alice": {Name: "Alice"}}
	cookie := authCookie(user)

	body := `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"about-header","title":"关于本站","content_md":"# 正文","status":"published","banner":"https://example.test/banner.jpg","banner_info":{"title":"横幅标题","subtitle":"横幅副标题","avatar":"/avatar.png","icon":"default:book"},"breadcrumb":false,"author":"alice","date":"2026-08-01"}`
	w := do2FA(r, http.MethodPost, "/api/v1/pages", body, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create page: status=%d body=%s", w.Code, w.Body.String())
	}
	var got pageJSON
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if got.Banner != "https://example.test/banner.jpg" || got.BannerInfo.Title != "横幅标题" || got.BannerInfo.Subtitle != "横幅副标题" || got.BannerInfo.Avatar != "/avatar.png" || got.BannerInfo.Icon != "default:book" || got.Breadcrumb || got.Author != "alice" || got.Date != "2026-08-01" {
		t.Fatalf("create response lost page header fields: %+v", got)
	}

	w = do2FA(r, http.MethodPut, "/api/v1/pages/about-header", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"about-header","title":"更新","content_md":"# 更新","status":"published","banner":"","banner_info":{"title":"新标题"},"breadcrumb":true,"author":"","date":"2026-08-27"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update page: status=%d body=%s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodGet, "/api/v1/pages/about-header", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("get page: status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got.Banner != "" || got.BannerInfo.Title != "新标题" || got.BannerInfo.Subtitle != "" || !got.Breadcrumb || got.Author != "" || got.Date != "2026-08-27" {
		t.Fatalf("update response lost page header fields: %+v", got)
	}

	for name, invalid := range map[string]string{
		"unknown author": `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"bad-author","title":"x","author":"nobody"}`,
		"invalid date":   `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"bad-date","title":"x","date":"2026/08/27"}`,
	} {
		t.Run(name, func(t *testing.T) {
			w := do2FA(r, http.MethodPost, "/api/v1/pages", invalid, cookie)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("invalid page header status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
