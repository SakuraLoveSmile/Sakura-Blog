# 写作助手的视觉依据

权威任务记录：`docs/plans/editor-writing-assistant.md`（BLG-20261106-01）。

- `before-editor-1280.png`、`before-editor-390.png`：改动前的文章编辑页（富文本模式，桌面右栏打开），唯一参照图。拍摄时用的是临时正文，文字与 `after-*` 不同，对比的是版式。
- `after-editor-1280.png`、`after-editor-390.png`：加入入口后、面板关闭时的同一页面（富文本）。`after-editor-390` 的正文是撑出滚动高度的占位段落。入口只有图标，占掉工具栏固定区一个图标的宽度。
- `after-editor-wide-1280.png`：收起右栏、工具栏变宽后，入口带上「写作助手」文字，格式工具也全部露出。
- `after-panel-390.png`：手机上的非模态底部面板，位于底部标签栏之上；未选中文字时只有「续写」与自定义指令可用。
- `after-panel-800.png`：800 宽下的底部面板（靠右的浮层卡片），右栏内容没有被挤出。
- `after-diff-1280.png`：桌面右栏里一次润色的结果，逐词对比（删除线 + 高亮），以及「替换选区 / 复制 / 重试 / 放弃」。
- `after-diff-390.png`：手机上生成结束后，对比结果与「替换选区」已自动滚进面板的可见区域。
- `after-error-1280.png`：未配置对话模型时的错误提示与「重试」，指向「设置 → AI」。

所有页面与数据都来自隔离的本地 e2e 站点；流式响应由 Playwright 在页面里注入，不是真实模型的输出，也不作为生产证据。`after-*.png` 由 `tests/e2e/writing-assistant.spec.ts` 在设置 `ASSISTANT_SHOTS_DIR` 时写出，用的是最终源码；手机截图是 390×844 视口模拟，不是真机。
