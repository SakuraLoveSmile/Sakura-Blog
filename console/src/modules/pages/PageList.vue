<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { pageApi } from "@/api";
import { Icon } from "@iconify/vue";

const router = useRouter();
const pages = ref<any[]>([]);
const loading = ref(false);

async function loadPages() {
  loading.value = true;
  try {
    const { data } = await pageApi.list();
    pages.value = data.items || [];
  } catch {} finally {
    loading.value = false;
  }
}

async function handleDelete(slug: string) {
  if (!confirm(`确定删除页面「${slug}」？`)) return;
  await pageApi.delete(slug);
  loadPages();
}

onMounted(loadPages);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800">页面管理</h2>
      <router-link to="/pages/edit/new"
        class="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors">
        <Icon icon="ri:add-line" />
        新建页面
      </router-link>
    </div>

    <div class="bg-white rounded-xl border border-gray-200 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 border-b border-gray-200">
          <tr>
            <th class="text-left px-4 py-3 font-medium text-gray-600">标题</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-24">Slug</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-20">状态</th>
            <th class="text-left px-4 py-3 font-medium text-gray-600 w-36">更新时间</th>
            <th class="text-right px-4 py-3 font-medium text-gray-600 w-28">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-50">
          <tr v-for="p in pages" :key="p.slug" class="hover:bg-gray-50/50">
            <td class="px-4 py-3 font-medium text-gray-800">{{ p.title || p.slug }}</td>
            <td class="px-4 py-3 text-gray-500">{{ p.slug }}</td>
            <td class="px-4 py-3">
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', p.status === 'published' ? 'bg-green-100 text-green-700' : 'bg-amber-100 text-amber-700']">
                {{ p.status === "published" ? "已发布" : "草稿" }}
              </span>
            </td>
            <td class="px-4 py-3 text-gray-400 text-xs">{{ p.updated_at?.slice(0, 10) }}</td>
            <td class="px-4 py-3 text-right">
              <button @click="router.push(`/pages/edit/${p.slug}`)" class="text-indigo-600 hover:text-indigo-800 text-xs mr-3">编辑</button>
              <button @click="handleDelete(p.slug)" class="text-red-500 hover:text-red-700 text-xs">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="pages.length === 0 && !loading" class="py-12 text-center text-sm text-gray-400">暂无页面</p>
    </div>
  </div>
</template>
