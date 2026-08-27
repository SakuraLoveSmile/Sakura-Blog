package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"blog/internal/content"
)

// PageBannerInfo 是 Page 横幅可编辑的覆盖信息。
// 数据库中以 JSON 字符串保存，API 侧使用同样的结构化字段。
type PageBannerInfo struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Avatar   string `json:"avatar"`
	Icon     string `json:"icon"`
}

// Value 实现 driver.Valuer，使仓储可以直接写入结构化横幅信息。
func (b PageBannerInfo) Value() (driver.Value, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// Scan 实现 sql.Scanner；历史库若存在空/损坏 JSON，安全降级为空配置。
func (b *PageBannerInfo) Scan(src any) error {
	*b = PageBannerInfo{}
	if src == nil {
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return fmt.Errorf("scan page banner_info: unsupported %T", src)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil
	}
	if err := json.Unmarshal(raw, b); err != nil {
		*b = PageBannerInfo{}
	}
	return nil
}

// WidgetConfig 页面级单个 widget 的配置：组件名 + 页面级覆盖参数（与全局 library 浅合并）。
// 向后兼容：JSON 存储同时支持两种格式，解析时自动归一化：
//   - 旧格式：["recent", "tagcloud"]（字符串数组）→ 每个元素转成 WidgetConfig{Name, 空Params}
//   - 新格式：[{"name":"recent","params":{"limit":10}}, ...]（对象数组）
type WidgetConfig struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params,omitempty"`
}

// Page 独立页面持久化记录，对齐 pages 表字段。
// LeftbarWidgets / RightbarWidgets 存 JSON，支持两种格式（见 WidgetConfig 注释）：
//   - 空串 "" = 未配置，走全局 widgets.pages.page 缺省（BarsFor 的 page 分支）
//   - "[]" = 明确配置为空侧栏，不渲染任何 widget（区别于「未配置」）
//   - "[\"recent\",\"tagcloud\"]" / "[{\"name\":\"recent\",...}]" = 按给定顺序渲染
type Page struct {
	ID              int64
	Slug            string
	Title           string
	ContentMD       string
	ContentHTML     string
	Status          string // draft / published
	Banner          string // 横幅背景图 URL
	BannerInfo      PageBannerInfo
	Breadcrumb      bool   // 是否显示横幅顶部面包屑与元信息
	Author          string // site.yml authors ID
	PublishedAt     string // YYYY-MM-DD，空串表示不显示发布日期
	IsSystem        bool   // 是否为系统页面
	Views           int64  // 浏览量
	LeftbarWidgets  string // JSON 编码，空串=继承全局
	RightbarWidgets string // JSON 编码，空串=继承全局
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       string // 软删除时间 RFC3339，空串=未删除
}

// TableName Entity 接口：pages 表。
func (p *Page) TableName() string { return "pages" }

// Columns Entity 接口：全列，首列为 id。
func (p *Page) Columns() []string {
	return []string{"id", "slug", "title", "content_md", "content_html", "status",
		"banner", "banner_info", "breadcrumb", "author", "published_at",
		"is_system", "views", "leftbar_widgets", "rightbar_widgets", "created_at", "updated_at", "deleted_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (p *Page) Values() []any {
	return []any{p.Slug, p.Title, p.ContentMD, p.ContentHTML, p.Status,
		p.Banner, p.BannerInfo, p.Breadcrumb, p.Author, p.PublishedAt,
		p.IsSystem, p.Views, p.LeftbarWidgets, p.RightbarWidgets, p.CreatedAt, p.UpdatedAt, p.DeletedAt}
}

// SetID Entity 接口：回填自增 id。
func (p *Page) SetID(id int64) { p.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (p *Page) ScanRow(row rowScanner) error {
	return row.Scan(&p.ID, &p.Slug, &p.Title, &p.ContentMD, &p.ContentHTML, &p.Status,
		&p.Banner, &p.BannerInfo, &p.Breadcrumb, &p.Author, &p.PublishedAt,
		&p.IsSystem, &p.Views, &p.LeftbarWidgets, &p.RightbarWidgets, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
}

// ParseLeftbar 解析 LeftbarWidgets JSON 为 WidgetConfig 列表（向后兼容）。
// 返回 (nil, false) 表示「未配置=继承全局」；返回 (cfgList, true) 表示显式配置（cfgList 可为 0 长=空侧栏）。
func (p *Page) ParseLeftbar() ([]WidgetConfig, bool) { return parseWidgetConfigs(p.LeftbarWidgets) }

// ParseRightbar 同 ParseLeftbar，对应右栏。
func (p *Page) ParseRightbar() ([]WidgetConfig, bool) { return parseWidgetConfigs(p.RightbarWidgets) }

// WidgetNames 从 WidgetConfig 列表提取纯组件名序列（用于 BuildBar 的 names 入参）。
func WidgetNames(cfgs []WidgetConfig) []string {
	out := make([]string, 0, len(cfgs))
	for _, c := range cfgs {
		out = append(out, c.Name)
	}
	return out
}

// WidgetOverrides 从 WidgetConfig 列表提取参数覆盖 map（组件名 → 页面级参数 Params）。
// 仅 Params 非空的组件进入返回值，用于 BuildBar 时浅合并到全局 library 的 WidgetDef。
func WidgetOverrides(cfgs []WidgetConfig) map[string]map[string]any {
	var out map[string]map[string]any
	for _, c := range cfgs {
		if len(c.Params) == 0 {
			continue
		}
		if out == nil {
			out = make(map[string]map[string]any)
		}
		out[c.Name] = c.Params
	}
	return out
}

// EncodeWidgetConfigs []WidgetConfig → JSON 字符串（用于写库）。nil/0 长写入 "[]" 而非空串。
func EncodeWidgetConfigs(cfgs []WidgetConfig) string {
	if cfgs == nil {
		return ""
	}
	b, err := json.Marshal(cfgs)
	if err != nil {
		return ""
	}
	return string(b)
}

// EncodeWidgetNames 旧格式编码（兼容老 API 调用方）：[]string → 转成 []WidgetConfig 再编码。
// 内部使用：每个 name 对应 WidgetConfig{Name, 空Params}，写入结果等价于新格式（向后兼容读取）。
func EncodeWidgetNames(names []string) string {
	if names == nil {
		return ""
	}
	cfgs := make([]WidgetConfig, 0, len(names))
	for _, n := range names {
		cfgs = append(cfgs, WidgetConfig{Name: n})
	}
	return EncodeWidgetConfigs(cfgs)
}

// parseWidgetConfigs 解析原始 JSON，支持两种格式并归一化为 []WidgetConfig：
//   - 旧格式字符串数组：["a","b"] → [{Name:a}, {Name:b}]
//   - 新格式对象数组：[{name:"a", params:{...}}, ...]
func parseWidgetConfigs(raw string) ([]WidgetConfig, bool) {
	if raw == "" {
		return nil, false
	}
	// 先试新格式：对象数组
	var cfgs []WidgetConfig
	if err := json.Unmarshal([]byte(raw), &cfgs); err == nil && len(cfgs) > 0 {
		// 校验：至少第一个元素的 Name 非空（避免被空对象数组误识别为新格式）
		if cfgs[0].Name != "" {
			return cfgs, true
		}
	}
	// 再试旧格式：字符串数组
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err == nil {
		cfgs = make([]WidgetConfig, 0, len(names))
		for _, n := range names {
			cfgs = append(cfgs, WidgetConfig{Name: n})
		}
		return cfgs, true
	}
	// JSON 解析均失败：视为未配置（安全兜底）
	return nil, false
}

// SetPageWriteHook 注册页面写路径回调（main.go 装配）。
func (s *Store) SetPageWriteHook(h func(op, slug string)) {
	s.pageWriteHook = h
}

// firePageWrite 触发页面写路径回调（未注册则静默）。
func (s *Store) firePageWrite(op, slug string) {
	if s.contentEvents != nil {
		s.contentEvents.Publish(content.Event{Kind: content.PageChanged, Op: op, Slug: slug})
	}
	if s.pageWriteHook != nil {
		s.pageWriteHook(op, slug)
	}
}

// EnsurePagesTable 独立建 pages 表（统一走 schema.go 注册表）。
// 幂等迁移：旧库缺列时 ADD COLUMN；SQLite ADD COLUMN 对已存在列报错，忽略即幂等。
func (s *Store) EnsurePagesTable() error {
	if err := s.ensureTable("pages"); err != nil {
		return err
	}
	want := map[string]string{
		"deleted_at":       `ALTER TABLE pages ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
		"leftbar_widgets":  `ALTER TABLE pages ADD COLUMN leftbar_widgets TEXT NOT NULL DEFAULT ''`,
		"rightbar_widgets": `ALTER TABLE pages ADD COLUMN rightbar_widgets TEXT NOT NULL DEFAULT ''`,
		"is_system":        `ALTER TABLE pages ADD COLUMN is_system INTEGER NOT NULL DEFAULT 0`,
		"views":            `ALTER TABLE pages ADD COLUMN views INTEGER NOT NULL DEFAULT 0`,
		"banner":           `ALTER TABLE pages ADD COLUMN banner TEXT NOT NULL DEFAULT ''`,
		"banner_info":      `ALTER TABLE pages ADD COLUMN banner_info TEXT NOT NULL DEFAULT '{}'`,
		"breadcrumb":       `ALTER TABLE pages ADD COLUMN breadcrumb INTEGER NOT NULL DEFAULT 1`,
		"author":           `ALTER TABLE pages ADD COLUMN author TEXT NOT NULL DEFAULT ''`,
		"published_at":     `ALTER TABLE pages ADD COLUMN published_at TEXT NOT NULL DEFAULT ''`,
	}
	for col, alter := range want {
		has, err := s.tableHasColumn("pages", col)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := s.db.Exec(alter); err != nil {
			return fmt.Errorf("alter pages add %s: %w", col, err)
		}
		// 仅在首次加列时回填旧页面，避免后续启动把用户主动清空的日期恢复。
		if col == "published_at" {
			if _, err := s.db.Exec(`UPDATE pages SET published_at = substr(created_at, 1, 10) WHERE published_at = ''`); err != nil {
				return fmt.Errorf("backfill pages published_at: %w", err)
			}
		}
	}
	return nil
}

// ListPublishedPages 返回全部已发布未删除页面（按更新时间倒序），公开只读 API 用。
func (s *Store) ListPublishedPages() ([]*Page, error) {
	return repo(s, &Page{}).List(Where("deleted_at = '' AND status = ?", StatusPublished), OrderBy("substr(updated_at, 1, 19) DESC, id DESC"))
}

// pageListCols 列表查询返回的列（不含大字段 content_md/content_html）
const pageListCols = "id, slug, title, status, is_system, views, leftbar_widgets, rightbar_widgets, created_at, updated_at, deleted_at"

// scanPageList 按 pageListCols 顺序扫描页面列表记录
func scanPageList(row rowScanner, p *Page) error {
	return row.Scan(&p.ID, &p.Slug, &p.Title, &p.Status, &p.IsSystem, &p.Views,
		&p.LeftbarWidgets, &p.RightbarWidgets, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
}

// ListPages 列出全部未删除页面（按更新时间倒序）。
func (s *Store) ListPages() ([]*Page, error) {
	return repo(s, &Page{}).List(Where("deleted_at = ''"), OrderBy("substr(updated_at, 1, 19) DESC, id DESC"))
}

// ListPagesPage 分页查询未删除页面（支持关键词搜索）
func (s *Store) ListPagesPage(page, size int, status, keyword string) ([]*Page, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := "deleted_at = ''"
	var args []any
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		where += " AND (title LIKE ? OR slug LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	var total int
	countQ := "SELECT COUNT(*) FROM pages WHERE " + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count pages: %w", err)
	}
	offset := (page - 1) * size
	rows, err := s.db.Query("SELECT "+pageListCols+" FROM pages WHERE "+where+
		" ORDER BY substr(updated_at, 1, 19) DESC, id DESC LIMIT ? OFFSET ?",
		append(args, size, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list pages page: %w", err)
	}
	defer rows.Close()
	var out []*Page
	for rows.Next() {
		p := &Page{}
		if err := scanPageList(rows, p); err != nil {
			return nil, 0, fmt.Errorf("scan page: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate pages: %w", err)
	}
	return out, total, nil
}

// ListTrashedPagesPage 分页查询回收站页面
func (s *Store) ListTrashedPagesPage(page, size int, keyword string) ([]*Page, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := "deleted_at != ''"
	var args []any
	if kw := strings.TrimSpace(keyword); kw != "" {
		where += " AND (title LIKE ? OR slug LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	var total int
	countQ := "SELECT COUNT(*) FROM pages WHERE " + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count trashed pages: %w", err)
	}
	offset := (page - 1) * size
	rows, err := s.db.Query("SELECT "+pageListCols+" FROM pages WHERE "+where+
		" ORDER BY datetime(deleted_at) DESC LIMIT ? OFFSET ?",
		append(args, size, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list trashed pages page: %w", err)
	}
	defer rows.Close()
	var out []*Page
	for rows.Next() {
		p := &Page{}
		if err := scanPageList(rows, p); err != nil {
			return nil, 0, fmt.Errorf("scan trashed page: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate trashed pages: %w", err)
	}
	return out, total, nil
}

// IncrementPageViews 增加页面浏览量
func (s *Store) IncrementPageViews(slug string) error {
	_, err := s.db.Exec(`UPDATE pages SET views = views + 1 WHERE slug = ? AND deleted_at = '' AND status = ?`, slug, StatusPublished)
	return err
}

// GetPageBySlug 按 slug 查单页（任意状态）。未找到返回 nil, nil。
func (s *Store) GetPageBySlug(slug string) (*Page, error) {
	return s.getPageBySlug(slug, false)
}

// GetPublished 按 slug 查已发布页面。未找到返回 nil, nil。
func (s *Store) GetPublished(slug string) (*Page, error) {
	return s.getPageBySlug(slug, true)
}

func (s *Store) getPageBySlug(slug string, publishedOnly bool) (*Page, error) {
	where := "slug = ? AND deleted_at = ''"
	args := []any{slug}
	if publishedOnly {
		where += " AND status = ?"
		args = append(args, StatusPublished)
	}
	p, err := repo(s, &Page{}).GetOne(where, args...)
	if err != nil {
		return nil, fmt.Errorf("get page by slug %s: %w", slug, err)
	}
	return p, nil
}

// CreatePage 新建页面，返回新记录 id。slug 冲突时返回错误。
func (s *Store) CreatePage(p *Page) (int64, error) {
	if p.Status == "" {
		p.Status = StatusDraft
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	id, err := repo(s, &Page{}).Create(p)
	if err != nil {
		return 0, fmt.Errorf("create page %s: %w", p.Slug, err)
	}
	s.firePageWrite("create", p.Slug)
	return id, nil
}

// UpdatePage 按 slug 更新页面字段，刷新 updated_at。
func (s *Store) UpdatePage(slug string, p *Page) error {
	p.UpdatedAt = time.Now().UTC()
	res, err := s.db.Exec(`UPDATE pages SET slug=?, title=?, content_md=?, content_html=?, status=?,
		banner=?, banner_info=?, breadcrumb=?, author=?, published_at=?,
		is_system=?, views=?, leftbar_widgets=?, rightbar_widgets=?, updated_at=?
		WHERE slug=?`,
		p.Slug, p.Title, p.ContentMD, p.ContentHTML, p.Status,
		p.Banner, p.BannerInfo, p.Breadcrumb, p.Author, p.PublishedAt,
		p.IsSystem, p.Views, p.LeftbarWidgets, p.RightbarWidgets, p.UpdatedAt, slug)
	if err != nil {
		return fmt.Errorf("update page %s: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update page %s: no rows affected", slug)
	}
	s.firePageWrite("update", p.Slug)
	return nil
}

// DeletePage 软删除：标记 deleted_at，页面进入回收站。不存在不报错。
func (s *Store) DeletePage(slug string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := s.db.Exec(`UPDATE pages SET deleted_at = ? WHERE slug = ? AND deleted_at = ''`, now, slug)
	if err != nil {
		return fmt.Errorf("soft delete page %s: %w", slug, err)
	}
	s.firePageWrite("delete", slug)
	return nil
}

// ListTrashedPages 返回回收站中的页面，按删除时间倒序。
func (s *Store) ListTrashedPages() ([]*Page, error) {
	return repo(s, &Page{}).List(Where("deleted_at != ''"), OrderBy("datetime(deleted_at) DESC"))
}

// RestorePage 从回收站恢复页面。
func (s *Store) RestorePage(slug string) error {
	_, err := s.db.Exec(`UPDATE pages SET deleted_at = '' WHERE slug = ? AND deleted_at != ''`, slug)
	if err != nil {
		return fmt.Errorf("restore page %s: %w", slug, err)
	}
	s.firePageWrite("restore", slug)
	return nil
}

// PurgePage 彻底删除页面（物理删除）。
func (s *Store) PurgePage(slug string) error {
	_, err := s.db.Exec(`DELETE FROM pages WHERE slug = ? AND deleted_at != ''`, slug)
	if err != nil {
		return fmt.Errorf("purge page %s: %w", slug, err)
	}
	s.firePageWrite("purge", slug)
	return nil
}
