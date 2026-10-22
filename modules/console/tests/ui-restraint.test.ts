import { test } from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

/**
 * 编辑器域边界保留清单：依 Approved Goal 边界约束，不得修改以下编辑器文件。
 */
const EDITOR_BOUNDARY_FILES = new Set([
  "modules/console/web/src/modules/posts/PostEditor.vue",
  "modules/console/web/src/modules/pages/PageEditor.vue",
  "modules/console/web/src/components/MarkdownEditor.vue",
  "modules/console/web/src/components/RichTextEditor.vue",
  "modules/console/web/src/components/ContentEditor.vue",
  "modules/console/web/src/components/editor/EditorToolbar.vue",
  "modules/console/web/src/modules/posts/EditorToolbar.vue",
  "modules/console/web/src/components/editor/EditorSidebar.vue",
  "modules/console/web/src/modules/posts/EditorSidebar.vue",
  "modules/console/web/src/components/editor/PageEditorSidebar.vue",
  "modules/console/web/src/modules/pages/PageEditorSidebar.vue",
  "modules/console/web/src/components/editor/AiGenerateModal.vue",
  "modules/console/web/src/composables/useLocalDraft.ts",
]);

/**
 * 功能性白名单：逐条明确原因，禁止未登记的装饰性例外。
 */
const WHITELIST: Record<string, { pattern: RegExp; reason: string }[]> = {
  // 编辑器域文件（按边界白名单全量豁免并标注原因）
  ...Array.from(EDITOR_BOUNDARY_FILES).reduce((acc, file) => {
    acc[file] = [
      { pattern: /.*/, reason: "BOUNDARY_EDITOR_PRESERVED: 编辑器域依边界规范保持原样" }
    ];
    return acc;
  }, {} as Record<string, { pattern: RegExp; reason: string }[]>),

  // 系统升级与监控功能性例外
  "modules/console/web/src/modules/system/SystemUpdateView.vue": [
    { pattern: /animate-pulse/, reason: "FUNCTIONAL_SYSTEM_UPDATE_BUSY: 系统检测升级中状态指示" }
  ],

  // 仪表盘设计器与小部件库功能性预览例外
  "modules/console/web/src/modules/dashboard/DashboardDesignerView.vue": [
    { pattern: /linear-gradient/, reason: "FUNCTIONAL_DESIGNER_GRID: 拖拽网格参考线渲染" }
  ],
  "modules/console/web/src/modules/dashboard/widgets/TrendChartWidget.vue": [
    { pattern: /linear-gradient|<linearGradient/, reason: "FUNCTIONAL_CHART_FILL: 趋势图表面积图填充渐变" }
  ],

  // 资源库图片棋盘格透明指示
  "modules/console/web/src/modules/attachments/AttachmentList.vue": [
    { pattern: /linear-gradient/, reason: "FUNCTIONAL_TRANSPARENT_CHECKERBOARD: PNG/SVG 透明背景棋盘格指示" }
  ],

  // 主题色板预览（用户自定义外观吸色）
  "modules/console/web/src/modules/settings/AppearanceTab.vue": [
    { pattern: /linear-gradient/, reason: "FUNCTIONAL_COLOR_PICKER_SPECTRUM: 调色盘全光谱色相条展示" },
    { pattern: /#ff9800|#2196f3/, reason: "FUNCTIONAL_UPSTREAM_THEME_FALLBACK: Stellar 前台主题默认强调色与链接色默认值" }
  ],
  "modules/console/web/src/modules/settings/settings-canonical-fields.ts": [
    { pattern: /default: \{ 探索号: "#f44336" \}/, reason: "FUNCTIONAL_UPSTREAM_CATEGORY_DEFAULT: 严格主题 schema 的分类颜色配置值，不是 Console 样式" },
    { pattern: /\["(?:manual|reviewed|polished|generated)",.*"#(?:03a9f4|4caf50|ff9800)"/, reason: "FUNCTIONAL_CONTENT_LABEL_DEFAULT: 前台 AI 创作标记的现有配置默认值，不是 Console 样式" }
  ],

  // 前台短代码组件可视化预览与渲染
  "modules/console/web/src/modules/plugins/ShortcodeStudioView.vue": [
    { pattern: /linear-gradient/, reason: "FUNCTIONAL_PREVIEW_FRAME: 短代码嵌入机模阴影与设备预览" }
  ],

  // 插件参数表单色相滑块
  "modules/console/web/src/components/forms/PluginParamForm.vue": [
    { pattern: /linear-gradient/, reason: "FUNCTIONAL_SPECTRUM_SLIDER: 参数色相范围条" }
  ],

  // 图标选择器 SVG 渐变
  "modules/console/web/src/components/IconPicker.vue": [
    { pattern: /linear-gradient|<linearGradient/, reason: "FUNCTIONAL_ICON_SWATCH: 图标库预览色样" }
  ]
};

/**
 * 递归收集源码文件
 */
function collectSourceFiles(dir: string, fileList: string[] = []): string[] {
  const entries = readdirSync(dir);
  for (const entry of entries) {
    if (entry.startsWith(".") || entry === "node_modules" || entry === "dist" || entry === "dist-uc") {
      continue;
    }
    const fullPath = join(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      collectSourceFiles(fullPath, fileList);
    } else if (/\.(vue|ts|css)$/.test(entry) && !entry.endsWith(".d.ts")) {
      fileList.push(fullPath);
    }
  }
  return fileList;
}

test("源码扫描守卫 · 禁止未登记的 backdrop-filter / backdrop-blur", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      if (/backdrop-filter|backdrop-blur/i.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的 backdrop-filter/backdrop-blur:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止未登记的渐变色与 SVG Gradient", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      if (/linear-gradient|radial-gradient|<linearGradient|<radialGradient/i.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的渐变色/SVG Gradient:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止 transition: all 与 transition-all", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      if (/transition\s*:\s*all\b|(?:\bclass="[^"]*\btransition-all\b)/i.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的 transition: all:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止未登记的 animate-pulse 呼吸动效", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      if (/\banimate-pulse\b/.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的 animate-pulse:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止未登记的 uppercase 英文大写修饰", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      if (/(?:\bclass="[^"]*\buppercase\b)|text-transform\s*:\s*uppercase/i.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的 uppercase 大写修饰:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止 .hover-lift 装饰性卡片位移与浮动", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      if (/\.hover-lift\b|(?:\bclass="[^"]*\bhover-lift\b)/i.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的 hover-lift:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止未登记的零偏移模糊发光与染色投影", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      // 0 0 Xpx (零偏移辉光发光)
      if (/box-shadow\s*:[^;]*?\b0\s+0\s+\d+px\b/i.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的零偏移发光阴影:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止源码中出现纯装饰性 emoji", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  const violations: string[] = [];
  // 匹配常见装饰性 Emoji，排除普通标点和符号
  const decorativeEmojiRegex = /[🎨🚀💡✨🔥🎉🤖👾🎯⚡💥🌟🌈👋💎🔮🛠️🕹️]/u;

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      if (decorativeEmojiRegex.test(line)) {
        const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
        if (!whitelisted) {
          violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未在白名单登记的纯装饰性 Emoji:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止越界未登记的裸 Hex 颜色（限定 Apple Design 深色调色板）", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  // 允许的 Apple Design 调色盘 Hex
  const ALLOWED_HEX = new Set([
    "#000000", "#000",
    "#ffffff", "#fff",
    "#0f0f10", "#101012",
    "#1c1c1e", "#2c2c2e", "#3a3a3c", "#48484a", "#6e6e73", "#8e8e93", "#aeaeb2", "#c7c7cc", "#d1d1d6", "#e5e5ea", "#f5f5f7",
    "#2e8dff", "#66abff", "#007aff", "#0064d6", "#004fad", "#9fcbff", "#cfe5ff", "#e8f2ff", "#003b82", "#00275a",
    "#30d158", "#34c759", "#e9f9ee",
    "#ff9f0a", "#ff9500",
    "#ff453a", "#ff3b30", "#ffecea",
    "#5e5ce6", "#5856d6",
    "#bf5af2", "#af52de",
    "#7c7c82", // text-quaternary
  ]);

  const violations: string[] = [];

  for (const file of targets) {
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    const lines = content.split("\n");

    lines.forEach((line, idx) => {
      // 提取行中所有 #hex
      const matches = line.match(/#[0-9a-fA-F]{3,8}\b/g);
      if (matches) {
        for (const m of matches) {
          const lower = m.toLowerCase();
          if (!ALLOWED_HEX.has(lower)) {
            const whitelisted = WHITELIST[rel]?.some(w => w.pattern.test(line));
            if (!whitelisted) {
              violations.push(`${rel}:${idx + 1}: ${m} (in line: ${line.trim()})`);
            }
          }
        }
      }
    });
  }

  assert.deepEqual(violations, [], `发现未登记的越界 Hex 颜色:\n${violations.join("\n")}`);
});
