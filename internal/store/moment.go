package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 瞬间可见性取值。
const (
	MomentVisibilityPublic  = "public"
	MomentVisibilityPrivate = "private"
)

// Moment 瞬间/说说持久化记录，对齐 moments 表字段。
type Moment struct {
	ID          int64
	ContentMD   string
	ContentHTML string
	Images      string // 图片 URL 的 JSON 数组文本（兼容旧库逗号分隔，读取时归一）
	Visibility  string // public / private（缺省 public）
	CreatedAt   time.Time
}

// TableName Entity 接口：moments 表。
func (m *Moment) TableName() string { return "moments" }

// Columns Entity 接口：全列，首列为 id。
func (m *Moment) Columns() []string {
	return []string{"id", "content_md", "content_html", "images", "visibility", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (m *Moment) Values() []any {
	return []any{m.ContentMD, m.ContentHTML, m.Images, m.Visibility, m.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (m *Moment) SetID(id int64) { m.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (m *Moment) ScanRow(row rowScanner) error {
	return row.Scan(&m.ID, &m.ContentMD, &m.ContentHTML, &m.Images, &m.Visibility, &m.CreatedAt)
}

// DecodeMomentImages 将 images 列文本解析为 URL 切片。
// 新格式为 JSON 数组；兼容旧库的逗号/换行分隔格式。
func DecodeMomentImages(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			return cleanImageURLs(arr)
		}
	}
	// 兼容旧库：逗号/换行分隔
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	return cleanImageURLs(parts)
}

// EncodeMomentImages 将 URL 切片编码为 JSON 数组文本（去空去重前的清洗后编码）。
// 空结果返回空串，避免存 "null"/"[]" 噪声。
func EncodeMomentImages(urls []string) string {
	clean := cleanImageURLs(urls)
	if len(clean) == 0 {
		return ""
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return ""
	}
	return string(b)
}

func cleanImageURLs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// normalizeVisibility 规范可见性，非法/空值回落 public。
func normalizeVisibility(v string) string {
	if strings.TrimSpace(v) == MomentVisibilityPrivate {
		return MomentVisibilityPrivate
	}
	return MomentVisibilityPublic
}

// EnsureMomentsTable 独立建 moments 表（不走 post.go createTables，避免与其它 spec 冲突）。
func (s *Store) EnsureMomentsTable() error {
	if err := s.ensureTable("moments"); err != nil {
		return err
	}
	// 老库迁移：CREATE TABLE IF NOT EXISTS 不给已存在的表加列，需检测后 ALTER（幂等）。
	return s.migrateMomentColumns()
}

// migrateMomentColumns 检测 moments 表缺失的列并 ALTER 补上（兼容老库）。
// SQLite 的 ADD COLUMN 要求默认值为常量（DEFAULT ” / DEFAULT 'public' 合法）。
func (s *Store) migrateMomentColumns() error {
	want := map[string]string{
		"images":     `ALTER TABLE moments ADD COLUMN images TEXT NOT NULL DEFAULT ''`,
		"visibility": `ALTER TABLE moments ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'`,
	}
	rows, err := s.db.Query(`PRAGMA table_info(moments)`)
	if err != nil {
		return fmt.Errorf("pragma table_info(moments): %w", err)
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		existing[name] = true
	}
	for col, alter := range want {
		if existing[col] {
			continue // 列已存在，无需迁移
		}
		if _, err := s.db.Exec(alter); err != nil {
			return fmt.Errorf("alter table moments add %s: %w", col, err)
		}
	}
	return nil
}

// MomentSlugKey 瞬间在搜索索引中的伪 slug（moments 无 slug 列，用 id 构造；
// store 写路径回调与 cmd/server 回填装配共用，格式必须一致）。
func MomentSlugKey(id int64) string {
	return "moment-" + strconv.FormatInt(id, 10)
}

// SetMomentWriteHook 注册瞬间写路径回调（main.go 装配）。
func (s *Store) SetMomentWriteHook(h func(op, slug string)) {
	s.momentWriteHook = h
}

// fireMomentWrite 触发瞬间写路径回调（未注册则静默）。
func (s *Store) fireMomentWrite(op, slug string) {
	if s.momentWriteHook != nil {
		s.momentWriteHook(op, slug)
	}
}

// CreateMoment 新建瞬间，返回 id 并回填 m.ID。
func (s *Store) CreateMoment(m *Moment) (int64, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	m.Visibility = normalizeVisibility(m.Visibility)
	res, err := s.db.Exec(`INSERT INTO moments (content_md, content_html, images, visibility, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		m.ContentMD, m.ContentHTML, m.Images, m.Visibility, m.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create moment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	m.ID = id
	s.fireMomentWrite("create", MomentSlugKey(id))
	return id, nil
}

// UpdateMoment 更新既有瞬间的正文/图片/可见性（不改 created_at）。
func (s *Store) UpdateMoment(m *Moment) error {
	m.Visibility = normalizeVisibility(m.Visibility)
	res, err := s.db.Exec(`UPDATE moments SET content_md = ?, content_html = ?, images = ?, visibility = ? WHERE id = ?`,
		m.ContentMD, m.ContentHTML, m.Images, m.Visibility, m.ID)
	if err != nil {
		return fmt.Errorf("update moment %d: %w", m.ID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update moment %d: no rows affected", m.ID)
	}
	s.fireMomentWrite("update", MomentSlugKey(m.ID))
	return nil
}

// GetMoment 按 id 读取单条瞬间（不存在返回 nil, nil）。
func (s *Store) GetMoment(id int64) (*Moment, error) {
	return repo(s, &Moment{}).GetByID(id)
}

// ListMoments 分页列出全部瞬间（含私密，供 Console 管理用）。page 从 1 开始。
func (s *Store) ListMoments(page, size int) ([]*Moment, error) {
	return s.listMoments(page, size, false)
}

// ListPublicMoments 分页列出公开瞬间（前台 /moments、/notes 近期动态、公开端点用）。
func (s *Store) ListPublicMoments(page, size int) ([]*Moment, error) {
	return s.listMoments(page, size, true)
}

func (s *Store) listMoments(page, size int, publicOnly bool) ([]*Moment, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	opts := []QueryOption{OrderBy("substr(created_at, 1, 19) DESC, id DESC"), Limit(size), Offset((page - 1) * size)}
	if publicOnly {
		opts = append(opts, Where("visibility = 'public'"))
	}
	return repo(s, &Moment{}).List(opts...)
}

// CountMoments 返回瞬间总数（含私密）。
func (s *Store) CountMoments() (int, error) {
	return repo(s, &Moment{}).Count()
}

// CountPublicMoments 返回公开瞬间数量。
func (s *Store) CountPublicMoments() (int, error) {
	return repo(s, &Moment{}).Count(Where("visibility = 'public'"))
}

// DeleteMoment 按 id 删除记录。
func (s *Store) DeleteMoment(id int64) error {
	res, err := s.db.Exec(`DELETE FROM moments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete moment %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("delete moment %d: no rows affected", id)
	}
	s.fireMomentWrite("delete", MomentSlugKey(id))
	return nil
}
