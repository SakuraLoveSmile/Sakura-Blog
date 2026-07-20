<script setup lang="ts">
import { ref, onMounted } from "vue";
import { momentApi } from "@/api";
import { Icon } from "@iconify/vue";

const moments = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const newContent = ref("");
const publishing = ref(false);

async function loadMoments() {
  loading.value = true;
  try {
    const { data } = await momentApi.list({ page: page.value, size: 20 });
    moments.value = data.items || [];
    total.value = data.total || 0;
  } catch {} finally {
    loading.value = false;
  }
}

async function handlePublish() {
  if (!newContent.value.trim()) return;
  publishing.value = true;
  try {
    await momentApi.create({ content_md: newContent.value });
    newContent.value = "";
    loadMoments();
  } catch {} finally {
    publishing.value = false;
  }
}

async function handleDelete(id: number) {
  if (!confirm("确定删除该瞬间？")) return;
  await momentApi.delete(id);
  loadMoments();
}

onMounted(loadMoments);
</script>

<template>
  <div class="p-6">
    <h2 class="text-xl font-semibold text-gray-800 mb-6">瞬间</h2>

    <!-- Publish box -->
    <div class="bg-white rounded-xl border border-gray-200 p-4 mb-6">
      <textarea v-model="newContent" rows="3" placeholder="记录此刻想法...（支持 Markdown）"
        class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg resize-y focus:outline-none focus:ring-2 focus:ring-indigo-500"></textarea>
      <div class="flex justify-end mt-3">
        <button @click="handlePublish" :disabled="publishing || !newContent.trim()"
          class="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">
          <Icon icon="ri:send-plane-fill" />
          {{ publishing ? "发布中..." : "发布" }}
        </button>
      </div>
    </div>

    <!-- Moment list -->
    <div class="space-y-3">
      <div v-for="m in moments" :key="m.id" class="bg-white rounded-xl border border-gray-200 p-4">
        <div class="flex items-start justify-between">
          <div class="flex-1">
            <p class="text-xs text-gray-400 mb-1">{{ m.created_at?.slice(0, 16).replace("T", " ") }}</p>
            <div class="text-sm text-gray-700 prose prose-sm" v-html="m.content"></div>
          </div>
          <button @click="handleDelete(m.id)" class="text-red-400 hover:text-red-600 ml-4">
            <Icon icon="ri:delete-bin-line" />
          </button>
        </div>
      </div>
      <p v-if="moments.length === 0 && !loading" class="py-12 text-center text-sm text-gray-400">暂无瞬间</p>
    </div>

    <!-- Pagination -->
    <div v-if="total > 20" class="flex justify-center gap-2 mt-4">
      <button v-for="n in Math.ceil(total / 20)" :key="n" @click="page = n; loadMoments()"
        :class="['w-8 h-8 text-sm rounded-lg', page === n ? 'bg-indigo-600 text-white' : 'text-gray-600 hover:bg-gray-100']">{{ n }}</button>
    </div>
  </div>
</template>
