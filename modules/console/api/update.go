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
	"github.com/gin-gonic/gin"
)

// updateRepoDefault 为 api 层兜底常量，与 config.DefaultApp 对齐；Config 缺省时由 New() 填入。
// 该值同时用于推导 GHCR 镜像名（小写 owner/repo）与更新日志来源的 GitHub 仓库。
const updateRepoDefault = "SakuraLoveSmile/Sakura-Blog"

// releaseNotesMaxBytes 截断 Release 正文的上限，防止超长更新日志撑爆响应。
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
// 发布只产出 Docker 镜像（GHCR），latest_version 来自镜像版本标签；
// release_url 指向镜像包页面，release_notes / release_notes_url / published_at
// 来自 tag 发布时自动创建的 GitHub Release（仅说明、无资产），取不到时留空。
type UpdateCheckResponse struct {
	CurrentVersion      string `json:"current_version"`
	LatestVersion       string `json:"latest_version"`
	UpdateAvailable     bool   `json:"update_available"`
	ReleaseURL          string `json:"release_url"`
	ReleaseNotes        string `json:"release_notes"`
	ReleaseNotesURL     string `json:"release_notes_url"`
	PublishedAt         string `json:"published_at"`
	CheckedAt           string `json:"checked_at"`
	Error               string `json:"error,omitempty"`
	GitCommit           string `json:"git_commit,omitempty"`
	BuildTime           string `json:"build_time,omitempty"`
	StellarCommit       string `json:"stellar_commit,omitempty"`
	FrontendFingerprint string `json:"frontend_fingerprint,omitempty"`
	IsDev               bool   `json:"is_dev,omitempty"`
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
// 版本来源为 GHCR 镜像标签：只读取公开镜像包，不访问 GitHub Release。
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

	if h.registry == nil {
		resp.Error = "镜像标签客户端未启用"
		c.JSON(http.StatusOK, resp)
		return
	}

	image := strings.ToLower(strings.TrimSpace(h.updateRepo))
	tags, err := h.registry.ListTags(image, c.Query("refresh") == "1")
	if err != nil {
		resp.Error = fmt.Sprintf("检查镜像标签失败: %s（GHCR 不可达或镜像包不是公开的）", err.Error())
		c.JSON(http.StatusOK, resp)
		return
	}

	latest := latestVersionTag(tags)
	if latest == "" {
		resp.Error = "镜像包暂无可比较的版本标签"
		c.JSON(http.StatusOK, resp)
		return
	}

	resp.LatestVersion = latest
	resp.ReleaseURL = registryPackageURL(h.updateRepo)
	if current != "unknown" {
		resp.UpdateAvailable = semverCompare(latest, current) > 0
	}
	// 更新日志是可选增强：只有 Release 与最新镜像版本一致时才展示，取不到不影响版本检查。
	if release, err := h.latestRelease(c.Query("refresh") == "1"); err == nil && release != nil &&
		normalizeSemver(release.TagName) == latest {
		resp.ReleaseNotes = truncateString(release.Body, releaseNotesMaxBytes)
		resp.ReleaseNotesURL = release.HTMLURL
		if !release.PublishedAt.IsZero() {
			resp.PublishedAt = release.PublishedAt.UTC().Format(time.RFC3339)
		}
	}
	c.JSON(http.StatusOK, resp)
}

// --- helpers ---

// githubRelease 是更新日志所需的 GitHub Release 字段子集。
type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
}

// latestRelease 读取 GitHub Release 作为更新日志来源；发布链路已不再产出二进制资产，
// Release 只承载自动生成的说明。GitHub 客户端未启用或请求失败时返回 nil，不阻断版本检查。
func (h *Handler) latestRelease(refresh bool) (*githubRelease, error) {
	if h.gh == nil {
		return nil, nil
	}
	owner, repo := splitOwnerRepo(h.updateRepo)
	var (
		payload []byte
		err     error
	)
	if refresh {
		payload, err = h.gh.RefreshLatestRelease(owner, repo)
	} else {
		payload, err = h.gh.GetLatestReleaseRaw(owner, repo)
	}
	if err != nil {
		return nil, err
	}
	var release githubRelease
	if err := json.Unmarshal(payload, &release); err != nil {
		return nil, err
	}
	return &release, nil
}

// latestVersionTag 从镜像标签中挑出最高的语义化版本；
// `2`、`2.0`、`latest`、`sha-xxxx` 等派生标签不参与比较。
func latestVersionTag(tags []string) string {
	latest := ""
	for _, tag := range tags {
		version := normalizeSemver(tag)
		if version == "" {
			continue
		}
		if latest == "" || semverCompare(version, latest) > 0 {
			latest = version
		}
	}
	return latest
}

// registryPackageURL 返回镜像包页面：
// https://github.com/<owner>/<repo>/pkgs/container/<package>（GHCR 包名小写）。
func registryPackageURL(repo string) string {
	owner, name := splitOwnerRepo(strings.TrimSpace(repo))
	if owner == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/%s/pkgs/container/%s", owner, name, strings.ToLower(name))
}

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

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// splitOwnerRepo 将 "owner/repo" 拆分为 (owner, repo)，缺斜杠时整体当 repo。
func splitOwnerRepo(repo string) (owner, repoName string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", repo
}
