<script setup lang="ts">
import { ref, computed, reactive, onMounted } from "vue";
import { taxonomyApi, settingsApi } from "@/api";
import { Icon } from "@iconify/vue";
import IconPicker from "@/components/IconPicker.vue";
import {
  VPageHeader,
  VButton,
  VCard,
  VEmpty,
  VLoading,
  VModal,
  VFloatingDock,
  Dialog,
  Toast,
} from "@/components/ui";

interface TaxItem {
  id?: number;
  name: string;
  path?: string;
  parent_id?: number;
  count: number;
}

interface TreeNode extends TaxItem {
  depth: number;
  children: TreeNode[];
  hasChildren: boolean;
}

const categories = ref<TaxItem[]>([]);
const tags = ref<TaxItem[]>([]);
const loading = ref(false);

// 分类配色（article.category_color，分类名→#RRGGBB，首页卡片分类颜色）
const catColors = ref<Record<string, string>>({});
// 分类/标签图标（article.category_icon / tag_icon，名称→icon key 或 URL，前台面包屑与索引页消费）
const catIcons = ref<Record<string, string>>({});
const tagIcons = ref<Record<string, string>>({});
const savingSettings = ref(false);

// 视图切换与搜索
const activeTab = ref<"categories" | "tags">("categories");
const searchQuery = ref("");

// 标签视图模式：云视图 / 表格视图
const tagViewMode = ref<"cloud" | "table">("cloud");
const tagSortBy = ref<"count" | "name">("count");
const tagSortOrder = ref<"desc" | "asc">("desc");

// 标签合并选择
const mergeMode = ref(false);
const selectedTags = ref<Set<string>>(new Set());
const mergeTarget = ref("");
const merging = ref(false);

// 树形节点折叠状态（以 category.id 为 key）
const collapsedNodeIds = ref<Set<number>>(new Set());

// Apple Dark 8 款精选预设色板
const APPLE_PRESET_COLORS = [
  { label: "Apple 蓝", value: "#007AFF" },
  { label: "极光紫", value: "#AF52DE" },
  { label: "深邃靛", value: "#5856D6" },
  { label: "湖泊青", value: "#30B0C7" },
  { label: "薄荷绿", value: "#00C7BE" },
  { label: "荧光绿", value: "#30D158" },
  { label: "琥珀橙", value: "#FF9500" },
  { label: "珊瑚红", value: "#FF453A" },
];

// 分类图标头像渐变
const categoryIconGradients = [1, 2, 3, 4, 5].map((n) => `avatar-gradient-${n}`);
const categoryDefaultIcons = [
  "ri:folder-line",
  "ri:code-box-line",
  "ri:book-open-line",
  "ri:compass-3-line",
  "ri:star-line",
  "ri:folder-chart-line",
];

// 统一分类新建/编辑弹窗
const categoryModalVisible = ref(false);
const categoryModalMode = ref<"create" | "edit">("create");
const categorySaving = ref(false);
const categoryForm = reactive({
  id: 0,
  name: "",
  parent_id: 0,
  color: "",
  icon: "",
  initialKey: "",
});

// 重命名标签弹窗
const renameModalVisible = ref(false);
const renameTargetTag = ref("");
const renameValue = ref("");
const renaming = ref(false);

// 标签图标设置弹窗
const tagIconModalVisible = ref(false);
const tagIconTarget = ref("");
const tagIconValue = ref("");
const tagIconSaving = ref(false);

// ---------- 统计数据计算 ----------
const stats = computed(() => {
  const totalCats = categories.value.length;
  const rootCats = categories.value.filter((c) => !c.parent_id || c.parent_id === 0).length;
  const subCats = totalCats - rootCats;
  const totalTgs = tags.value.length;
  const activeTgs = tags.value.filter((t) => t.count > 0).length;
  const totalPostRefs = categories.value.reduce((sum, c) => sum + (c.count || 0), 0);

  return {
    totalCats,
    rootCats,
    subCats,
    totalTgs,
    activeTgs,
    totalPostRefs,
  };
});

// ---------- 分类工具函数与树构建 ----------
function categoryKey(c: TaxItem): string {
  return c.path || c.name;
}

function isIconUrl(v?: string): boolean {
  const s = (v || "").trim();
  return s.startsWith("http://") || s.startsWith("https://") || s.startsWith("/");
}

// 构建分类树状结构
const categoryTree = computed<TreeNode[]>(() => {
  const map = new Map<number, TreeNode>();
  const roots: TreeNode[] = [];

  // 1. 初始化所有节点
  for (const c of categories.value) {
    const id = c.id || 0;
    map.set(id, {
      ...c,
      depth: 0,
      children: [],
      hasChildren: false,
    });
  }

  // 2. 挂载父子关系
  for (const c of categories.value) {
    const id = c.id || 0;
    const node = map.get(id)!;
    const pId = c.parent_id || 0;
    if (pId > 0 && map.has(pId)) {
      const parent = map.get(pId)!;
      parent.children.push(node);
      parent.hasChildren = true;
    } else {
      roots.push(node);
    }
  }

  // 3. 递归标记深度
  function markDepth(nodes: TreeNode[], depth: number) {
    for (const n of nodes) {
      n.depth = depth;
      if (n.children.length > 0) {
        markDepth(n.children, depth + 1);
      }
    }
  }
  markDepth(roots, 0);

  return roots;
});

// 扁平化展示树（受折叠与搜索过滤控制）
const flatVisibleCategories = computed<TreeNode[]>(() => {
  const q = searchQuery.value.trim().toLowerCase();

  // 若存在搜索词，直接扁平匹配展示所有命中的分类
  if (q) {
    return categories.value
      .filter((c) => (c.path || c.name).toLowerCase().includes(q))
      .map((c) => {
        const depth = Math.max(0, categoryKey(c).split("/").length - 1);
        return {
          ...c,
          depth,
          children: [],
          hasChildren: false,
        };
      });
  }

  // 无搜索词时，按树形递归展开
  const list: TreeNode[] = [];
  function traverse(nodes: TreeNode[]) {
    for (const node of nodes) {
      list.push(node);
      const isCollapsed = node.id && collapsedNodeIds.value.has(node.id);
      if (!isCollapsed && node.children.length > 0) {
        traverse(node.children);
      }
    }
  }
  traverse(categoryTree.value);
  return list;
});

function toggleNodeCollapse(id?: number) {
  if (!id) return;
  if (collapsedNodeIds.value.has(id)) {
    collapsedNodeIds.value.delete(id);
  } else {
    collapsedNodeIds.value.add(id);
  }
}

// 可选父分类列表（新建/编辑时使用，防止自选和循环依赖）
const availableParentCategories = computed(() => {
  if (categoryModalMode.value === "create") {
    return categories.value;
  }
  // 编辑模式：排除自身以及所有子孙节点
  const currentId = categoryForm.id;
  if (!currentId) return categories.value;

  const forbiddenIds = new Set<number>([currentId]);
  function collectDescendants(nodes: TreeNode[]) {
    for (const n of nodes) {
      if (forbiddenIds.has(n.parent_id || 0) && n.id) {
        forbiddenIds.add(n.id);
      }
      if (n.children.length > 0) {
        collectDescendants(n.children);
      }
    }
  }
  collectDescendants(categoryTree.value);

  return categories.value.filter((c) => c.id && !forbiddenIds.has(c.id));
});

// ---------- 标签排序与过滤 ----------
const filteredAndSortedTags = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  let list = tags.value;
  if (q) {
    list = list.filter((t) => t.name.toLowerCase().includes(q));
  }

  return [...list].sort((a, b) => {
    if (tagSortBy.value === "count") {
      const diff = b.count - a.count;
      return tagSortOrder.value === "desc" ? diff : -diff;
    } else {
      const cmp = a.name.localeCompare(b.name, "zh-CN");
      return tagSortOrder.value === "desc" ? -cmp : cmp;
    }
  });
});

function toggleTagSort(by: "count" | "name") {
  if (tagSortBy.value === by) {
    tagSortOrder.value = tagSortOrder.value === "desc" ? "asc" : "desc";
  } else {
    tagSortBy.value = by;
    tagSortOrder.value = "desc";
  }
}

// ---------- API 加载与设置同步 ----------
async function load() {
  loading.value = true;
  try {
    const [cRes, tRes, sRes] = await Promise.all([
      taxonomyApi.categories(),
      taxonomyApi.tags(),
      settingsApi.all(),
    ]);
    categories.value = (cRes.data.items || []).map((c: any) => ({
      id: Number(c.id || 0),
      name: c.name,
      path: c.path || c.name,
      parent_id: Number(c.parent_id || 0),
      count: Number(c.count || 0),
    }));
    tags.value = tRes.data.items || [];
    catColors.value = { ...(sRes.data?.article?.category_color || {}) };
    catIcons.value = { ...(sRes.data?.article?.category_icon || {}) };
    tagIcons.value = { ...(sRes.data?.article?.tag_icon || {}) };
  } finally {
    loading.value = false;
  }
}

// 持久化 article 段的 category_color / category_icon / tag_icon
async function saveTaxonomySettings(
  updatedColors?: Record<string, string>,
  updatedCatIcons?: Record<string, string>,
  updatedTagIcons?: Record<string, string>
) {
  savingSettings.value = true;
  try {
    const { data: res } = await settingsApi.all();
    const article = { ...(res.article || {}) };

    const pick = (m: Record<string, string>) => {
      const out: Record<string, string> = {};
      for (const [name, v] of Object.entries(m)) {
        const s = (v || "").trim();
        if (s) out[name] = s;
      }
      return out;
    };

    article.category_color = pick(updatedColors ?? catColors.value);
    article.category_icon = pick(updatedCatIcons ?? catIcons.value);
    article.tag_icon = pick(updatedTagIcons ?? tagIcons.value);

    await settingsApi.updateSection("article", article);
    catColors.value = article.category_color;
    catIcons.value = article.category_icon;
    tagIcons.value = article.tag_icon;
    return true;
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "保存配色/图标失败");
    return false;
  } finally {
    savingSettings.value = false;
  }
}

// ---------- 分类弹窗操作 ----------
function openCreateCategory(parentId = 0) {
  categoryModalMode.value = "create";
  categoryForm.id = 0;
  categoryForm.name = "";
  categoryForm.parent_id = parentId;
  categoryForm.color = "";
  categoryForm.icon = "";
  categoryForm.initialKey = "";
  categoryModalVisible.value = true;
}

function openEditCategory(c: TaxItem) {
  categoryModalMode.value = "edit";
  categoryForm.id = c.id || 0;
  categoryForm.name = c.name;
  categoryForm.parent_id = c.parent_id || 0;
  const key = categoryKey(c);
  categoryForm.initialKey = key;
  categoryForm.color = catColors.value[key] || "";
  categoryForm.icon = catIcons.value[key] || "";
  categoryModalVisible.value = true;
}

async function saveCategoryModal() {
  const name = categoryForm.name.trim();
  if (!name) {
    Toast.warning("分类名称不能为空");
    return;
  }
  if (name.includes("/")) {
    Toast.warning("分类名称不能包含斜杠 '/'");
    return;
  }

  categorySaving.value = true;
  try {
    let savedKey = name;
    if (categoryModalMode.value === "create") {
      const res = await taxonomyApi.createCategory(name, categoryForm.parent_id);
      savedKey = res.data?.path || name;
      Toast.success("分类创建成功");
    } else {
      if (categoryForm.id) {
        const res = await taxonomyApi.updateCategory(categoryForm.id, {
          name,
          parent_id: categoryForm.parent_id,
        });
        savedKey = res.data?.path || name;
      } else {
        await taxonomyApi.renameCategory(categoryForm.initialKey, name);
      }
      Toast.success("分类修改成功");
    }

    // 同步更新并保存配色与图标
    const nextColors = { ...catColors.value };
    const nextIcons = { ...catIcons.value };

    // 若旧 key 变化，清理旧 key
    if (categoryForm.initialKey && categoryForm.initialKey !== savedKey) {
      delete nextColors[categoryForm.initialKey];
      delete nextIcons[categoryForm.initialKey];
    }

    if (categoryForm.color?.trim()) {
      nextColors[savedKey] = categoryForm.color.trim();
    } else {
      delete nextColors[savedKey];
    }

    if (categoryForm.icon?.trim()) {
      nextIcons[savedKey] = categoryForm.icon.trim();
    } else {
      delete nextIcons[savedKey];
    }

    await saveTaxonomySettings(nextColors, nextIcons);
    categoryModalVisible.value = false;
    await load();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "操作失败");
  } finally {
    categorySaving.value = false;
  }
}

function deleteCategory(c: TaxItem) {
  const name = categoryKey(c);
  Dialog.warning({
    title: `确定删除分类「${c.name}」？`,
    description: "仅空叶子分类允许删除。若该分类下有关联文章或子分类，需先清理关联。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        if (c.id) await taxonomyApi.deleteCategoryById(c.id);
        else await taxonomyApi.deleteCategory(name);
        Toast.success("已删除");
        // 清理对应的配色与图标
        const nextColors = { ...catColors.value };
        const nextIcons = { ...catIcons.value };
        delete nextColors[name];
        delete nextIcons[name];
        await saveTaxonomySettings(nextColors, nextIcons);
        await load();
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      }
    },
  });
}

// ---------- 标签操作 ----------
function openRenameTag(name: string) {
  renameTargetTag.value = name;
  renameValue.value = name;
  renameModalVisible.value = true;
}

async function confirmRenameTag() {
  const old = renameTargetTag.value;
  const next = renameValue.value.trim();
  if (!next || next === old) {
    renameModalVisible.value = false;
    return;
  }
  renaming.value = true;
  try {
    await taxonomyApi.renameTag(old, next);
    // 同步迁移标签图标
    if (tagIcons.value[old]) {
      const nextTagIcons = { ...tagIcons.value };
      nextTagIcons[next] = nextTagIcons[old];
      delete nextTagIcons[old];
      await saveTaxonomySettings(undefined, undefined, nextTagIcons);
    }
    Toast.success("标签已重命名");
    renameModalVisible.value = false;
    await load();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "重命名失败");
  } finally {
    renaming.value = false;
  }
}

function deleteTag(name: string) {
  Dialog.warning({
    title: `确定从所有文章移除标签「${name}」？`,
    description: "该操作将直接修改所有关联文章的标签记录，移除后不可撤销。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await taxonomyApi.deleteTag(name);
        const nextTagIcons = { ...tagIcons.value };
        delete nextTagIcons[name];
        await saveTaxonomySettings(undefined, undefined, nextTagIcons);
        Toast.success("标签已移除");
        await load();
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      }
    },
  });
}

function openTagIcon(name: string) {
  tagIconTarget.value = name;
  tagIconValue.value = tagIcons.value[name] || "";
  tagIconModalVisible.value = true;
}

async function confirmTagIcon() {
  const v = tagIconValue.value.trim();
  const nextTagIcons = { ...tagIcons.value };
  if (v) nextTagIcons[tagIconTarget.value] = v;
  else delete nextTagIcons[tagIconTarget.value];

  tagIconSaving.value = true;
  const ok = await saveTaxonomySettings(undefined, undefined, nextTagIcons);
  tagIconSaving.value = false;
  if (ok) {
    tagIconModalVisible.value = false;
    Toast.success("标签图标已保存");
  }
}

// ---------- 标签合并操作 ----------
function toggleTagSelect(name: string) {
  if (selectedTags.value.has(name)) {
    selectedTags.value.delete(name);
  } else {
    selectedTags.value.add(name);
  }
}

function selectAllFilteredTags() {
  for (const t of filteredAndSortedTags.value) {
    selectedTags.value.add(t.name);
  }
}

function clearTagSelection() {
  selectedTags.value.clear();
  mergeTarget.value = "";
}

function toggleMergeMode() {
  mergeMode.value = !mergeMode.value;
  if (!mergeMode.value) {
    clearTagSelection();
  }
}

async function confirmMergeTags() {
  const sources = [...selectedTags.value];
  const target = mergeTarget.value.trim();
  if (!target) {
    Toast.warning("请选择或输入合并的目标标签名称");
    return;
  }
  if (sources.length === 0) {
    Toast.warning("请至少勾选一个来源标签");
    return;
  }

  Dialog.warning({
    title: `确定将 ${sources.length} 个标签合并为「${target}」？`,
    description: `已选来源：${sources.join("、")}。合并后文章的所有关联将统一替换为「${target}」。`,
    confirmType: "primary",
    onConfirm: async () => {
      merging.value = true;
      try {
        await taxonomyApi.mergeTags(sources, target);
        Toast.success("标签合并成功");
        clearTagSelection();
        mergeMode.value = false;
        await load();
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "合并失败");
      } finally {
        merging.value = false;
      }
    },
  });
}

onMounted(load);
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="分类与标签">
      <template #icon>
        <Icon icon="ri:price-tag-3-line" class="text-primary" />
      </template>
      <template #subtitle>管理文章的分类层级与标签系统，配置前台展示的专属配色与图标</template>
      <template #actions>
        <div class="apple-search min-w-[200px] max-w-[280px]">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input
            v-model="searchQuery"
            :placeholder="activeTab === 'categories' ? '搜索分类名称/路径...' : '搜索标签名称...'"
          />
          <button v-if="searchQuery" type="button" class="text-text-quaternary hover:text-foreground" @click="searchQuery = ''">
            <Icon icon="ri:close-line" />
          </button>
        </div>

        <VButton
          v-if="activeTab === 'categories'"
          type="primary"
          @click="openCreateCategory(0)"
        >
          <template #icon>
            <Icon icon="ri:add-line" class="h-full w-full" />
          </template>
          新建分类
        </VButton>

        <VButton
          v-else
          :type="mergeMode ? 'default' : 'primary'"
          @click="toggleMergeMode"
        >
          <template #icon>
            <Icon :icon="mergeMode ? 'ri:close-line' : 'ri:git-merge-line'" class="h-full w-full" />
          </template>
          {{ mergeMode ? "退出合并" : "合并标签" }}
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="loading" />

      <template v-else>
        <!-- 顶部概览数据卡片 -->
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
          <div class="stats-card">
            <div class="stats-icon text-primary bg-primary/10">
              <Icon icon="ri:folder-2-line" />
            </div>
            <div>
              <div class="stats-label">全部分类</div>
              <div class="stats-value">
                {{ stats.totalCats }}
                <span class="stats-sub">{{ stats.rootCats }} 顶级 / {{ stats.subCats }} 子级</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30B0C7] bg-[#30B0C7]/10">
              <Icon icon="ri:price-tag-3-line" />
            </div>
            <div>
              <div class="stats-label">全部标签</div>
              <div class="stats-value">
                {{ stats.totalTgs }}
                <span class="stats-sub">{{ stats.activeTgs }} 个已被引用</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
              <Icon icon="ri:article-line" />
            </div>
            <div>
              <div class="stats-label">分类关联总计</div>
              <div class="stats-value">
                {{ stats.totalPostRefs }}
                <span class="stats-sub">篇次归档文章</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
              <Icon icon="ri:palette-line" />
            </div>
            <div>
              <div class="stats-label">自定义展示</div>
              <div class="stats-value">
                {{ Object.keys(catColors).length + Object.keys(catIcons).length }}
                <span class="stats-sub">处配色与图标</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 胶囊分段控件与工具栏 -->
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="apple-segmented">
            <button
              type="button"
              :class="{ active: activeTab === 'categories' }"
              @click="activeTab = 'categories'"
            >
              <Icon icon="ri:folder-line" class="mr-1.5 text-sm inline-block align-sub" />
              分类 ({{ categories.length }})
            </button>
            <button
              type="button"
              :class="{ active: activeTab === 'tags' }"
              @click="activeTab = 'tags'"
            >
              <Icon icon="ri:hashtag" class="mr-1.5 text-sm inline-block align-sub" />
              标签 ({{ tags.length }})
            </button>
          </div>

          <!-- 标签专属视图切换 & 排序 -->
          <div v-if="activeTab === 'tags'" class="flex items-center gap-2">
            <div class="apple-segmented">
              <button
                type="button"
                :class="{ active: tagViewMode === 'cloud' }"
                title="胶囊云视图"
                @click="tagViewMode = 'cloud'"
              >
                <Icon icon="ri:apps-line" />
              </button>
              <button
                type="button"
                :class="{ active: tagViewMode === 'table' }"
                title="表格列表视图"
                @click="tagViewMode = 'table'"
              >
                <Icon icon="ri:list-check" />
              </button>
            </div>

            <button
              type="button"
              class="apple-sort-btn"
              title="按文章数排序"
              @click="toggleTagSort('count')"
            >
              <Icon icon="ri:arrow-up-down-line" />
              文章数 {{ tagSortBy === 'count' ? (tagSortOrder === 'desc' ? '↓' : '↑') : '' }}
            </button>
          </div>
        </div>

        <!-- ============ 分类视图 ============ -->
        <section v-if="activeTab === 'categories'" class="space-y-3">
          <VCard>
            <VEmpty
              v-if="flatVisibleCategories.length === 0"
              :title="searchQuery ? '未找到匹配的分类' : '暂无分类'"
              :message="searchQuery ? '换个关键词试试' : '发布文章时设置分类，或点击右上角新建分类'"
            >
              <template #actions>
                <VButton v-if="!searchQuery" size="sm" type="primary" @click="openCreateCategory(0)">
                  新建首个分类
                </VButton>
              </template>
            </VEmpty>

            <div v-else class="divide-y divide-border/60">
              <div
                v-for="(c, idx) in flatVisibleCategories"
                :key="c.id || c.name"
                class="category-row group relative flex items-center justify-between gap-3 px-4 py-3 transition-colors hover:bg-secondary/40"
                :style="{ paddingLeft: `${Math.max(16, c.depth * 28 + 16)}px` }"
              >
                <!-- 层级连线导轨指示 -->
                <div
                  v-if="c.depth > 0"
                  class="category-tree-line"
                  :style="{ left: `${c.depth * 28 - 6}px` }"
                ></div>

                <!-- 左侧：折叠展开 + 图标 + 名称与路径 -->
                <div class="flex min-w-0 flex-1 items-center gap-3">
                  <!-- 折叠展开按钮 -->
                  <button
                    v-if="c.hasChildren"
                    type="button"
                    class="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-text-quaternary transition-colors hover:bg-secondary hover:text-foreground"
                    @click.stop="toggleNodeCollapse(c.id)"
                  >
                    <Icon
                      :icon="c.id && collapsedNodeIds.has(c.id) ? 'ri:arrow-right-s-line' : 'ri:arrow-down-s-line'"
                      class="text-base"
                    />
                  </button>
                  <span v-else class="h-6 w-6 shrink-0"></span>

                  <!-- 图标块 -->
                  <div
                    :class="[
                      'flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px] text-foreground transition-transform group-hover:scale-105',
                      categoryIconGradients[idx % categoryIconGradients.length],
                    ]"
                  >
                    <img
                      v-if="isIconUrl(catIcons[categoryKey(c)])"
                      :src="catIcons[categoryKey(c)].trim()"
                      class="h-5 w-5 object-contain"
                      alt=""
                    />
                    <Icon
                      v-else
                      :icon="catIcons[categoryKey(c)]?.trim() || categoryDefaultIcons[idx % categoryDefaultIcons.length]"
                      class="text-lg"
                    />
                  </div>

                  <!-- 名称与层级面包屑 -->
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                      <span class="truncate text-[14px] font-semibold text-foreground">
                        {{ c.name }}
                      </span>
                      <span
                        v-if="c.path && c.path !== c.name"
                        class="hidden rounded bg-secondary/80 px-1.5 py-0.5 font-mono text-[11px] text-text-quaternary sm:inline-block"
                      >
                        {{ c.path }}
                      </span>
                    </div>
                  </div>
                </div>

                <!-- 中间：展示配置的颜色与图标标记 -->
                <div class="hidden items-center gap-2 md:flex">
                  <span
                    v-if="catColors[categoryKey(c)]"
                    class="inline-flex items-center gap-1.5 rounded-full border border-border bg-card px-2.5 py-0.5 text-xs text-text-secondary shadow-xs"
                  >
                    <span
                      class="h-2.5 w-2.5 rounded-full"
                      :style="{ background: catColors[categoryKey(c)] }"
                    ></span>
                    {{ catColors[categoryKey(c)] }}
                  </span>

                  <span
                    v-if="catIcons[categoryKey(c)]"
                    class="inline-flex items-center gap-1 rounded-full border border-border bg-card px-2.5 py-0.5 text-xs text-text-secondary shadow-xs"
                  >
                    <Icon icon="ri:shape-line" class="text-text-tertiary" />
                    {{ catIcons[categoryKey(c)] }}
                  </span>
                </div>

                <!-- 右侧：文章计数与操作组 -->
                <div class="flex shrink-0 items-center gap-2">
                  <span
                    class="inline-flex items-center rounded-full bg-secondary px-2.5 py-0.5 text-[11px] font-semibold tracking-wide text-text-tertiary"
                  >
                    {{ c.count }} 篇
                  </span>

                  <div class="flex items-center gap-1">
                    <button
                      type="button"
                      title="添加子分类"
                      class="row-action-btn"
                      @click="openCreateCategory(c.id || 0)"
                    >
                      <Icon icon="ri:node-tree" />
                    </button>
                    <button
                      type="button"
                      title="编辑分类属性"
                      class="row-action-btn"
                      @click="openEditCategory(c)"
                    >
                      <Icon icon="ri:pencil-line" />
                    </button>
                    <button
                      type="button"
                      title="删除分类"
                      class="row-action-btn hover:text-[var(--color-state-error)]"
                      @click="deleteCategory(c)"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </VCard>

          <p class="px-1 text-xs text-text-quaternary">
            分类支持多级树形层级管理。删除分类仅允许在无子分类且无文章引用的空节点进行，以保障数据安全。
          </p>
        </section>

        <!-- ============ 标签视图 ============ -->
        <section v-else class="space-y-4">
          <!-- 标签合并提示栏 -->
          <div
            v-if="mergeMode"
            class="flex items-center justify-between rounded-xl border border-primary/30 bg-primary/10 px-4 py-3 text-sm text-foreground"
          >
            <div class="flex items-center gap-2">
              <Icon icon="ri:information-line" class="text-primary text-base" />
              <span>请勾选需要合并的来源标签，然后在下方悬浮栏选择或输入目标标签。</span>
            </div>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="text-xs text-primary underline underline-offset-2 hover:opacity-80"
                @click="selectAllFilteredTags"
              >
                全选当前
              </button>
              <button
                type="button"
                class="text-xs text-text-quaternary hover:text-foreground"
                @click="clearTagSelection"
              >
                清空已选
              </button>
            </div>
          </div>

          <VEmpty
            v-if="filteredAndSortedTags.length === 0"
            :title="searchQuery ? '未找到匹配的标签' : '暂无标签'"
            :message="searchQuery ? '换个关键词试试' : '发布文章时输入标签，即可在此自动归档并管理'"
          />

          <!-- 模式 1：优雅胶囊云视图 -->
          <div
            v-else-if="tagViewMode === 'cloud'"
            class="flex flex-wrap items-center gap-2.5 rounded-2xl border border-border bg-card p-5"
          >
            <div
              v-for="t in filteredAndSortedTags"
              :key="t.name"
              :class="[
                'apple-tag-pill group',
                { 'selected-for-merge': mergeMode && selectedTags.has(t.name) },
              ]"
              @click="mergeMode ? toggleTagSelect(t.name) : undefined"
            >
              <!-- 合并模式多选框 -->
              <input
                v-if="mergeMode"
                type="checkbox"
                :checked="selectedTags.has(t.name)"
                class="h-3.5 w-3.5 rounded border-border-strong text-primary pointer-events-none"
              />

              <!-- 图标 -->
              <img
                v-if="isIconUrl(tagIcons[t.name])"
                :src="tagIcons[t.name].trim()"
                class="h-3.5 w-3.5 object-contain"
                alt=""
              />
              <Icon
                v-else-if="tagIcons[t.name]?.trim()"
                :icon="tagIcons[t.name].trim()"
                class="text-primary text-xs"
              />

              <!-- 标签名 -->
              <span class="tag-name">{{ t.name }}</span>

              <!-- 文章数 -->
              <span :class="['tag-count-badge', t.count >= 10 ? 'tag-count-hot' : '']">
                {{ t.count }}
              </span>

              <!-- 悬浮操作按钮 -->
              <span v-if="!mergeMode" class="tag-hover-actions">
                <button
                  type="button"
                  title="配置前台图标"
                  @click.stop="openTagIcon(t.name)"
                >
                  <Icon icon="ri:shape-line" />
                </button>
                <button
                  type="button"
                  title="重命名"
                  @click.stop="openRenameTag(t.name)"
                >
                  <Icon icon="ri:pencil-line" />
                </button>
                <button
                  type="button"
                  title="移除标签"
                  class="hover:text-[var(--color-state-error)]"
                  @click.stop="deleteTag(t.name)"
                >
                  <Icon icon="ri:delete-bin-line" />
                </button>
              </span>
            </div>
          </div>

          <!-- 模式 2：表格列表视图 -->
          <VCard v-else>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[640px] text-left">
                <thead>
                  <tr class="border-b border-border text-xs font-medium uppercase tracking-wide text-text-quaternary">
                    <th v-if="mergeMode" class="w-12 px-4 py-3 text-center">选择</th>
                    <th class="px-4 py-3">标签名称</th>
                    <th class="px-4 py-3">前台图标</th>
                    <th class="w-32 px-4 py-3 text-right">引用文章数</th>
                    <th class="w-32 px-4 py-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-border/60">
                  <tr
                    v-for="t in filteredAndSortedTags"
                    :key="t.name"
                    class="transition-colors hover:bg-secondary/40"
                  >
                    <td v-if="mergeMode" class="px-4 py-3 text-center">
                      <input
                        type="checkbox"
                        :checked="selectedTags.has(t.name)"
                        class="h-4 w-4 rounded border-border text-primary"
                        @change="toggleTagSelect(t.name)"
                      />
                    </td>
                    <td class="px-4 py-3">
                      <div class="flex items-center gap-2">
                        <Icon icon="ri:hashtag" class="text-text-quaternary" />
                        <span class="font-medium text-foreground text-sm">{{ t.name }}</span>
                      </div>
                    </td>
                    <td class="px-4 py-3 text-xs text-text-secondary">
                      <div v-if="tagIcons[t.name]" class="flex items-center gap-1.5">
                        <img
                          v-if="isIconUrl(tagIcons[t.name])"
                          :src="tagIcons[t.name].trim()"
                          class="h-4 w-4 object-contain"
                          alt=""
                        />
                        <Icon v-else :icon="tagIcons[t.name].trim()" class="text-primary text-sm" />
                        <span class="font-mono">{{ tagIcons[t.name] }}</span>
                      </div>
                      <span v-else class="text-text-quaternary">默认无图标</span>
                    </td>
                    <td class="px-4 py-3 text-right font-mono text-sm text-foreground">
                      <span class="inline-block rounded-md bg-secondary px-2 py-0.5 text-xs font-semibold text-text-tertiary">
                        {{ t.count }}
                      </span>
                    </td>
                    <td class="px-4 py-3 text-right">
                      <div class="flex justify-end gap-1">
                        <button
                          type="button"
                          class="row-action-btn"
                          title="设置图标"
                          @click="openTagIcon(t.name)"
                        >
                          <Icon icon="ri:shape-line" />
                        </button>
                        <button
                          type="button"
                          class="row-action-btn"
                          title="重命名"
                          @click="openRenameTag(t.name)"
                        >
                          <Icon icon="ri:pencil-line" />
                        </button>
                        <button
                          type="button"
                          class="row-action-btn hover:text-[var(--color-state-error)]"
                          title="删除标签"
                          @click="deleteTag(t.name)"
                        >
                          <Icon icon="ri:delete-bin-line" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </VCard>
        </section>
      </template>
    </div>

    <!-- ============ 统一分类新建 / 编辑弹窗 ============ -->
    <VModal
      v-model:visible="categoryModalVisible"
      :title="categoryModalMode === 'create' ? '新建分类' : '编辑分类'"
      :width="540"
    >
      <form class="space-y-4" @submit.prevent="saveCategoryModal">
        <!-- 基础信息 -->
        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">分类名称 *</label>
          <input
            v-model="categoryForm.name"
            class="apple-input"
            required
            placeholder="例如：前端开发"
            maxlength="40"
          />
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">父级分类</label>
          <select v-model.number="categoryForm.parent_id" class="apple-select w-full">
            <option :value="0">顶级分类（无父级）</option>
            <option
              v-for="c in availableParentCategories"
              :key="c.id"
              :value="c.id"
            >
              {{ c.path || c.name }}
            </option>
          </select>
        </div>

        <!-- 配色选择 -->
        <div>
          <div class="mb-2 flex items-center justify-between">
            <label class="text-xs font-medium text-text-tertiary">前台标记配色</label>
            <button
              v-if="categoryForm.color"
              type="button"
              class="text-xs text-text-quaternary hover:text-foreground"
              @click="categoryForm.color = ''"
            >
              清除颜色
            </button>
          </div>

          <!-- Apple 预设色调色板 -->
          <div class="mb-2.5 flex flex-wrap items-center gap-2">
            <button
              v-for="preset in APPLE_PRESET_COLORS"
              :key="preset.value"
              type="button"
              class="color-preset-dot"
              :class="{ active: categoryForm.color.toLowerCase() === preset.value.toLowerCase() }"
              :style="{ background: preset.value }"
              :title="preset.label"
              @click="categoryForm.color = preset.value"
            ></button>
          </div>

          <input
            v-model="categoryForm.color"
            class="apple-input font-mono text-xs"
            placeholder="#007AFF 或其他 Hex 色值，留空=不设置专属配色"
            maxlength="7"
          />
        </div>

        <!-- 图标配置 -->
        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">前台专属图标</label>
          <IconPicker
            v-model="categoryForm.icon"
            placeholder="icons.yml key、Iconify key 或图片 URL，留空=默认"
          />
        </div>

        <!-- 实时效果预览卡片 -->
        <div class="rounded-xl border border-border bg-secondary/50 p-3">
          <div class="mb-1.5 text-[11px] font-medium text-text-quaternary">前台卡片效果预览</div>
          <div class="flex items-center gap-3 rounded-lg border border-border/80 bg-card p-3 shadow-xs">
            <div
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] text-foreground avatar-gradient-1"
            >
              <img
                v-if="isIconUrl(categoryForm.icon)"
                :src="categoryForm.icon.trim()"
                class="h-5 w-5 object-contain"
                alt=""
              />
              <Icon
                v-else
                :icon="categoryForm.icon?.trim() || 'ri:folder-line'"
                class="text-xl"
              />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="text-sm font-semibold text-foreground">
                  {{ categoryForm.name || "示例分类名称" }}
                </span>
                <span
                  v-if="categoryForm.color?.trim()"
                  class="inline-flex items-center gap-1 rounded-full border border-border bg-secondary px-2 py-0.5 text-[10px] text-text-secondary"
                >
                  <span
                    class="h-2 w-2 rounded-full"
                    :style="{ background: categoryForm.color }"
                  ></span>
                  {{ categoryForm.color }}
                </span>
              </div>
              <div class="text-xs text-text-quaternary mt-0.5">文章归类展示样式</div>
            </div>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="categoryModalVisible = false">取消</VButton>
          <VButton type="primary" :loading="categorySaving" @click="saveCategoryModal">
            保存
          </VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 标签重命名弹窗 ============ -->
    <VModal v-model:visible="renameModalVisible" title="重命名标签" :width="440">
      <div>
        <label class="mb-1.5 block text-xs font-medium text-text-tertiary">
          将标签「{{ renameTargetTag }}」重命名为
        </label>
        <input
          v-model="renameValue"
          class="apple-input"
          placeholder="输入新的标签名称"
          @keyup.enter="confirmRenameTag"
        />
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="renameModalVisible = false">取消</VButton>
          <VButton type="primary" :loading="renaming" @click="confirmRenameTag">确认</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 标签图标设置弹窗 ============ -->
    <VModal v-model:visible="tagIconModalVisible" title="设置标签前台图标" :width="460">
      <div>
        <label class="mb-1.5 block text-xs font-medium text-text-tertiary">
          「{{ tagIconTarget }}」的展示图标（icons.yml key / Iconify key / 图片 URL）
        </label>
        <IconPicker v-model="tagIconValue" placeholder="例如：ri:hashtag 或图片链接" />
        <p class="mt-2 text-xs text-text-quaternary">
          设置后前台标签面包屑与标签索引页将优先展示此图标，留空则恢复默认。
        </p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="tagIconModalVisible = false">取消</VButton>
          <VButton type="primary" :loading="tagIconSaving" @click="confirmTagIcon">保存图标</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 底部悬浮合并操作栏 (VFloatingDock) ============ -->
    <VFloatingDock
      :visible="mergeMode && selectedTags.size > 0"
      :count="selectedTags.size"
      title="已选来源标签"
      @clear="clearTagSelection"
    >
      <div class="flex items-center gap-2">
        <Icon icon="ri:arrow-right-line" class="text-text-tertiary text-sm" />
        <input
          v-model="mergeTarget"
          class="apple-input h-8 w-44 text-xs font-medium"
          placeholder="合并到（目标标签名）"
          @keyup.enter="confirmMergeTags"
        />
        <VButton size="sm" type="primary" :loading="merging" @click="confirmMergeTags">
          确认合并
        </VButton>
      </div>
    </VFloatingDock>
  </div>
</template>

<style scoped>
/* 统计卡片 */
.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-xs);
}
.stats-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 8%, transparent), transparent);
  pointer-events: none;
}
.stats-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  font-size: 20px;
  flex-shrink: 0;
}
.stats-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-quaternary);
  text-transform: uppercase;
  letter-spacing: 0.2px;
}
.stats-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.2;
}
.stats-sub {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-tertiary);
  margin-top: 1px;
}

/* 分类树引导线 */
.category-row {
  position: relative;
}
.category-tree-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 16px;
  border-left: 1px dashed var(--color-border-strong);
  pointer-events: none;
}
.category-tree-line::after {
  content: "";
  position: absolute;
  top: 50%;
  left: 0;
  width: 12px;
  border-top: 1px dashed var(--color-border-strong);
}

/* 行内操作按钮 */
.row-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  background: transparent;
  border: 1px solid transparent;
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.row-action-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

/* 标签云胶囊 */
.apple-tag-pill {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--color-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-full);
  font-size: 13px;
  font-weight: 500;
  color: var(--color-foreground);
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
  user-select: none;
}
.apple-tag-pill:hover {
  border-color: color-mix(in srgb, var(--color-primary) 45%, transparent);
  background: color-mix(in srgb, var(--color-secondary) 85%, var(--color-primary));
  box-shadow: 0 4px 12px color-mix(in srgb, var(--color-primary) 15%, transparent);
  transform: translateY(-1px);
}
.apple-tag-pill.selected-for-merge {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-primary) 20%, transparent);
}

.tag-name {
  line-height: 1;
}

.tag-count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: var(--radius-full);
  background: color-mix(in srgb, var(--color-card) 90%, transparent);
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-tertiary);
}
.tag-count-hot {
  background: color-mix(in srgb, var(--color-primary) 20%, transparent);
  color: var(--color-primary);
}

/* 标签悬浮按钮组 */
.tag-hover-actions {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-left: 2px;
  opacity: 0;
  max-width: 0;
  overflow: hidden;
  transition: all var(--duration-fast) var(--ease-standard);
}
.apple-tag-pill:hover .tag-hover-actions {
  opacity: 1;
  max-width: 70px;
}
.tag-hover-actions button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: var(--radius-full);
  color: var(--color-text-tertiary);
  transition: all var(--duration-fast) var(--ease-standard);
}
.tag-hover-actions button:hover {
  background: var(--color-card);
  color: var(--color-foreground);
}

/* 调色板预设小圆点 */
.color-preset-dot {
  width: 24px;
  height: 24px;
  border-radius: var(--radius-full);
  border: 2px solid transparent;
  cursor: pointer;
  transition: transform var(--duration-fast) var(--ease-standard),
    box-shadow var(--duration-fast) var(--ease-standard);
}
.color-preset-dot:hover {
  transform: scale(1.15);
}
.color-preset-dot.active {
  border-color: #fff;
  box-shadow: 0 0 0 2px var(--color-primary);
  transform: scale(1.15);
}

/* 排序按钮 */
.apple-sort-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 12px;
  background: var(--color-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-full);
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.apple-sort-btn:hover {
  color: var(--color-foreground);
  border-color: var(--color-border-strong);
}

/* 移动端适配 */
@media (max-width: 640px) {
  .stats-card {
    padding: 10px 12px;
    gap: 8px;
  }
  .stats-icon {
    width: 32px;
    height: 32px;
    font-size: 16px;
  }
  .stats-value {
    font-size: 14px;
  }
  .category-row {
    padding: 10px 12px !important;
  }
}
</style>
