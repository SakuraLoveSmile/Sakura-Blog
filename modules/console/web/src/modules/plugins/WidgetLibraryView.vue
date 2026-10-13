<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { http } from "@blog/api-client/transport";
import { getApiErrorMessage } from "@/api";
import { VButton, VCard, VLoading, VPageHeader } from "@/components/ui";

interface WidgetItem {
  name: string;
  layout: string;
  config: string;
}
const items = ref<WidgetItem[]>([]);
const selected = ref("");
const candidate = ref("{}");
const search = ref("");
const layout = ref("page");
const path = ref("");
const loading = ref(true);
const rendering = ref(false);
const error = ref("");
const document = ref("");
const token = ref("");
const frame = ref<HTMLIFrameElement | null>(null);
const height = ref(480);
const matches = computed(() =>
  items.value.filter((item) => `${item.name} ${item.layout}`.toLowerCase().includes(search.value.toLowerCase())),
);
let generation = 0;
let timer: ReturnType<typeof setTimeout> | undefined;

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const { data } = await http.get<{ items: WidgetItem[] }>("/plugins/widgets/catalog");
    items.value = data.items;
    if (data.items[0]) select(data.items[0]);
  } catch (reason) {
    error.value = getApiErrorMessage(reason);
  } finally {
    loading.value = false;
  }
}
function select(item: WidgetItem) {
  selected.value = item.name;
  candidate.value = item.config;
  schedule(true);
}
function schedule(immediate = false) {
  clearTimeout(timer);
  timer = setTimeout(preview, immediate ? 0 : 500);
}
async function preview() {
  const current = ++generation;
  rendering.value = true;
  error.value = "";
  try {
    const previewToken = Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
      byte.toString(16).padStart(2, "0"),
    ).join("");
    const { data } = await http.post<{ document: string; preview_token: string }>("/preview/widget", {
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
      widget_config: candidate.value,
      layout: layout.value,
      path: path.value,
      preview_token: previewToken,
    });
    if (current !== generation) return;
    document.value = data.document;
    token.value = data.preview_token;
  } catch (reason) {
    if (current === generation) error.value = getApiErrorMessage(reason);
  } finally {
    if (current === generation) rendering.value = false;
  }
}
function onMessage(event: MessageEvent) {
  if (
    event.source !== frame.value?.contentWindow ||
    event.data?.token !== token.value ||
    event.data?.type !== "component-preview-resize"
  )
    return;
  const next = event.data.height;
  if (typeof next === "number" && Number.isFinite(next) && next > 0)
    height.value = Math.min(1200, Math.max(120, Math.ceil(next)));
}
watch([candidate, layout, path], () => schedule());
onMounted(() => {
  window.addEventListener("message", onMessage);
  void load();
});
onBeforeUnmount(() => {
  generation++;
  clearTimeout(timer);
  window.removeEventListener("message", onMessage);
});
</script>

<template>
  <div>
    <VPageHeader title="Stellar 2 侧栏组件" subtitle="预览实际运行时 catalog 与原文候选；候选不会写入站点。" />
    <p class="mb-4 text-sm leading-relaxed text-text-tertiary">
      命名组件保存在独立 Blog.data.widgets 中。检查候选后，到
      <router-link class="text-primary" to="/settings?tab=stellar2">Stellar 2 完整配置</router-link> 审查并保存原文；旧
      widgets.library/pages 不再写入。
    </p>
    <p v-if="error" class="mb-4 whitespace-pre-wrap break-words text-sm text-destructive" role="alert">{{ error }}</p>
    <VLoading v-if="loading" />
    <div v-else class="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
      <VCard>
        <label class="block text-xs text-text-tertiary"
          >搜索组件<input v-model="search" class="apple-input mt-2 w-full" placeholder="名称或 layout"
        /></label>
        <div class="mt-3 space-y-1">
          <button
            v-for="item in matches"
            :key="item.name"
            type="button"
            :aria-pressed="selected === item.name"
            class="block min-h-11 w-full rounded-lg px-3 py-2 text-left text-sm hover:bg-accent"
            :class="selected === item.name ? 'bg-accent text-primary' : ''"
            @click="select(item)"
          >
            <span class="block font-medium">{{ item.name }}</span
            ><span class="font-mono text-xs text-text-tertiary">{{ item.layout || "registry 默认规则" }}</span>
          </button>
          <p v-if="!matches.length" class="text-xs text-text-tertiary">没有匹配的命名组件。</p>
        </div>
      </VCard>
      <div class="min-w-0 space-y-4">
        <VCard>
          <label class="block text-xs text-text-tertiary"
            >完整 Widget JSON 候选<textarea
              v-model="candidate"
              rows="12"
              spellcheck="false"
              class="apple-textarea mt-2 w-full font-mono text-xs"
            />
          </label>
          <div class="mt-3 grid gap-3 sm:grid-cols-2">
            <label class="block text-xs text-text-tertiary"
              >公开上下文类型<select v-model="layout" class="apple-select mt-2 w-full">
                <option value="page">页面</option>
                <option value="post">文章</option>
                <option value="wiki">Wiki</option>
                <option value="note">笔记</option>
              </select></label
            >
            <label class="block text-xs text-text-tertiary"
              >公开内容路径（可选）<input
                v-model="path"
                class="apple-input mt-2 w-full"
                placeholder="/post/example 或 /wiki/project/doc"
            /></label>
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-3">
            <VButton :loading="rendering" @click="preview">只读预览</VButton
            ><span class="text-xs text-text-tertiary">没有内容上下文的组件按上游空输出或诊断规则显示。</span>
          </div>
        </VCard>
        <VCard
          ><iframe
            v-if="document"
            ref="frame"
            :srcdoc="document"
            sandbox="allow-scripts"
            title="Stellar Widget 隔离预览"
            class="w-full border-0"
            :style="{ height: height + 'px' }"
          />
          <p v-else class="text-xs text-text-tertiary">选择组件并输入候选以预览。</p></VCard
        >
      </div>
    </div>
  </div>
</template>
