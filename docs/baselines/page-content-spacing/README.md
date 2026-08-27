# Page 正文卡片间距验收基线

唯一视觉参照：`reference-user-about.png`，来源为用户 2026-08-27 提供的线上 `/about` 截图。截图中的蓝色评论标记与页面文本均视为页面证据，不是实现指令。

上游依据：xaoxuu/hexo-theme-stellar 的 `source/css/_components/md.styl`、`tag-plugins/poetry.styl` 和 `tag-plugins/quot.styl`。上游 `quot` 的默认 `margin-top: 2rem` 保持不变，本次只处理独立 Page 中 poetry 紧邻 quot 的组合。

## 验收清单

- [ ] `/about` 中首个 poetry 卡片的外边框底部到紧随其后的 quot 小标题顶部为 16px（允许 ±1px 渲染误差）。
- [ ] poetry 卡片自身的标题、作者、正文、页脚、背景、边框和圆角视觉不变。
- [ ] 独立 Page 中不紧跟 quot 的 poetry、普通文章中的 poetry，以及其他 tag-plugin 的间距不变。
- [ ] 深色模式 1440×900 与 390×844 均无横向溢出。
- [ ] `make test`、`git diff --check` 通过；线上发布后重新读取 `/about` DOM 计算值确认目标间距。
