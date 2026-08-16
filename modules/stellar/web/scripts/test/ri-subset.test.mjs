// Spec 61 阶段 1：ri 图标子集一致性机械校验。
// 1. 源码中出现的 `ri:` 图标必须都能在 src/icons/ri-subset.json 中解析（含别名父级）；
// 2. 子集不得包含源码未使用的图标（保持子集最小，防止体积回涨）。
// 子集由 node modules/stellar/web/scripts/gen-ri-subset.mjs 生成，本测试与生成器共用同一扫描规则。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const stellarWebDir = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const srcDir = join(stellarWebDir, "..", "..", "console", "web", "src");

function collectFiles(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) collectFiles(p, out);
    else if (/\.(vue|ts|js|json)$/.test(name)) out.push(p);
  }
  return out;
}

// 扫描源码中全部 `ri:<name>` 用法（与生成器同规则）
function scanUsedIcons() {
  const used = new Set();
  for (const f of collectFiles(srcDir)) {
    const text = readFileSync(f, "utf8");
    for (const m of text.matchAll(/ri:([a-z0-9-]+)/g)) used.add(m[1]);
  }
  return used;
}

// 图标是否能在子集中解析（自身或别名链到父图标）
function resolveIcon(subset, name, seen = new Set()) {
  if (name in subset.icons) return true;
  const alias = subset.aliases?.[name];
  if (!alias) return false;
  if (seen.has(alias.parent)) return false;
  seen.add(alias.parent);
  return resolveIcon(subset, alias.parent, seen);
}

test("ri 子集覆盖源码全部 ri: 用法", () => {
  const subset = JSON.parse(readFileSync(join(srcDir, "icons", "ri-subset.json"), "utf8"));
  const used = scanUsedIcons();
  const missing = [...used].filter((n) => !resolveIcon(subset, n));
  assert.deepEqual(
    missing,
    [],
    `源码用到但子集缺失的 ri 图标：${missing.join(", ")}（请运行 node modules/stellar/web/scripts/gen-ri-subset.mjs 重新生成）`
  );
});

test("ri 子集无未使用图标（保持最小）", () => {
  const subset = JSON.parse(readFileSync(join(srcDir, "icons", "ri-subset.json"), "utf8"));
  const used = scanUsedIcons();
  const aliasParents = new Set(Object.values(subset.aliases ?? {}).map((a) => a.parent));
  const orphan = Object.keys(subset.icons).filter(
    (n) => !used.has(n) && !aliasParents.has(n)
  );
  assert.deepEqual(
    orphan,
    [],
    `子集中未使用的图标：${orphan.join(", ")}（请运行 node modules/stellar/web/scripts/gen-ri-subset.mjs 重新生成）`
  );
});
