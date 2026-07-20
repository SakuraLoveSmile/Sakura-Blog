<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { postApi } from "@/api";
import { Icon } from "@iconify/vue";

const router = useRouter();
const posts = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const statusFilter = ref("");
const loading = ref(false);

async function loadPosts() {
  loading.value = true;
  try {
    const params: Record<string, unknown> = { page: page.value, size: 20 };
    if (statusFilter.value) params.status = statusFilter.value;
    const { data } = await postApi.list(params);
    posts.value = data.items || [];
    total.value = data.total || 0;
  } catch {} finally {
    loading.value = false;
  }
}

async function handleDelete(slug: string) {
  if (!confirm(`确定删除文章「${slug}」？`)) return;
  await postApi.delete(slug);
  loadPosts();
}

onMounted(loadPosts);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">文章管理</h2>
      <router-link
        to="/posts/new"
        class="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
      >
        <Icon icon="ri:add-line" />
        新建文章
      </router-link>
    </div>

    <!-- Filter -->
    <div class="flex gap-2 mb-4">
      <button
        @click="statusFilter = ''; page = 1; loadPosts()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', !statusFilter ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']"
      >全部</button>
      <button
        @click="statusFilter = 'published'; page = 1; loadPosts()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', statusFilter === 'published' ? 'bg-green-100 text-green-700 dark:bg-green-500/15 dark:text-green-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']"
      >已发布</button>
      <button
        @click="statusFilter = 'draft'; page = 1; loadPosts()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', statusFilter === 'draft' ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']"
      >草稿</button>
    </div>

    <!-- Table -->
    <div class="bg-white rounded-xl border border-gray-200 overflow-hidden dark:bg-gray-800 dark:border-gray-700">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 border-b border-gray-200 dark:bg-gray-700/50 dark:border-gray-700">
          <tr>
            <th class="text-left px-4 py-3 font-medium text-gray-600 dark:text-gray-400">标题</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-24 dark:text-gray-400">分类</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-20 dark:text-gray-400">状态</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-36 dark:text-gray-400">日期</th>
            <th class="text-right px-4 py-3 font-medium text-gray-600 w-28 dark:text-gray-400">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50 dark:divide-gray-700">
          <tr v-for="p in posts" :key="p.slug" class="hover:bg-gray-50/50 dark:hover:bg-gray-700/30">
            <td class="px-4 py-3">
              <div class="flex items-center gap-2">
                <Icon v-if="p.pinned" icon="ri:pushpin-2-fill" class="text-red-400 text-xs" />
                <span class="font-medium text-gray-800 dark:text-gray-100">{{ p.title || p.slug }}</span>
              </div>
            </td>
            <td class="px-4 py-3 text-gray-500 dark:text-gray-400">{{ p.category || "-" }}</td>
            <td class="px-4 py-3">
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', p.status === 'published' ? 'bg-green-100 text-green-700 dark:bg-green-500/15 dark:text-green-400' : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400']">
                {{ p.status === "published" ? "已发布" : "草稿" }}
              </span>
            </td>
            <td class="px-4 py-3 text-gray-400 text-xs dark:text-gray-500">{{ p.created_at?.slice(0, 10) }}</td>
            <td class="px-4 py-3 text-right">
              <button @click="router.push(`/posts/edit/${p.slug}`)" class="text-indigo-600 hover:text-indigo-800 text-xs mr-3 dark:text-indigo-400 dark:hover:text-indigo-300">编辑</button>
              <button @click="handleDelete(p.slug)" class="text-red-500 hover:text-red-700 text-xs dark:text-red-400 dark:hover:text-red-300">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="posts.length === 0 && !loading" class="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无文章</p>
    </div>

    <!-- Pagination -->
    <div v-if="total > 20" class="flex justify-center gap-2 mt-4">
      <button
        v-for="n in Math.ceil(total / 20)"
        :key="n"
        @click="page = n; loadPosts()"
        :class="['w-8 h-8 text-sm rounded-lg', page === n ? 'bg-indigo-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']"
      >{{ n }}</button>
    </div>
  </div>
</template>
