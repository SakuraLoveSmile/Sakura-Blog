package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"blog/internal/app"
	"blog/internal/search"
	"blog/internal/stellarmigration"
	"blog/internal/store"
)

// 正式单binary命令、批准门禁、完整上传保留及演练Bootstrap/SSR不能由mock代替。
func TestStellarMigrationActualBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("actual binary integration")
	}
	bin := filepath.Join(t.TempDir(), "blog")
	build := exec.Command("go", "build", "-o", bin, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	t.Setenv("STELLAR_CLI_BINARY", bin)
	root, _ := cliFixture(t)
	fixtureStore, err := store.OpenExisting(filepath.Join(root, "data", "blog.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = fixtureStore.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "synthetic-admin-hash"}); err != nil {
		t.Fatal(err)
	}
	if _, err = fixtureStore.Create(&store.Post{Slug: "canonical-search", Title: "Canonical search", Status: store.StatusPublished, ContentMD: "CurrentCanonicalOnly", ContentHTML: "<p>ObsoleteCachedOnly</p>", StellarVersion: 2, StellarConfig: `{"title":"Canonical search"}`, BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
	fixtureStore.Close()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	appPath := filepath.Join(root, "config", "app.yml")
	if err = os.WriteFile(appPath, []byte("listen_addr: "+addr+"\nauto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "config", "site.yml"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// 稳定正式WAL读取侧车，再捕获用于完整批准的资源前像。
	if _, err = stellarmigration.ReadSnapshot(context.Background(), filepath.Join(root, "data", "blog.db"), nil); err != nil {
		t.Fatal(err)
	}
	// 异常退出后的普通CLI不得先打开Store，也不能隐式恢复真实源。
	journalPath := filepath.Join(root, "data", ".stellar-migration-journal.json")
	journal := []byte("synthetic-pending-journal")
	lockedDB, err := os.ReadFile(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(journalPath, journal, 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"doctor", "--format", "json"}, {"new", "note", "--notebook", "journal", "--title", "Pending must not write"}, {"images"}} {
		stdout, stderr, code := cliProcess(t, root, command...)
		if code != 1 || len(stdout) != 0 || !bytes.Contains(stderr, []byte("stellar migrate recover")) {
			t.Fatalf("pending journal command %s: %d %s %s", command[0], code, stdout, stderr)
		}
	}
	unchangedDB, err := os.ReadFile(filepath.Join(root, "data", "blog.db"))
	if err != nil || !bytes.Equal(lockedDB, unchangedDB) {
		t.Fatal("ordinary command wrote pending source DB")
	}
	unchangedJournal, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(journal, unchangedJournal) {
		t.Fatal("ordinary command implicitly recovered pending journal")
	}
	if _, err = os.Stat(filepath.Join(root, "content", "_data", "caches", "images_metadata.json")); !os.IsNotExist(err) {
		t.Fatal("images wrote cache before migration recovery")
	}
	if err = os.Remove(journalPath); err != nil {
		t.Fatal(err)
	}
	preview := filepath.Join(t.TempDir(), "review")
	stdout, stderr, code := cliProcess(t, root, "migrate", "preview", "--out", preview)
	if code != 0 {
		t.Fatalf("preview %d %s %s", code, stdout, stderr)
	}
	if bytes.Contains(stdout, []byte("fixture-password")) || bytes.Contains(stderr, []byte("fixture-password")) {
		t.Fatal("secret printed")
	}
	for _, name := range []string{"root-capture.private.json", "candidate.private.json", "approval-template.private.json", "report.public.json"} {
		info, e := os.Stat(filepath.Join(preview, name))
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("private mode %s %v", name, e)
		}
	}
	var capture stellarmigration.RootCapture
	var candidate stellarmigration.Candidate
	read := func(name string, target any) {
		data, e := os.ReadFile(filepath.Join(preview, name))
		if e != nil {
			t.Fatal(e)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if e = decoder.Decode(target); e != nil {
			t.Fatal(e)
		}
	}
	read("root-capture.private.json", &capture)
	read("candidate.private.json", &candidate)
	if candidate.PublicReport().Blocking != 0 {
		t.Fatalf("canonical fixture has conflicts: %+v", candidate.PublicReport())
	}
	before, err := os.ReadFile(capture.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _, code = cliProcess(t, root, "migrate", "apply", "--review", preview, "--approval", filepath.Join(preview, "approval-template.private.json"))
	if code != 1 {
		t.Fatal("unapproved template applied")
	}
	approved := struct {
		Approval stellarmigration.RootMigrationApproval      `json:"approval"`
		Notebook stellarmigration.NotebookStructureDecisions `json:"notebook"`
		Offline  map[string]any                              `json:"offline_attestation"`
	}{Approval: stellarmigration.RootMigrationApproval{MigrationApproval: stellarmigration.MigrationApproval{Approved: true, SourceDigest: capture.Source.Digest, CandidateDigest: stellarmigration.CandidateDigest(candidate)}, RootDigest: capture.Digest}, Offline: map[string]any{"approved": true, "root_digest": capture.Digest, "stopped_server_and_workers": true, "legacy_instances_checked": true}}
	approvalPath := filepath.Join(preview, "approved.private.json")
	approved.Approval.ReviewDigest, err = stellarmigration.RootReviewDigest(capture, candidate, approved.Approval.Resolutions, approved.Notebook)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(approved)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(approvalPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	selectedReview := filepath.Join(t.TempDir(), "selected-review")
	stdout, stderr, code = cliProcess(t, root, "migrate", "preview", "--review", preview, "--approval", approvalPath, "--out", selectedReview)
	if code != 0 {
		t.Fatalf("selected pure preview %d %s %s", code, stdout, stderr)
	}
	var selectedApproval struct {
		Approval stellarmigration.RootMigrationApproval `json:"approval"`
		Offline  struct {
			Approved bool `json:"approved"`
		} `json:"offline_attestation"`
	}
	template, err := os.ReadFile(filepath.Join(selectedReview, "approval-template.private.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(template, &selectedApproval); err != nil {
		t.Fatal(err)
	}
	if selectedApproval.Approval.Approved || selectedApproval.Offline.Approved || len(selectedApproval.Approval.BodyDigests) != 0 || selectedApproval.Approval.ReviewDigest != approved.Approval.ReviewDigest {
		t.Fatal("selection preview granted body or execution approval")
	}
	// 用户无需手写最终批准JSON；真实approve命令重建完整审核，未声明离线时只能演练。
	rehearsalApprovalDir := filepath.Join(t.TempDir(), "rehearsal-approval")
	stdout, stderr, code = cliProcess(t, root, "migrate", "approve", "--review", selectedReview, "--out", rehearsalApprovalDir)
	if code != 0 {
		t.Fatalf("approve rehearsal %d %s %s", code, stdout, stderr)
	}
	rehearsalApprovalPath := filepath.Join(rehearsalApprovalDir, "approval.private.json")
	_, _, code = cliProcess(t, root, "migrate", "apply", "--review", selectedReview, "--approval", rehearsalApprovalPath)
	if code != 1 {
		t.Fatal("rehearsal-only approval applied")
	}
	offlineApprovalDir := filepath.Join(t.TempDir(), "offline-approval")
	stdout, stderr, code = cliProcess(t, root, "migrate", "approve", "--review", selectedReview, "--out", offlineApprovalDir, "--server-stopped", "--legacy-instances-checked")
	if code != 0 {
		t.Fatalf("approve offline %d %s %s", code, stdout, stderr)
	}
	approvalPath = filepath.Join(offlineApprovalDir, "approval.private.json")
	preview = selectedReview
	debugRehearsal, err := stellarmigration.BuildRootRehearsal(context.Background(), capture, candidate, approved.Approval, approved.Notebook)
	if err != nil {
		t.Fatalf("core rehearsal preflight: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(debugRehearsal.Directory) })
	validation := exec.Command(bin, "-root", debugRehearsal.Layout.Root, "-config", debugRehearsal.Layout.AppPath, "stellar", "migrate", "validate-staged")
	validation.Env = []string{"PATH=" + os.Getenv("PATH"), "BLOG_STELLAR_MIGRATION_VALIDATION=1", "BLOG_AUTO_BACKUP=false", "BLOG_SEED=false"}
	if err = validation.Run(); err != nil {
		diagnostic, _ := os.ReadFile(filepath.Join(debugRehearsal.Layout.Root, ".stellar-migration-validation.private.json"))
		t.Fatalf("staged bootstrap preflight: %v %s", err, diagnostic)
	}
	// 与实际服务相同的shared guard证明CLI排它互斥，不只检查maintenance.lock。
	config, configFile, err := app.StellarMigrationConfig(app.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	guard, err := app.AcquireStellarRootGuard(config, configFile, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, code = cliProcess(t, root, "migrate", "apply", "--review", preview, "--approval", approvalPath)
	if code != 1 {
		t.Fatal("apply crossed active lifetime guard")
	}
	guard.Close()
	stdout, stderr, code = cliProcess(t, root, "migrate", "rehearse", "--review", preview, "--approval", rehearsalApprovalPath)
	if code != 0 {
		t.Fatalf("rehearse %d %s %s", code, stdout, stderr)
	}
	var rehearsed struct {
		Directory string `json:"directory"`
	}
	if err = json.Unmarshal(stdout, &rehearsed); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(rehearsed.Directory) })
	after, err := os.ReadFile(capture.Layout.DBPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rehearsal changed source DB")
	}
	// 源上传变更使完整RootDigest失效；恢复字节后才可Apply。
	imagePath := filepath.Join(root, "data", "uploads", "fixture.png")
	image, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(imagePath, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code = cliProcess(t, root, "migrate", "preview", "--review", preview, "--approval", approvalPath, "--out", filepath.Join(t.TempDir(), "stale-review"))
	if code != 1 {
		t.Fatal("selected preview reused stale complete root")
	}
	_, _, code = cliProcess(t, root, "migrate", "apply", "--review", preview, "--approval", approvalPath)
	if code != 1 {
		t.Fatal("stale upload digest applied")
	}
	if err = os.WriteFile(imagePath, image, 0644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = cliProcess(t, root, "migrate", "apply", "--review", preview, "--approval", approvalPath)
	if code != 0 {
		t.Fatalf("apply %d %s %s", code, stdout, stderr)
	}
	if bytes.Contains(stdout, []byte("fixture-password")) || bytes.Contains(stderr, []byte("fixture-password")) {
		t.Fatal("apply secret printed")
	}
	imageAfter, err := os.ReadFile(imagePath)
	if err != nil || !bytes.Equal(image, imageAfter) {
		t.Fatal("apply changed upload")
	}
	server := exec.Command(bin, "-root", root)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "BLOG_") {
			server.Env = append(server.Env, entry)
		}
	}
	var serverLog bytes.Buffer
	server.Stdout = &serverLog
	server.Stderr = &serverLog
	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Wait() }()
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			server.Process.Kill()
			<-done
		}
	})
	client := http.Client{Timeout: 200 * time.Millisecond}
	ready := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		response, e := client.Get("http://" + addr + "/")
		if e == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		select {
		case e := <-done:
			stopped = true
			t.Fatalf("migrated server stopped before ready: %v %s", e, serverLog.String())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("migrated single binary did not serve actual SSR")
	}
	_, _, code = cliProcess(t, root, "migrate", "apply", "--review", preview, "--approval", approvalPath)
	if code != 1 {
		t.Fatal("apply crossed actual running server")
	}
	if err = server.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		stopped = true
		if err != nil {
			t.Fatalf("server did not drain and close: %v %s", err, serverLog.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server shutdown timed out")
	}
	st, err := store.OpenExisting(capture.Layout.DBPath, true)
	if err != nil {
		t.Fatal(err)
	}
	note, err := st.GetNoteBySlug("journal", "public")
	if err != nil || note == nil || note.AuthorID == 0 {
		t.Fatal("canonical data or owner lost")
	}
	items, err := search.New(st.DB()).Search("CurrentCanonicalOnly", search.TypePost, 20)
	if err != nil || len(items) != 1 || items[0].Slug != "canonical-search" {
		t.Fatalf("staged canonical search not installed: %+v %v", items, err)
	}
	items, err = search.New(st.DB()).Search("ObsoleteCachedOnly", "", 20)
	if err != nil || len(items) != 0 {
		t.Fatal("old HTML cache indexed after migration")
	}
	st.Close()
	stdout, stderr, code = cliProcess(t, root, "migrate", "recover")
	if code != 0 {
		t.Fatalf("recover %d %s %s", code, stdout, stderr)
	}
}
