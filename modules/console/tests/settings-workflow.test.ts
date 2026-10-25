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

test("主题设置停止旧主题写入并引导 canonical，保留 Feedback 旧链接", () => {
  const code = read("modules/console/web/src/modules/appearance/AppearanceView.vue");
  assert.match(code, /CanonicalSettingsNotice/);
  assert.doesNotMatch(code, /settingsApi\.updateSection/);
  assert.match(code, /route\.query\.tab === "feedback"/);
  const notice = read("modules/console/web/src/modules/settings/CanonicalSettingsNotice.vue");
  assert.match(notice, /stellarSettingsApi\.get\(\)/);
  assert.match(notice, /stellar_migration_required/);
  assert.match(notice, /先审阅完整迁移预览/);
  assert.match(notice, /tab: props.tab/);
});

test("ThemeYamlTab correctly exposes loading/saving states, error feedback, and dirty guards", () => {
  const code = read("modules/console/web/src/modules/settings/ThemeYamlTab.vue");
  // 重命名为样式 YAML
  assert.match(code, /保存样式 YAML/);
  assert.doesNotMatch(code, /保存主题 YAML/);
  // 不吞掉错误，使用 Toast.error 展示明确明细
  assert.match(code, /Toast\.error\(getApiErrorMessage/);
  assert.doesNotMatch(code, /catch\s*\{\s*\}/);
  // 防重复提交与保存中状态（本地保存状态更名为 localSaving，避免与页面级 saving 键混淆）
  assert.match(code, /if\s*\(localSaving\.value\)\s*return;/);
  assert.match(code, /localSaving\.value\s*=\s*true/);
  // 必须接线页面级 saving 键，才能参与全局保存禁用
  assert.match(code, /defineProps<\{ saving\?: string \}>/);
  assert.match(code, /:disabled="loading \|\| localSaving \|\| Boolean\(props\.saving\)"/);
  // 脏检查与离开守卫
  assert.match(code, /isDirty\s*=\s*computed/);
  assert.match(code, /onBeforeRouteLeave/);
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
