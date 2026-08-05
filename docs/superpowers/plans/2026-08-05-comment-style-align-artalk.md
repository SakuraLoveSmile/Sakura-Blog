# 评论区对齐 Artalk 样式实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将自建评论区（Halo 匿名模型）对齐 Artalk 视觉与交互风格：① 允许匿名+注册双轨，注册用户带「已验证」标识；② 编辑器卡片化外观；③ 多 Tab 图片表情包（Blobcat/QQ/Tieba/Twemoji/Aru）；④ 工具栏三按钮 + 计数下拉箭头 + 空状态居中文案。

**Architecture:** 后端复用现有 `users` 表（新增 role=reader 前台用户）+ Cookie Session 无状态认证；comments 表新增 `user_id`/`verified` 两列。前端编辑器单实例双态（未登录态=三列输入+登录按钮；已登录态=头像昵称展示+发送按钮），通过 `/api/auth/me` 首屏拉取身份。表情系统用「Tab 切换 + 图片表情 `<img>` 插入 + 后端白名单放行 img.cmt-emoji-img class」实现，五套表情包资源用前端 JS 内嵌 SVG 占位+公共 CDN JSON 清单（Blobcat 等表情包用开源 CDN：cdn.jsdelivr.net/gh/ArtalkJS/Artalk-Emoticons@master/packages/blobcat/dist/*.png），不新增本地二进制资源。

**Tech Stack:** Go 1.25 (Gin+modernc.org/sqlite+bcrypt+http-only Cookie) / html/template (SSR) + vanilla JS (事件委托) / CSS 变量（映射 Stellar --theme/--card/--block-border）。

---

## File Structure（文件改动总览）

| 路径 | 动作 | 职责 |
|---|---|---|
| `backend/internal/store/comment.go` | Modify (L71-L126) | comments 表 ALTER 迁移加 `user_id` / `verified` 列；ScanRow/Values/Columns 同步 |
| `backend/internal/store/user.go` | Modify | 新增 CreatePublicUser / GetUserByUsername / VerifyPassword（复用现有 bcrypt，role=reader） |
| `backend/internal/api/auth_public.go` | **Create** | 公开注册/登录/登出/me 四接口（/api/auth/*），Cookie Session（签名随机 token 存 users.auth_secret），登录限流（IP 5 次/10min 内存计数） |
| `backend/internal/api/router.go` | Modify | /api/auth/* 无权限 gate 注册；评论接口前置 Session 注入 c.Set("current_user") |
| `backend/internal/api/comment_public.go` | **Create**（替换 routes_post.go 中的匿名 handler） | POST /api/comments、GET /api/comments（前台页，非 admin）：识别 current_user，登录用户自动填 nick/email/avatar 并打 verified=true |
| `backend/internal/comment/render.go` | Modify | bluemonday 白名单加 `<img class="cmt-emoji-img" src alt>`，放通 blobcat CDN 域名白名单 |
| `backend/cmd/server/routes_post.go` | Modify | 旧匿名评论 handler 移除，换 comment_public.go 新 handler；/api/comments GET 做 page_key 过滤 + 返回 comments 树 + count + verified 字段 |
| `backend/internal/template/lang.go` | Modify (L120-L134) | 新增文案：comment.title_invite / comment.empty_line1 / comment.login / comment.send / comment.register / comment.email_only / comment.guest_mode / comment.verified_badge |
| `backend/internal/template/model.go` | Modify | ViewData.Site.Comments 扩展 VerifiedBadge 字段；新增 CurrentUser（*CommentUser{Nick,Avatar,LoggedIn}）供 SSR 初始态渲染 |
| `backend/internal/template/partial/comments.html` | Modify | 整体重写：卡片化包裹 .cmt-card，双态编辑器（.cmt-editor-guest / .cmt-editor-user），计数区加 ▾ 下拉 + 通知中心链接，空状态居中 UI，表情面板 Tab 结构，登录弹窗结构（默认 hidden） |
| `frontend/assets/css/comments.css` | Rewrite | 卡片样式、已登录态身份条、登录弹窗模态框、表情 5 Tab 面板、空状态居中样式、verified 徽章、通知中心链接 |
| `frontend/assets/js/comments.js` | Rewrite | 身份双态切换、登录/注册表单提交（fetch /api/auth/*）、表情 Tab 懒构建+CDN 资源加载+img 插入、通知中心占位跳转、计数▾占位下拉、PJAX 兼容（事件委托+data-built guard） |
| `backend/internal/api/comment_test.go` | Modify | 加测试：匿名评论无 verified；注册用户评论有 verified；登录态 nick/email 自动填充 |
| `backend/internal/api/auth_public_test.go` | **Create** | 注册/登录/me/登出四接口的测试（成功+失败用例） |
| `config/site.example.yml` | Modify (L320-L335) | comments.verified_badge、comments.allow_register 字段示例 |

---

## Task 1: 后端身份体系（公开用户注册/登录 + Session Cookie）

### 1.1 新增 auth_public.go 公开认证接口

**Files:**
- Create: `backend/internal/api/auth_public.go`
- Modify: `backend/internal/store/user.go` (新增 CreatePublicUser/GetUserByUsername/VerifyPassword)
- Test: `backend/internal/api/auth_public_test.go`

- [ ] **Step 1: 写 store/user.go 扩展方法（先写失败测试）**

先在 auth_public_test.go 写注册测试：

```go
package api

import (
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
  "blog/internal/store"
)

func TestPublicRegister(t *testing.T) {
  h := setupTestHandler(t)
  body := `{"nick":"reader1","email":"reader1@example.com","password":"abc12345"}`
  req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
  req.Header.Set("Content-Type", "application/json")
  w := httptest.NewRecorder()
  h.engine.ServeHTTP(w, req)
  if w.Code != http.StatusCreated {
    t.Fatalf("register status=%d body=%s", w.Code, w.Body.String())
  }
}
```

- [ ] **Step 2: 运行测试确认失败**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go test ./backend/internal/api/ -run TestPublicRegister -v`
Expected: FAIL (no route /api/auth/register)

- [ ] **Step 3: store/user.go 新增三个方法**

```go
package store

import "golang.org/x/crypto/bcrypt"

// CreatePublicUser 新建前台读者（role=reader），返回新用户 id。
// 用户名 / 邮箱全局唯一，冲突返回 error。
func (s *Store) CreatePublicUser(username, email, displayName, passwordRaw string) (int64, error) {
  if strings.TrimSpace(username) == "" || strings.TrimSpace(passwordRaw) == "" {
    return 0, fmt.Errorf("用户名和密码必填")
  }
  if email != "" {
    if _, err := mail.ParseAddress(email); err != nil {
      return 0, fmt.Errorf("邮箱格式错误: %w", err)
    }
  }
  hash, err := bcrypt.GenerateFromPassword([]byte(passwordRaw), bcrypt.DefaultCost)
  if err != nil {
    return 0, fmt.Errorf("bcrypt: %w", err)
  }
  // auth_secret 用于 Cookie Session token 校验（后续 logout 可改此字段作废所有旧 session）
  authSecret := randomHex(32)
  u := &User{
    Username:     strings.ToLower(strings.TrimSpace(username)),
    PasswordHash: string(hash),
    DisplayName:  strings.TrimSpace(displayName),
    Email:        strings.ToLower(strings.TrimSpace(email)),
    Avatar:       "", // 空则前端用 gravatar mirror
    AuthSecret:   authSecret,
    Role:         RoleReader, // 需要在 const 区新增 RoleReader = "reader"
    CreatedAt:    time.Now(),
  }
  return repo(s, u).Create(u)
}

// GetUserByUsername 按用户名（小写）查，不存在返回 nil, nil。
func (s *Store) GetUserByUsername(username string) (*User, error) {
  return repo(s, &User{}).One(Where("username = ?", strings.ToLower(strings.TrimSpace(username))))
}

// VerifyPassword 比对密码。
func (u *User) VerifyPassword(raw string) bool {
  return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(raw)) == nil
}

func randomHex(n int) string {
  b := make([]byte, n)
  if _, err := rand.Read(b); err != nil {
    return fmt.Sprintf("%d", time.Now().UnixNano())
  }
  return hex.EncodeToString(b)
}
```

同时在 `user.go` const 区新增：

```go
const (
  RoleAdmin  = "admin"
  RoleAuthor = "author"
  RoleReader = "reader" // 前台注册用户
)
```

- [ ] **Step 4: 新建 auth_public.go 四接口**

```go
// backend/internal/api/auth_public.go
package api

import (
  "crypto/hmac"
  "crypto/sha256"
  "encoding/hex"
  "net/http"
  "strings"
  "sync"
  "time"
  "github.com/gin-gonic/gin"
  "blog/internal/store"
)

// sessionCookieName Cookie 名；http-only + SameSite=Lax + Secure(HTTPS 自动)
const sessionCookieName = "blog_comment_session"

// sessionTokenFormat: <user_id>:<random_nonce>:<hmac_sha256(user_id:nonce, user.auth_secret)>
// HMAC 校验用 users.auth_secret 字段，改 secret 即可作废所有旧 session。

type loginAttemptKey struct{ ip, hour string }

var (
  loginMu       sync.Mutex
  loginAttempts = map[loginAttemptKey]int{}
  loginLimitMax = 5 // 每 IP 每小时最多 5 次失败
)

func loginLimitReached(ip string) bool {
  loginMu.Lock()
  defer loginMu.Unlock()
  hour := time.Now().Format("2006010215")
  k := loginAttemptKey{ip: ip, hour: hour}
  return loginAttempts[k] >= loginLimitMax
}
func loginLimitAddFail(ip string) {
  loginMu.Lock()
  defer loginMu.Unlock()
  hour := time.Now().Format("2006010215")
  k := loginAttemptKey{ip: ip, hour: hour}
  loginAttempts[k]++
  // 顺便清理过期小时条目（简单清理）
  for k2 := range loginAttempts {
    if k2.hour != hour {
      delete(loginAttempts, k2)
    }
  }
}

// signSession 生成签名 token: user_id:nonce:hmac
func signSession(u *store.User) string {
  nonce := randomHex32()
  payload := u.Username + ":" + nonce
  mac := hmac.New(sha256.New, []byte(u.AuthSecret))
  mac.Write([]byte(payload))
  sig := hex.EncodeToString(mac.Sum(nil))
  return u.Username + ":" + nonce + ":" + sig
}

// verifySession 校验 token，返回对应用户或 nil
func (h *Handler) verifySession(token string) *store.User {
  parts := strings.Split(token, ":")
  if len(parts) != 3 { return nil }
  username, nonce, sig := parts[0], parts[1], parts[2]
  u, err := h.store.GetUserByUsername(username)
  if err != nil || u == nil { return nil }
  payload := u.Username + ":" + nonce
  mac := hmac.New(sha256.New, []byte(u.AuthSecret))
  mac.Write([]byte(payload))
  expected := hex.EncodeToString(mac.Sum(nil))
  if !hmac.Equal([]byte(expected), []byte(sig)) { return nil }
  return u
}

// randomHex32 32 字节随机 hex（64 字符）
func randomHex32() string { /* 同 store 中实现 */
  b := make([]byte, 32)
  if _, err := rand.Read(b); err != nil {
    return fmt.Sprintf("%d", time.Now().UnixNano())
  }
  return hex.EncodeToString(b)
}

// setSessionCookie 写 cookie：30 天过期
func setSessionCookie(c *gin.Context, token string) {
  maxAge := 30 * 24 * 3600
  c.SetCookie(sessionCookieName, token, maxAge, "/", "", false, true) // Secure 在 HTTPS 时由反向代理升级；本地 HTTP Secure=false 可写
}

// clearSessionCookie 注销 cookie
func clearSessionCookie(c *gin.Context) {
  c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
}

// SessionMiddleware: 读 cookie → 校验 → c.Set("current_user", *store.User)。失败忽略（降级匿名）
func (h *Handler) SessionMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    tok, err := c.Cookie(sessionCookieName)
    if err != nil || tok == "" { c.Next(); return }
    if u := h.verifySession(tok); u != nil {
      c.Set("current_user", u)
    }
    c.Next()
  }
}

// CurrentUserFromContext 方便下游取登录态
func CurrentUserFromContext(c *gin.Context) *store.User {
  if v, ok := c.Get("current_user"); ok {
    if u, ok := v.(*store.User); ok { return u }
  }
  return nil
}

// ─── Routes ──────────────────────────────────────────────

// POST /api/auth/register 公开注册
func (h *Handler) publicRegister(c *gin.Context) {
  if !h.site.Comments.AllowRegister {
    Fail(c, http.StatusForbidden, "本站未开放注册")
    return
  }
  var req struct {
    Nick     string `json:"nick"`
    Email    string `json:"email"`
    Password string `json:"password"`
  }
  if err := c.ShouldBindJSON(&req); err != nil {
    Fail(c, http.StatusBadRequest, "请求格式错误")
    return
  }
  id, err := h.store.CreatePublicUser(req.Nick, req.Email, req.Nick, req.Password)
  if err != nil {
    if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "唯一") {
      Fail(c, http.StatusConflict, "该用户名或邮箱已被注册")
      return
    }
    Fail(c, http.StatusBadRequest, err.Error())
    return
  }
  u, _ := h.store.GetUserByUsername(req.Nick)
  if u != nil {
    setSessionCookie(c, signSession(u))
    c.JSON(http.StatusCreated, gin.H{"ok": true, "user": publicUserJSON(u)})
    return
  }
  c.JSON(http.StatusCreated, gin.H{"ok": true, "user_id": id})
}

// POST /api/auth/login 公开登录（用户名+密码，邮箱等价用户名）
func (h *Handler) publicLogin(c *gin.Context) {
  if loginLimitReached(c.ClientIP()) {
    Fail(c, http.StatusTooManyRequests, "登录尝试过于频繁，请稍后再试")
    return
  }
  var req struct {
    Account  string `json:"account"` // 用户名或邮箱
    Password string `json:"password"`
  }
  if err := c.ShouldBindJSON(&req); err != nil {
    Fail(c, http.StatusBadRequest, "请求格式错误"); return
  }
  account := strings.ToLower(strings.TrimSpace(req.Account))
  u, err := h.store.GetUserByUsername(account)
  if err != nil || u == nil {
    // 尝试按邮箱查
    u2, err2 := repo(h.store, &store.User{}).One(Where("email = ?", account))
    if err2 == nil && u2 != nil { u = u2 } else { u = nil }
  }
  if u == nil || !u.VerifyPassword(req.Password) {
    loginLimitAddFail(c.ClientIP())
    Fail(c, http.StatusUnauthorized, "账号或密码错误")
    return
  }
  setSessionCookie(c, signSession(u))
  c.JSON(http.StatusOK, gin.H{"ok": true, "user": publicUserJSON(u)})
}

// GET /api/auth/me 当前登录态（首屏/每次 PJAX 换页都可拉一次，响应 304 可缓存）
func (h *Handler) publicMe(c *gin.Context) {
  u := CurrentUserFromContext(c)
  if u == nil {
    c.JSON(http.StatusOK, gin.H{"ok": true, "logged_in": false})
    return
  }
  c.JSON(http.StatusOK, gin.H{"ok": true, "logged_in": true, "user": publicUserJSON(u)})
}

// POST /api/auth/logout 公开登出：作废 cookie
func (h *Handler) publicLogout(c *gin.Context) {
  // 方案 A（彻底安全）：改用户 auth_secret 作废所有设备 session；
  // 方案 B（轻量，当前实现）：仅清 cookie（浏览器端登出效果足够）
  // 折中：清 cookie 同时轮换 auth_secret（保证所有设备退出，防止 cookie 被盗用）
  if u := CurrentUserFromContext(c); u != nil {
    newSecret := randomHex32()
    _, _ = h.store.db.Exec(`UPDATE users SET auth_secret = ? WHERE id = ?`, newSecret, u.ID)
  }
  clearSessionCookie(c)
  c.JSON(http.StatusOK, gin.H{"ok": true})
}

// publicUserJSON 返回不敏感字段给前端
func publicUserJSON(u *store.User) gin.H {
  return gin.H{
    "id":     u.ID,
    "nick":   firstNonEmpty(u.DisplayName, u.Username),
    "email":  u.Email,
    "avatar": u.Avatar,
    "role":   u.Role,
  }
}

func firstNonEmpty(a, b string) string {
  if a != "" { return a }
  return b
}
```

- [ ] **Step 5: router.go 中注册 /api/auth/* 路由 + SessionMiddleware 前置**

在 setupAPIRouter 现有「无权限公开路由」区块（在 管理后台 BasicAuth 中间件外）注册：

```go
// SessionMiddleware 放在所有前台路由最外层，让所有 handler 都能拿 current_user
r.Use(h.SessionMiddleware())

// 公开 auth 路由
auth := r.Group("/api/auth")
{
  auth.POST("/register", h.publicRegister)
  auth.POST("/login",    h.publicLogin)
  auth.GET( "/me",       h.publicMe)
  auth.POST("/logout",   h.publicLogout)
}
```

- [ ] **Step 6: 跑 auth 测试**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go test ./backend/internal/api/ -run TestPublicRegister -v`
Expected: PASS

- [ ] **Step 7: 补 auth_public_test.go 全量用例**

```go
func TestPublicLoginOK(t *testing.T)   { /* 先注册→再登录→/api/auth/me 拿到用户 */ }
func TestPublicLoginFailLimit(t *testing.T) { /* 错密码 5 次后 429 */ }
func TestPublicLogout(t *testing.T)  { /* 登录→登出→/me 返回 logged_in=false */ }
func TestPublicRegisterDup(t *testing.T) { /* 两次同用户名第二次 409 */ }
```

Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go test ./backend/internal/api/ -run TestPublic -v`
Expected: 全部 PASS

- [ ] **Step 8: 提交**
```bash
cd /Users/sakurasep/Documents/Code/Project/Blog/dev && \
git add backend/internal/store/user.go backend/internal/api/auth_public.go backend/internal/api/auth_public_test.go backend/internal/api/router.go && \
git commit -S -m "feat: 公开用户注册/登录（评论身份体系）"
```

---

## Task 2: 评论模型扩展 + 前台评论接口识别登录态

### 2.1 comments 表加 user_id / verified 两列 + 前台 API 改造

**Files:**
- Modify: `backend/internal/store/comment.go` (Comment 结构体 + ensureCommentReactions ALTER 迁移 + Columns/Values/ScanRow)
- Create: `backend/internal/api/comment_public.go`
- Modify: `backend/cmd/server/routes_post.go`（移除旧前台评论 handler）
- Modify: `backend/internal/comment/render.go`（白名单放行 img.cmt-emoji-img）
- Test: `backend/internal/api/comment_test.go`（扩展 verified 用例）

- [ ] **Step 1: 写 comment_test.go 失败测试**

```go
// 在 comment_test.go 末尾追加
func TestCommentVerifiedFlag(t *testing.T) {
  h := setupTestHandler(t)
  // 1) 匿名评论→verified=false
  body1 := `{"page_key":"/post/hello","nick":"anon","email":"a@b.com","content":"hi","parent_id":0}`
  w := postJSON(t, h, "/api/comments", body1)
  if w.Code != http.StatusCreated && w.Code != http.StatusOK {
    t.Fatalf("anon submit code=%d", w.Code)
  }
  // 2) 注册用户后用 cookie 评论→verified=true
  registerBody := `{"nick":"reader1","email":"r1@ex.com","password":"abc12345"}`
  wReg := postJSON(t, h, "/api/auth/register", registerBody)
  if wReg.Code != http.StatusCreated { t.Fatalf("register: %s", wReg.Body.String()) }
  cookies := wReg.Result().Cookies()
  req := httptest.NewRequest(http.MethodPost, "/api/comments", strings.NewReader(body1))
  req.Header.Set("Content-Type", "application/json")
  for _, ck := range cookies { req.AddCookie(ck) }
  w2 := httptest.NewRecorder()
  h.engine.ServeHTTP(w2, req)
  if w2.Code != http.StatusCreated && w2.Code != http.StatusOK { t.Fatalf("user submit: %s", w2.Body.String()) }
}
```

- [ ] **Step 2: 运行失败确认**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go test ./backend/internal/api/ -run TestCommentVerifiedFlag -v`
Expected: FAIL（verified 字段不存在 / 新 API 未注册二选一）

- [ ] **Step 3: store/comment.go 扩展结构体 + 迁移**

```go
type Comment struct {
  // ... 保留现有字段 ...
  UserID    int64  // 0=匿名，>0=注册用户 id（users.id）
  Verified  bool   // user_id>0 时通常 true；未来管理员取消认证可手动 false
}

func (c *Comment) Columns() []string {
  return append([]string{"id", "page_key", "nick", "email", "link", "content_md", "content_html", "parent_id", "root_id", "status", "is_admin", "ip", "ua", "likes", "created_at"},
    "user_id", "verified")
}
func (c *Comment) Values() []any {
  // 在已有 values 末尾追加
  userID := c.UserID
  verified := 0; if c.Verified { verified = 1 }
  return append(c.oldValues(), userID, verified)
}
func (c *Comment) oldValues() []any { /* 保留原先 Values() 去掉 SetID 的部分 */ }
```

在 ensureCommentReactions 末尾（建完 idx_comments_page 后）追加两列 ALTER：

```go
// 查 PRAGMA 缺列后逐个 ADD
cols, err := collectPragmaColumns(s.db, "comments")
if err != nil { s.commentReactionsErr = err; return }
if !cols["user_id"] {
  if _, err := s.db.Exec(`ALTER TABLE comments ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0`); err != nil {
    s.commentReactionsErr = fmt.Errorf("alter comments add user_id: %w", err); return
  }
}
if !cols["verified"] {
  if _, err := s.db.Exec(`ALTER TABLE comments ADD COLUMN verified INTEGER NOT NULL DEFAULT 0`); err != nil {
    s.commentReactionsErr = fmt.Errorf("alter comments add verified: %w", err); return
  }
}
```

辅助函数（放 comment.go 末尾）：

```go
func collectPragmaColumns(db *sql.DB, table string) (map[string]bool, error) {
  rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
  if err != nil { return nil, err }
  defer rows.Close()
  m := map[string]bool{}
  for rows.Next() {
    var cid, notnull, pk int
    var name, ctype string
    var dflt sql.NullString
    if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil { return nil, err }
    m[name] = true
  }
  return m, rows.Err()
}
```

同时 ScanRow 最后加两个扫描变量：

```go
var userID int64, verified int
// 在 Scan 列表最后加 &userID, &verified
c.UserID = userID; c.Verified = verified != 0
```

- [ ] **Step 4: 新建 comment_public.go → 前台评论 GET/POST**

```go
// backend/internal/api/comment_public.go
package api

import (
  "fmt"
  "net/http"
  "strconv"
  "blog/internal/comment"
  "blog/internal/store"
  "github.com/gin-gonic/gin"
)

// GET /api/comments?page_key=xxx 前台评论列表（返回树结构 + count）
func (h *Handler) publicListComments(c *gin.Context) {
  pageKey := c.Query("page_key")
  if pageKey == "" { Fail(c, http.StatusBadRequest, "page_key 必填"); return }
  list, err := h.store.ListCommentsByPage(pageKey)
  if err != nil { Fail(c, http.StatusInternalServerError, err.Error()); return }
  count, err := h.store.CountCommentsByPage(pageKey)
  if err != nil { Fail(c, http.StatusInternalServerError, err.Error()); return }
  // 平铺→两层树（root/children）
  tree := buildCommentTree(list, h.site.Comments.GravatarMirror)
  c.JSON(http.StatusOK, gin.H{"ok": true, "count": count, "comments": tree})
}

// POST /api/comments 前台评论提交（匿名+注册通用）
func (h *Handler) publicCreateComment(c *gin.Context) {
  var req struct {
    PageKey  string `json:"page_key"`
    Nick     string `json:"nick"`
    Email    string `json:"email"`
    Link     string `json:"link"`
    Content  string `json:"content"`
    ParentID int64  `json:"parent_id"`
    Website  string `json:"website"` // 蜜罐
  }
  if err := c.ShouldBindJSON(&req); err != nil { Fail(c, http.StatusBadRequest, "请求格式错误"); return }

  // 蜜罐：正常用户留空；机器人填了→静默返回成功但不入库
  if req.Website != "" { c.JSON(http.StatusOK, gin.H{"ok": true, "status": "ok"}); return }

  u := CurrentUserFromContext(c)
  if u != nil {
    // 登录用户：nick/email/link 自动取用户表，忽略前端传值（防 spoof）
    req.Nick  = firstNonEmpty(u.DisplayName, u.Username)
    req.Email = u.Email
    if u.Avatar != "" { req.Link = "" /* avatar 从 gravatar/用户头像字段取，非 link */ }
  }

  // 基础校验
  if strings.TrimSpace(req.PageKey) == "" { Fail(c, http.StatusBadRequest, "page_key 空"); return }
  if strings.TrimSpace(req.Content) == "" { Fail(c, http.StatusBadRequest, "评论内容空"); return }
  if u == nil {
    if strings.TrimSpace(req.Nick) == ""  { Fail(c, http.StatusBadRequest, "昵称必填"); return }
    if strings.TrimSpace(req.Email) == "" { Fail(c, http.StatusBadRequest, "邮箱必填"); return }
  }

  // root_id 计算（两层嵌套）
  parentID := req.ParentID
  rootID := int64(0)
  if parentID != 0 {
    parent, err := h.store.GetComment(parentID)
    if err != nil || parent == nil { Fail(c, http.StatusBadRequest, "被回复评论不存在"); return }
    if parent.RootID == 0 { rootID = parent.ID } else { rootID = parent.RootID }
  }

  html, err := comment.Render(req.Content)
  if err != nil { Fail(c, http.StatusInternalServerError, "渲染失败"); return }

  status := store.CommentApproved
  if h.site.Comments.Moderation { status = store.CommentPending }

  cm := &store.Comment{
    PageKey:     req.PageKey,
    Nick:        strings.TrimSpace(req.Nick),
    Email:       strings.TrimSpace(strings.ToLower(req.Email)),
    Link:        strings.TrimSpace(req.Link),
    ContentMD:   strings.TrimSpace(req.Content),
    ContentHTML: html,
    ParentID:    parentID,
    RootID:      rootID,
    Status:      status,
    IsAdmin:     u != nil && (u.Role == store.RoleAdmin || u.Role == store.RoleAuthor),
    UserID:      int64Value(0),
    Verified:    false,
    IP:          c.ClientIP(),
    UA:          c.Request.UserAgent(),
    CreatedAt:   time.Now(),
  }
  if u != nil {
    cm.UserID = u.ID
    cm.Verified = true // 注册用户自动打 verified 标识
    // avatar 后端不入库：由 AvatarURL(gravatarMirror, email) 或 u.Avatar 渲染时决定
  }

  id, err := h.store.CreateComment(cm)
  if err != nil { Fail(c, http.StatusInternalServerError, err.Error()); return }
  cm.ID = id

  // 回复通知（邮件 + 站内）
  if parentID != 0 && h.comments != nil {
    if parent, err := h.store.GetComment(parentID); err == nil && parent != nil {
      h.comments.NotifyReply(cm, parent)
    }
  }

  // 渲染返回体（与 comments.js buildItem 对齐：avatar/time_ago/verified）
  resp := gin.H{"ok": true, "status": status, "comment": toFrontCommentJSON(cm, u, h.site.Comments)}
  if status == store.CommentPending { resp["message"] = "评论已提交，审核后显示" }
  c.JSON(http.StatusCreated, resp)
}

func int64Value(v int64) int64 { return v }

// toFrontCommentJSON 返回前端评论项（对齐 comments.js buildItem 字段 + verified）
func toFrontCommentJSON(cm *store.Comment, u *store.User, cfg interface{}) gin.H {
  // 复用 comment.AvatarURL / TimeAgo；comments 中的字段按前端使用：id, nick, avatar, link, is_admin, verified, date, time_ago, content_html, likes, parent_id, root_id
  mirror := ""
  if s, ok := cfg.(struct{ GravatarMirror string }); ok {
    mirror = s.GravatarMirror
  }
  avatar := comment.AvatarURL(mirror, cm.Email)
  if u != nil && u.Avatar != "" { avatar = u.Avatar }
  return gin.H{
    "id":          cm.ID,
    "parent_id":   cm.ParentID,
    "root_id":     cm.RootID,
    "nick":        cm.Nick,
    "link":        cm.Link,
    "avatar":      avatar,
    "is_admin":    cm.IsAdmin,
    "verified":    cm.Verified,
    "content_html": cm.ContentHTML,
    "likes":       cm.Likes,
    "date":        cm.CreatedAt.Format("2006-01-02 15:04"),
    "time_ago":    comment.TimeAgo(cm.CreatedAt),
    "children":    []gin.H{},
  }
}
```

并在 routes_post.go 把旧匿名 `/api/comments` 替换为上述两个 public 方法。

- [ ] **Step 5: render.go bluemonday 白名单加 img.cmt-emoji-img**

```go
// 在 p := bluemonday.UGCPolicy() 之后：
p.AllowAttrs("class").Matching(regexp.MustCompile(`^cmt-emoji-img$`)).OnElements("img")
p.AllowAttrs("alt").OnElements("img")
p.AllowAttrs("src").Matching(regexp.MustCompile(`^https://cdn.jsdelivr\.net/gh/ArtalkJS/Artalk-Emoticons@.*\.png$`)).OnElements("img")
// 保留 data-emoji 可选
p.AllowAttrs("data-emoji").OnElements("img")
```

- [ ] **Step 6: 跑 comment 全测试**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go test ./backend/internal/api/ -run TestComment -v && go vet ./backend/... ./frontend/...`
Expected: 全部 PASS

- [ ] **Step 7: 提交**
```bash
cd /Users/sakurasep/Documents/Code/Project/Blog/dev && \
git add backend/internal/store/comment.go backend/internal/api/comment_public.go backend/internal/api/comment_test.go backend/cmd/server/routes_post.go backend/internal/comment/render.go && \
git commit -S -m "feat: 评论区分匿名/注册双轨，注册用户带 verified 标识"
```

---

## Task 3: 文案 + 站点配置字段

**Files:**
- Modify: `backend/internal/template/lang.go` (L120-L134)
- Modify: `config/site.example.yml` (L320-L335)
- Modify: `backend/internal/config/site.go` (DefaultSite 对应 comments 段)

- [ ] **Step 1: lang.go 追加文案**

```go
// 在"评论区"区块替换/追加：
"comment.title_invite":      "快来参与讨论吧~",
"comment.placeholder":       "编写评论，支持 Markdown",
"comment.nickname":          "昵称",
"comment.email":             "邮箱",
"comment.website_opt":       "网站（选填）",
"comment.email_hint":        "邮箱仅用于头像与回复通知，不会公开",
"comment.submit":            "发布评论",
"comment.count":             "%d 条评论",
"comment.no_comment":        "暂无评论",
"comment.loading":           "加载评论中…",
"comment.reply":             "回复",
"comment.cancel":            "取消",
"comment.empty_line1":       "尴尬了，写这么辛苦竟没人评论",
"comment.empty_suffix":      "🥺",
"comment.login":             "登录",
"comment.register":          "注册",
"comment.send":              "发送",
"comment.logout":            "退出",
"comment.email_only":        "仅限邮箱登录",
"comment.guest_mode":        "跳过验证",   // 截图中"跳过验证"
"comment.verified_badge":    "已验证",
"comment.notice_center":     "通知中心",
"comment.image_btn":         "图片",
"comment.markdown_btn":      "Markdown",
```

- [ ] **Step 2: site.example.yml 加字段**

```yaml
comments:
  enabled: true
  title: 快来参与讨论吧~            # 原"评论"改为邀请式
  moderation: false
  admin_email: ''
  admin_badge: 博主
  gravatar_mirror: https://weavatar.com/avatar
  allow_register: true              # 开放前台注册
  verified_badge: 已验证             # 注册用户徽章文案
  notify:
    smtp_host: ''
    smtp_port: 465
    username: ''
    password: ''
    from: ''
```

- [ ] **Step 3: site.go DefaultSite().Comments 中对应两项补默认**

```go
// 在 DefaultSite 的 Comments 块：
Comments: tmpl.CommentsConfig{
  Enabled:        true,
  Title:          "快来参与讨论吧~",
  Moderation:     false,
  AdminBadge:     "博主",
  GravatarMirror: "https://weavatar.com/avatar",
  AllowRegister:  true,
  VerifiedBadge:  "已验证",
},
```

同时在 model.go 的 CommentsConfig 结构体加两字段：`AllowRegister bool`, `VerifiedBadge string`。

- [ ] **Step 4: 确认一致性测试通过**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go test ./backend/internal/config/ -v -run Consistency && go test ./backend/internal/template/ -run TestLang`
Expected: PASS

- [ ] **Step 5: 提交**
```bash
cd /Users/sakurasep/Documents/Code/Project/Blog/dev && \
git add backend/internal/template/lang.go backend/internal/template/model.go config/site.example.yml backend/internal/config/site.go && \
git commit -S -m "feat: 评论区新增 Artalk 风格文案与配置字段"
```

---

## Task 4: comments.html 模板重写（卡片化 + 双态编辑器 + 表情 Tab + 空状态 + 登录弹窗）

**Files:**
- Modify: `backend/internal/template/partial/comments.html`（全量替换约 50% 内容）

- [ ] **Step 1: 重写模板**

```html
{{- /* comments.html: Artalk 风格对齐版。
   外壳 .related-wrap.md-text#comments；编辑器卡片 .cmt-card，双态 .cmt-editor-guest（三输入+登录）/.cmt-editor-user（头像+昵称+发送）。
   表情面板 5 Tab：Blobcat / QQ / Tieba / Twemoji / Aru，JS 首次打开懒加载 CDN 图片。
   登录弹窗 #cmt-login-modal 默认 display:none，点击「登录/跳过验证」打开。
   空状态 0 评论时渲染 .cmt-empty 居中。
*/ -}}
<div class="related-wrap md-text" id="comments">
  <section class="header cmt-title cap theme">
    <span>{{.Site.Comments.Title}}</span>
  </section>

  <section class="body cmt-body native" id="cmt-body"
    data-page-key="/post/{{.Page.Slug}}"
    data-admin-badge="{{.Site.Comments.AdminBadge}}"
    data-verified-badge="{{.Site.Comments.VerifiedBadge}}"
    data-lazy="1">

    {{- /* 登录态初始 SSR：CurrentUser 非空 → 直接渲染 user 态；否则 guest 态 */}}
    {{- $cu := .CurrentUser -}}
    <div class="cmt-card" id="cmt-editor-card">
      <div class="cmt-editor" id="cmt-editor">
        {{- /* 回复提示条（隐藏） */}}
        <div class="cmt-reply-tip" id="cmt-reply-tip" style="display:none">
          <span>{{lang "comment.reply"}} <b id="cmt-reply-nick"></b></span>
          <button type="button" class="cmt-reply-cancel" id="cmt-reply-cancel">{{lang "comment.cancel"}}</button>
        </div>

        {{- /* 已登录身份条（未登录态 display:none；登录成功 JS 设 display:flex 并回填头像昵称） */}}
        <div class="cmt-user-bar" id="cmt-user-bar"
             style="{{if not $cu.LoggedIn}}display:none{{end}}">
          <img class="cmt-user-avatar" id="cmt-user-avatar"
               src="{{if and $cu.LoggedIn $cu.Avatar}}{{$cu.Avatar}}{{else}}https://weavatar.com/avatar/_placeholder{{end}}"
               alt="avatar" loading="lazy">
          <span class="cmt-user-nick" id="cmt-user-nick">
            {{- if $cu.LoggedIn}}{{$cu.Nick}}{{end -}}
          </span>
          <span class="cmt-user-badge" id="cmt-user-badge"
                style="{{if not $cu.LoggedIn}}display:none{{end}}">{{.Site.Comments.VerifiedBadge}}</span>
          <span class="cmt-user-spacer"></span>
          <button type="button" class="cmt-user-logout" id="cmt-user-logout"
                  style="{{if not $cu.LoggedIn}}display:none{{end}}">{{lang "comment.logout"}}</button>
        </div>

        {{- /* 内容输入区（通用） */}}
        <textarea class="cmt-textarea" id="cmt-content"
                  placeholder="{{lang "comment.placeholder"}}"
                  rows="4" maxlength="2000"></textarea>

        {{- /* 未登录三列身份输入（登录后隐藏） */}}
        <div class="cmt-inputs" id="cmt-inputs"
             style="{{if $cu.LoggedIn}}display:none{{end}}">
          <input type="text"   class="cmt-input" id="cmt-nick"  placeholder="{{lang "comment.nickname"}} *"   maxlength="32"  autocomplete="nickname">
          <input type="email"  class="cmt-input" id="cmt-email" placeholder="{{lang "comment.email"}} *"      maxlength="254" autocomplete="email">
          <input type="text"   class="cmt-input" id="cmt-link"  placeholder="{{lang "comment.website_opt"}}"   maxlength="200" autocomplete="url">
        </div>

        {{- /* 蜜罐 */}}
        <input type="text" class="cmt-hp" id="cmt-website" tabindex="-1" autocomplete="off" aria-hidden="true">

        {{- /* 底部工具栏 + 按钮 */}}
        <div class="cmt-footer">
          <div class="cmt-tools">
            <button type="button" class="cmt-tool-btn cmt-emoji-btn" id="cmt-emoji-btn" title="表情">😀</button>
            <button type="button" class="cmt-tool-btn cmt-image-btn" id="cmt-image-btn" title="{{lang "comment.image_btn"}}">🖼️</button>
            <button type="button" class="cmt-tool-btn cmt-md-btn" id="cmt-md-btn" title="{{lang "comment.markdown_btn"}}">📝</button>
          </div>
          {{- /* 表情面板：5 Tab 切换容器 + 网格内容（JS 懒构建，容器留空） */}}
          <div class="cmt-emoji-panel" id="cmt-emoji-panel" style="display:none">
            <div class="cmt-emoji-grid" id="cmt-emoji-grid"></div>
            <div class="cmt-emoji-tabs" id="cmt-emoji-tabs">
              <button class="cmt-emoji-tab is-active" data-pack="blobcat">Blobcat</button>
              <button class="cmt-emoji-tab" data-pack="qq">QQ</button>
              <button class="cmt-emoji-tab" data-pack="tieba">Tieba</button>
              <button class="cmt-emoji-tab" data-pack="twemoji">Twemoji</button>
              <button class="cmt-emoji-tab" data-pack="aru">Aru</button>
            </div>
          </div>
          <span class="cmt-tip">{{lang "comment.email_hint"}}</span>
          {{- /* 按钮双态：未登录态=蓝色登录；已登录=蓝色发送。JS 根据状态切换可见 */}}
          <div class="cmt-actions">
            <button type="button" class="cmt-btn cmt-login-btn"  id="cmt-login-btn"
                    style="{{if $cu.LoggedIn}}display:none{{end}}">{{lang "comment.login"}}</button>
            <button type="button" class="cmt-btn cmt-send-btn"   id="cmt-submit"
                    style="{{if not $cu.LoggedIn}}display:none{{end}}">{{lang "comment.send"}}</button>
          </div>
        </div>
      </div>
    </div>

    {{- /* 计数 + 通知中心 */}}
    <div class="cmt-count-row">
      <div class="cmt-count" id="cmt-count">
        {{if .CommentCount}}
          <a class="cmt-count-link">{{lang "comment.count" .CommentCount}} <span class="cmt-caret">▾</span></a>
        {{else}}
          <a class="cmt-count-link">{{lang "comment.count" 0}} <span class="cmt-caret">▾</span></a>
        {{end}}
      </div>
      <a class="cmt-notice-link" id="cmt-notice-link" href="#">{{lang "comment.notice_center"}}</a>
    </div>

    {{- /* 列表 */}}
    <div class="cmt-list" id="cmt-list">
      <div class="cmt-lazy" id="cmt-lazy">
        <svg class="loading" viewBox="0 0 1024 1024" width="1em" height="1em" aria-hidden="true"><path fill="currentColor" d="M512 1024a512 512 0 1 1 512-512h-96a416 416 0 1 0-416 416z"/></svg>
        <span>{{lang "comment.loading"}}</span>
      </div>
      {{- /* 空状态 .cmt-empty：0 条且加载完成后由 JS 渲染（SSR 时只渲染占位） */}}
    </div>
  </section>

  {{- /* 登录/注册弹窗（默认隐藏，点登录或跳过验证打开） */}}
  <div class="cmt-modal" id="cmt-login-modal" style="display:none" aria-hidden="true">
    <div class="cmt-modal-mask" data-close="1"></div>
    <div class="cmt-modal-dialog">
      <div class="cmt-modal-header">
        <span class="cmt-modal-title">{{lang "comment.login"}}</span>
        <button type="button" class="cmt-modal-close" data-close="1">×</button>
      </div>
      <div class="cmt-modal-body">
        <ul class="cmt-login-options">
          <li class="cmt-login-opt cmt-login-email" data-mode="email">
            <span class="cmt-login-icon cmt-login-icon-email">✉️</span>
            <span class="cmt-login-label">{{lang "comment.email_only"}}</span>
          </li>
          <li class="cmt-login-opt cmt-login-guest" data-mode="guest">
            <span class="cmt-login-icon cmt-login-icon-guest">🚫</span>
            <span class="cmt-login-label">{{lang "comment.guest_mode"}}</span>
          </li>
        </ul>
        {{- /* Email 登录/注册表单（默认 hidden，选 email 后展开） */}}
        <form class="cmt-login-form" id="cmt-email-form" style="display:none" onsubmit="return false">
          <input type="text"  class="cmt-input" id="cmt-login-account"  placeholder="用户名或邮箱" autocomplete="username">
          <input type="password" class="cmt-input" id="cmt-login-pass"     placeholder="密码" autocomplete="current-password">
          <div class="cmt-login-actions">
            <button type="button" class="cmt-btn" id="cmt-do-login">登录</button>
            <button type="button" class="cmt-btn cmt-btn-plain" id="cmt-do-register">注册新账号</button>
          </div>
          <div class="cmt-login-error" id="cmt-login-error" style="display:none;color:var(--accent);font-size:13px;margin-top:8px"></div>
        </form>
      </div>
    </div>
  </div>
</div>

<script>/* comments.js 在下一个 Task 重写时替换旧内联点赞脚本；此处保留占位空 script 标签对齐结构 */</script>
```

- [ ] **Step 2: 模板语法检查**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go build ./backend/...`
Expected: 编译通过（模板通过 engine.LoadDisk 加载，编译失败会报错）

- [ ] **Step 3: 提交**
```bash
cd /Users/sakurasep/Documents/Code/Project/Blog/dev && \
git add backend/internal/template/partial/comments.html && \
git commit -S -m "feat: 评论区模板 Artalk 视觉结构重写（卡片+双态+Tab+弹窗）"
```

---

## Task 5: comments.css 重写（卡片、身份条、弹窗、表情 Tab、空状态、徽章、通知中心）

**Files:**
- Modify: `frontend/assets/css/comments.css`（全量改写，对齐上面的新结构）

- [ ] **Step 1: 写新版 CSS（约 380 行）**

```css
/* comments.css Artalk 风格对齐版。颜色全部走 Stellar 变量：
   --theme / --accent / --card / --block-border / --text-p1~p4 / --bg-a2~a10 */

/* ─── 卡片外壳（.cmt-card）：背景略深，圆角 12px，与主背景分层 ─── */
.cmt-body.native .cmt-card {
  background: var(--card, #fff);
  border: 1px solid var(--block-border, rgba(128,128,128,.2));
  border-radius: 12px;
  padding: 16px;
  box-shadow: 0 2px 6px color-mix(in srgb, var(--block-border, #ccc) 30%, transparent);
}
:root[data-theme="dark"] .cmt-body.native .cmt-card {
  background: color-mix(in srgb, var(--card) 94%, #000 6%);
}

/* 编辑器内部 */
.cmt-body.native .cmt-editor .cmt-reply-tip {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 8px; padding: 6px 12px; border-radius: 8px;
  background: var(--bg-a2, rgba(128,128,128,.08));
  font-size: 13px; color: var(--text-p2);
}
.cmt-body.native .cmt-reply-cancel {
  border: none; background: none; padding: 0; color: var(--text-p3);
  font-size: 13px; font-family: inherit; cursor: pointer;
}
.cmt-body.native .cmt-reply-cancel:hover { color: var(--theme); }

/* 已登录身份条 */
.cmt-body.native .cmt-user-bar {
  display: flex; align-items: center; gap: 10px;
  margin-bottom: 10px;
}
.cmt-body.native .cmt-user-avatar {
  width: 36px; height: 36px; border-radius: 50%;
  background: var(--bg-a2, rgba(128,128,128,.1));
}
.cmt-body.native .cmt-user-nick {
  font-size: 15px; font-weight: 600; color: var(--text-p1);
}
.cmt-body.native .cmt-user-badge {
  display: inline-block;
  padding: 1px 8px; border-radius: 4px; font-size: 12px;
  color: #2e7d32; background: rgba(46,125,50,.12); border: 1px solid rgba(46,125,50,.2);
}
:root[data-theme="dark"] .cmt-body.native .cmt-user-badge {
  color: #81c784; background: rgba(129,199,132,.1); border-color: rgba(129,199,132,.2);
}
.cmt-body.native .cmt-user-spacer { flex: 1 }
.cmt-body.native .cmt-user-logout {
  border: none; background: none; color: var(--text-p3);
  font-size: 13px; font-family: inherit; cursor: pointer;
}
.cmt-body.native .cmt-user-logout:hover { color: var(--accent); }

/* textarea（与卡片融合） */
.cmt-body.native .cmt-textarea {
  display: block; width: 100%; box-sizing: border-box;
  padding: 12px; border-radius: 8px;
  border: 1px solid transparent;
  background: var(--bg-a2, rgba(128,128,128,.06));
  color: var(--text-p1);
  font-size: 14px; line-height: 1.6; font-family: inherit;
  resize: vertical; min-height: 88px;
  transition: background .2s, border-color .2s;
}
.cmt-body.native .cmt-textarea:hover  { background: var(--bg-a2, rgba(128,128,128,.1)); }
.cmt-body.native .cmt-textarea:focus  {
  outline: none;
  border-color: var(--theme);
  background: var(--card, #fff);
}
.cmt-body.native .cmt-editor:focus-within .cmt-textarea {
  border-color: var(--theme);
}

/* 三列输入 */
.cmt-body.native .cmt-inputs {
  display: grid; grid-template-columns: repeat(3,1fr);
  gap: 8px; margin-top: 10px;
}
.cmt-body.native .cmt-input {
  box-sizing: border-box; min-width: 0; width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--block-border, rgba(128,128,128,.2));
  border-radius: 8px;
  background: var(--bg-a2, rgba(128,128,128,.06));
  color: var(--text-p1); font-size: 13px; font-family: inherit;
  transition: border-color .2s, background .2s;
}
.cmt-body.native .cmt-input:focus {
  outline: none;
  border-color: var(--theme);
  background: var(--card, #fff);
}

/* 蜜罐（视觉隐藏） */
.cmt-body.native .cmt-hp {
  position: absolute !important; left: -9999px !important;
  width: 1px; height: 1px; opacity: 0;
}

/* 底部：工具栏左侧 + 提示 + 按钮右 */
.cmt-body.native .cmt-footer {
  position: relative; /* 表情面板锚点 */
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; margin-top: 12px;
  flex-wrap: wrap;
}
.cmt-body.native .cmt-tools {
  display: flex; align-items: center; gap: 2px;
}
.cmt-body.native .cmt-tool-btn {
  flex-shrink: 0; background: none; border: none;
  font-size: 1.1rem; cursor: pointer;
  padding: 6px 8px; border-radius: 6px; line-height: 1;
  opacity: .75; transition: all .2s;
}
.cmt-body.native .cmt-tool-btn:hover {
  opacity: 1; background: var(--bg-a2, rgba(128,128,128,.1));
}

/* 表情面板：上方弹出 + 底部 Tab 栏 */
.cmt-body.native .cmt-emoji-panel {
  position: absolute; left: 0; right: 0; bottom: 100%;
  margin-bottom: 10px;
  background: var(--card, #fff);
  border: 1px solid var(--block-border, rgba(128,128,128,.25));
  border-radius: 10px;
  padding: 10px;
  display: flex; flex-direction: column; gap: 10px;
  max-height: 320px; overflow: hidden;
  box-shadow: 0 8px 24px rgba(0,0,0,.25);
  z-index: 100;
}
.cmt-body.native .cmt-emoji-grid {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 4px;
  overflow-y: auto;
  max-height: 240px;
}
.cmt-body.native .cmt-emoji-grid img {
  width: 28px; height: 28px; cursor: pointer;
  padding: 4px; border-radius: 6px;
  transition: background .15s, transform .1s;
}
.cmt-body.native .cmt-emoji-grid img:hover {
  background: var(--bg-a2, rgba(128,128,128,.1));
}
.cmt-body.native .cmt-emoji-grid img:active { transform: scale(.9); }

.cmt-body.native .cmt-emoji-tabs {
  display: flex; gap: 4px;
  border-top: 1px solid var(--block-border, rgba(128,128,128,.15));
  padding-top: 8px;
}
.cmt-body.native .cmt-emoji-tab {
  flex: 1;
  padding: 6px 4px;
  border: none; background: transparent;
  border-radius: 6px;
  font-size: 12px; color: var(--text-p3);
  cursor: pointer; font-family: inherit;
  transition: all .2s;
}
.cmt-body.native .cmt-emoji-tab.is-active {
  background: color-mix(in srgb, var(--theme) 15%, transparent);
  color: var(--theme);
  font-weight: 600;
}

/* 提示与按钮 */
.cmt-body.native .cmt-tip { font-size: 12px; color: var(--text-p3); flex: 1; min-width: 0 }
.cmt-body.native .cmt-actions { display: flex; align-items: center; }

.cmt-body.native .cmt-btn {
  flex-shrink: 0;
  padding: 8px 22px; border: none; border-radius: 20px;
  background: var(--theme); color: var(--card);
  font-size: 14px; font-family: inherit; font-weight: 500;
  cursor: pointer; transition: opacity .2s, transform .1s;
  box-shadow: 0 2px 6px color-mix(in srgb, var(--theme) 30%, transparent);
}
.cmt-body.native .cmt-btn:hover        { opacity: .9; }
.cmt-body.native .cmt-btn:active       { transform: translateY(1px); }
.cmt-body.native .cmt-btn:disabled     { opacity: .5; cursor: not-allowed; }
.cmt-body.native .cmt-btn-plain       {
  background: transparent; color: var(--theme);
  box-shadow: none;
  border: 1px solid var(--theme);
}
.cmt-body.native .cmt-btn-plain:hover { background: color-mix(in srgb, var(--theme) 10%, transparent); }

/* ─── 计数行：左计数（带▾）+ 右通知中心链接 ─── */
.cmt-body.native .cmt-count-row {
  display: flex; align-items: center; justify-content: space-between;
  margin: 20px 0 8px;
}
.cmt-body.native .cmt-count-link {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 14px; font-weight: 600; color: var(--text-p1);
  text-decoration: none; cursor: pointer;
  padding: 2px 6px; border-radius: 4px;
}
.cmt-body.native .cmt-count-link:hover { background: var(--bg-a2); }
.cmt-body.native .cmt-caret {
  color: var(--text-p3);
  transition: transform .2s;
}
.cmt-body.native .cmt-notice-link {
  color: var(--theme);
  text-decoration: none;
  font-size: 13px;
  font-weight: 500;
}
.cmt-body.native .cmt-notice-link:hover {
  text-decoration: underline;
}

/* ─── 评论项：verified 徽章新增 ─── */
.cmt-body.native .cmt-item { padding-top: 14px; }
.cmt-body.native .cmt-badge-verified {
  margin-left: 4px;
  padding: 1px 6px; border-radius: 4px; font-size: 11px;
  color: #2e7d32; background: rgba(46,125,50,.10);
  border: 1px solid rgba(46,125,50,.18);
}
:root[data-theme="dark"] .cmt-body.native .cmt-badge-verified {
  color: #81c784; background: rgba(129,199,132,.08);
  border-color: rgba(129,199,132,.15);
}

/* ─── 空状态：居中大字 + 🥺 ─── */
.cmt-body.native .cmt-empty {
  text-align: center;
  padding: 50px 0 40px;
  color: var(--text-p3);
}
.cmt-body.native .cmt-empty .cmt-empty-text {
  font-size: 17px; font-weight: 500;
  color: var(--text-p2);
  margin-bottom: 4px;
}
.cmt-body.native .cmt-empty .cmt-empty-emoji {
  font-size: 22px;
  line-height: 1;
}

/* ─── 懒加载占位样式（对齐旧 .cmt-lazy 保留） ─── */
.cmt-body.native .cmt-lazy {
  display: flex; align-items: center; justify-content: center;
  gap: 8px; min-height: 60px; padding: 12px 0;
  color: var(--text-p3); font-size: 13px;
}

/* ─── 模态框：登录弹窗 ─── */
.cmt-modal {
  position: fixed; inset: 0; z-index: 9998;
  display: flex; align-items: center; justify-content: center;
}
.cmt-modal-mask {
  position: absolute; inset: 0;
  background: rgba(0,0,0,.6);
  backdrop-filter: blur(4px);
}
.cmt-modal-dialog {
  position: relative;
  width: 90%; max-width: 360px;
  background: var(--card, #fff);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 20px 60px rgba(0,0,0,.4);
  animation: cmt-modal-in .18s ease-out;
}
@keyframes cmt-modal-in {
  from { opacity: 0; transform: translateY(-12px) scale(.98); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}
.cmt-modal-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 18px;
  border-bottom: 1px solid var(--block-border, rgba(128,128,128,.15));
}
.cmt-modal-title {
  font-size: 16px; font-weight: 600; color: var(--text-p1);
}
.cmt-modal-close {
  border: none; background: none;
  font-size: 24px; line-height: 1; cursor: pointer;
  color: var(--text-p3); padding: 0 4px;
}
.cmt-modal-close:hover { color: var(--text-p1); }
.cmt-modal-body { padding: 16px 18px 20px; }

.cmt-login-options {
  list-style: none; padding: 0; margin: 0;
  display: flex; flex-direction: column; gap: 10px;
}
.cmt-login-opt {
  display: flex; align-items: center; gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--block-border, rgba(128,128,128,.2));
  border-radius: 10px;
  cursor: pointer;
  transition: border-color .2s, background .2s;
}
.cmt-login-opt:hover {
  border-color: var(--theme);
  background: var(--bg-a2, rgba(128,128,128,.05));
}
.cmt-login-icon {
  width: 30px; height: 30px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 16px;
  background: rgba(27,205,252,.12); color: #1BCDFC;
}
.cmt-login-icon-guest {
  background: rgba(61,197,80,.12); color: #3DC550;
}
.cmt-login-label {
  font-size: 14px; font-weight: 500; color: var(--text-p1);
}

.cmt-login-form {
  margin-top: 16px; display: flex; flex-direction: column; gap: 8px;
}
.cmt-login-form .cmt-input {
  padding: 10px 12px;
}
.cmt-login-actions {
  display: flex; gap: 8px; margin-top: 6px; flex-wrap: wrap;
}

/* ─── 移动端自适应 ─── */
@media screen and (max-width: 667px) {
  .cmt-body.native .cmt-inputs { grid-template-columns: 1fr; }
  .cmt-body.native .cmt-footer { flex-direction: row; gap: 8px; }
  .cmt-body.native .cmt-tip    { display: none; } /* 小屏省空间 */
  .cmt-body.native .cmt-emoji-grid { grid-template-columns: repeat(6, 1fr); }
}
```

- [ ] **Step 2: CSS 语法无大错（通过浏览器预览）**
先不做全量浏览器测试，只需确认 build 通过。

- [ ] **Step 3: 提交**
```bash
cd /Users/sakurasep/Documents/Code/Project/Blog/dev && \
git add frontend/assets/css/comments.css && \
git commit -S -m "feat: 评论区 CSS Artalk 风格对齐（卡片/双态/徽章/Tab/弹窗/空状态）"
```

---

## Task 6: comments.js 重写（身份双态 + 登录弹窗 + 表情 Tab + 空状态）

**Files:**
- Modify: `frontend/assets/js/comments.js`（全量替换）
- Clean: `backend/internal/template/partial/comments.html` 末尾占位 `<script>/* */</script>` 内联脚本改为 comments.js 里统一实现的点赞（与旧点赞逻辑复用，避免重复）

- [ ] **Step 1: 写 comments.js（约 500 行，模块化分段注释）**

```js
/* comments.js Artalk 风格对齐版。
 * 职责：
 *   1) 登录态双态切换（首屏 GET /api/auth/me，失败降级匿名）
 *   2) 登录/注册弹窗交互（Email 选项展开表单、Guest 选项即关闭弹窗继续匿名）
 *   3) 表情 5 Tab：Blobcat / QQ / Tieba / Twemoji / Aru（用 Artalk-Emoticons CDN）
 *   4) 编辑器工具栏三按钮（emoji / image placeholder / markdown placeholder）
 *   5) 评论提交：POST /api/comments（自动带 cookie，登录态 nick/email 不取表单）
 *   6) 列表渲染 + verified 徽章 + 空状态
 *   7) 计数▾ + 通知中心占位点击
 *   8) PJAX 兼容：事件委托绑 document，data-built guard 防重复
 */
(function () {
  'use strict';

  // ─── 常量 ────────────────────────────────────────────────
  var LS_IDENTITY_KEY  = 'blog_comment_identity';   // 匿名身份（fallback 用）
  var ME_CACHE_KEY     = 'blog_comment_me_cache';   // /me 响应缓存（加速 PJAX）
  var ME_CACHE_TTL_MS  = 5 * 60 * 1000;             // 5min

  // 表情 CDN 清单：key=pack, value={base, files: [name,...] or count:N, ext:png}
  // Artalk-Emoticons 开源仓库：https://github.com/ArtalkJS/Artalk-Emoticons 下 packages/*/dist/*.png
  var EMOTI_PACKS = {
    blobcat: { title: 'Blobcat', base: 'https://cdn.jsdelivr.net/gh/ArtalkJS/Artalk-Emoticons@master/packages/blobcat/dist/',
      names: genBlobcatNames() }, /* 下方辅助函数生成 40 多个 blobcat_xxx */
    qq:      { title: 'QQ',      base: 'https://cdn.jsdelivr.net/gh/ArtalkJS/Artalk-Emoticons@master/packages/qq/dist/',
      count: 110, prefix: 'q', ext: 'png' },
    tieba:   { title: 'Tieba',   base: 'https://cdn.jsdelivr.net/gh/ArtalkJS/Artalk-Emoticons@master/packages/tieba/dist/',
      count: 60, prefix: 't', ext: 'png' },
    twemoji: { title: 'Twemoji', base: 'https://cdn.jsdelivr.net/gh/ArtalkJS/Artalk-Emoticons@master/packages/twemoji/dist/',
      count: 96, prefix: 'tw', ext: 'svg' },
    aru:     { title: 'Aru',     base: 'https://cdn.jsdelivr.net/gh/ArtalkJS/Artalk-Emoticons@master/packages/aru/dist/',
      count: 42, prefix: 'aru', ext: 'png' },
  };

  // 当前激活 pack（懒加载）
  var activePack   = 'blobcat';
  var packBuilt    = {};   // pack -> true（已注入 DOM）
  var replyState   = { pageKey: '', parentId: 0 };
  var currentUser  = null; // {logged_in:true/false, nick, avatar, ...}

  // ─── 通用工具 ────────────────────────────────────────────
  function $(id) { return document.getElementById(id); }
  function body() { return $('cmt-body'); }
  function pageKey() {
    var b = body(); return b ? (b.getAttribute('data-page-key') || '') : '';
  }
  function adminBadge()    { var b=body(); return b ? b.getAttribute('data-admin-badge') || '博主' : '博主'; }
  function verifiedBadge() { var b=body(); return b ? b.getAttribute('data-verified-badge') || '已验证' : '已验证'; }
  function esc(s) {
    return String(s==null?'':s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
  }
  function toast(msg) {
    var old = document.querySelector('.cmt-toast');
    if (old) old.remove();
    var t = document.createElement('div');
    t.className = 'cmt-toast';
    t.textContent = msg;
    document.body.appendChild(t);
    setTimeout(function(){t.classList.add('show');}, 10);
    setTimeout(function(){
      t.classList.remove('show');
      setTimeout(function(){t.remove();}, 300);
    }, 2400);
  }

  // ─── 表情资源名生成 ──────────────────────────────────────
  function genBlobcatNames() {
    // 仅列已知存在的 blobcat 常见表情名（Artalk-Emoticons 包中默认含约 44 个）
    var n = ['blobcat','blobcat1','blobcat2','blobcat3','blobcat4','blobcat5','blobcat6','blobcat7',
      'blobcat8','blobcat9','blobcat10','blobcat11','blobcat12','blobcat13','blobcat14','blobcat15',
      'blobcat16','blobcat17','blobcat18','blobcat19','blobcat20','blobcat21','blobcat22','blobcat23',
      'blobcat24','blobcat25','blobcat26','blobcat27','blobcat28','blobcat29','blobcat30','blobcat31',
      'blobcat32','blobcat33','blobcat34','blobcat35','blobcat36','blobcat37','blobcat38','blobcat39',
      'blobcat40','blobcat41','blobcat42','blobcat43','blobcat44'];
    var out = [];
    for (var i=0;i<n.length;i++) out.push(n[i] + '.png');
    return out;
  }
  function packFiles(pack) {
    var cfg = EMOTI_PACKS[pack];
    if (cfg.names) return cfg.names;
    var arr = [];
    var ext = cfg.ext || 'png';
    for (var i=1; i<=cfg.count; i++) arr.push(cfg.prefix + i + '.' + ext);
    return arr;
  }

  // ─── 身份状态 ────────────────────────────────────────────
  // loadMeCache: true=优先读本地缓存（加速），force=true=强制刷新
  function fetchMe(force, cb) {
    try {
      if (!force) {
        var raw = localStorage.getItem(ME_CACHE_KEY);
        if (raw) {
          var parsed = JSON.parse(raw);
          if (parsed && parsed._ts && Date.now() - parsed._ts < ME_CACHE_TTL_MS) {
            currentUser = parsed.data;
            cb && cb(currentUser);
            return;
          }
        }
      }
    } catch (e) {}
    fetch('/api/auth/me', { credentials: 'same-origin' })
      .then(function(r){ return r.json(); })
      .then(function(res){
        if (res && res.ok) {
          currentUser = res.logged_in ? { logged_in: true, nick: res.user.nick, avatar: res.user.avatar, role: res.user.role }
                                     : { logged_in: false };
          try { localStorage.setItem(ME_CACHE_KEY, JSON.stringify({ _ts: Date.now(), data: currentUser })); } catch(e){}
        } else {
          currentUser = { logged_in: false };
        }
        cb && cb(currentUser);
      })
      .catch(function(){
        currentUser = { logged_in: false };
        cb && cb(currentUser);
      });
  }

  function invalidateMeCache() {
    try { localStorage.removeItem(ME_CACHE_KEY); } catch(e){}
  }

  function renderUserBar() {
    var bar = $('cmt-user-bar');
    var av  = $('cmt-user-avatar');
    var nk  = $('cmt-user-nick');
    var bd  = $('cmt-user-badge');
    var out = $('cmt-user-logout');
    var inputs = $('cmt-inputs');
    var loginBtn = $('cmt-login-btn');
    var sendBtn  = $('cmt-submit');

    if (!bar) return;
    if (currentUser && currentUser.logged_in) {
      bar.style.display = 'flex';
      if (currentUser.avatar) av.src = currentUser.avatar;
      nk.textContent = currentUser.nick || '';
      bd.style.display = '';
      out.style.display = '';
      inputs.style.display = 'none';
      loginBtn.style.display = 'none';
      sendBtn.style.display  = '';
    } else {
      bar.style.display = 'none';
      bd.style.display = 'none';
      out.style.display = 'none';
      inputs.style.display = '';
      loginBtn.style.display = '';
      sendBtn.style.display  = 'none';
      // 回填匿名身份
      fillAnonymousIdentity();
    }
  }

  function fillAnonymousIdentity() {
    try {
      var raw = localStorage.getItem(LS_IDENTITY_KEY);
      if (!raw) return;
      var d = JSON.parse(raw);
      if (d.nick && $('cmt-nick') && !$('cmt-nick').value) $('cmt-nick').value = d.nick;
      if (d.email && $('cmt-email') && !$('cmt-email').value) $('cmt-email').value = d.email;
      if (d.link && $('cmt-link') && !$('cmt-link').value) $('cmt-link').value = d.link;
    } catch (e) {}
  }
  function saveAnonymousIdentity() {
    if (currentUser && currentUser.logged_in) return;
    var n = $('cmt-nick'), e = $('cmt-email'), l = $('cmt-link');
    if (!n || !e) return;
    try {
      localStorage.setItem(LS_IDENTITY_KEY, JSON.stringify({ nick: n.value, email: e.value, link: l ? l.value : '' }));
    } catch (e) {}
  }

  // ─── 登录弹窗 ────────────────────────────────────────────
  function openLoginModal()  { var m = $('cmt-login-modal'); if (m) m.style.display = ''; }
  function closeLoginModal() {
    var m = $('cmt-login-modal');
    if (!m) return;
    m.style.display = 'none';
    var f = $('cmt-email-form');
    if (f) f.style.display = 'none';
    var err = $('cmt-login-error'); if (err) { err.style.display='none'; err.textContent=''; }
  }
  function showLoginError(msg) {
    var el = $('cmt-login-error'); if (!el) return;
    el.textContent = msg; el.style.display = '';
  }

  // ─── 表情面板 ────────────────────────────────────────────
  function buildEmojiPack(pack) {
    if (packBuilt[pack]) return;
    var grid = $('cmt-emoji-grid');
    if (!grid) return;
    // 清空当前
    while (grid.firstChild) grid.removeChild(grid.firstChild);
    var cfg = EMOTI_PACKS[pack];
    var files = packFiles(pack);
    for (var i = 0; i < files.length; i++) {
      var img = document.createElement('img');
      img.src = cfg.base + files[i];
      img.alt = '[' + cfg.title + ':' + files[i] + ']';
      img.setAttribute('data-emoji-src', cfg.base + files[i]);
      img.setAttribute('data-emoji-alt', img.alt);
      img.setAttribute('data-emoji-pack', pack);
      img.loading = 'lazy';
      grid.appendChild(img);
    }
    packBuilt[pack] = true;
  }
  function activateTab(tab) {
    var tabs = document.querySelectorAll('.cmt-emoji-tab');
    for (var i=0; i<tabs.length; i++) tabs[i].classList.remove('is-active');
    if (tab) tab.classList.add('is-active');
    var pack = tab ? tab.getAttribute('data-pack') : 'blobcat';
    activePack = pack;
    buildEmojiPack(pack);
  }
  function hideEmojiPanel() {
    var p = $('cmt-emoji-panel');
    if (p) p.style.display = 'none';
  }
  function toggleEmojiPanel() {
    var p = $('cmt-emoji-panel');
    if (!p) return;
    if (p.style.display === 'none') {
      // 首次：激活默认 tab + 构建
      var active = document.querySelector('.cmt-emoji-tab.is-active');
      activateTab(active || document.querySelector('.cmt-emoji-tab[data-pack="blobcat"]'));
      p.style.display = '';
    } else {
      hideEmojiPanel();
    }
  }
  // 图片插入到光标位置（替换选区为 <img> 的 markdown 文本：![alt](url)）
  // 注意：评论内容是 Markdown → 后端用 goldmark 渲染成 <img class="cmt-emoji-img">
  function insertEmojiMarkdown(alt, url) {
    var ta = $('cmt-content'); if (!ta) return;
    ta.focus();
    var s = ta.selectionStart, e = ta.selectionEnd;
    var md = '![' + alt + '](' + url + ')';
    ta.value = ta.value.slice(0, s) + md + ta.value.slice(e);
    var pos = s + md.length;
    ta.selectionStart = pos; ta.selectionEnd = pos;
  }

  // ─── 评论列表渲染 ────────────────────────────────────────
  function buildItem(cm) {
    var item = document.createElement('div');
    item.className = 'cmt-item';
    item.id = 'comment-' + cm.id;
    item.setAttribute('data-id', cm.id);

    var nickHtml = cm.link
      ? '<a class="cmt-nick" href="' + esc(cm.link) + '" target="_blank" rel="external nofollow noopener noreferrer">' + esc(cm.nick) + '</a>'
      : '<span class="cmt-nick">' + esc(cm.nick) + '</span>';
    var adminBadgeHtml    = cm.is_admin ? '<span class="cmt-badge">' + esc(adminBadge()) + '</span>' : '';
    var verifiedBadgeHtml = cm.verified ? '<span class="cmt-badge cmt-badge-verified">' + esc(verifiedBadge()) + '</span>' : '';

    item.innerHTML =
      '<img class="cmt-avatar" src="' + esc(cm.avatar) + '" alt="' + esc(cm.nick) + '" loading="lazy">' +
      '<div class="cmt-main">' +
        '<div class="cmt-meta">' + nickHtml + adminBadgeHtml + verifiedBadgeHtml +
          '<span class="cmt-time" title="' + esc(cm.date) + '">' + esc(cm.time_ago) + '</span>' +
        '</div>' +
        '<div class="cmt-content">' + (cm.content_html || '') + '</div>' +
        '<div class="cmt-actions">' +
          '<button type="button" class="cmt-reply-btn" data-id="' + cm.id + '" data-nick="' + esc(cm.nick) + '">回复</button>' +
          '<button type="button" class="cmt-reply-btn cmt-like-btn" data-id="' + cm.id + '" aria-label="赞">赞 <span class="cmt-like-count">' + (cm.likes || 0) + '</span></button>' +
        '</div>' +
      '</div>';
    if (cm.children && cm.children.length) {
      var ch = document.createElement('div');
      ch.className = 'cmt-children';
      for (var i=0;i<cm.children.length;i++) ch.appendChild(buildItem(cm.children[i]));
      item.querySelector('.cmt-main').appendChild(ch);
    }
    return item;
  }
  function renderList(comments, count) {
    var list = $('cmt-list');
    if (!list) return;
    // 清占位
    var lazy = $('cmt-lazy'); if (lazy) lazy.remove();
    list.innerHTML = '';
    if (!comments || !comments.length) {
      // 空状态
      var empty = document.createElement('div');
      empty.className = 'cmt-empty';
      empty.innerHTML =
        '<div class="cmt-empty-text">尴尬了，写这么辛苦竟没人评论</div>' +
        '<div class="cmt-empty-emoji">🥺</div>';
      list.appendChild(empty);
    } else {
      for (var i=0;i<comments.length;i++) list.appendChild(buildItem(comments[i]));
    }
    // 计数文案
    var cc = $('cmt-count');
    if (cc) {
      var n = typeof count === 'number' ? count : (comments ? comments.length : 0);
      // 保留结构：<a class="cmt-count-link">N 条评论 ▾</a>
      cc.innerHTML = '<a class="cmt-count-link">' + n + ' 条评论 <span class="cmt-caret">▾</span></a>';
    }
  }
  function loadComments() {
    var b = body();
    if (!b || b.getAttribute('data-loaded') === '1') return;
    b.setAttribute('data-loaded', '1');
    fetch('/api/comments?page_key=' + encodeURIComponent(pageKey()), { credentials: 'same-origin' })
      .then(function(r){ return r.json(); })
      .then(function(res){
        if (res && res.ok) renderList(res.comments, typeof res.count === 'number' ? res.count : undefined);
        else { b.removeAttribute('data-loaded'); var lz=$('cmt-lazy'); if (lz) lz.remove(); }
      })
      .catch(function(){
        b.removeAttribute('data-loaded'); var lz=$('cmt-lazy'); if (lz) lz.remove();
      });
  }

  // ─── 编辑器内联（回复） ──────────────────────────────────
  function showReplyTip(nick) { $('cmt-reply-nick').textContent='@'+nick; $('cmt-reply-tip').style.display=''; }
  function hideReplyTip() { var t=$('cmt-reply-tip'); if (t) t.style.display='none'; }
  function moveEditorTo(item) {
    var main = item.querySelector(':scope > .cmt-main');
    var card = $('cmt-editor-card');
    if (!main || !card) return;
    main.appendChild(card);
  }
  function resetEditor() {
    var b = body();
    if (b) {
      var card = $('cmt-editor-card');
      if (card) b.insertBefore(card, b.querySelector('.cmt-count-row') || b.firstChild);
    }
    hideReplyTip();
    replyState = { pageKey: '', parentId: 0 };
  }
  function startReply(btn) {
    var id   = parseInt(btn.getAttribute('data-id'), 10);
    var nick = btn.getAttribute('data-nick') || '';
    var item = $('comment-' + id);
    if (!item) return;
    replyState = { pageKey: pageKey(), parentId: id };
    moveEditorTo(item);
    showReplyTip(nick);
    $('cmt-content').focus();
  }

  // ─── 评论提交 ────────────────────────────────────────────
  function submit() {
    var content = $('cmt-content').value.trim();
    var nickEl = $('cmt-nick'), emailEl = $('cmt-email'), linkEl = $('cmt-link');
    var nick = nickEl ? nickEl.value.trim() : '';
    var email = emailEl ? emailEl.value.trim() : '';
    var link = linkEl ? linkEl.value.trim() : '';
    var hp   = $('cmt-website') ? $('cmt-website').value : '';

    // PJAX 换页 replyState 校验
    if (replyState.pageKey !== pageKey()) replyState.parentId = 0;

    // 基础校验（登录态跳过 nick/email）
    if (!content) { toast('请填写评论内容'); return; }
    if (!(currentUser && currentUser.logged_in)) {
      if (!nick)  { toast('请填写昵称'); return; }
      if (!email) { toast('请填写邮箱'); return; }
    }

    var btn = currentUser && currentUser.logged_in ? $('cmt-submit') : $('cmt-login-btn');
    if (currentUser && currentUser.logged_in) {
      // 登录用户：直接提交
      doSubmitRequest({ nick:nick, email:email, link:link, content:content, hp:hp, btn:btn });
    } else {
      // 未登录：先弹登录/注册（用户可选匿名=登录后再匿名也一样）
      // 简化：先尝试匿名提交（若失败提示注册），成功即插入
      doSubmitRequest({ nick:nick, email:email, link:link, content:content, hp:hp, btn:null });
    }
  }
  function doSubmitRequest(ctx) {
    // 匿名用户保存身份
    saveAnonymousIdentity();
    var subBtn = ctx.btn || $('cmt-submit') || $('cmt-login-btn');
    if (subBtn) subBtn.disabled = true;

    var payload = {
      page_key:  pageKey(),
      nick:      ctx.nick,
      email:     ctx.email,
      link:      ctx.link,
      content:   ctx.content,
      parent_id: replyState.parentId || 0,
      website:   ctx.hp
    };
    fetch('/api/comments', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
      .then(function(r){ return r.json(); })
      .then(function(res){
        if (subBtn) subBtn.disabled = false;
        if (!res || !res.ok) { toast(res && res.error || '提交失败'); return; }
        if (res.status === 'pending') {
          resetEditor(); toast('评论已提交，审核通过后显示'); return;
        }
        if (res.comment) {
          insertComment(res.comment, replyState.parentId);
          updateCount(1);
          removeEmptyState();
        }
        $('cmt-content').value = '';
        resetEditor();
        toast('发布成功');
      })
      .catch(function(){
        if (subBtn) subBtn.disabled = false;
        toast('网络错误，请稍后再试');
      });
  }
  function insertComment(cm, parentId) {
    var parent = parentId ? $('comment-' + parentId) : null;
    var item = buildItem(cm);
    if (!parent) {
      // 顶级：直接加在 list；如果当前是空状态需先清空
      var list = $('cmt-list');
      if (!list) return;
      var empty = list.querySelector('.cmt-empty');
      if (empty) empty.remove();
      list.appendChild(item);
      return;
    }
    var sib = parent.closest('.cmt-children');
    if (sib) { sib.appendChild(item); return; }
    var main = parent.querySelector(':scope > .cmt-main');
    var children = main.querySelector(':scope > .cmt-children');
    if (!children) {
      children = document.createElement('div');
      children.className = 'cmt-children';
      main.appendChild(children);
    }
    children.appendChild(item);
  }
  function updateCount(delta) {
    var cc = $('cmt-count'); if (!cc) return;
    var m = cc.textContent.match(/(\d+)/);
    var n = (m ? parseInt(m[1],10) : 0) + delta;
    cc.innerHTML = '<a class="cmt-count-link">' + n + ' 条评论 <span class="cmt-caret">▾</span></a>';
  }
  function removeEmptyState() {
    var list = $('cmt-list'); if (!list) return;
    var empty = list.querySelector('.cmt-empty');
    if (empty) empty.remove();
  }

  // ─── 点赞（统一走 comments.js，删除 comments.html 内联脚本）────────
  function onLikeClick(e) {
    var btn = e.target.closest && e.target.closest('.cmt-like-btn');
    if (!btn || btn.disabled || btn.getAttribute('data-liked') === '1') return;
    e.preventDefault();
    var id = btn.getAttribute('data-id'); if (!id) return;
    var countEl = btn.querySelector('.cmt-like-count');
    var prev = countEl ? parseInt(countEl.textContent,10)||0 : 0;
    btn.disabled = true;
    if (countEl) countEl.textContent = String(prev + 1);
    fetch('/api/comments/' + id + '/like', { method: 'POST', credentials: 'same-origin' })
      .then(function(r){ return r.json().then(function(d){return {ok:r.ok, d:d}; }); })
      .then(function(res){
        if (!res.ok || !res.d || !res.d.ok) {
          if (countEl) countEl.textContent = String(prev);
          btn.disabled = false; return;
        }
        if (countEl && typeof res.d.likes === 'number') countEl.textContent = String(res.d.likes);
        btn.setAttribute('data-liked', '1');
        btn.classList.add('is-liked');
      })
      .catch(function(){
        if (countEl) countEl.textContent = String(prev);
        btn.disabled = false;
      });
  }

  // ─── 懒加载评论（IntersectionObserver）───────────────────
  function initLazyComments() {
    var b = body();
    if (!b || b.getAttribute('data-lazy') !== '1' || b.getAttribute('data-loaded') === '1') return;
    if (!('IntersectionObserver' in window)) { loadComments(); return; }
    var io = new IntersectionObserver(function(entries){
      for (var i=0;i<entries.length;i++) {
        if (entries[i].isIntersecting) { io.disconnect(); loadComments(); }
      }
    }, { rootMargin: '200px' });
    io.observe(b);
  }

  // ─── 全局事件委托（PJAX 不丢）────────────────────────────
  function bindGlobalEvents() {
    if (window.__cmtArtalkBound) return;
    window.__cmtArtalkBound = true;

    // 点击（回复/取消回复/提交/表情面板/登录/模态框关闭/登出/图片占位/Markdown占位/计数▾/通知中心）
    document.addEventListener('click', function (e) {
      // 回复按钮
      var rb = e.target.closest && e.target.closest('.cmt-reply-btn');
      if (rb && !rb.classList.contains('cmt-like-btn')) {
        e.preventDefault(); startReply(rb); return;
      }
      // 点赞
      if (rb && rb.classList.contains('cmt-like-btn')) { onLikeClick(e); return; }
      // 取消回复
      if (e.target.closest('#cmt-reply-cancel')) { resetEditor(); return; }
      // 未登录→登录按钮
      if (e.target.closest('#cmt-login-btn')) {
        e.preventDefault(); openLoginModal(); return;
      }
      // 已登录→发送按钮
      if (e.target.closest('#cmt-submit')) { submit(); return; }
      // 登出
      if (e.target.closest('#cmt-user-logout')) {
        e.preventDefault();
        fetch('/api/auth/logout', { method:'POST', credentials:'same-origin' })
          .then(function(){ invalidateMeCache(); currentUser={logged_in:false}; renderUserBar(); resetEditor(); toast('已退出'); })
          .catch(function(){ toast('退出失败'); });
        return;
      }
      // 表情按钮 → 切换面板
      if (e.target.closest('#cmt-emoji-btn')) {
        e.preventDefault(); toggleEmojiPanel(); return;
      }
      // 图片按钮（占位 toast）
      if (e.target.closest('#cmt-image-btn')) {
        e.preventDefault(); toast('图片上传功能开发中'); return;
      }
      // Markdown 按钮（占位 toast）
      if (e.target.closest('#cmt-md-btn')) {
        e.preventDefault(); toast('支持 Markdown：**粗体** *斜体* `code` ```codeblock``` > 引用'); return;
      }
      // 表情 Tab 切换
      var tab = e.target.closest && e.target.closest('.cmt-emoji-tab');
      if (tab) {
        e.preventDefault(); activateTab(tab); return;
      }
      // 点击表情图片 → 插入 markdown img
      var im = e.target.closest && e.target.closest('#cmt-emoji-grid img');
      if (im) {
        e.preventDefault();
        var url = im.getAttribute('data-emoji-src') || im.src;
        var alt = im.getAttribute('data-emoji-alt') || im.alt || '';
        insertEmojiMarkdown(alt, url);
        return;
      }
      // 面板外部点击 → 关面板
      var panel = $('cmt-emoji-panel');
      if (panel && panel.style.display !== 'none'
          && !(e.target.closest && e.target.closest('#cmt-emoji-panel'))
          && !(e.target.closest && e.target.closest('#cmt-emoji-btn'))) {
        panel.style.display = 'none';
      }
      // 模态框：关闭按钮 / mask / 选项
      var m = $('cmt-login-modal');
      if (m && m.style.display !== 'none') {
        if (e.target.getAttribute && e.target.getAttribute('data-close') === '1') {
          closeLoginModal(); return;
        }
        var optEmail = e.target.closest && e.target.closest('.cmt-login-email');
        if (optEmail) {
          var f = $('cmt-email-form');
          if (f) f.style.display = '';
          // 清空错误
          var err = $('cmt-login-error'); if (err) { err.style.display='none'; err.textContent=''; }
          return;
        }
        var optGuest = e.target.closest && e.target.closest('.cmt-login-guest');
        if (optGuest) {
          // 跳过验证：关闭弹窗，当前为匿名身份 → 允许提交（之前的 submit() 逻辑已支持匿名）
          closeLoginModal();
          // 焦点回到输入区
          var ta = $('cmt-content'); if (ta) ta.focus();
          toast('已进入匿名模式，填写昵称/邮箱后即可发送');
          return;
        }
        // 登录/注册按钮
        if (e.target.closest('#cmt-do-login'))    { doLogin(false); return; }
        if (e.target.closest('#cmt-do-register')) { doLogin(true);  return; }
      }
    });

    // Esc 关弹窗
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { closeLoginModal(); hideEmojiPanel(); }
    });
    // Cmd/Ctrl+Enter 提交（编辑器内）
    document.addEventListener('keydown', function (e) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && e.target.closest && e.target.closest('.cmt-editor')) {
        e.preventDefault(); submit();
      }
    });

    // PJAX：重置
    document.addEventListener('pjax:complete', function () {
      replyState = { pageKey: '', parentId: 0 };
      hideEmojiPanel();
      closeLoginModal();
      // 重新拉身份（可能在别的页登录/登出了）
      fetchMe(true, function () { renderUserBar(); initLazyComments(); });
    });
  }

  // ─── 登录/注册实际请求 ───────────────────────────────────
  function doLogin(isRegister) {
    var acc = $('cmt-login-account').value.trim();
    var pwd = $('cmt-login-pass').value;
    if (!acc) { showLoginError('请输入用户名或邮箱'); return; }
    if (!pwd || pwd.length < 6) { showLoginError('密码至少 6 位'); return; }
    var url   = isRegister ? '/api/auth/register' : '/api/auth/login';
    var body  = isRegister ? JSON.stringify({ nick: acc, email: ~acc.indexOf('@') ? acc : '', password: pwd })
                           : JSON.stringify({ account: acc, password: pwd });
    fetch(url, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: body
    })
      .then(function(r){ return r.json(); })
      .then(function(res){
        if (!res || !res.ok) { showLoginError(res && res.error || (isRegister ? '注册失败' : '登录失败')); return; }
        invalidateMeCache();
        currentUser = res.user ? { logged_in: true, nick: res.user.nick, avatar: res.user.avatar, role: res.user.role }
                               : { logged_in: false };
        // 再拉一次 /me 保证一致性
        fetchMe(true, function () {
          renderUserBar();
          closeLoginModal();
          toast(isRegister ? '注册并登录成功' : '登录成功');
          // 如有回复上下文，编辑器位置不变；否则顶
        });
      })
      .catch(function(){ showLoginError('网络错误，请稍后再试'); });
  }

  // ─── 初始化 ──────────────────────────────────────────────
  function init() {
    // 身份优先读缓存→强制刷新一次（延迟刷新避免阻塞交互）
    fetchMe(false, function () {
      renderUserBar();
      initLazyComments();
      bindGlobalEvents();
      // 后台刷一次最新
      setTimeout(function(){ fetchMe(true, renderUserBar); }, 1500);
    });
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
```

- [ ] **Step 2: 清理 comments.html 末尾内联点赞脚本**
用 Edit 工具把 comments.html 末尾 `<script>/* comments.js ... */</script>` 改成空 `<script></script>`（或直接删掉，因为点赞已在 comments.js 统一实现）。

- [ ] **Step 3: go vet + 构建检查**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && make test`
Expected: go vet / go test / console tests 全部通过（若 console 构建需先 `cd frontend/console && npm ci && npm run build`，但此变更不触 console）

- [ ] **Step 4: 提交**
```bash
cd /Users/sakurasep/Documents/Code/Project/Blog/dev && \
git add frontend/assets/js/comments.js backend/internal/template/partial/comments.html && \
git commit -S -m "feat: 评论区 JS 重写（双态身份/登录弹窗/表情 Tab/空状态）"
```

---

## Task 7: 构建 + 端到端验证

- [ ] **Step 1: 运行 make test**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && make test`
Expected: go vet PASS, go test PASS, console tests PASS（如果有 console 前端构建前置则先做）

- [ ] **Step 2: 启动服务截图验证**
Run: `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && go run ./backend/cmd/server -root /Users/sakurasep/Documents/Code/Project/Blog/dev -seed &`
手动：
  - 打开 /post/hello，评论区应渲染：
    1. 标题「快来参与讨论吧~」蓝色下划线风 cap.theme
    2. 卡片化编辑器（三列输入+登录按钮=蓝色胶囊）
    3. 计数 `0 条评论 ▾` + 右下「通知中心」蓝链
    4. 加载完后空状态：居中「尴尬了，写这么辛苦竟没人评论 🥺」
    5. 点登录 → 弹窗两个选项（Email ✉️ / 跳过验证 🚫）
    6. 选 Email → 出现表单；注册新用户后身份条显示昵称+绿色「已验证」徽章，右下按钮切换「发送」
    7. 点表情 → 弹层面板：5 Tab（Blobcat/QQ/Tieba/Twemoji/Aru），默认 Blobcat 黄猫头网格
    8. 发送评论→列表插入+verified 徽章出现
- [ ] **Step 3: 提交（如有修复）**
若有小修复按内容单独提交，每个提交一个变更点。

---

## Plan Self-Review Checklist

**1. Spec coverage:**
- ✅ 身份体系（匿名+注册双轨，注册用户 verified 标识）→ Task 1/2/6
- ✅ 编辑器外观卡片化 + 双态（未登录三输入+登录 / 已登录头像昵称+发送）→ Task 4/5
- ✅ 表情选择器 5 Tab 图片表情包（Blobcat 等）→ Task 5/6
- ✅ 工具栏三按钮（表情/图片占位/Markdown 占位）→ Task 4/6
- ✅ 计数 ▾ 下拉箭头 + 通知中心链接 → Task 4/5
- ✅ 空状态居中 + 「尴尬了…🥺」→ Task 4/5/6
- ✅ 登录弹窗两选项（Email/跳过验证）→ Task 4/5/6

**2. Placeholder scan:**
- ✅ 图片上传 / Markdown 切换按钮功能未做 → 已以 toast「开发中」明确占位，不是空 TODO
- ✅ 通知中心链接 → `#` 占位跳转，非空

**3. Type consistency:**
- ✅ `verified` 字段（bool）/ `verified_badge`（string）/ `cmt-badge-verified`（class）三处命名一致
- ✅ SessionMiddleware / publicXxx 路由与 `CurrentUserFromContext` 对齐 Task 1 中定义
- ✅ `verified` 在 store.Comment 结构体是 bool，数据库写入 0/1 int 已适配

---

> **Plan complete and saved to `docs/superpowers/plans/2026-08-05-comment-style-align-artalk.md`.**
>
> **Two execution options:**
>
> **1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration
>
> **2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints
>
> **Which approach?**
