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
	r, st, admin, _ := newFullEnv(t)
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
			t.Run(testCase.name+prefix, func(t *testing.T) {
				slug := fmt.Sprintf("owners-%s-%d", testCase.name, len(prefix))
				if _, err := st.Create(&store.Post{Slug: slug, Title: "原文", ContentMD: "正文", Status: store.StatusPublished, AuthorIDs: testCase.owners}); err != nil {
					t.Fatal(err)
				}
				body := `{"title":"更新正文","content_md":"新正文","authors":["bob"],"status":"published"}`
				if testCase.actor.ID == admin.ID {
					body = `{"title":"更新正文","content_md":"新正文","status":"published"}`
				}
				for _, operation := range []struct{ method, suffix string }{
					{http.MethodPut, ""}, {http.MethodPut, "/draft"}, {http.MethodPost, "/publish"},
				} {
					w := do2FA(r, operation.method, prefix+slug+operation.suffix, body, authorCookie(testCase.actor))
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
						want := make([]string, len(testCase.owners))
						for i, id := range testCase.owners {
							want[i] = fmt.Sprint(id)
						}
						if !slices.Equal(draft.Authors, want) {
							t.Fatalf("draft authors changed: %v", draft.Authors)
						}
					}
				}
			})
		}
	}
}
