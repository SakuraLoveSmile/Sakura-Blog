# Page 横幅对齐验收基线

唯一视觉参照：`reference-xaoxuu-about.png`，来源为 xaoxuu Stellar 上游 About 页截图。
`before-blog-page.png` 仅记录本项目改造前状态，不作为目标样式。

## 验收清单

- [x] 普通 Page 输出与上游相同的 `article-banner-wrap > article.banner.top > content` 结构。
- [x] 可编辑背景图、横幅标题、副标题、头像/图标、面包屑、作者和发布日期均能保存并在前台回显。
- [x] 横幅标题为空时回退页面标题；头像同时存在时优先显示头像。
- [x] `breadcrumb=false` 只隐藏横幅顶部面包屑和元信息，不隐藏标题区。
- [x] 横幅后的正文紧邻横幅，不新增全局 margin 或 padding；正文自身的 `<br>` 才产生局部留白。
- [x] `friends`、`links`、`notes` 与普通 Page 走同一横幅数据链，旧 Friends Hero 配置只迁移一次。
- [x] 深色模式下 1440×900 与 390×844 均无横向溢出，横幅文字、头像和遮罩不越界。
