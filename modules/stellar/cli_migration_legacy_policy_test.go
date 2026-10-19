package stellar

import (
	"path/filepath"
	"testing"

	"blog/internal/stellarmigration"
	"blog/internal/store"
)

func TestMigrationCLILegacyPolicyGuideKeepsChoicesUnselected(t *testing.T) {
	root, layout := migrationLegacyCLIFixture(t, "")
	st, err := store.OpenExisting(layout.DBPath, false)
	if err != nil {
		t.Fatal(err)
	}
	for section, raw := range map[string]string{
		"style": `{"gradient":{"avatar":"custom"},"page_transition":{"enable":null}}`,
		"wiki":  `[{"id":"missing","title":"Keep archived","url":"/archives/"}]`,
	} {
		if err := st.PutSetting(section, raw); err != nil {
			st.Close()
			t.Fatal(err)
		}
	}
	st.Close()
	review := filepath.Join(t.TempDir(), "review")
	if code, out, stderr := runMigrationTestCLI(t, root, "preview", "--out", review); code != 0 {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	var guide migrationDecisionGuide
	var pending migrationCLIApproval
	readMigrationTestArtifact(t, review, "decisions-guide.private.json", &guide)
	readMigrationTestArtifact(t, review, "approval-template.private.json", &pending)
	want := map[string]string{"style.gradient.avatar": "use_official", "style.page_transition.enable": "use_official", "wiki[0].id": "remove_missing_shelf"}
	for _, item := range guide.Resolutions {
		if action, exists := want[item.Path]; exists {
			if len(item.Actions) != 1 || item.Actions[0] != action {
				t.Fatalf("unexpected actions for %s: %v", item.Path, item.Actions)
			}
			delete(want, item.Path)
		}
	}
	if len(want) != 0 || pending.Approval.Approved || pending.Offline.Approved {
		t.Fatal("missing guidance or an implicit execution approval")
	}
	for _, choice := range pending.Approval.Resolutions {
		if choice.Action != "" {
			t.Fatal("legacy policy choice was inferred")
		}
	}
}

func TestMigrationCLIExistingProjectGuideDoesNotChooseProject(t *testing.T) {
	c := stellarmigration.Candidate{
		Source: stellarmigration.Snapshot{Tables: map[string]stellarmigration.Table{
			"wiki_projects": {Rows: []stellarmigration.Row{{"id": "real"}}},
		}},
		Diagnostics: []stellarmigration.Diagnostic{{Source: "settings", Code: "wiki_shelf_project_missing", Path: "wiki[0].id", Blocking: true}},
	}
	pending, guide := migrationDecisionTemplate(stellarmigration.RootCapture{Source: c.Source}, c)
	if len(guide.Resolutions) != 1 || len(guide.Resolutions[0].Actions) != 2 || guide.Resolutions[0].Actions[1] != "choose_existing_project" || len(guide.Resolutions[0].ProjectIDs) != 1 || guide.Resolutions[0].ProjectIDs[0] != "real" {
		t.Fatalf("missing explicit existing-project guidance: %+v", guide.Resolutions)
	}
	if len(pending.Approval.Resolutions) != 1 || pending.Approval.Resolutions[0].ReplacementID != "" || pending.Approval.Resolutions[0].Action != "" || pending.Approval.Approved {
		t.Fatal("guide inferred a project or execution approval")
	}
}
