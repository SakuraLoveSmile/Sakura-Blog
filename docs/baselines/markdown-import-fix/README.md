# Markdown 旧版文章导入修复验收

唯一现状参照：用户提供的 `before.png`，备份页导入含 1.x 字段的文章后出现重复的 Stellar v2 校验错误提示。

本次差异：沿用备份页与现有 `Dialog.warning`，检测到旧版字段时提供“转换并导入”确认；转换仍须用户明确选择。复用现有按钮与语义 token。

验收项：

- [x] 普通导入检测到旧字段后提示 1.x 转换，不出现重复错误 Toast。
- [x] 取消转换不会写入文章；重新选择同一文件可继续导入。
- [x] 确认后使用同一上传文件转换；包含 `mathjax`、`mermaid`、`type`、`references`、`topic`、字符串 `banner` 的文章可完整保存。
- [x] 新旧字段冲突或转换失败时整批不写入；新文章未指定状态时保存为草稿。
- [x] 显式勾选转换与普通 2.0 导入均可使用；普通格式错误只提示一次。
- [x] 真实构建与隔离服务下核验桌面、390px 窄屏弹窗，并核对当前 Console 资源 hash。

本次状态：待体验。2026-10-20 本地验证，尚未发布或部署。

实测依据：

- 新回归用例在修改前失败：转换器报 `mathjax: render.math`；API 返回不可操作的通用 `validation_error`。
- `go test ./internal/stellarmigration ./internal/backup -count=1` 与 API 导入专项测试通过，包含旧开关启用/关闭、tech/story、目标冲突、既有数据和空库路径。
- `make test`、`make build` 及受影响前端 ESLint 通过；独立只读审查未发现本次变更的实质问题。
- `tests/e2e/backup-import.spec.ts` 的 6 项真实浏览器流程通过。首轮其余 4 项通过，新增两项在成功导入后因 Playwright 不提供 multipart `postData()` 而失败；移除该非业务断言后，仅重跑这两项，成功读取保存的完整元数据和正文。没有降低导入、取消、发布状态及数据保留断言。
- 桌面 1280×900 与窄屏 390×900 截图分别为 `after-conversion-1280.png` 和 `after-conversion-390.png`，主 agent 已查看并核验文字换行与操作区。
- 真实构建后的隔离服务 `127.0.0.1:8120` 返回的 Console HTML 和 6 个入口资源均与当前 `dist` 逐字节一致。入口 SHA-256：`d55dba6809d22fd537ae443216f6291dea6681e99efb8d37936a571f250eb261`。

用户只提供了截图；本次使用包含同样六种旧字段的文档复现和验证，未获得原文章文件。所有导入使用独立测试根目录、配置、数据库、内容和上传目录。
