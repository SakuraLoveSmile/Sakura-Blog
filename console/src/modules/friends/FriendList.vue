<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { linkApi } from "@/api";
import { Icon } from "@iconify/vue";

interface LinkRow {
  id: number;
  group_title: string;
  name: string;
  url: string;
  avatar: string;
  description: string;
  rss: string;
  label: string;
  color: string;
  sort: number;
}

const links = ref<LinkRow[]>([]);
const loading = ref(false);
const showModal = ref(false);
const saving = ref(false);
const error = ref("");
const editing = ref<Partial<LinkRow>>({});

// 按 group_title 分组展示（保留服务端返回顺序）
const grouped = computed(() => {
  const map: Record<string, LinkRow[]> = {};
  const order: string[] = [];
  for (const l of links.value) {
    const g = l.group_title || "未分组";
    if (!map[g]) {
      map[g] = [];
      order.push(g);
    }
    map[g].push(l);
  }
  return order.map((title) => ({ title, items: map[title] }));
});

async function load() {
  loading.value = true;
  try {
    const { data } = await linkApi.list();
    links.value = data.items || [];
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = { group_title: "", name: "", url: "", avatar: "", description: "", rss: "", label: "", color: "", sort: 0 };
  error.value = "";
  showModal.value = true;
}
function openEdit(l: LinkRow) {
  editing.value = { ...l };
  error.value = "";
  showModal.value = true;
}

async function handleSave() {
  saving.value = true;
  error.value = "";
  try {
    const payload = { ...editing.value };
    if (editing.value.id) {
      await linkApi.update(editing.value.id, payload);
    } else {
      await linkApi.create(payload);
    }
    showModal.value = false;
    load();
  } catch (e: any) {
    error.value = e?.response?.data?.error || "保存失败";
  } finally {
    saving.value = false;
  }
}

async function handleDelete(l: LinkRow) {
  if (!confirm(`确定删除友链「${l.name}」？`)) return;
  await linkApi.delete(l.id);
  load();
}

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";

onMounted(load);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">友链管理</h2>
      <button @click="openCreate" class="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors">
        <Icon icon="ri:add-line" /> 新增友链
      </button>
    </div>

    <div v-if="loading" class="text-sm text-gray-400 py-12 text-center dark:text-gray-500">加载中...</div>
    <p v-else-if="links.length === 0" class="py-12 text-center text-sm text-gray-400 dark:text-gray-500">暂无友链</p>

    <div v-else class="space-y-6">
      <div v-for="g in grouped" :key="g.title">
        <h3 class="text-sm font-medium text-gray-500 mb-2 dark:text-gray-400">{{ g.title }}</h3>
        <div class="bg-white rounded-xl border border-gray-200 divide-y divide-gray-50 dark:bg-gray-800 dark:border-gray-700 dark:divide-gray-700">
          <div v-for="l in g.items" :key="l.id" class="flex items-center gap-3 px-4 py-3">
            <img v-if="l.avatar" :src="l.avatar" class="w-9 h-9 rounded-full object-cover bg-gray-100 dark:bg-gray-700" />
            <div v-else class="w-9 h-9 rounded-full bg-gray-100 flex items-center justify-center text-gray-300 dark:bg-gray-700 dark:text-gray-600"><Icon icon="ri:link" /></div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <a :href="l.url" target="_blank" class="text-sm font-medium text-gray-800 hover:text-indigo-600 truncate dark:text-gray-100 dark:hover:text-indigo-400">{{ l.name }}</a>
                <span v-if="l.label" class="px-1.5 py-0.5 rounded text-[10px] font-medium text-white" :style="{ backgroundColor: l.color ? '#' + l.color : '#9ca3af' }">{{ l.label }}</span>
              </div>
              <p class="text-xs text-gray-400 truncate dark:text-gray-500">{{ l.description }}</p>
            </div>
            <button @click="openEdit(l)" class="text-indigo-600 hover:text-indigo-800 text-xs dark:text-indigo-400 dark:hover:text-indigo-300">编辑</button>
            <button @click="handleDelete(l)" class="text-red-500 hover:text-red-700 text-xs dark:text-red-400 dark:hover:text-red-300">删除</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal -->
    <div v-if="showModal" class="fixed inset-0 bg-black/30 flex items-center justify-center z-50" @click.self="showModal = false">
      <div class="bg-white rounded-xl p-6 w-full max-w-lg max-h-[90vh] overflow-auto dark:bg-gray-800">
        <h3 class="text-lg font-semibold text-gray-800 mb-4 dark:text-gray-100">{{ editing.id ? "编辑友链" : "新增友链" }}</h3>
        <p v-if="error" class="mb-3 text-sm text-red-600 bg-red-50 px-3 py-2 rounded-lg dark:text-red-400 dark:bg-red-500/10">{{ error }}</p>
        <div class="space-y-3">
          <div class="grid grid-cols-2 gap-3">
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">分组</label><input v-model="editing.group_title" :class="inputCls" placeholder="我的朋友" /></div>
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">名称 *</label><input v-model="editing.name" :class="inputCls" /></div>
          </div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">链接 *</label><input v-model="editing.url" :class="inputCls" placeholder="https://" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">头像 URL</label><input v-model="editing.avatar" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">描述</label><input v-model="editing.description" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">RSS/Atom（动态聚合用）</label><input v-model="editing.rss" :class="inputCls" /></div>
          <div class="grid grid-cols-3 gap-3">
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">标签</label><input v-model="editing.label" :class="inputCls" /></div>
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">配色（不带 #）</label><input v-model="editing.color" :class="inputCls" placeholder="1BCDFC" /></div>
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">排序（大在前）</label><input v-model.number="editing.sort" type="number" :class="inputCls" /></div>
          </div>
        </div>
        <div class="flex justify-end gap-2 mt-5">
          <button @click="showModal = false" class="px-4 py-2 text-sm text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200">取消</button>
          <button @click="handleSave" :disabled="saving" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">{{ saving ? "保存中..." : "保存" }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
