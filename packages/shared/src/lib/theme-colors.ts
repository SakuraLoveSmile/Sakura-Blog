/**
 * 把调用方传入的主题键规范化为 CSS 变量名。
 *
 * CSS 变量定义使用 kebab-case（`--color-chart-2`、`--color-primary-hover`），
 * 调用方历史上同时使用 camelCase（`chart2`、`primaryHover`）与 kebab-case。
 * 统一在读取时归一化，避免任一命名方式静默落到硬编码 fallback。
 */
export function themeColorVariable(name: string): string {
  const normalized = name
    .trim()
    .replace(/^--color-/, "")
    .replace(/^--/, "")
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    // `chart2` → `chart-2`：CSS 变量以 kebab-case 定义，数字后缀同样需要分隔。
    .replace(/([a-zA-Z])(\d)/g, "$1-$2")
    .replace(/_/g, "-")
    .toLowerCase();
  return `--color-${normalized}`;
}

/** Read a console semantic color at runtime while keeping non-DOM consumers usable. */
export function themeColor(name: string, fallback: string): string {
  if (typeof document === "undefined") return fallback;
  const value = getComputedStyle(document.documentElement).getPropertyValue(themeColorVariable(name)).trim();
  return value || fallback;
}

export function themeColors(entries: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(entries).map(([name, fallback]) => [name, themeColor(name, fallback)]));
}
