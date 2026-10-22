package stellar

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/app"
	"blog/internal/stellarmigration"
	"blog/internal/store"
)

func migrationLegacyCLIFixture(t *testing.T, markdown string) (string, stellarmigration.RootLayout) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "app.yml"), []byte("auto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ensure := range []func() error{st.EnsureSchema, st.EnsureUsersTable} {
		if err = ensure(); err != nil {
			t.Fatal(err)
		}
	}
	if err = st.PutSetting("site", `{"title":"Old blog","url":"https://fixture.example/","avatar_bg":"/private-fixture.png"}`); err != nil {
		t.Fatal(err)
	}
	if err = st.PutSetting("article", `{"read_next":false}`); err != nil {
		t.Fatal(err)
	}
	if markdown != "" {
		if _, err = st.Create(&store.Post{Slug: "legacy", Title: "Legacy", Status: store.StatusDraft, ContentMD: markdown}); err != nil {
			t.Fatal(err)
		}
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	a, path, err := app.StellarMigrationConfig(app.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	return root, app.StellarMigrationLayout(a, path)
}
func runMigrationTestCLI(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := RunMigrationCLI(context.Background(), args, root, "", &out, &stderr)
	return code, out.String(), stderr.String()
}
func readMigrationTestArtifact(t *testing.T, dir, name string, target any) {
	t.Helper()
	if err := readMigrationPrivateJSON(filepath.Join(dir, name), target); err != nil {
		t.Fatal(err)
	}
}
func migrationSelectedCLIFixture(t *testing.T, markdown string) (string, stellarmigration.RootLayout, string) {
	t.Helper()
	root, layout := migrationLegacyCLIFixture(t, markdown)
	review := filepath.Join(t.TempDir(), "preview")
	if code, out, stderr := runMigrationTestCLI(t, root, "preview", "--out", review); code != 0 {
		t.Fatalf("preview: %d %s %s", code, out, stderr)
	}
	selected := filepath.Join(t.TempDir(), "selected")
	if code, out, stderr := runMigrationTestCLI(t, root, "preview", "--review", review, "--approval", filepath.Join(review, "approval-template.private.json"), "--out", selected); code != 0 {
		t.Fatalf("selected preview: %d %s %s", code, out, stderr)
	}
	return root, layout, selected
}

func TestMigrationCLIHostCapabilitiesAndExplicitOverride(t *testing.T) {
	root, _ := migrationLegacyCLIFixture(t, "")
	if len(migrationHostCapabilities()) != 20 {
		t.Fatal("host capabilities changed without review")
	}
	for name, explicit := range map[string]string{"host": "", "empty": "{}", "null": "null", "partial": `{"avatar_background":true}`} {
		t.Run(name, func(t *testing.T) {
			args := []string{"preview", "--out", filepath.Join(t.TempDir(), "review")}
			if explicit != "" {
				dir := t.TempDir()
				os.Chmod(dir, 0700)
				path := filepath.Join(dir, "capabilities.json")
				if err := os.WriteFile(path, []byte(explicit), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--capabilities", path)
			}
			code, out, stderr := runMigrationTestCLI(t, root, args...)
			if code != 0 {
				t.Fatalf("preview %d %s %s", code, out, stderr)
			}
			var summary migrationCLISummary
			if err := json.Unmarshal([]byte(out), &summary); err != nil {
				t.Fatal(err)
			}
			want := 0
			if name == "empty" || name == "null" {
				want = 2
			}
			if name == "partial" {
				want = 1
			}
			if summary.Blocking != want {
				t.Fatalf("blockers=%d expected=%d", summary.Blocking, want)
			}
			var c stellarmigration.Candidate
			readMigrationTestArtifact(t, summary.Directory, "candidate.private.json", &c)
			if name == "host" {
				raw, err := stellarmigration.Preview(c.Source)
				if err != nil || raw.PublicReport().Blocking != 2 {
					t.Fatal("internal Preview default was loosened")
				}
				var pending migrationCLIApproval
				readMigrationTestArtifact(t, summary.Directory, "approval-template.private.json", &pending)
				if pending.Approval.Approved || pending.Offline.Approved {
					t.Fatal("initial preview approved")
				}
			}
		})
	}
}

func TestMigrationCLIDecisionTemplateKeepsPolicyAndNotebookUnselected(t *testing.T) {
	root, layout := migrationLegacyCLIFixture(t, "")
	st, err := store.OpenExisting(layout.DBPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.PutSetting("plugins", `{"pjax":{"enable":true,"selectors":[".private-value"],"timeout":5000}}`); err != nil {
		t.Fatal(err)
	}
	if err = st.PutSetting("unknown-private-config", `{"secret":"PRIVATE_VALUE"}`); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(`DROP TABLE notes; DROP TABLE notebooks; CREATE TABLE notebooks(id INTEGER PRIMARY KEY,name TEXT,created_at TEXT); INSERT INTO notebooks VALUES(2,'Book','2024-01-01 00:00:00'); CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER,title TEXT,content TEXT,created_at TEXT,updated_at TEXT); INSERT INTO notes VALUES(2,2,'Note','private body','2024-01-01 00:00:00','2024-01-01 00:00:00');`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	review := filepath.Join(t.TempDir(), "review")
	code, out, stderr := runMigrationTestCLI(t, root, "preview", "--out", review)
	if code != 0 {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	var pending migrationCLIApproval
	var guide migrationDecisionGuide
	readMigrationTestArtifact(t, review, "approval-template.private.json", &pending)
	readMigrationTestArtifact(t, review, "decisions-guide.private.json", &guide)
	if len(pending.Notebook.Records) != 2 || len(pending.Approval.Resolutions) < 2 {
		t.Fatal("named missing decisions omitted")
	}
	for _, r := range pending.Approval.Resolutions {
		if r.Action != "" || r.AlternativeName != "" || r.ReplacementURL != "" {
			t.Fatal("policy value inferred")
		}
	}
	for _, r := range pending.Notebook.Records {
		if r.Approved || r.OwnerID != nil || r.Status != nil || r.Slug != nil || r.UpdatedAt != nil {
			t.Fatal("owner, route, status or date inferred")
		}
	}
	hasUnknown := false
	for _, g := range guide.Resolutions {
		if g.Code == "unmapped_setting" && len(g.Actions) == 0 {
			hasUnknown = true
		}
	}
	if !hasUnknown {
		t.Fatal("unknown blocker was offered a bypass")
	}
	code, _, stderr = runMigrationTestCLI(t, root, "preview", "--review", review, "--approval", filepath.Join(review, "approval-template.private.json"), "--out", filepath.Join(t.TempDir(), "selected"))
	if code != 1 || !strings.Contains(stderr, "SELECTIONS_BLOCKED") || strings.Contains(stderr, "PRIVATE_VALUE") {
		t.Fatalf("unselected accepted or leaked: %d %s", code, stderr)
	}
}

func TestMigrationCLIApproveRequiresSelectedArtifactsAndBodyAcceptance(t *testing.T) {
	root, layout, selected := migrationSelectedCLIFixture(t, "{% links %}")
	dir := filepath.Join(t.TempDir(), "approval")
	code, _, stderr := runMigrationTestCLI(t, root, "approve", "--review", selected, "--out", dir)
	if code != 1 || !strings.Contains(stderr, "BODY_APPROVAL_REQUIRED") {
		t.Fatalf("body approval inferred: %d %s", code, stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("failed approval created output")
	}
	before, err := os.ReadFile(layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := runMigrationTestCLI(t, root, "approve", "--review", selected, "--out", dir, "--accept-body-changes")
	if code != 0 {
		t.Fatalf("approval: %d %s %s", code, out, stderr)
	}
	var summary migrationCLISummary
	if err = json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatal(err)
	}
	var approved migrationCLIApproval
	readMigrationTestArtifact(t, dir, "approval.private.json", &approved)
	if !approved.Approval.Approved || approved.Offline.Approved || len(approved.Approval.BodyDigests) != 1 || summary.OfflineApproved {
		t.Fatal("wrong rehearsal/body approval")
	}
	for _, path := range []string{dir, summary.ApprovalFile} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0600)
		if path == dir {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Fatal("private permission lost")
		}
	}
	if code, _, _ := runMigrationTestCLI(t, root, "approve", "--review", selected, "--out", dir, "--accept-body-changes"); code != 1 {
		t.Fatal("overwrote approval directory")
	}
	if code, _, _ := runMigrationTestCLI(t, root, "apply", "--review", selected, "--approval", summary.ApprovalFile); code != 1 {
		t.Fatal("rehearsal-only approval applied")
	}
	var capture stellarmigration.RootCapture
	var candidate stellarmigration.Candidate
	readMigrationTestArtifact(t, selected, "root-capture.private.json", &capture)
	readMigrationTestArtifact(t, selected, "candidate.private.json", &candidate)
	stage, err := stellarmigration.BuildRootRehearsal(context.Background(), capture, candidate, approved.Approval, approved.Notebook)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(stage.Directory) })
	after, err := os.ReadFile(layout.DBPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("approval/rehearsal changed source")
	}
	var pending migrationCLIApproval
	readMigrationTestArtifact(t, selected, "approval-template.private.json", &pending)
	if pending.Approval.Approved || pending.Offline.Approved || len(pending.Approval.BodyDigests) != 0 {
		t.Fatal("approval mutated selection preview")
	}
}

func TestMigrationCLIApproveRejectsTamperedAndExpiredReview(t *testing.T) {
	root, layout, selected := migrationSelectedCLIFixture(t, "")
	// 首次preview缺少经过重建的选值与review digest，不可直接批准。
	initial := filepath.Join(t.TempDir(), "initial")
	if code, _, stderr := runMigrationTestCLI(t, root, "preview", "--out", initial); code != 0 {
		t.Fatal(stderr)
	}
	if _, err := approveMigrationSelections(context.Background(), layout, initial, filepath.Join(t.TempDir(), "bad"), false, false); err == nil {
		t.Fatal("initial preview approved")
	}
	for _, name := range []string{"root-capture.private.json", "candidate.private.json", "selected-candidate.private.json", "reviewed-selections.private.json", "original-report.public.json", "report.public.json", "body-review.public.json", "approval-template.private.json"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(selected, name)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// 保持JSON与主要结构有效，仅篡改一项摘要或被审核的实际值。
			var artifact map[string]any
			if err = json.Unmarshal(original, &artifact); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "root-capture.private.json":
				artifact["digest"] = "tampered"
			case "candidate.private.json", "selected-candidate.private.json":
				artifact["version"] = 3
			case "reviewed-selections.private.json":
				artifact["body_digests"] = map[string]string{"posts/1": "fake"}
			case "original-report.public.json", "report.public.json":
				artifact["blocking"] = -1
			case "body-review.public.json":
				artifact["posts/1"] = "fake"
			case "approval-template.private.json":
				artifact["approval"].(map[string]any)["review_digest"] = "tampered"
			}
			changed, err := json.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}

			if err = os.WriteFile(path, changed, 0600); err != nil {
				t.Fatal(err)
			}
			defer os.WriteFile(path, original, 0600)
			output := filepath.Join(t.TempDir(), "rejected")
			if _, err = approveMigrationSelections(context.Background(), layout, selected, output, true, true); err == nil {
				t.Fatal("tampered review approved")
			}
			if _, err = os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("tampered approval created output")
			}
		})
	}
	if err := os.MkdirAll(filepath.Join(root, "runtime"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "runtime", "new-runtime-state"), []byte("new writer state"), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runMigrationTestCLI(t, root, "approve", "--review", selected, "--out", filepath.Join(t.TempDir(), "stale"))
	if code != 1 || !strings.Contains(stderr, "SOURCE_CHANGED") {
		t.Fatalf("stale root approved: %d %s", code, stderr)
	}
}

func TestMigrationCLIApproveOfflineRequiresBothAttestations(t *testing.T) {
	root, _, selected := migrationSelectedCLIFixture(t, "")
	for _, flags := range [][]string{nil, {"--server-stopped"}, {"--legacy-instances-checked"}, {"--server-stopped", "--legacy-instances-checked"}} {
		args := append([]string{"approve", "--review", selected, "--out", filepath.Join(t.TempDir(), "approved")}, flags...)
		code, out, stderr := runMigrationTestCLI(t, root, args...)
		if code != 0 {
			t.Fatalf("%d %s", code, stderr)
		}
		var summary migrationCLISummary
		if err := json.Unmarshal([]byte(out), &summary); err != nil {
			t.Fatal(err)
		}
		var approved migrationCLIApproval
		readMigrationTestArtifact(t, summary.Directory, "approval.private.json", &approved)
		want := len(flags) == 2
		if summary.OfflineApproved != want || approved.Offline.Approved != want || approved.Offline.StoppedServerAndWorkers != want || approved.Offline.LegacyInstancesChecked != want {
			t.Fatal("partial attestation enabled apply")
		}
	}
}

func TestMigrationCLIFirstPreviewClosedLegacyWALDatabase(t *testing.T) {
	root, layout := migrationLegacyCLIFixture(t, "old body")
	if _, err := os.Stat(layout.DBPath + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("fixture must be a closed WAL database without sidecars: %v", err)
	}
	before, err := os.ReadFile(layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := runMigrationTestCLI(t, root, "preview", "--out", filepath.Join(t.TempDir(), "first-review"))
	if code != 0 {
		t.Fatalf("first preview required manual sidecar warmup: %d %s %s", code, out, stderr)
	}
	after, err := os.ReadFile(layout.DBPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("first preview changed database bytes")
	}
	var summary migrationCLISummary
	if err = json.Unmarshal([]byte(out), &summary); err != nil || !summary.OK {
		t.Fatal("missing first preview proof")
	}
	var capture stellarmigration.RootCapture
	readMigrationTestArtifact(t, summary.Directory, "root-capture.private.json", &capture)
	if err = capture.Verify(); err != nil {
		t.Fatal(err)
	}
	fresh, err := stellarmigration.CaptureMigrationRoot(context.Background(), layout, capture.TrackedSources)
	if err != nil || fresh.Digest != capture.Digest {
		t.Fatalf("sidecar stabilization weakened complete capture: %v", err)
	}
}
