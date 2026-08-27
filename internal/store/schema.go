package store

import "fmt"

// tableDDL 独立表建表 DDL 注册表（EnsureSchema/ensureTable 统一执行，幂等）。
// 值可含多条语句（分号分隔，SQLite Exec 支持）；posts/comments 两张核心表仍在
// post.go createTables 管理（含专用 ALTER 迁移），不在此注册。
// 各表如需补列迁移（ALTER），保留在对应 Ensure*Table 方法内（见下表注释）。
var tableDDL = map[string]string{
	"links": `CREATE TABLE IF NOT EXISTS links (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		group_title TEXT NOT NULL DEFAULT '',
		name        TEXT NOT NULL DEFAULT '',
		url         TEXT NOT NULL DEFAULT '',
		avatar      TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		rss         TEXT NOT NULL DEFAULT '',
		label       TEXT NOT NULL DEFAULT '',
		color       TEXT NOT NULL DEFAULT '',
		sort        INTEGER NOT NULL DEFAULT 0,
		snapshot    TEXT NOT NULL DEFAULT '',
		last_status INTEGER NOT NULL DEFAULT 0,
		last_checked_at INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_links_group ON links(group_title);`,

	"moments": `CREATE TABLE IF NOT EXISTS moments (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		images       TEXT NOT NULL DEFAULT '',
		visibility   TEXT NOT NULL DEFAULT 'public',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_moments_created ON moments(created_at);`,

	"shared_posts": `CREATE TABLE IF NOT EXISTS shared_posts (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		token      TEXT NOT NULL UNIQUE,
		post_slug  TEXT NOT NULL DEFAULT '',
		share_type TEXT NOT NULL DEFAULT 'published',
		expires_at TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_shared_posts_token ON shared_posts(token);
	CREATE INDEX IF NOT EXISTS idx_shared_posts_slug ON shared_posts(post_slug);`,

	"crawler_logs": `CREATE TABLE IF NOT EXISTS crawler_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		engine     TEXT NOT NULL DEFAULT '',
		ua         TEXT NOT NULL DEFAULT '',
		path       TEXT NOT NULL DEFAULT '',
		ip_hash    TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
	);
	CREATE INDEX IF NOT EXISTS idx_crawler_created ON crawler_logs(created_at);
	CREATE INDEX IF NOT EXISTS idx_crawler_engine_created ON crawler_logs(engine, created_at);`,

	"seo_push_logs": `CREATE TABLE IF NOT EXISTS seo_push_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		engine     TEXT NOT NULL DEFAULT '',
		url        TEXT NOT NULL DEFAULT '',
		status     TEXT NOT NULL DEFAULT '',
		message    TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
	);
	CREATE INDEX IF NOT EXISTS idx_push_created ON seo_push_logs(created_at);`,

	"feed_articles": `CREATE TABLE IF NOT EXISTS feed_articles (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		friend_title TEXT NOT NULL DEFAULT '',
		friend_url   TEXT NOT NULL DEFAULT '',
		title        TEXT NOT NULL DEFAULT '',
		link         TEXT NOT NULL DEFAULT '',
		summary      TEXT NOT NULL DEFAULT '',
		published    TEXT NOT NULL DEFAULT '',
		guid         TEXT NOT NULL DEFAULT '',
		fetched_at   TEXT NOT NULL DEFAULT '',
		read         INTEGER NOT NULL DEFAULT 0,
		favorite     INTEGER NOT NULL DEFAULT 0,
		later        INTEGER NOT NULL DEFAULT 0
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_feed_articles_guid ON feed_articles(guid);
	CREATE INDEX IF NOT EXISTS idx_feed_articles_friend ON feed_articles(friend_url);`,

	"module_states": `CREATE TABLE IF NOT EXISTS module_states (
		id         TEXT PRIMARY KEY,
		enabled    INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT ''
	);`,

	"roles": `CREATE TABLE IF NOT EXISTS roles (
		name        TEXT PRIMARY KEY,
		description TEXT NOT NULL DEFAULT ''
	);`,

	"permissions": `CREATE TABLE IF NOT EXISTS permissions (
		name        TEXT PRIMARY KEY,
		description TEXT NOT NULL DEFAULT ''
	);`,

	"role_permissions": `CREATE TABLE IF NOT EXISTS role_permissions (
		role       TEXT NOT NULL,
		permission TEXT NOT NULL,
		PRIMARY KEY (role, permission),
		FOREIGN KEY (role) REFERENCES roles(name) ON DELETE CASCADE,
		FOREIGN KEY (permission) REFERENCES permissions(name) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_role_permissions_permission ON role_permissions(permission);`,

	"user_roles": `CREATE TABLE IF NOT EXISTS user_roles (
		user_id    INTEGER NOT NULL,
		role       TEXT NOT NULL,
		is_primary INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, role),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (role) REFERENCES roles(name) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_user_roles_user_primary ON user_roles(user_id, is_primary DESC, role);`,

	"page_views": `CREATE TABLE IF NOT EXISTS page_views (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		path       TEXT NOT NULL DEFAULT '',
		referrer   TEXT NOT NULL DEFAULT '',
		ua_hash    TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
	);
	CREATE INDEX IF NOT EXISTS idx_pv_created ON page_views(created_at);
	CREATE INDEX IF NOT EXISTS idx_pv_path_created ON page_views(path, created_at);`,

	"revisions": `CREATE TABLE IF NOT EXISTS revisions (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		post_slug  TEXT NOT NULL DEFAULT '',
		title      TEXT NOT NULL DEFAULT '',
		excerpt    TEXT NOT NULL DEFAULT '',
		content_md TEXT NOT NULL DEFAULT '',
		tags       TEXT NOT NULL DEFAULT '',
		category   TEXT NOT NULL DEFAULT '',
		status     TEXT NOT NULL DEFAULT '',
		word_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_revisions_slug_created ON revisions(post_slug, created_at);`,

	"ratings": `CREATE TABLE IF NOT EXISTS ratings (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		target_id  TEXT NOT NULL DEFAULT '',
		score      INTEGER NOT NULL DEFAULT 0,
		voter_hash TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_ratings_target_voter ON ratings(target_id, voter_hash);
	CREATE INDEX IF NOT EXISTS idx_ratings_target ON ratings(target_id);`,

	"votes": `CREATE TABLE IF NOT EXISTS votes (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		target_id  TEXT NOT NULL DEFAULT '',
		value      TEXT NOT NULL DEFAULT '',
		voter_hash TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_target_voter ON votes(target_id, voter_hash);
	CREATE INDEX IF NOT EXISTS idx_votes_target ON votes(target_id);`,

	"attachments": `CREATE TABLE IF NOT EXISTS attachments (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		filename   TEXT NOT NULL DEFAULT '',
		path       TEXT NOT NULL DEFAULT '',
		url        TEXT NOT NULL DEFAULT '',
		mime       TEXT NOT NULL DEFAULT '',
		size       INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		thumb_path TEXT NOT NULL DEFAULT '',
		thumb_url  TEXT NOT NULL DEFAULT '',
		width      INTEGER NOT NULL DEFAULT 0,
		height     INTEGER NOT NULL DEFAULT 0,
		owner_id   INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_attachments_created ON attachments(created_at);`,

	"attachment_groups": `CREATE TABLE IF NOT EXISTS attachment_groups (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL DEFAULT '',
		"order"    INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,

	"wikidocs": `CREATE TABLE IF NOT EXISTS wikidocs (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id   TEXT NOT NULL,
		slug         TEXT NOT NULL,
		title        TEXT NOT NULL DEFAULT '',
		doc_order    INTEGER NOT NULL DEFAULT 0,
		parent       INTEGER NOT NULL DEFAULT 0,
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'published',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, slug)
	);
	CREATE INDEX IF NOT EXISTS idx_wikidocs_project ON wikidocs(project_id, parent);`,

	"wiki_projects": `CREATE TABLE IF NOT EXISTS wiki_projects (
		id                  TEXT PRIMARY KEY,
		name                TEXT NOT NULL DEFAULT '',
		title               TEXT NOT NULL DEFAULT '',
		headline            TEXT NOT NULL DEFAULT '',
		subtitle            TEXT NOT NULL DEFAULT '',
		description         TEXT NOT NULL DEFAULT '',
		icon                TEXT NOT NULL DEFAULT '',
		cover               TEXT NOT NULL DEFAULT '',
		available           TEXT NOT NULL DEFAULT '',
		url                 TEXT NOT NULL DEFAULT '',
		repo                TEXT NOT NULL DEFAULT '',
		branch              TEXT NOT NULL DEFAULT '',
		sort                INTEGER NOT NULL DEFAULT 0,
		pin                 INTEGER NOT NULL DEFAULT 0,
		visible             INTEGER NOT NULL DEFAULT 1,
		hero_json           TEXT NOT NULL DEFAULT '',
		created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_wiki_projects_sort ON wiki_projects(visible, pin DESC, sort DESC, id);
	CREATE TABLE IF NOT EXISTS wiki_project_tags (
		project_id TEXT NOT NULL,
		tag        TEXT NOT NULL,
		position   INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (project_id, tag)
	);
	CREATE INDEX IF NOT EXISTS idx_wiki_project_tags_tag ON wiki_project_tags(tag, position, project_id);`,

	"topics": `CREATE TABLE IF NOT EXISTS topics (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE,
		slug        TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		icon        TEXT NOT NULL DEFAULT '',
		cover       TEXT NOT NULL DEFAULT '',
		sort        INTEGER NOT NULL DEFAULT 0,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_topics_slug ON topics(slug);`,

	"users": `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		avatar TEXT NOT NULL DEFAULT '',
		totp_secret TEXT NOT NULL DEFAULT '',
		totp_recovery_codes TEXT NOT NULL DEFAULT '[]',
		auth_secret TEXT NOT NULL DEFAULT '',
		api_token_hash TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT 'author',
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,

	"login_logs": `CREATE TABLE IF NOT EXISTS login_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL DEFAULT 0,
		username TEXT NOT NULL DEFAULT '',
		ip TEXT NOT NULL DEFAULT '',
		ua TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_login_logs_created ON login_logs(created_at);
	CREATE INDEX IF NOT EXISTS idx_login_logs_status ON login_logs(status);`,

	"settings": `CREATE TABLE IF NOT EXISTS settings (
		section TEXT PRIMARY KEY,
		value   TEXT NOT NULL DEFAULT ''
	);`,

	"user_preferences": `CREATE TABLE IF NOT EXISTS user_preferences (
		user_id          INTEGER NOT NULL DEFAULT 0,
		preference_group TEXT NOT NULL,
		value            TEXT NOT NULL DEFAULT '',
		updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, preference_group)
	);`,

	"notifications": `CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL DEFAULT 0,
		type TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		link TEXT NOT NULL DEFAULT '',
		read INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, read, created_at);`,

	"pages": `CREATE TABLE IF NOT EXISTS pages (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		slug               TEXT NOT NULL UNIQUE,
		title              TEXT NOT NULL DEFAULT '',
		content_md         TEXT NOT NULL DEFAULT '',
		content_html       TEXT NOT NULL DEFAULT '',
		status             TEXT NOT NULL DEFAULT 'draft',
		leftbar_widgets  TEXT NOT NULL DEFAULT '',
		rightbar_widgets TEXT NOT NULL DEFAULT '',
		created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at         TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_pages_status ON pages(status);
	CREATE INDEX IF NOT EXISTS idx_pages_created ON pages(created_at);`,

	"perf_samples": `CREATE TABLE IF NOT EXISTS perf_samples (
		id     INTEGER PRIMARY KEY AUTOINCREMENT,
		ts     TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		metric TEXT NOT NULL DEFAULT '',
		value  REAL NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_perf_samples_metric_ts ON perf_samples(metric, ts);`,

	"perf_requests": `CREATE TABLE IF NOT EXISTS perf_requests (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		minute_ts TEXT NOT NULL DEFAULT '',
		method    TEXT NOT NULL DEFAULT '',
		route     TEXT NOT NULL DEFAULT '',
		count     INTEGER NOT NULL DEFAULT 0,
		total_ms  REAL NOT NULL DEFAULT 0,
		max_ms    REAL NOT NULL DEFAULT 0,
		err_count INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_perf_requests_minute ON perf_requests(minute_ts);
	CREATE INDEX IF NOT EXISTS idx_perf_requests_route ON perf_requests(route);`,
}

// EnsureSchema 统一建表入口：按注册表执行全部独立表 DDL（幂等）。
// 每张表建表 + 索引语句原子执行；单表失败即返回错误。
// 建表后执行追加迁移（旧库缺列时 ADD COLUMN）——含 pages 表左右栏字段与软删列。
func (s *Store) EnsureSchema() error {
	for table := range tableDDL {
		if err := s.ensureTable(table); err != nil {
			return err
		}
	}
	if err := s.EnsurePagesTable(); err != nil {
		return err
	}
	return nil
}

// ensureTable 执行单表 DDL（含建表与索引，幂等）。
func (s *Store) ensureTable(table string) error {
	ddl, ok := tableDDL[table]
	if !ok {
		return fmt.Errorf("ensure table %s: not registered", table)
	}
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create %s table: %w", table, err)
	}
	return nil
}
