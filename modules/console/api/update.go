package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blog/internal/app"
	ghpkg "blog/internal/github"
	"github.com/gin-gonic/gin"
)

// updateRepoDefault 为 api 层兜底常量，与 config.DefaultApp 对齐；Config 缺省时由 New() 填入。
const updateRepoDefault = "SakuraLoveSmile/Sakura-Blog-Release"

// releaseNotesMaxBytes 截断 release body 的上限，防止超长正文撑爆响应。
const releaseNotesMaxBytes = 32 * 1024

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
	CheckedAt           string `json:"checked_at"`
	Error               string `json:"error,omitempty"`
	GitCommit           string `json:"git_commit,omitempty"`
	BuildTime           string `json:"build_time,omitempty"`
	StellarCommit       string `json:"stellar_commit,omitempty"`
	FrontendFingerprint string `json:"frontend_fingerprint,omitempty"`
	IsDev               bool   `json:"is_dev,omitempty"`
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
		GitCommit:           app.GitCommit,
		BuildTime:           app.BuildTime,
		StellarCommit:       app.StellarCommit,
		FrontendFingerprint: app.FrontendFingerprint,
		IsDev:               app.IsDev(),
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
