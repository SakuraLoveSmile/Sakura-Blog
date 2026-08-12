// Spec 61 阶段 1：ri 图标子集生成器。
// 扫描 frontend/console/src 中全部 `ri:<name>` 用法（.vue/.ts/.js/.json），
// 从 @iconify/json 的 ri.json 抽取实际用到的图标/别名，写入 console/src/icons/ri-subset.json。
// 用法：node frontend/scripts/gen-ri-subset.mjs（前置：frontend/console 已 npm ci）
import { readFileSync, writeFileSync, mkdirSync, readdirSync, statSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), ".."); // frontend/
const consoleDir = join(root, "console");
const srcDir = join(consoleDir, "src");
const riJsonPath = join(consoleDir, "node_modules", "@iconify/json", "json", "ri.json");

// 递归收集 src 下的可扫描文件（与 ri-subset.test.mjs 保持同一文件集合）
function collectFiles(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) collectFiles(p, out);
    else if (/\.(vue|ts|js|json)$/.test(name)) out.push(p);
  }
  return out;
}

const files = collectFiles(srcDir);
const ri = JSON.parse(readFileSync(riJsonPath, "utf8"));

// 提取全部 `ri:<name>`（单双引号皆可；本项目无动态拼接用法）
const used = new Set();
for (const f of files) {
  const text = readFileSync(f, "utf8");
  for (const m of text.matchAll(/ri:([a-z0-9-]+)/g)) used.add(m[1]);
}

// 构建子集：别名连同其父图标一并收入，addCollection 才能解析完整
const icons = {};
const aliases = {};
for (const name of used) {
  if (name in ri.icons) {
    icons[name] = ri.icons[name];
  } else if (name in ri.aliases) {
    aliases[name] = ri.aliases[name];
    let parent = ri.aliases[name].parent;
    while (!(parent in ri.icons)) {
      if (!(parent in ri.aliases)) {
        console.error(`ri:${name} 的别名父级 ${parent} 在 ri.json 中不存在`);
        process.exit(1);
      }
      parent = ri.aliases[parent].parent;
    }
    icons[parent] = ri.icons[parent];
  } else {
    console.error(`ri:${name} 不在 @iconify/json 的 ri.json 中（检查拼写或改用已有图标）`);
    process.exit(1);
  }
}

const subset = { prefix: "ri", icons, aliases, width: ri.width, height: ri.height };
const outPath = join(srcDir, "icons", "ri-subset.json");
mkdirSync(join(srcDir, "icons"), { recursive: true });
writeFileSync(outPath, JSON.stringify(subset, null, 2) + "\n");

const totalIcons = Object.keys(ri.icons).length;
const usedCount = used.size;
console.log(
  `ri 子集已写入 ${outPath}\n` +
    `  全量 ${totalIcons} 图标 + ${Object.keys(ri.aliases).length} 别名 → 子集 ${Object.keys(icons).length} 图标 + ${Object.keys(aliases).length} 别名（源码用法 ${usedCount} 个）`
);
