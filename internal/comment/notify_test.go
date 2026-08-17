package comment

import (
	"strings"
	"sync/atomic"
	"testing"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// mailRecord 记录一次发信调用。
type mailRecord struct {
	To      string
	Subject string
	Body    string
}

// setupNotify 创建带 mock 发信的 Handler，返回 Handler 和记录切片指针。
func setupNotify(t *testing.T, cfg *tmpl.CommentsConfig) (*Handler, *[]mailRecord) {
	t.Helper()
	var records []mailRecord
	sendMailOverride = func(to, subject, body string) {
		records = append(records, mailRecord{To: to, Subject: subject, Body: body})
	}
	t.Cleanup(func() { sendMailOverride = nil })
	// Spec 61 阶段 3：Handler 改持站点原子指针，测试用指定配置构造
	site := &tmpl.Site{Comments: *cfg, Title: "测试博客"}
	sp := &atomic.Pointer[tmpl.Site]{}
	sp.Store(site)
	h := &Handler{site: sp}
	return h, &records
}

func baseCfg() *tmpl.CommentsConfig {
	return &tmpl.CommentsConfig{
		AdminEmail: "admin@blog.com",
		Notify: tmpl.NotifyConfig{
			SMTPHost: "smtp.test.com",
			SMTPPort: 465,
			Username: "admin@blog.com",
			Password: "pass",
		},
	}
}

// TestNotifyNewCommentToAdmin 新评论（非博主）→ 通知博主。
func TestNotifyNewCommentToAdmin(t *testing.T) {
	h, records := setupNotify(t, baseCfg())
	cm := &store.Comment{Nick: "访客", Email: "visitor@x.com", PageKey: "/post/hello", ContentMD: "好文章", Status: store.CommentApproved}
	h.notify(cm, nil)
	if len(*records) != 1 {
		t.Fatalf("expect 1 mail, got %d", len(*records))
	}
	if (*records)[0].To != "admin@blog.com" {
		t.Errorf("to = %s, want admin@blog.com", (*records)[0].To)
	}
	if !strings.Contains((*records)[0].Subject, "新评论") {
		t.Errorf("subject = %s", (*records)[0].Subject)
	}
}

// TestNotifySelfCommentNoNotify 博主自己评论 → 不通知博主。
func TestNotifySelfCommentNoNotify(t *testing.T) {
	h, records := setupNotify(t, baseCfg())
	cm := &store.Comment{Nick: "博主", Email: "admin@blog.com", PageKey: "/post/hello", ContentMD: "自己评论", Status: store.CommentApproved}
	h.notify(cm, nil)
	if len(*records) != 0 {
		t.Fatalf("expect 0 mails for self-comment, got %d", len(*records))
	}
}

// TestNotifyPendingCommentToAdmin 待审评论 → 仍通知博主。
func TestNotifyPendingCommentToAdmin(t *testing.T) {
	h, records := setupNotify(t, baseCfg())
	cm := &store.Comment{Nick: "访客", Email: "v@x.com", PageKey: "/post/a", ContentMD: "待审", Status: store.CommentPending}
	h.notify(cm, nil)
	if len(*records) != 1 {
		t.Fatalf("expect 1 mail for pending, got %d", len(*records))
	}
	if !strings.Contains((*records)[0].Body, "待审核") {
		t.Error("body should mention 待审核")
	}
}

// TestNotifyReplyToParent 已发布回复 → 通知被回复者。
func TestNotifyReplyToParent(t *testing.T) {
	h, records := setupNotify(t, baseCfg())
	parent := &store.Comment{Nick: "层主", Email: "parent@x.com", ContentMD: "原评论"}
	cm := &store.Comment{Nick: "回复者", Email: "reply@x.com", PageKey: "/post/a", ContentMD: "回复内容", Status: store.CommentApproved}
	h.notify(cm, parent)
	// 应发 2 封：1 给博主（新评论），1 给被回复者
	if len(*records) != 2 {
		t.Fatalf("expect 2 mails, got %d", len(*records))
	}
	if (*records)[1].To != "parent@x.com" {
		t.Errorf("reply notify to = %s, want parent@x.com", (*records)[1].To)
	}
}

// TestNotifySelfReplyNoNotify 自回（回复自己）→ 不通知被回复者。
func TestNotifySelfReplyNoNotify(t *testing.T) {
	h, records := setupNotify(t, baseCfg())
	parent := &store.Comment{Nick: "A", Email: "same@x.com", ContentMD: "原"}
	cm := &store.Comment{Nick: "A", Email: "same@x.com", PageKey: "/post/a", ContentMD: "自回", Status: store.CommentApproved}
	h.notify(cm, parent)
	// 只有博主通知（1 封），无被回复者通知
	if len(*records) != 1 {
		t.Fatalf("expect 1 mail (admin only), got %d", len(*records))
	}
	if (*records)[0].To != "admin@blog.com" {
		t.Errorf("to = %s, want admin", (*records)[0].To)
	}
}

// TestNotifyReplyToAdminNoDuplicate 回复博主 → 不重复通知博主（已被新评论规则覆盖）。
func TestNotifyReplyToAdminNoDuplicate(t *testing.T) {
	h, records := setupNotify(t, baseCfg())
	parent := &store.Comment{Nick: "博主", Email: "admin@blog.com", ContentMD: "博主原评论"}
	cm := &store.Comment{Nick: "访客", Email: "v@x.com", PageKey: "/post/a", ContentMD: "回复博主", Status: store.CommentApproved}
	h.notify(cm, parent)
	// 只有 1 封给博主（新评论规则），不重复发回复通知
	if len(*records) != 1 {
		t.Fatalf("expect 1 mail, got %d", len(*records))
	}
}

// TestNotifySMTPNotConfigured smtp_host 空 → 整体跳过。
func TestNotifySMTPNotConfigured(t *testing.T) {
	cfg := baseCfg()
	cfg.Notify.SMTPHost = ""
	h, records := setupNotify(t, cfg)
	cm := &store.Comment{Nick: "访客", Email: "v@x.com", PageKey: "/post/a", ContentMD: "test", Status: store.CommentApproved}
	h.notify(cm, nil)
	if len(*records) != 0 {
		t.Fatalf("expect 0 mails when smtp not configured, got %d", len(*records))
	}
}

// TestNotifyPendingReplyNotToParent 待审回复 → 不通知被回复者（仅通知博主）。
func TestNotifyPendingReplyNotToParent(t *testing.T) {
	h, records := setupNotify(t, baseCfg())
	parent := &store.Comment{Nick: "层主", Email: "parent@x.com", ContentMD: "原"}
	cm := &store.Comment{Nick: "回复者", Email: "reply@x.com", PageKey: "/post/a", ContentMD: "待审回复", Status: store.CommentPending}
	h.notify(cm, parent)
	// 只有博主通知，被回复者不收（待审）
	if len(*records) != 1 {
		t.Fatalf("expect 1 mail (admin only for pending reply), got %d", len(*records))
	}
}
