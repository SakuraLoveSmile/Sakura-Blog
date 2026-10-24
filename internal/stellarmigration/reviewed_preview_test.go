package stellarmigration

import (
	"context"
	"testing"
)

func TestReviewedSelectionPreviewDoesNotApproveBodiesOrExecution(t *testing.T) {
	s := legacyStructureFixture(t)
	c := candidate(t, s)
	before := CandidateDigest(c)
	resolutions := []MigrationResolution{{Source: "schema", Code: "legacy_notebook_schema", Path: "notebooks/notes", Action: "use_notebook_decisions"}, {Source: "notes/9007199254740993", Code: "note_missing_slug_status", Path: "slug/status", Action: "use_notebook_decisions"}}
	p, err := PreviewReviewedSelections(c, resolutions, structureApprovals(s))
	if err != nil {
		t.Fatal(err)
	}
	if CandidateDigest(c) != before || len(p.Resolutions) != 2 || p.Notebook.Blocking() || len(p.BodyDigests) != 1 {
		t.Fatal("selection preview changed source or lost pending body")
	}
	if p.BodyDigests["notes/9007199254740993"] != bytesDigest([]byte(c.Records[1].Markdown)) {
		t.Fatal("body review digest differs from exact transformed bytes")
	}
	pending := MigrationApproval{SourceDigest: s.Digest, CandidateDigest: before, Resolutions: resolutions}
	if f, err := BuildMigrationFixture(context.Background(), c, pending, structureApprovals(s)); err == nil || f.Directory != "" {
		t.Fatal("review preview silently approved execution")
	}
	pending.Approved = true
	if f, err := BuildMigrationFixture(context.Background(), c, pending, structureApprovals(s)); err == nil || f.Directory != "" {
		t.Fatal("review preview silently approved body")
	}
	if _, err := PreviewReviewedSelections(c, resolutions[:1], structureApprovals(s)); err == nil {
		t.Fatal("incomplete named choices bypassed diagnostic")
	}
}
