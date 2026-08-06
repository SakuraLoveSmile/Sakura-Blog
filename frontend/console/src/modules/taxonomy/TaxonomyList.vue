<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { taxonomyApi, settingsApi } from "@/api";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, VEmpty, VLoading, VModal, Dialog, Toast } from "@/components/ui";

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

// 视图切换与搜索
const activeTab = ref<"categories" | "tags">("categories");
const searchQuery = ref("");

// 分类图标盒 5 色系统渐变（按索引取色）
const categoryIconGradients = [
  "linear-gradient(135deg, #0A84FF, #409CFF)",
  "linear-gradient(135deg, #30D158, #34E07A)",
  "linear-gradient(135deg, #BF5AF2, #D47BFF)",
  "linear-gradient(135deg, #FF9F0A, #FFB340)",
  "linear-gradient(135deg, #FF453A, #FF6B60)",
];
const categoryIconShadow = [
  "0 4px 12px rgba(10,132,255,0.35)",
  "0 4px 12px rgba(48,209,88,0.35)",
  "0 4px 12px rgba(191,90,242,0.35)",
  "0 4px 12px rgba(255,159,10,0.35)",
  "0 4px 12px rgba(255,69,58,0.35)",
];
const categoryIconKeys = [
  "ri:monitor-line",
  "ri:book-open-line",
  "ri:compass-3-line",
  "ri:star-line",
  "ri:folder-chart-line",
];

// 标签云 7 色循环（低透明度背景 + 同色边框）
const tagPalette = [
  { bg: "rgba(10,132,255,0.15)", border: "rgba(10,132,255,0.25)", text: "#409CFF", glow: "rgba(10,132,255,0.2)" },
  { bg: "rgba(191,90,242,0.15)", border: "rgba(191,90,242,0.25)", text: "#D47BFF", glow: "rgba(191,90,242,0.2)" },
  { bg: "rgba(48,209,88,0.15)", border: "rgba(48,209,88,0.25)", text: "#5DDF7D", glow: "rgba(48,209,88,0.2)" },
  { bg: "rgba(255,159,10,0.15)", border: "rgba(255,159,10,0.25)", text: "#FFB340", glow: "rgba(255,159,10,0.2)" },
  { bg: "rgba(255,55,95,0.15)", border: "rgba(255,55,95,0.25)", text: "#FF7A8F", glow: "rgba(255,55,95,0.2)" },
  { bg: "rgba(50,173,230,0.15)", border: "rgba(50,173,230,0.25)", text: "#64D2FF", glow: "rgba(50,173,230,0.2)" },
  { bg: "rgba(94,92,230,0.15)", border: "rgba(94,92,230,0.25)", text: "#8B89F0", glow: "rgba(94,92,230,0.2)" },
];

// 重命名弹窗（替代 prompt()）
const renameModalVisible = ref(false);
const renameTarget = ref<{ type: "category" | "tag"; name: string } | null>(null);
const renameValue = ref("");
const renaming = ref(false);

// 新建提示弹窗（后端无独立创建接口，分类/标签随文章产生）
const createHintVisible = ref(false);

function tagSizeClass(count: number): string {
  if (count >= 25) return "tag-chip-xl";
  if (count >= 15) return "tag-chip-lg";
  if (count >= 8) return "tag-chip-md";
  return "tag-chip-sm";
}

function tagStyle(index: number) {
  const c = tagPalette[index % tagPalette.length];
  return {
    backgroundColor: c.bg,
    borderColor: c.border,
    color: c.text,
    "--tag-glow": c.glow,
  };
}

const filteredCategories = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return categories.value;
  return categories.value.filter((c) => c.name.toLowerCase().includes(q));
});

const filteredTags = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return tags.value;
  return tags.value.filter((t) => t.name.toLowerCase().includes(q));
});

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

function openRenameCategory(name: string) {
  renameTarget.value = { type: "category", name };
  renameValue.value = name;
  renameModalVisible.value = true;
}

function openRenameTag(name: string) {
  renameTarget.value = { type: "tag", name };
  renameValue.value = name;
  renameModalVisible.value = true;
}

async function confirmRename() {
  if (!renameTarget.value) return;
  const { type, name } = renameTarget.value;
  const next = renameValue.value.trim();
  if (!next || next === name) {
    renameModalVisible.value = false;
    return;
  }
  renaming.value = true;
  try {
    if (type === "category") {
      await taxonomyApi.renameCategory(name, next);
    } else {
      await taxonomyApi.renameTag(name, next);
    }
    Toast.success("已重命名");
    renameModalVisible.value = false;
    load();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "操作失败");
  } finally {
    renaming.value = false;
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
  <div class="apple-ambient-glow min-h-full">
    <VPageHeader title="分类与标签">
      <template #icon>
        <Icon icon="ri:price-tag-3-line" class="mr-2 self-center text-xl" />
      </template>
      <template #subtitle>管理文章的分类与标签，配色用于前台卡片展示</template>
      <template #actions>
        <div class="apple-search" style="width: 280px;">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="searchQuery" :placeholder="activeTab === 'categories' ? '搜索分类...' : '搜索标签...'" />
        </div>
        <VButton type="primary" @click="createHintVisible = true">
          <template #icon>
            <Icon icon="ri:add-line" class="h-full w-full" />
          </template>
          {{ activeTab === "categories" ? "新建分类" : "新建标签" }}
        </VButton>
      </template>
    </VPageHeader>

    <div class="px-5 py-5 md:px-10">
      <VLoading v-if="loading" />

      <template v-else>
        <!-- 胶囊分段控件 -->
        <div class="apple-segmented mb-5">
          <button
            type="button"
            :class="{ active: activeTab === 'categories' }"
            @click="activeTab = 'categories'"
          >
            分类
          </button>
          <button
            type="button"
            :class="{ active: activeTab === 'tags' }"
            @click="activeTab = 'tags'"
          >
            标签
          </button>
        </div>

        <!-- ============ 分类视图 ============ -->
        <section v-if="activeTab === 'categories'">
          <div class="mb-5 flex items-center justify-between">
            <div class="flex items-center gap-3">
              <h2 class="text-[17px] font-bold tracking-tight text-foreground">全部分类</h2>
              <span
                class="rounded-full border border-border bg-card px-2.5 py-0.5 text-[13px] font-medium text-text-tertiary"
              >
                {{ categories.length }} 个分类
              </span>
            </div>
            <VButton size="sm" :loading="savingColors" @click="saveColors">保存配色</VButton>
          </div>

          <VEmpty
            v-if="filteredCategories.length === 0"
            :title="searchQuery ? '未找到匹配的分类' : '暂无分类'"
            :message="searchQuery ? '换个关键词试试' : '发布文章时设置分类后会显示在这里'"
          />

          <div v-else class="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div
              v-for="(c, idx) in filteredCategories"
              :key="c.name"
              class="apple-card-highlight group relative flex items-center gap-4 rounded-[14px] border border-border bg-card p-5 transition-all duration-200 hover:-translate-y-px hover:shadow-[var(--shadow-lg)]"
            >
              <div
                class="flex h-12 w-12 shrink-0 items-center justify-center rounded-[14px] text-white"
                :style="{
                  background: categoryIconGradients[idx % categoryIconGradients.length],
                  boxShadow: categoryIconShadow[idx % categoryIconShadow.length],
                }"
              >
                <Icon :icon="categoryIconKeys[idx % categoryIconKeys.length]" class="text-2xl" />
              </div>

              <div class="min-w-0 flex-1">
                <div class="mb-0.5 truncate text-[15px] font-semibold tracking-tight text-foreground">
                  {{ c.name }}
                </div>
                <div class="taxonomy-clamp mb-2 text-xs leading-snug text-text-tertiary">
                  <input
                    v-model="catColors[c.name]"
                    placeholder="#RRGGBB（前台分类标记配色，留空=不配色）"
                    maxlength="7"
                    class="h-7 w-full rounded-md border border-border bg-secondary px-2 font-mono text-xs text-foreground outline-none transition-colors placeholder:text-text-quaternary focus:border-primary"
                  />
                </div>
                <div class="flex items-center gap-2">
                  <span
                    class="inline-flex items-center justify-center rounded-full bg-accent px-2.5 py-0.5 text-[11px] font-semibold tracking-wide text-text-tertiary"
                  >
                    {{ c.count }} 篇
                  </span>
                  <span
                    v-if="catColors[c.name] && catColors[c.name].trim()"
                    class="inline-flex items-center gap-1 rounded-full border border-border bg-secondary px-2 py-0.5 text-[11px] text-text-tertiary"
                  >
                    <span
                      class="h-2.5 w-2.5 rounded-full"
                      :style="{ background: catColors[c.name] }"
                    ></span>
                    配色已设
                  </span>
                </div>
              </div>

              <div class="flex shrink-0 items-center gap-1.5">
                <button
                  type="button"
                  title="重命名"
                  class="flex h-8 w-8 items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-accent hover:text-foreground"
                  @click="openRenameCategory(c.name)"
                >
                  <Icon icon="ri:pencil-line" />
                </button>
                <button
                  type="button"
                  title="删除"
                  class="flex h-8 w-8 items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-accent hover:text-[var(--color-state-error)]"
                  @click="deleteCategory(c.name)"
                >
                  <Icon icon="ri:delete-bin-line" />
                </button>
              </div>
            </div>
          </div>

          <p class="mt-4 text-xs text-text-quaternary">
            配色用于首页卡片分类标记（#RRGGBB，留空=不配色），修改后点右上「保存配色」。分类随文章创建，暂不支持单独新建。
          </p>
        </section>

        <!-- ============ 标签视图 ============ -->
        <section v-else>
          <div class="mb-6 flex items-center justify-between">
            <div class="flex items-center gap-3">
              <h2 class="text-[17px] font-bold tracking-tight text-foreground">全部标签</h2>
              <span
                class="rounded-full border border-border bg-card px-2.5 py-0.5 text-[13px] font-medium text-text-tertiary"
              >
                {{ tags.length }} 个标签
              </span>
            </div>
            <div class="flex items-center gap-2">
              <template v-if="mergeMode">
                <input
                  v-model="mergeTarget"
                  placeholder="合并到（目标标签）"
                  class="apple-input"
                  style="width: 200px;"
                />
                <VButton size="sm" type="primary" @click="confirmMerge">确认合并</VButton>
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

          <VEmpty
            v-if="filteredTags.length === 0"
            :title="searchQuery ? '未找到匹配的标签' : '暂无标签'"
            :message="searchQuery ? '换个关键词试试' : '发布文章时设置标签后会显示在这里'"
          />

          <div v-else-if="mergeMode" class="space-y-1.5">
            <div
              v-for="t in filteredTags"
              :key="t.name"
              class="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-2.5"
            >
              <input
                type="checkbox"
                :checked="selected.has(t.name)"
                class="h-4 w-4 rounded border-border-strong text-primary"
                @change="toggleSelect(t.name)"
              />
              <span class="flex-1 truncate text-sm text-foreground">{{ t.name }}</span>
              <span class="text-xs text-text-quaternary">{{ t.count }} 篇</span>
            </div>
            <p class="pt-1 text-xs text-text-quaternary">
              已选 {{ selected.size }} 个来源标签，将合并到目标标签。
            </p>
          </div>

          <div v-else class="flex flex-wrap items-center gap-2.5">
            <span
              v-for="(t, idx) in filteredTags"
              :key="t.name"
              :class="['tag-chip', tagSizeClass(t.count)]"
              :style="tagStyle(idx)"
            >
              {{ t.name }}
              <span class="tag-count">{{ t.count }}</span>
              <span class="tag-actions">
                <button
                  type="button"
                  title="重命名"
                  @click.stop="openRenameTag(t.name)"
                >
                  <Icon icon="ri:pencil-line" />
                </button>
                <button
                  type="button"
                  title="删除"
                  @click.stop="deleteTag(t.name)"
                >
                  <Icon icon="ri:delete-bin-line" />
                </button>
              </span>
            </span>
          </div>
        </section>
      </template>
    </div>

    <!-- 重命名弹窗（替代 prompt()） -->
    <VModal v-model:visible="renameModalVisible" title="重命名" :width="440">
      <div v-if="renameTarget">
        <label class="mb-1.5 block text-xs text-text-tertiary">
          将「{{ renameTarget.name }}」{{ renameTarget.type === "category" ? "分类" : "标签" }}重命名为
        </label>
        <input
          v-model="renameValue"
          class="apple-input"
          placeholder="请输入新名称"
          @keyup.enter="confirmRename"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="renameModalVisible = false">取消</VButton>
          <VButton type="primary" :loading="renaming" @click="confirmRename">确认</VButton>
        </div>
      </template>
    </VModal>

    <!-- 新建提示弹窗（后端无独立创建接口） -->
    <VModal
      v-model:visible="createHintVisible"
      :title="activeTab === 'categories' ? '新建分类' : '新建标签'"
      :width="440"
    >
      <p class="text-sm leading-relaxed text-text-tertiary">
        {{ activeTab === "categories" ? "分类" : "标签" }}会在发布或编辑文章时自动创建。
        请前往文章编辑器，在对应字段中输入新的{{ activeTab === "categories" ? "分类" : "标签" }}名称并发布即可。
      </p>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton type="primary" @click="createHintVisible = false">我知道了</VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>
.taxonomy-clamp {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 标签云胶囊 */
.tag-chip {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: var(--radius-full);
  font-weight: 500;
  letter-spacing: -0.1px;
  border: 1px solid transparent;
  cursor: default;
  transition: transform 0.2s cubic-bezier(0.25, 0.1, 0.25, 1),
    box-shadow 0.2s cubic-bezier(0.25, 0.1, 0.25, 1),
    background-color 0.2s ease;
}
.tag-chip:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px var(--tag-glow);
  filter: brightness(1.15);
}
.tag-chip-sm {
  padding: 6px 14px;
  font-size: 12px;
}
.tag-chip-md {
  padding: 7px 16px;
  font-size: 13px;
}
.tag-chip-lg {
  padding: 8px 18px;
  font-size: 14px;
}
.tag-chip-xl {
  padding: 9px 20px;
  font-size: 15px;
  font-weight: 600;
}
.tag-count {
  font-size: 10px;
  font-weight: 600;
  opacity: 0.75;
  padding: 1px 6px;
  background-color: rgba(255, 255, 255, 0.1);
  border-radius: var(--radius-full);
  min-width: 18px;
  text-align: center;
}

/* 悬停显示的标签操作按钮 */
.tag-actions {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-left: 2px;
  opacity: 0;
  transition: opacity 0.18s ease;
}
.tag-chip:hover .tag-actions {
  opacity: 1;
}
.tag-actions button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: var(--radius-full);
  color: inherit;
  opacity: 0.8;
  transition: all 0.15s ease;
}
.tag-actions button:hover {
  background: rgba(255, 255, 255, 0.18);
  opacity: 1;
}
</style>
