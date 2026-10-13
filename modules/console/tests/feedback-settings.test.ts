import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

function read(relativePath: string): string {
  return readFileSync(join(repoRoot, relativePath), "utf8");
}

// v1.7.1：Feedback 组件设置从「主题设置 → 内容增强」一节迁出，
// 成为「系统设置」专属 tab，并补服务连通性自测与入口位置示意。

test("SettingsView registers a dedicated feedback tab wired to the feedback section", () => {
  const code = read("modules/console/web/src/modules/settings/SettingsView.vue");
  // tab 定义：键、文案、图标
  assert.match(code, /\{\s*key:\s*"feedback",\s*label:\s*"反馈",\s*icon:\s*"ri:feedback-line"/);
  // 分段 → tab 的脏标记映射
  assert.match(code, /feedback:\s*\["feedback"\]/);
  // 组件导入与渲染分支
  assert.match(code, /import FeedbackSettings from "\.\/tabs\/FeedbackSettings\.vue"/);
  assert.match(code, /<FeedbackSettings[\s\S]*?:data="data"[\s\S]*?@save="save"/);
  // legacy 分段键改道到本 tab（不再落到主题设置）
  assert.match(code, /feedback:\s*\{\s*path:\s*"\/settings",\s*tab:\s*"feedback"\s*\}/);
});

test("FeedbackSettings tab owns the full feedback form, health check and launcher preview", () => {
  const code = read("modules/console/web/src/modules/settings/tabs/FeedbackSettings.vue");
  // 完整字段集
  for (const id of [
    "settings-feedback-api-base",
    "settings-feedback-app-id",
    "settings-feedback-app-name",
    "settings-feedback-app-version",
    "settings-feedback-launcher-mode",
    "settings-feedback-capture-mode",
    "settings-feedback-side",
    "settings-feedback-launcher-bottom",
  ]) {
    assert.match(code, new RegExp(`id="${id}"`), `missing field ${id}`);
  }
  // 显示范围：front|console|both 三选一（v1.7.4 新增 display 字段）
  assert.match(code, /id="settings-feedback-display"/);
  assert.match(code, /v-model="data\.feedback\.display"/);
  assert.match(code, /value="front"/);
  assert.match(code, /value="console"/);
  assert.match(code, /value="both"/);
  // 启用开关与保存走既有 section 保存契约
  assert.match(code, /v-model="data\.feedback\.enabled"/);
  assert.match(code, /emit\("save",\s*"feedback"\)/);
  // 连接测试：提交草稿 api_base 到 check 端点，渲染可达性徽标
  assert.match(code, /settingsApi\.checkFeedback\(base\)/);
  assert.match(code, /feedback-check-badge/);
  // 草稿改动后旧结论失效（stale 提示）
  assert.match(code, /checkStale/);
  // 入口位置示意：side/mode 类 + bottom 定位
  assert.match(code, /feedback-stage-launcher/);
  assert.match(code, /normalizedBottom/);
  assert.match(code, /is-\$\{data\.feedback\.side === 'left' \? 'left' : 'right'\}/);
});

test("feedback form is moved out of appearance enhancements (migration, not duplication)", () => {
  const code = read("modules/console/web/src/modules/appearance/EnhancementsSettings.vue");
  assert.doesNotMatch(code, /settings-feedback-api-base/);
  assert.doesNotMatch(code, /saveFeedback/);
  const view = read("modules/console/web/src/modules/appearance/AppearanceView.vue");
  // 脏跟踪与分组不再包含 feedback（数组项形态）；?tab=feedback 旧链接改道 /settings
  assert.match(view, /CanonicalSettingsNotice/);
  assert.doesNotMatch(view, /^\s*"feedback",\s*$/m);
  assert.match(view, /route\.query\.tab === "feedback"/);
  assert.match(view, /router\.replace\(\{\s*path:\s*"\/settings",\s*query:\s*\{\s*tab:\s*"feedback"\s*\}\s*\}\)/);
});

test("feedback check api contract: named response type generated for Console", () => {
  const client = read("packages/api-client/src/api/settings.ts");
  assert.match(client, /checkFeedback:\s*\(apiBase:\s*string\)/);
  assert.match(client, /\/settings\/feedback\/check/);
  const gen = read("packages/api-client/src/generated.d.ts");
  assert.match(gen, /export interface FeedbackCheckResponse/);
  assert.match(gen, /latency_ms: number/);
});

test("backend check endpoint is admin-scoped, validates scheme and never reflects bodies", () => {
  const api = read("modules/console/api/settings.go");
  // 具名 struct（非 gin.H）+ 仅回传 status/latency
  assert.match(api, /type FeedbackCheckResponse struct/);
  assert.match(api, /Endpoint\s+string/);
  // scheme + host + userinfo 校验
  assert.match(api, /u\.Scheme != "http" && u\.Scheme != "https"/);
  assert.match(api, /u\.User != nil/);
  // 4s 超时、不跟随重定向、丢弃响应体
  assert.match(api, /Timeout:\s*4 \* time\.Second/);
  assert.match(api, /ErrUseLastResponse/);
  assert.match(api, /io\.Copy\(io\.Discard/);
  const router = read("modules/console/api/router.go");
  // 挂在 sys（admin）组
  assert.match(router, /sys\.POST\("\/settings\/feedback\/check", h\.checkFeedback\)/);
});

// v1.7.4：feedback.display 决定组件挂载位置；Console 侧由 FeedbackLauncher
// 在登录后布局挂载同一 <feedback-widget>（ESM 运行时注册，不经构建）。
test("console mounts <feedback-widget> in DefaultLayout when display targets console", () => {
  const launcher = read("modules/console/web/src/components/FeedbackLauncher.vue");
  // 挂载条件镜像 settings.FeedbackConfig.ShowOnConsole：enabled+api_base+app_id+display
  assert.match(launcher, /settingsApi\.all\(\)/);
  assert.match(launcher, /!f\.enabled \|\| !f\.api_base \|\| !f\.app_id/);
  assert.match(launcher, /d === "console" \|\| d === "both"/);
  // 与前台 base.html 一致的属性集（side/theme/launcher-mode/capture-mode/launcher-bottom）
  assert.match(launcher, /api-base=/);
  assert.match(launcher, /app-id=/);
  assert.match(launcher, /theme="dark"/);
  assert.match(launcher, /launcher-mode/);
  // ESM 入口与前台同一路径（构建期不打包，运行时注入 script）
  assert.match(launcher, /\/assets\/vendor\/feedback\/feedback-web\.js/);
  assert.match(launcher, /document\.createElement\("script"\)/);

  const layout = read("modules/console/web/src/layouts/DefaultLayout.vue");
  assert.match(layout, /import FeedbackLauncher from "@\/components\/FeedbackLauncher\.vue"/);
  assert.match(layout, /<FeedbackLauncher \/>/);

  const vite = read("modules/console/web/vite.config.ts");
  // <feedback-widget> 为 Custom Element，避免 Vue 当作未注册组件告警
  assert.match(vite, /isCustomElement.*feedback-widget/s);
});

test("feedback display field round-trips through settings-state defaults", () => {
  const state = read("modules/console/web/src/modules/settings/settings-state.ts");
  assert.match(state, /d\.feedback\.display = d\.feedback\.display \|\| "front"/);

  const model = read("internal/settings/model.go");
  assert.match(model, /Display\s+string\s+`yaml:"display" json:"display"`/);
  assert.match(model, /func \(c FeedbackConfig\) ShowOnFront\(\) bool/);
  assert.match(model, /func \(c FeedbackConfig\) ShowOnConsole\(\) bool/);
});
