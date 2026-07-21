<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { pageApi } from "@/api";
import { Icon } from "@iconify/vue";

const router = useRouter();
const pages = ref<any[]>([]);
const loading = ref(false);
const statusFilter = ref("");

async function loadPages() {
  loading.value = true;
  try {
    const params: Record<string, unknown> = {};
    if (statusFilter.value) params.status = statusFilter.value;
    const { data } = await pageApi.list(params);
    pages.value = data.items || [];
  } catch {} finally {
    loading.value = false;
  }
}

async function handleDelete(slug: string) {
  if (!confirm(`确定将页面「${slug}」移入回收站？`)) return;
  await pageApi.delete(slug);
  loadPages();
}

async function handleRestore(slug: string) {
  await pageApi.restore(slug);
  loadPages();
}

async function handlePurge(slug: string) {
  if (!confirm(`彻底删除页面「${slug}」？此操作不可恢复！`)) return;
  await pageApi.purge(slug);
  loadPages();
}

onMounted(loadPages);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">页面管理</h2>
      <router-link to="/pages/edit/new"
        class="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors">
        <Icon icon="ri:add-line" />
        新建页面
      </router-link>
    </div>

    <!-- Filter -->
    <div class="flex gap-2 mb-4">
      <button
        @click="statusFilter = ''; loadPages()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', !statusFilter ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']"
      >全部</button>
      <button
        @click="statusFilter = 'trashed'; loadPages()"
        :class="['px-3 py-1.5 text-sm rounded-lg transition-colors', statusFilter === 'trashed' ? 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-400' : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700']"
      >回收站</button>
    </div>

    <div class="bg-white rounded-xl border border-gray-200 overflow-hidden dark:bg-gray-800 dark:border-gray-700">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 border-b border-gray-200 dark:bg-gray-700/50 dark:border-gray-700">
          <tr>
            <th class="text-left px-4 py-3 font-medium text-gray-600 dark:text-gray-400">标题</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-24 dark:text-gray-400">Slug</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-20 dark:text-gray-400">状态</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-36 dark:text-gray-400">更新时间</th>
            <th class="text-right px-4 py-3 font-medium text-gray-600 w-28 dark:text-gray-400">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50 dark:divide-gray-700">
          <tr v-for="p in pages" :key="p.slug" class="hover:bg-gray-50/50 dark:hover:bg-gray-700/30">
            <td class="px-4 py-3 font-medium text-gray-800 dark:text-gray-100">{{ p.title || p.slug }}</td>
            <td class="px-4 py-3 text-gray-500 dark:text-gray-400">{{ p.slug }}</td>
            <td class="px-4 py-3">
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', p.status === 'published' ? 'bg-green-100 text-green-700 dark:bg-green-500/15 dark:text-green-400' : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400']">
                {{ p.status === "published" ? "已发布" : "草稿" }}
              </span>
            </td>
            <td class="px-4 py-3 text-gray-400 text-xs dark:text-gray-500">{{ p.updated_at?.slice(0, 10) }}</td>
            <td class="px-4 py-3 text-right">
              <template v-if="statusFilter === 'trashed'">
                <button @click="handleRestore(p.slug)" class="text-green-600 hover:text-green-800 text-xs mr-3 dark:text-green-400 dark:hover:text-green-300">恢复</button>
                <button @click="handlePurge(p.slug)" class="text-red-500 hover:text-red-700 text-xs dark:text-red-400 dark:hover:text-red-300">彻底删除</button>
              </template>
              <template v-else>
                <button @click="router.push(`/pages/edit/${p.slug}`)" class="text-indigo-600 hover:text-indigo-800 text-xs mr-3 dark:text-indigo-400 dark:hover:text-indigo-300">编辑</button>
                <button @click="handleDelete(p.slug)" class="text-red-500 hover:text-red-700 text-xs dark:text-red-400 dark:hover:text-red-300">删除</button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="pages.length === 0 && !loading" class="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无页面</p>
    </div>
  </div>
</template>
