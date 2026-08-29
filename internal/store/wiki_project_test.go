package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func newWikiProjectStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "wiki-projects.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureWikiProjectsTable(); err != nil {
		t.Fatalf("ensure wiki projects: %v", err)
	}
	if err := s.EnsureWikidocsTable(); err != nil {
		t.Fatalf("ensure wikidocs: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestWikiProjectMigrationIsIdempotentAndKeepsMetadata(t *testing.T) {
	s := newWikiProjectStore(t)
	placeholder := &WikiDoc{ProjectID: "legacy-docs", Slug: "intro", Title: "Intro", Status: StatusPublished}
	if err := s.CreateWikidoc(placeholder); err != nil {
		t.Fatalf("create legacy doc: %v", err)
	}

	source := []*WikiProject{{
		ID: "stellar", Name: "stellar", Title: "Stellar", Description: "docs",
		Tags: []string{"主题", "开源", "主题"}, Visible: true,
		Hero: &WikiHero{Background: "galaxy", Title: "Stellar Hero", Preview: &WikiHeroPreview{
			Type: "terminal", Commands: []WikiHeroCommand{{Codes: []string{"make dev"}}},
		}},
	}}
	if err := s.MigrateWikiProjects(source); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := s.MigrateWikiProjects(source); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	projects, err := s.ListWikiProjectsWithCounts()
	if err != nil {
		t.Fatalf("list migrated projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("migration should be idempotent and add one placeholder, got %d projects", len(projects))
	}
	got, err := s.GetWikiProject("stellar")
	if err != nil || got == nil {
		t.Fatalf("get migrated project: %v (nil=%v)", err, got == nil)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "主题" || got.Tags[1] != "开源" {
		t.Fatalf("tags should be ordered and de-duplicated: %#v", got.Tags)
	}
	if got.Hero == nil || got.Hero.Background != "" || got.Hero.Animation == nil || got.Hero.Animation.Type != "galaxy" || got.Hero.Preview == nil || got.Hero.Preview.Commands[0].Codes[0] != "make dev" {
		t.Fatalf("hero metadata was not migrated: %#v", got.Hero)
	}
	legacy, err := s.GetWikiProject("legacy-docs")
	docs, docsErr := s.ListWikidocs("legacy-docs")
	if docsErr != nil || len(docs) != 1 {
		t.Fatalf("legacy document should remain after migration: docs=%#v err=%v", docs, docsErr)
	}
	if legacy != nil {
		if err := s.PopulateWikiProjectCounts(legacy); err != nil {
			t.Fatalf("populate placeholder counts: %v", err)
		}
	}
	if err != nil || legacy == nil || !legacy.HasDocs || legacy.DocCount != 1 {
		t.Fatalf("document placeholder should be created with counts: project=%#v err=%v", legacy, err)
	}
}

func TestWikiProjectCRUDAndDeleteProtection(t *testing.T) {
	s := newWikiProjectStore(t)
	p := &WikiProject{ID: "hidden", Name: "Hidden", Visible: false, Tags: []string{"one", "two"}}
	if err := s.CreateWikiProject(p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	got, err := s.GetWikiProject("hidden")
	if err != nil || got == nil || got.Visible {
		t.Fatalf("explicit hidden state should persist: project=%#v err=%v", got, err)
	}
	if err := s.CreateWikiProject(&WikiProject{ID: "hidden"}); !errors.Is(err, ErrWikiProjectConflict) {
		t.Fatalf("duplicate id error=%v, want conflict", err)
	}
	if err := s.CreateWikiProject(&WikiProject{ID: "bad/id"}); !errors.Is(err, ErrWikiProjectInvalid) {
		t.Fatalf("invalid id error=%v, want invalid", err)
	}

	doc := &WikiDoc{ProjectID: "hidden", Slug: "guide", Title: "Guide", Status: StatusPublished}
	if err := s.CreateWikidoc(doc); err != nil {
		t.Fatalf("create project doc: %v", err)
	}
	if err := s.DeleteWikiProject("hidden"); !errors.Is(err, ErrWikiProjectConflict) {
		t.Fatalf("project with docs delete error=%v, want conflict", err)
	}
	if err := s.DeleteWikidoc(doc.ID); err != nil {
		t.Fatalf("remove project doc: %v", err)
	}
	if err := s.DeleteWikiProject("hidden"); err != nil {
		t.Fatalf("empty project should delete: %v", err)
	}
}

func TestWikiProjectMigrationPreservesHiddenVisibility(t *testing.T) {
	s := newWikiProjectStore(t)
	source := []*WikiProject{{ID: "hidden", Name: "Hidden", Visible: false, Pin: false}}
	if err := s.MigrateWikiProjects(source); err != nil {
		t.Fatalf("migrate hidden project: %v", err)
	}
	got, err := s.GetWikiProject("hidden")
	if err != nil || got == nil {
		t.Fatalf("get hidden project: %#v, %v", got, err)
	}
	if got.Visible {
		t.Fatal("hidden project became visible during migration")
	}
}
