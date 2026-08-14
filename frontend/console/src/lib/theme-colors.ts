/** Read a console semantic color at runtime while keeping non-DOM consumers usable. */
export function themeColor(name: string, fallback: string): string {
  if (typeof document === "undefined") return fallback;
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue(`--color-${name}`)
    .trim();
  return value || fallback;
}

export function themeColors(entries: Record<string, string>): Record<string, string> {
  return Object.fromEntries(
    Object.entries(entries).map(([name, fallback]) => [name, themeColor(name, fallback)]),
  );
}
