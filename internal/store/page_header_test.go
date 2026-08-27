package store

import "testing"

func TestPageBannerFieldsRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages table: %v", err)
	}

	want := &Page{
		Slug:        "about",
		Title:       "关于本站",
		ContentMD:   "正文",
		ContentHTML: "<p>正文</p>",
		Status:      StatusPublished,
		Banner:      "https://example.test/about.jpg",
		BannerInfo:  PageBannerInfo{Title: "关于", Subtitle: "副标题", Avatar: "/avatar.png", Icon: "default:book"},
		Breadcrumb:  false,
		Author:      "alice",
		PublishedAt: "2026-08-01",
	}
	if _, err := s.CreatePage(want); err != nil {
		t.Fatalf("create page: %v", err)
	}

	got, err := s.GetPageBySlug(want.Slug)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if got == nil {
		t.Fatal("page not found")
	}
	if got.Banner != want.Banner || got.BannerInfo != want.BannerInfo || got.Breadcrumb != want.Breadcrumb || got.Author != want.Author || got.PublishedAt != want.PublishedAt {
		t.Fatalf("banner fields changed after round trip: got=%+v want=%+v", got, want)
	}

	updated := *got
	updated.Banner = "https://example.test/updated.jpg"
	updated.BannerInfo = PageBannerInfo{Title: "更新标题", Subtitle: "更新副标题", Avatar: "", Icon: "default:note"}
	updated.Breadcrumb = true
	updated.PublishedAt = "2026-08-27"
	if err := s.UpdatePage(want.Slug, &updated); err != nil {
		t.Fatalf("update page: %v", err)
	}
	got, err = s.GetPageBySlug(want.Slug)
	if err != nil {
		t.Fatalf("get updated page: %v", err)
	}
	if got.Banner != updated.Banner || got.BannerInfo != updated.BannerInfo || got.Breadcrumb != updated.Breadcrumb || got.PublishedAt != updated.PublishedAt {
		t.Fatalf("updated banner fields not persisted: got=%+v want=%+v", got, updated)
	}
}

func TestEnsurePagesTableMigratesLegacyPageColumns(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsurePagesTable(); err != nil {
		t.Fatalf("create pages table before legacy migration: %v", err)
	}
	if _, err := s.DB().Exec(`DROP TABLE pages`); err != nil {
		t.Fatalf("drop pages table: %v", err)
	}
	if _, err := s.DB().Exec(`CREATE TABLE pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL DEFAULT '',
		content_md TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'draft',
		is_system INTEGER NOT NULL DEFAULT 0,
		views INTEGER NOT NULL DEFAULT 0,
		leftbar_widgets TEXT NOT NULL DEFAULT '',
		rightbar_widgets TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create legacy pages table: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO pages (slug, title, status, created_at, updated_at) VALUES ('legacy', '旧页面', 'published', '2026-08-19 12:34:56', '2026-08-19 12:34:56')`); err != nil {
		t.Fatalf("insert legacy page: %v", err)
	}

	if err := s.EnsurePagesTable(); err != nil {
		t.Fatalf("migrate pages table: %v", err)
	}
	got, err := s.GetPageBySlug("legacy")
	if err != nil {
		t.Fatalf("get migrated page: %v", err)
	}
	if got == nil {
		t.Fatal("migrated page not found")
	}
	if got.Banner != "" || got.BannerInfo != (PageBannerInfo{}) || got.Author != "" {
		t.Fatalf("new banner fields should use empty defaults: %+v", got)
	}
	if !got.Breadcrumb {
		t.Fatal("migrated page should show breadcrumb by default")
	}
	if got.PublishedAt != "2026-08-19" {
		t.Fatalf("published_at backfill = %q, want 2026-08-19", got.PublishedAt)
	}

	// 迁移必须幂等，且不能覆盖后来主动清空的日期。
	if _, err := s.DB().Exec(`UPDATE pages SET published_at = '' WHERE slug = 'legacy'`); err != nil {
		t.Fatalf("clear migrated date: %v", err)
	}
	if err := s.EnsurePagesTable(); err != nil {
		t.Fatalf("rerun pages migration: %v", err)
	}
	got, err = s.GetPageBySlug("legacy")
	if err != nil {
		t.Fatalf("get rerun page: %v", err)
	}
	if got.PublishedAt != "" {
		t.Fatalf("rerun migration restored cleared date: %q", got.PublishedAt)
	}
}
