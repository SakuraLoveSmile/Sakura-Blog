// Package updaterunner 提供系统更新执行抽象，通过 systemd.path 请求机制实现无 sudo 安全更新。
package updaterunner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// defaultUnit 是 systemd oneshot unit 名，硬编码常量。
const defaultUnit = "blog-update"

// 更新执行驱动：DriverAuto 先探测 docker 执行器标记再回退 systemd 契约；
// DriverSystemd / DriverDocker 为显式指定。
const (
	DriverAuto    = "auto"
	DriverSystemd = "systemd"
	DriverDocker  = "docker"
)

// dockerScriptName 是 docker 部署的执行器标记文件：宿主把 deploy/update-docker.sh
// 放入安装目录后，经 bind mount 在应用根目录（-root）下可见。
const dockerScriptName = "update-docker.sh"

// updateLogTailBytes 读取 update.log 尾部的近似字节数。
const updateLogTailBytes = 8 * 1024

// ErrBusy 表示已有更新任务正在队列或执行中。
var ErrBusy = errors.New("已有更新正在进行中")

// TaskInfo 描述已提交的更新任务。
type TaskInfo struct {
	TaskID        string `json:"task_id"`
	TargetVersion string `json:"target_version"`
	Stage         string `json:"stage"`
	StartedAt     string `json:"started_at"`
}

// StatusInfo 描述更新系统的当前运行或历史状态。
type StatusInfo struct {
	TaskID         string `json:"task_id,omitempty"`
	State          string `json:"state"` // idle | running | succeeded | failed
	Stage          string `json:"stage,omitempty"`
	CurrentVersion string `json:"current_version"`
	TargetVersion  string `json:"target_version,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	CompletedAt    string `json:"completed_at,omitempty"`
	Message        string `json:"message,omitempty"`
	Error          string `json:"error,omitempty"`
	LogTail        string `json:"log_tail,omitempty"`
	RolledBack     bool   `json:"rolled_back,omitempty"`
	RollbackOK     bool   `json:"rollback_succeeded,omitempty"`
}

// UpdateRequest 写入 requests/<task_id>.json 的结构。
type UpdateRequest struct {
	Version   int    `json:"version"`
	TaskID    string `json:"task_id"`
	TargetTag string `json:"target_tag"`
	CreatedAt string `json:"created_at"`
}

// Runner 抽象更新执行环境，便于测试注入与环境探测。
type Runner interface {
	Supported() bool
	UnsupportedReason() string
	Start(ctx context.Context, targetTag string) (*TaskInfo, error)
	Status(ctx context.Context, currentVersion string) (*StatusInfo, error)
	Active(ctx context.Context) bool
}

// ExecRunner 生产环境实现：基于专用请求目录与宿主侧执行器（systemd.path 或 docker watcher）。
type ExecRunner struct {
	InstallDir    string
	ControlDir    string
	UpdateService string
	// Driver 更新执行驱动：auto（默认）先探测 docker 执行器标记再走 systemd 契约；
	// systemd / docker 为显式指定。
	Driver string
	// Root 应用根目录（-root 解析结果）。docker 驱动下控制目录固定为 <Root>/update，
	// 经 bind mount 与宿主安装目录下的 update/ 是同一目录。
	Root string
}

// New 构造 ExecRunner。
func New(installDir string, updateService ...string) *ExecRunner {
	svc := defaultUnit
	if len(updateService) > 0 && strings.TrimSpace(updateService[0]) != "" {
		svc = strings.TrimSpace(updateService[0])
	}
	return &ExecRunner{
		InstallDir:    installDir,
		ControlDir:    filepath.Join(installDir, "update"),
		UpdateService: svc,
	}
}

// driver 解析生效驱动。auto 模式下宿主放入 update-docker.sh（容器内经 bind mount
// 在 Root 下可见）即视为 docker 部署，无需改配置。
func (r *ExecRunner) driver() string {
	switch strings.ToLower(strings.TrimSpace(r.Driver)) {
	case DriverSystemd:
		return DriverSystemd
	case DriverDocker:
		return DriverDocker
	}
	if r.dockerScriptVisible() {
		return DriverDocker
	}
	return DriverSystemd
}

// dockerScriptVisible docker 执行器标记：宿主安装目录里的 update-docker.sh
// 经 bind mount 后在应用根目录可见。
func (r *ExecRunner) dockerScriptVisible() bool {
	if r.Root == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(r.Root, dockerScriptName))
	return err == nil && !info.IsDir()
}

// controlDir 更新控制目录：docker 驱动固定为 <Root>/update（与宿主安装目录的
// update/ 是同一 bind mount 目录），systemd 驱动为 <InstallDir>/update。
func (r *ExecRunner) controlDir() string {
	if r.driver() == DriverDocker && r.Root != "" {
		return filepath.Join(r.Root, "update")
	}
	return r.ControlDir
}

func (r *ExecRunner) Supported() bool {
	return r.UnsupportedReason() == ""
}

func (r *ExecRunner) UnsupportedReason() string {
	if r.driver() == DriverDocker {
		if !r.dockerScriptVisible() {
			return "docker 驱动需要在宿主安装执行器：将 deploy/update-docker.sh 放入安装目录并配置 watcher（见 deploy/README.md）"
		}
		if runtime.GOOS != "linux" {
			return "当前系统非 Linux，不支持一键更新"
		}
		return ""
	}
	if r.UpdateService != defaultUnit {
		return fmt.Sprintf("不支持自定义 update_service（仅支持默认 %s）", defaultUnit)
	}
	if runtime.GOOS != "linux" {
		return "当前系统非 Linux，不支持一键更新"
	}
	if _, err := os.Stat("/usr/bin/systemctl"); err != nil {
		return "未找到 systemctl，需 systemd 部署环境（docker 部署可在 app.yml 设 update_driver: docker 并按 deploy/README.md 安装执行器）"
	}
	if _, err := os.Stat(filepath.Join(r.InstallDir, "update.sh")); err != nil {
		return "未找到 update.sh，需部署在 " + r.InstallDir
	}
	return ""
}

// Start 向 requests 目录写入小体积请求 JSON。
// 若已有处于 queued 或 running 的同目标任务，幂等返回该任务；若目标不同或正在运行，返回 ErrBusy。
func (r *ExecRunner) Start(ctx context.Context, targetTag string) (*TaskInfo, error) {
	if !r.Supported() {
		return nil, errors.New(r.UnsupportedReason())
	}

	targetTag = strings.TrimSpace(targetTag)
	if targetTag == "" {
		return nil, errors.New("目标版本号为空")
	}

	requestsDir := filepath.Join(r.controlDir(), "requests")
	if err := os.MkdirAll(requestsDir, 0o770); err != nil {
		return nil, fmt.Errorf("创建请求目录失败: %w", err)
	}

	// 检查现有 status.json 是否处于 running 状态
	statusPath := filepath.Join(r.controlDir(), "status.json")
	if b, err := os.ReadFile(statusPath); err == nil {
		var st StatusInfo
		if json.Unmarshal(b, &st) == nil {
			if st.State == "running" {
				if st.TargetVersion == targetTag && st.TaskID != "" {
					return &TaskInfo{
						TaskID:        st.TaskID,
						TargetVersion: st.TargetVersion,
						Stage:         st.Stage,
						StartedAt:     st.StartedAt,
					}, nil
				}
				return nil, ErrBusy
			}
		}
	}

	// 检查 requests 目录是否有待消费的请求
	entries, err := os.ReadDir(requestsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			reqBytes, err := os.ReadFile(filepath.Join(requestsDir, entry.Name()))
			if err != nil {
				continue
			}
			var existingReq UpdateRequest
			if json.Unmarshal(reqBytes, &existingReq) == nil && existingReq.TaskID != "" {
				if existingReq.TargetTag == targetTag {
					return &TaskInfo{
						TaskID:        existingReq.TaskID,
						TargetVersion: existingReq.TargetTag,
						Stage:         "queued",
						StartedAt:     existingReq.CreatedAt,
					}, nil
				}
				return nil, ErrBusy
			}
		}
	}

	taskID := generateTaskID()
	now := time.Now().UTC().Format(time.RFC3339)
	req := UpdateRequest{
		Version:   1,
		TaskID:    taskID,
		TargetTag: targetTag,
		CreatedAt: now,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	tmpFile := filepath.Join(requestsDir, fmt.Sprintf(".tmp_%s.json", taskID))
	finalFile := filepath.Join(requestsDir, fmt.Sprintf("%s.json", taskID))

	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o660)
	if err != nil {
		return nil, fmt.Errorf("创建请求临时文件失败: %w", err)
	}
	if _, err := f.Write(reqBytes); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return nil, fmt.Errorf("写入请求文件失败: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return nil, fmt.Errorf("同步请求文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return nil, fmt.Errorf("关闭请求文件失败: %w", err)
	}

	if err := os.Rename(tmpFile, finalFile); err != nil {
		_ = os.Remove(tmpFile)
		return nil, fmt.Errorf("提交请求文件失败: %w", err)
	}

	return &TaskInfo{
		TaskID:        taskID,
		TargetVersion: targetTag,
		Stage:         "queued",
		StartedAt:     now,
	}, nil
}

// Status 读取当前更新状态；成功状态持久化保留，不随单次查询删除。
func (r *ExecRunner) Status(ctx context.Context, currentVersion string) (*StatusInfo, error) {
	statusPath := filepath.Join(r.controlDir(), "status.json")
	var status StatusInfo

	if b, err := os.ReadFile(statusPath); err == nil {
		if json.Unmarshal(b, &status) == nil && status.State != "" {
			status.CurrentVersion = currentVersion
			status.LogTail = r.readLogTail()
			return &status, nil
		}
	}

	// 检查是否有排队中的请求
	requestsDir := filepath.Join(r.controlDir(), "requests")
	if entries, err := os.ReadDir(requestsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			reqBytes, err := os.ReadFile(filepath.Join(requestsDir, entry.Name()))
			if err != nil {
				continue
			}
			var pending UpdateRequest
			if json.Unmarshal(reqBytes, &pending) == nil && pending.TaskID != "" {
				return &StatusInfo{
					TaskID:         pending.TaskID,
					State:          "running",
					Stage:          "queued",
					CurrentVersion: currentVersion,
					TargetVersion:  pending.TargetTag,
					StartedAt:      pending.CreatedAt,
					Message:        "更新请求已提交，等待后台服务处理",
					LogTail:        r.readLogTail(),
				}, nil
			}
		}
	}

	return &StatusInfo{
		State:          "idle",
		CurrentVersion: currentVersion,
		LogTail:        r.readLogTail(),
	}, nil
}

// Active 报告是否有更新正在执行。systemd 驱动调用 systemctl is-active 查询
// oneshot unit；docker 驱动容器内无 systemctl，改判控制目录文件态
// （status.json=running 或存在待消费请求）。
func (r *ExecRunner) Active(ctx context.Context) bool {
	if r.driver() == DriverDocker {
		st, err := r.Status(ctx, "")
		return err == nil && st.State == "running"
	}
	if runtime.GOOS != "linux" {
		return false
	}
	return runChildWithoutOutput("/usr/bin/systemctl", []string{"systemctl", "is-active", "--quiet", defaultUnit}) == nil
}

func (r *ExecRunner) readLogTail() string {
	paths := []string{
		filepath.Join(r.controlDir(), "update.log"),
		filepath.Join(r.InstallDir, "data", "update.log"),
	}
	if r.driver() == DriverDocker && r.Root != "" {
		paths[1] = filepath.Join(r.Root, "data", "update.log")
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err == nil && len(b) > 0 {
			if len(b) <= updateLogTailBytes {
				return string(b)
			}
			return string(b[len(b)-updateLogTailBytes:])
		}
	}
	return ""
}

func generateTaskID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("upd_%d_%08x", time.Now().Unix(), time.Now().UnixNano()%0xFFFFFFFF)
	}
	return fmt.Sprintf("upd_%d_%s", time.Now().Unix(), hex.EncodeToString(b))
}

func runChildWithoutOutput(bin string, argv []string) error {
	proc, err := os.StartProcess(bin, argv, &os.ProcAttr{
		Files: []*os.File{nil, nil, nil},
	})
	if err != nil {
		return err
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

// NoopRunner 非 systemd 环境的占位 runner。
type NoopRunner struct{}

func (NoopRunner) Supported() bool { return false }
func (NoopRunner) UnsupportedReason() string {
	return "当前环境不支持一键更新（非 systemd 部署）"
}
func (NoopRunner) Start(context.Context, string) (*TaskInfo, error) {
	return nil, errors.New("不支持")
}
func (NoopRunner) Status(_ context.Context, currentVersion string) (*StatusInfo, error) {
	return &StatusInfo{State: "idle", CurrentVersion: currentVersion}, nil
}
func (NoopRunner) Active(context.Context) bool { return false }
