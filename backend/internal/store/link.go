package store

// Link 友链条目（DB 持久化）。GroupTitle 承载 site.yml 的分组标题（扁平化）。
// 字段命名与前端 friends_and_posts 渲染对齐；desc 列名用 description（避开 SQL 关键字 DESC）。
type Link struct {
	ID          int64
	GroupTitle  string
	Name        string
	URL         string
	Avatar      string
	Description string
	RSS         string
	Label       string
	Color       string
	Sort        int
}

// TableName Entity 接口：links 表。
func (l *Link) TableName() string { return "links" }

// Columns Entity 接口：全列，首列为 id。
func (l *Link) Columns() []string {
	return []string{"id", "group_title", "name", "url", "avatar", "description", "rss", "label", "color", "sort"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (l *Link) Values() []any {
	return []any{l.GroupTitle, l.Name, l.URL, l.Avatar, l.Description, l.RSS, l.Label, l.Color, l.Sort}
}

// SetID Entity 接口：回填自增 id。
func (l *Link) SetID(id int64) { l.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (l *Link) ScanRow(row rowScanner) error {
	return row.Scan(&l.ID, &l.GroupTitle, &l.Name, &l.URL, &l.Avatar,
		&l.Description, &l.RSS, &l.Label, &l.Color, &l.Sort)
}

// EnsureLinksTable 建 links 表（独立建表，友链由 site.yml 迁移到 DB）。
func (s *Store) EnsureLinksTable() error {
	return s.ensureTable("links")
}

// ListLinks 全部友链。sort 降序优先（越大越靠前），其次 id 升序（保留插入=site.yml 顺序）。
func (s *Store) ListLinks() ([]*Link, error) {
	return repo(s, &Link{}).List(OrderBy("sort DESC, id ASC"))
}

// GetLink 按 id 查友链，未找到返回 nil, nil。
func (s *Store) GetLink(id int64) (*Link, error) {
	return repo(s, &Link{}).GetByID(id)
}

// CreateLink 插入友链，回填 ID。
func (s *Store) CreateLink(l *Link) (int64, error) {
	return repo(s, &Link{}).Create(l)
}

// UpdateLink 更新友链全部字段。
func (s *Store) UpdateLink(id int64, l *Link) error {
	return repo(s, &Link{}).Update(id, l)
}

// DeleteLink 删除友链。
func (s *Store) DeleteLink(id int64) error {
	return repo(s, &Link{}).Delete(id)
}

// CountLinks 统计友链数（Bootstrap 判定空表用）。
func (s *Store) CountLinks() (int, error) {
	return repo(s, &Link{}).Count()
}
