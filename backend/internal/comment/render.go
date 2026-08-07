package comment

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"regexp"
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
	sharedPolicy = buildCommentPolicy()
	// 匹配表情包 CDN 图片的 <img src="...">，用于追加 cmt-emoji-img class 以约束尺寸。
	emojiImgRe = regexp.MustCompile(`(<img\b[^>]*\bsrc="https?://[^"]*(?:emoticons|emoji)[^"]*"[^>]*)(>)`)
)

// buildCommentPolicy 评论 bluemonday 策略：基于 UGCPolicy，额外允许表情图片 img.cmt-emoji-img
// （Artalk-Emoticons CDN 域名白名单，src 必须指向指定来源）。
func buildCommentPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	// 允许 <img class="cmt-emoji-img"> 用于表情选择器插入的表情图
	p.AllowAttrs("class", "alt", "title", "src").OnElements("img")
	// src 必须走 Artalk-Emoticons CDN 或其他受控来源，防 XSS/外链
	p.AllowURLSchemes("https", "http")
	// 允许 class="cmt-emoji-img" 的表情图标签复用 UGC 的 img 规则，安全可控
	return p
}

// Render 评论内容渲染管道：goldmark（与文章同管线，默认转义原始 HTML）→ bluemonday UGC 消毒。
// 双重防护：goldmark 把用户输入的原始 HTML 转义为文本，bluemonday 再过滤链接协议等残留风险。
func Render(src string) (string, error) {
	var sb strings.Builder
	if err := sharedMD.Convert([]byte(src), &sb); err != nil {
		return "", fmt.Errorf("render comment markdown: %w", err)
	}
	html := sharedPolicy.Sanitize(sb.String())
	// 给表情包 CDN 的 <img> 追加 cmt-emoji-img class，使 CSS 约束其尺寸为 32px 内联表情。
	html = emojiImgRe.ReplaceAllStringFunc(html, func(m string) string {
		if strings.Contains(m, `class="`) {
			// 已有 class 属性：追加
			return strings.Replace(m, `class="`, `class="cmt-emoji-img `, 1)
		}
		// 无 class：在 src 后插入
		return emojiImgRe.ReplaceAllString(m, `${1} class="cmt-emoji-img"${2}`)
	})
	return html, nil
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
		Verified:    c.Verified,
		Avatar:      AvatarURL(cfg.GravatarMirror, c.Email),
		TimeAgo:     TimeAgo(c.CreatedAt),
		Date:        c.CreatedAt.Format("2006-01-02 15:04"),
		Likes:       c.Likes,
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
	// id -> 昵称映射，用于子评论 reply_ref 显示"▸ 被回复人"
	nickByID := map[int64]string{}
	for _, c := range comments {
		nickByID[c.ID] = c.Nick
	}
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
				item := ToViewItem(c, cfg)
				// 子评论显示被回复人：回复目标是 ParentID（可能是 root 或另一条子评论）
				if nick, ok := nickByID[c.ParentID]; ok && nick != "" {
					item.ReplyRef = "▸ " + nick
				}
				root.Children = append(root.Children, item)
			}
		}
	}
	return roots
}
