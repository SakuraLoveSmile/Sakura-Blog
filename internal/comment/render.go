package comment

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// 包级共享渲染管道：goldmark 与 bluemonday 均并发安全，公开 API 与 admin 回复共用。
var (
	sharedMD     = goldmark.New() // 评论无需 AutoHeadingID
	sharedPolicy = bluemonday.UGCPolicy()
)

// Render 评论内容渲染管道：goldmark（与文章同管线，默认转义原始 HTML）→ bluemonday UGC 消毒。
// 双重防护：goldmark 把用户输入的原始 HTML 转义为文本，bluemonday 再过滤链接协议等残留风险。
func Render(src string) (string, error) {
	var sb strings.Builder
	if err := sharedMD.Convert([]byte(src), &sb); err != nil {
		return "", fmt.Errorf("render comment markdown: %w", err)
	}
	return sharedPolicy.Sanitize(sb.String()), nil
}

// AvatarURL 由邮箱生成 Gravatar 头像 URL（md5(小写 trim 后邮箱)），?d=mm 兜底默认头像。
func AvatarURL(mirror, email string) string {
	sum := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return fmt.Sprintf("%s/%x?d=mm", strings.TrimRight(mirror, "/"), sum)
}

// TimeAgo 相对时间，文案对齐 Stellar ctx.date_suffix（刚刚/N 分钟前/N 小时前/N 天前）。
// 导出供 main.go 最新评论 widget 复用。
func TimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// ToViewItem 单条 store 评论转前台视图模型。
func ToViewItem(c *store.Comment, cfg *tmpl.CommentsConfig) *tmpl.CommentItem {
	return &tmpl.CommentItem{
		ID:          c.ID,
		Nick:        c.Nick,
		Link:        c.Link,
		ContentHTML: template.HTML(c.ContentHTML), // 服务端已消毒，模板直接输出
		IsAdmin:     c.IsAdmin,
		Avatar:      AvatarURL(cfg.GravatarMirror, c.Email),
		TimeAgo:     TimeAgo(c.CreatedAt),
		Date:        c.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// ToViewItems 把按时间正序的 store 评论列表组装成两层树：
// 顶级评论（root_id=0）保持正序，子评论挂到对应 root 的 Children 下。
// root 不在结果集（父评论待审/已删）的子评论不展示，属预期行为。
// main.go SSR 填充与 API 返回共用。
func ToViewItems(comments []*store.Comment, cfg *tmpl.CommentsConfig) []*tmpl.CommentItem {
	// 初始化为空 slice 而非 nil：无评论时 JSON 序列化为 [] 而不是 null（API 消费方更友好）
	roots := []*tmpl.CommentItem{}
	byID := map[int64]*tmpl.CommentItem{}
	for _, c := range comments {
		if c.RootID == 0 {
			item := ToViewItem(c, cfg)
			byID[c.ID] = item
			roots = append(roots, item)
		}
	}
	for _, c := range comments {
		if c.RootID != 0 {
			if root, ok := byID[c.RootID]; ok {
				root.Children = append(root.Children, ToViewItem(c, cfg))
			}
		}
	}
	return roots
}
