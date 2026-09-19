<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { attachmentApi, attachmentGroupApi } from "@/api";

// 附件库选择弹窗：分页展示图片附件，点击回传 URL。
const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{
  (e: "select", url: string): void;
  (e: "close"): void;
}>();

interface AttachmentItem {
  id: number;
  filename: string;
  url: string;
  mime: string;
  size: number;
}

const items = ref<AttachmentItem[]>([]);
const page = ref(1);
const total = ref(0);
const size = 24;
const loading = ref(false);

// 分组过滤（Spec 44）：undefined=全部；0=未分组；>0=指定组
interface GroupItem {
  id: number;
  name: string;
  count: number;
}
const groups = ref<GroupItem[]>([]);
const activeGroup = ref<number | undefined>(undefined);

async function loadGroups() {
  try {
    const { data } = await attachmentGroupApi.list();
    groups.value = data.items || [];
  } catch {}
}

async function load() {
  loading.value = true;
  try {
    const params: Record<string, unknown> = { page: page.value, size };
    // 让后端按图片分页，避免混入文档后出现空页、页数不准。
    params.kind = "image";
    if (activeGroup.value !== undefined) params.group = activeGroup.value;
    const { data } = await attachmentApi.list(params);
    // 保留前端兜底过滤，兼容旧版或代理层忽略 kind 参数的情况。
    items.value = (data.items || []).filter((a: AttachmentItem) => a.mime?.startsWith("image/"));
    total.value = data.total || 0;
  } finally {
    loading.value = false;
  }
}

function onGroupChange(v: number | undefined) {
  activeGroup.value = v;
  if (page.value === 1) load();
  else page.value = 1;
}

const totalPages = () => Math.max(1, Math.ceil(total.value / size));

onMounted(() => {
  if (props.open) {
    loadGroups();
    load();
  }
});
watch(
  () => props.open,
  (v) => {
    if (v) {
      page.value = 1;
      activeGroup.value = undefined;
      loadGroups();
      load();
    }
  },
);
watch(page, load);

// 焦点与 Esc：打开时把焦点移入弹层、监听窗口 Esc，关闭后把焦点还给触发元素。
// 仅靠弹层元素上的 @keydown.esc 无法在焦点仍在触发按钮时关闭弹层。
const dialogRef = ref<HTMLElement | null>(null);
let previouslyFocusedElement: HTMLElement | null = null;

function handleWindowEscape(e: KeyboardEvent) {
  if (e.key === "Escape") emit("close");
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      previouslyFocusedElement = (document.activeElement as HTMLElement | null) ?? null;
      window.addEventListener("keydown", handleWindowEscape);
      void nextTick(() => dialogRef.value?.focus());
      return;
    }
    window.removeEventListener("keydown", handleWindowEscape);
    previouslyFocusedElement?.focus();
    previouslyFocusedElement = null;
  },
);

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleWindowEscape);
});
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      ref="dialogRef"
      tabindex="-1"
      class="fixed inset-0 z-50 flex items-center justify-center bg-background/40 p-4 outline-none"
      role="dialog"
      aria-modal="true"
      aria-label="从附件库选择图片"
      @click.self="emit('close')"
      @keydown.esc="emit('close')"
    >
      <div class="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-xl bg-card shadow-xl">
        <div class="flex items-center justify-between border-b border-border px-5 py-3">
          <h3 class="text-sm font-medium text-foreground">从附件库选择图片</h3>
          <div class="flex items-center gap-2">
            <select
              :value="activeGroup === undefined ? '' : String(activeGroup)"
              class="apple-select px-2 py-1 text-xs"
              @change="
                onGroupChange(
                  ($event.target as HTMLSelectElement).value === ''
                    ? undefined
                    : Number(($event.target as HTMLSelectElement).value),
                )
              "
            >
              <option value="">全部分组</option>
              <option value="0">未分组</option>
              <option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }}（{{ g.count }}）</option>
            </select>
            <button type="button" @click="emit('close')" class="text-text-quaternary hover:text-foreground">
              <Icon icon="ri:close-line" class="text-xl" />
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto p-5">
          <p v-if="loading" class="text-sm text-text-tertiary">加载中...</p>
          <p v-else-if="items.length === 0" class="text-sm text-text-tertiary">
            当前分组暂无图片附件，可先在「附件」页上传，或直接在编辑器里粘贴图片。
          </p>
          <div v-else class="grid grid-cols-3 gap-3 sm:grid-cols-4">
            <button
              v-for="item in items"
              :key="item.id"
              type="button"
              @click="emit('select', item.url)"
              class="group relative aspect-square overflow-hidden rounded-lg border border-border hover:border-chart-4 hover:ring-2 hover:ring-chart-4/30"
              :title="item.filename"
            >
              <img :src="item.url" :alt="item.filename" loading="lazy" class="h-full w-full object-cover" />
              <span
                class="absolute inset-x-0 bottom-0 truncate bg-background/50 px-1.5 py-0.5 text-[10px] text-foreground opacity-0 transition-opacity group-hover:opacity-100"
                >{{ item.filename }}</span
              >
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between border-t border-border px-5 py-3">
          <span class="text-xs text-text-tertiary">第 {{ page }} / {{ totalPages() }} 页</span>
          <div class="flex gap-2">
            <button
              type="button"
              :disabled="page <= 1"
              @click="page--"
              class="rounded-lg px-3 py-1 text-xs text-text-tertiary hover:bg-accent disabled:opacity-40"
            >
              上一页
            </button>
            <button
              type="button"
              :disabled="page >= totalPages()"
              @click="page++"
              class="rounded-lg px-3 py-1 text-xs text-text-tertiary hover:bg-accent disabled:opacity-40"
            >
              下一页
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
