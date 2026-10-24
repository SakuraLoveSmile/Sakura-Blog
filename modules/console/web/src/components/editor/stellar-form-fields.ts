import type { StellarFormField } from "./useStellarForm";
const lines = (value: unknown) => (Array.isArray(value) ? value.join("\n") : typeof value === "string" ? value : "");
const list = (value: string) =>
  value
    .split(/[,，\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
export const postFields: StellarFormField[] = [
  { form: "title", path: "title", host: true },
  { form: "excerpt", path: "excerpt", host: true },
  { form: "cover", path: "cover" },
  { form: "banner", path: "content.banner_image", blog: true },
  {
    form: "categories",
    path: "categories",
    // 原文允许单个分类；表单/API 使用列表，名称中的逗号和层级斜杠不拆分。
    read: (value) => (typeof value === "string" ? (value.trim() ? [value] : []) : (value ?? [])),
  },
  {
    form: "tags",
    path: "tags",
    read: (value) => (Array.isArray(value) ? value.join(", ") : (value ?? "")),
    write: list,
  },
  { form: "author", path: "article.author" },
  { form: "authors", path: "content.authors", blog: true, fallback: [] },
  { form: "ai_label", path: "article.ai_label", write: (value) => value || null },
  { form: "meta_description", path: "description", host: true },
  { form: "meta_keywords", path: "keywords", host: true },
  { form: "pinned", path: "listing.priority", read: (value) => Number(value) > 0, write: (value) => (value ? 1 : 0) },
];
const widgetFields = (region: string): StellarFormField => ({
  form: region + "_widgets",
  path: region + ".widgets",
  read: (value) =>
    Array.isArray(value)
      ? value.map((widget) => {
          if (typeof widget === "string") return { name: widget, params: {} };
          const { override, ...params } = widget;
          return { name: override ?? widget.layout ?? "", params };
        })
      : null,
  write: (value) =>
    value === null
      ? undefined
      : value.map((widget: any) =>
          Object.keys(widget.params).length ? { override: widget.name, ...widget.params } : widget.name,
        ),
});
export const pageFields: StellarFormField[] = [
  { form: "title", path: "title", host: true },
  { form: "banner", path: "content.banner_image", blog: true },
  { form: "banner_info.title", path: "banner.headline", write: (value) => value || undefined },
  { form: "banner_info.subtitle", path: "banner.tagline" },
  { form: "banner_info.avatar", path: "banner.avatar" },
  { form: "banner_info.icon", path: "leftbar.brand.image.src" },
  { form: "breadcrumb", path: "breadcrumb", fallback: true },
  { form: "author", path: "article.author" },
  { form: "date", path: "date", host: true },
  widgetFields("leftbar"),
  widgetFields("rightbar"),
];
export const collectionFields: StellarFormField[] = [
  { form: "name", path: "name" },
  { form: "description", path: "description", host: true },
  { form: "icon", path: "icon" },
  { form: "cover", path: "cover" },
  { form: "sort", path: "content.collection_order", blog: true, fallback: 0 },
];
export const wikiFields: StellarFormField[] = [
  ...collectionFields,
  { form: "pin", path: "listing.priority", read: (value) => Number(value) > 0, write: (value) => (value ? 1 : 0) },
  { form: "hero.enabled", path: "hero.enabled", fallback: false },
  { form: "headline", path: "headline" },
  { form: "subtitle", path: "tagline" },
  { form: "available", path: "audience" },
  { form: "url", path: "content.wiki_external_url", blog: true },
  { form: "repo", path: "source.repository" },
  { form: "branch", path: "source.branch" },
  { form: "tagsText", path: "tags", read: (value) => (Array.isArray(value) ? value.join(", ") : ""), write: list },
  {
    form: "hero.start",
    path: "hero.actions",
    read: (value) => value?.[0]?.title ?? "",
    write: (value, current) => [{ ...(current?.[0] ?? {}), title: value }, ...(current?.slice(1) ?? [])],
  },
  { form: "hero.background", path: "hero.background.image" },
  { form: "hero.title", path: "headline" },
  { form: "hero.description", path: "description", host: true },
  {
    form: "hero.animationType",
    path: "hero.background.effect",
    read: (value) => value?.type ?? "",
    write: (value, current) => (value ? { ...(current ?? {}), type: value } : null),
  },
  { form: "hero.animationParamsText", path: "hero.background.effect.options", raw: true },
  { form: "hero.previewType", path: "hero.preview.type", write: (value) => value || null },
  { form: "hero.previewImage", path: "hero.preview.src" },
  { form: "hero.commands", path: "hero.preview.commands", fallback: [] },
  { form: "hero.actions", path: "hero.actions", fallback: [] },
];
export const wikiDocFields: StellarFormField[] = [
  { form: "title", path: "title", host: true },
  { form: "order", path: "content.collection_order", blog: true, fallback: 0 },
];
export const noteFields: StellarFormField[] = [
  {
    form: "tags",
    path: "tags",
    read: lines,
    write: (value) =>
      value
        .split(/\r?\n/)
        .map((tag: string) => tag.trim())
        .filter(Boolean),
  },
];
