# BLG-20261111-02 验收基线：友链多角标

参照图：`reference.webp`（用户提供，友链卡片挂「社区先锋」「老友」两枚角标）。
下表是本次可复核的验收项；截图均由本地隔离站点（`-dev`、独立数据目录、1280/390 视口）实拍。

| # | 验收项 | 预期 | 证据 |
| --- | --- | --- | --- |
| 1 | Console 列表展示多枚角标 | 同一条友链的 3 枚角标按顺序显示，浅色角标用深色字 | `after-console-list-1280.webp` |
| 2 | 编辑弹窗可维护 0–3 枚 | 每行文案 + 色板 + Hex；已 3 枚时“添加角标”禁用 | `after-console-editor-1280.webp`、`after-console-editor-bottom-1280.webp` |
| 3 | 保存按序落库 | 第 1 枚写回 `label/color`，第 2、3 枚写 `labels` JSON | 浏览器脚本日志：删掉第 3 枚后接口返回 `["社区先锋","老友"]` |
| 4 | 前台 `/friends` 渲染 | 3 枚 / 2 枚 / 1 枚三种数据都按顺序显示，颜色与后台一致 | `after-friends-page-1280.webp` |
| 5 | 手机宽度 | 角标换行不溢出，编辑弹窗无横向滚动 | `after-friends-page-390.webp`、`after-console-editor-390.webp`（`scrollWidth == clientWidth`） |

说明：本次前台截图使用主题默认浅色外观；深色外观的角标配色由同一份 `friends_and_posts.js`
按颜色亮度选择文字颜色，未单独截图。
