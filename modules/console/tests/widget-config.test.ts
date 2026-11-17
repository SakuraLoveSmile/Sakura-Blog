// 仪表盘组件实例配置：纯函数行为 + registry 声明自洽。
// 参照 Halo「每个 widget 实例带自己的 config」的思路，字段与默认值均为本项目自有。
//
// 注意：`registry.ts` 会 import 组件 `.vue`，node:test 无法加载，因此
// 行为测试针对 `widget-config.ts`，registry 侧只做文本级声明校验。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  configFieldOptions,
  configInt,
  resolveWidgetConfig,
} from "../web/src/modules/dashboard/widgets/widget-config.ts";
import { WIDGET_TYPES } from "../web/src/modules/dashboard/widgets/layout.ts";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");
const dashboardRoot = join(repoRoot, "modules/console/web/src/modules/dashboard");
const registrySource = readFileSync(join(dashboardRoot, "widgets/registry.ts"), "utf8");

interface ParsedField {
  key: string;
  type: string;
  options: string[];
}
interface ParsedWidget {
  id: string;
  defaultConfig: Record<string, unknown>;
  fields: ParsedField[];
}

/** 从 registry 源码里取出每个组件的 id / defaultConfig / configFields 骨架。 */
function parseRegistry(): ParsedWidget[] {
  const widgets: ParsedWidget[] = [];
  const blocks = registrySource.split("defineWidget({").slice(1);
  for (const block of blocks) {
    const idMatch = block.match(/^\s*id: "([^"]+)",/);
    if (!idMatch) continue;
    const id = idMatch[1];
    const configMatch = block.match(/defaultConfig: (\{[^\n]*\}),/);
    let defaultConfig: Record<string, unknown> = {};
    if (configMatch) {
      // 源码里是简单字面量（数字/字符串/布尔/字符串数组），转成 JSON 后可解析。
      const literal = configMatch[1].replace(/(\w+):/g, '"$1":').replace(/'/g, '"');
      defaultConfig = JSON.parse(literal);
    }
    const fields: ParsedField[] = [];
    const fieldSection = block.split("configFields: [")[1];
    if (fieldSection) {
      for (const fieldBlock of fieldSection.split("key: ").slice(1)) {
        const key = fieldBlock.match(/^"([^"]+)"/)?.[1];
        const type = fieldBlock.match(/type: "([^"]+)"/)?.[1];
        if (!key || !type) continue;
        const options = [...fieldBlock.matchAll(/value: "([^"]+)"/g)].map((m) => m[1]);
        fields.push({ key, type, options });
      }
    }
    widgets.push({ id, defaultConfig, fields });
  }
  return widgets;
}

const parsed = parseRegistry();

test("registry 里每个 configField 都在 defaultConfig 中有默认值", () => {
  assert.ok(parsed.length >= 17, `解析到的组件数过少：${parsed.length}`);
  for (const widget of parsed) {
    for (const field of widget.fields) {
      assert.ok(
        field.key in widget.defaultConfig,
        `${widget.id}.${field.key} 缺少 defaultConfig 默认值`,
      );
    }
  }
});

test("configField 的 key 不重复，select 必须有候选项", () => {
  for (const widget of parsed) {
    const keys = widget.fields.map((field) => field.key);
    assert.equal(new Set(keys).size, keys.length, `${widget.id} 存在重复 configField key`);
    for (const field of widget.fields) {
      if (field.type === "select") {
        assert.ok(field.options.length > 0, `${widget.id}.${field.key} 是 select 但没有候选项`);
      }
    }
  }
});

test("select 类配置的默认值必须是字符串，与候选项 value 类型一致", () => {
  // 反例：默认写数字 0、选项写字符串 "0"，用户不改该项时回读会与该选项失配。
  for (const widget of parsed) {
    for (const field of widget.fields) {
      if (field.type !== "select") continue;
      const value = widget.defaultConfig[field.key];
      // 多值 select（叠放容器的子组件列表）存字符串数组，每个都必须命中候选项。
      if (Array.isArray(value)) {
        for (const item of value) {
          assert.equal(typeof item, "string", `${widget.id}.${field.key} 数组项应为字符串`);
          assert.ok(
            field.options.includes(item as string),
            `${widget.id}.${field.key} 的默认项 ${String(item)} 不在候选项中`,
          );
        }
        continue;
      }
      assert.equal(
        typeof value,
        "string",
        `${widget.id}.${field.key} 的默认值应为字符串，实际是 ${typeof value}`,
      );
      if (field.options.length > 0) {
        assert.ok(
          field.options.includes(value as string),
          `${widget.id}.${field.key} 的默认值 ${String(value)} 不在候选项 ${JSON.stringify(field.options)} 中`,
        );
      }
    }
  }
});

test("叠放容器的候选项都是已注册组件，且不含自身", () => {
  const stack = parsed.find((widget) => widget.id === "stack");
  assert.ok(stack, "registry 缺少 stack 组件");
  assert.ok(WIDGET_TYPES.includes("stack"), "stack 必须登记进 WIDGET_TYPES");
  const stackField = stack.fields.find((field) => field.key === "stackWidgets");
  assert.ok(stackField, "stack 缺少 stackWidgets 配置项");
  assert.ok(!stackField.options.includes("stack"), "叠放容器不能把自己作为子组件");
  // `recent-moments` 由 moments 模块在运行时通过 syncPluginWidgets 注册，
  // 不属于静态 WIDGET_TYPES；模块关闭时叠放容器按 entries 过滤会静默跳过。
  const runtimeContributed = new Set(["recent-moments"]);
  for (const option of stackField.options) {
    if (runtimeContributed.has(option)) continue;
    assert.ok(
      WIDGET_TYPES.includes(option),
      `叠放容器候选项 ${option} 不是已注册组件`,
    );
  }
});

test("defaultConfig 里声明的键都被 configFields 覆盖（避免哑配置）", () => {
  for (const widget of parsed) {
    if (!widget.fields.length) continue;
    const declared = new Set(widget.fields.map((field) => field.key));
    for (const key of Object.keys(widget.defaultConfig)) {
      assert.ok(declared.has(key), `${widget.id} 的 defaultConfig.${key} 没有对应 configField`);
    }
  }
});

test("resolveWidgetConfig 用默认值兜底，忽略未声明的历史键", () => {
  const definition = {
    id: "demo",
    defaultConfig: { entryCount: 4 },
    configFields: [
      { key: "entryCount", label: "入口数", type: "select" as const, options: ["2", "4"] },
      { key: "showUv", label: "UV", type: "boolean" as const },
    ],
  };
  const fallback = resolveWidgetConfig(definition as never, undefined);
  assert.deepEqual(fallback, { entryCount: 4 });

  const withJunk = resolveWidgetConfig(definition as never, { entryCount: "2", legacyUnknown: "x" });
  assert.equal(withJunk.entryCount, "2");
  assert.ok(!("legacyUnknown" in withJunk), "未声明的键进入了渲染配置");
});

test("resolveWidgetConfig 按字段类型收敛值", () => {
  const definition = {
    id: "demo",
    defaultConfig: { showUv: false, itemLimit: 5, title: "x" },
    configFields: [
      { key: "showUv", label: "UV", type: "boolean" as const },
      { key: "itemLimit", label: "条数", type: "number" as const },
      { key: "title", label: "标题", type: "text" as const },
    ],
  };
  // 字符串 "yes" 不是 true，布尔字段必须严格收敛。
  assert.equal(resolveWidgetConfig(definition as never, { showUv: "yes" }).showUv, false);
  assert.equal(resolveWidgetConfig(definition as never, { showUv: true }).showUv, true);
  // 数字字段接受可转换的字符串。
  assert.equal(resolveWidgetConfig(definition as never, { itemLimit: "8" }).itemLimit, 8);
  // 非法数字不覆盖默认值。
  assert.equal(resolveWidgetConfig(definition as never, { itemLimit: "abc" }).itemLimit, 5);
});

test("resolveWidgetConfig 不修改调用方的默认对象", () => {
  const definition = {
    id: "demo",
    defaultConfig: { stackWidgets: ["a", "b"] },
    configFields: [{ key: "stackWidgets", label: "子组件", type: "select" as const }],
  };
  const before = JSON.stringify(definition.defaultConfig);
  const resolved = resolveWidgetConfig(definition as never, { stackWidgets: ["c"] });
  assert.deepEqual(resolved.stackWidgets, ["c"]);
  assert.equal(JSON.stringify(definition.defaultConfig), before, "默认 config 被就地修改");
});

test("configFieldOptions 兼容纯字符串与 {value,label} 两种写法", () => {
  assert.deepEqual(
    configFieldOptions({ key: "a", label: "A", type: "select", options: ["x", { value: "y", label: "Y" }] }),
    [
      { value: "x", label: "x" },
      { value: "y", label: "Y" },
    ],
  );
  assert.deepEqual(configFieldOptions({ key: "b", label: "B", type: "boolean" }), []);
});

test("configInt 钳制范围并回落默认值", () => {
  assert.equal(configInt({ n: "8" }, "n", 4, 1, 10), 8);
  assert.equal(configInt({ n: "99" }, "n", 4, 1, 10), 10);
  assert.equal(configInt({ n: "-5" }, "n", 4, 1, 10), 1);
  assert.equal(configInt({ n: "abc" }, "n", 4, 1, 10), 4);
  assert.equal(configInt(undefined, "n", 4), 4);
});

test("消费 config 的组件都声明了 config prop", () => {
  // 统计卡目前没有 configFields，因此不在消费清单里。
  const consumers = [
    "widgets/TrendChartWidget.vue",
    "widgets/QuickActionsWidget.vue",
    "widgets/RecentPostsWidget.vue",
    "widgets/RecentCommentsWidget.vue",
    "widgets/TopPostsWidget.vue",
    "widgets/StackWidget.vue",
  ];
  for (const relativePath of consumers) {
    const source = readFileSync(join(dashboardRoot, relativePath), "utf8");
    assert.match(source, /config\??: Record<string, unknown>/, `${relativePath} 缺少 config prop 声明`);
  }
});

/** 递归收集 dashboard 目录下的 Vue 组件。 */
function collectVue(dir: string, files: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === "dist") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) collectVue(full, files);
    else if (entry.endsWith(".vue")) files.push(full);
  }
  return files;
}

test("组件不用模块级变量保存实例配置", () => {
  for (const file of collectVue(join(dashboardRoot, "widgets"))) {
    const source = readFileSync(file, "utf8");
    const rel = relative(repoRoot, file);
    assert.doesNotMatch(source, /^let .*config/m, `${rel} 用模块级变量保存了 config`);
  }
});

test("默认布局里叠放容器至多出现一次", () => {
  const layoutSource = readFileSync(join(dashboardRoot, "widgets/layout.ts"), "utf8");
  const matches = layoutSource.match(/defaultWidget\("stack"/g) ?? [];
  assert.ok(matches.length <= 1, `默认布局里 stack 出现了 ${matches.length} 次`);
});
