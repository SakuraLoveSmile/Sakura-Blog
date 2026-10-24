package routes

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"blog/internal/store"
)

type stellarInventory struct {
	moments   []*store.Moment
	posts     []*store.Post
	pages     []*store.Page
	projects  []*store.WikiProject
	topics    []*store.Topic
	notebooks []*store.Notebook
	docs      map[string][]*store.WikiDoc
	notes     map[string][]*store.Note
	comments  map[string][]*store.Comment
	boosts    map[string]map[int64][]*store.CommentBoost
}

// 整站公开输入由一个只读事务读取；渲染前结束事务，不把连接带入标签或模板。
func (s *webServer) readStellarInventory(ctx context.Context, include *store.Post) (*stellarInventory, error) {
	tx, err := s.st.DB().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	in := &stellarInventory{docs: map[string][]*store.WikiDoc{}, notes: map[string][]*store.Note{}, comments: map[string][]*store.Comment{}, boosts: map[string]map[int64][]*store.CommentBoost{}}
	read := func(query string, consume func(*sql.Rows) error) error {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := consume(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	if err := read(`SELECT `+strings.Join((&store.Post{}).Columns(), ",")+` FROM posts WHERE status='published' AND deleted_at='' AND (publish_at='' OR publish_at IS NULL OR publish_at <= strftime('%Y-%m-%dT%H:%M:%SZ','now')) ORDER BY substr(created_at,1,19) DESC,id DESC`, func(rows *sql.Rows) error {
		p := &store.Post{}
		if err := p.ScanRow(rows); err != nil {
			return err
		}
		in.posts = append(in.posts, p)
		return nil
	}); err != nil {
		return nil, err
	}
	// 显式分享草稿在同一事务中重新读取，撤回/删除后的旧对象不能再回填。
	if include != nil && include.ID > 0 && include.Status != store.StatusPublished {
		found := false
		for _, p := range in.posts {
			if p.ID == include.ID {
				found = true
				break
			}
		}
		if !found {
			p := &store.Post{}
			err := p.ScanRow(tx.QueryRowContext(ctx, `SELECT `+strings.Join(p.Columns(), ",")+` FROM posts WHERE id=? AND deleted_at=''`, include.ID))
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if err == nil {
				in.posts = append(in.posts, p)
			}
		}
	}
	if err := read(`SELECT `+strings.Join((&store.Page{}).Columns(), ",")+` FROM pages WHERE status='published' AND deleted_at='' ORDER BY id`, func(rows *sql.Rows) error {
		p := &store.Page{}
		if err := p.ScanRow(rows); err != nil {
			return err
		}
		in.pages = append(in.pages, p)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT id,visible,stellar_version,stellar_config,blog_extensions FROM wiki_projects ORDER BY id`, func(rows *sql.Rows) error {
		p := &store.WikiProject{}
		if err := rows.Scan(&p.ID, &p.Visible, &p.StellarVersion, &p.StellarConfig, &p.BlogExtensions); err != nil {
			return err
		}
		in.projects = append(in.projects, p)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT `+strings.Join((&store.Topic{}).Columns(), ",")+` FROM topics ORDER BY id`, func(rows *sql.Rows) error {
		p := &store.Topic{}
		if err := p.ScanRow(rows); err != nil {
			return err
		}
		in.topics = append(in.topics, p)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT id,stellar_version,stellar_config,blog_extensions FROM notebooks ORDER BY id`, func(rows *sql.Rows) error {
		p := &store.Notebook{}
		if err := rows.Scan(&p.ID, &p.StellarVersion, &p.StellarConfig, &p.BlogExtensions); err != nil {
			return err
		}
		in.notebooks = append(in.notebooks, p)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT `+strings.Join((&store.WikiDoc{}).Columns(), ",")+` FROM wikidocs WHERE status='published' ORDER BY id`, func(rows *sql.Rows) error {
		p := &store.WikiDoc{}
		if err := p.ScanRow(rows); err != nil {
			return err
		}
		in.docs[p.ProjectID] = append(in.docs[p.ProjectID], p)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT `+strings.Join((&store.Note{}).Columns(), ",")+` FROM notes WHERE status='published' ORDER BY id`, func(rows *sql.Rows) error {
		p := &store.Note{}
		if err := p.ScanRow(rows); err != nil {
			return err
		}
		in.notes[p.NotebookID] = append(in.notes[p.NotebookID], p)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT `+strings.Join((&store.Moment{}).Columns(), ",")+` FROM moments WHERE visibility='public' ORDER BY created_at DESC,id DESC`, func(rows *sql.Rows) error {
		m := &store.Moment{}
		if err := m.ScanRow(rows); err != nil {
			return err
		}
		in.moments = append(in.moments, m)
		return nil
	}); err != nil {
		return nil, err
	}
	byPost := map[int64]*store.Post{}
	for _, p := range in.posts {
		byPost[p.ID] = p
	}
	var hasAuthors int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='post_authors'`).Scan(&hasAuthors); err != nil {
		return nil, err
	}
	if hasAuthors > 0 {
		if err := read(`SELECT pa.post_id,u.username,pa.author_id FROM post_authors pa JOIN users u ON u.id=pa.author_id ORDER BY pa.post_id,pa.sort_order`, func(rows *sql.Rows) error {
			var id, author int64
			var name string
			if err := rows.Scan(&id, &name, &author); err != nil {
				return err
			}
			if p := byPost[id]; p != nil {
				p.Authors = append(p.Authors, name)
				p.AuthorIDs = append(p.AuthorIDs, author)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	type category struct {
		name   string
		parent int64
	}
	categories := map[int64]category{}
	if err := read(`SELECT id,name,parent_id FROM categories`, func(rows *sql.Rows) error {
		var id, parent int64
		var name string
		if err := rows.Scan(&id, &name, &parent); err != nil {
			return err
		}
		categories[id] = category{name, parent}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT post_id,category_id FROM post_categories ORDER BY post_id,position`, func(rows *sql.Rows) error {
		var id, categoryID int64
		if err := rows.Scan(&id, &categoryID); err != nil {
			return err
		}
		p := byPost[id]
		if p == nil {
			return nil
		}
		parts := []string{}
		seen := map[int64]bool{}
		for categoryID != 0 {
			if seen[categoryID] {
				return fmt.Errorf("category parent cycle")
			}
			seen[categoryID] = true
			c, exists := categories[categoryID]
			if !exists {
				return fmt.Errorf("category parent missing")
			}
			parts = append([]string{c.name}, parts...)
			categoryID = c.parent
		}
		p.Categories = append(p.Categories, strings.Join(parts, "/"))
		return nil
	}); err != nil {
		return nil, err
	}
	if err := read(`SELECT id,views FROM posts`, func(rows *sql.Rows) error {
		var id, views int64
		if err := rows.Scan(&id, &views); err != nil {
			return err
		}
		if p := byPost[id]; p != nil {
			p.Views = views
		}
		return nil
	}); err != nil {
		return nil, err
	}
	columns := map[string]bool{}
	if err := read(`PRAGMA table_info(comments)`, func(rows *sql.Rows) error {
		var cid, notnull, pk int
		var name, kind string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
			return err
		}
		columns[name] = true
		return nil
	}); err != nil {
		return nil, err
	}
	projection := (&store.Comment{}).Columns()
	for i, col := range projection {
		if !columns[col] && (col == "user_id" || col == "verified" || col == "likes") {
			projection[i] = "0 AS " + col
		}
	}
	if err := read(`SELECT `+strings.Join(projection, ",")+` FROM comments WHERE status='approved' ORDER BY substr(created_at,1,19),id`, func(rows *sql.Rows) error {
		comment := &store.Comment{}
		if err := comment.ScanRow(rows); err != nil {
			return err
		}
		in.comments[comment.PageKey] = append(in.comments[comment.PageKey], comment)
		return nil
	}); err != nil {
		return nil, err
	}
	var hasBoosts int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='comment_boosts'`).Scan(&hasBoosts); err != nil {
		return nil, err
	}
	if hasBoosts > 0 {
		if err := read(`SELECT c.page_key,b.id,b.comment_id,b.emoji,b.text,b.user_id,b.nick,b.email,b.ip_hash,b.identity_key,b.created_at FROM comment_boosts b JOIN comments c ON c.id=b.comment_id WHERE c.status='approved' ORDER BY b.comment_id,b.created_at,b.id`, func(rows *sql.Rows) error {
			b := &store.CommentBoost{}
			var page string
			if err := rows.Scan(&page, &b.ID, &b.CommentID, &b.Emoji, &b.Text, &b.UserID, &b.Nick, &b.Email, &b.IPHash, &b.IdentityKey, &b.CreatedAt); err != nil {
				return err
			}
			if in.boosts[page] == nil {
				in.boosts[page] = map[int64][]*store.CommentBoost{}
			}
			in.boosts[page][b.CommentID] = append(in.boosts[page][b.CommentID], b)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return in, nil
}
