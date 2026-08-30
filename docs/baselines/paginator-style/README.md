# 分页器样式修复基线

## 唯一参照

- 页面：`http://localhost:8080/`
- 参照视口：`904 × 795`
- 参照来源：用户在首页分页器上的浏览器评论（评论标记选中 `.paginator-wrap`）
- 上游实现：`resource/hexo-theme-stellar/layout/_partial/main/post_list/paginator.ejs`
- 上游主题版本：`1f4cb4bc0a44e829b6d0929ce4102aea9cd9a2a8`

## 修复前观察

- `.paginator-wrap` 的计算尺寸为 `512 × 62px`。
- `.paginator` 是唯一子元素且为普通块元素，宽度约 `68px`；下一页箭头因此掉到第二行。
- 修复前截图：`before.png`。

## 验收清单

- [x] `.paginator-wrap` 直接承载页码和前后翻页链接，不再嵌套 `nav.paginator`。
- [x] 页码与箭头在单行内紧凑排列，容器不被异常撑高。
- [x] 首页、瞬间页、作者页使用同一结构约定（由模板契约测试覆盖；当前瞬间/作者数据不足以生成运行时分页器）。
- [x] `904 × 795` 视口无横向溢出，页脚位置恢复正常。
