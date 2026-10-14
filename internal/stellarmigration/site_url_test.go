package stellarmigration

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
)

const selectedSiteURL = "https://blog.xn--fhqths51enha.cn/"

func siteURLResolution(address string) MigrationResolution {
	return MigrationResolution{Source: "settings", Code: "site_url_required", Path: "site.url", Action: "replace_url", ReplacementURL: address}
}

func TestSiteURLRequiresNamedDecision(t *testing.T) {
	for _, value := range []any{"", "/blog", "//example.test/blog", "javascript:alert(1)", "https://user:pass@example.test/", "https://example.test/?q=1", "https://example.test/#fragment", 42, nil} {
		t.Run(encode(value), func(t *testing.T) {
			s := settingsSnapshot(t, map[string]any{"site": map[string]any{"url": value}})
			c := candidate(t, s)
			if !hasCode(c.Theme.Diagnostics, "site_url_required") || c.PublicReport().Blocking < 1 {
				t.Fatal("invalid host URL did not produce the named blocking policy")
			}
			if ValidateCandidate(c, s) == nil {
				t.Fatal("source URL without an explicit choice was accepted")
			}
			if _, err := PreviewReviewedSelections(c, nil, NotebookStructureDecisions{}); err == nil {
				t.Fatal("review silently invented a site URL")
			}
		})
	}
}

func TestSiteURLSelectionAndFixtureKeepAllOtherSourceValues(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{
		"site":  map[string]any{"url": "", "title": "Synthetic fixture", "language": "zh-CN", "per_page": 10, "subtitle": "", "keywords": "unchanged"},
		"admin": map[string]any{"opaque": "SYNTHETIC_PRIVATE_FIXTURE"}, "dashboard": map[string]any{"enabled": false},
	})
	c := candidate(t, s)
	before := CandidateDigest(c)
	resolutions := []MigrationResolution{siteURLResolution(selectedSiteURL)}
	p, err := PreviewReviewedSelections(c, resolutions, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Candidate.Theme.SiteURL != selectedSiteURL || p.Candidate.PublicReport().Blocking != 0 || len(p.Resolutions) != 1 || len(p.BodyDigests) != 0 {
		t.Fatal("named site URL preview did not preserve the review boundary")
	}
	if CandidateDigest(c) != before || encode(c.Source) != encode(s) || encode(p.Candidate.Source) != encode(s) || p.Candidate.Source.Digest != s.Digest {
		t.Fatal("selected site URL changed the original candidate/source digest")
	}
	if err = ValidateCandidate(p.Candidate, s); err != nil {
		t.Fatal(err)
	}
	a := approveFixture(c)
	a.Resolutions = resolutions
	pending := a
	pending.Approved = false
	if f, err := BuildMigrationFixture(context.Background(), c, pending, NotebookStructureDecisions{}); err == nil || f.Directory != "" {
		t.Fatal("site URL choice silently approved fixture execution")
	}
	if f, err := BuildMigrationFixture(context.Background(), c, approveFixture(c), NotebookStructureDecisions{}); err == nil || f.Directory != "" {
		t.Fatal("complete approval without the named choice was accepted")
	}
	f, err := BuildMigrationFixture(context.Background(), c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	archiveBytes, err := os.ReadFile(f.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var archive struct {
		Reviewed Candidate `json:"reviewed"`
		Resolved Candidate `json:"resolved"`
	}
	if err = json.Unmarshal(archiveBytes, &archive); err != nil || encode(archive.Reviewed.Source) != encode(s) || archive.Reviewed.Theme.SiteURL != "" || archive.Resolved.Theme.SiteURL != selectedSiteURL {
		t.Fatal("private source archive lost the original URL or selected decision")
	}
	db, err := sql.Open("sqlite", f.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, row := range s.Tables["settings"].Rows {
		section := stringValue(row["section"])
		var got string
		if err = db.QueryRow("SELECT value FROM settings WHERE section=?", section).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if section != "site" {
			if got != stringValue(row["value"]) {
				t.Fatal("an unrelated setting changed")
			}
			continue
		}
		original, _ := decode(stringValue(row["value"]))
		want := object(original)
		want["url"] = selectedSiteURL
		value, err := decode(got)
		if err != nil || encode(value) != encode(want) {
			t.Fatal("fixture site fields other than url changed")
		}
	}
	var blog string
	if err = db.QueryRow("SELECT value FROM settings WHERE section='blog_extensions'").Scan(&blog); err != nil {
		t.Fatal(err)
	}
	if blog != p.Candidate.Theme.BlogJSON {
		t.Fatal("fixture Blog settings differ from the selected preview")
	}
	value, _ := decode(blog)
	if url, _ := get(object(value), "site.url"); url != selectedSiteURL {
		t.Fatal("Blog URL did not match the host URL")
	}
}

func TestSiteURLRejectsInvalidOrMisnamedResolutions(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{"site": map[string]any{"url": ""}})
	c := candidate(t, s)
	for name, decision := range map[string]MigrationResolution{
		"wrong source": {Source: "posts/1", Code: "site_url_required", Path: "site.url", Action: "replace_url", ReplacementURL: selectedSiteURL},
		"wrong path":   {Source: "settings", Code: "site_url_required", Path: "url", Action: "replace_url", ReplacementURL: selectedSiteURL},
		"wrong action": {Source: "settings", Code: "site_url_required", Path: "site.url", Action: "use_official"},
		"relative":     siteURLResolution("/blog"), "userinfo": siteURLResolution("https://user@example.test/"),
		"query": siteURLResolution("https://example.test/?"), "fragment": siteURLResolution("https://example.test/#"),
		"missing host": siteURLResolution("https:///blog"), "empty hostname": siteURLResolution("http://:8080/"),
		"unsafe scheme": siteURLResolution("file:///tmp/blog"), "empty": siteURLResolution(""),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := PreviewReviewedSelections(c, []MigrationResolution{decision}, NotebookStructureDecisions{}); err == nil {
				t.Fatal("invalid or misnamed site URL decision accepted")
			}
		})
	}
}

func TestSiteURLCandidateCannotBypassDiagnosticOrChangeBlogURL(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{"site": map[string]any{"url": ""}})
	c := candidate(t, s)
	p, err := PreviewReviewedSelections(c, []MigrationResolution{siteURLResolution(selectedSiteURL)}, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Candidate){
		"no chosen URL":        func(c *Candidate) { c.Theme.SiteURL = "" },
		"different chosen URL": func(c *Candidate) { c.Theme.SiteURL = "https://tampered.example/" },
		"unsafe chosen URL":    func(c *Candidate) { c.Theme.SiteURL = "javascript:alert(1)" },
		"no named diagnostic":  func(c *Candidate) { c.Theme.Diagnostics = nil },
		"fake diagnostic":      func(c *Candidate) { c.Theme.Diagnostics[0].Kind = "consumer" },
		"different Blog URL":   func(c *Candidate) { c.Theme.BlogJSON = `{"site":{"url":"https://tampered.example/"}}` },
	} {
		t.Run(name, func(t *testing.T) {
			copy := clone(p.Candidate)
			mutate(&copy)
			if ValidateCandidate(copy, s) == nil {
				t.Fatal("altered site URL candidate accepted")
			}
		})
	}
	// 即使重算原候选approval，预填地址也不是完整原始Preview，不能越过具名选择。
	forged := clone(c)
	forged.Theme = p.Candidate.Theme
	if _, _, _, err := resolveMigration(forged, approveFixture(forged), NotebookStructureDecisions{}); err == nil {
		t.Fatal("selected URL injected into original preview accepted")
	}
}

func TestValidSiteURLNeedsNoDecisionAndCannotBeOverwritten(t *testing.T) {
	for _, address := range []string{"https://example.test/", "http://localhost:8080", "https://example.test/sub/directory", "https://example.test/sub/%E4%B8%AD/"} {
		t.Run(address, func(t *testing.T) {
			s := settingsSnapshot(t, map[string]any{"site": map[string]any{"url": address}})
			c := candidate(t, s)
			if hasCode(c.Theme.Diagnostics, "site_url_required") || c.Theme.SiteURL != "" || ValidateCandidate(c, s) != nil {
				t.Fatal("valid source URL requires a replacement")
			}
			if _, err := PreviewReviewedSelections(c, []MigrationResolution{siteURLResolution(selectedSiteURL)}, NotebookStructureDecisions{}); err == nil {
				t.Fatal("valid source URL overwritten by an unmatched choice")
			}
			copy := clone(c)
			copy.Theme.SiteURL = selectedSiteURL
			if ValidateCandidate(copy, s) == nil {
				t.Fatal("host update with valid source URL accepted")
			}
			if _, exists := objectJSON(t, encode(c.Theme))["site_url"]; exists {
				t.Fatal("unselected optional host change emitted")
			}
			f, err := BuildMigrationFixture(context.Background(), c, approveFixture(c), NotebookStructureDecisions{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
			db, err := sql.Open("sqlite", f.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var got string
			if err = db.QueryRow("SELECT value FROM settings WHERE section='site'").Scan(&got); err != nil || got != stringValue(s.Tables["settings"].Rows[0]["value"]) {
				t.Fatal("valid host setting raw JSON was overwritten")
			}
		})
	}
	s := settingsSnapshot(t, map[string]any{"site": map[string]any{"title": "Fixture without URL"}})
	p, err := PreviewReviewedSelections(candidate(t, s), []MigrationResolution{siteURLResolution("https://example.test/sub/without-trailing-slash")}, NotebookStructureDecisions{})
	if err != nil || p.Candidate.Theme.SiteURL != "https://example.test/sub/without-trailing-slash" {
		t.Fatal("valid literal subdirectory replacement was changed or rejected")
	}
}

func objectJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	value, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return object(value)
}

func TestCanonicalThemeStillRequiresHostSiteURL(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{"site": map[string]any{"url": ""}, "stellar_v2": map[string]any{"version": 2, "theme": map[string]any{}}, "blog_extensions": map[string]any{"site": map[string]any{"title": "Canonical fixture", "url": ""}}})
	c := candidate(t, s)
	if !c.Theme.Preserved || !hasCode(c.Theme.Diagnostics, "site_url_required") || c.PublicReport().Blocking != 1 {
		t.Fatal("already canonical theme bypassed the invalid host URL")
	}
	p, err := PreviewReviewedSelections(c, []MigrationResolution{siteURLResolution(selectedSiteURL)}, NotebookStructureDecisions{})
	if err != nil || p.Candidate.Theme.SiteURL != selectedSiteURL || p.Candidate.Theme.ThemeJSON != c.Theme.ThemeJSON || p.Candidate.Theme.DocumentJSON != c.Theme.DocumentJSON {
		t.Fatalf("canonical theme URL selection changed upstream values: %v", err)
	}
}

func TestSiteURLCompleteRootReviewBindsDecisionAndIsolatedWrite(t *testing.T) {
	r, _, _ := completeRootSource(t)
	db, err := sql.Open("sqlite", r.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO settings(section,value) VALUES('site',?)", `{"title":"Root URL fixture","url":"","language":"zh-CN"}`)
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("prepare isolated source: %v %v", err, closeErr)
	}
	_ = snapshot(t, r.Layout.DBPath, nil)
	r, err = CaptureMigrationRoot(context.Background(), r.Layout, r.TrackedSources)
	if err != nil {
		t.Fatal(err)
	}
	c := candidate(t, r.Source)
	a := RootMigrationApproval{MigrationApproval: approveFixture(c), RootDigest: r.Digest}
	a.Resolutions = []MigrationResolution{{Source: "content/posts/tracked.md", Code: "file_host_fields", Path: "status", Action: "preserve_status"}, siteURLResolution(selectedSiteURL)}
	a.ReviewDigest, err = RootReviewDigest(r, c, a.Resolutions, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = validateRootReview(r, c, a, NotebookStructureDecisions{}); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RootMigrationApproval){
		"different URL":    func(a *RootMigrationApproval) { a.Resolutions[1].ReplacementURL = "https://changed.example/sub/" },
		"different action": func(a *RootMigrationApproval) { a.Resolutions[1].Action = "use_official" },
		"missing choice":   func(a *RootMigrationApproval) { a.Resolutions = a.Resolutions[:1] },
	} {
		t.Run(name, func(t *testing.T) {
			copy := clone(a)
			mutate(&copy)
			if validateRootReview(r, c, copy, NotebookStructureDecisions{}) == nil {
				t.Fatal("changed decision retained the original complete review approval")
			}
		})
	}
	p, err := PreviewReviewedSelections(c, a.Resolutions, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	tampered := clone(c)
	tampered.Theme.SiteURL = p.Candidate.Theme.SiteURL
	if validateRootReview(r, tampered, a, NotebookStructureDecisions{}) == nil {
		t.Fatal("URL changed in original candidate retained complete review approval")
	}
	before, err := os.ReadFile(r.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	f, err := BuildRootRehearsal(context.Background(), r, c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	after, err := os.ReadFile(r.Layout.DBPath)
	if err != nil || bytesDigest(before) != bytesDigest(after) {
		t.Fatal("root rehearsal modified its source DB")
	}
	again, err := CaptureMigrationRoot(context.Background(), r.Layout, r.TrackedSources)
	if err != nil || again.Digest != r.Digest || again.Source.Digest != r.Source.Digest {
		t.Fatalf("root rehearsal modified source/root digest: %v", err)
	}
	db, err = sql.Open("sqlite", f.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var site, blog string
	if err = db.QueryRow("SELECT value FROM settings WHERE section='site'").Scan(&site); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT value FROM settings WHERE section='blog_extensions'").Scan(&blog); err != nil {
		t.Fatal(err)
	}
	if objectJSON(t, site)["url"] != selectedSiteURL {
		t.Fatal("isolated full root did not receive the selected host URL")
	}
	if value, _ := get(objectJSON(t, blog), "site.url"); value != selectedSiteURL {
		t.Fatal("isolated full root Blog URL differs")
	}
}
