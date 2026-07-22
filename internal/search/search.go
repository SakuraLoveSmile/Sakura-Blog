// Package search 站内全文搜索（Spec 25），基于 SQLite FTS5 虚表。
//
// 中文处理：unicode61 分词器会把连续 CJK 串当作单个 token，无法子串命中。
// 因此索引与查询统一做「按字切分」预处理（CJK 字符间插空格），查询包成 phrase
// 保证相邻语义；snippet 返回后再把 CJK 间空格合并还原。
package search

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// 高亮占位符：先让 FTS5 snippet 用不可见控制符包裹命中词，
// 待 CJK 空格合并后再替换成前台样式 span（对齐 local-search.js 的 .search-keyword）。
const (
	hlOpen  = "\x01"
	hlClose = "\x02"
)

// Result 单条搜索结果。Snippet 为含高亮 span 的 HTML 片段。
type Result struct {
	Title   string
	Slug    string
	Snippet string
}

// Item 全量重建入参（正文应为纯文本；受保护文章传空正文）。
type Item struct {
	Slug    string
	Title   string
	Content string
}

// Index FTS5 索引访问对象。与 store 解耦：仅持 *sql.DB，由 main.go 装配回调。
type Index struct {
	db *sql.DB
}

// New 创建索引对象，db 复用 store 的连接（SetMaxOpenConns(1) 已串行化写）。
func New(db *sql.DB) *Index {
	return &Index{db: db}
}

// Ensure 建 FTS5 虚表（幂等）。title/content 存切分后文本；
// slug/title_raw UNINDEXED 存原始值，避免结果反解切分空格。
// 若存在旧版不兼容结构（历史实验表）则丢弃重建，重建后 Count=0 触发启动回填。
func (ix *Index) Ensure() error {
	// 清理历史遗留的基于触发器的 FTS 同步（早期实验方案，引用 content_md/content_html 列）。
	// 现方案改用显式 IndexPost/Unindex（postWriteHook 调用）；遗留触发器会因
	// 新表结构（title/content/slug/title_raw）无 content_md 列而在 posts 增删改时报错，必须先删除。
	for _, trg := range []string{"posts_fts_ai", "posts_fts_ad", "posts_fts_au"} {
		if _, err := ix.db.Exec(`DROP TRIGGER IF EXISTS ` + trg); err != nil {
			return fmt.Errorf("drop legacy fts trigger %s: %w", trg, err)
		}
	}
	var existing string
	err := ix.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='posts_fts'`).Scan(&existing)
	if err == nil && !strings.Contains(existing, "title_raw") {
		if _, err := ix.db.Exec(`DROP TABLE posts_fts`); err != nil {
			return fmt.Errorf("drop legacy posts_fts: %w", err)
		}
	}
	const ddl = `CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
		title, content, slug UNINDEXED, title_raw UNINDEXED,
		tokenize='unicode61'
	)`
	if _, err := ix.db.Exec(ddl); err != nil {
		return fmt.Errorf("create posts_fts: %w", err)
	}
	return nil
}

// Count 返回索引条目数（启动判断是否需要全量回填）。
func (ix *Index) Count() (int, error) {
	var n int
	if err := ix.db.QueryRow(`SELECT count(*) FROM posts_fts`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count posts_fts: %w", err)
	}
	return n, nil
}

// IndexPost 增量写入（先删后插，幂等）。content 传纯文本；受保护文章传空串。
func (ix *Index) IndexPost(slug, title, content string) error {
	if err := ix.Unindex(slug); err != nil {
		return err
	}
	_, err := ix.db.Exec(`INSERT INTO posts_fts(title, content, slug, title_raw) VALUES (?, ?, ?, ?)`,
		segmentCJK(title), segmentCJK(content), slug, title)
	if err != nil {
		return fmt.Errorf("index post %s: %w", slug, err)
	}
	return nil
}

// Unindex 从索引移除指定文章（删除/下线/进回收站时调用）。
func (ix *Index) Unindex(slug string) error {
	if _, err := ix.db.Exec(`DELETE FROM posts_fts WHERE slug = ?`, slug); err != nil {
		return fmt.Errorf("unindex post %s: %w", slug, err)
	}
	return nil
}

// Rebuild 全量重建（清空后回填）。启动时空表回填用，幂等。
func (ix *Index) Rebuild(items []Item) error {
	if _, err := ix.db.Exec(`DELETE FROM posts_fts`); err != nil {
		return fmt.Errorf("truncate posts_fts: %w", err)
	}
	for _, it := range items {
		if _, err := ix.db.Exec(`INSERT INTO posts_fts(title, content, slug, title_raw) VALUES (?, ?, ?, ?)`,
			segmentCJK(it.Title), segmentCJK(it.Content), it.Slug, it.Title); err != nil {
			return fmt.Errorf("rebuild index %s: %w", it.Slug, err)
		}
	}
	return nil
}

// Search 按 BM25 相关性搜索（标题权重 10 倍），snippet 服务端高亮。
// 空关键词返回空结果不报错。
func (ix *Index) Search(q string, limit int) ([]Result, error) {
	match := buildMatch(q)
	if match == "" {
		return []Result{}, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	rows, err := ix.db.Query(`SELECT title_raw, slug, snippet(posts_fts, 1, ?, ?, '…', 32)
		FROM posts_fts WHERE posts_fts MATCH ?
		ORDER BY bm25(posts_fts, 10.0, 1.0) LIMIT ?`,
		hlOpen, hlClose, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search %q: %w", q, err)
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		var snip string
		if err := rows.Scan(&r.Title, &r.Slug, &snip); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		r.Snippet = renderSnippet(snip)
		out = append(out, r)
	}
	if out == nil {
		out = []Result{}
	}
	return out, rows.Err()
}

// buildMatch 把用户输入转成 FTS5 MATCH 表达式：
// 按空白拆词，每个词内部按字切分后包成 phrase（相邻语义 = 子串命中），词间 AND。
func buildMatch(q string) string {
	var parts []string
	for _, term := range strings.Fields(q) {
		seg := strings.TrimSpace(segmentCJK(term))
		if seg == "" {
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(seg, `"`, `""`)+`"`)
	}
	return strings.Join(parts, " ")
}

// renderSnippet 合并 CJK 间的切分空格，并把高亮占位符替换成前台样式 span。
func renderSnippet(s string) string {
	s = joinCJK(s)
	s = strings.ReplaceAll(s, hlOpen, `<span class="search-keyword">`)
	s = strings.ReplaceAll(s, hlClose, `</span>`)
	return s
}

// isCJK 判断需要按字切分的字符（汉字/假名/谚文）。
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}

// segmentCJK 在 CJK 字符边界插入空格：CJK-CJK、CJK-拉丁 边界均切开，
// 使 unicode61 得到单字粒度 token；已有空白不重复插。
func segmentCJK(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	var prev rune
	for i, r := range s {
		if i > 0 && !unicode.IsSpace(r) && !unicode.IsSpace(prev) && (isCJK(r) || isCJK(prev)) {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// joinCJK 移除两个 CJK 字符之间的单个空格（跳过高亮占位符找邻居），
// 还原 snippet 的自然中文排版。
func joinCJK(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		if r == ' ' {
			prev := adjacentRune(runes, i-1, -1)
			next := adjacentRune(runes, i+1, +1)
			if isCJK(prev) && isCJK(next) {
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// adjacentRune 从 idx 沿 step 方向找第一个非高亮占位符的字符；越界返回 0。
func adjacentRune(runes []rune, idx, step int) rune {
	for i := idx; i >= 0 && i < len(runes); i += step {
		if runes[i] != '\x01' && runes[i] != '\x02' {
			return runes[i]
		}
	}
	return 0
}
