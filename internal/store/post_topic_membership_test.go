package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func strictTopic(t *testing.T, s *Store, slug string) *Topic {
	t.Helper()
	topic := &Topic{Name: slug, Slug: slug, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}
	if err := s.CreateTopic(topic); err != nil {
		t.Fatal(err)
	}
	return topic
}
func strictTopicPost(slug, topic string) *Post {
	raw := `{}`
	if topic != "" {
		raw = fmt.Sprintf(`{"collection":{"profile":"topic","id":%q}}`, topic)
	}
	return &Post{Slug: slug, Title: slug, Status: StatusPublished, StellarVersion: 2, StellarConfig: raw, BlogExtensions: `{}`}
}
func strictTopicDraft(topic string) string {
	p := strictTopicPost("draft", topic)
	raw, _ := json.Marshal(p)
	return string(raw)
}

func TestStrictTopicMembershipProtectsRawAndDraft(t *testing.T) {
	for _, mode := range []string{"raw", "draft", "trashed"} {
		t.Run(mode, func(t *testing.T) {
			s := newTopicStore(t)
			topic := strictTopic(t, s, "members")
			p := strictTopicPost("member", "")
			if mode != "draft" {
				p = strictTopicPost("member", topic.Slug)
			}
			if _, err := s.Create(p); err != nil {
				t.Fatal(err)
			}
			if mode == "draft" {
				if err := s.SaveDraft(p.Slug, strictTopicDraft(topic.Slug)); err != nil {
					t.Fatal(err)
				}
			}
			// 模拟已有原文成员但旧投影列尚未回填；保护必须以原文为真值。
			if _, err := s.db.Exec(`UPDATE posts SET topic_id=0,topic='' WHERE slug=?`, p.Slug); err != nil {
				t.Fatal(err)
			}
			if mode == "trashed" {
				if err := s.Delete(p.Slug); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := s.GetBySlug(p.Slug)
			if mode == "trashed" {
				before, _ = s.GetBySlug(p.Slug)
			}
			if n, err := s.CountAllByTopicID(topic.ID); err != nil || n != 1 {
				t.Fatalf("count=%d err=%v", n, err)
			}
			topic.Name, topic.Slug = "must not change", "renamed"
			if err := s.UpdateTopic(topic); !errors.Is(err, ErrTopicInUse) {
				t.Fatalf("rename: %v", err)
			}
			if err := s.DeleteTopic(topic.ID); !errors.Is(err, ErrTopicInUse) {
				t.Fatalf("delete: %v", err)
			}
			got, _ := s.GetTopicByID(topic.ID)
			if got.Slug != "members" || got.Name != "members" {
				t.Fatal("failed rename mutated topic")
			}
			after, _ := s.GetBySlug(p.Slug)
			if mode == "trashed" {
				after, _ = s.GetBySlug(p.Slug)
			}
			if before.StellarConfig != after.StellarConfig || before.DraftJSON != after.DraftJSON {
				t.Fatal("member raw was rewritten")
			}
		})
	}
}

func TestStrictTopicWriteRejectsDeletedAndLegacyCollections(t *testing.T) {
	s := newTopicStore(t)
	topic := strictTopic(t, s, "gone")
	p := strictTopicPost("base", "")
	if _, err := s.Create(p); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTopic(topic.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(strictTopicPost("new", "gone")); !errors.Is(err, ErrPostTopicNotFound) {
		t.Fatalf("create: %v", err)
	}
	changed := strictTopicPost("base", "gone")
	if err := s.UpdateSlugWithRevision("base", "base", changed); !errors.Is(err, ErrPostTopicNotFound) {
		t.Fatalf("update: %v", err)
	}
	if err := s.SaveDraft("base", strictTopicDraft("gone")); !errors.Is(err, ErrPostTopicNotFound) {
		t.Fatalf("draft: %v", err)
	}
	if err := s.SaveDraft("base", `{"stellar_version":2,"stellar_config":42,"blog_extensions":"{}"}`); err == nil {
		t.Fatal("invalid strict draft type accepted")
	}
	got, _ := s.GetBySlug("base")
	if got.StellarConfig != "{}" || got.DraftJSON != "" {
		t.Fatal("failed write mutated post")
	}
	mkTopic(t, s, "legacy", "legacy", 0)
	if _, err := s.Create(strictTopicPost("legacy-member", "legacy")); !errors.Is(err, ErrPostTopicNotFound) {
		t.Fatalf("legacy: %v", err)
	}
	empty := strictTopic(t, s, "empty")
	empty.Slug = "renamed"
	if err := s.UpdateTopic(empty); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTopic(empty.ID); err != nil {
		t.Fatal(err)
	}
}

func TestStrictTopicDeleteAndWriteAreAtomic(t *testing.T) {
	for _, mode := range []string{"create", "update", "draft"} {
		t.Run(mode, func(t *testing.T) {
			for attempt := 0; attempt < 8; attempt++ {
				s := newTopicStore(t)
				topic := strictTopic(t, s, "race")
				if mode != "create" {
					if _, err := s.Create(strictTopicPost("member", "")); err != nil {
						t.Fatal(err)
					}
				}
				start := make(chan struct{})
				var wg sync.WaitGroup
				var deletion, write error
				wg.Add(2)
				go func() { defer wg.Done(); <-start; deletion = s.DeleteTopic(topic.ID) }()
				go func() {
					defer wg.Done()
					<-start
					switch mode {
					case "create":
						_, write = s.Create(strictTopicPost("member", "race"))
					case "update":
						write = s.Update("member", strictTopicPost("member", "race"))
					case "draft":
						write = s.SaveDraft("member", strictTopicDraft("race"))
					}
				}()
				close(start)
				wg.Wait()
				if deletion == nil && write == nil {
					t.Fatal("deletion and member write both succeeded")
				}
				if deletion != nil && !errors.Is(deletion, ErrTopicInUse) {
					t.Fatalf("delete: %v", deletion)
				}
				if write != nil && !errors.Is(write, ErrPostTopicNotFound) {
					t.Fatalf("write: %v", write)
				}
				got, _ := s.GetTopicByID(topic.ID)
				if write == nil && got == nil {
					t.Fatal("orphan member committed")
				}
			}
		})
	}
}
