package app

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"blog/internal/config"
)

func TestStellarRootLifetimeGuardPreventsApplyAcrossStartupAndShutdown(t *testing.T) {
	root := t.TempDir()
	a := &config.App{Root: root, DBPath: filepath.Join(root, "data", "blog.db"), UploadDir: filepath.Join(root, "uploads")}
	path := filepath.Join(root, "config", "app.yml")
	server, err := AcquireStellarRootGuard(a, path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if apply, e := AcquireStellarRootGuard(a, path, true); e == nil {
		apply.Close()
		t.Fatal("apply crossed exclusive startup recovery")
	}
	if err = server.Downgrade(); err != nil {
		t.Fatal(err)
	}
	if apply, e := AcquireStellarRootGuard(a, path, true); e == nil {
		apply.Close()
		t.Fatal("apply crossed running server shared guard")
	}
	// 一个root的不同app配置仍因DB/content anchor相同互斥。
	if apply, e := AcquireStellarRootGuard(a, filepath.Join(root, "other-app.yml"), true); e == nil {
		apply.Close()
		t.Fatal("alternate config bypassed root guard")
	}
	otherRoot := t.TempDir()
	other := &config.App{Root: otherRoot, DBPath: a.DBPath, UploadDir: filepath.Join(otherRoot, "uploads")}
	if apply, e := AcquireStellarRootGuard(other, filepath.Join(otherRoot, "app.yml"), true); e == nil {
		apply.Close()
		t.Fatal("different root bypassed shared external DB guard")
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	apply, err := AcquireStellarRootGuard(a, path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer apply.Close()
	runtime := &OfflineStellarMigrationRuntime{Guard: apply}
	if err = runtime.Quiesce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if shared, e := AcquireStellarRootGuard(a, path, false); e == nil {
		shared.Close()
		t.Fatal("server crossed offline apply guard")
	}
}

func TestStellarStartupRecoveryRejectsJournalBeforeStoreOpen(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", ".stellar-migration-journal.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if guard, err := PrepareStellarMigrationStartup(context.Background(), Options{Root: root}); err == nil {
		guard.Close()
		t.Fatal("invalid journal reached ordinary bootstrap")
	}
	if _, err := os.Stat(filepath.Join(root, "data", "blog.db")); !os.IsNotExist(err) {
		t.Fatal("recovery opened or created the Store")
	}
}

func TestStellarStartupUsesExactGuardedConfigurationWhenAppFileChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config", "app.yml")
	if err := os.WriteFile(path, []byte("db_path: data/locked.db\nauto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "site.yml"), []byte("title: Guard fixture\nadmin:\n  password: synthetic-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	guard, err := PrepareStellarMigrationStartup(context.Background(), Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	if err = os.WriteFile(path, []byte("db_path: data/unlocked.db\nauto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	services, err := Bootstrap(Options{Root: root, ResolvedApp: guard.AppSnapshot})
	if err != nil {
		t.Fatal(err)
	}
	if err = services.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if services.App.DBPath != guard.AppSnapshot.DBPath {
		t.Fatal("bootstrap escaped the held DB anchor")
	}
	if _, err = os.Stat(filepath.Join(root, "data", "locked.db")); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "data", "unlocked.db")); !os.IsNotExist(err) {
		t.Fatal("bootstrap opened newly configured unlocked DB")
	}
}

func TestStellarOfflinePortRequiresStoppedConfiguredListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if CheckStellarOfflineListen(addr) == nil {
		t.Fatal("active listener accepted")
	}
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err = CheckStellarOfflineListen(addr); err != nil {
		t.Fatal(err)
	}
	if CheckStellarOfflineListen("127.0.0.1:0") == nil {
		t.Fatal("unknown ephemeral port accepted")
	}
}

func TestStellarOrdinaryToolRefusesPendingJournalWithoutRecovery(t *testing.T) {
	root := t.TempDir()
	a := &config.App{Root: root, DBPath: filepath.Join(root, "blog.db")}
	if err := CheckStellarMigrationToolReady(a); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(root, ".stellar-migration-journal.json")
	if err := os.WriteFile(journal, []byte("pending recovery"), 0600); err != nil {
		t.Fatal(err)
	}
	if CheckStellarMigrationToolReady(a) == nil {
		t.Fatal("ordinary tool accepted pending migration")
	}
	data, err := os.ReadFile(journal)
	if err != nil || string(data) != "pending recovery" {
		t.Fatal("ordinary tool changed the recovery journal")
	}
}

func TestStellarGuardSnapshotSurvivesAncestorSymlinkRetarget(t *testing.T) {
	parent := t.TempDir()
	first, second := filepath.Join(parent, "first"), filepath.Join(parent, "second")
	for _, directory := range []string{first, second} {
		if err := os.MkdirAll(filepath.Join(directory, "root", "config"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "root", "config", "site.yml"), []byte("title: Alias fixture\nadmin:\n  password: synthetic-password\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(alias, "root")
	a := &config.App{Root: root, DBPath: filepath.Join(root, "data", "blog.db"), UploadDir: filepath.Join(root, "data", "uploads"), AutoBackup: false}
	g, err := AcquireStellarRootGuard(a, filepath.Join(root, "config", "app.yml"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	canonicalFirst, err := filepath.EvalSymlinks(filepath.Join(first, "root"))
	if err != nil {
		t.Fatal(err)
	}
	if g.AppSnapshot.Root != canonicalFirst || g.AppPath != filepath.Join(canonicalFirst, "config", "app.yml") {
		t.Fatal("guard retained alias paths in runtime snapshot")
	}
	if err = os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(second, alias); err != nil {
		t.Fatal(err)
	}
	services, err := Bootstrap(Options{Root: root, ResolvedApp: g.AppSnapshot})
	if err != nil {
		t.Fatal(err)
	}
	if err = services.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(first, "root", "data", "blog.db")); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(second, "root", "data", "blog.db")); !os.IsNotExist(err) {
		t.Fatal("ancestor retarget opened an unguarded DB")
	}
}
