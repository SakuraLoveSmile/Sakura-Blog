<script setup lang="ts">
import { ref, computed, onMounted, nextTick, reactive } from"vue";
import { useQuery, useQueryClient } from"@tanstack/vue-query";
import { momentApi, settingsApi, attachmentApi } from"@/api";
import { queryKeys } from"@/api/query-client";
import { Icon } from"@iconify/vue";
import AttachmentPickerModal from"@/components/AttachmentPickerModal.vue";
import {
  VCard,
  VButton,
  VEmpty,
  VLoading,
  VPagination,
  Dialog,
  Toast,
} from "@/components/ui";

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

// 时间线节点色：蓝/橙/绿/紫/红（按 id hash 取色）
const DOT_COLORS = [
  { bg: "#2E8DFF", glow: "color-mix(in srgb, var(--color-primary) 45%, transparent)" },
  { bg: "#FF9F0A", glow: "color-mix(in srgb, var(--color-state-warning) 45%, transparent)" },
  { bg: "#30D158", glow: "color-mix(in srgb, var(--color-state-success) 45%, transparent)" },
  { bg: "#BF5AF2", glow: "color-mix(in srgb, var(--color-chart-5) 45%, transparent)" },
  { bg: "#FF453A", glow: "color-mix(in srgb, var(--color-state-error) 45%, transparent)" },
];
function dotStyle(id: number) {
  const c = DOT_COLORS[Math.abs(Number(id) || 0) % DOT_COLORS.length];
  return {
    background: c.bg,
    boxShadow: `0 0 0 3px var(--color-background, #000), 0 0 10px ${c.glow}`,
  };
}
// 头像渐变：avatar-gradient-1..8，按 id hash 取
function avatarClass(id: number) {
  const n = (Math.abs(Number(id) || 0) % 8) + 1;
  return `avatar-gradient-${n}`;
}
function avatarText(m: any) {
  const name: string = m?.author_name || m?.author || "A";
  return String(name).trim().charAt(0).toUpperCase() || "A";
}

// 按日期分组：今天 / 昨天 / 本周 / 更早
type DateGroup = { key: string; label: string; items: any[] };
const groupedMoments = computed<DateGroup[]>(() => {
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const dayMs = 86400000;
  const startOfWeek = (() => {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const w = d.getDay(); // 0=Sun
    const offset = w === 0 ? 6 : w - 1; // Monday start
    d.setDate(d.getDate() - offset);
    return d.getTime();
  })();

  const buckets: Record<string, any[]> = {
    today: [],
    yesterday: [],
    week: [],
    earlier: [],
  };
  for (const m of moments.value) {
    const t = new Date(m?.created_at).getTime();
    if (isNaN(t)) {
      buckets.earlier.push(m);
      continue;
    }
    if (t >= startOfToday) buckets.today.push(m);
    else if (t >= startOfToday - dayMs) buckets.yesterday.push(m);
    else if (t >= startOfWeek) buckets.week.push(m);
    else buckets.earlier.push(m);
  }
  const groups: DateGroup[] = [];
  if (buckets.today.length) groups.push({ key: "today", label: "今天", items: buckets.today });
  if (buckets.yesterday.length) groups.push({ key: "yesterday", label: "昨天", items: buckets.yesterday });
  if (buckets.week.length) groups.push({ key: "week", label: "本周", items: buckets.week });
  if (buckets.earlier.length) groups.push({ key: "earlier", label: "更早", items: buckets.earlier });
  return groups;
});

// 站长头像（发布框与列表项展示）
const ownerAvatar = ref("");

// 发布框常驻顶部：头部「发布瞬间」按钮用于滚动并聚焦输入框
const composerBox = ref<HTMLTextAreaElement | null>(null);
function focusComposer() {
  composerBox.value?.scrollIntoView({ block: "center", behavior: "smooth" });
  composerBox.value?.focus({ preventScroll: true });
}
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

// --- 表情面板（发布框与编辑框共用；与评论区 cmt-emoji-panel 完全一致：CDN 表情包/底部 tabs/网格/插入逻辑） ---
// 与评论区同源：cdn-x/emoticons Artalk 格式清单（items: [{key, val}]，val 为图片直链）
const EMOJI_BASE = "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/artalk/";
const EMOJI_PACKS = [
  { key: "blobcat", label: "Blobcat" },
  { key: "qq", label: "QQ" },
  { key: "tieba", label: "Tieba" },
  { key: "aru", label: "Aru" },
  { key: "twemoji", label: "Twemoji" },
];
type EmojiItem = { key: string; val: string };
const emojiOpen = ref<"new" | "edit" | null>(null);
const emojiPack = ref("blobcat");
const emojiLoading = ref(false);
const emojiError = ref(false);
const packCache: Record<string, EmojiItem[]> = reactive({});
const editBox = ref<HTMLTextAreaElement | null>(null);
const currentEmojis = computed<EmojiItem[]>(() => packCache[emojiPack.value] || []);

// 懒加载 pack 清单（首次打开/切 tab），缓存后直接渲染
async function ensurePackLoaded(pack: string) {
  if (packCache[pack]?.length) return;
  emojiLoading.value = true;
  emojiError.value = false;
  try {
    const r = await fetch(EMOJI_BASE + pack + ".json", { cache: "force-cache" });
    if (!r.ok) throw new Error("http " + r.status);
    const data = await r.json();
    const items = Array.isArray(data?.items) ? (data.items as EmojiItem[]).filter((it) => it?.val) : [];
    if (!items.length) throw new Error("empty");
    packCache[pack] = items;
  } catch {
    emojiError.value = true;
  } finally {
    emojiLoading.value = false;
  }
}

function switchEmojiPack(pack: string) {
  emojiPack.value = pack;
  ensurePackLoaded(pack);
}

function toggleEmoji(target: "new" | "edit") {
  const open = emojiOpen.value === target;
  emojiOpen.value = open ? null : target;
  if (!open) ensurePackLoaded(emojiPack.value);
}

// 与评论区一致：插入 Markdown 图片 ![emoji](src)，并恢复光标位置
function insertEmoji(target: "new" | "edit", src: string) {
  const md = `![emoji](${src})`;
  const box = target === "new" ? newContent : editContent;
  const el = target === "new" ? composerBox.value : editBox.value;
  const start = el?.selectionStart ?? box.value.length;
  const end = el?.selectionEnd ?? start;
  box.value = box.value.slice(0, start) + md + box.value.slice(end);
  nextTick(() => {
    if (!el) return;
    el.focus();
    el.selectionStart = el.selectionEnd = start + md.length;
  });
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
    newContent.value = "";
    newImages.value = [];
    newVisibility.value = "public";
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
 emojiOpen.value = null;
}
</script>

<template>
  <div class="moments-page" @click="closeMenus">
    <!-- Page Header -->
    <header class="moments-page-header">
      <div class="min-w-0 flex-1">
        <h1 class="moments-page-title">瞬间</h1>
      </div>
      <div class="flex items-center gap-3">
        <button type="button" class="moments-btn-primary" @click="focusComposer">
          <Icon icon="ri:add-line" class="text-[15px]" />
          <span>发布瞬间</span>
        </button>
      </div>
    </header>

    <!-- Content Area -->
    <div class="moments-content-area">
      <div class="moments-container mx-auto w-full max-w-[680px]">
        <!-- Composer card：常驻顶部 -->
        <VCard :body-class="['!p-4']" class="moment-card-apple mb-5">
          <div class="flex gap-3">
            <img
              v-if="ownerAvatar"
              :src="ownerAvatar"
              alt=""
              class="h-10 w-10 shrink-0 rounded-full border border-border object-cover"
            />
            <span
              v-else
              class="avatar-gradient-1 flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-[14px] font-semibold text-white"
            >
              <Icon icon="ri:user-3-line" class="text-lg" />
            </span>
            <div class="min-w-0 flex-1">
              <textarea
                ref="composerBox"
                v-model="newContent"
                :rows="composerFocused || newContent ? 4 : 2"
                placeholder="分享此刻的想法…（可粘贴图片直传）"
                class="moment-textarea w-full resize-none border-0 bg-transparent px-0 py-1.5 text-[14px] leading-[1.6] text-foreground outline-none transition-all placeholder:text-text-quaternary"
                @focus="composerFocused = true"
                @blur="composerFocused = false"
                @keydown="onComposerKeydown"
                @paste="onPaste('new', $event)"
              ></textarea>

              <div v-if="newImages.length" class="moment-images mt-3">
                <div
                  v-for="(url, i) in newImages"
                  :key="url"
                  class="group relative aspect-square overflow-hidden rounded-[10px]"
                >
                  <img :src="url" alt="" class="h-full w-full object-cover" />
                  <button
                    type="button"
                    class="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100"
                    @click.stop="removeImage('new', i)"
                  >
                    <Icon icon="ri:close-line" class="text-sm" />
                  </button>
                </div>
              </div>

              <p v-if="uploadError" class="mt-2 text-xs text-[var(--color-destructive)]">{{ uploadError }}</p>

              <div class="mt-3 flex items-center justify-between border-t border-border pt-2.5">
                <div class="flex items-center gap-1 text-text-quaternary">
                  <button
                    type="button"
                    class="flex h-7 w-7 items-center justify-center rounded-md transition-colors hover:bg-accent hover:text-primary"
                    title="从图床选择图片"
                    @click.stop="openPicker('new')"
                  >
                    <Icon icon="ri:image-add-line" class="text-base" />
                  </button>
                  <button
                    type="button"
                    class="flex h-7 w-7 items-center justify-center rounded-md transition-colors hover:bg-accent hover:text-primary"
                    title="上传本地图片"
                    @click.stop="newFileInput?.click()"
                  >
                    <Icon icon="ri:upload-2-line" class="text-base" />
                  </button>
                  <button
                    type="button"
                    class="emoji-tool-btn"
                    :class="{ 'is-active': emojiOpen === 'new' }"
                    title="插入表情"
                    @click.stop="toggleEmoji('new')"
                  >
                    <Icon icon="ri:emotion-line" class="text-[20px]" />
                  </button>
                  <input
                    ref="newFileInput"
                    type="file"
                    accept="image/*"
                    multiple
                    class="hidden"
                    @change="onPickFiles('new', $event)"
                  />
                  <button
                    type="button"
                    class="flex h-7 items-center gap-1 rounded-md px-2 text-xs transition-colors hover:bg-accent"
                    :class="newVisibility === 'private' ? 'text-state-warning' : 'hover:text-primary'"
                    :title="newVisibility === 'private' ? '仅自己可见，点击设为公开' : '所有人可见，点击设为私密'"
                    @click.stop="newVisibility = newVisibility === 'private' ? 'public' : 'private'"
                  >
                    <Icon :icon="newVisibility === 'private' ? 'ri:lock-line' : 'ri:earth-line'" class="text-base" />
                    <span>{{ newVisibility === "private" ? "私密" : "公开" }}</span>
                  </button>
                  <span v-if="charCount" class="ml-2 text-xs tabular-nums">{{ charCount }} 字</span>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    v-if="newContent || newImages.length"
                    type="button"
                    class="rounded-full px-3 py-1.5 text-xs text-text-tertiary transition-colors hover:text-foreground"
                    @click="newContent = ''; newImages = []"
                  >
                    清空
                  </button>
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

          <!-- 表情面板：发布框（与评论区一致：body 网格 + 底部 tabs） -->
          <div v-if="emojiOpen === 'new'" class="emoji-panel" @click.stop>
            <div class="emoji-body">
              <div v-if="emojiLoading" class="emoji-loading">
                <Icon icon="ri:loader-4-line" class="animate-spin" />
                <span>加载中…</span>
              </div>
              <div v-else-if="emojiError" class="emoji-error" title="点击重试" @click.stop="ensurePackLoaded(emojiPack)">表情包加载失败，请检查网络后重试</div>
              <div v-else class="emoji-grid">
                <button
                  v-for="it in currentEmojis"
                  :key="it.val"
                  type="button"
                  class="emoji-cell"
                  :title="it.key"
                  @click.stop="insertEmoji('new', it.val)"
                >
                  <img class="emoji-img" :src="it.val" :alt="it.key" loading="lazy" />
                </button>
              </div>
            </div>
            <div class="emoji-tabs" role="tablist">
              <button
                v-for="p in EMOJI_PACKS"
                :key="p.key"
                type="button"
                role="tab"
                :aria-selected="emojiPack === p.key"
                :class="['emoji-tab', { 'is-active': emojiPack === p.key }]"
                @click.stop="switchEmojiPack(p.key)"
              >{{ p.label }}</button>
            </div>
          </div>
        </VCard>

        <VLoading v-if="isLoading" />

        <Transition v-else-if="!moments.length" appear name="fade">
          <VEmpty message="发布第一条瞬间吧" title="当前没有瞬间">
            <template #actions>
              <VButton @click="refetch">刷新</VButton>
            </template>
          </VEmpty>
        </Transition>

        <template v-else>
          <template v-for="group in groupedMoments" :key="group.key">
            <div class="date-separator">
              <span class="date-separator-text">{{ group.label }}</span>
            </div>

            <div v-for="m in group.items" :key="m.id" class="moment-item">
              <div class="moment-timeline-dot">
                <div class="moment-dot" :style="dotStyle(m.id)"></div>
              </div>

              <div class="moment-card apple-card-highlight">
                <div class="moment-header">
                  <img
                    v-if="ownerAvatar"
                    :src="ownerAvatar"
                    alt=""
                    class="h-10 w-10 shrink-0 rounded-full border border-border object-cover"
                  />
                  <div
                    v-else
                    :class="['h-10 w-10 shrink-0 rounded-full flex items-center justify-center text-white text-[14px] font-semibold', avatarClass(m.id)]"
                  >
                    {{ avatarText(m) }}
                  </div>
                  <div class="moment-author-wrap">
                    <div class="moment-author">
                      <span>{{ m.author_name || m.author || "Admin" }}</span>
                      <span
                        v-if="m.visibility === 'private'"
                        class="inline-flex items-center gap-0.5 rounded-full bg-[var(--color-warning)]/15 px-1.5 py-0.5 text-[10px] font-medium text-state-warning"
                        title="私密：仅后台可见，前台不展示"
                      >
                        <Icon icon="ri:lock-line" class="text-[11px]" />私密
                      </span>
                    </div>
                    <div class="moment-time" :title="formatTime(m.created_at)">
                      {{ formatRelative(m.created_at) }}
                    </div>
                  </div>

                  <div class="relative">
                    <button
                      type="button"
                      class="moment-more-btn"
                      @click.stop="menuOpenId = menuOpenId === m.id ? null : m.id"
                    >
                      <Icon icon="ri:more-2-fill" />
                    </button>
                    <Transition name="fade">
                      <div
                        v-if="menuOpenId === m.id"
                        class="absolute right-0 top-full z-30 mt-1 w-36 space-y-1 rounded-[10px] border border-border bg-card p-1.5 shadow-lg"
                        @click.stop
                      >
                        <VButton block size="sm" @click="startEdit(m)">编辑</VButton>
                        <VButton block size="sm" @click="toggleVisibility(m)">
                          {{ m.visibility === "private" ? "设为公开" : "设为私密" }}
                        </VButton>
                        <VButton block type="danger" size="sm" @click="handleDelete(m)">删除</VButton>
                      </div>
                    </Transition>
                  </div>
                </div>

                <div
                  v-if="m.content"
                  class="moment-content prose prose-sm max-w-none"
                  v-html="m.content"
                ></div>

                <div v-if="m.images && m.images.length" class="moment-images">
                  <div
                    v-for="url in m.images"
                    :key="url"
                    class="aspect-square overflow-hidden rounded-[10px] bg-secondary"
                  >
                    <img :src="url" alt="" loading="lazy" class="h-full w-full object-cover transition-transform duration-300 hover:scale-[1.03]" />
                  </div>
                </div>

                <div class="moment-actions">
                  <button type="button" class="moment-action">
                    <Icon icon="ri:heart-3-line" />
                    <span>点赞</span>
                  </button>
                  <button type="button" class="moment-action">
                    <Icon icon="ri:chat-3-line" />
                    <span>评论</span>
                  </button>
                  <button type="button" class="moment-action">
                    <Icon icon="ri:share-forward-line" />
                    <span>分享</span>
                  </button>
                </div>
              </div>
            </div>
          </template>
        </template>

        <div
          v-if="total > size"
          class="mt-6 flex items-center justify-center border-t border-border pt-4"
        >
          <VPagination
            v-model:page="page"
            v-model:size="size"
            :total="total"
            :size-options="[20, 30, 50, 100]"
          />
        </div>
      </div>
    </div>

    <!-- 编辑弹窗 -->
    <div
      v-if="editingId !== null"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="cancelEdit"
    >
      <div class="flex max-h-[85vh] w-full max-w-lg flex-col overflow-hidden rounded-[14px] border border-border bg-card shadow-2xl">
        <div class="flex items-center justify-between border-b border-border px-5 py-3">
          <h3 class="text-sm font-semibold text-foreground">编辑瞬间</h3>
          <button
            class="flex h-7 w-7 items-center justify-center rounded-md text-text-quaternary transition-colors hover:bg-accent hover:text-foreground"
            @click="cancelEdit"
          >
            <Icon icon="ri:close-line" class="text-xl" />
          </button>
        </div>
        <div class="flex-1 space-y-3 overflow-y-auto p-5">
          <textarea
            ref="editBox"
            v-model="editContent"
            rows="4"
            placeholder="分享此刻的想法…（可粘贴图片直传）"
            class="apple-textarea moment-textarea w-full"
            @paste="onPaste('edit', $event)"
          ></textarea>

          <div v-if="editImages.length" class="grid grid-cols-3 gap-1.5 sm:grid-cols-4">
            <div
              v-for="(url, i) in editImages"
              :key="url"
              class="group relative aspect-square overflow-hidden rounded-[10px] border border-border"
            >
              <img :src="url" alt="" class="h-full w-full object-cover" />
              <button
                type="button"
                class="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100"
                @click.stop="removeImage('edit', i)"
              >
                <Icon icon="ri:close-line" class="text-sm" />
              </button>
            </div>
          </div>

          <div class="flex items-center gap-1 text-text-quaternary">
            <button
              type="button"
              class="flex h-7 w-7 items-center justify-center rounded-md transition-colors hover:bg-accent hover:text-primary"
              title="从图床选择图片"
              @click.stop="openPicker('edit')"
            >
              <Icon icon="ri:image-add-line" class="text-base" />
            </button>
            <button
              type="button"
              class="flex h-7 w-7 items-center justify-center rounded-md transition-colors hover:bg-accent hover:text-primary"
              title="上传本地图片"
              @click.stop="editFileInput?.click()"
            >
              <Icon icon="ri:upload-2-line" class="text-base" />
            </button>
            <button
              type="button"
              class="emoji-tool-btn"
              :class="{ 'is-active': emojiOpen === 'edit' }"
              title="插入表情"
              @click.stop="toggleEmoji('edit')"
            >
              <Icon icon="ri:emotion-line" class="text-[20px]" />
            </button>
            <input
              ref="editFileInput"
              type="file"
              accept="image/*"
              multiple
              class="hidden"
              @change="onPickFiles('edit', $event)"
            />
            <button
              type="button"
              class="flex h-7 items-center gap-1 rounded-md px-2 text-xs transition-colors hover:bg-accent"
              :class="editVisibility === 'private' ? 'text-state-warning' : 'hover:text-primary'"
              @click.stop="editVisibility = editVisibility === 'private' ? 'public' : 'private'"
            >
              <Icon :icon="editVisibility === 'private' ? 'ri:lock-line' : 'ri:earth-line'" class="text-base" />
              <span>{{ editVisibility === "private" ? "私密" : "公开" }}</span>
            </button>
          </div>
        </div>

        <!-- 表情面板：编辑框 -->
        <div v-if="emojiOpen === 'edit'" class="emoji-panel" @click.stop>
          <div class="emoji-body">
            <div v-if="emojiLoading" class="emoji-loading">
              <Icon icon="ri:loader-4-line" class="animate-spin" />
              <span>加载中…</span>
            </div>
            <div v-else-if="emojiError" class="emoji-error" title="点击重试" @click.stop="ensurePackLoaded(emojiPack)">表情包加载失败，请检查网络后重试</div>
            <div v-else class="emoji-grid">
              <button
                v-for="it in currentEmojis"
                :key="it.val"
                type="button"
                class="emoji-cell"
                :title="it.key"
                @click.stop="insertEmoji('edit', it.val)"
              >
                <img class="emoji-img" :src="it.val" :alt="it.key" loading="lazy" />
              </button>
            </div>
          </div>
          <div class="emoji-tabs" role="tablist">
            <button
              v-for="p in EMOJI_PACKS"
              :key="p.key"
              type="button"
              role="tab"
              :aria-selected="emojiPack === p.key"
              :class="['emoji-tab', { 'is-active': emojiPack === p.key }]"
              @click.stop="switchEmojiPack(p.key)"
            >{{ p.label }}</button>
          </div>
        </div>

        <div class="flex justify-end gap-2 border-t border-border px-5 py-3">
          <VButton size="sm" @click="cancelEdit">取消</VButton>
          <VButton type="secondary" size="sm" :loading="savingEdit" @click="saveEdit">保存</VButton>
        </div>
      </div>
    </div>

    <AttachmentPickerModal :open="pickerOpen" @select="onPickerSelect" @close="pickerOpen = false" />
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--duration-fast) var(--ease-standard);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* ===== Page Header ===== */
.moments-page {
  min-height: 100%;
  background-color: var(--color-background);
}
.moments-page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 28px 14px;
  background: linear-gradient(180deg, color-mix(in srgb, #000 78%, transparent) 0%, color-mix(in srgb, #000 45%, transparent) 80%, transparent 100%);
  backdrop-filter: saturate(180%) blur(16px);
  -webkit-backdrop-filter: saturate(180%) blur(16px);
  position: sticky;
  top: 0;
  z-index: 20;
}
.moments-page-title {
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.6px;
  line-height: 1.15;
  color: var(--color-foreground);
  margin: 0;
}
.moments-content-area {
  padding: 8px 20px 48px;
}

/* 输入框聚焦不显示全局蓝色 outline（光标即焦点指示，视觉更干净） */
.moment-textarea:focus,
.moment-textarea:focus-visible {
  outline: none;
  box-shadow: none;
}

/* ===== 表情面板（内嵌式，对齐评论区 cmt-emoji-panel-v2：不遮挡内容，擑开布局） ===== */
.emoji-panel {
  display: flex;
  flex-direction: column;
  max-height: 220px;
  margin-top: 0.75rem;
  border-top: 1px solid var(--color-border);
  border-bottom: 1px solid var(--color-border);
  background: transparent;
}
.emoji-tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 8px;
  color: var(--color-text-tertiary);
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-standard), background-color var(--duration-fast) var(--ease-standard);
}
.emoji-tool-btn:hover,
.emoji-tool-btn.is-active {
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
}
.emoji-body {
  padding: 0.4rem 0.5rem;
  overflow: auto;
  flex: 1;
}
/* 网格：对齐评论区 auto-fill minmax(38px,1fr) */
.emoji-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(38px, 1fr));
  gap: 0.1rem;
}
.emoji-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 4px;
  font-size: 20px;
  cursor: pointer;
  transition: background-color 0.12s var(--ease-standard);
}
.emoji-cell:hover {
  background: color-mix(in srgb, var(--color-primary) 14%, transparent);
}
/* 表情图：对齐评论区 .cmt-emoji-img（30px contain） */
.emoji-img {
  max-width: 30px;
  max-height: 30px;
  object-fit: contain;
  display: block;
}
.emoji-loading {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.5rem 0.1rem;
  font-size: 0.75rem;
  color: var(--color-text-tertiary);
}
.emoji-error {
  padding: 0.6rem 0.1rem;
  font-size: 0.75rem;
  color: var(--color-text-tertiary);
  cursor: pointer;
}
/* 瞬间内容中插入的表情图内联 30px（与评论区一致） */
.moment-content :deep(img[src*="emoticons"]) {
  max-width: 30px;
  max-height: 30px;
  display: inline-block;
  vertical-align: middle;
  object-fit: contain;
  margin: 0 2px;
}
/* 底部 tabs：对齐评论区 .cmt-emoji-tabs */
.emoji-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.1rem;
  padding: 0.25rem 0.5rem;
  border-top: 1px solid var(--color-border);
}
.emoji-tab {
  padding: 0.3rem 0.75rem;
  font-size: 0.8125rem;
  font-weight: 400;
  color: var(--color-text-tertiary);
  background: transparent;
  border: none;
  border-radius: 3px;
  cursor: pointer;
  transition: all 0.15s var(--ease-standard);
}
.emoji-tab:hover {
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 8%, transparent);
}
.emoji-tab.is-active {
  color: var(--color-primary);
  font-weight: 600;
}

/* ===== Primary pill button ===== */
.moments-btn-primary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  border-radius: var(--radius-full, 9999px);
  background-color: var(--color-primary);
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: -0.1px;
  white-space: nowrap;
  box-shadow: var(--shadow-md);
  transition: all 0.2s var(--ease-standard);
}
.moments-btn-primary:hover {
  background-color: var(--color-primary-hover);
  transform: translateY(-1px);
  box-shadow: var(--shadow-lg);
}
.moments-btn-primary:active {
  transform: translateY(0);
}

/* ===== Timeline ===== */
.moments-container {
  position: relative;
  padding-bottom: 8px;
}
.moments-container::before {
  content: "";
  position: absolute;
  left: 15px;
  top: 8px;
  bottom: 8px;
  width: 1px;
  background: linear-gradient(
    180deg,
    transparent 0%,
    var(--color-border) 5%,
    var(--color-border) 95%,
    transparent 100%
  );
  pointer-events: none;
}

/* Date separator */
.date-separator {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 22px 0 12px;
  position: relative;
}
.date-separator:first-child {
  padding-top: 8px;
}
.date-separator-text {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-tertiary);
  letter-spacing: 0.6px;
  text-transform: uppercase;
  margin-left: 42px;
}

/* Moment item row */
.moment-item {
  display: flex;
  gap: 18px;
  margin-bottom: 16px;
  position: relative;
}
.moment-timeline-dot {
  width: 32px;
  flex-shrink: 0;
  display: flex;
  justify-content: center;
  padding-top: 22px;
}
.moment-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

/* Card */
.moment-card {
  flex: 1;
  min-width: 0;
  background-color: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  padding: 16px 18px;
  transition: all 0.25s var(--ease-standard);
  position: relative;
  overflow: hidden;
}
.moment-card:hover {
  border-color: var(--color-border-strong);
  transform: translateY(-1px);
  box-shadow: var(--shadow-xl);
}

/* Header */
.moment-header {
  display: flex;
  align-items: center;
  gap: 12px;
}
.moment-author-wrap {
  flex: 1;
  min-width: 0;
}
.moment-author {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}
.moment-time {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-top: 2px;
}
.moment-more-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  transition: all 0.18s;
  flex-shrink: 0;
}
.moment-more-btn:hover {
  background-color: color-mix(in srgb, #fff 6%, transparent);
  color: var(--color-foreground);
}
.moment-more-btn :deep(svg) {
  width: 16px;
  height: 16px;
}

/* Content */
.moment-content {
  margin-top: 12px;
  font-size: 14px;
  line-height: 1.6;
  color: #e5e5e7;
  word-break: break-word;
}
.moment-content :deep(a) {
  color: var(--color-primary);
  text-decoration: none;
}
.moment-content :deep(a:hover) {
  text-decoration: underline;
}

/* Images grid: 3 columns, 6px gap, 10px radius */
.moment-images {
  margin-top: 12px;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
}

/* Actions */
.moment-actions {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  gap: 20px;
}
.moment-action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-tertiary);
  transition: color 0.18s var(--ease-standard);
}
.moment-action:hover {
  color: var(--color-foreground);
}
.moment-action :deep(svg) {
  width: 16px;
  height: 16px;
}

/* Responsive */
@media (max-width: 640px) {
  .moments-page-header {
    padding: 16px 16px 12px;
    flex-wrap: wrap;
    gap: 10px;
  }
  .moments-content-area {
    padding: 4px 12px 36px;
  }
  .moments-page-title {
    font-size: 22px;
  }
  .apple-segmented :deep(button) {
    padding: 6px 10px;
    font-size: 12px;
  }
}
</style>
