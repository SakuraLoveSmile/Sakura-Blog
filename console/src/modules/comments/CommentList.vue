<script setup lang="ts">
import { ref, onMounted } from "vue";
import { commentApi } from "@/api";

const comments = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const statusFilter = ref("");
const loading = ref(false);
const replyTarget = ref<any>(null);
const replyContent = ref("");

async function loadComments() {
  loading.value = true;
  try {
    const params: Record<string, unknown> = { page: page.value, size: 20 };
    if (statusFilter.value) params.status = statusFilter.value;
    const { data } = await commentApi.list(params);
    comments.value = data.items || [];
    total.value = data.total || 0;
  } catch {} finally {
    loading.value = false;
  }
}

async function handleApprove(id: number) {
  await commentApi.approve(id);
  loadComments();
}

async function handleDelete(id: number) {
  if (!confirm("确定删除该评论？")) return;
  await commentApi.delete(id);
  loadComments();
}

async function handleReply() {
  if (!replyTarget.value || !replyContent.value.trim()) return;
  await commentApi.reply({ parent_id: replyTarget.value.id, content: replyContent.value });
  replyTarget.value = null;
  replyContent.value = "";
  loadComments();
}

onMounted(loadComments);
</script>

<template>
  <div class="p-6">
    <h2 class="text-xl font-semibold text-gray-800 mb-6 dark:text-gray-100">评论管理</h2>

    <!-- Filter tabs -->
    <div class="flex gap-2 mb-4">
      <button @click="statusFilter = ''; page = 1; loadComments()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', !statusFilter ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']">全部</button>
      <button @click="statusFilter = 'pending'; page = 1; loadComments()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', statusFilter === 'pending' ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']">待审核</button>
      <button @click="statusFilter = 'approved'; page = 1; loadComments()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', statusFilter === 'approved' ? 'bg-green-100 text-green-700 dark:bg-green-500/15 dark:text-green-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']">已通过</button>
    </div>

    <!-- Comment list -->
    <div class="space-y-3">
      <div v-for="cm in comments" :key="cm.id" class="bg-white rounded-xl border border-gray-200 p-4 dark:bg-gray-800 dark:border-gray-700">
        <div class="flex items-start gap-3">
          <img :src="cm.avatar" class="w-9 h-9 rounded-full mt-0.5" />
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="font-medium text-sm text-gray-800 dark:text-gray-100">{{ cm.nick }}</span>
              <span v-if="cm.is_admin" class="px-1.5 py-0.5 bg-indigo-100 text-indigo-600 text-xs rounded dark:bg-indigo-500/15 dark:text-indigo-400">博主</span>
              <span :class="['px-1.5 py-0.5 text-xs rounded', cm.status === 'approved' ? 'bg-green-100 text-green-600 dark:bg-green-500/15 dark:text-green-400' : 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400']">
                {{ cm.status === "approved" ? "已通过" : "待审核" }}
              </span>
              <span class="text-xs text-gray-400 dark:text-gray-500">{{ cm.date }}</span>
              <span class="text-xs text-gray-400 dark:text-gray-500">{{ cm.page_key }}</span>
            </div>
            <p v-if="cm.reply_nick" class="text-xs text-gray-400 mt-0.5 dark:text-gray-500">回复 @{{ cm.reply_nick }}</p>
            <p class="text-sm text-gray-600 mt-1 dark:text-gray-400">{{ cm.content }}</p>
            <div class="flex gap-3 mt-2">
              <button v-if="cm.status === 'pending'" @click="handleApprove(cm.id)" class="text-xs text-green-600 hover:text-green-800 dark:text-green-400 dark:hover:text-green-300">通过</button>
              <button @click="replyTarget = cm" class="text-xs text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300">回复</button>
              <button @click="handleDelete(cm.id)" class="text-xs text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300">删除</button>
            </div>
          </div>
        </div>
      </div>
      <p v-if="comments.length === 0 && !loading" class="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无评论</p>
    </div>

    <!-- Pagination -->
    <div v-if="total > 20" class="flex justify-center gap-2 mt-4">
      <button v-for="n in Math.ceil(total / 20)" :key="n" @click="page = n; loadComments()"
        :class="['w-8 h-8 text-sm rounded-lg', page === n ? 'bg-indigo-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']">{{ n }}</button>
    </div>

    <!-- Reply modal -->
    <div v-if="replyTarget" class="fixed inset-0 bg-black/30 flex items-center justify-center z-50" @click.self="replyTarget = null">
      <div class="bg-white rounded-xl p-6 w-full max-w-md dark:bg-gray-800">
        <h3 class="text-sm font-medium text-gray-700 mb-3 dark:text-gray-300">回复 @{{ replyTarget.nick }}</h3>
        <textarea v-model="replyContent" rows="4" placeholder="回复内容..."
          class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg resize-y focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"></textarea>
        <div class="flex justify-end gap-2 mt-4">
          <button @click="replyTarget = null" class="px-4 py-2 text-sm text-gray-600 dark:text-gray-400">取消</button>
          <button @click="handleReply" class="px-4 py-2 text-sm bg-indigo-600 text-white rounded-lg hover:bg-indigo-700">发送</button>
        </div>
      </div>
    </div>
  </div>
</template>
