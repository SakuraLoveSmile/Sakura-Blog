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

/**
 * 方向 A「编辑台」收敛范围：共享控件、仪表盘、列表页、设置、外观与设计令牌。
 * 规则只约束本次重构范围，避免把编辑器域等保留模块的既有实现一起判失败。
 */
const DIRECTION_A_SCOPE = [
  "modules/console/web/src/modules/dashboard",
  "modules/console/web/src/modules/settings",
  "modules/console/web/src/modules/appearance",
  "modules/console/web/src/components/AttachmentField.vue",
  "modules/console/web/src/layouts/DefaultLayout.vue",
  "modules/console/web/src/modules/attachments/AttachmentList.vue",
  "modules/console/web/src/modules/comments/CommentList.vue",
  "modules/console/web/src/modules/friends/FriendList.vue",
  "modules/console/web/src/modules/modules/ModulesView.vue",
  "modules/console/web/src/modules/moments/MomentList.vue",
  "modules/console/web/src/modules/notifications/NotificationsView.vue",
  "modules/console/web/src/modules/pages/PageList.vue",
  "modules/console/web/src/modules/plugins/AssetGallery.vue",
  "modules/console/web/src/modules/plugins/ScriptPluginListView.vue",
  "modules/console/web/src/modules/plugins/ShortcodeStudioView.vue",
  "modules/console/web/src/modules/posts/PostList.vue",
  "modules/console/web/src/modules/subscriptions/SubscriptionList.vue",
  "modules/console/web/src/modules/system/SystemUpdateView.vue",
  "modules/console/web/src/modules/taxonomy/TaxonomyList.vue",
  "modules/console/web/src/modules/topics/TopicsView.vue",
  "modules/console/web/src/modules/users/UserListView.vue",
  "modules/console/web/src/modules/wiki/WikiDocsView.vue",
  "packages/components/src/ui/VButton.vue",
  "packages/components/src/ui/VCard.vue",
  "packages/components/src/ui/VEmpty.vue",
  "packages/components/src/ui/VPageHeader.vue",
  "packages/components/src/ui/VStatusDot.vue",
  "packages/components/src/ui/VTabbar.vue",
  "packages/components/src/ui/VTag.vue",
  "modules/console/web/src/styles/main.css",
  "modules/console/web/src/styles/apple-base.css",
];

function collectDirectionAFiles(): string[] {
  const files: string[] = [];
  for (const entry of DIRECTION_A_SCOPE) {
    const fullPath = resolve(repoRoot, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) collectSourceFiles(fullPath, files);
    else files.push(fullPath);
  }
  return files;
}

const CARD_CLASS_RE = /\bclass="[^"]*(?<![\w-])(?:apple-card|settings-panel|dashboard-card|console-card)(?![\w-])/;
const VOID_TAGS = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"]);

/** 扫描 Vue template 中「卡片容器嵌套卡片容器」的结构，返回行号。 */
function scanCardNesting(content: string): { line: number; tag: string }[] {
  const start = content.indexOf("<template>");
  if (start < 0) return [];
  const end = content.lastIndexOf("</template>");
  const template = content.slice(start, end > start ? end : content.length);
  const violations: { line: number; tag: string }[] = [];
  const stack: string[] = [];
  const tagRe = /<(\/?)([A-Za-z][\w.-]*)\b([^>]*)>/g;
  let match: RegExpExecArray | null;
  while ((match = tagRe.exec(template))) {
    const [, closing, tag, attrs] = match;
    if (closing) {
      const index = stack.lastIndexOf(tag);
      if (index >= 0) stack.length = index;
      continue;
    }
    const isCard = tag === "VCard" || CARD_CLASS_RE.test(attrs);
    const selfClosing = /\/\s*$/.test(attrs);
    if (!isCard || selfClosing || VOID_TAGS.has(tag.toLowerCase())) continue;
    if (stack.length > 0) {
      const line = template.slice(0, match.index).split("\n").length + content.slice(0, start).split("\n").length - 1;
      violations.push({ line, tag });
    }
    stack.push(tag);
  }
  return violations;
}

test("方向 A 守卫 · 禁止卡片套卡片", () => {
  const violations: string[] = [];
  for (const file of collectDirectionAFiles()) {
    if (!file.endsWith(".vue")) continue;
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    for (const item of scanCardNesting(content)) {
      violations.push(`${rel}:${item.line}: <${item.tag}> 嵌套在卡片容器内`);
    }
  }
  assert.deepEqual(violations, [], `方向 A 要求单层卡片，发现嵌套卡片:\n${violations.join("\n")}`);
});

test("方向 A 守卫 · 页面标题不再渲染图标", () => {
  const headerPath = resolve(repoRoot, "packages/components/src/ui/VPageHeader.vue");
  const header = readFileSync(headerPath, "utf8");
  assert.doesNotMatch(header, /\bIcon\b/, "VPageHeader 不应再渲染标题图标");
  assert.doesNotMatch(header, /name="icon"/, "VPageHeader 不应保留 icon 插槽");

  // 口径覆盖 Console 全部源码，而不是只看本轮改动文件。
  const violations: string[] = [];
  for (const file of collectSourceFiles(resolve(repoRoot, "modules/console/web/src"))) {
    if (!file.endsWith(".vue")) continue;
    const content = readFileSync(file, "utf8");
    if (!content.includes("<VPageHeader")) continue;
    const start = content.indexOf("<VPageHeader");
    const close = content.indexOf("</VPageHeader>", start);
    const selfClose = content.indexOf("/>", start);
    const end =
      selfClose >= 0 && (close < 0 || selfClose < close)
        ? selfClose + 2
        : close >= 0
          ? close + "</VPageHeader>".length
          : content.length;
    const block = content.slice(start, end);
    const startTagEnd = block.indexOf(">");
    const startTag = startTagEnd >= 0 ? block.slice(0, startTagEnd + 1) : block;
    // 只禁止 VPageHeader 自身的 icon 属性与其直接子级 #icon 插槽；
    // actions 里 VButton 的 #icon 插槽属于按钮图标，不在此列。
    const inner = block.slice(startTag.length).trimStart();
    if (/\bicon\s*=/.test(startTag) || /^<template\s+#icon\b/.test(inner)) {
      violations.push(`${relative(repoRoot, file)}: VPageHeader 仍传入图标`);
    }
  }
  assert.deepEqual(violations, [], `方向 A 页面标题不使用图标:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止空 @media 块", () => {
  const violations: string[] = [];
  const emptyMedia = /@media[^{]*\{(?:\s*[^{}]*\{\s*\})?\s*\}/g;
  for (const file of collectSourceFiles(resolve(repoRoot, "modules/console/web/src"))) {
    if (!file.endsWith(".vue") && !file.endsWith(".css")) continue;
    const rel = relative(repoRoot, file);
    const content = readFileSync(file, "utf8");
    for (const match of content.matchAll(emptyMedia)) {
      const line = content.slice(0, match.index).split("\n").length;
      violations.push(`${rel}:${line}: ${match[0].replace(/\s+/g, " ").slice(0, 120)}`);
    }
  }
  assert.deepEqual(violations, [], `发现没有声明的空 @media 块:\n${violations.join("\n")}`);
});

/**
 * 状态底的逐文件功能性白名单：只放行既有页面的可达性/可见性标记，
 * 不把 `bg-[#hex]` 整体从规则里豁免。
 */
const STATUS_BG_ALLOWLIST: Record<string, { pattern: RegExp; reason: string }[]> = {
  "modules/console/web/src/layouts/DefaultLayout.vue": [
    { pattern: /bg-state-error/, reason: "FUNCTIONAL_UNREAD_BADGE: 未读通知数量角标" },
  ],
  "modules/console/web/src/modules/friends/FriendList.vue": [
    { pattern: /bg-state-error/, reason: "FUNCTIONAL_FRIEND_IMPORT_ERROR: 导入错误提示面板" },
  ],
  "modules/console/web/src/modules/system/SystemUpdateView.vue": [
    { pattern: /bg-state-(?:success|error)/, reason: "FUNCTIONAL_SYSTEM_UPDATE_STATUS: 升级状态与错误面板" },
  ],
};

/**
 * 内联背景色的功能性白名单：只保留数据驱动的头像/分组色、图表色与色彩预览。
 * 状态/角色胶囊的 inline backgroundColor 不允许在这里豁免。
 */
const INLINE_BG_ALLOWLIST: Record<string, RegExp[]> = {
  "modules/console/web/src/modules/friends/FriendList.vue": [/normalizeColor\(l\.color\)/],
  "modules/console/web/src/modules/dashboard/widgets/CategoryDonutWidget.vue": [/a\.color/],
  "modules/console/web/src/modules/dashboard/widgets/HourlyChartWidget.vue": [/bgColors/, /rgba\(28, 28, 30, 0\.92\)/],
  "modules/console/web/src/modules/settings/SettingColorEditor.vue": [/backgroundColor: preview/, /backgroundColor: color\.value/],
};

test("方向 A 守卫 · 状态只用圆点+文字，禁止彩色状态底", () => {
  const statusBg = /bg-state-(?:success|warning|error|info)\b|bg-(?:green|red|yellow|blue|amber|emerald|rose|orange)-[0-9]|bg-\[#[0-9a-f]{3,8}\](?:\/\d+)?/i;
  const allowance = /\b(?:unread|switch)\b/;
  const violations: string[] = [];
  for (const file of collectDirectionAFiles()) {
    if (!file.endsWith(".vue")) continue;
    const rel = relative(repoRoot, file);
    readFileSync(file, "utf8")
      .split("\n")
      .forEach((line, idx) => {
        // hover/focus 只是交互反馈，不是状态底色。
        const staticLine = line.replace(/\b(?:hover|focus|focus-visible|active|group-hover):bg-[^\s"']+/g, "");
        if (/\bbackgroundColor\s*:/.test(staticLine)) {
          const functional = INLINE_BG_ALLOWLIST[rel]?.some((pattern) => pattern.test(staticLine));
          if (!functional) violations.push(`${rel}:${idx + 1}: 内联状态背景色 ${line.trim()}`);
          return;
        }
        if (!statusBg.test(staticLine)) return;
        if (allowance.test(staticLine)) return;
        // 5px 圆点属于方向 A 允许的状态表达。
        if (/\brounded-full\b/.test(staticLine) && /\b[hw]-(?:1|1\.5|2|\[5px\])/.test(staticLine)) return;
        const functional = STATUS_BG_ALLOWLIST[rel]?.some((entry) => entry.pattern.test(staticLine));
        if (!functional) violations.push(`${rel}:${idx + 1}: ${line.trim()}`);
      });
  }
  assert.deepEqual(violations, [], `状态不得使用彩色背景，只保留圆点+文字:\n${violations.join("\n")}`);
});

// 56 是唯一新增的显示级字号：统一足迹后整行「大」档统计卡需要一个 hero 数字，
// 取自 Apple 设计库的 56 号标题档；其余字号仍限定在 11/12/13/14/20/24。
const ALLOWED_FONT_PX = new Set([11, 12, 13, 14, 20, 24, 56]);

function fontPx(value: string, unit: string): number {
  const number = Number(value);
  return unit === "rem" ? number * 16 : number;
}

const NAMED_FONT_PX: Record<string, number> = {
  xs: 12,
  sm: 14,
  base: 16,
  lg: 18,
  xl: 20,
  "2xl": 24,
  "3xl": 30,
  "4xl": 36,
  "5xl": 48,
  "6xl": 60,
};

function findFontViolations(content: string): string[] {
  const violations: string[] = [];
  let inIconTag = false;
  content.split("\n").forEach((line, idx) => {
    // 图标尺寸不是文字字号；Icon 组件（含多行属性）单独豁免。
    if (inIconTag) {
      if (/\/?>/.test(line)) inIconTag = false;
      return;
    }
    const iconStart = line.search(/<Icon\b/);
    if (iconStart >= 0) {
      const rest = line.slice(iconStart);
      if (!/\/>|>/.test(rest.slice("<Icon".length))) inIconTag = true;
      return;
    }
    // Tailwind 命名档按真实像素比对：xs=12 / sm=14 / xl=20 / 2xl=24 可用。
    for (const named of line.matchAll(/\btext-(xs|sm|base|lg|xl|[2-6]xl)\b/g)) {
      const px = NAMED_FONT_PX[named[1]];
      if (!ALLOWED_FONT_PX.has(px)) violations.push(`第 ${idx + 1} 行命名字号 ${named[0]}: ${line.trim()}`);
    }
    for (const pattern of [/text-\[([0-9.]+)(px|rem)\]/g, /font-size\s*:\s*([0-9.]+)(px|rem)/g]) {
      let match: RegExpExecArray | null;
      while ((match = pattern.exec(line))) {
        const px = fontPx(match[1], match[2]);
        if (!ALLOWED_FONT_PX.has(px)) violations.push(`第 ${idx + 1} 行字号 ${match[0]}: ${line.trim()}`);
      }
    }
  });
  return violations;
}

test("方向 A 守卫 · 字号白名单（11/12/13/14/20/24 + hero 56）", () => {
  const violations: string[] = [];
  for (const file of collectDirectionAFiles()) {
    const rel = relative(repoRoot, file);
    for (const item of findFontViolations(readFileSync(file, "utf8"))) {
      violations.push(`${rel}:${item}`);
    }
  }
  assert.deepEqual(violations, [], `方向 A 只允许 11/12/13/14/20/24 与 hero 56 字号:\n${violations.join("\n")}`);
});

const ALLOWED_RADIUS_PX = new Set([0, 6, 8, 10]);
const ALLOWED_NAMED_RADIUS = new Set(["none", "xs", "sm", "base", "md", "lg", "xl", "2xl", "3xl"]);
const RADIUS_TOKEN_RE = /^(?:var\([^)]*\)|inherit)$/;
/** 全圆/9999 的显式上下文白名单：头像、圆点、状态、开关、徽标、骨架、按钮变体。 */
const FULL_RADIUS_CONTEXT = /avatar|object-cover|avatar-gradient|dot|status|badge|unread|notif|switch|toggle|thumb|track|slider|checkbox|select|orb|skeleton|spinner|loading|progress|min-w-|btn-pill|btn-circle/i;

function fullRadiusAllowed(context: string): boolean {
  if (FULL_RADIUS_CONTEXT.test(context)) return true;
  // 头像图片本身就是圆形语义，不要求类名再写 avatar。
  if (/<img\b/.test(context)) return true;
  // 小尺寸圆点（1–2.5 档或 1–6px 任意值）是方向 A 允许的状态表达。
  if (/\b[hw]-(?:1|1\.5|2|2\.5)\b|\b[hw]-\[[1-6]px\]/.test(context)) return true;
  // CSS 中选择器附近声明了 5–10px 的几何尺寸，属于圆点/滑块手柄。
  return /\b(?:width|height)\s*:\s*(?:[5-9]|10)px\b/.test(context);
}

function radiusAllowed(raw: string, context: string): boolean {
  const value = raw.replace(/!important/g, "").trim();
  // calc() 从半径 token 派生，视为白名单。
  if (value.startsWith("calc(") && value.endsWith(")")) return true;
  // var(--radius-sm, 6px) 内部含空格，先折叠为一个整体再按四角拆词。
  const parts = value.replace(/var\([^)]*\)/g, "var()").split(/\s+/);
  // 支持四角简写，每个角都必须落在白名单。
  return parts.every((part) => {
    if (part === "0" || part === "0px" || part === "inherit") return true;
    if (part === "50%" || part === "9999px") return fullRadiusAllowed(context);
    if (RADIUS_TOKEN_RE.test(part)) return true;
    const px = part.match(/^([0-9.]+)(px|rem)$/);
    return Boolean(px && ALLOWED_RADIUS_PX.has(Number(px[1]) * (px[2] === "rem" ? 16 : 1)));
  });
}

function findRadiusViolations(content: string, rel: string): string[] {
  const violations: string[] = [];
  const lines = content.split("\n");
  lines.forEach((line, idx) => {
    // 选择器可能位于声明上方多行（如 ::before 圆点），取最近 8 行作为上下文。
    const context = `${rel}\n${lines.slice(Math.max(0, idx - 8), idx + 1).join("\n")}`;
    for (const pattern of [/rounded-\[([^\]]+)\]/g, /border-radius\s*:\s*([^;"]+)/g]) {
      let match: RegExpExecArray | null;
      while ((match = pattern.exec(line))) {
        if (!radiusAllowed(match[1], context)) violations.push(`第 ${idx + 1} 行圆角 ${match[0]}: ${line.trim()}`);
      }
    }
    // 命名档位同样映射到真实像素：xs/sm/base=6、md/lg/xl=8、2xl/3xl=10。
    // full 不再无条件放行，只允许命中头像/圆点/开关等显式上下文。
    for (const attr of line.matchAll(/\bclass="([^"]*)"/g)) {
      const namedRe = /\brounded(?!-\[)(?:-(?:t|r|b|l|tl|tr|br|bl))?(?:-([a-z0-9]+))?\b/g;
      let named: RegExpExecArray | null;
      while ((named = namedRe.exec(attr[1]))) {
        const tier = named[1];
        if (tier === "full") {
          if (!fullRadiusAllowed(context)) {
            violations.push(`第 ${idx + 1} 行全圆角缺少选择器白名单上下文 ${named[0]}: ${line.trim()}`);
          }
          continue;
        }
        if (tier === undefined || !ALLOWED_NAMED_RADIUS.has(tier)) {
          violations.push(`第 ${idx + 1} 行命名圆角档 ${named[0]}: ${line.trim()}`);
        }
      }
    }
  });
  return violations;
}

test("方向 A 守卫 · 圆角白名单（6/8/10 与 0、胶囊）", () => {
  const violations: string[] = [];
  for (const file of collectDirectionAFiles()) {
    const rel = relative(repoRoot, file);
    for (const item of findRadiusViolations(readFileSync(file, "utf8"), rel)) {
      violations.push(`${rel}:${item}`);
    }
  }
  assert.deepEqual(violations, [], `方向 A 圆角只允许控件 6 / 容器 8 / 浮层 10，全圆仅允许头像/圆点/开关等选择器白名单:\n${violations.join("\n")}`);
});

test("源码扫描守卫 · 禁止越界未登记的裸 Hex 颜色（限定 Apple Design 深色调色板）", () => {
  const targets = [
    ...collectSourceFiles(resolve(repoRoot, "modules/console/web/src")),
    ...collectSourceFiles(resolve(repoRoot, "packages/components/src/ui"))
  ];

  // 允许的 Apple Design 调色盘 Hex + 方向 A「编辑台」中性色阶
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

  /**
   * 方向 A 令牌只在定义文件与其直接消费文件放行，避免把全局白名单整体放宽。
   */
  const DIRECTION_A_HEX = new Set([
    "#0a0a0b", // background / sidebar
    "#121214", // card / muted
    "#151517", // popover
    "#141416", // secondary
    "#1b1b1e", // accent
    "#19191c", // sidebar-accent
    "#242428", // border / secondary-700
    "#37373d", // border-strong
    "#f4f4f6", // foreground
    "#e8e8ec", // primary
    "#d6d6dc", // primary-active
    "#b9b9bf", // text-secondary
    "#7e7e85", // text-tertiary
    "#6a6a71", // text-quaternary
    "#8b8b91", // muted-foreground
    "#32ade6", // chart-6
    "#ff6482", // chart-7
    "#64d2ff", // chart-8
  ]);
  const FILE_ALLOWED_HEX: Record<string, Set<string>> = {
    "modules/console/web/src/styles/main.css": DIRECTION_A_HEX,
    "packages/components/src/ui/VButton.vue": new Set(["#242428", "#e8e8ec", "#d6d6dc", "#f4f4f6", "#8b8b91"]),
    "modules/console/web/src/modules/dashboard/widgets/CategoryDonutWidget.vue": new Set(["#32ade6", "#ff6482", "#64d2ff"]),
  };

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
          if (!ALLOWED_HEX.has(lower) && !FILE_ALLOWED_HEX[rel]?.has(lower)) {
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
