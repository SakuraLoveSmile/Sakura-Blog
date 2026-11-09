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

test("SettingsView protects against concurrent duplicate saves and isolates section snapshots", () => {
  const code = read("modules/console/web/src/modules/settings/SettingsView.vue");
  // 防并发重复提交锁
  assert.match(code, /if\s*\(savingKey\.value\)\s*return\s*false;/);
  // 精确按段更新快照，避免保存当前段时抹除其他段的新编辑
  assert.match(code, /updateSectionSnapshot\(section,\s*body\)/);
  // 保持路由离开与页面关闭告警
  assert.match(code, /onBeforeRouteLeave/);
  assert.match(code, /beforeunload/);
});

test("退役的主题设置重定向到 canonical 设置页，保留 Feedback 旧链接", () => {
  const redirect = read("modules/console/web/src/router/retired-pages.ts");
  assert.match(redirect, /appearance: \{ tab: "appearance", legacyTabs: \{ feedback: "feedback" \} \}/);
  const core = read("modules/console/web/src/modules/core/module.ts");
  assert.match(core, /retiredConsoleRedirect\("appearance", to\.query\.tab\)/);
  assert.doesNotMatch(core, /modules\/appearance\/AppearanceView\.vue/);
  const notice = read("modules/console/web/src/modules/settings/CanonicalSettingsNotice.vue");
  assert.match(notice, /stellarSettingsApi\.get\(\)/);
  assert.match(notice, /stellar_migration_required/);
  assert.match(notice, /先审阅完整迁移预览/);
  assert.match(notice, /tab: props.tab/);
});

test("样式 YAML 整段编辑器已退役：代码与原文页只保留完整配置原文", () => {
  // ThemeYamlTab 是 12 页结构化设置接管前的整段 YAML 保存页，已无任何模块挂载。
  // 它的职责由「代码与原文」页的完整配置原文（可校验、按 section 保存）承接。
  assert.throws(
    () => read("modules/console/web/src/modules/settings/ThemeYamlTab.vue"),
    /ENOENT/,
    "ThemeYamlTab.vue 已退役，不应重新出现",
  );
  const raw = read("modules/console/web/src/modules/settings/SettingsRawEditor.vue");
  assert.match(raw, /Stellar2Tab/);
  assert.match(raw, /完整配置原文/);
});

test("AppearanceTab displays effective default color and inherit/override state badges", () => {
  const code = read("modules/console/web/src/modules/settings/AppearanceTab.vue");
  // 继承默认值与已覆盖标记
  assert.match(code, /settings-color-badge-inherit/);
  assert.match(code, /settings-color-badge-override/);
  assert.match(code, /恢复默认/);
  // 颜色选择器绑定有效回退色，杜绝留空时默认降级纯黑 #000000
  assert.match(code, /data\.style\.color\?\.theme\s*\|\|\s*'#2E8DFF'/);
  assert.match(code, /data\.style\.color\?\.accent\s*\|\|\s*'#ff9800'/);
  assert.match(code, /data\.style\.color\?\.link\s*\|\|\s*'#2196f3'/);
});

test("Console API and client use named struct responses for style YAML", () => {
  const apiCode = read("modules/console/api/settings.go");
  assert.match(apiCode, /type StyleYAMLResponse struct/);
  assert.match(apiCode, /type StyleYAMLUpdateResponse struct/);
  assert.match(apiCode, /StyleYAMLResponse\{YAML:\s*string\(b\)\}/);

  const clientCode = read("packages/api-client/src/api/settings.ts");
  assert.match(clientCode, /getStyleYAML:\s*\(\)\s*=>\s*http\.get<StyleYAMLResponse>/);
  assert.match(clientCode, /updateStyleYAML:\s*\(yamlText:\s*string\)\s*=>\s*http\.put<StyleYAMLUpdateResponse>/);
});
