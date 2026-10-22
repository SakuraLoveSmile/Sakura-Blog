package stellarmigration

import (
	"fmt"
	"strings"
	"testing"
)

func TestReviewedLegacyPoliciesNeedExactNamedApproval(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{"style": map[string]any{
		"gradient":        map[string]any{"avatar": "conic-gradient(red, blue)"},
		"page_transition": map[string]any{"enable": nil},
	}})
	c := candidate(t, s)
	before := CandidateDigest(c)
	choices := []MigrationResolution{
		{Source: "settings", Code: "legacy_official_policy", Path: "style.gradient.avatar", Action: "use_official"},
		{Source: "settings", Code: "legacy_official_policy", Path: "style.page_transition.enable", Action: "use_official"},
	}
	if _, err := PreviewReviewedSelections(c, choices[:1], NotebookStructureDecisions{}); err == nil {
		t.Fatal("partial official-policy choices were accepted")
	}
	p, err := PreviewReviewedSelections(c, choices, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Candidate.PublicReport().Blocking != 0 || len(p.Resolutions) != 2 || CandidateDigest(c) != before || encode(p.Candidate.Source) != encode(s) {
		t.Fatal("policy selection changed its source or left an unreviewed blocker")
	}
	unknown := candidate(t, settingsSnapshot(t, map[string]any{"style": map[string]any{"unknown": "private"}}))
	for i := range unknown.Theme.Diagnostics {
		unknown.Theme.Diagnostics[i].Code = "legacy_official_policy"
	}
	if _, err := PreviewReviewedSelections(unknown, []MigrationResolution{{Source: "settings", Code: "legacy_official_policy", Path: "style.unknown", Action: "use_official"}}, NotebookStructureDecisions{}); err == nil {
		t.Fatal("unknown setting was disguised as an approved official policy")
	}
}

func TestReviewedMissingWikiShelfChoicesDoNotDeleteSource(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{"wiki": []any{
		map[string]any{"id": "first", "title": "First", "url": "https://example.com/first"},
		map[string]any{"id": "second", "title": "Second", "url": "/archives/"},
		map[string]any{"id": "third", "title": "Third", "url": "/categories/"},
	}})
	c := candidate(t, s)
	before := CandidateDigest(c)
	choices := []MigrationResolution{}
	for i := 0; i < 3; i++ {
		choices = append(choices, MigrationResolution{Source: "settings", Code: "wiki_shelf_project_missing", Path: fmt.Sprintf("wiki[%d].id", i), Action: "remove_missing_shelf"})
	}
	if _, err := PreviewReviewedSelections(c, choices[:2], NotebookStructureDecisions{}); err == nil {
		t.Fatal("an unselected missing shelf was silently removed")
	}
	p, err := PreviewReviewedSelections(c, choices, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	blog, _ := decode(p.Candidate.Theme.BlogJSON)
	ids, ok := get(object(blog), "data.wiki")
	if !ok || len(array(ids)) != 0 || len(p.Resolutions) != 3 || p.Candidate.PublicReport().Blocking != 0 {
		t.Fatal("named removals lost their source indexes or retained missing IDs")
	}
	if CandidateDigest(c) != before || encode(p.Candidate.Source) != encode(s) || !strings.Contains(encode(p.Candidate.Source), "First") {
		t.Fatal("missing shelf selection discarded the original cards")
	}
	known := clone(c)
	known.Source.Tables["wiki_projects"] = Table{Rows: []Row{{"id": "first"}}}
	d := Diagnostic{Source: "settings", Code: "wiki_shelf_project_missing", Path: "wiki[0].id"}
	if err := applyResolution(&known, d, choices[0], NotebookStructurePreview{}, map[string]string{}); err == nil {
		t.Fatal("missing shelf action was allowed to remove an existing project")
	}
}

func TestReviewedMissingWikiShelfCanListChosenExistingProject(t *testing.T) {
	s := snapshot(t, fixture(t, `
CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);
INSERT INTO settings VALUES('wiki','[{"id":"first"},{"id":"second"},{"id":"third"}]');
CREATE TABLE wiki_projects(id TEXT PRIMARY KEY,name TEXT,visible INTEGER);
INSERT INTO wiki_projects VALUES('real','Real collection',1);
`), nil)
	c := candidate(t, s)
	before := CandidateDigest(c)
	choices := []MigrationResolution{
		{Source: "settings", Code: "wiki_shelf_project_missing", Path: "wiki[0].id", Action: "remove_missing_shelf"},
		{Source: "settings", Code: "wiki_shelf_project_missing", Path: "wiki[1].id", Action: "remove_missing_shelf"},
		{Source: "settings", Code: "wiki_shelf_project_missing", Path: "wiki[2].id", Action: "choose_existing_project", ReplacementID: "real"},
	}
	p, err := PreviewReviewedSelections(c, choices, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	blog, _ := decode(p.Candidate.Theme.BlogJSON)
	ids, _ := get(object(blog), "data.wiki")
	if encode(ids) != `["real"]` || p.Candidate.PublicReport().Blocking != 0 || encode(p.Candidate.Source) != encode(s) || CandidateDigest(c) != before {
		t.Fatal("chosen project was not listed exactly once or original data changed")
	}
	for _, test := range []struct {
		name string
		edit func([]MigrationResolution)
	}{
		{"missing ID", func(r []MigrationResolution) { r[2].ReplacementID = "unknown" }},
		{"empty ID", func(r []MigrationResolution) { r[2].ReplacementID = "" }},
		{"duplicate ID", func(r []MigrationResolution) { r[1].Action, r[1].ReplacementID = "choose_existing_project", "real" }},
		{"unrelated action", func(r []MigrationResolution) { r[0].ReplacementID = "real" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := append([]MigrationResolution(nil), choices...)
			test.edit(invalid)
			if _, err := PreviewReviewedSelections(c, invalid, NotebookStructureDecisions{}); err == nil {
				t.Fatal("invalid project selection was accepted")
			}
		})
	}
}
