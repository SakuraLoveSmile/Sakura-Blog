<script setup lang="ts">
import { ref, onMounted } from "vue";
import { attachmentApi } from "@/api";
import { Icon } from "@iconify/vue";

const attachments = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const uploading = ref(false);

async function loadAttachments() {
  loading.value = true;
  try {
    const { data } = await attachmentApi.list({ page: page.value, size: 24 });
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

async function handleDelete(id: number) {
  if (!confirm("确定删除该附件？")) return;
  await attachmentApi.delete(id);
  loadAttachments();
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

    <!-- Grid -->
    <div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
      <div v-for="att in attachments" :key="att.id" class="group relative bg-white rounded-xl border border-gray-200 overflow-hidden dark:bg-gray-800 dark:border-gray-700">
        <div class="aspect-square bg-gray-50 flex items-center justify-center dark:bg-gray-700/50">
          <img :src="att.url" :alt="att.filename" class="w-full h-full object-cover" />
        </div>
        <div class="p-2">
          <p class="text-xs text-gray-500 truncate dark:text-gray-400">{{ att.filename }}</p>
          <p class="text-xs text-gray-400 dark:text-gray-500">{{ (att.size / 1024).toFixed(1) }} KB</p>
        </div>
        <button
          @click="handleDelete(att.id)"
          class="absolute top-2 right-2 w-6 h-6 bg-red-500 text-white rounded-full text-xs opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center"
        >
          <Icon icon="ri:close-line" />
        </button>
      </div>
    </div>
    <p v-if="attachments.length === 0 && !loading" class="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无附件</p>

    <!-- Pagination -->
    <div v-if="total > 24" class="flex justify-center gap-2 mt-6">
      <button v-for="n in Math.ceil(total / 24)" :key="n" @click="page = n; loadAttachments()"
        :class="['w-8 h-8 text-sm rounded-lg', page === n ? 'bg-indigo-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']">{{ n }}</button>
    </div>
  </div>
</template>
