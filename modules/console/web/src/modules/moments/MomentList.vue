<script setup lang="ts">
import { ref, computed, onMounted, nextTick, reactive, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { momentApi, settingsApi, attachmentApi, previewApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { VPageHeader, VButton, VEmpty, VLoading, VPagination, VModal, Dialog, Toast } from "@/components/ui";

interface MomentRow {
  id: number;
  content_md: string;
  content: string;
  images: string[];
  visibility: "public" | "private";
  created_at: string;
  author_name?: string;
  author_avatar?: string;
}

const queryClient = useQueryClient();
const MAX_IMAGES = 9;

const page = ref(1);
const size = ref(20);

// 发布框状态
const newContent = ref("");
const newImages = ref<string[]>([]);
const newVisibility = ref<"public" | "private">("public");
const publishing = ref(false);
const uploadError = ref("");

// 编辑弹窗状态
const editModalOpen = ref(false);
const editingId = ref<number | null>(null);
const editContent = ref("");
const editImages = ref<string[]>([]);
const editVisibility = ref<"public" | "private">("public");
const savingEdit = ref(false);

// 图床选择弹窗
const pickerOpen = ref(false);
const pickerTarget = ref<"new" | "edit">("new");

// 图片大图预览
const previewImage = ref<string | null>(null);

const queryParams = computed(() => ({ page: page.value, size: size.value }));

const {
  data: queryData,
  isLoading,
  refetch,
} = useQuery({
  queryKey: computed(() => queryKeys.moments(queryParams.value)),
  queryFn: () => momentApi.list(queryParams.value).then(({ data }) => data),
});

const moments = computed<MomentRow[]>(() => (queryData.value?.items || []) as MomentRow[]);
const total = computed(() => queryData.value?.total || 0);

// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = moments.value;
  let publicN = 0;
  let privateN = 0;
  let withImgN = 0;
  for (const m of all) {
    if (m.visibility === "private") privateN++;
    else publicN++;
    if (Array.isArray(m.images) && m.images.length > 0) withImgN++;
  }
  return {
    total: total.value,
    publicCount: publicN,
    privateCount: privateN,
    withImg: withImgN,
  };
});

// 头像渐变
function avatarClass(id: number) {
  const n = (Math.abs(Number(id) || 0) % 5) + 1;
  return `avatar-gradient-${n}`;
}

// 按日期分组：今天 / 昨天 / 本周 / 更早
type DateGroup = { key: string; label: string; items: MomentRow[] };
const groupedMoments = computed<DateGroup[]>(() => {
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const dayMs = 86400000;
  const startOfWeek = (() => {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const w = d.getDay();
    const offset = w === 0 ? 6 : w - 1;
    d.setDate(d.getDate() - offset);
    return d.getTime();
  })();

  const buckets: Record<string, MomentRow[]> = {
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

// 站长头像
const ownerAvatar = ref("");
const composerBox = ref<HTMLTextAreaElement | null>(null);

function focusComposer() {
  composerBox.value?.scrollIntoView({ block: "center", behavior: "smooth" });
  composerBox.value?.focus({ preventScroll: true });
}

onMounted(async () => {
  try {
    const { data } = await settingsApi.all();
    ownerAvatar.value = data?.site?.avatar || "";
  } catch {}
});

const charCount = computed(() => {
  let n = 0;
  for (const ch of newContent.value) {
    if (!/\s/.test(ch)) n++;
  }
  return n;
});

const canPublish = computed(() => !!newContent.value.trim() || newImages.value.length > 0);

// 快捷键 ⌘+Enter 发布
function onComposerKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
    e.preventDefault();
    handlePublish();
  }
}

// ---------- 表情面板 ----------
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

function insertEmoji(target: "new" | "edit", src: string) {
  const md = `![emoji](${src})`;
  const box = target === "new" ? newContent : editContent;
  const el = target === "new" ? composerBox.value : editBox.value;
  const start = el?.selectionStart ?? box.value.length;
  const end = el?.selectionEnd ?? start;
  box.value = box.value.slice(0, start) + md + box.value.slice(end);
  // 立即触发预览
  refreshMomentPreview(target, box.value);
  nextTick(() => {
    if (!el) return;
    el.focus();
    el.selectionStart = el.selectionEnd = start + md.length;
  });
}

// 实时预览
const newPreviewHtml = ref("");
const editPreviewHtml = ref("");

async function refreshMomentPreview(which: "new" | "edit", md: string) {
  if (!md.trim()) {
    if (which === "new") newPreviewHtml.value = "";
    else editPreviewHtml.value = "";
    return;
  }
  try {
    const { data } = await previewApi.render(md);
    if (which === "new") newPreviewHtml.value = data.html;
    else editPreviewHtml.value = data.html;
  } catch {
    // 若后端请求异常，使用本地基础转义兜底
    const fallbackHtml = md
      .replace(/!\[emoji\]\((https?:\/\/[^\)]+)\)/g, '<img src="$1" alt="emoji" class="emoji" />')
      .replace(/\n/g, "<br/>");
    if (which === "new") newPreviewHtml.value = fallbackHtml;
    else editPreviewHtml.value = fallbackHtml;
  }
}

let newPreviewTimer: ReturnType<typeof setTimeout> | undefined;
let editPreviewTimer: ReturnType<typeof setTimeout> | undefined;
watch(newContent, (v) => {
  clearTimeout(newPreviewTimer);
  newPreviewTimer = setTimeout(() => refreshMomentPreview("new", v), 200);
});
watch(editContent, (v) => {
  clearTimeout(editPreviewTimer);
  editPreviewTimer = setTimeout(() => refreshMomentPreview("edit", v), 200);
});

// ---------- 图片管理 ----------
function targetImages(target: "new" | "edit") {
  return target === "new" ? newImages : editImages;
}

function addImage(target: "new" | "edit", url: string) {
  const box = targetImages(target);
  if (!url || box.value.includes(url)) return;
  if (box.value.length >= MAX_IMAGES) {
    Toast.warning(`单条最多 ${MAX_IMAGES} 张图片`);
    return;
  }
  box.value = [...box.value, url];
}

function removeImage(target: "new" | "edit", idx: number) {
  const box = targetImages(target);
  box.value = box.value.filter((_, i) => i !== idx);
}

function openPicker(target: "new" | "edit") {
  pickerTarget.value = target;
  pickerOpen.value = true;
}

function onPickerSelect(url: string) {
  addImage(pickerTarget.value, url);
  pickerOpen.value = false;
}

async function uploadFile(target: "new" | "edit", file: File) {
  uploadError.value = "";
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

function onPaste(target: "new" | "edit", e: ClipboardEvent) {
  const files = Array.from(e.clipboardData?.files || []).filter((f) => f.type.startsWith("image/"));
  if (files.length === 0) return;
  e.preventDefault();
  files.forEach((f) => uploadFile(target, f));
}

const newFileInput = ref<HTMLInputElement | null>(null);
const editFileInput = ref<HTMLInputElement | null>(null);

function onPickFiles(target: "new" | "edit", e: Event) {
  const input = e.target as HTMLInputElement;
  for (const file of Array.from(input.files || [])) {
    if (file.type.startsWith("image/")) uploadFile(target, file);
  }
  input.value = "";
}

// ---------- 发布与编辑 ----------
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
    newPreviewHtml.value = "";
    await queryClient.invalidateQueries({ queryKey: queryKeys.moments() });
    await refetch();
    Toast.success("瞬间发布成功");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "发布失败");
  } finally {
    publishing.value = false;
  }
}

function startEdit(m: MomentRow) {
  editingId.value = m.id;
  editContent.value = m.content_md || "";
  editImages.value = Array.isArray(m.images) ? [...m.images] : [];
  editVisibility.value = m.visibility === "private" ? "private" : "public";
  refreshMomentPreview("edit", editContent.value);
  editModalOpen.value = true;
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
    await queryClient.invalidateQueries({ queryKey: queryKeys.moments() });
    await refetch();
    Toast.success("瞬间已更新");
    editModalOpen.value = false;
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "保存失败");
  } finally {
    savingEdit.value = false;
  }
}

async function toggleVisibility(m: MomentRow) {
  const next = m.visibility === "private" ? "public" : "private";
  // 乐观更新
  m.visibility = next;
  try {
    await momentApi.update(m.id, {
      content_md: m.content_md,
      images: Array.isArray(m.images) ? m.images : [],
      visibility: next,
    });
    await queryClient.invalidateQueries({ queryKey: queryKeys.moments() });
    await refetch();
    Toast.success(next === "private" ? "已设为仅自己可见" : "已设为公开展示");
  } catch (e: any) {
    // 失败回退
    m.visibility = next === "private" ? "public" : "private";
    Toast.error(e?.response?.data?.error || "更新可见性失败");
  }
}

function handleDelete(m: MomentRow) {
  Dialog.warning({
    title: "确定删除该瞬间吗？",
    description: "删除后将无法恢复，包含的图文信息将从前台移除。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await momentApi.delete(m.id);
        await queryClient.invalidateQueries({ queryKey: queryKeys.moments() });
        await refetch();
        Toast.success("已删除");
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      }
    },
  });
}

function formatTime(s: string): string {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="瞬间">
      <template #icon>
        <Icon icon="ri:chat-smile-2-line" class="text-primary" />
      </template>
      <template #subtitle>记录生活碎片、灵感微动态与短想法，支持九宫格图文与 Markdown 表情</template>
      <template #actions>
        <VButton type="primary" @click="focusComposer">
          <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
          发布新动态
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[800px] space-y-5 px-4 py-5 sm:px-6">
      <VLoading v-if="isLoading" />

      <template v-else>
        <!-- 概览数据指标卡 -->
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
          <div class="stats-card">
            <div class="stats-icon text-primary bg-primary/10">
              <Icon icon="ri:chat-smile-2-line" />
            </div>
            <div>
              <div class="stats-label">全部瞬间</div>
              <div class="stats-value">{{ stats.total }} <span class="stats-sub">条动态记录</span></div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
              <Icon icon="ri:global-line" />
            </div>
            <div>
              <div class="stats-label">公开展示</div>
              <div class="stats-value text-[#30D158]">
                {{ stats.publicCount }} <span class="stats-sub">条公开动态</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#FF9500] bg-[#FF9500]/10">
              <Icon icon="ri:lock-line" />
            </div>
            <div>
              <div class="stats-label">私密归档</div>
              <div class="stats-value text-[#FF9500]">
                {{ stats.privateCount }} <span class="stats-sub">条仅己可见</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
              <Icon icon="ri:image-2-line" />
            </div>
            <div>
              <div class="stats-label">图文动态</div>
              <div class="stats-value">{{ stats.withImg }} <span class="stats-sub">条包含配图</span></div>
            </div>
          </div>
        </div>

        <!-- ============ 顶部发布框 (Composer) ============ -->
        <div class="rounded-2xl border border-border bg-card p-4 shadow-sm sm:p-5">
          <div class="flex gap-3.5">
            <img
              v-if="ownerAvatar"
              :src="ownerAvatar"
              alt=""
              class="h-10 w-10 shrink-0 rounded-full border border-border object-cover"
            />
            <div
              v-else
              class="avatar-gradient-1 flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-base font-bold text-white"
            >
              ✍️
            </div>

            <div class="min-w-0 flex-1">
              <textarea
                ref="composerBox"
                v-model="newContent"
                rows="3"
                placeholder="分享此刻的想法、灵感或工作碎碎念…（支持粘贴图片或拖拽）"
                class="w-full resize-none border-0 bg-transparent text-sm leading-relaxed text-foreground placeholder:text-text-quaternary focus:outline-none"
                @keydown="onComposerKeydown"
                @paste="onPaste('new', $event)"
              ></textarea>

              <!-- Markdown 实时渲染预览 -->
              <div
                v-if="newPreviewHtml"
                class="moment-preview mt-2 rounded-xl border border-border/80 bg-secondary/40 p-3 text-xs leading-relaxed text-foreground"
                v-html="newPreviewHtml"
              ></div>

              <!-- 9 宫格图片预览 -->
              <div v-if="newImages.length > 0" class="mt-3 grid grid-cols-3 gap-2 sm:grid-cols-4">
                <div
                  v-for="(url, idx) in newImages"
                  :key="url"
                  class="group relative aspect-square overflow-hidden rounded-xl border border-border bg-secondary"
                >
                  <img :src="url" alt="" class="h-full w-full object-cover" />
                  <button
                    type="button"
                    class="absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-black/70 text-white opacity-0 transition-opacity group-hover:opacity-100"
                    title="移除此图"
                    @click.stop="removeImage('new', idx)"
                  >
                    <Icon icon="ri:close-line" class="text-sm" />
                  </button>
                </div>
              </div>

              <p v-if="uploadError" class="mt-2 text-xs text-state-error">{{ uploadError }}</p>

              <!-- 发布工具栏 -->
              <div class="mt-3.5 flex flex-wrap items-center justify-between gap-2 border-t border-border/60 pt-3">
                <div class="flex items-center gap-1.5">
                  <!-- 选图/图床 -->
                  <button
                    type="button"
                    class="action-tool-btn"
                    title="从媒体库选择图片"
                    @click.stop="openPicker('new')"
                  >
                    <Icon icon="ri:image-line" />
                  </button>

                  <!-- 本地上传 -->
                  <input
                    ref="newFileInput"
                    type="file"
                    accept="image/*"
                    multiple
                    class="hidden"
                    @change="onPickFiles('new', $event)"
                  />
                  <button type="button" class="action-tool-btn" title="上传本地图片" @click="newFileInput?.click()">
                    <Icon icon="ri:upload-2-line" />
                  </button>

                  <!-- 表情选择 -->
                  <div class="relative">
                    <button
                      type="button"
                      class="action-tool-btn"
                      :class="{ active: emojiOpen === 'new' }"
                      title="插入表情"
                      @click.stop="toggleEmoji('new')"
                    >
                      <Icon icon="ri:emotion-line" />
                    </button>

                    <!-- 表情弹层 -->
                    <div
                      v-if="emojiOpen === 'new'"
                      class="absolute left-0 top-full z-30 mt-2 w-72 rounded-2xl border border-border bg-card p-3 shadow-xl"
                      @click.stop
                    >
                      <div class="flex items-center gap-1 border-b border-border pb-2 text-xs">
                        <button
                          v-for="pack in EMOJI_PACKS"
                          :key="pack.key"
                          type="button"
                          class="rounded-md px-2 py-1 text-text-tertiary transition-colors hover:bg-secondary hover:text-foreground"
                          :class="{ 'bg-secondary font-bold text-foreground': emojiPack === pack.key }"
                          @click="switchEmojiPack(pack.key)"
                        >
                          {{ pack.label }}
                        </button>
                      </div>

                      <div class="max-h-48 overflow-y-auto pt-2">
                        <div v-if="emojiLoading" class="py-6 text-center text-xs text-text-quaternary">
                          加载表情包中...
                        </div>
                        <div v-else class="grid grid-cols-6 gap-1.5">
                          <button
                            v-for="item in currentEmojis"
                            :key="item.key"
                            type="button"
                            class="flex aspect-square items-center justify-center rounded-lg p-1 transition-transform hover:scale-125 hover:bg-secondary"
                            :title="item.key"
                            @click="insertEmoji('new', item.val)"
                          >
                            <img :src="item.val" :alt="item.key" class="max-h-full max-w-full object-contain" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- 可见性切换 -->
                  <div class="apple-segmented ml-1 !p-0.5">
                    <button
                      type="button"
                      class="!px-2.5 !py-1 !text-xs"
                      :class="{ active: newVisibility === 'public' }"
                      @click="newVisibility = 'public'"
                    >
                      <Icon icon="ri:global-line" class="mr-1 inline-block" /> 公开
                    </button>
                    <button
                      type="button"
                      class="!px-2.5 !py-1 !text-xs"
                      :class="{ active: newVisibility === 'private' }"
                      @click="newVisibility = 'private'"
                    >
                      <Icon icon="ri:lock-line" class="mr-1 inline-block" /> 私密
                    </button>
                  </div>
                </div>

                <!-- 字符计数与发布按钮 -->
                <div class="flex items-center gap-3">
                  <span v-if="charCount > 0" class="text-xs text-text-quaternary"> {{ charCount }} 字 </span>
                  <VButton
                    type="primary"
                    size="sm"
                    :loading="publishing"
                    :disabled="!canPublish"
                    @click="handlePublish"
                  >
                    <template #icon><Icon icon="ri:send-plane-fill" /></template>
                    发布瞬间
                  </VButton>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- ============ 瞬间列表 / 时间轴 ============ -->
        <VEmpty v-if="moments.length === 0" title="暂无任何瞬间" message="在上方输入框写下你的第一条微动态吧！" />

        <div v-else class="space-y-6">
          <div v-for="g in groupedMoments" :key="g.key" class="space-y-3">
            <!-- 分组标题 -->
            <div class="flex items-center gap-2">
              <span class="text-xs font-bold uppercase tracking-wider text-text-tertiary">
                {{ g.label }}
              </span>
              <div class="h-px flex-1 bg-border/60"></div>
            </div>

            <!-- 卡片列表 -->
            <div class="space-y-3">
              <div
                v-for="m in g.items"
                :key="m.id"
                class="rounded-2xl border border-border bg-card p-4 transition-all hover:border-border-strong sm:p-5"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="flex items-center gap-2.5">
                    <img
                      v-if="m.author_avatar || ownerAvatar"
                      :src="m.author_avatar || ownerAvatar"
                      alt=""
                      class="h-9 w-9 rounded-full border border-border object-cover"
                    />
                    <div
                      v-else
                      :class="[
                        avatarClass(m.id),
                        'flex h-9 w-9 items-center justify-center rounded-full text-xs font-bold text-white',
                      ]"
                    >
                      {{ m.author_name ? m.author_name.charAt(0) : "A" }}
                    </div>

                    <div>
                      <div class="flex items-center gap-2">
                        <span class="text-sm font-semibold text-foreground">
                          {{ m.author_name || "站长" }}
                        </span>
                        <span
                          class="rounded-full px-2 py-0.5 text-[10px] font-medium"
                          :class="
                            m.visibility === 'private'
                              ? 'bg-[#FF9500]/10 text-[#FF9500]'
                              : 'bg-[#30D158]/10 text-[#30D158]'
                          "
                        >
                          {{ m.visibility === "private" ? "私密" : "公开" }}
                        </span>
                      </div>
                      <div class="text-[11px] text-text-quaternary">
                        {{ formatTime(m.created_at) }}
                      </div>
                    </div>
                  </div>

                  <!-- 动作小按钮 -->
                  <div class="flex items-center gap-1">
                    <button
                      type="button"
                      class="action-icon-btn"
                      :title="m.visibility === 'private' ? '切换为公开' : '切换为私密'"
                      @click="toggleVisibility(m)"
                    >
                      <Icon :icon="m.visibility === 'private' ? 'ri:lock-line' : 'ri:global-line'" />
                    </button>
                    <button type="button" class="action-icon-btn" title="编辑" @click="startEdit(m)">
                      <Icon icon="ri:pencil-line" />
                    </button>
                    <button
                      type="button"
                      class="action-icon-btn hover:text-[var(--color-state-error)]"
                      title="删除"
                      @click="handleDelete(m)"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </div>

                <!-- 动态正文（富文本 HTML 优先，保障表情与格式正确呈现） -->
                <div
                  v-if="m.content"
                  class="moment-rich-content mt-3 text-sm leading-relaxed text-foreground"
                  v-html="m.content"
                ></div>
                <p v-else-if="m.content_md" class="mt-3 text-sm leading-relaxed text-foreground whitespace-pre-wrap">
                  {{ m.content_md }}
                </p>

                <!-- 配图九宫格 -->
                <div
                  v-if="Array.isArray(m.images) && m.images.length > 0"
                  class="mt-3.5 grid gap-2"
                  :class="[
                    m.images.length === 1
                      ? 'grid-cols-1 max-w-[320px]'
                      : m.images.length === 2 || m.images.length === 4
                        ? 'grid-cols-2 max-w-[440px]'
                        : 'grid-cols-3 max-w-[540px]',
                  ]"
                >
                  <div
                    v-for="img in m.images"
                    :key="img"
                    class="group relative aspect-square overflow-hidden rounded-xl border border-border bg-secondary cursor-pointer"
                    @click="previewImage = img"
                  >
                    <img
                      :src="img"
                      alt=""
                      class="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 分页条 -->
          <div v-if="total > size" class="flex justify-center pt-4">
            <VPagination
              :page="page"
              :size="size"
              :total="total"
              @update:page="page = $event"
              @update:size="size = $event"
            />
          </div>
        </div>
      </template>
    </div>

    <!-- ============ 编辑弹窗 ============ -->
    <VModal v-model:visible="editModalOpen" title="编辑瞬间" :width="560">
      <div class="space-y-4">
        <textarea
          ref="editBox"
          v-model="editContent"
          rows="4"
          class="apple-textarea"
          placeholder="瞬间内容..."
          @paste="onPaste('edit', $event)"
        ></textarea>

        <!-- 编辑预览 -->
        <div
          v-if="editPreviewHtml"
          class="moment-preview rounded-xl border border-border bg-secondary/40 p-3 text-xs leading-relaxed text-foreground"
          v-html="editPreviewHtml"
        ></div>

        <!-- 图片列表 -->
        <div v-if="editImages.length > 0" class="grid grid-cols-4 gap-2">
          <div
            v-for="(url, idx) in editImages"
            :key="url"
            class="group relative aspect-square overflow-hidden rounded-xl border border-border bg-secondary"
          >
            <img :src="url" alt="" class="h-full w-full object-cover" />
            <button
              type="button"
              class="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/70 text-white opacity-0 transition-opacity group-hover:opacity-100"
              @click="removeImage('edit', idx)"
            >
              <Icon icon="ri:close-line" class="text-xs" />
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between border-t border-border/60 pt-3">
          <div class="flex items-center gap-2">
            <button type="button" class="action-tool-btn" title="从媒体库选择图片" @click.stop="openPicker('edit')">
              <Icon icon="ri:image-line" />
            </button>
            <input
              ref="editFileInput"
              type="file"
              accept="image/*"
              multiple
              class="hidden"
              @change="onPickFiles('edit', $event)"
            />
            <button type="button" class="action-tool-btn" title="上传本地图片" @click="editFileInput?.click()">
              <Icon icon="ri:upload-2-line" />
            </button>

            <!-- 编辑表情选择 -->
            <div class="relative">
              <button
                type="button"
                class="action-tool-btn"
                :class="{ active: emojiOpen === 'edit' }"
                title="插入表情"
                @click.stop="toggleEmoji('edit')"
              >
                <Icon icon="ri:emotion-line" />
              </button>

              <div
                v-if="emojiOpen === 'edit'"
                class="absolute left-0 bottom-full z-30 mb-2 w-72 rounded-2xl border border-border bg-card p-3 shadow-xl"
                @click.stop
              >
                <div class="flex items-center gap-1 border-b border-border pb-2 text-xs">
                  <button
                    v-for="pack in EMOJI_PACKS"
                    :key="pack.key"
                    type="button"
                    class="rounded-md px-2 py-1 text-text-tertiary transition-colors hover:bg-secondary hover:text-foreground"
                    :class="{ 'bg-secondary font-bold text-foreground': emojiPack === pack.key }"
                    @click="switchEmojiPack(pack.key)"
                  >
                    {{ pack.label }}
                  </button>
                </div>

                <div class="max-h-44 overflow-y-auto pt-2">
                  <div v-if="emojiLoading" class="py-6 text-center text-xs text-text-quaternary">加载表情包中...</div>
                  <div v-else class="grid grid-cols-6 gap-1.5">
                    <button
                      v-for="item in currentEmojis"
                      :key="item.key"
                      type="button"
                      class="flex aspect-square items-center justify-center rounded-lg p-1 transition-transform hover:scale-125 hover:bg-secondary"
                      :title="item.key"
                      @click="insertEmoji('edit', item.val)"
                    >
                      <img :src="item.val" :alt="item.key" class="max-h-full max-w-full object-contain" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="apple-segmented !p-0.5">
            <button
              type="button"
              class="!px-2.5 !py-1 !text-xs"
              :class="{ active: editVisibility === 'public' }"
              @click="editVisibility = 'public'"
            >
              公开
            </button>
            <button
              type="button"
              class="!px-2.5 !py-1 !text-xs"
              :class="{ active: editVisibility === 'private' }"
              @click="editVisibility = 'private'"
            >
              私密
            </button>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="editModalOpen = false">取消</VButton>
          <VButton type="primary" :loading="savingEdit" @click="saveEdit">保存修改</VButton>
        </div>
      </template>
    </VModal>

    <!-- 图床选择模态框 -->
    <AttachmentPickerModal :open="pickerOpen" @select="onPickerSelect" @close="pickerOpen = false" />

    <!-- 大图放大查看模态框 -->
    <VModal :visible="!!previewImage" title="图片预览" :width="680" @update:visible="previewImage = null">
      <div class="flex items-center justify-center p-2">
        <img :src="previewImage || ''" class="max-h-[70vh] rounded-xl object-contain shadow-lg" alt="" />
      </div>
      <template #footer>
        <div class="flex justify-end">
          <VButton @click="previewImage = null">关闭</VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>
/* 统计卡片 */
.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-xs);
}
.stats-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 8%, transparent), transparent);
  pointer-events: none;
}
.stats-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  font-size: 20px;
  flex-shrink: 0;
}
.stats-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-quaternary);
  text-transform: uppercase;
  letter-spacing: 0.2px;
}
.stats-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.2;
}
.stats-sub {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-tertiary);
  margin-top: 1px;
}

/* 工具栏小按钮 */
.action-tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  background: transparent;
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-tool-btn:hover,
.action-tool-btn.active {
  background: var(--color-secondary);
  color: var(--color-foreground);
}

/* 操作小按钮 */
.action-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  background: transparent;
  border: 1px solid transparent;
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-icon-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

/* 富文本与表情渲染样式 */
:deep(.moment-rich-content),
:deep(.moment-preview) {
  word-break: break-word;
}
:deep(.moment-rich-content p),
:deep(.moment-preview p) {
  margin: 0.25rem 0;
}
:deep(.moment-rich-content p:first-child),
:deep(.moment-preview p:first-child) {
  margin-top: 0;
}
:deep(.moment-rich-content p:last-child),
:deep(.moment-preview p:last-child) {
  margin-bottom: 0;
}
:deep(.moment-rich-content img[alt="emoji"]),
:deep(.moment-preview img[alt="emoji"]),
:deep(.moment-rich-content img.emoji),
:deep(.moment-preview img.emoji),
:deep(.moment-rich-content img[src*="artalk"]),
:deep(.moment-preview img[src*="artalk"]),
:deep(.moment-rich-content img[src*="emoticons"]),
:deep(.moment-preview img[src*="emoticons"]) {
  display: inline-block !important;
  height: 1.5em !important;
  width: 1.5em !important;
  max-width: 1.5em !important;
  max-height: 1.5em !important;
  vertical-align: -0.22em !important;
  margin: 0 2px !important;
  object-fit: contain !important;
}
:deep(.moment-rich-content img:not([alt="emoji"]):not([src*="artalk"]):not([src*="emoticons"])),
:deep(.moment-preview img:not([alt="emoji"]):not([src*="artalk"]):not([src*="emoticons"])) {
  max-width: 100%;
  border-radius: var(--radius-md);
  margin: 6px 0;
  display: block;
}
:deep(.moment-rich-content a),
:deep(.moment-preview a) {
  color: var(--color-primary);
  text-decoration: underline;
  text-underline-offset: 3px;
}
:deep(.moment-rich-content code),
:deep(.moment-preview code) {
  background: var(--color-secondary);
  padding: 2px 5px;
  border-radius: 4px;
  font-size: 0.85em;
  font-family: monospace;
}

/* 移动端适配 */
@media (max-width: 640px) {
  .stats-card {
    padding: 10px 12px;
    gap: 8px;
  }
  .stats-icon {
    width: 32px;
    height: 32px;
    font-size: 16px;
  }
  .stats-value {
    font-size: 14px;
  }
}
</style>
