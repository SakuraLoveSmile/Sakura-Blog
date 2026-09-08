package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blog/internal/app"
	ghpkg "blog/internal/github"
	updr "blog/internal/updaterunner"
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

// SystemVersionResponse GET /api/v1/system/version
type SystemVersionResponse struct {
	Version             string `json:"version"`
	GitCommit           string `json:"git_commit"`
	BuildTime           string `json:"build_time"`
	StellarCommit       string `json:"stellar_commit"`
	FrontendFingerprint string `json:"frontend_fingerprint"`
	IsDev               bool   `json:"is_dev"`
}

// UpdateCheckResponse GET /api/v1/system/update/check
type UpdateCheckResponse struct {
	CurrentVersion      string `json:"current_version"`
	LatestVersion       string `json:"latest_version"`
	UpdateAvailable     bool   `json:"update_available"`
	ReleaseURL          string `json:"release_url"`
	ReleaseNotes        string `json:"release_notes"`
	PublishedAt         string `json:"published_at"`
	ApplySupported      bool   `json:"apply_supported"`
	CheckedAt           string `json:"checked_at"`
	ApplyNote           string `json:"apply_note,omitempty"`
	Error               string `json:"error,omitempty"`
	GitCommit           string `json:"git_commit,omitempty"`
	BuildTime           string `json:"build_time,omitempty"`
	StellarCommit       string `json:"stellar_commit,omitempty"`
	FrontendFingerprint string `json:"frontend_fingerprint,omitempty"`
	IsDev               bool   `json:"is_dev,omitempty"`
}

// UpdateApplyResponse POST /api/v1/system/update/apply
type UpdateApplyResponse struct {
	Started       bool   `json:"started"`
	TaskID        string `json:"task_id"`
	TargetVersion string `json:"target_version"`
	Stage         string `json:"stage,omitempty"`
}

// UpdateStatusResponse GET /api/v1/system/update/status
type UpdateStatusResponse struct {
	State          string `json:"state"` // idle|running|succeeded|failed
	Stage          string `json:"stage,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	CurrentVersion string `json:"current_version"`
	TargetVersion  string `json:"target_version,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	LogTail        string `json:"log_tail,omitempty"`
	Message        string `json:"message,omitempty"`
	Error          string `json:"error,omitempty"`
}

// githubClientFor 解析本次请求的 GitHub 客户端：
// 若请求携带 X-GitHub-Token 头，则构造一个带该 Token 的请求级客户端（复用同一磁盘缓存），
// 用于私有仓库鉴权或提升速率上限；否则回落到启动时构建的共享客户端（已含 app.yml / 环境变量 Token）。
// 两者皆不可用时返回 nil（GitHub 功能未启用）。
func (h *Handler) githubClientFor(c *gin.Context) *ghpkg.Client {
	if t := strings.TrimSpace(c.GetHeader("X-GitHub-Token")); t != "" {
		cacheDir := "data/ghcache"
		if h.gh != nil {
			cacheDir = h.gh.CacheDir()
		}
		return ghpkg.New(ghpkg.Config{Token: t, CacheDir: cacheDir})
	}
	return h.gh
}

// systemVersion GET /api/v1/system/version
func (h *Handler) systemVersion(c *gin.Context) {
	current := h.resolveCurrentVersion()
	resp := SystemVersionResponse{
		Version:             current,
		GitCommit:           app.GitCommit,
		BuildTime:           app.BuildTime,
		StellarCommit:       app.StellarCommit,
		FrontendFingerprint: app.FrontendFingerprint,
		IsDev:               app.IsDev(),
	}
	c.JSON(http.StatusOK, resp)
}

// systemUpdateCheck GET /api/v1/system/update/check?refresh=1
func (h *Handler) systemUpdateCheck(c *gin.Context) {
	current := h.resolveCurrentVersion()
	resp := UpdateCheckResponse{
		CurrentVersion:      current,
		CheckedAt:           time.Now().UTC().Format(time.RFC3339),
		ApplySupported:      h.upd.Supported(),
		GitCommit:           app.GitCommit,
		BuildTime:           app.BuildTime,
		StellarCommit:       app.StellarCommit,
		FrontendFingerprint: app.FrontendFingerprint,
		IsDev:               app.IsDev(),
	}

	if !h.upd.Supported() {
		// 不支持一键更新时仅通过 ApplyNote 告知原因，不污染 error——
		// error 只承载「检查失败」，避免真实上游错误（404/403/网络）被「非 Linux」文案掩盖。
		resp.ApplyNote = h.upd.UnsupportedReason()
	}

	gh := h.githubClientFor(c)
	if gh == nil {
		resp.Error = "GitHub 客户端未启用"
		c.JSON(200, resp)
		return
	}

	refresh := c.Query("refresh") == "1"
	var payload []byte
	var err error
	if refresh {
		payload, err = gh.RefreshLatestRelease(splitOwnerRepo(h.updateRepo))
	} else {
		payload, err = gh.GetLatestReleaseRaw(splitOwnerRepo(h.updateRepo))
	}
	if err != nil {
		resp.Error = fmt.Sprintf("检查更新失败: %s（私有仓库可在「系统更新」页填入 GitHub Token 鉴权；仓库无 Release 时也会返回 404）", err.Error())
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
		resp.Error = "解析 GitHub release 失败: " + err.Error()
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

	// 先查 latest 版本以确定 target（携带请求级 Token 以支持私有仓库）
	gh := h.githubClientFor(c)
	if gh == nil {
		Fail(c, 500, "GitHub 客户端未启用")
		return
	}
	payload, err := gh.GetLatestReleaseRaw(splitOwnerRepo(h.updateRepo))
	if err != nil {
		// 私有仓库无 Token 时 GitHub 返回 404，给出可操作提示
		if strings.Contains(err.Error(), "404") {
			Fail(c, 500, "获取最新版本失败: "+err.Error()+"（私有仓库可在「系统更新」页填入 GitHub Token 鉴权）")
			return
		}
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

	task, err := h.upd.Start(ctx, target)
	if err != nil {
		if errors.Is(err, updr.ErrBusy) {
			Fail(c, 409, "已有更新任务正在进行中")
			return
		}
		Fail(c, 500, "触发更新服务失败: "+err.Error())
		return
	}

	// 写入状态文件供重启后恢复上下文
	state := updateState{
		From:      current,
		To:        target,
		StartedAt: task.StartedAt,
	}
	h.writeUpdateState(state)

	c.JSON(202, UpdateApplyResponse{
		Started:       true,
		TaskID:        task.TaskID,
		TargetVersion: task.TargetVersion,
		Stage:         task.Stage,
	})
}

// systemUpdateStatus GET /api/v1/system/update/status
func (h *Handler) systemUpdateStatus(c *gin.Context) {
	current := h.resolveCurrentVersion()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	statusInfo, err := h.upd.Status(ctx, current)
	if err != nil {
		Fail(c, 500, "查询更新状态失败: "+err.Error())
		return
	}

	resp := UpdateStatusResponse{
		State:          statusInfo.State,
		Stage:          statusInfo.Stage,
		TaskID:         statusInfo.TaskID,
		CurrentVersion: current,
		TargetVersion:  statusInfo.TargetVersion,
		StartedAt:      statusInfo.StartedAt,
		LogTail:        statusInfo.LogTail,
		Message:        statusInfo.Message,
		Error:          statusInfo.Error,
	}

	// 如果 runner 处于 idle，但本地有向后兼容的 update-state.json
	if resp.State == "idle" {
		if state, hasState := h.readUpdateState(); hasState {
			resp.TargetVersion = state.To
			resp.StartedAt = state.StartedAt
			if h.upd.Active(ctx) {
				resp.State = "running"
				resp.Stage = "replacing"
				resp.Message = "更新正在进行中，服务即将重启"
			} else if current == state.To {
				resp.State = "succeeded"
				resp.Stage = "succeeded"
				resp.Message = "更新成功"
			} else {
				resp.State = "failed"
				resp.Stage = "failed"
				resp.Message = "更新失败或已回滚，请查看日志"
			}
			if resp.LogTail == "" {
				resp.LogTail = h.readUpdateLogTail()
			}
		}
	}

	if resp.LogTail == "" {
		resp.LogTail = h.readUpdateLogTail()
	}

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
