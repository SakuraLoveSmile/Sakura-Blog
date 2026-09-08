package updaterunner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
