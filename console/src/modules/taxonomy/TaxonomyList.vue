<script setup lang="ts">
import { ref, onMounted } from "vue";
import { taxonomyApi, settingsApi } from "@/api";
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

// 分类配色（article.category_color，分类名→#RRGGBB，首页卡片分类颜色）
const catColors = ref<Record<string, string>>({});
const savingColors = ref(false);

// 标签合并选择
const mergeMode = ref(false);
const selected = ref<Set<string>>(new Set());
const mergeTarget = ref("");

async function load() {
  loading.value = true;
  try {
    const [cRes, tRes, sRes] = await Promise.all([
      taxonomyApi.categories(),
      taxonomyApi.tags(),
      settingsApi.all(),
    ]);
    categories.value = cRes.data.items || [];
    tags.value = tRes.data.items || [];
    catColors.value = { ...(sRes.data?.article?.category_color || {}) };
  } finally {
    loading.value = false;
  }
}

// 保存前重拉最新 article 段再合并 category_color，避免覆盖 copyright/share/reward
async function saveColors() {
  savingColors.value = true;
  try {
    const { data: res } = await settingsApi.all();
    const article = { ...(res.article || {}) };
    const colors: Record<string, string> = {};
    for (const [name, color] of Object.entries(catColors.value)) {
      const v = color.trim();
      if (v) colors[name] = v;
    }
    article.category_color = colors;
    await settingsApi.updateSection("article", article);
    notify("配色已保存，前台已生效");
  } catch (e: any) {
    notify(e?.response?.data?.error || "保存失败", true);
  } finally {
    savingColors.value = false;
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
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">分类 / 标签</h2>
      <span v-if="message" :class="['text-sm px-3 py-1.5 rounded-lg', isError ? 'text-red-600 bg-red-50 dark:text-red-400 dark:bg-red-500/10' : 'text-green-700 bg-green-50 dark:text-green-400 dark:bg-green-500/10']">{{ message }}</span>
    </div>

    <div v-if="loading" class="text-sm text-gray-400 py-12 text-center dark:text-gray-500">加载中...</div>

    <div v-else class="grid grid-cols-2 gap-6">
      <!-- 分类 -->
      <div>
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-medium text-gray-500 dark:text-gray-400">分类</h3>
          <button @click="saveColors" :disabled="savingColors" class="text-xs text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300 disabled:opacity-50">保存配色</button>
        </div>
        <div class="bg-white rounded-xl border border-gray-200 divide-y divide-gray-50 dark:bg-gray-800 dark:border-gray-700 dark:divide-gray-700">
          <div v-for="c in categories" :key="c.name" class="flex items-center gap-2 px-4 py-2.5">
            <span class="w-3.5 h-3.5 rounded-full border border-gray-200 shrink-0 dark:border-gray-600" :style="{ background: catColors[c.name] || 'transparent' }"></span>
            <span class="flex-1 text-sm text-gray-800 dark:text-gray-100">{{ c.name }}</span>
            <input v-model="catColors[c.name]" placeholder="#RRGGBB" maxlength="7" class="w-20 px-2 py-1 text-xs font-mono border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
            <span class="text-xs text-gray-400 dark:text-gray-500">{{ c.count }} 篇</span>
            <button @click="renameCategory(c.name)" class="text-indigo-600 hover:text-indigo-800 text-xs dark:text-indigo-400 dark:hover:text-indigo-300">重命名</button>
            <button @click="deleteCategory(c.name)" class="text-red-500 hover:text-red-700 text-xs dark:text-red-400 dark:hover:text-red-300">删除</button>
          </div>
          <p v-if="categories.length === 0" class="py-8 text-center text-sm text-gray-400 dark:text-gray-500">暂无分类</p>
        </div>
        <p class="text-xs text-gray-400 mt-2 dark:text-gray-500">配色用于首页卡片分类标记（#RRGGBB，留空=不配色），修改后点右上「保存配色」。</p>
      </div>

      <!-- 标签 -->
      <div>
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-medium text-gray-500 dark:text-gray-400">标签</h3>
          <div class="flex items-center gap-2">
            <template v-if="mergeMode">
              <input v-model="mergeTarget" placeholder="合并到（目标标签）" class="px-2 py-1 text-xs border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
              <button @click="confirmMerge" class="text-xs text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300">确认合并</button>
              <button @click="cancelMerge" class="text-xs text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300">取消</button>
            </template>
            <button v-else @click="startMerge" class="inline-flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300"><Icon icon="ri:git-merge-line" /> 合并标签</button>
          </div>
        </div>
        <div class="bg-white rounded-xl border border-gray-200 divide-y divide-gray-50 dark:bg-gray-800 dark:border-gray-700 dark:divide-gray-700">
          <div v-for="t in tags" :key="t.name" class="flex items-center gap-2 px-4 py-2.5">
            <input v-if="mergeMode" type="checkbox" :checked="selected.has(t.name)" @change="toggleSelect(t.name)" />
            <span class="flex-1 text-sm text-gray-800 dark:text-gray-100">{{ t.name }}</span>
            <span class="text-xs text-gray-400 dark:text-gray-500">{{ t.count }} 篇</span>
            <template v-if="!mergeMode">
              <button @click="renameTag(t.name)" class="text-indigo-600 hover:text-indigo-800 text-xs dark:text-indigo-400 dark:hover:text-indigo-300">重命名</button>
              <button @click="deleteTag(t.name)" class="text-red-500 hover:text-red-700 text-xs dark:text-red-400 dark:hover:text-red-300">删除</button>
            </template>
          </div>
          <p v-if="tags.length === 0" class="py-8 text-center text-sm text-gray-400 dark:text-gray-500">暂无标签</p>
        </div>
      </div>
    </div>
  </div>
</template>
