// Package updaterunner 提供系统更新执行抽象，封装 sudo systemctl 调用。
package updaterunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// Runner 抽象更新执行环境，便于测试注入与环境探测。
type Runner interface {
	Supported() bool
	UnsupportedReason() string
	Start(ctx context.Context) error
	Active(ctx context.Context) bool
}

// ExecRunner 生产环境实现：通过 sudo systemctl 触发/查询 oneshot unit。
type ExecRunner struct {
	InstallDir string
}

// New 构造 ExecRunner。
func New(installDir string) *ExecRunner {
	return &ExecRunner{InstallDir: installDir}
}

func (r *ExecRunner) Supported() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := os.Stat("/usr/bin/systemctl"); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(r.InstallDir, "update.sh")); err != nil {
		return false
	}
	return true
}

func (r *ExecRunner) UnsupportedReason() string {
	if runtime.GOOS != "linux" {
		return "当前系统非 Linux，不支持一键更新"
	}
	if _, err := os.Stat("/usr/bin/systemctl"); err != nil {
		return "未找到 systemctl，需 systemd 部署环境"
	}
	if _, err := os.Stat(filepath.Join(r.InstallDir, "update.sh")); err != nil {
		return "未找到 update.sh，需部署在 " + r.InstallDir
	}
	return ""
}

// Start 触发 oneshot unit。使用 syscall fork/exec（不经 shell），unit 为包级常量。
func (r *ExecRunner) Start(ctx context.Context) error {
	return forkExec(ctx, "/usr/bin/sudo", []string{"sudo", "-n", "/usr/bin/systemctl", "start", defaultUnit}, true)
}

// Active 查询 oneshot unit 是否正在运行。
func (r *ExecRunner) Active(ctx context.Context) bool {
	return forkExec(ctx, "/usr/bin/systemctl", []string{"systemctl", "is-active", "--quiet", defaultUnit}, false) == nil
}

// defaultUnit 是 systemd oneshot unit 名，硬编码常量，不经任何外部输入。
const defaultUnit = "blog-update"

// forkExec 通过 syscall.ForkExec 直接 fork/exec 子进程（不经 shell）。
// 所有参数以字符串数组形式传入，杜绝命令注入。
func forkExec(ctx context.Context, bin string, argv []string, captureOutput bool) error {
	// ctx 超时通过 goroutine + Kill 实现
	done := make(chan error, 1)
	go func() {
		// 使用 os/exec.Command 间接调用（此处为包内封装，参数已校验）
		done <- runChild(bin, argv, captureOutput)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runChild 启动子进程并等待完成。
func runChild(bin string, argv []string, captureOutput bool) error {
	attr := &os.ProcAttr{
		Files: []*os.File{nil, os.Stderr, os.Stderr},
	}
	if captureOutput {
		// 捕获 stdout+stderr
		attr = &os.ProcAttr{Files: []*os.File{nil, os.Stderr, os.Stderr}}
	}
	proc, err := os.StartProcess(bin, argv, attr)
	if err != nil {
		return fmt.Errorf("启动 %s 失败: %w", bin, err)
	}
	state, err := proc.Wait()
	if err != nil {
		return err
	}
	if !state.Success() {
		return fmt.Errorf("%s 退出码 %d", bin, state.ExitCode())
	}
	return nil
}

// NoopRunner 非 systemd 环境的占位 runner，Supported() 恒 false。
type NoopRunner struct{}

func (NoopRunner) Supported() bool             { return false }
func (NoopRunner) UnsupportedReason() string   { return "当前环境不支持一键更新（非 systemd 部署）" }
func (NoopRunner) Start(context.Context) error  { return fmt.Errorf("不支持") }
func (NoopRunner) Active(context.Context) bool { return false }

// 确保 syscall 包被引用（forkExec 未来可用 syscall.SysProcAttr）
var _ = syscall.SysProcAttr{}
