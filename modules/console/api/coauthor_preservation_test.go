package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"blog/internal/store"
)

func TestAuthorWritesPreserveNumericUsernameAndCoauthors(t *testing.T) {
	r, st, admin, _ := newFullStellarEnv(t)
	alice, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := st.GetUserByUsername("bob")
	if err != nil {
		t.Fatal(err)
	}
	// 数字用户名恰好等于另一个用户的 ID，确保重新解析用户名会被测试捕获。
	numeric := &store.User{Username: fmt.Sprint(bob.ID), PasswordHash: "x", AuthSecret: store.GenerateAuthSecret(), Role: store.RoleAuthor}
	if err := st.CreateUser(numeric); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name   string
		owners []int64
		actor  *store.User
	}{
		{"solo", []int64{numeric.ID}, numeric},
		{"coauthored", []int64{numeric.ID, alice.ID}, alice},
		{"admin", []int64{numeric.ID, alice.ID}, admin},
	} {
		prefixes := []string{"/api/v1/posts/", "/api/v1/uc/posts/"}
		for _, prefix := range prefixes {
			if testCase.actor.ID == admin.ID && prefix == "/api/v1/uc/posts/" {
				continue // 后台管理员身份不能替换 UC 的公共作者会话。
			}
			t.Run(testCase.name+prefix, func(t *testing.T) {
				slug := fmt.Sprintf("owners-%s-%d", testCase.name, len(prefix))
				if _, err := st.Create(&store.Post{Slug: slug, Title: "原文", ContentMD: "正文", Status: store.StatusPublished, AuthorIDs: testCase.owners}); err != nil {
					t.Fatal(err)
				}
				body := `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"更新正文","content_md":"新正文","authors":["bob"],"status":"published"}`
				if testCase.actor.ID == admin.ID {
					body = `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"更新正文","content_md":"新正文","status":"published"}`
				}
				for _, operation := range []struct{ method, suffix string }{
					{http.MethodPut, ""}, {http.MethodPut, "/draft"}, {http.MethodPost, "/publish"},
				} {
					consoleCookie, publicCookie := authorCookie(testCase.actor), ""
					if prefix == "/api/v1/uc/posts/" {
						consoleCookie, publicCookie = "", signReaderCookie(testCase.actor)
					}
					w := doUCSessionReq(r, operation.method, prefix+slug+operation.suffix, body, consoleCookie, publicCookie, "")
					if w.Code != http.StatusOK {
						t.Fatalf("%s %s: status %d", operation.method, prefix+operation.suffix, w.Code)
					}
					saved, err := st.GetBySlug(slug)
					if err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(saved.AuthorIDs, testCase.owners) {
						t.Fatalf("authors changed: got %v want %v", saved.AuthorIDs, testCase.owners)
					}
					if operation.suffix == "/draft" {
						var draft struct {
							Authors []string `json:"authors"`
						}
						if err := json.Unmarshal([]byte(saved.DraftJSON), &draft); err != nil {
							t.Fatal(err)
						}
						if len(draft.Authors) != 0 {
							t.Fatalf("ownership IDs leaked into strict public draft authors: %v", draft.Authors)
						}
					}
				}
			})
		}
	}
}
