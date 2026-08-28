# Page 首个标签卡片间距基线

## 唯一参照

- `reference-xaoxuu-about.png`：上游 xaoxuu Stellar `/about/` 页面（源码主线提交 `1f4cb4bc0a44e829b6d0929ce4102aea9cd9a2a8`）。
- 本任务只对比横幅底部与正文首个标签卡片之间的外部留白；横幅内部视觉不在范围内。

## 验收清单

- [x] `page` 页面横幅后的首个直接标签卡片有 `--gap-p` 的逻辑上边距，不再与横幅贴合。
- [x] 首个块是普通段落、文章页或无横幅时，现有布局保持不变。
- [x] 现有 page poetry → quot 间距仍为 16px。
- [x] 桌面与移动视口均无水平溢出。
