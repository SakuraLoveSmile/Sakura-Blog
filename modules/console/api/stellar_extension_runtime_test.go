package api

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"blog/internal/comments"
	"blog/internal/config"
)

// 所有网络访问仅连接本测试的本地 SMTP 夹具，不连接真实邮件或模型服务。
func TestStellarSettingsPublishUpdatesRealLocalConsumers(t *testing.T) {
	r, st, user, h := newFullStellarEnv(t)
	// 基础 Console 夹具为评论/Console 各建了指针；此集成夹具按生产共享指针。
	h.comments = comments.New(st, h.site)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	mailResult := make(chan bool, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			mailResult <- false
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		out := bufio.NewWriter(conn)
		write := func(value string) { _, _ = out.WriteString(value + "\r\n"); _ = out.Flush() }
		write("220 localhost test SMTP")
		scanner := bufio.NewScanner(conn)
		authOK, senderOK, recipientOK := false, false, false
		inData := false
		for scanner.Scan() {
			line := scanner.Text()
			if inData {
				if line == "." {
					inData = false
					write("250 accepted")
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				write("250-localhost\r\n250 AUTH PLAIN")
			case strings.HasPrefix(line, "AUTH PLAIN "):
				decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
				authOK = string(decoded) == "\x00fixture-user\x00fixture-mail-secret"
				write("235 authenticated")
			case strings.HasPrefix(line, "MAIL FROM:"):
				senderOK = strings.Contains(line, "sender@example.test")
				write("250 accepted")
			case strings.HasPrefix(line, "RCPT TO:"):
				recipientOK = strings.Contains(line, "owner@example.test")
				write("250 accepted")
			case line == "DATA":
				inData = true
				write("354 end with dot")
			case line == "QUIT":
				write("221 goodbye")
				mailResult <- authOK && senderOK && recipientOK
				return
			default:
				write("500 unexpected command")
			}
		}
		mailResult <- false
	}()
	callbackCount := 0
	h.onLinksChanged = func() { callbackCount++ }
	document := fmt.Sprintf(`{"version":2,"theme":{},"blog_extensions":{"native_comments":{"enabled":false,"admin_email":"owner@example.test","notify":{"smtp_host":"127.0.0.1","smtp_port":%d,"username":"fixture-user","from":"sender@example.test","password":"fixture-mail-secret"}},"friends":{"posts_per_link":4,"check_interval":0},"ai":{"api_key":"fixture-model-secret","summary":{"enabled":false,"allow_user_api":false},"ask":{"enabled":false,"allow_user_llm":false}}}}`, port)
	cookie := authCookie(user)
	response := do2FA(r, http.MethodPut, "/api/v1/settings/stellar", document, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("canonical save: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "fixture-mail-secret") || strings.Contains(response.Body.String(), "fixture-model-secret") {
		t.Fatal("private credentials leaked in API response")
	}
	if callbackCount != 1 || h.site.Load().Friends.MaxPostsPerFeed != 4 || h.site.Load().Comments.Enabled {
		t.Fatal("local runtime or friends refresh was not published")
	}
	response = do2FA(r, http.MethodPost, "/api/v1/settings/notify/test", "", cookie)
	var sent struct {
		OK bool `json:"ok"`
	}
	if json.Unmarshal(response.Body.Bytes(), &sent) != nil || response.Code != http.StatusOK || !sent.OK {
		t.Fatalf("actual SMTP service did not use saved config: %d", response.Code)
	}
	select {
	case ok := <-mailResult:
		if !ok {
			t.Fatal("SMTP credentials or recipient did not come from canonical config")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("isolated SMTP fixture did not finish")
	}
	response = do2FA(r, http.MethodPost, "/api/v1/ai/summary/guest", `{}`, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("saved reader AI policy not applied: %d", response.Code)
	}
	cfg, err := config.LoadAI(st)
	if err != nil || cfg.APIKey != "fixture-model-secret" || cfg.SummaryOn() || cfg.UserAPIOn() || cfg.QAOn() || cfg.QAUserLLMOn() {
		t.Fatal("backend AI settings did not use canonical document")
	}
	before, _ := st.ReadSettingsSnapshot()
	old := h.site.Load()
	response = do2FA(r, http.MethodPut, "/api/v1/settings/stellar", `{"version":2,"theme":{},"blog_extensions":{"native_comments":{"notify":{"smtp_port":"secret-text"}}}}`, cookie)
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "secret-text") {
		t.Fatal("invalid service config accepted or reflected raw data")
	}
	after, _ := st.ReadSettingsSnapshot()
	if len(before) != len(after) || before[config.SectionBlogExtensions] != after[config.SectionBlogExtensions] || h.site.Load() != old || callbackCount != 1 {
		t.Fatal("failed save changed service state or refreshed friends")
	}
}
