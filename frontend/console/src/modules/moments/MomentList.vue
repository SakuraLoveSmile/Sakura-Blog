<script setup lang="ts">
import { ref, computed, onMounted } from"vue";
import { useQuery, useQueryClient } from"@tanstack/vue-query";
import { momentApi, settingsApi, attachmentApi } from"@/api";
import { queryKeys } from"@/api/query-client";
import { Icon } from"@iconify/vue";
import AttachmentPickerModal from"@/components/AttachmentPickerModal.vue";
import {
 VPageHeader,
 VCard,
 VButton,
 VEntityContainer,
 VEntity,
 VEntityField,
 VEmpty,
 VLoading,
 VPagination,
 Dialog,
 Toast,
} from"@/components/ui";

const queryClient = useQueryClient();

const MAX_IMAGES = 9;

const page = ref(1);
const size = ref(20);
const menuOpenId = ref<number | null>(null);

// 发布框状态
const newContent = ref("");
const newImages = ref<string[]>([]);
const newVisibility = ref<"public" |"private">("public");
const publishing = ref(false);
const uploadError = ref("");

// 编辑弹窗状态
const editingId = ref<number | null>(null);
const editContent = ref("");
const editImages = ref<string[]>([]);
const editVisibility = ref<"public" |"private">("public");
const savingEdit = ref(false);

// 图床选择弹窗：target 决定回填到发布框还是编辑框
const pickerOpen = ref(false);
const pickerTarget = ref<"new" |"edit">("new");

const queryParams = computed(() => ({ page: page.value, size: size.value }));

const { data: queryData, isLoading, refetch } = useQuery({
 queryKey: computed(() => queryKeys.moments(queryParams.value)),
 queryFn: () => momentApi.list(queryParams.value).then(({ data }) => data),
});

const moments = computed(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

// 站长头像（发布框与列表项展示）
const ownerAvatar = ref("");
onMounted(async () => {
 try {
 const { data } = await settingsApi.all();
 ownerAvatar.value = data?.site?.avatar ||"";
 } catch {
 /* 静默 */
 }
});

// 发布框：聚焦后展开；字数统计（非空白 rune）
const composerFocused = ref(false);
const charCount = computed(() => {
 let n = 0;
 for (const ch of newContent.value) {
 if (!/\s/.test(ch)) n++;
 }
 return n;
});

const canPublish = computed(
 () => !!newContent.value.trim() || newImages.value.length > 0
);

// Cmd/Ctrl+Enter 快捷发布
function onComposerKeydown(e: KeyboardEvent) {
 if ((e.metaKey || e.ctrlKey) && e.key ==="Enter") {
 e.preventDefault();
 handlePublish();
 }
}

// --- 选图 / 上传 / 预览（发布框与编辑框共用工具函数） ---

function targetImages(target:"new" |"edit") {
 return target ==="new" ? newImages : editImages;
}

function addImage(target:"new" |"edit", url: string) {
 const box = targetImages(target);
 if (!url || box.value.includes(url)) return;
 if (box.value.length >= MAX_IMAGES) {
 Toast.warning(`单条最多 ${MAX_IMAGES} 张图片`);
 return;
 }
 box.value = [...box.value, url];
}

function removeImage(target:"new" |"edit", idx: number) {
 const box = targetImages(target);
 box.value = box.value.filter((_, i) => i !== idx);
}

function openPicker(target:"new" |"edit") {
 pickerTarget.value = target;
 pickerOpen.value = true;
}

function onPickerSelect(url: string) {
 addImage(pickerTarget.value, url);
 pickerOpen.value = false;
}

// 粘贴直传：参照 MarkdownEditor.onPaste，取剪贴板图片文件上传后回填 URL
async function uploadFile(target:"new" |"edit", file: File) {
 uploadError.value ="";
 const box = targetImages(target);
 if (box.value.length >= MAX_IMAGES) {
 Toast.warning(`单条最多 ${MAX_IMAGES} 张图片`);
 return;
 }
 try {
 const { data } = await attachmentApi.upload(file);
 addImage(target, data.url);
 } catch (e: any) {
 uploadError.value = e.response?.data?.error || `图片 ${file.name} 上传失败`;
 }
}

function onPaste(target:"new" |"edit", e: ClipboardEvent) {
 const files = Array.from(e.clipboardData?.files || []).filter((f) =>
 f.type.startsWith("image/")
 );
 if (files.length === 0) return;
 e.preventDefault();
 files.forEach((f) => uploadFile(target, f));
}

// 隐藏 file input（本地选择直传）
const newFileInput = ref<HTMLInputElement | null>(null);
const editFileInput = ref<HTMLInputElement | null>(null);
function onPickFiles(target:"new" |"edit", e: Event) {
 const input = e.target as HTMLInputElement;
 for (const file of Array.from(input.files || [])) {
 if (file.type.startsWith("image/")) uploadFile(target, file);
 }
 input.value ="";
}

async function handlePublish() {
 if (!canPublish.value) return;
 publishing.value = true;
 try {
 await momentApi.create({
 content_md: newContent.value,
 images: newImages.value,
 visibility: newVisibility.value,
 });
 newContent.value ="";
 newImages.value = [];
 newVisibility.value ="public";
 queryClient.invalidateQueries({ queryKey: ["moments"] });
 Toast.success("发布成功");
 } catch {
 } finally {
 publishing.value = false;
 }
}

// --- 编辑 ---

function startEdit(m: any) {
 menuOpenId.value = null;
 editingId.value = m.id;
 editContent.value = m.content_md ||"";
 editImages.value = Array.isArray(m.images) ? [...m.images] : [];
 editVisibility.value = m.visibility ==="private" ?"private" :"public";
}

function cancelEdit() {
 editingId.value = null;
 editContent.value ="";
 editImages.value = [];
 savingEdit.value = false;
}

async function saveEdit() {
 if (editingId.value == null) return;
 if (!editContent.value.trim() && editImages.value.length === 0) {
 Toast.warning("内容不能为空");
 return;
 }
 savingEdit.value = true;
 try {
 await momentApi.update(editingId.value, {
 content_md: editContent.value,
 images: editImages.value,
 visibility: editVisibility.value,
 });
 queryClient.invalidateQueries({ queryKey: ["moments"] });
 Toast.success("已保存");
 cancelEdit();
 } catch {
 } finally {
 savingEdit.value = false;
 }
}

// 列表快捷切换可见性（不进编辑框）
async function toggleVisibility(m: any) {
 menuOpenId.value = null;
 const next = m.visibility ==="private" ?"public" :"private";
 try {
 await momentApi.update(m.id, {
 content_md: m.content_md,
 images: Array.isArray(m.images) ? m.images : [],
 visibility: next,
 });
 queryClient.invalidateQueries({ queryKey: ["moments"] });
 Toast.success(next ==="private" ?"已设为私密" :"已设为公开");
 } catch {}
}

function handleDelete(m: any) {
 menuOpenId.value = null;
 Dialog.warning({
 title:"确定要删除该瞬间吗？",
 description:"该操作不可恢复",
 confirmType:"danger",
 onConfirm: async () => {
 await momentApi.delete(m.id);
 queryClient.invalidateQueries({ queryKey: ["moments"] });
 Toast.success("删除成功");
 },
 });
}

function formatTime(s: string): string {
 if (!s) return"";
 return s.slice(0, 16).replace("T","");
}

// 相对时间：列表项展示用（超过 30 天回退绝对时间）
function formatRelative(s: string): string {
 if (!s) return"";
 const t = new Date(s).getTime();
 if (isNaN(t)) return formatTime(s);
 const diff = Date.now() - t;
 const min = Math.floor(diff / 60000);
 if (min < 1) return"刚刚";
 if (min < 60) return `${min} 分钟前`;
 const hour = Math.floor(min / 60);
 if (hour < 24) return `${hour} 小时前`;
 const day = Math.floor(hour / 24);
 if (day <= 30) return `${day} 天前`;
 return formatTime(s);
}

function closeMenus() {
 menuOpenId.value = null;
}
</script>

<template>
 <div @click="closeMenus">
 <VPageHeader title="瞬间">
 <template #icon>
 <Icon icon="ri:quill-pen-line" class="mr-2 self-center text-xl" />
 </template>
 </VPageHeader>

 <div class="m-0 space-y-4 md:m-4">
 <!-- 发布框（Halo Moments 风格：头像 + 无边框输入 + 底部工具条） -->
 <VCard :body-class="['!p-4']">
 <div class="flex gap-3">
 <img
 v-if="ownerAvatar"
 :src="ownerAvatar"
 alt=""
 class="h-9 w-9 shrink-0 rounded-full border border-border object-cover"
 />
 <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/15">
 <Icon icon="ri:user-3-line" class="text-lg text-primary" />
 </span>
 <div class="min-w-0 flex-1">
 <textarea
 v-model="newContent"
 :rows="composerFocused || newContent ? 4 : 2"
 placeholder="分享此刻的想法…（可粘贴图片直传）"
 class="w-full resize-none border-0 bg-transparent px-0 py-1.5 text-sm text-foreground outline-none transition-all placeholder:text-text-quaternary"
 @focus="composerFocused = true"
 @blur="composerFocused = false"
 @keydown="onComposerKeydown"
 @paste="onPaste('new', $event)"
 ></textarea>

 <!-- 已选图片九宫格预览 -->
 <div v-if="newImages.length" class="mt-2 grid grid-cols-3 gap-2 sm:grid-cols-4">
 <div
 v-for="(url, i) in newImages"
 :key="url"
 class="group relative aspect-square overflow-hidden rounded-lg border border-border"
 >
 <img :src="url" alt="" class="h-full w-full object-cover" />
 <button
 type="button"
 class="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/50 text-white opacity-0 transition-opacity group-hover:opacity-100"
 @click.stop="removeImage('new', i)"
 >
 <Icon icon="ri:close-line" class="text-sm" />
 </button>
 </div>
 </div>

 <p v-if="uploadError" class="mt-2 text-xs text-red-600 text-red-400">{{ uploadError }}</p>

 <div class="mt-2 flex items-center justify-between border-t border-border pt-2.5">
 <div class="flex items-center gap-3 text-text-quaternary">
 <button
 type="button"
 class="flex items-center gap-1 rounded p-1 transition-colors hover:text-primary"
 title="从图床选择图片"
 @click.stop="openPicker('new')"
 >
 <Icon icon="ri:image-add-line" class="text-base" />
 </button>
 <button
 type="button"
 class="flex items-center gap-1 rounded p-1 transition-colors hover:text-primary"
 title="上传本地图片"
 @click.stop="newFileInput?.click()"
 >
 <Icon icon="ri:upload-2-line" class="text-base" />
 </button>
 <input ref="newFileInput" type="file" accept="image/*" multiple class="hidden" @change="onPickFiles('new', $event)" />
 <button
 type="button"
 class="flex items-center gap-1 rounded px-1.5 py-1 text-xs transition-colors hover:text-primary"
 :class="newVisibility === 'private' ? 'text-amber-500' : ''"
 :title="newVisibility === 'private' ? '仅自己可见，点击设为公开' : '所有人可见，点击设为私密'"
 @click.stop="newVisibility = newVisibility === 'private' ? 'public' : 'private'"
 >
 <Icon :icon="newVisibility === 'private' ? 'ri:lock-line' : 'ri:earth-line'" class="text-base" />
 <span>{{ newVisibility ==="private" ?"私密" :"公开" }}</span>
 </button>
 <span v-if="charCount" class="text-xs tabular-nums">{{ charCount }} 字</span>
 </div>
 <div class="flex items-center gap-3">
 <span class="hidden text-xs text-text-quaternary sm:inline">⌘ + Enter 发布</span>
 <VButton
 type="secondary"
 size="sm"
 :loading="publishing"
 :disabled="!canPublish"
 @click="handlePublish"
 >
 <template #icon>
 <Icon icon="ri:send-plane-fill" class="h-full w-full" />
 </template>
 发布
 </VButton>
 </div>
 </div>
 </div>
 </div>
 </VCard>

 <!-- Moment list -->
 <VCard :body-class="['!p-0']">
 <VLoading v-if="isLoading" />

 <Transition v-else-if="!moments.length" appear name="fade">
 <VEmpty message="发布第一条瞬间吧" title="当前没有瞬间">
 <template #actions>
 <VButton @click="refetch">刷新</VButton>
 </template>
 </VEmpty>
 </Transition>

 <Transition v-else appear name="fade">
 <VEntityContainer>
 <VEntity v-for="m in moments" :key="m.id">
 <template #start>
 <VEntityField max-width="40rem">
 <template #description>
 <div class="flex gap-3">
 <img
 v-if="ownerAvatar"
 :src="ownerAvatar"
 alt=""
 class="mt-0.5 h-7 w-7 shrink-0 rounded-full border border-border object-cover"
 />
 <div class="flex min-w-0 flex-col gap-1.5">
 <span class="flex items-center gap-1.5 text-xs text-text-quaternary" :title="formatTime(m.created_at)">
 {{ formatRelative(m.created_at) }}
 <span
 v-if="m.visibility === 'private'"
 class="inline-flex items-center gap-0.5 rounded bg-amber-100 px-1 py-0.5 text-[10px] font-medium text-amber-600 bg-amber-500/15 text-amber-400"
 title="私密：仅后台可见，前台不展示"
 >
 <Icon icon="ri:lock-line" class="text-[11px]" />私密
 </span>
 </span>
 <div
 v-if="m.content"
 class="prose prose-sm max-w-none text-text-secondary"
 v-html="m.content"
 ></div>
 <!-- 缩略图行 -->
 <div v-if="m.images && m.images.length" class="mt-1 flex flex-wrap gap-1.5">
 <img
 v-for="url in m.images"
 :key="url"
 :src="url"
 alt=""
 loading="lazy"
 class="h-16 w-16 rounded border border-border object-cover"
 />
 </div>
 </div>
 </div>
 </template>
 </VEntityField>
 </template>
 <template #dropdownItems>
 <div class="relative">
 <div
 class="cursor-pointer rounded p-1 transition-all hover:bg-accent/60 hover:text-blue-600"
 @click.stop="menuOpenId = menuOpenId === m.id ? null : m.id"
 >
 <Icon icon="ri:more-fill" />
 </div>
 <Transition name="fade">
 <div
 v-if="menuOpenId === m.id"
 class="absolute right-0 top-full z-30 mt-1 w-36 space-y-1 rounded-base border border-border bg-card p-2 shadow-lg"
 @click.stop
 >
 <VButton block size="sm" @click="startEdit(m)">编辑</VButton>
 <VButton block size="sm" @click="toggleVisibility(m)">
 {{ m.visibility ==="private" ?"设为公开" :"设为私密" }}
 </VButton>
 <VButton block type="danger" size="sm" @click="handleDelete(m)">删除</VButton>
 </div>
 </Transition>
 </div>
 </template>
 </VEntity>
 </VEntityContainer>
 </Transition>

 <template #footer>
 <div class="bg-card sm:flex sm:items-center sm:justify-end">
 <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 30, 50, 100]" />
 </div>
 </template>
 </VCard>
 </div>

 <!-- 编辑弹窗 -->
 <div
 v-if="editingId !== null"
 class="fixed inset-0 z-40 flex items-center justify-center bg-black/40 p-4"
 @click.self="cancelEdit"
 >
 <div class="flex max-h-[85vh] w-full max-w-lg flex-col rounded-xl bg-card shadow-xl">
 <div class="flex items-center justify-between border-b border-border px-5 py-3">
 <h3 class="text-sm font-medium text-foreground">编辑瞬间</h3>
 <button @click="cancelEdit" class="text-text-quaternary hover:text-text-tertiary hover:text-foreground">
 <Icon icon="ri:close-line" class="text-xl" />
 </button>
 </div>
 <div class="flex-1 space-y-3 overflow-y-auto p-5">
 <textarea
 v-model="editContent"
 rows="4"
 placeholder="分享此刻的想法…（可粘贴图片直传）"
 class="w-full resize-none rounded-lg border border-border bg-transparent p-2.5 text-sm text-foreground outline-none focus:border-primary"
 @paste="onPaste('edit', $event)"
 ></textarea>

 <div v-if="editImages.length" class="grid grid-cols-3 gap-2 sm:grid-cols-4">
 <div
 v-for="(url, i) in editImages"
 :key="url"
 class="group relative aspect-square overflow-hidden rounded-lg border border-border"
 >
 <img :src="url" alt="" class="h-full w-full object-cover" />
 <button
 type="button"
 class="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/50 text-white opacity-0 transition-opacity group-hover:opacity-100"
 @click.stop="removeImage('edit', i)"
 >
 <Icon icon="ri:close-line" class="text-sm" />
 </button>
 </div>
 </div>

 <div class="flex items-center gap-3 text-text-quaternary">
 <button type="button" class="flex items-center gap-1 rounded p-1 hover:text-primary" title="从图床选择图片" @click.stop="openPicker('edit')">
 <Icon icon="ri:image-add-line" class="text-base" />
 </button>
 <button type="button" class="flex items-center gap-1 rounded p-1 hover:text-primary" title="上传本地图片" @click.stop="editFileInput?.click()">
 <Icon icon="ri:upload-2-line" class="text-base" />
 </button>
 <input ref="editFileInput" type="file" accept="image/*" multiple class="hidden" @change="onPickFiles('edit', $event)" />
 <button
 type="button"
 class="flex items-center gap-1 rounded px-1.5 py-1 text-xs hover:text-primary"
 :class="editVisibility === 'private' ? 'text-amber-500' : ''"
 @click.stop="editVisibility = editVisibility === 'private' ? 'public' : 'private'"
 >
 <Icon :icon="editVisibility === 'private' ? 'ri:lock-line' : 'ri:earth-line'" class="text-base" />
 <span>{{ editVisibility ==="private" ?"私密" :"公开" }}</span>
 </button>
 </div>
 </div>
 <div class="flex justify-end gap-2 border-t border-border px-5 py-3">
 <VButton size="sm" @click="cancelEdit">取消</VButton>
 <VButton type="secondary" size="sm" :loading="savingEdit" @click="saveEdit">保存</VButton>
 </div>
 </div>
 </div>

 <!-- 图床选择弹窗（发布框 / 编辑框共用） -->
 <AttachmentPickerModal :open="pickerOpen" @select="onPickerSelect" @close="pickerOpen = false" />
 </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
 transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
 opacity: 0;
}
</style>
