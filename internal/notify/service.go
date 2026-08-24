// Package notify 站内通知中心（Wave 3b）：
// 评论事件（新评论/回复/审核通过）写入站内通知（users 表全部用户）并可选推送 Webhook。
// Webhook 地址存 settings 表 notify 段（{"webhook_url": "..."}），未配置时跳过。
package notify

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"blog/internal/safefetch"
	"blog/internal/store"
)

// Service 通知服务：写站内通知 + 可选 Webhook。所有写操作异步降级，不阻塞主流程。
type Service struct {
	st             *store.Store
	client         *http.Client
	enforceSafeURL bool
}

// New 创建通知服务。st 不可为空。
func New(st *store.Store) *Service {
	return &Service{
		st:             st,
		client:         safefetch.New(safefetch.Options{Timeout: 5 * time.Second}),
		enforceSafeURL: true,
	}
}

// NewWithClient 构造通知服务并允许测试注入 HTTP client；生产调用应使用 New，
// 由统一 safefetch 客户端拒绝非 HTTP(S)、环回、私网和保留地址。
func NewWithClient(st *store.Store, client *http.Client) *Service {
	if client == nil {
		client = safefetch.New(safefetch.Options{Timeout: 5 * time.Second})
	}
	return &Service{
		st:     st,
		client: client,
	}
}

// Event 通知事件。type 取值 comment_new / comment_reply / comment_approved。
// Title/Content/Link 用于站内通知；其余字段仅 Webhook payload 使用。
type Event struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Link       string `json:"link"`
	Nick       string `json:"nick,omitempty"`
	Email      string `json:"email,omitempty"`
	IP         string `json:"ip,omitempty"`
	PageKey    string `json:"page_key,omitempty"`
	Status     string `json:"status,omitempty"`
	ParentNick string `json:"parent_nick,omitempty"`
}

// webhookConfig settings 表 notify 段结构。
type webhookConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// Notify 处理一条通知事件：写入站内通知（users 表全部用户）+ 推送可选 Webhook。
// 内部异步执行（goroutine），失败仅记日志，不影响评论提交/审核主流程。
func (s *Service) Notify(ev Event) {
	go s.notify(ev)
}

// notify 同步实现（单测可直接调用断言）。
func (s *Service) notify(ev Event) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("notify panic: %v", r)
		}
	}()
	// 站内通知：通知全部用户（当前单管理员场景）
	users, err := s.st.ListUsers()
	if err == nil {
		for _, u := range users {
			n := &store.Notification{
				UserID:    u.ID,
				Type:      ev.Type,
				Title:     ev.Title,
				Content:   ev.Content,
				Link:      ev.Link,
				Read:      0,
				CreatedAt: time.Now().UTC(),
			}
			if err := s.st.CreateNotification(n); err != nil {
				log.Printf("notify: create notification for user %d: %v", u.ID, err)
			}
		}
	} else {
		log.Printf("notify: list users: %v", err)
	}
	s.PostWebhook(ev)
}

// PostWebhook 推送可选 Webhook：settings 表 notify 段配置 webhook_url 时，
// POST JSON（Event 全字段）。未配置/推送失败仅记日志。
func (s *Service) PostWebhook(ev Event) {
	raw, err := s.st.GetSetting("notify")
	if err != nil {
		log.Printf("notify: get webhook config: %v", err)
		return
	}
	var cfg webhookConfig
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			log.Printf("notify: decode webhook config: %v", err)
			return
		}
	}
	if cfg.WebhookURL == "" {
		return
	}
	if s.enforceSafeURL {
		if _, err := safefetch.ValidateURL(cfg.WebhookURL); err != nil {
			log.Printf("notify: reject unsafe webhook URL: %v", err)
			return
		}
	}
	body, err := json.Marshal(ev)
	if err != nil {
		log.Printf("notify: marshal webhook payload: %v", err)
		return
	}
	resp, err := s.client.Post(cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("notify: webhook post failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Printf("notify: webhook returned %d", resp.StatusCode)
	}
}

// List 分页查询某用户的通知（未读优先），转发 store。
func (s *Service) List(userID int64, page, pageSize int) ([]*store.Notification, int64, error) {
	return s.st.ListNotifications(userID, page, pageSize)
}

// MarkRead 标记单条已读，返回是否命中。
func (s *Service) MarkRead(userID, id int64) (bool, error) {
	return s.st.MarkNotificationRead(userID, id)
}

// MarkAllRead 全部标记已读，返回更新的条数。
func (s *Service) MarkAllRead(userID int64) (int64, error) {
	return s.st.MarkAllNotificationsRead(userID)
}

// UnreadCount 未读通知数（铃铛角标用）。
func (s *Service) UnreadCount(userID int64) (int64, error) {
	return s.st.CountUnreadNotifications(userID)
}
