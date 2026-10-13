package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"blog/internal/config"
	"blog/internal/stellarmigration"
)

func StellarMigrationLayout(a *config.App, appPath string) stellarmigration.RootLayout {
	return stellarmigration.RootLayout{Root: a.Root, DBPath: a.DBPath, UploadDir: a.UploadDir, AppPath: appPath, OperationLock: filepath.Join(filepath.Dir(a.DBPath), "maintenance.lock")}
}

// CheckStellarMigrationToolReady只在已持共享guard后调用，不为只读命令隐式恢复。
// 未完成journal必须由明确的recover命令或正式服务启动先处理，再打开任何Store。
func CheckStellarMigrationToolReady(a *config.App) error {
	_, err := os.Lstat(filepath.Join(filepath.Dir(a.DBPath), ".stellar-migration-journal.json"))
	if os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("migration recovery is pending or unavailable; run stellar migrate recover before ordinary commands")
}

// PrepareStellarMigrationStartup在Store.Open之前恢复。整个恢复窗口先持排它锁，
// 后转共享锁直到HTTP排空、模块worker及Store全部关闭，期间无无锁窗口。
func PrepareStellarMigrationStartup(ctx context.Context, options Options) (*StellarRootGuard, error) {
	a, path, err := StellarMigrationConfig(options)
	if err != nil {
		return nil, err
	}
	g, err := AcquireStellarRootGuard(a, path, true)
	if err != nil {
		return nil, err
	}
	if _, err = stellarmigration.RecoverRootMigration(ctx, StellarMigrationLayout(g.AppSnapshot, g.AppPath)); err != nil {
		_ = g.Close()
		return nil, fmt.Errorf("stellar migration recovery incomplete: %w", err)
	}
	if err = g.Downgrade(); err != nil {
		_ = g.Close()
		return nil, err
	}
	return g, nil
}

// OfflineStellarMigrationRuntime仅用于已停服的CLI：没有HTTP、Store或后台worker。
// 它不声称可以停掉旧二进制；调用方须取得排它guard、核对停机声明及配置端口。
type OfflineStellarMigrationRuntime struct {
	Guard         *StellarRootGuard
	Validate      func(context.Context, stellarmigration.RootRehearsal) error
	RecoveryError error
}

func (r *OfflineStellarMigrationRuntime) Quiesce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || !r.Guard.Exclusive() {
		return fmt.Errorf("offline migration requires the exclusive root lifetime guard")
	}
	return nil
}
func (r *OfflineStellarMigrationRuntime) ValidateStaged(ctx context.Context, staged stellarmigration.RootRehearsal) error {
	if err := r.Quiesce(ctx); err != nil {
		return err
	}
	if r.Validate == nil {
		return fmt.Errorf("staged runtime validation is required")
	}
	return r.Validate(ctx, staged)
}
func (r *OfflineStellarMigrationRuntime) StartHeld(ctx context.Context) error { return r.Quiesce(ctx) }
func (r *OfflineStellarMigrationRuntime) Release(ctx context.Context) error   { return r.Quiesce(ctx) }
func (r *OfflineStellarMigrationRuntime) HoldRecovery(err error)              { r.RecoveryError = err }
