# 设置效果展示的视觉依据

权威任务记录：`docs/plans/settings-effects.md`（BLG-20261025-02）。

- `reference.png`：基线 e05aec52 的真实设置页，1168×778，唯一视觉参照。
- `after-appearance-reference-viewport.png`：当前实现，同一内置浏览器、同一视口的外观页。
- `after-appearance-1280.png`、`after-appearance-390.png`：当前真实构建的桌面/手机分类导读和外观选择。
- `after-choices-390.png`：手机滚动到选择区的视图。
- `after-comments-*.png`：评论开关效果说明。
- `draft-preview.png`：未保存草稿的手机视口预览。
- `browser-draft-preview.png`：主 agent 在内置浏览器选择浅色玻璃外观后的真实本地草稿预览。
- `article-images-preview.png`：隔离 E2E 的真实文章横幅与正文图片，验证静态预览不会因禁用脚本而缺图。
- `verification.json`：本次源码指纹、检查结果、当前预览进程和 HTTP/磁盘资产哈希、截图及日志哈希。

选择卡里的简图属于风格示意；iframe 中为真实只读前台渲染。所有页面和数据均来自本次隔离本地环境，不作为生产已发布证据。
