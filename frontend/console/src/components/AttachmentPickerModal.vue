<script setup lang="ts">
import { ref, onMounted, watch } from"vue";
import { Icon } from"@iconify/vue";
import { attachmentApi, attachmentGroupApi } from"@/api";

// 图床选择弹窗：分页展示图片附件，点击回传 URL。
const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{
 (e:"select", url: string): void;
 (e:"close"): void;
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
interface GroupItem { id: number; name: string; count: number }
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
 if (activeGroup.value !== undefined) params.group = activeGroup.value;
 const { data } = await attachmentApi.list(params);
 // 仅展示图片类附件
 items.value = (data.items || []).filter((a: AttachmentItem) =>
 a.mime?.startsWith("image/")
 );
 total.value = data.total || 0;
 } finally {
 loading.value = false;
 }
}

function onGroupChange(v: number | undefined) {
 activeGroup.value = v;
 page.value = 1;
 load();
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
 }
);
watch(page, load);
</script>

<template>
 <div
 v-if="open"
 class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
 @click.self="emit('close')"
 >
 <div class="w-full max-w-2xl max-h-[80vh] flex flex-col bg-card rounded-xl shadow-xl">
 <div class="flex items-center justify-between px-5 py-3 border-b border-border">
 <h3 class="text-sm font-medium text-foreground">从图床选择图片</h3>
 <div class="flex items-center gap-2">
 <select
 :value="activeGroup === undefined ? '' : String(activeGroup)"
 class="apple-select px-2 py-1 text-xs"
 @change="onGroupChange(($event.target as HTMLSelectElement).value === '' ? undefined : Number(($event.target as HTMLSelectElement).value))"
 >
 <option value="">全部分组</option>
 <option value="0">未分组</option>
 <option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }}（{{ g.count }}）</option>
 </select>
 <button @click="emit('close')" class="text-text-quaternary hover:text-foreground">
 <Icon icon="ri:close-line" class="text-xl" />
 </button>
 </div>
 </div>

 <div class="flex-1 overflow-y-auto p-5">
 <p v-if="loading" class="text-sm text-text-tertiary">加载中...</p>
 <p v-else-if="items.length === 0" class="text-sm text-text-tertiary">
 本页暂无图片附件，可先在「附件」页上传，或直接在编辑器里粘贴图片。
 </p>
 <div v-else class="grid grid-cols-3 sm:grid-cols-4 gap-3">
 <button
 v-for="item in items"
 :key="item.id"
 @click="emit('select', item.url)"
 class="group relative aspect-square rounded-lg overflow-hidden border border-border hover:border-indigo-500 hover:ring-2 hover:ring-indigo-500/30"
 :title="item.filename"
 >
 <img :src="item.url" :alt="item.filename" loading="lazy" class="w-full h-full object-cover" />
 <span
 class="absolute inset-x-0 bottom-0 px-1.5 py-0.5 text-[10px] text-white bg-black/50 truncate opacity-0 group-hover:opacity-100 transition-opacity"
 >{{ item.filename }}</span>
 </button>
 </div>
 </div>

 <div class="flex items-center justify-between px-5 py-3 border-t border-border">
 <span class="text-xs text-text-tertiary">第 {{ page }} / {{ totalPages() }} 页</span>
 <div class="flex gap-2">
 <button
 :disabled="page <= 1"
 @click="page--"
 class="px-3 py-1 text-xs rounded-lg text-text-tertiary hover:bg-accent disabled:opacity-40"
 >上一页</button>
 <button
 :disabled="page >= totalPages()"
 @click="page++"
 class="px-3 py-1 text-xs rounded-lg text-text-tertiary hover:bg-accent disabled:opacity-40"
 >下一页</button>
 </div>
 </div>
 </div>
 </div>
</template>
