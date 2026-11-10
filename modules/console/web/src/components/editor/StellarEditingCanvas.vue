<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { Editor } from "@tiptap/core";
import type { StellarEditorMetadata } from "./stellar-context";
import { usePluginPreview } from "@/modules/plugins/preview/usePluginPreview";
import { PreviewCache } from "@/modules/plugins/preview/render-gate";
import { createStellarPageInfoPlugin, stellarPageInfoKey } from "./stellar-editor-decoration";
import { createStellarPageBlocksPlugin, extractStellarCodeBlocks, stellarPageBlocksKey } from "./stellar-editor-blocks";

const props = defineProps<{ editor?: Editor | null; markdown: string; metadata?: StellarEditorMetadata }>();
const viewport = ref<HTMLElement | null>(null);
const frame = ref<HTMLIFrameElement | null>(null);
const target = ref<HTMLElement | null>(null);
const width = ref(typeof window === "undefined" ? 1280 : window.innerWidth);
const height = ref(560);
let article: HTMLElement | null = null;
let content: HTMLElement | null = null;
let frameDocument: Document | null = null;
let viewportObserver: ResizeObserver | undefined;
let contentObserver: ResizeObserver | undefined;
let registeredEditor: Editor | null = null;
let presentationRoot: HTMLElement | null = null;
let addedMarkdownClass = false;
const styleFingerprints = new Map<string, string>();

// 此 iframe 只承载 app 创建的 TipTap DOM；从未执行服务端预览的 HTML 或脚本。
// 动态 NodeView 仍使用自己原有的 opaque sandbox，不获得本画布同源权限。
const editingDocument = '<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style data-editing-styles="layer-order">@layer stellar,theme,base,components,utilities;</style></head><body></body></html>';
const preview = usePluginPreview(() => ({ capability: "static", height: 560 }), undefined, {
  delayMs: 350,
  // 页面重新打开时重新取得站点设置，缓存只在本画布生命周期内复用。
  cache: new PreviewCache(20),
});
const refreshing = computed(() => preview.status.value === "loading" || preview.status.value === "updating");

function schedule() {
  // 空正文也需要正式页的样式/配色；占位仅用于预览请求，不进入编辑文档或 Markdown。
  preview.scheduleRender(props.markdown.trim() ? props.markdown : "<p></p>", { profile: null, metadata: props.metadata as unknown as Record<string, unknown> });
}
watch(() => [props.markdown, props.metadata], schedule, { deep: true, immediate: true });

function copyAttributes(source: Element, destination: Element) {
  for (const attribute of [...destination.attributes]) {
    if (attribute.name === "class" || attribute.name === "lang" || attribute.name === "style" || attribute.name.startsWith("data-")) destination.removeAttribute(attribute.name);
  }
  for (const attribute of [...source.attributes]) {
    if (attribute.name === "class" || attribute.name === "lang" || attribute.name === "style" || attribute.name.startsWith("data-")) destination.setAttribute(attribute.name, attribute.value);
  }
}

function copyStyles(source: Document, label: string) {
  const destination = frameDocument;
  if (!destination) return;
  const base = source.querySelector("base")?.getAttribute("href") || window.location.href;
  const styles = [...source.head.querySelectorAll("style,link[rel='stylesheet']")];
  const fingerprint = JSON.stringify(styles.map((node) => node.outerHTML)) + base;
  if (styleFingerprints.get(label) === fingerprint) return;
  styleFingerprints.set(label, fingerprint);
  destination.head.querySelectorAll(`[data-editing-styles="${label}"]`).forEach((node) => node.remove());
  for (const node of styles) {
    const element = destination.createElement("style");
    element.setAttribute("data-editing-styles", label);
    if (node.hasAttribute("media")) element.setAttribute("media", node.getAttribute("media") || "");
    // 仅在编辑画布分层：控件 scope 内的 Console utilities 优先，正文继续沿用同一份前台规则。
    if (node.tagName === "STYLE") element.textContent = `@layer stellar {${node.textContent || ""}}`;
    else {
      try {
        const href = new URL(node.getAttribute("href") || "", base);
        if (href.protocol !== "http:" && href.protocol !== "https:") continue;
        element.textContent = `@import url(${JSON.stringify(href.href)}) layer(stellar);`;
      } catch { continue; }
    }
    destination.head.appendChild(element);
  }
}

function extractEditingRules(rules: CSSRuleList | CSSRule[]): string {
  const auxiliary = /\.(?:rte-search-match(?:-current)?|ProseMirror-gapcursor|ProseMirror-hideselection|ProseMirror-separator|is-editor-empty|selectedCell)(?![\w-])/;
  const result: string[] = [];
  for (const rule of [...rules]) {
    if ("selectorText" in rule && "style" in rule) {
      const styleRule = rule as CSSStyleRule;
      const selectors = styleRule.selectorText.split(",").filter((selector) => auxiliary.test(selector)
        || (/^\s*(?:\.ProseMirror|\.rte-content\s+\.ProseMirror\s+(?:td|th))\s*$/.test(selector)
          && styleRule.style.length === 1 && styleRule.style.getPropertyValue("position") === "relative"));
      if (selectors.length) result.push(`${selectors.map((selector) => selector.replace(/\.rte-content\s+/g, "")).join(",")} {${styleRule.style.cssText}}`);
    } else if ("name" in rule && rule.name === "ProseMirror-cursor-blink") {
      result.push(rule.cssText);
    } else if ("cssRules" in rule) {
      const children = extractEditingRules((rule as CSSGroupingRule).cssRules);
      if (children) result.push(`${rule.cssText.slice(0, rule.cssText.indexOf("{"))}{${children}}`);
    }
  }
  return result.join("\n");
}

function copyConsoleStyles() {
  if (!frameDocument) return;
  const source = window.document;
  const styles: string[] = [];
  const editingRules: string[] = [];
  for (const sheet of [...source.styleSheets]) {
    // 只复制 app 自己已加载的同源样式；不把全局 reset 带到编辑正文。
    if (sheet.href && new URL(sheet.href, source.baseURI).origin !== window.location.origin) continue;
    const base = sheet.href || source.baseURI;
    const rules = [...sheet.cssRules].map((rule) => rule.cssText).join("\n");
    editingRules.push(extractEditingRules(sheet.cssRules));
    styles.push(rules.replace(/url\(\s*(?:"([^"]*)"|'([^']*)'|([^\s)]+))\s*\)/g, (match, double, single, unquoted) => {
      const value = double ?? single ?? unquoted;
      if (!value || value.startsWith("#") || value.startsWith("data:")) return match;
      return `url(${JSON.stringify(new URL(value, base).href)})`;
    }));
  }
  const root = window.getComputedStyle(source.documentElement);
  const body = window.getComputedStyle(source.body);
  const declarations = source.createElement("div").style;
  for (const name of [...root]) if (name.startsWith("--")) declarations.setProperty(name, root.getPropertyValue(name));
  for (const name of ["font-family", "font-size", "line-height", "color", "text-rendering"]) {
    declarations.setProperty(name, body.getPropertyValue(name));
  }
  // @scope 内普通 utility 不匹配 scope 根；气泡必须相对此节点定位。
  declarations.setProperty("position", "relative");
  const element = frameDocument.createElement("style");
  element.setAttribute("data-editing-styles", "console");
  element.textContent = `@scope (.stellar-tag-node) {${styles.join("\n")}\n:scope {${declarations.cssText}} :scope:hover [data-drag-handle] {opacity:1}}`;
  frameDocument.head.appendChild(element);
  const auxiliary = frameDocument.createElement("style");
  auxiliary.setAttribute("data-editing-styles", "auxiliary");
  auxiliary.textContent = `${editingRules.join("\n")}\n.ProseMirror .rte-search-match{--color-primary:var(--theme)}.ProseMirror .is-editor-empty{--color-muted-foreground:var(--text-p3)}.ProseMirror .selectedCell{--color-chart-4:var(--theme)}`;
  frameDocument.head.appendChild(auxiliary);
}

function setReadOnlyInformation(source: Document) {
  const editor = registeredEditor;
  if (!editor || editor.isDestroyed) return;
  const info = source.querySelector(".friends-site-info");
  editor.view.dispatch(editor.state.tr.setMeta(stellarPageInfoKey, info?.outerHTML || ""));
  editor.view.dispatch(editor.state.tr.setMeta(stellarPageBlocksKey, extractStellarCodeBlocks(source)));
}

function measure() {
  if (!viewport.value || !article || !frameDocument) return;
  width.value = window.innerWidth;
  // 媒体查询沿浏览器视口，正文列沿当前可用宽度；多出的视口只是右侧空白。
  article.style.width = `${Math.max(1, viewport.value.clientWidth - 48)}px`;
  height.value = Math.max(560, Math.ceil(article.getBoundingClientRect().height + 32));
}

function applyPresentation(document: string) {
  if (!frameDocument || !article || !content || !document) return;
  const source = new DOMParser().parseFromString(document, "text/html");
  copyAttributes(source.documentElement, frameDocument.documentElement);
  // Console 的节点操作控件使用 .dark 变体；前台配色仍由其 data-theme 属性决定。
  frameDocument.documentElement.classList.toggle("dark", window.document.documentElement.classList.contains("dark"));
  copyAttributes(source.body, frameDocument.body);
  copyStyles(source, "stellar");
  article.className = source.querySelector("article.md-text.content")?.className || "md-text content";
  content.className = source.querySelector(".friends-page") ? "friends-page" : "";
  const base = frameDocument.createElement("style");
  base.setAttribute("data-editing-styles", "layout");
  frameDocument.head.querySelectorAll('[data-editing-styles="layout"]').forEach((node) => node.remove());
  // 编辑把手、气泡和属性面板可伸入 gutter；横向边界仍由 body/html 与外层 viewport 裁剪。
  base.textContent = 'html,body{height:auto;min-height:0;margin:0;overflow-x:hidden}body{padding:16px 24px;box-sizing:border-box;background:var(--background)}article.md-text.content{margin:0;padding:0;max-width:none;overflow:visible}.ProseMirror.md-text{padding:0;max-width:none}.ProseMirror{min-height:528px;outline:none;white-space:pre-wrap;overflow-wrap:break-word}.ProseMirror .ProseMirror-widget{white-space:normal}.ProseMirror-selectednode{outline:2px solid var(--theme);outline-offset:2px}.ProseMirror :is(ul,ol)[data-tight="true"]>li>p,.ProseMirror :is(th,td)>p:only-child{margin-block:0;font-size:inherit;line-height:inherit}.ProseMirror .stellar-site-info-hint{display:block;margin:0 0 4px;font-size:12px;line-height:1.5;letter-spacing:.02em;color:var(--text-p3);text-align:left;user-select:none}';
  frameDocument.head.appendChild(base);
  setReadOnlyInformation(source);
  measure();
}
watch(preview.srcdoc, (document) => applyPresentation(document));

function clearPresentationRoot() {
  if (addedMarkdownClass) presentationRoot?.classList.remove("md-text");
  presentationRoot = null;
  addedMarkdownClass = false;
}

function registerEditor() {
  const editor = props.editor;
  if (registeredEditor === editor) return;
  if (registeredEditor && !registeredEditor.isDestroyed) {
    registeredEditor.unregisterPlugin(stellarPageInfoKey);
    registeredEditor.unregisterPlugin(stellarPageBlocksKey);
  }
  registeredEditor?.off("destroy", clearPresentationRoot);
  clearPresentationRoot();
  registeredEditor = editor || null;
  if (editor && !editor.isDestroyed) {
    // 直接子项规则也作用到真正的编辑根；只改变 DOM 呈现，不写入 schema/Markdown。
    presentationRoot = editor.view.dom;
    addedMarkdownClass = !presentationRoot.classList.contains("md-text");
    presentationRoot.classList.add("md-text");
    editor.on("destroy", clearPresentationRoot);
    editor.registerPlugin(createStellarPageInfoPlugin());
    editor.registerPlugin(createStellarPageBlocksPlugin());
  }
  nextTick(() => {
    editor?.view.updateRoot();
    applyPresentation(preview.srcdoc.value);
  });
}
watch(() => props.editor, registerEditor);

function activateCanvas(event: PointerEvent) {
  const host = frame.value;
  // 先激活 host，避免 Chromium 的默认 focus 将整张自适应高 iframe 在外层滚动区居中。
  // 编辑目标的 pointer/mousedown 保持不变，光标和选区继续使用编辑器原有的滚动规则。
  if (host && host.ownerDocument.activeElement !== host) host.focus({ preventScroll: true });
  const document = host?.contentDocument;
  if (!document) return;
  const target = event.target as Element | null;
  const control = target?.closest?.<HTMLElement>("button,input,textarea,select")
    || target?.closest?.<HTMLLabelElement>("label")?.control
    || target?.closest?.<HTMLElement>("[tabindex]");
  const editor = props.editor;
  const view = editor && !editor.isDestroyed ? editor.view : null;
  // 空白背景无编辑目标，阻止默认 focus 再次居中 host；事件仍传播给点外关闭逻辑。
  const background = event.target === document.body || event.target === document.documentElement;
  if (background) event.preventDefault();
  // 控件原生默认 focus 也可能居中整张 iframe。先无滚动聚焦实际目标，仍放行后续光标定位。
  if (!background && control && control !== view?.dom) {
    if (control.ownerDocument === document && typeof control.focus === "function"
      && !control.matches(":disabled,input[type='hidden']") && !control.closest("[inert]")
      && document.activeElement !== control) control.focus({ preventScroll: true });
  } else if (view && document.activeElement !== view.dom) {
    view.focus();
  }
}

async function initialize() {
  const document = frame.value?.contentDocument;
  if (!document || frameDocument === document) return;
  frameDocument?.removeEventListener("pointerdown", activateCanvas, true);
  frameDocument = document;
  document.addEventListener("pointerdown", activateCanvas, true);
  styleFingerprints.clear();
  copyConsoleStyles();
  article = document.createElement("article");
  article.className = "md-text content";
  content = document.createElement("div");
  article.appendChild(content);
  document.body.appendChild(article);
  target.value = content;
  contentObserver?.disconnect();
  contentObserver = new ResizeObserver(measure);
  contentObserver.observe(article);
  await nextTick();
  registerEditor();
  props.editor?.view.updateRoot();
  applyPresentation(preview.srcdoc.value);
  measure();
}

onMounted(() => {
  viewportObserver = new ResizeObserver(measure);
  if (viewport.value) viewportObserver.observe(viewport.value);
  window.addEventListener("resize", measure);
});
onBeforeUnmount(() => {
  frameDocument?.removeEventListener("pointerdown", activateCanvas, true);
  viewportObserver?.disconnect();
  contentObserver?.disconnect();
  window.removeEventListener("resize", measure);
  if (registeredEditor && !registeredEditor.isDestroyed) {
    registeredEditor.unregisterPlugin(stellarPageInfoKey);
    registeredEditor.unregisterPlugin(stellarPageBlocksKey);
  }
  registeredEditor?.off("destroy", clearPresentationRoot);
  clearPresentationRoot();
});
</script>

<template>
  <div class="stellar-editing-canvas">
    <p v-if="preview.status.value === 'error'" class="px-4 py-2 text-xs text-state-error" role="status">
      页面样式更新失败，仍保留当前编辑内容。<button type="button" class="ml-1 text-primary" @click="preview.retry()">重试</button>
    </p>
    <div ref="viewport" class="stellar-editing-viewport relative overflow-hidden" :aria-busy="refreshing">
      <iframe ref="frame" :srcdoc="editingDocument" :style="{width: width + 'px', height: height + 'px'}" class="block max-w-none border-0" title="页面编辑画布" @load="initialize" />
    </div>
    <Teleport v-if="target" :to="target"><slot /></Teleport>
  </div>
</template>
