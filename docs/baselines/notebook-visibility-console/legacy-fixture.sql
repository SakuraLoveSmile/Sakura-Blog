-- Stellar 2.0 之前的旧版笔记本布局，用来复现“站点尚未迁移”的状态（沙箱 B）。
-- 两张表的 DDL 与 internal/store/notebook_legacy_test.go 的夹具一致（该文件注明与真实 sqlite_master 声明相同）。
-- 数据是合成的：1 个笔记本 + 1 篇笔记，形状与用户旧库里“只有 1 篇测试笔记”的情形相同。
-- 必须先把本文件灌进一个全新的空库，再运行 scripts/e2e/prepare.go（启动迁移会识别并原样保留旧布局）；
-- 顺序反过来会因为 notebooks 表已存在而失败。步骤见同目录 README.md。

CREATE TABLE notebooks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	cover TEXT NOT NULL DEFAULT '',
	nb_order INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	notebook_id INTEGER NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	content_html TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (notebook_id) REFERENCES notebooks(id)
);

INSERT INTO notebooks (id, name, description, cover, nb_order, created_at)
VALUES (2, '1.40.0 验收笔记本', 'Console 与前台功能验证数据', '', 10, '2026-01-01 00:00:00');

INSERT INTO notes (id, notebook_id, title, content, content_html, tags, created_at, updated_at)
VALUES (
	1,
	2,
	'响应式检查记录（已更新）',
	'## 检查项' || char(10) || '- 375px' || char(10) || '- 768px' || char(10) || '- 1440px',
	'<h2>检查项</h2><ul><li>375px</li><li>768px</li><li>1440px</li></ul>',
	'responsive,console,1.40.0',
	'2026-01-01 00:00:00',
	'2026-01-01 00:00:00'
);
