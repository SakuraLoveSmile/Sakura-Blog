<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { attachmentApi } from "@/api";
import { Icon } from "@iconify/vue";

const attachments = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const uploading = ref(false);

// 搜索与筛选
const keyword = ref("");
const kindFilter = ref(""); // "" | "image" | "other"
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    page.value = 1;
    loadAttachments();
  }, 300);
});

async function loadAttachments() {
  loading.value = true;
  try {
    const params: Record<string, unknown> = { page: page.value, size: 24 };
    if (keyword.value.trim()) params.keyword = keyword.value.trim();
    if (kindFilter.value) params.kind = kindFilter.value;
    const { data } = await attachmentApi.list(params);
    attachments.value = data.items || [];
    total.value = data.total || 0;
  } catch {} finally {
    loading.value = false;
  }
}

async function handleUpload(e: Event) {
  const input = e.target as HTMLInputElement;
  if (!input.files?.length) return;
  uploading.value = true;
  try {
    await attachmentApi.upload(input.files[0]);
    loadAttachments();
  } catch {} finally {
    uploading.value = false;
    input.value = "";
  }
}

// 详情弹窗
const detailItem = ref<any | null>(null);
const detailRefs = ref<any[]>([]);
const refsLoading = ref(false);

async function openDetail(att: any) {
  detailItem.value = att;
  detailRefs.value = [];
  refsLoading.value = true;
  try {
    const { data } = await attachmentApi.refs(att.id);
    detailRefs.value = data.refs || [];
  } catch {} finally {
    refsLoading.value = false;
  }
}
function closeDetail() {
  detailItem.value = null;
}

// 删除（带引用警告）
const deleteTarget = ref<any | null>(null);
const deleteRefs = ref<any[]>([]);
const deleteRefsLoading = ref(false);

async function requestDelete(att: any) {
  deleteTarget.value = att;
  deleteRefs.value = [];
  deleteRefsLoading.value = true;
  try {
    const { data } = await attachmentApi.refs(att.id);
    deleteRefs.value = data.refs || [];
  } catch {} finally {
    deleteRefsLoading.value = false;
  }
}

async function confirmDelete() {
  if (!deleteTarget.value) return;
  await attachmentApi.delete(deleteTarget.value.id);
  deleteTarget.value = null;
  if (detailItem.value?.id === deleteTarget.value?.id) closeDetail();
  loadAttachments();
}

function formatSize(bytes: number) {
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + " MB";
  return (bytes / 1024).toFixed(1) + " KB";
}

function isImage(mime: string) {
  return mime?.startsWith("image/");
}

onMounted(loadAttachments);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">附件管理</h2>
      <label class="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 cursor-pointer transition-colors">
        <Icon icon="ri:upload-2-line" />
        {{ uploading ? "上传中..." : "上传文件" }}
        <input type="file" accept="image/*" class="hidden" @change="handleUpload" :disabled="uploading" />
      </label>
    </div>

    <!-- 搜索与筛选 -->
    <div class="flex items-center gap-3 mb-5">
      <div class="relative flex-1 max-w-xs">
        <Icon icon="ri:search-line" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          v-model="keyword"
          placeholder="搜索文件名..."
          class="w-full pl-9 pr-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"
        />
      </div>
      <select
        v-model="kindFilter"
        @change="page = 1; loadAttachments()"
        class="px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100"
      >
        <option value="">全部类型</option>
        <option value="image">仅图片</option>
        <option value="other">非图片</option>
      </select>
      <span class="text-xs text-gray-400 dark:text-gray-500">共 {{ total }} 项</span>
    </div>

    <!-- Grid -->
    <div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
      <div
        v-for="att in attachments"
        :key="att.id"
        class="group relative bg-white rounded-xl border border-gray-200 overflow-hidden dark:bg-gray-800 dark:border-gray-700 cursor-pointer hover:shadow-md transition-shadow"
        @click="openDetail(att)"
      >
        <div class="aspect-square bg-gray-50 flex items-center justify-center dark:bg-gray-700/50 overflow-hidden">
          <img
            v-if="isImage(att.mime)"
            :src="att.thumb_url || att.url"
            :alt="att.filename"
            class="w-full h-full object-cover"
            loading="lazy"
          />
          <div v-else class="flex flex-col items-center gap-1 text-gray-300 dark:text-gray-600">
            <Icon icon="ri:file-3-line" class="text-4xl" />
            <span class="text-[10px] uppercase">{{ att.mime?.split("/")[1] || "file" }}</span>
          </div>
        </div>
        <div class="p-2">
          <p class="text-xs text-gray-600 truncate dark:text-gray-300">{{ att.filename }}</p>
          <div class="flex items-center gap-2 mt-0.5">
            <span class="text-[10px] text-gray-400 dark:text-gray-500">{{ formatSize(att.size) }}</span>
            <span v-if="att.width && att.height" class="text-[10px] text-gray-400 dark:text-gray-500">{{ att.width }}×{{ att.height }}</span>
          </div>
        </div>
        <!-- 操作按钮 -->
        <div class="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <button
            @click.stop="requestDelete(att)"
            class="w-6 h-6 bg-red-500 text-white rounded-full text-xs flex items-center justify-center hover:bg-red-600"
          >
            <Icon icon="ri:delete-bin-line" />
          </button>
        </div>
      </div>
    </div>
    <p v-if="attachments.length === 0 && !loading" class="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无附件</p>

    <!-- Pagination -->
    <div v-if="total > 24" class="flex justify-center gap-2 mt-6">
      <button v-for="n in Math.ceil(total / 24)" :key="n" @click="page = n; loadAttachments()"
        :class="['w-8 h-8 text-sm rounded-lg', page === n ? 'bg-indigo-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']">{{ n }}</button>
    </div>

    <!-- 详情弹窗 -->
    <Teleport to="body">
      <div v-if="detailItem" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40" @click.self="closeDetail">
        <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-xl w-full max-w-lg mx-4 max-h-[80vh] overflow-y-auto p-6">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-100 truncate">{{ detailItem.filename }}</h3>
            <button @click="closeDetail" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
              <Icon icon="ri:close-line" class="text-xl" />
            </button>
          </div>

          <!-- 预览 -->
          <div class="rounded-lg overflow-hidden bg-gray-50 dark:bg-gray-700/50 mb-4">
            <img v-if="isImage(detailItem.mime)" :src="detailItem.url" :alt="detailItem.filename" class="w-full max-h-64 object-contain" />
            <div v-else class="py-12 flex flex-col items-center text-gray-300 dark:text-gray-600">
              <Icon icon="ri:file-3-line" class="text-5xl" />
            </div>
          </div>

          <!-- 元信息 -->
          <div class="grid grid-cols-2 gap-3 text-sm mb-4">
            <div><span class="text-gray-400 dark:text-gray-500">格式：</span><span class="text-gray-700 dark:text-gray-300">{{ detailItem.mime }}</span></div>
            <div><span class="text-gray-400 dark:text-gray-500">大小：</span><span class="text-gray-700 dark:text-gray-300">{{ formatSize(detailItem.size) }}</span></div>
            <div v-if="detailItem.width"><span class="text-gray-400 dark:text-gray-500">尺寸：</span><span class="text-gray-700 dark:text-gray-300">{{ detailItem.width }}×{{ detailItem.height }} px</span></div>
            <div><span class="text-gray-400 dark:text-gray-500">上传：</span><span class="text-gray-700 dark:text-gray-300">{{ new Date(detailItem.created_at).toLocaleDateString() }}</span></div>
          </div>

          <!-- URL -->
          <div class="mb-4">
            <label class="text-xs text-gray-400 dark:text-gray-500 block mb-1">引用 URL</label>
            <code class="block text-xs bg-gray-50 dark:bg-gray-700/50 rounded px-3 py-2 text-gray-600 dark:text-gray-300 break-all select-all">{{ detailItem.url }}</code>
          </div>

          <!-- 引用列表 -->
          <div>
            <label class="text-xs text-gray-400 dark:text-gray-500 block mb-1">
              引用来源
              <span v-if="refsLoading" class="ml-1">加载中...</span>
              <span v-else class="ml-1">({{ detailRefs.length }})</span>
            </label>
            <div v-if="detailRefs.length === 0 && !refsLoading" class="text-xs text-gray-400 dark:text-gray-500 py-2">无引用</div>
            <div v-else class="space-y-1.5">
              <div v-for="r in detailRefs" :key="r.type + r.slug" class="flex items-center gap-2 text-sm">
                <Icon :icon="r.type === 'post' ? 'ri:article-line' : 'ri:pages-line'" class="text-gray-400" />
                <span class="text-gray-600 dark:text-gray-300">{{ r.title || r.slug }}</span>
                <span class="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 text-gray-400 dark:bg-gray-700 dark:text-gray-500">{{ r.type === "post" ? "文章" : "页面" }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 删除确认弹窗 -->
    <Teleport to="body">
      <div v-if="deleteTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40" @click.self="deleteTarget = null">
        <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-xl w-full max-w-sm mx-4 p-6">
          <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-100 mb-2">删除附件</h3>
          <p class="text-sm text-gray-500 dark:text-gray-400 mb-3">确定删除「{{ deleteTarget.filename }}」？此操作不可恢复。</p>

          <!-- 引用警告 -->
          <div v-if="deleteRefsLoading" class="text-xs text-gray-400 mb-3">检查引用中...</div>
          <div v-else-if="deleteRefs.length > 0" class="mb-3 p-3 bg-red-50 dark:bg-red-500/10 rounded-lg border border-red-200 dark:border-red-500/30">
            <p class="text-xs font-medium text-red-600 dark:text-red-400 mb-1.5">
              <Icon icon="ri:alert-line" class="inline mr-1" />该附件被 {{ deleteRefs.length }} 处引用，删除后将导致裂图：
            </p>
            <ul class="text-xs text-red-500 dark:text-red-400 space-y-0.5">
              <li v-for="r in deleteRefs" :key="r.type + r.slug">· {{ r.title || r.slug }}（{{ r.type === "post" ? "文章" : "页面" }}）</li>
            </ul>
          </div>

          <div class="flex justify-end gap-3">
            <button @click="deleteTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg dark:text-gray-400 dark:hover:bg-gray-700">取消</button>
            <button
              @click="confirmDelete"
              :class="['px-4 py-2 text-sm text-white rounded-lg font-medium', deleteRefs.length > 0 ? 'bg-red-600 hover:bg-red-700' : 'bg-red-500 hover:bg-red-600']"
            >确认删除</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
