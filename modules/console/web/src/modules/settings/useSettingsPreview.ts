import { computed, onBeforeUnmount, ref, watch, type Ref } from "vue";
import { previewApi } from "@/api";

interface PreviewFocus {
  page: string;
  selectors: string[];
  label: string;
}

const PREVIEW_FOCUS: Record<string, PreviewFocus> = {
  basic: { page: "index", selectors: [".navbar.top", "footer.page-footer"], label: "顶栏与页脚" },
  post: { page: "index", selectors: [".l_main"], label: "文章卡片" },
  comments: { page: "post", selectors: ["#comments"], label: "评论区" },
};

function filterPreviewHead(head: HTMLHeadElement): void {
  for (const script of Array.from(head.querySelectorAll("script"))) {
    const text = script.textContent || "";
    if (!script.src && text.includes("prefers-color-scheme") && text.includes("data-theme-auto")) continue;
    script.remove();
  }
}

function markChanged(oldEl: Element, newEl: Element): void {
  if (oldEl.outerHTML === newEl.outerHTML) return;
  const oldChildren = Array.from(oldEl.children);
  const newChildren = Array.from(newEl.children);
  let descended = false;
  for (let i = 0; i < newChildren.length; i++) {
    const oldChild = oldChildren[i];
    const newChild = newChildren[i];
    if (oldChild && newChild && oldChild.tagName === newChild.tagName && oldChild.outerHTML !== newChild.outerHTML) {
      markChanged(oldChild, newChild);
      descended = true;
    }
  }
  if (!descended) newEl.classList.add("settings-preview-changed");
}

export function useSettingsPreview(
  data: Ref<Record<string, any>>,
  active: Ref<string>,
  sections: readonly string[],
) {
  const previewOpen = ref(false);
  const previewViewport = ref<"desktop" | "tablet" | "mobile">("desktop");
  const previewHtml = ref("");
  const previewLoading = ref(false);
  const previewFocus = computed(() => PREVIEW_FOCUS[active.value] ?? null);
  let previewTimer: ReturnType<typeof setTimeout> | undefined;
  let previousHtml = "";
  let sequence = 0;

  function previewSections(): Record<string, unknown> {
    const result: Record<string, unknown> = {};
    for (const section of sections) {
      if (data.value[section] !== undefined) result[section] = data.value[section];
    }
    return result;
  }

  function buildFragmentDoc(fullHtml: string, focus: PreviewFocus): string {
    const doc = new DOMParser().parseFromString(fullHtml, "text/html");
    const previousDoc = previousHtml ? new DOMParser().parseFromString(previousHtml, "text/html") : null;
    for (const selector of focus.selectors) {
      const element = doc.querySelector(selector);
      if (!element) continue;
      const oldElement = previousDoc?.querySelector(selector);
      if (!oldElement) element.classList.add("settings-preview-flash");
      else if (oldElement.outerHTML !== element.outerHTML) {
        element.classList.add("settings-preview-flash");
        markChanged(oldElement, element);
      }
    }
    previousHtml = fullHtml;
    const fragments = focus.selectors.map((selector) => doc.querySelector(selector)?.outerHTML || "").filter(Boolean);
    const body = fragments.length ? fragments.join("\n") : doc.body.innerHTML;
    filterPreviewHead(doc.head);
    const theme = doc.documentElement.getAttribute("data-theme") || "dark";
    const auto = doc.documentElement.hasAttribute("data-theme-auto") ? " data-theme-auto" : "";
    return `<!doctype html><html lang="zh-CN" data-theme="${theme}"${auto}><head>${doc.head.innerHTML}<style>body{padding:20px!important}.settings-preview-flash{animation:settingsPreviewFlash 1.1s cubic-bezier(.32,.72,0,1)}@keyframes settingsPreviewFlash{0%{background:color-mix(in srgb,var(--color-primary,#007AFF) 14%,transparent)}100%{background:transparent}}.settings-preview-changed{outline:2px solid var(--color-primary,#007AFF);outline-offset:3px}</style></head><body class="${doc.body.className || ""}">${body}</body></html>`;
  }

  async function refreshPreview() {
    if (!previewOpen.value || !previewFocus.value) return;
    const focus = previewFocus.value;
    const currentSequence = ++sequence;
    previewLoading.value = true;
    try {
      const { data: response } = await previewApi.site(previewSections(), focus.page);
      if (currentSequence === sequence) previewHtml.value = buildFragmentDoc(response.html || "", focus);
    } catch {
      if (currentSequence === sequence) previewHtml.value = "";
    } finally {
      if (currentSequence === sequence) previewLoading.value = false;
    }
  }

  function schedulePreview() {
    if (!previewOpen.value) return;
    clearTimeout(previewTimer);
    previewTimer = setTimeout(() => void refreshPreview(), 1000);
  }

  watch(previewOpen, (open) => {
    if (open) void refreshPreview();
  });
  watch(active, () => {
    previousHtml = "";
    if (previewOpen.value) void refreshPreview();
  });
  watch(data, schedulePreview, { deep: true });
  onBeforeUnmount(() => clearTimeout(previewTimer));

  return { previewOpen, previewViewport, previewHtml, previewLoading, previewFocus, refreshPreview };
}
