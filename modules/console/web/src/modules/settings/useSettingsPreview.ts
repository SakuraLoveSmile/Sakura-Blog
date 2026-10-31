import { computed, onBeforeUnmount, ref, watch, type Ref } from "vue";
import { getApiErrorMessage, previewApi } from "@/api";
import { readJSONFieldRaw } from "@/components/editor/stellar-form-json";

const PREVIEW_FOCUS: Record<string, { label: string; page: string }> = {
  basic: { label: "站点标识与页脚", page: "index" },
  appearance: { label: "整站外观", page: "index" },
  navigation: { label: "顶栏、侧栏与页脚", page: "index" },
  layout: { label: "当前页面布局", page: "index" },
  post: { label: "文章与卡片", page: "post" },
};

export const settingsPreviewPages = [
  { value: "index", label: "首页 / 文章列表" },
  { value: "post", label: "文章详情" },
  { value: "wiki", label: "文档详情" },
  { value: "wiki-index", label: "文档列表" },
  { value: "topic", label: "专栏列表" },
  { value: "notebooks", label: "笔记本列表" },
  { value: "categories", label: "分类页" },
  { value: "tags", label: "标签页" },
  { value: "archives", label: "归档页" },
  { value: "settings", label: "访客设置页" },
];

export function useSettingsPreview(
  data: Ref<Record<string, any>>,
  active: Ref<string>,
  canonicalDocument: Ref<string>,
  blocked?: Ref<boolean>,
) {
  const previewOpen = ref(false);
  const previewViewport = ref<"desktop" | "tablet" | "mobile">("desktop");
  const previewPage = ref("index");
  const previewHtml = ref("");
  const previewLoading = ref(false);
  const previewError = ref("");
  const previewFocus = computed(() => PREVIEW_FOCUS[active.value] ?? null);
  let previewTimer: ReturnType<typeof setTimeout> | undefined;
  let sequence = 0;

  function previewSections(): string {
    const theme = readJSONFieldRaw(canonicalDocument.value, "theme");
    const blog = readJSONFieldRaw(canonicalDocument.value, "blog_extensions");
    if (!theme || !blog) throw new Error("请先修正完整主题配置");
    return `{"site":${JSON.stringify(data.value.site ?? {})},"stellar_v2":{"version":2,"theme":${theme}},"blog_extensions":${blog}}`;
  }

  function buildPreviewDocument(fullHtml: string): string {
    if (!fullHtml.trim()) throw new Error("没有收到预览页面，请重试。");
    const doc = new DOMParser().parseFromString(fullHtml, "text/html");
    // 保留真实整页骨架及全部外观属性，静态预览不执行站点脚本或代码注入。
    doc
      .querySelectorAll("script, iframe, object, embed, base, meta[http-equiv='refresh' i]")
      .forEach((el) => el.remove());
    for (const el of doc.querySelectorAll("*")) {
      for (const attr of [...el.attributes])
        if (attr.name.toLowerCase().startsWith("on")) el.removeAttribute(attr.name);
    }
    // 前台依靠运行时展开懒加载；静态预览直接显示图片，避免横幅和头像留白。
    for (const image of doc.querySelectorAll<HTMLImageElement>("img[data-src]")) {
      const source = image.getAttribute("data-src");
      if (!source) continue;
      try {
        const url = new URL(source, location.origin);
        if (url.protocol !== "http:" && url.protocol !== "https:") continue;
        image.setAttribute("src", url.href);
        image.classList.remove("loading");
        image.classList.add("loaded");
        // 静态预览直接显示最终态，避免 iframe 中的淡入动画把已加载图片留在透明起点。
        image.style.animation = "none";
        (image.closest(".lazy-box") ?? image.parentElement)?.querySelector(".lazy-icon")?.remove();
      } catch {
        // 无效地址维持原样，交由用户修正配置。
      }
    }
    if (doc.documentElement.hasAttribute("data-theme-auto")) {
      doc.documentElement.setAttribute(
        "data-theme",
        matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light",
      );
    }
    const style = doc.createElement("style");
    style.textContent = "a,button,input,select,textarea,form{pointer-events:none}html{scroll-behavior:auto!important}";
    doc.head.append(style);
    return `<!doctype html>${doc.documentElement.outerHTML}`;
  }

  async function refreshPreview() {
    clearTimeout(previewTimer);
    if (!previewOpen.value || !previewFocus.value) return;
    const currentSequence = ++sequence;
    previewError.value = "";
    if (blocked?.value) {
      previewLoading.value = false;
      previewError.value = "部分设置尚未填写完成或存在错误，请先修正，再查看草稿效果。";
      return;
    }
    previewLoading.value = true;
    try {
      const { data: response } = await previewApi.site(previewSections(), previewPage.value);
      if (currentSequence === sequence) previewHtml.value = buildPreviewDocument(response.html || "");
    } catch (error) {
      if (currentSequence === sequence)
        previewError.value = getApiErrorMessage(error, "预览生成失败，请检查设置后重试。");
    } finally {
      if (currentSequence === sequence) previewLoading.value = false;
    }
  }

  function schedulePreview() {
    if (!previewOpen.value) return;
    // 草稿一变就作废旧请求，避免防抖期间旧响应覆盖新草稿。
    sequence++;
    clearTimeout(previewTimer);
    previewLoading.value = true;
    previewTimer = setTimeout(() => void refreshPreview(), 500);
  }

  watch(previewOpen, (open) => {
    sequence++;
    clearTimeout(previewTimer);
    if (open) void refreshPreview();
    else previewLoading.value = false;
  });
  watch(active, () => {
    sequence++;
    clearTimeout(previewTimer);
    previewHtml.value = "";
    previewError.value = "";
    previewPage.value = previewFocus.value?.page ?? "index";
    if (!previewFocus.value) previewOpen.value = false;
    else if (previewOpen.value) schedulePreview();
  });
  watch(previewPage, schedulePreview);
  watch(data, schedulePreview, { deep: true });
  watch(canonicalDocument, schedulePreview);
  if (blocked) watch(blocked, schedulePreview);
  onBeforeUnmount(() => {
    sequence++;
    clearTimeout(previewTimer);
  });

  return {
    previewOpen,
    previewViewport,
    previewPage,
    previewHtml,
    previewLoading,
    previewError,
    previewFocus,
    refreshPreview,
  };
}
