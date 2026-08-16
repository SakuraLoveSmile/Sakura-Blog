# Boost 行为约束

视觉唯一参照为三张入库截图：`boost-btn-reference.png`（火箭入口：实心灰色斜向火箭）、`boost-composer-reference.png`（胶囊编辑器闭合态：头像外圈 + Boost 占位 + 描边笑脸 + 绿环勾 + 红环叉）、`boost-picker-reference.png`（表情选择器展开态：搜索框 + 右置选中表情 + 彩色分类栏 + 底部 chevron + 大网格）。本文件只描述不随视觉调整改变的行为边界。

每条评论的 actions 行有一个火箭入口。点击后出现单行胶囊编辑器，表情入口展开本地 Unicode 表情选择器，短文本最多 16 个 Unicode 字符。每个 Boost 以独立 chip 挂在评论正文下方，chip 依次展示发布者头像、表情和短文本。每楼最多 50 条；不能 Boost 自己的评论；发布者可以删除自己的 Boost。当前身份已经 Boost 时，火箭入口移动到 chip 条末尾；达到 50 条后隐藏火箭入口。

验收只对照 `boost-btn-reference.png`、`boost-composer-reference.png`、`boost-picker-reference.png` 和 `spec.md` 中的逐项清单。
