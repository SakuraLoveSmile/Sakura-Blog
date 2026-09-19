package updaterunner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUnsupportedCustomService(t *testing.T) {
	r := New("/opt/blog", "custom-service")
	if r.Supported() {
		t.Fatal("expected custom update_service to be unsupported")
	}
	want := "不支持自定义 update_service（仅支持默认 blog-update）"
	if r.UnsupportedReason() != want {
		t.Fatalf("UnsupportedReason() = %q, want %q", r.UnsupportedReason(), want)
	}
}

func TestUnsupportedNonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("skipping non-linux test on linux")
	}
	r := New("/opt/blog")
	if r.Supported() {
		t.Fatal("expected Supported() = false on non-linux")
	}
	if r.UnsupportedReason() != "当前系统非 Linux，不支持一键更新" {
		t.Fatalf("unexpected reason: %s", r.UnsupportedReason())
	}
}

func TestStartAndStatusLifecycle(t *testing.T) {
	tmp := t.TempDir()
	r := &ExecRunner{
		InstallDir:    tmp,
		ControlDir:    filepath.Join(tmp, "update"),
		UpdateService: "blog-update",
	}

	// 模拟已存在 update.sh 以通过环境检查
	_ = os.WriteFile(filepath.Join(tmp, "update.sh"), []byte("#!/bin/sh\n"), 0o755)

	// 在非 linux 环境绕过 Supported 检查直接测文件读写逻辑
	ctx := context.Background()

	// 1. 初始状态：idle
	st, err := r.Status(ctx, "v1.4.1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.State != "idle" {
		t.Fatalf("expected state idle, got %s", st.State)
	}

	// 2. 模拟直接写请求
	reqDir := filepath.Join(r.ControlDir, "requests")
	if err := os.MkdirAll(reqDir, 0o770); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 手动触发一次 Start 内部的请求写入（在 test 忽略 Supported 限制直接测 Start 内部逻辑）
	taskID := generateTaskID()
	req := UpdateRequest{
		Version:   1,
		TaskID:    taskID,
		TargetTag: "v1.4.2",
		CreatedAt: "2026-09-08T12:00:00Z",
	}
	b, _ := json.Marshal(req)
	reqFile := filepath.Join(reqDir, taskID+".json")
	if err := os.WriteFile(reqFile, b, 0o660); err != nil {
		t.Fatalf("write req: %v", err)
	}

	// 3. 有待消费请求时：status 返回 running/queued
	st, err = r.Status(ctx, "v1.4.1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.State != "running" || st.Stage != "queued" {
		t.Fatalf("expected running/queued, got %s/%s", st.State, st.Stage)
	}
	if st.TargetVersion != "v1.4.2" {
		t.Fatalf("expected target v1.4.2, got %s", st.TargetVersion)
	}
	if st.TaskID != taskID {
		t.Fatalf("expected taskID %s, got %s", taskID, st.TaskID)
	}

	// 4. 模拟 update.sh 写入 status.json
	statusPath := filepath.Join(r.ControlDir, "status.json")
	persistedStatus := StatusInfo{
		TaskID:         taskID,
		State:          "succeeded",
		Stage:          "succeeded",
		CurrentVersion: "v1.4.2",
		TargetVersion:  "v1.4.2",
		StartedAt:      "2026-09-08T12:00:00Z",
		CompletedAt:    "2026-09-08T12:00:10Z",
		Message:        "更新成功",
	}
	sb, _ := json.Marshal(persistedStatus)
	if err := os.WriteFile(statusPath, sb, 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}

	// 写入日志
	logPath := filepath.Join(r.ControlDir, "update.log")
	_ = os.WriteFile(logPath, []byte("Update finished successfully\n"), 0o644)

	// 5. 查询 status：应读取成功状态与日志
	st, err = r.Status(ctx, "v1.4.2")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.State != "succeeded" || st.Stage != "succeeded" {
		t.Fatalf("expected succeeded, got %s/%s", st.State, st.Stage)
	}
	if st.LogTail != "Update finished successfully\n" {
		t.Fatalf("unexpected log tail: %q", st.LogTail)
	}

	// 6. 验证查询不会删除 status.json
	if _, err := os.Stat(statusPath); err != nil {
		t.Fatalf("status.json was deleted by status query: %v", err)
	}
}

// docker 驱动：Root 下出现 update-docker.sh 即 auto 判定为 docker 部署。
func TestDockerDriverDetection(t *testing.T) {
	tmp := t.TempDir()
	newRunner := func() *ExecRunner {
		return &ExecRunner{
			InstallDir:    filepath.Join(tmp, "install"),
			ControlDir:    filepath.Join(tmp, "install", "update"),
			UpdateService: defaultUnit,
			Root:          tmp,
		}
	}

	// auto 无标记 → systemd 契约
	if r := newRunner(); r.driver() != DriverSystemd {
		t.Fatalf("auto without marker: driver() = %s, want systemd", r.driver())
	}

	// auto 有标记 → docker
	r := newRunner()
	if err := os.WriteFile(filepath.Join(tmp, dockerScriptName), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if r.driver() != DriverDocker {
		t.Fatalf("auto with marker: driver() = %s, want docker", r.driver())
	}

	// 显式 systemd 覆盖标记
	r.Driver = DriverSystemd
	if r.driver() != DriverSystemd {
		t.Fatalf("explicit systemd: driver() = %s", r.driver())
	}
}

// docker 驱动缺执行器标记时给出可操作的提示（与平台无关）。
func TestDockerDriverMissingExecutor(t *testing.T) {
	r := &ExecRunner{
		InstallDir:    "/opt/blog",
		ControlDir:    "/opt/blog/update",
		UpdateService: defaultUnit,
		Driver:        DriverDocker,
		Root:          t.TempDir(),
	}
	if r.Supported() {
		t.Fatal("expected Supported() = false without update-docker.sh marker")
	}
	if reason := r.UnsupportedReason(); !strings.Contains(reason, dockerScriptName) {
		t.Fatalf("UnsupportedReason() = %q, want mention %s", reason, dockerScriptName)
	}
}

// docker 驱动下控制目录固定为 <Root>/update，与 InstallDir 无关。
func TestDockerControlDirUnderRoot(t *testing.T) {
	root := t.TempDir()
	install := t.TempDir()
	r := &ExecRunner{
		InstallDir:    install,
		ControlDir:    filepath.Join(install, "update"),
		UpdateService: defaultUnit,
		Driver:        DriverDocker,
		Root:          root,
	}
	if got := r.controlDir(); got != filepath.Join(root, "update") {
		t.Fatalf("controlDir() = %s, want %s", got, filepath.Join(root, "update"))
	}

	// 请求文件落在 <Root>/update/requests 才被消费（bind mount 到宿主）。
	reqDir := filepath.Join(root, "update", "requests")
	if err := os.MkdirAll(reqDir, 0o770); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	req := UpdateRequest{Version: 1, TaskID: "task-docker-1", TargetTag: "v9.9.9", CreatedAt: "2026-09-20T00:00:00Z"}
	b, _ := json.Marshal(req)
	if err := os.WriteFile(filepath.Join(reqDir, "task-docker-1.json"), b, 0o660); err != nil {
		t.Fatalf("write req: %v", err)
	}
	st, err := r.Status(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.State != "running" || st.TargetVersion != "v9.9.9" {
		t.Fatalf("docker driver did not read request under Root, got %+v", st)
	}
	if !r.Active(context.Background()) {
		t.Fatal("docker Active() should be true while a request is pending")
	}
}

// docker 驱动的 Active 依据 status.json 文件态，而非 systemctl。
func TestDockerActiveFromStatusFile(t *testing.T) {
	root := t.TempDir()
	r := &ExecRunner{
		InstallDir:    t.TempDir(),
		UpdateService: defaultUnit,
		Driver:        DriverDocker,
		Root:          root,
	}
	r.ControlDir = filepath.Join(r.InstallDir, "update")

	if r.Active(context.Background()) {
		t.Fatal("Active() should be false with no status file")
	}

	controlDir := filepath.Join(root, "update")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write := func(state string) {
		b, _ := json.Marshal(StatusInfo{State: state})
		if err := os.WriteFile(filepath.Join(controlDir, "status.json"), b, 0o644); err != nil {
			t.Fatalf("write status: %v", err)
		}
	}
	write("running")
	if !r.Active(context.Background()) {
		t.Fatal("Active() should be true while status.json is running")
	}
	write("succeeded")
	if r.Active(context.Background()) {
		t.Fatal("Active() should be false after succeeded")
	}
}
