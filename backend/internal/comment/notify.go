package comment

import (
	"crypto/tls"
	"fmt"
	"log"
	"mime"
	"net/smtp"
	"strings"

	"blog/internal/notify"
	"blog/internal/store"
)

// sendFunc 发信函数类型（可注入 seam，单测用）。
type sendFunc func(to, subject, body string)

// sendMailFn 包级发信 seam：生产走 Handler.send，单测可替换为记录器。
var sendMailOverride sendFunc

// notify 新评论邮件通知（goroutine 异步执行，不得阻塞评论提交）。
// 规则：
//   - 任何新评论（含待审）→ 通知博主（admin_email），博主自己发的除外；
//   - 回复且已发布 → 通知被回复者（有邮箱、非自回、非博主——博主已被上条覆盖）。
//
// SMTP 未配置（smtp_host 空）时整体跳过；发送失败仅记日志。
func (h *Handler) notify(cm, parent *store.Comment) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("comment notify panic: %v", r)
		}
	}()
	if h.cfg.Notify.SMTPHost == "" {
		return
	}
	adminEmail := h.cfg.AdminEmail
	sendFn := h.send
	if sendMailOverride != nil {
		sendFn = sendMailOverride
	}

	if adminEmail != "" && !strings.EqualFold(adminEmail, cm.Email) {
		subject := fmt.Sprintf("「%s」有新评论", h.siteTitle)
		body := fmt.Sprintf("%s 在 %s 发表了评论（状态：%s）：\n\n%s\n",
			cm.Nick, cm.PageKey, statusText(cm.Status), cm.ContentMD)
		h.sendFn(sendFn, adminEmail, subject, body)
	}

	if parent != nil && cm.Status == store.CommentApproved && parent.Email != "" &&
		!strings.EqualFold(parent.Email, cm.Email) && !strings.EqualFold(parent.Email, adminEmail) {
		subject := fmt.Sprintf("「%s」有人回复了你的评论", h.siteTitle)
		body := fmt.Sprintf("%s 在 %s 回复了你的评论：\n\n%s\n\n你的原评论：\n%s\n",
			cm.Nick, cm.PageKey, cm.ContentMD, parent.ContentMD)
		h.sendFn(sendFn, parent.Email, subject, body)
	}

	// 站内通知 + Webhook（Wave 3b）：规则与邮件一致，博主自己发的评论不通知。
	if h.notifier != nil && (adminEmail == "" || !strings.EqualFold(adminEmail, cm.Email)) {
		content := cm.ContentMD
		if r := []rune(content); len(r) > 200 {
			content = string(r[:200]) + "…" // 通知内容截断，防长文刷屏
		}
		ev := notify.Event{
			Type:    "comment_new",
			Title:   fmt.Sprintf("新评论：%s", cm.Nick),
			Content: content,
			Link:    cm.PageKey,
			Nick:    cm.Nick,
			Email:   cm.Email,
			IP:      cm.IP,
			PageKey: cm.PageKey,
			Status:  cm.Status,
		}
		if parent != nil {
			ev.Type = "comment_reply"
			ev.Title = fmt.Sprintf("%s 回复了评论", cm.Nick)
			ev.ParentNick = parent.Nick
		}
		h.notifier.Notify(ev)
	}
}

// sendFn 统一发信入口（支持注入 seam）。
func (h *Handler) sendFn(fn sendFunc, to, subject, body string) {
	fn(to, subject, body)
}

// statusText 评论状态中文文案（邮件用）。
func statusText(s string) string {
	if s == store.CommentPending {
		return "待审核"
	}
	return "已发布"
}

// send 发送单封邮件。465 端口走隐式 TLS（QQ/163 常用），其余端口走 STARTTLS（net/smtp 默认）。
func (h *Handler) send(to, subject, body string) {
	n := h.cfg.Notify
	from := n.From
	if from == "" {
		from = n.Username
	}
	port := n.SMTPPort
	if port == 0 {
		port = 465
	}
	addr := fmt.Sprintf("%s:%d", n.SMTPHost, port)

	// Subject 含中文需 RFC 2047 编码，否则部分客户端乱码
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n",
		from, to, mime.QEncoding.Encode("utf-8", subject))
	msg := []byte(header + body)

	auth := smtp.PlainAuth("", n.Username, n.Password, n.SMTPHost)
	var err error
	if port == 465 {
		err = sendImplicitTLS(addr, n.SMTPHost, auth, from, to, msg)
	} else {
		err = smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	if err != nil {
		log.Printf("comment notify: send mail to %s failed: %v", to, err)
	}
}

// sendImplicitTLS 465 端口隐式 TLS 发送（smtp.SendMail 只支持 STARTTLS，465 需手动 tls.Dial）。
func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close: %w", err)
	}
	return client.Quit()
}

// SendTestMail 向 admin_email 发送测试邮件（Console 配置页「发送测试」按钮用）。
// SMTP 未配置时返回错误。
func (h *Handler) SendTestMail() error {
	if h.cfg.Notify.SMTPHost == "" {
		return fmt.Errorf("SMTP 未配置（smtp_host 为空）")
	}
	to := h.cfg.AdminEmail
	if to == "" {
		return fmt.Errorf("博主邮箱（admin_email）未配置")
	}
	subject := fmt.Sprintf("「%s」邮件通知测试", h.siteTitle)
	body := "这是一封测试邮件，用于验证 SMTP 配置是否正确。\n\n如果你收到了这封邮件，说明邮件通知功能已就绪。\n"
	n := h.cfg.Notify
	from := n.From
	if from == "" {
		from = n.Username
	}
	port := n.SMTPPort
	if port == 0 {
		port = 465
	}
	addr := fmt.Sprintf("%s:%d", n.SMTPHost, port)
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n",
		from, to, mime.QEncoding.Encode("utf-8", subject))
	msg := []byte(header + body)
	auth := smtp.PlainAuth("", n.Username, n.Password, n.SMTPHost)
	if port == 465 {
		return sendImplicitTLS(addr, n.SMTPHost, auth, from, to, msg)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
