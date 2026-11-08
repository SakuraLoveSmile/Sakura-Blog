package routes

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// wantsHTML 判断请求是否来自浏览器的页面导航：Accept 明确包含 text/html。
// curl、API 客户端和脚本的 Accept 通常是 */* 或 application/json，仍得到原来的 JSON 错误。
func wantsHTML(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept"), "text/html")
}

// stellarMigrationPendingHTML 是旧数据尚未迁移到 Stellar 2.0 时浏览器看到的说明页。
// 页面只含固定文案，不拼接任何请求或站点数据；配色取自本地设计库的语义色（浅色 Silver Mist、深色 Space Black）。
const stellarMigrationPendingHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex">
<title>站点内容正在升级</title>
<style>
:root{color-scheme:light dark;--bg:#f5f5f7;--card:#fff;--fg:#1d1d1f;--muted:#6e6e73;--border:#d2d2d7}
@media (prefers-color-scheme:dark){:root{--bg:#0a0a0a;--card:#161618;--fg:#f5f5f7;--muted:#a1a1a6;--border:#3a3a3c}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px;background:var(--bg);color:var(--fg);font:16px/1.6 -apple-system,BlinkMacSystemFont,"DM Sans","PingFang SC","Noto Sans SC",sans-serif}
main{width:100%;max-width:520px;padding:32px;background:var(--card);border:1px solid var(--border);border-radius:19px}
h1{margin:0 0 12px;font-size:22px;font-weight:600;letter-spacing:-.01em}
p{margin:0 0 12px;color:var(--muted);font-size:15px}
.admin{margin:20px 0 0;padding-top:16px;border-top:1px solid var(--border);font-size:14px}
code{font:13px ui-monospace,SFMono-Regular,Menlo,monospace}
</style>
</head>
<body>
<main>
<h1>站点内容正在升级</h1>
<p>这个站点的部分内容还在迁移到 Stellar 2.0，文章、文库、专栏和笔记本等前台页面暂时无法访问，请稍后再试。</p>
<p class="admin">站点管理员：请在服务器上审阅迁移预览并执行迁移，完成后重启服务即可恢复。错误代码 <code>stellar_migration_required</code>。</p>
</main>
</body>
</html>
`
