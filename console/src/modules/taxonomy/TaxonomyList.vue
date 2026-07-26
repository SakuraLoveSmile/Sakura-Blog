<script setup lang="ts">
import { ref, onMounted } from "vue";
import { taxonomyApi, settingsApi } from "@/api";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VButton, VEmpty, VLoading, Dialog, Toast } from "@/components/ui";

interface TaxItem {
  name: string;
  count: number;
}

const categories = ref<TaxItem[]>([]);
const tags = ref<TaxItem[]>([]);
const loading = ref(false);

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
    Toast.success("配色已保存，前台已生效");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "保存失败");
  } finally {
    savingColors.value = false;
  }
}

async function renameCategory(name: string) {
  const next = prompt(`重命名分类「${name}」为：`, name);
  if (!next || next.trim() === name) return;
  try {
    await taxonomyApi.renameCategory(name, next.trim());
    Toast.success("已重命名");
    load();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "操作失败");
  }
}

function deleteCategory(name: string) {
  Dialog.warning({
    title: `确定从所有文章移除分类「${name}」？`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await taxonomyApi.deleteCategory(name);
        Toast.success("已删除");
        load();
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "操作失败");
      }
    },
  });
}

async function renameTag(name: string) {
  const next = prompt(`重命名标签「${name}」为：`, name);
  if (!next || next.trim() === name) return;
  try {
    await taxonomyApi.renameTag(name, next.trim());
    Toast.success("已重命名");
    load();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "操作失败");
  }
}

function deleteTag(name: string) {
  Dialog.warning({
    title: `确定从所有文章移除标签「${name}」？`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await taxonomyApi.deleteTag(name);
        Toast.success("已删除");
        load();
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "操作失败");
      }
    },
  });
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
    Toast.error("请选择来源标签并填写目标标签");
    return;
  }
  try {
    await taxonomyApi.mergeTags(sources, target);
    Toast.success("已合并");
    cancelMerge();
    load();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "合并失败");
  }
}

onMounted(load);
</script>

<template>
  <div>
    <VPageHeader title="分类 / 标签">
      <template #icon>
        <Icon icon="ri:price-tag-3-line" class="mr-2 self-center text-xl" />
      </template>
    </VPageHeader>

    <VLoading v-if="loading" />

    <div v-else class="m-0 grid grid-cols-1 gap-4 md:m-4 lg:grid-cols-2">
      <!-- 分类 -->
      <VCard :body-class="['!p-0']">
        <template #header>
          <div class="flex items-center justify-between">
            <span class="flex items-center gap-2 text-base font-bold text-gray-900 dark:text-gray-100">
              <Icon icon="ri:folder-3-line" />
              分类
            </span>
            <VButton size="sm" :loading="savingColors" @click="saveColors">保存配色</VButton>
          </div>
        </template>

        <VEmpty v-if="categories.length === 0" title="暂无分类" message="发布文章时设置分类后会显示在这里" />

        <ul v-else class="divide-y divide-gray-100 dark:divide-gray-700">
          <li v-for="c in categories" :key="c.name" class="flex items-center gap-2 px-4 py-2.5">
            <span
              class="h-3.5 w-3.5 shrink-0 rounded-full border border-gray-200 dark:border-gray-600"
              :style="{ background: catColors[c.name] || 'transparent' }"
            ></span>
            <span class="flex-1 truncate text-sm text-gray-800 dark:text-gray-100">{{ c.name }}</span>
            <input
              v-model="catColors[c.name]"
              placeholder="#RRGGBB"
              maxlength="7"
              class="w-20 border border-gray-200 px-2 py-1 font-mono text-xs outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
              style="border-radius: 4px"
            />
            <span class="text-xs text-gray-400 dark:text-gray-500">{{ c.count }} 篇</span>
            <button
              class="text-xs text-gray-500 transition-colors hover:text-blue-600 dark:text-gray-400 dark:hover:text-blue-400"
              @click="renameCategory(c.name)"
            >
              重命名
            </button>
            <button
              class="text-xs text-gray-500 transition-colors hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400"
              @click="deleteCategory(c.name)"
            >
              删除
            </button>
          </li>
        </ul>

        <template #footer>
          <p class="text-xs text-gray-400 dark:text-gray-500">
            配色用于首页卡片分类标记（#RRGGBB，留空=不配色），修改后点右上「保存配色」。
          </p>
        </template>
      </VCard>

      <!-- 标签 -->
      <VCard :body-class="['!p-0']">
        <template #header>
          <div class="flex items-center justify-between">
            <span class="flex items-center gap-2 text-base font-bold text-gray-900 dark:text-gray-100">
              <Icon icon="ri:price-tag-3-line" />
              标签
            </span>
            <div class="flex items-center gap-2">
              <template v-if="mergeMode">
                <input
                  v-model="mergeTarget"
                  placeholder="合并到（目标标签）"
                  class="border border-gray-200 px-2 py-1 text-xs outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
                  style="border-radius: 4px"
                />
                <VButton size="sm" type="secondary" @click="confirmMerge">确认合并</VButton>
                <VButton size="sm" @click="cancelMerge">取消</VButton>
              </template>
              <VButton v-else size="sm" @click="startMerge">
                <template #icon>
                  <Icon icon="ri:git-merge-line" class="h-full w-full" />
                </template>
                合并标签
              </VButton>
            </div>
          </div>
        </template>

        <VEmpty v-if="tags.length === 0" title="暂无标签" message="发布文章时设置标签后会显示在这里" />

        <ul v-else class="divide-y divide-gray-100 dark:divide-gray-700">
          <li v-for="t in tags" :key="t.name" class="flex items-center gap-2 px-4 py-2.5">
            <input
              v-if="mergeMode"
              type="checkbox"
              :checked="selected.has(t.name)"
              class="h-4 w-4 rounded border-gray-300 text-primary"
              @change="toggleSelect(t.name)"
            />
            <span class="flex-1 truncate text-sm text-gray-800 dark:text-gray-100">{{ t.name }}</span>
            <span class="text-xs text-gray-400 dark:text-gray-500">{{ t.count }} 篇</span>
            <template v-if="!mergeMode">
              <button
                class="text-xs text-gray-500 transition-colors hover:text-blue-600 dark:text-gray-400 dark:hover:text-blue-400"
                @click="renameTag(t.name)"
              >
                重命名
              </button>
              <button
                class="text-xs text-gray-500 transition-colors hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400"
                @click="deleteTag(t.name)"
              >
                删除
              </button>
            </template>
          </li>
        </ul>
      </VCard>
    </div>
  </div>
</template>
