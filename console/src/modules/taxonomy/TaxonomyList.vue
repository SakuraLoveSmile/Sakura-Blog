<script setup lang="ts">
import { ref, onMounted } from "vue";
import { taxonomyApi } from "@/api";
import { Icon } from "@iconify/vue";

interface TaxItem {
  name: string;
  count: number;
}

const categories = ref<TaxItem[]>([]);
const tags = ref<TaxItem[]>([]);
const loading = ref(false);
const message = ref("");
const isError = ref(false);

// 标签合并选择
const mergeMode = ref(false);
const selected = ref<Set<string>>(new Set());
const mergeTarget = ref("");

async function load() {
  loading.value = true;
  try {
    const [cRes, tRes] = await Promise.all([taxonomyApi.categories(), taxonomyApi.tags()]);
    categories.value = cRes.data.items || [];
    tags.value = tRes.data.items || [];
  } finally {
    loading.value = false;
  }
}

function notify(msg: string, err = false) {
  message.value = msg;
  isError.value = err;
  setTimeout(() => (message.value = ""), 3000);
}

async function renameCategory(name: string) {
  const next = prompt(`重命名分类「${name}」为：`, name);
  if (!next || next.trim() === name) return;
  try {
    await taxonomyApi.renameCategory(name, next.trim());
    notify("已重命名");
    load();
  } catch (e: any) {
    notify(e?.response?.data?.error || "操作失败", true);
  }
}
async function deleteCategory(name: string) {
  if (!confirm(`确定从所有文章移除分类「${name}」？`)) return;
  try {
    await taxonomyApi.deleteCategory(name);
    notify("已删除");
    load();
  } catch (e: any) {
    notify(e?.response?.data?.error || "操作失败", true);
  }
}
async function renameTag(name: string) {
  const next = prompt(`重命名标签「${name}」为：`, name);
  if (!next || next.trim() === name) return;
  try {
    await taxonomyApi.renameTag(name, next.trim());
    notify("已重命名");
    load();
  } catch (e: any) {
    notify(e?.response?.data?.error || "操作失败", true);
  }
}
async function deleteTag(name: string) {
  if (!confirm(`确定从所有文章移除标签「${name}」？`)) return;
  try {
    await taxonomyApi.deleteTag(name);
    notify("已删除");
    load();
  } catch (e: any) {
    notify(e?.response?.data?.error || "操作失败", true);
  }
}

function toggleSelect(name: string) {
  if (selected.value.has(name)) selected.value.delete(name);
  else selected.value.add(name);
}
function startMerge() {
  mergeMode.value = true;
  selected.value = new Set();
  mergeTarget.value = "";
}
function cancelMerge() {
  mergeMode.value = false;
  selected.value = new Set();
}
async function confirmMerge() {
  const sources = [...selected.value];
  const target = mergeTarget.value.trim();
  if (!target || sources.length === 0) {
    notify("请选择来源标签并填写目标标签", true);
    return;
  }
  try {
    await taxonomyApi.mergeTags(sources, target);
    notify("已合并");
    cancelMerge();
    load();
  } catch (e: any) {
    notify(e?.response?.data?.error || "合并失败", true);
  }
}

onMounted(load);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800">分类 / 标签</h2>
      <span v-if="message" :class="['text-sm px-3 py-1.5 rounded-lg', isError ? 'text-red-600 bg-red-50' : 'text-green-700 bg-green-50']">{{ message }}</span>
    </div>

    <div v-if="loading" class="text-sm text-gray-400 py-12 text-center">加载中...</div>

    <div v-else class="grid grid-cols-2 gap-6">
      <!-- 分类 -->
      <div>
        <h3 class="text-sm font-medium text-gray-500 mb-2">分类</h3>
        <div class="bg-white rounded-xl border border-gray-200 divide-y divide-gray-50">
          <div v-for="c in categories" :key="c.name" class="flex items-center gap-2 px-4 py-2.5">
            <span class="flex-1 text-sm text-gray-800">{{ c.name }}</span>
            <span class="text-xs text-gray-400">{{ c.count }} 篇</span>
            <button @click="renameCategory(c.name)" class="text-indigo-600 hover:text-indigo-800 text-xs">重命名</button>
            <button @click="deleteCategory(c.name)" class="text-red-500 hover:text-red-700 text-xs">删除</button>
          </div>
          <p v-if="categories.length === 0" class="py-8 text-center text-sm text-gray-400">暂无分类</p>
        </div>
      </div>

      <!-- 标签 -->
      <div>
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-medium text-gray-500">标签</h3>
          <div class="flex items-center gap-2">
            <template v-if="mergeMode">
              <input v-model="mergeTarget" placeholder="合并到（目标标签）" class="px-2 py-1 text-xs border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-indigo-500" />
              <button @click="confirmMerge" class="text-xs text-indigo-600 hover:text-indigo-800">确认合并</button>
              <button @click="cancelMerge" class="text-xs text-gray-400 hover:text-gray-600">取消</button>
            </template>
            <button v-else @click="startMerge" class="inline-flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800"><Icon icon="ri:git-merge-line" /> 合并标签</button>
          </div>
        </div>
        <div class="bg-white rounded-xl border border-gray-200 divide-y divide-gray-50">
          <div v-for="t in tags" :key="t.name" class="flex items-center gap-2 px-4 py-2.5">
            <input v-if="mergeMode" type="checkbox" :checked="selected.has(t.name)" @change="toggleSelect(t.name)" />
            <span class="flex-1 text-sm text-gray-800">{{ t.name }}</span>
            <span class="text-xs text-gray-400">{{ t.count }} 篇</span>
            <template v-if="!mergeMode">
              <button @click="renameTag(t.name)" class="text-indigo-600 hover:text-indigo-800 text-xs">重命名</button>
              <button @click="deleteTag(t.name)" class="text-red-500 hover:text-red-700 text-xs">删除</button>
            </template>
          </div>
          <p v-if="tags.length === 0" class="py-8 text-center text-sm text-gray-400">暂无标签</p>
        </div>
      </div>
    </div>
  </div>
</template>
