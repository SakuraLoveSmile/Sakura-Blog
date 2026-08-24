import type { PluginMeta, PluginPreviewInfo } from "./plugin-types";

// ==================== 预览能力清单（Spec 63） ====================
// 每个插件一条预览描述，供共享预览运行时（preview/）按能力加载资源与 Fixture。
// 静态插件的预览描述由 previewInfoFor 兜底（capability=static），无需逐条登记。

const STATIC_PLUGIN_PREVIEW: PluginPreviewInfo = {
 capability: "static",
};

export const PLUGIN_PREVIEW: Record<string, PluginPreviewInfo> = {
 // ---- 布局容器：交互初始化 ----
 folding: {
  capability: "interactive",
  mainJs: true,
  height: 180,
  fallback: "折叠块为原生 details 结构，无需脚本即可展开/收起。",
 },
 tabs: {
  capability: "interactive",
  mainJs: true,
  height: 220,
  fallback: "Tab 切换依赖前台 main.js，加载失败时仅显示第一个标签页。",
 },
 gallery: {
  capability: "interactive",
  height: 320,
  fallback: "图片墙展示本地示例图；灯箱（fancybox）资源不可用时点击无放大效果。",
 },
 folders: {
  capability: "interactive",
  mainJs: true,
  height: 260,
  fallback: "折叠组为原生 details 结构，无需脚本即可展开/收起。",
 },
 navbar: {
  capability: "interactive",
  mainJs: true,
  height: 160,
  fallback: "导航栏高亮依赖前台脚本，加载失败时仅显示静态链接。",
 },
 grid: {
  capability: "static",
  height: 240,
  fallback: "信息卡为原生网格结构，列宽/间距/圆角由样式直接呈现。",
 },
 // ---- 媒体播放：Swiper ----
 swiper: {
  capability: "interactive",
  services: ["swiper"],
  height: 320,
  fallback: "Swiper 资源加载失败时轮播不可滑动，图片仍按列表展示。",
 },
 // ---- 数据 service + 本地 Fixture ----
 albums: {
  capability: "service",
  services: ["friends"],
  fixture: "friends",
  height: 300,
  fallback: "专辑数据依赖 ds-friends 服务与本地 Fixture，加载失败时显示空态。",
 },
 posters: {
  capability: "service",
  services: ["friends"],
  fixture: "friends",
  height: 300,
  fallback: "海报数据依赖 ds-friends 服务与本地 Fixture，加载失败时显示空态。",
 },
 rating: {
  capability: "service",
  services: ["rating"],
  fixture: "rating",
  height: 180,
  fallback: "评分数据为预览 Fixture，不写入真实统计；加载失败时显示 0 分。",
 },
 vote: {
  capability: "service",
  services: ["vote"],
  fixture: "vote",
  height: 180,
  fallback: "投票数据为预览 Fixture，不写入真实统计；加载失败时显示 0 票。",
 },
 friends: {
  capability: "service",
  services: ["friends"],
  fixture: "friends",
  height: 320,
  fallback: "友链数据为本地 Fixture；加载失败时显示空态。",
 },
 sites: {
  capability: "service",
  services: ["sites"],
  fixture: "sites",
  height: 320,
  fallback: "站点数据为本地 Fixture；加载失败时显示空态。",
 },
 // ---- GitHub 卡片：服务端确定性 Fixture ----
 ghcard: {
  capability: "service",
  fixture: "gh",
  height: 200,
  fallback: "GitHub 卡片使用服务端确定性 Fixture，不发公网请求。",
 },
 ghrepo: {
  capability: "service",
  fixture: "gh",
  height: 200,
  fallback: "仓库卡片使用服务端确定性 Fixture，不发公网请求。",
 },
 ghuser: {
  capability: "service",
  fixture: "gh",
  height: 200,
  fallback: "用户卡片使用服务端确定性 Fixture，不发公网请求。",
 },
 // ---- 数据图表 ----
 chat: {
  capability: "static",
  height: 360,
  fallback: "聊天记录为服务端渲染结构，头像为本地素材。",
 },
 // ---- 外部资源：Mermaid / 外部 Markdown ----
 mermaid: {
  capability: "external",
  services: ["mermaid"],
  height: 320,
  fallback: "Mermaid 为本地 vendor（11.16.1），加载失败时保留图表源码并显示本提示。",
 },
 md: {
  capability: "external",
  services: ["mdrender"],
  fixture: "md",
  height: 320,
  fallback: "外部 Markdown 为本地 Fixture；marked 为本地 vendor，加载失败时显示本提示。",
 },
 // ---- 布局容器 / 文艺装饰 / 提示标注：数据上下文说明 ----
 timeline: {
  capability: "static",
  height: 260,
  fallback: "时间轴内容来自示例正文，无需外部数据。",
 },
 toc: {
  capability: "static",
  height: 160,
  fallback: "预览展示 TOC 容器骨架；真实文章页由前台脚本填充目录项。",
 },
 emoji: {
  capability: "static",
  height: 120,
  fallback: "表情素材为本地资源；未收录的表情名显示占位。",
 },
 image: {
  capability: "static",
  height: 300,
  fallback: "示例图片为本地资源；手动填入的外链图片在无网络时显示占位。",
 },
 video: {
  capability: "static",
  height: 320,
  fallback: "示例视频为本地资源（体积受控）。",
 },
 audio: {
  capability: "static",
  height: 120,
  fallback: "示例音频为本地资源（体积受控）。",
 },
};

/** 取插件预览描述：未登记项按静态插件处理（只注入 CSS + HTML）。 */
export function previewInfoFor(p: Pick<PluginMeta, "name">): PluginPreviewInfo {
 return PLUGIN_PREVIEW[p.name] || STATIC_PLUGIN_PREVIEW;
}
