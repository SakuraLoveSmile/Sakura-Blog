import { canonicalDocument, patchJSONField, readJSONFieldRaw } from "../../components/editor/stellar-form-json";
import type { CanonicalSettingsField } from "./settings-canonical-fields";
import {
  canonicalFieldPaths,
  settingsPageFields,
  settingsPagePaths,
  settingsThemePages,
  type SettingsPageKey,
} from "./settings-pages";

// 设置页的未保存计数、覆盖判断与“放弃本页修改”。全部是对草稿/已保存原文的纯计算，
// 页面只负责展示；计数只用于提示，是否可保存仍以原文是否变化为准。
type JSONObject = Record<string, unknown>;
const isObject = (value: unknown): value is JSONObject =>
  value !== null && typeof value === "object" && !Array.isArray(value);

export function parseDocument(raw: string): JSONObject | undefined {
  try {
    const value = JSON.parse(raw);
    return isObject(value) ? value : undefined;
  } catch {
    return undefined;
  }
}

export function valueAt(doc: JSONObject | undefined, path: string): unknown {
  let current: unknown = doc;
  for (const key of path.split(".")) {
    if (!isObject(current) || !Object.hasOwn(current, key)) return undefined;
    current = current[key];
  }
  return current;
}

/** 两个值之间有多少处叶子差异；缺失与空对象视为相同。 */
export function changeCount(draft: unknown, saved: unknown): number {
  if (draft === undefined && saved === undefined) return 0;
  const draftObject = draft === undefined || isObject(draft);
  const savedObject = saved === undefined || isObject(saved);
  if (draftObject && savedObject) {
    const a = (draft ?? {}) as JSONObject;
    const b = (saved ?? {}) as JSONObject;
    let count = 0;
    for (const key of new Set([...Object.keys(a), ...Object.keys(b)])) count += changeCount(a[key], b[key]);
    return count;
  }
  return JSON.stringify(draft) === JSON.stringify(saved) ? 0 : 1;
}

/** 结构化字段按内部叶子计数；其他控件无论改了几处都算一项。 */
export function fieldChangeCount(
  draft: JSONObject | undefined,
  saved: JSONObject | undefined,
  item: CanonicalSettingsField,
): number {
  if (item.kind === "structured") return changeCount(valueAt(draft, item.path), valueAt(saved, item.path));
  return canonicalFieldPaths(item).some((path) => changeCount(valueAt(draft, path), valueAt(saved, path)) > 0) ? 1 : 0;
}

export function pageChangeCount(
  draft: JSONObject | undefined,
  saved: JSONObject | undefined,
  page: SettingsPageKey,
): number {
  return settingsPageFields(page).reduce((sum, item) => sum + fieldChangeCount(draft, saved, item), 0);
}

/** 草稿里是否存在对该字段的显式覆盖；“恢复默认”就是移除这个覆盖。 */
export function hasOverride(draft: JSONObject | undefined, item: CanonicalSettingsField): boolean {
  return canonicalFieldPaths(item).some((path) => {
    const value = valueAt(draft, path);
    if (value === undefined) return false;
    // 恢复默认后父级可能留下空对象，不算覆盖。
    return !(item.kind === "structured" && isObject(value) && Object.keys(value).length === 0);
  });
}

/** 草稿里该路径下是否留有任何显式值（空对象不算）；页面级“恢复默认”据此决定是否出现。 */
export function pathHasOverride(draft: JSONObject | undefined, path: string): boolean {
  const value = valueAt(draft, path);
  return value !== undefined && !(isObject(value) && Object.keys(value).length === 0);
}

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) return "[" + value.map(stableStringify).join(",") + "]";
  if (isObject(value))
    return (
      "{" +
      Object.keys(value)
        .sort()
        .map((key) => JSON.stringify(key) + ":" + stableStringify(value[key]))
        .join(",") +
      "}"
    );
  return JSON.stringify(value);
}

function removeAt(doc: JSONObject, keys: string[]) {
  let current: unknown = doc;
  for (const key of keys.slice(0, -1)) {
    if (!isObject(current) || !Object.hasOwn(current, key)) return;
    current = current[key];
  }
  if (isObject(current)) delete current[keys[keys.length - 1]];
}

function prune(value: JSONObject): JSONObject {
  for (const key of Object.keys(value)) {
    const child = value[key];
    if (isObject(child)) {
      prune(child);
      if (Object.keys(child).length === 0) delete value[key];
    }
  }
  return value;
}

let catalogPaths: string[] | undefined;
const allCatalogPaths = () => (catalogPaths ??= settingsThemePages.flatMap(settingsPagePaths));

/** 目录之外的内容（例如只在完整配置原文里改过的参数）是否与已保存配置不同。 */
export function offCatalogChanged(draft: JSONObject | undefined, saved: JSONObject | undefined): boolean {
  if (!draft || !saved) return false;
  const strip = (doc: JSONObject) => {
    const copy = structuredClone(doc);
    for (const path of allCatalogPaths()) removeAt(copy, path.split("."));
    return stableStringify(prune(copy));
  };
  return strip(draft) !== strip(saved);
}

export interface ThemeChangeSummary {
  pages: Partial<Record<SettingsPageKey, number>>;
  /** 完整配置原文里目录之外的改动，计为 1 处。 */
  other: number;
  total: number;
}

export function themeChangeSummary(draftRaw: string, savedRaw: string): ThemeChangeSummary {
  const draft = parseDocument(draftRaw);
  const saved = parseDocument(savedRaw);
  const pages: ThemeChangeSummary["pages"] = {};
  if (!draft || !saved) return { pages, other: 0, total: 0 };
  let total = 0;
  for (const page of settingsThemePages) {
    const count = pageChangeCount(draft, saved, page);
    pages[page] = count;
    total += count;
  }
  const other = offCatalogChanged(draft, saved) ? 1 : 0;
  return { pages, other, total: total + other };
}

export function sameDocument(a: string, b: string): boolean {
  try {
    return canonicalDocument(a) === canonicalDocument(b);
  } catch {
    return false;
  }
}

function pruneEmptyAncestors(doc: string, saved: string, path: string): string {
  const keys = path.split(".");
  // 第一层（theme / blog_extensions）始终保留。
  for (let depth = keys.length - 1; depth >= 2; depth--) {
    const parent = keys.slice(0, depth).join(".");
    const raw = readJSONFieldRaw(doc, parent);
    if (raw === undefined) continue;
    const value = JSON.parse(raw);
    if (!isObject(value) || Object.keys(value).length > 0 || readJSONFieldRaw(saved, parent) !== undefined) break;
    doc = patchJSONField(doc, parent, undefined);
  }
  return doc;
}

/**
 * 把指定路径还原为已保存值并保留其余草稿。已保存值缺失的路径直接移除，并清理由此留下的空父对象；
 * 若结果与已保存配置语义一致则直接返回已保存原文，避免格式差异让草稿一直显示为已修改。
 */
export function restorePaths(draft: string, saved: string, paths: string[]): string {
  let next = draft;
  for (const path of paths) {
    const savedRaw = readJSONFieldRaw(saved, path);
    if (savedRaw === readJSONFieldRaw(next, path)) continue;
    if (savedRaw === undefined) next = pruneEmptyAncestors(patchJSONField(next, path, undefined), saved, path);
    else next = patchJSONField(next, path, undefined, savedRaw);
  }
  return sameDocument(next, saved) ? saved : next;
}

/**
 * 放弃一页的全部未保存修改。普通主题页只还原自己拥有的路径；“代码与原文”页同时拥有目录之外的全部内容，
 * 因此以已保存配置为底稿，仅把其余页面的草稿字段带回来（角色互换后复用 restorePaths 的逐字拼接）。
 */
export function restorePage(draft: string, saved: string, page: SettingsPageKey): string {
  if (page !== "advanced") return restorePaths(draft, saved, settingsPagePaths(page));
  const others = settingsThemePages.filter((key) => key !== "advanced").flatMap(settingsPagePaths);
  return restorePaths(saved, draft, others);
}
