package stellar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"blog/internal/app"
	"blog/internal/backup"
	"blog/internal/config"
	"blog/internal/search"
	"blog/internal/stellarcli"
	"blog/internal/stellarmigration"
	"blog/modules/stellar/routes"
	"blog/modules/stellar/web"
	"golang.org/x/sys/unix"
)

type migrationOfflineAttestation struct {
	Approved                bool   `json:"approved"`
	RootDigest              string `json:"root_digest"`
	StoppedServerAndWorkers bool   `json:"stopped_server_and_workers"`
	LegacyInstancesChecked  bool   `json:"legacy_instances_checked"`
}
type migrationCLIApproval struct {
	Approval stellarmigration.RootMigrationApproval      `json:"approval"`
	Notebook stellarmigration.NotebookStructureDecisions `json:"notebook"`
	Offline  migrationOfflineAttestation                 `json:"offline_attestation"`
}
type migrationCLISummary struct {
	OK                      bool     `json:"ok"`
	Operation               string   `json:"operation"`
	Directory               string   `json:"directory,omitempty"`
	RootDigest              string   `json:"root_digest,omitempty"`
	SourceDigest            string   `json:"source_digest,omitempty"`
	CandidateDigest         string   `json:"candidate_digest,omitempty"`
	ReviewDigest            string   `json:"review_digest,omitempty"`
	SelectedCandidateDigest string   `json:"selected_candidate_digest,omitempty"`
	Blocking                int      `json:"blocking,omitempty"`
	Recovered               bool     `json:"recovered,omitempty"`
	BackupDirectory         string   `json:"backup_directory,omitempty"`
	BinarySHA256            string   `json:"runtime_binary_sha256"`
	ApprovalFile            string   `json:"approval_file,omitempty"`
	OfflineApproved         bool     `json:"offline_approved"`
	NextSteps               []string `json:"next_steps,omitempty"`
}
type migrationTrackedFlags []string

// RunResolvedToolCLI确保普通创作/图片命令打开已持lifetime guard的配置快照。
func RunResolvedToolCLI(ctx context.Context, args []string, resolved *config.App, stdout, stderr io.Writer) int {
	env, err := ToolEnvironment()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return stellarcli.Run(ctx, args, stellarcli.Options{ResolvedApp: resolved, Environment: env, Images: PrepareToolImages}, stdout, stderr)
}

func (s *migrationTrackedFlags) String() string     { return strings.Join(*s, ",") }
func (s *migrationTrackedFlags) Set(v string) error { *s = append(*s, v); return nil }

// RunMigrationCLI不自动批准候选或正文。完整私密前像只写权限受限的artifact，
// stdout输出脱敏摘要。真实Apply仍须完整审查后明确执行，不等于批准候选取值。
func RunMigrationCLI(ctx context.Context, args []string, root, configPath string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: stellar migrate preview|approve|rehearse|apply|recover")
		return 2
	}
	op := args[0]
	flags := flag.NewFlagSet("stellar migrate "+op, flag.ContinueOnError)
	flags.SetOutput(stderr)
	var out, review, approval, capabilities string
	var tracked migrationTrackedFlags
	var acceptBody, serverStopped, legacyChecked bool
	flags.StringVar(&out, "out", "", "new private preview directory outside runtime resources")
	flags.StringVar(&review, "review", "", "private preview directory")
	flags.StringVar(&approval, "approval", "", "private decisions template or explicit approval JSON")
	flags.StringVar(&capabilities, "capabilities", "", "explicit verified consumer capability map")
	flags.BoolVar(&acceptBody, "accept-body-changes", false, "approve the reviewed Markdown changes by digest")
	flags.BoolVar(&serverStopped, "server-stopped", false, "attest all servers and workers are stopped")
	flags.BoolVar(&legacyChecked, "legacy-instances-checked", false, "attest legacy instances cannot write runtime resources")
	flags.Var(&tracked, "tracked", "reviewed content source path relative to root; repeat for each source")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return 2
	}
	if op != "preview" && op != "approve" && op != "rehearse" && op != "apply" && op != "recover" && op != "validate-staged" {
		fmt.Fprintln(stderr, "unknown migration operation")
		return 2
	}
	if op != "approve" && (acceptBody || serverStopped || legacyChecked) {
		fmt.Fprintln(stderr, "approval flags are only valid with migrate approve")
		return 2
	}
	a, appPath, err := app.StellarMigrationConfig(app.Options{Root: root, ConfigPath: configPath})
	if err != nil {
		return migrationCLIError(stderr, "CONFIG_INVALID")
	}
	binaryHash, err := migrationBinaryHash()
	if err != nil {
		return migrationCLIError(stderr, "BINARY_UNAVAILABLE")
	}
	if op == "validate-staged" {
		g, e := app.AcquireStellarRootGuard(a, appPath, false)
		if e != nil {
			return migrationCLIError(stderr, "MIGRATION_REJECTED")
		}
		defer g.Close()
		a, appPath = g.AppSnapshot, g.AppPath
		if os.Getenv("BLOG_STELLAR_MIGRATION_VALIDATION") != "1" || !isPrivateMigrationRehearsal(a.Root, a.DBPath, a.UploadDir, appPath) {
			return migrationCLIError(stderr, "RUNTIME_VALIDATION_FAILED")
		}
		if err = validateMigrationRuntime(ctx, a, appPath); err != nil {
			_ = writeMigrationPrivateJSON(filepath.Join(a.Root, ".stellar-migration-validation.private.json"), struct {
				Error string `json:"error"`
			}{err.Error()})
			return migrationCLIError(stderr, "RUNTIME_VALIDATION_FAILED")
		}
		if json.NewEncoder(stdout).Encode(migrationCLISummary{OK: true, Operation: op, BinarySHA256: binaryHash}) != nil {
			return 1
		}
		return 0
	}
	g, err := app.AcquireStellarRootGuard(a, appPath, op == "apply" || op == "recover")
	if err != nil {
		fmt.Fprintln(stderr, "migration root is in use or its lifetime guard is unavailable")
		return 1
	}
	defer g.Close()
	a, appPath = g.AppSnapshot, g.AppPath
	layout := app.StellarMigrationLayout(a, appPath)
	var summary migrationCLISummary
	summary.Operation = op
	summary.OK = true
	summary.BinarySHA256 = binaryHash
	switch op {
	case "preview":
		if review != "" || approval != "" {
			if review == "" || approval == "" || capabilities != "" || len(tracked) != 0 {
				fmt.Fprintln(stderr, "reviewed preview requires --review and --approval with the existing capture inventory")
				return 2
			}
			selected, e := previewMigrationSelections(ctx, layout, review, approval, out)
			if e != nil {
				return migrationCLIError(stderr, migrationApprovalErrorCode(e))
			}
			selected.BinarySHA256 = binaryHash
			if json.NewEncoder(stdout).Encode(selected) != nil {
				return 1
			}
			return 0
		}
		caps := migrationHostCapabilities()
		if capabilities != "" {
			caps = nil // 显式映射完整覆盖宿主默认值，包括空对象与null。
			if err = readMigrationPrivateJSON(capabilities, &caps); err != nil {
				return migrationCLIError(stderr, "CAPABILITIES_INVALID")
			}
		}
		sources, e := migrationSourceInventory(layout.Root, tracked)
		if e != nil {
			return migrationCLIError(stderr, "INVENTORY_INVALID")
		}
		// WAL旧库第一次只读连接可能创建侧车，先以mode=ro稳定SQLite读路径。
		// 随后的完整资源捕获仍双读严格比较；不删除侧车、不允许业务数据写入。
		if _, e = stellarmigration.ReadSnapshot(ctx, layout.DBPath, nil); e != nil {
			return migrationCLIError(stderr, "CAPTURE_FAILED")
		}
		capture, e := stellarmigration.CaptureMigrationRoot(ctx, layout, sources)
		if e != nil {
			return migrationCLIError(stderr, "CAPTURE_FAILED")
		}
		after, e := migrationSourceInventory(layout.Root, tracked)
		if e != nil || !reflect.DeepEqual(sources, after) {
			return migrationCLIError(stderr, "SOURCE_CHANGED")
		}
		candidate, e := stellarmigration.PreviewWithCapabilities(capture.Source, caps)
		if e != nil {
			return migrationCLIError(stderr, "PREVIEW_FAILED")
		}
		dir, e := createMigrationPreviewDirectory(out, layout)
		if e != nil {
			return migrationCLIError(stderr, "OUTPUT_INVALID")
		}
		unapproved, guide := migrationDecisionTemplate(capture, candidate)
		for _, f := range []struct {
			name  string
			value any
		}{{"root-capture.private.json", capture}, {"candidate.private.json", candidate}, {"report.public.json", candidate.PublicReport()}, {"approval-template.private.json", unapproved}, {"decisions-guide.private.json", guide}} {
			if e = writeMigrationPrivateJSON(filepath.Join(dir, f.name), f.value); e != nil {
				return migrationCLIError(stderr, "OUTPUT_INVALID")
			}
		}
		summary.Directory = dir
		summary.RootDigest = capture.Digest
		summary.SourceDigest = capture.Source.Digest
		summary.CandidateDigest = stellarmigration.CandidateDigest(candidate)
		summary.Blocking = candidate.PublicReport().Blocking
		summary.NextSteps = migrationSelectionNextSteps()
	case "approve":
		if review == "" || out == "" || approval != "" || capabilities != "" || len(tracked) != 0 {
			fmt.Fprintln(stderr, "approve requires --review <selected-review> --out <new-private-directory>; run a selected preview first")
			return 2
		}
		approved, e := approveMigrationSelections(ctx, layout, review, out, acceptBody, serverStopped && legacyChecked)
		if e != nil {
			return migrationCLIError(stderr, migrationApprovalErrorCode(e))
		}
		approved.BinarySHA256 = binaryHash
		if json.NewEncoder(stdout).Encode(approved) != nil {
			return 1
		}
		return 0
	case "recover":
		if err = app.CheckStellarOfflineListen(a.ListenAddr); err != nil {
			fmt.Fprintln(stderr, "configured server port is occupied or cannot prove offline status")
			return 1
		}
		result, e := stellarmigration.RecoverRootMigration(ctx, layout)
		if e != nil {
			return migrationCLIError(stderr, "RECOVERY_FAILED")
		}
		summary.Recovered = result.Recovered
		summary.BackupDirectory = result.BackupDirectory
	case "rehearse", "apply":
		if review == "" || approval == "" {
			fmt.Fprintln(stderr, "--review and --approval are required; approval is never inferred")
			return 2
		}
		var capture stellarmigration.RootCapture
		var candidate stellarmigration.Candidate
		var approved migrationCLIApproval
		if readMigrationPrivateJSON(filepath.Join(review, "root-capture.private.json"), &capture) != nil || readMigrationPrivateJSON(filepath.Join(review, "candidate.private.json"), &candidate) != nil || readMigrationPrivateJSON(approval, &approved) != nil {
			return migrationCLIError(stderr, "ARTIFACT_INVALID")
		}
		// CLI当前配置不能替换artifact绑定的目标root或外置资源。
		if !sameMigrationLayout(capture.Layout, layout) {
			fmt.Fprintln(stderr, "reviewed runtime layout differs from the selected root")
			return 1
		}
		summary.ReviewDigest = approved.Approval.ReviewDigest
		if op == "rehearse" {
			result, e := stellarmigration.BuildRootRehearsal(ctx, capture, candidate, approved.Approval, approved.Notebook)
			if e != nil {
				return migrationCLIError(stderr, "REHEARSAL_FAILED")
			}
			if e = validateMigrationSubprocess(ctx, result); e != nil {
				_ = json.NewEncoder(stdout).Encode(migrationCLISummary{Operation: op, Directory: result.Directory, BinarySHA256: binaryHash})
				return migrationCLIError(stderr, "RUNTIME_VALIDATION_FAILED")
			}
			summary.Directory = result.Directory
			summary.RootDigest = result.RootDigest
			summary.CandidateDigest = result.ResolvedCandidateDigest
		} else {
			offline := approved.Offline
			if !offline.Approved || offline.RootDigest != capture.Digest || !offline.StoppedServerAndWorkers || !offline.LegacyInstancesChecked {
				fmt.Fprintln(stderr, "apply requires an explicit digest-bound offline attestation after stopping server/workers and checking legacy instances")
				return 1
			}
			if err = app.CheckStellarOfflineListen(a.ListenAddr); err != nil {
				fmt.Fprintln(stderr, "configured server port is occupied or cannot prove offline status")
				return 1
			}
			lastStage := ""
			runtime := &app.OfflineStellarMigrationRuntime{Guard: g, Validate: func(ctx context.Context, staged stellarmigration.RootRehearsal) error {
				lastStage = staged.Directory
				return validateMigrationSubprocess(ctx, staged)
			}}
			result, e := stellarmigration.ApplyReviewedRootMigration(ctx, capture, candidate, approved.Approval, approved.Notebook, runtime)
			if e != nil {
				if lastStage != "" {
					_ = json.NewEncoder(stdout).Encode(migrationCLISummary{Operation: op, Directory: lastStage, BinarySHA256: binaryHash})
				}
				return migrationCLIError(stderr, "APPLY_FAILED")
			}
			summary.RootDigest = result.RootDigest
			summary.CandidateDigest = result.ResolvedCandidateDigest
			summary.BackupDirectory = result.BackupDirectory
		}
	}
	if json.NewEncoder(stdout).Encode(summary) != nil {
		return 1
	}
	return 0
}

func previewMigrationSelections(ctx context.Context, layout stellarmigration.RootLayout, review, approval, requested string) (migrationCLISummary, error) {
	var capture stellarmigration.RootCapture
	var candidate stellarmigration.Candidate
	var pending migrationCLIApproval
	if err := readMigrationPrivateJSON(filepath.Join(review, "root-capture.private.json"), &capture); err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("ARTIFACT_INVALID", err)
	}
	if err := readMigrationPrivateJSON(filepath.Join(review, "candidate.private.json"), &candidate); err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("ARTIFACT_INVALID", err)
	}
	if err := readMigrationPrivateJSON(approval, &pending); err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("ARTIFACT_INVALID", err)
	}
	if err := capture.Verify(); err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("REVIEW_CHANGED", err)
	}
	if !sameMigrationLayout(capture.Layout, layout) || pending.Approval.RootDigest != capture.Digest || pending.Approval.SourceDigest != capture.Source.Digest || pending.Approval.CandidateDigest != stellarmigration.CandidateDigest(candidate) {
		return migrationCLISummary{}, migrationApprovalFailure("REVIEW_CHANGED", nil)
	}
	inventory, err := migrationSourceInventory(layout.Root, capture.TrackedSources)
	if err != nil || !reflect.DeepEqual(inventory, capture.TrackedSources) {
		return migrationCLISummary{}, migrationApprovalFailure("SOURCE_CHANGED", err)
	}
	fresh, err := stellarmigration.CaptureMigrationRoot(ctx, capture.Layout, capture.TrackedSources)
	if err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("CAPTURE_FAILED", err)
	}
	if fresh.Digest != capture.Digest {
		return migrationCLISummary{}, migrationApprovalFailure("SOURCE_CHANGED", nil)
	}
	selected, err := stellarmigration.PreviewReviewedSelections(candidate, pending.Approval.Resolutions, pending.Notebook)
	if err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("SELECTIONS_BLOCKED", err)
	}
	hash, err := stellarmigration.RootReviewDigest(capture, candidate, pending.Approval.Resolutions, pending.Notebook)
	if err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("SELECTIONS_BLOCKED", err)
	}
	pending.Approval.Approved = false
	pending.Approval.BodyDigests = nil
	pending.Approval.ReviewDigest = hash
	pending.Offline = migrationOfflineAttestation{RootDigest: capture.Digest}
	dir, err := createMigrationPreviewDirectory(requested, capture.Layout)
	if err != nil {
		return migrationCLISummary{}, migrationApprovalFailure("OUTPUT_INVALID", err)
	}
	for _, artifact := range []struct {
		name  string
		value any
	}{
		{"root-capture.private.json", capture}, {"candidate.private.json", candidate}, {"selected-candidate.private.json", selected.Candidate}, {"reviewed-selections.private.json", selected}, {"original-report.public.json", candidate.PublicReport()}, {"report.public.json", selected.Candidate.PublicReport()}, {"body-review.public.json", selected.BodyDigests}, {"approval-template.private.json", pending},
	} {
		if err = writeMigrationPrivateJSON(filepath.Join(dir, artifact.name), artifact.value); err != nil {
			return migrationCLISummary{}, migrationApprovalFailure("OUTPUT_INVALID", err)
		}
	}
	return migrationCLISummary{OK: true, Operation: "preview", Directory: dir, RootDigest: capture.Digest, SourceDigest: capture.Source.Digest, CandidateDigest: stellarmigration.CandidateDigest(candidate), SelectedCandidateDigest: stellarmigration.CandidateDigest(selected.Candidate), ReviewDigest: hash, Blocking: selected.Candidate.PublicReport().Blocking, NextSteps: []string{"Review selected-candidate.private.json and body-review.public.json; then run stellar migrate approve --review <this-directory> --out <new-approval-directory> [--accept-body-changes]."}}, nil
}

func sameMigrationLayout(captured, selected stellarmigration.RootLayout) bool {
	return sameMigrationPath(captured.Root, selected.Root) && sameMigrationPath(captured.DBPath, selected.DBPath) && sameMigrationPath(captured.UploadDir, selected.UploadDir) && sameMigrationPath(captured.AppPath, selected.AppPath) && sameMigrationPath(captured.OperationLock, selected.OperationLock)
}

func migrationSourceInventory(root string, extra []string) ([]string, error) {
	sources := map[string]bool{}
	for _, source := range extra {
		if !filepath.IsLocal(source) || !strings.HasPrefix(filepath.ToSlash(source), "content/") {
			return nil, fmt.Errorf("tracked source must be within content")
		}
		sources[filepath.ToSlash(filepath.Clean(source))] = true
	}
	contentRoot := filepath.Join(root, "content")
	for _, dir := range []string{contentRoot, filepath.Join(contentRoot, "posts")} {
		if info, err := os.Lstat(dir); err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("content source directory must be ordinary")
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	manifestPath := filepath.Join(contentRoot, backup.StellarContentManifestName)
	if _, err := os.Lstat(manifestPath); err == nil {
		manifest, e := backup.ReadStellarContentDirectory(contentRoot)
		if e != nil {
			return nil, e
		}
		for _, record := range manifest.Records {
			if record.Source != "" {
				sources[filepath.ToSlash(filepath.Join("content", record.Source))] = true
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(contentRoot, "posts"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		info, e := entry.Info()
		if e != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("tracked Markdown source must be an ordinary file")
		}
		sources["content/posts/"+entry.Name()] = true
	}
	list := make([]string, 0, len(sources))
	for source := range sources {
		list = append(list, source)
	}
	sort.Strings(list)
	return list, nil
}

func isPrivateMigrationRehearsal(root string, paths ...string) bool {
	parent := filepath.Dir(root)
	if !strings.HasPrefix(filepath.Base(parent), "stellar-root-rehearsal-") {
		return false
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return false
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return false
	}
	for _, path := range append([]string{root}, paths...) {
		resolved, e := filepath.EvalSymlinks(path)
		if os.IsNotExist(e) {
			if dir, pe := filepath.EvalSymlinks(filepath.Dir(path)); pe == nil {
				resolved = filepath.Join(dir, filepath.Base(path))
				e = nil
			}
		}
		if e != nil {
			return false
		}
		rel, e := filepath.Rel(parent, resolved)
		if e != nil || !filepath.IsLocal(rel) {
			return false
		}
	}
	return true
}

func sameMigrationPath(captured, selected string) bool {
	if captured == selected {
		return true
	}
	// 仅规范化系统祖先（如macOS /var），最终资源anchor不接受符号链接。
	if info, err := os.Lstat(selected); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if path, err := filepath.EvalSymlinks(selected); err == nil {
		return captured == path
	}
	if parent, err := filepath.EvalSymlinks(filepath.Dir(selected)); err == nil {
		return captured == filepath.Join(parent, filepath.Base(selected))
	}
	return false
}

func migrationCLIError(stderr io.Writer, stage ...string) int {
	// 底层YAML/SQL错误可能含配置或正文值，不转发到公开终端。
	code := "MIGRATION_REJECTED"
	if len(stage) > 0 {
		code = stage[0]
	}
	fmt.Fprintf(stderr, "migration rejected [%s]: %s\n", code, migrationErrorHint(code))
	return 1
}

func readMigrationPrivateJSON(path string, target any) error {
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 || parent.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("private artifact directory required")
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("private regular artifact required")
	}
	decoder := json.NewDecoder(f)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("artifact contains trailing data")
	}
	return nil
}
func writeMigrationPrivateJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}
func createMigrationPreviewDirectory(requested string, layout stellarmigration.RootLayout) (string, error) {
	useTemporaryDirectory := requested == ""
	if useTemporaryDirectory {
		// 先检查默认临时父目录，不能在捕获的runtime内创建artifact后才发现越界。
		requested = filepath.Join(os.TempDir(), "stellar-migration-preview-pending")
	}
	path, err := filepath.Abs(requested)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	path = filepath.Join(parent, filepath.Base(path))
	for _, anchor := range []string{layout.Root, filepath.Dir(layout.DBPath), layout.UploadDir, filepath.Dir(layout.AppPath)} {
		resolved, e := filepath.EvalSymlinks(anchor)
		if e == nil {
			anchor = resolved
		}
		rel, e := filepath.Rel(anchor, path)
		if e == nil && (rel == "." || filepath.IsLocal(rel)) {
			return "", fmt.Errorf("preview artifacts must be outside captured runtime resources")
		}
	}
	if useTemporaryDirectory {
		return os.MkdirTemp(parent, "stellar-migration-preview-")
	}
	if err = os.Mkdir(path, 0700); err != nil {
		return "", err
	}
	return path, nil
}

// 真实子进程执行正式Bootstrap导入及search/SSR检查，清除环境中的外部root/DB/seed
// 覆盖；所有连接随子进程结束关闭，不在当前离线进程打开源Store。
func validateMigrationSubprocess(ctx context.Context, staged stellarmigration.RootRehearsal) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	expectedHash, err := migrationBinaryHash()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "-root", staged.Layout.Root, "-config", staged.Layout.AppPath, "stellar", "migrate", "validate-staged")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "BLOG_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "BLOG_STELLAR_MIGRATION_VALIDATION=1", "BLOG_AUTO_BACKUP=false", "BLOG_SEED=false")
	var output bytes.Buffer
	cmd.Stdout = &output
	if err = cmd.Run(); err != nil {
		return err
	}
	var proof migrationCLISummary
	if err = json.Unmarshal(output.Bytes(), &proof); err != nil || !proof.OK || proof.Operation != "validate-staged" || proof.BinarySHA256 != expectedHash {
		return fmt.Errorf("staged validator binary identity changed")
	}
	return nil
}

func migrationBinaryHash() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func validateMigrationRuntime(ctx context.Context, resolved *config.App, appPath string) (resultErr error) {
	if !isPrivateMigrationRehearsal(resolved.Root, resolved.DBPath, resolved.UploadDir, appPath) {
		return fmt.Errorf("validation requires an isolated rehearsal root")
	}
	services, err := app.Bootstrap(app.Options{Root: resolved.Root, ConfigPath: appPath, ResolvedApp: resolved})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, services.Close(context.WithoutCancel(ctx))) }()
	if !sameMigrationLayout(app.StellarMigrationLayout(resolved, appPath), app.StellarMigrationLayout(services.App, appPath)) {
		return fmt.Errorf("staged bootstrap escaped validated runtime paths")
	}
	if services.Store == nil || services.Maintenance.Active() {
		return fmt.Errorf("staged bootstrap unavailable")
	}
	legacy, err := services.Store.DetectLegacyNotebookSchema()
	if err != nil {
		return err
	}
	if legacy.Required() {
		return legacy.Error()
	}
	if services.Search == nil {
		return fmt.Errorf("staged search unavailable")
	}
	assetVersioner, err := newIconAssetVersioner(false, services.App.Root, services.Icons)
	if err != nil {
		return err
	}
	web.SetAssetVersioner(assetVersioner)
	engine, err := web.NewEngineFromEmbed(services.Icons)
	if err != nil {
		return err
	}
	engine.SetStellarMarkdownRender(services.Post.RenderStellarV2WithContext)
	deps := routes.Dependencies{App: services.App, Site: services.Site, Store: services.Store, Posts: services.Post, Engine: engine, Icons: services.Icons, Root: services.App.Root, UploadDir: services.App.UploadDir, NotesPath: filepath.Join(services.App.Root, "content", "notes.md")}
	items, err := routes.RenderToolSearchItems(ctx, deps)
	if err != nil {
		return err
	}
	if err = services.Search.Rebuild(items); err != nil {
		return err
	}
	_, err = routes.RenderToolSnapshots(ctx, deps)
	return err
}

func (s *Server) RenderSearchItems(ctx context.Context) ([]search.Item, error) {
	return s.Server.RenderSearchItems(ctx)
}
