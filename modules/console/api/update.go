package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// updateRepoDefault / updateInstallDirDefault 为 api 层兜底常量，
// 与 config.DefaultApp 对齐；Config 缺省时由 New() 填入。
const (
	updateRepoDefault       = "SakuraLoveSmile/Sakura-Blog"
	updateInstallDirDefault = "/opt/blog"
)

// releaseNotesMaxBytes 截断 release body 的上限，防止超长正文撑爆响应。
const releaseNotesMaxBytes = 32 * 1024

// applyTimeout sudo systemctl start 的超时时间——oneshot unit 由 update.sh 控制实际耗时，
// start 命令本身只是触发（fire-and-forget），不应长时间阻塞。
const applyTimeout = 10 * time.Second

// updateStateFile 由本功能自管理，记录一键更新的 from/to/started_at，供重启后的新进程恢复上下文。
const updateStateFile = "data/update-state.json"

// updateLogFile update.sh 写入的日志路径（相对于 install_dir）。
const updateLogFile = "data/update.log"

// updateLogTailBytes 读取 update.log 尾部的近似字节数。
const updateLogTailBytes = 8 * 1024

// updateState 一键更新的持久化状态（写入 update-state.json）。
type updateState struct {
	From      string `json:"from"`
	To        string `json:"to"`
	StartedAt string `json:"started_at"`
}

// updateCheckResponse GET /api/v1/system/update/check
type updateCheckResponse struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url"`
	ReleaseNotes    string `json:"release_notes"`
	PublishedAt     string `json:"published_at"`
	ApplySupported  bool   `json:"apply_supported"`
	CheckedAt       string `json:"checked_at"`
	Error           string `json:"error,omitempty"`
}

// updateStatusResponse GET /api/v1/system/update/status
type updateStatusResponse struct {
	State          string `json:"state"` // idle|running|succeeded|failed
	CurrentVersion string `json:"current_version"`
	TargetVersion  string `json:"target_version"`
	StartedAt      string `json:"started_at"`
	LogTail        string `json:"log_tail"`
	Message        string `json:"message,omitempty"`
}

// systemUpdateCheck GET /api/v1/system/update/check?refresh=1
func (h *Handler) systemUpdateCheck(c *gin.Context) {
	current := h.resolveCurrentVersion()
	resp := updateCheckResponse{
		CurrentVersion: current,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		ApplySupported: h.upd.Supported(),
	}

	if !h.upd.Supported() {
		// 不支持一键更新时仍返回当前版本与原因，前端据此降级为"仅检查"
		resp.Error = h.upd.UnsupportedReason()
	}

	if h.gh == nil {
		if resp.Error == "" {
			resp.Error = "GitHub 客户端未启用"
		}
		c.JSON(200, resp)
		return
	}

	refresh := c.Query("refresh") == "1"
	var payload []byte
	var err error
	if refresh {
		payload, err = h.gh.RefreshLatestRelease(splitOwnerRepo(h.updateRepo))
	} else {
		payload, err = h.gh.GetLatestReleaseRaw(splitOwnerRepo(h.updateRepo))
	}
	if err != nil {
		if resp.Error == "" {
			resp.Error = fmt.Sprintf("检查更新失败: %s（私有仓库需配置 github_token 或 BLOG_GITHUB_TOKEN）", err.Error())
		}
		c.JSON(200, resp)
		return
	}

	var release struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		Body        string    `json:"body"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.Unmarshal(payload, &release); err != nil {
		if resp.Error == "" {
			resp.Error = "解析 GitHub release 失败: " + err.Error()
		}
		c.JSON(200, resp)
		return
	}

	latest := normalizeSemver(release.TagName)
	resp.LatestVersion = latest
	resp.ReleaseURL = release.HTMLURL
	resp.ReleaseNotes = truncateString(release.Body, releaseNotesMaxBytes)
	if !release.PublishedAt.IsZero() {
		resp.PublishedAt = release.PublishedAt.UTC().Format(time.RFC3339)
	}

	if latest != "" && current != "unknown" {
		resp.UpdateAvailable = semverCompare(latest, current) > 0
	}

	c.JSON(200, resp)
}

// systemUpdateApply POST /api/v1/system/update/apply
func (h *Handler) systemUpdateApply(c *gin.Context) {
	if !h.upd.Supported() {
		Fail(c, 400, h.upd.UnsupportedReason())
		return
	}

	// CAS 防并发：同一时刻只允许一个 apply
	if !h.updateApplying.CompareAndSwap(false, true) {
		Fail(c, 409, "已有更新正在进行中")
		return
	}
	defer h.updateApplying.Store(false)

	current := h.resolveCurrentVersion()

	// 先查 latest 版本以确定 target
	payload, err := h.gh.GetLatestReleaseRaw(splitOwnerRepo(h.updateRepo))
	if err != nil {
		Fail(c, 500, "获取最新版本失败: "+err.Error())
		return
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(payload, &release); err != nil {
		Fail(c, 500, "解析最新版本失败: "+err.Error())
		return
	}
	target := normalizeSemver(release.TagName)
	if target == "" {
		Fail(c, 500, "最新版本号无效")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), applyTimeout)
	defer cancel()
	if err := h.upd.Start(ctx); err != nil {
		Fail(c, 500, "触发更新服务失败: "+err.Error()+"（请检查 sudoers 配置）")
		return
	}

	// 写入状态文件供重启后恢复上下文
	state := updateState{
		From:      current,
		To:        target,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	h.writeUpdateState(state)

	c.JSON(202, gin.H{
		"started":        true,
		"target_version": target,
	})
}

// systemUpdateStatus GET /api/v1/system/update/status
func (h *Handler) systemUpdateStatus(c *gin.Context) {
	current := h.resolveCurrentVersion()
	resp := updateStatusResponse{
		CurrentVersion: current,
	}

	state, hasState := h.readUpdateState()
	if !hasState {
		resp.State = "idle"
		c.JSON(200, resp)
		return
	}

	resp.TargetVersion = state.To
	resp.StartedAt = state.StartedAt

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if h.upd.Active(ctx) {
		resp.State = "running"
		resp.Message = "更新正在进行中，服务即将重启"
		c.JSON(200, resp)
		return
	}

	// unit 已结束：对比当前版本与 target
	if current == state.To {
		resp.State = "succeeded"
		resp.Message = "更新成功"
		// 成功后清理状态文件
		h.clearUpdateState()
	} else {
		resp.State = "failed"
		resp.Message = "更新失败或已回滚，请查看日志"
	}

	resp.LogTail = h.readUpdateLogTail()
	c.JSON(200, resp)
}

// --- helpers ---

// resolveCurrentVersion 解析当前版本：app.Version ≠ "dev" → 用之；否则读 <Root>/version；都无 → unknown。
func (h *Handler) resolveCurrentVersion() string {
	v := normalizeSemver(h.appVersion)
	if v != "" {
		return v
	}
	if h.root != "" {
		if b, err := os.ReadFile(filepath.Join(h.root, "version")); err == nil {
			if v = normalizeSemver(strings.TrimSpace(string(b))); v != "" {
				return v
			}
		}
	}
	return "unknown"
}

// splitOwnerRepo 将 "owner/repo" 拆分为 (owner, repo)，缺斜杠时整体当 repo。
func splitOwnerRepo(repo string) (owner, repoName string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", repo
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// writeUpdateState 将更新状态写入 <Root>/data/update-state.json。
func (h *Handler) writeUpdateState(s updateState) {
	if h.root == "" {
		return
	}
	path := filepath.Join(h.root, updateStateFile)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	b, _ := json.Marshal(s)
	_ = os.WriteFile(path, b, 0o644)
}

// readUpdateState 读取更新状态文件，不存在返回 false。
func (h *Handler) readUpdateState() (updateState, bool) {
	if h.root == "" {
		return updateState{}, false
	}
	b, err := os.ReadFile(filepath.Join(h.root, updateStateFile))
	if err != nil {
		return updateState{}, false
	}
	var s updateState
	if err := json.Unmarshal(b, &s); err != nil {
		return updateState{}, false
	}
	return s, true
}

// clearUpdateState 删除状态文件（更新成功后调用）。
func (h *Handler) clearUpdateState() {
	if h.root == "" {
		return
	}
	_ = os.Remove(filepath.Join(h.root, updateStateFile))
}

// readUpdateLogTail 读取 <install_dir>/data/update.log 尾部约 8KB。
func (h *Handler) readUpdateLogTail() string {
	path := filepath.Join(h.updateInstallDir, updateLogFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) <= updateLogTailBytes {
		return string(b)
	}
	return string(b[len(b)-updateLogTailBytes:])
}
