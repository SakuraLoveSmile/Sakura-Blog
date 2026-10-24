<script setup lang="ts">
import { ref, computed, reactive, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { queryKeys } from "@/api/query-client";
import { linkApi } from "@/api";
import FriendSettings from "./FriendSettings.vue";
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import { VPageHeader, VButton, VCard, VEmpty, VLoading, VModal, VFloatingDock, Dialog, Toast } from "@/components/ui";

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
  snapshot: string;
  last_status: number;
  last_checked_at: number;
}

const queryClient = useQueryClient();
const route = useRoute();
const router = useRouter();
const activeTab = ref(route.query.tab === "settings" ? "settings" : "list");

watch(activeTab, (tab) => {
  const query = { ...route.query };
  if (tab === "list") delete query.tab;
  else query.tab = tab;
  if (route.query.tab !== query.tab) void router.push({ query });
});
watch(() => route.query.tab, (tab) => {
  activeTab.value = tab === "settings" ? "settings" : "list";
});

const { data: queryData, isLoading } = useQuery({
  queryKey: queryKeys.friends(),
  queryFn: () => linkApi.list().then(({ data }) => data),
});

const links = computed<LinkRow[]>(() => queryData.value?.items || []);

// 视图切换与搜索
const viewMode = ref<"grid" | "table">("grid");
const searchQuery = ref("");
const statusFilter = ref<"all" | "ok" | "error">("all");

// 批量选择
const selectedIds = ref<Set<number>>(new Set());
const batchMoving = ref(false);
const batchTargetGroup = ref("");

// 友链检测
const checking = ref(false);

// Apple 8 款精选预设色板
const APPLE_PRESET_COLORS = [
  { label: "Apple 蓝", value: "007AFF" },
  { label: "极光紫", value: "AF52DE" },
  { label: "深邃靛", value: "5856D6" },
  { label: "湖泊青", value: "30B0C7" },
  { label: "薄荷绿", value: "00C7BE" },
  { label: "荧光绿", value: "30D158" },
  { label: "琥珀橙", value: "FF9500" },
  { label: "珊瑚红", value: "FF453A" },
];

// 编辑/新建弹窗状态
const showModal = ref(false);
const saving = ref(false);
const error = ref("");
const editing = reactive<Partial<LinkRow>>({
  group_title: "我的朋友",
  name: "",
  url: "",
  avatar: "",
  description: "",
  rss: "",
  label: "",
  color: "",
  sort: 0,
});
const snapshotting = ref(false);

// 导入弹窗状态
const showImportModal = ref(false);
const importSource = ref<"github" | "text">("github");
const importGroup = ref("GitHub 友链");
const importContent = ref("");
const importing = ref(false);
const importError = ref("");

const importGroupDefaults: Record<"github" | "text", string> = {
  github: "GitHub 友链",
  text: "导入友链",
};

// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = links.value;
  let okN = 0;
  let errN = 0;
  let rssN = 0;
  for (const l of all) {
    const s = linkState(l);
    if (s === "ok") okN++;
    else if (s === "error") errN++;
    if (l.rss) rssN++;
  }
  return {
    total: all.length,
    ok: okN,
    error: errN,
    rss: rssN,
  };
});

// 所有现有分组名称列表
const existingGroups = computed(() => {
  const set = new Set<string>();
  for (const l of links.value) {
    if (l.group_title?.trim()) set.add(l.group_title.trim());
  }
  return Array.from(set);
});

// 过滤后的友链
const filteredLinks = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  return links.value.filter((l) => {
    if (statusFilter.value !== "all" && linkState(l) !== statusFilter.value) return false;
    if (!q) return true;
    return (
      l.name.toLowerCase().includes(q) ||
      l.url.toLowerCase().includes(q) ||
      (l.group_title || "").toLowerCase().includes(q) ||
      (l.description || "").toLowerCase().includes(q)
    );
  });
});

// 按分组聚类
const grouped = computed(() => {
  const map: Record<string, LinkRow[]> = {};
  const order: string[] = [];
  for (const l of filteredLinks.value) {
    const g = l.group_title || "未分组";
    if (!map[g]) {
      map[g] = [];
      order.push(g);
    }
    map[g].push(l);
  }
  return order.map((title) => ({ title, items: map[title] }));
});

// ---------- 状态辅助函数 ----------
function linkState(l: LinkRow): "ok" | "error" | "unknown" {
  if (!l.last_checked_at) return "unknown";
  return l.last_status >= 200 && l.last_status < 400 ? "ok" : "error";
}

function errorReason(l: LinkRow): string {
  return l.last_status > 0 ? `HTTP ${l.last_status}` : "无法访问";
}

function checkedAtLabel(ts: number): string {
  if (!ts) return "";
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "short", timeStyle: "short" }).format(new Date(ts * 1000));
}

function normalizeColor(hex?: string): string {
  if (!hex) return "var(--color-primary)";
  const clean = hex.replace(/^#/, "");
  return `#${clean}`;
}

// ---------- 操作方法 ----------
async function runCheckAll() {
  checking.value = true;
  try {
    const { data } = await linkApi.check();
    let okN = 0;
    let errN = 0;
    for (const r of data.items || []) {
      r.ok ? okN++ : errN++;
    }
    const tagged: number = data.tagged ?? 0;
    const restored: number = data.restored ?? 0;
    let desc = `正常 ${okN} · 异常 ${errN}`;
    if (tagged > 0 || restored > 0) {
      desc += ` · 已标记失效 ${tagged} · 恢复 ${restored}`;
    }
    await queryClient.invalidateQueries({ queryKey: queryKeys.friends() });
    Toast.success("检测完成", { description: desc });
  } catch {
  } finally {
    checking.value = false;
  }
}

function openCreate(defaultGroup = "我的朋友") {
  Object.assign(editing, {
    id: undefined,
    group_title: defaultGroup,
    name: "",
    url: "",
    avatar: "",
    description: "",
    rss: "",
    label: "",
    color: "",
    sort: 0,
    snapshot: "",
  });
  error.value = "";
  showModal.value = true;
}

function openEdit(l: LinkRow) {
  Object.assign(editing, { ...l });
  error.value = "";
  showModal.value = true;
}

async function handleSave() {
  if (!editing.name?.trim()) {
    Toast.warning("请输入网站名称");
    return;
  }
  if (!editing.url?.trim()) {
    Toast.warning("请输入网站链接");
    return;
  }

  saving.value = true;
  error.value = "";
  try {
    const payload = {
      ...editing,
      color: (editing.color || "").replace(/^#/, ""),
    };
    if (editing.id) {
      await linkApi.update(editing.id, payload);
      Toast.success("友链已更新");
    } else {
      await linkApi.create(payload);
      Toast.success("友链创建成功");
    }
    showModal.value = false;
    queryClient.invalidateQueries({ queryKey: queryKeys.friends() });
  } catch {
    error.value = "保存失败";
  } finally {
    saving.value = false;
  }
}

function handleDelete(l: LinkRow) {
  Dialog.warning({
    title: `确定删除友链「${l.name}」？`,
    description: "删除后不可恢复，请确认。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await linkApi.delete(l.id);
        selectedIds.value.delete(l.id);
        queryClient.invalidateQueries({ queryKey: queryKeys.friends() });
        Toast.success("已删除");
      } catch {}
    },
  });
}

async function takeSnapshot() {
  if (!editing.id) return;
  snapshotting.value = true;
  error.value = "";
  try {
    const { data } = await linkApi.snapshot(editing.id);
    Object.assign(editing, data);
    queryClient.invalidateQueries({ queryKey: queryKeys.friends() });
    Toast.success("截图已更新");
  } catch {
    error.value = "截图失败，请检查截图服务配置";
  } finally {
    snapshotting.value = false;
  }
}

// ---------- 导入 ----------
function openImport() {
  importSource.value = "github";
  importGroup.value = importGroupDefaults.github;
  importContent.value = "";
  importError.value = "";
  showImportModal.value = true;
}

function switchImportSource(src: "github" | "text") {
  if (importSource.value === src) return;
  if (importGroup.value === importGroupDefaults[importSource.value]) {
    importGroup.value = importGroupDefaults[src];
  }
  importSource.value = src;
  importError.value = "";
}

async function handleImport() {
  importing.value = true;
  importError.value = "";
  try {
    const { data } = await linkApi.importLinks({
      source: importSource.value,
      group: importGroup.value.trim(),
      content: importSource.value === "text" ? importContent.value : undefined,
    });
    const failed = data?.failed || [];
    const msg = `导入 ${data.imported ?? 0} 条 · 跳过 ${data.skipped ?? 0} 条`;
    if (failed.length > 0) {
      Toast.error("部分导入失败", {
        description: `${msg}，失败 ${failed.length} 条（${failed.slice(0, 3).join("；")}${failed.length > 3 ? "…" : ""}）`,
      });
    } else {
      Toast.success("导入完成", { description: msg });
    }
    showImportModal.value = false;
    queryClient.invalidateQueries({ queryKey: queryKeys.friends() });
  } catch {
    importError.value = "导入失败，请稍后重试";
  } finally {
    importing.value = false;
  }
}

// ---------- 批量操作 ----------
function toggleSelect(id: number) {
  if (selectedIds.value.has(id)) selectedIds.value.delete(id);
  else selectedIds.value.add(id);
}

function selectAll() {
  for (const l of filteredLinks.value) selectedIds.value.add(l.id);
}

function clearSelection() {
  selectedIds.value.clear();
  batchTargetGroup.value = "";
}

// 搜索、筛选或分页使旧选择失效时清除失效选择；加载中沿用上一批数据，不伪造空态
watch(
  () => (isLoading.value ? null : filteredLinks.value.map((l) => l.id)),
  (ids) => {
    if (!ids) return;
    const visible = new Set(ids);
    for (const id of Array.from(selectedIds.value)) {
      if (!visible.has(id)) selectedIds.value.delete(id);
    }
    if (selectedIds.value.size === 0) batchTargetGroup.value = "";
  },
);

async function batchDelete() {
  const ids = Array.from(selectedIds.value);
  if (ids.length === 0) return;
  Dialog.warning({
    title: `确定批量删除选中的 ${ids.length} 条友链？`,
    description: "删除后不可恢复。",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await Promise.all(ids.map((id) => linkApi.delete(id)));
        clearSelection();
        queryClient.invalidateQueries({ queryKey: queryKeys.friends() });
        Toast.success("批量删除成功");
      } catch {}
    },
  });
}

async function batchMoveGroup() {
  const target = batchTargetGroup.value.trim();
  if (!target) {
    Toast.warning("请输入或选择目标分组名称");
    return;
  }
  const ids = Array.from(selectedIds.value);
  batchMoving.value = true;
  try {
    await Promise.all(
      ids.map((id) => {
        const item = links.value.find((l) => l.id === id);
        if (!item) return Promise.resolve();
        return linkApi.update(id, { ...item, group_title: target });
      }),
    );
    clearSelection();
    queryClient.invalidateQueries({ queryKey: queryKeys.friends() });
    Toast.success(`已将 ${ids.length} 条友链迁移至「${target}」`);
  } catch {
  } finally {
    batchMoving.value = false;
  }
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="友链">
      <template #icon>
        <Icon icon="ri:links-line" class="text-primary" />
      </template>
      <template #subtitle>维护博客友链与外部网站，支持健康度自动检测、网站截图与 RSS 订阅</template>
      <template #actions>
        <template v-if="activeTab === 'list'">
        <div class="apple-search min-w-[200px] max-w-[260px]">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="searchQuery" placeholder="搜索友链名称/URL/分组..." />
          <button
            v-if="searchQuery"
            type="button"
            class="text-text-quaternary hover:text-foreground"
            @click="searchQuery = ''"
          >
            <Icon icon="ri:close-line" />
          </button>
        </div>

        <router-link
          to="/subscriptions"
          class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary transition-colors hover:bg-secondary hover:text-foreground"
        >
          <Icon icon="ri:rss-line" class="text-[#FF9500]" />
          订阅源
        </router-link>

        <VButton @click="openImport">
          <template #icon>
            <Icon icon="ri:import-line" class="h-full w-full" />
          </template>
          导入友链
        </VButton>

        <VButton type="primary" @click="openCreate('我的朋友')">
          <template #icon>
            <Icon icon="ri:add-line" class="h-full w-full" />
          </template>
          新增友链
        </VButton>
        </template>
      </template>
    </VPageHeader>

    <nav class="friends-tabs apple-segmented rect" aria-label="友链分类">
      <button type="button" :class="{ active: activeTab === 'list' }" @click="activeTab = 'list'">列表</button>
      <button type="button" :class="{ active: activeTab === 'settings' }" @click="activeTab = 'settings'">设置</button>
    </nav>

    <div v-if="activeTab === 'list'" class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="isLoading" />

      <template v-else>
        <!-- 概览数据指标卡 -->
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
          <div class="stats-card">
            <div class="stats-icon text-primary bg-primary/10">
              <Icon icon="ri:links-line" />
            </div>
            <div>
              <div class="stats-label">全部友链</div>
              <div class="stats-value">
                {{ stats.total }}
                <span class="stats-sub">{{ existingGroups.length }} 个展示分组</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
              <Icon icon="ri:checkbox-circle-line" />
            </div>
            <div>
              <div class="stats-label">访问正常</div>
              <div class="stats-value text-[#30D158]">
                {{ stats.ok }}
                <span class="stats-sub">HTTP 2xx ~ 3xx</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#FF453A] bg-[#FF453A]/10">
              <Icon icon="ri:error-warning-line" />
            </div>
            <div>
              <div class="stats-label">访问异常</div>
              <div class="stats-value text-[#FF453A]">
                {{ stats.error }}
                <span class="stats-sub">不可达或超时</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#FF9500] bg-[#FF9500]/10">
              <Icon icon="ri:rss-line" />
            </div>
            <div>
              <div class="stats-label">RSS 订阅</div>
              <div class="stats-value">
                {{ stats.rss }}
                <span class="stats-sub">条支持动态同步</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 工具栏 -->
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-3">
            <!-- 状态过滤胶囊 -->
            <div class="apple-segmented">
              <button type="button" :class="{ active: statusFilter === 'all' }" @click="statusFilter = 'all'">
                全部 ({{ links.length }})
              </button>
              <button type="button" :class="{ active: statusFilter === 'ok' }" @click="statusFilter = 'ok'">
                正常 ({{ stats.ok }})
              </button>
              <button type="button" :class="{ active: statusFilter === 'error' }" @click="statusFilter = 'error'">
                异常 ({{ stats.error }})
              </button>
            </div>

            <VButton size="sm" :loading="checking" @click="runCheckAll">
              <template #icon>
                <Icon :icon="checking ? 'ri:loader-4-line' : 'ri:pulse-line'" :class="checking && 'animate-spin'" />
              </template>
              检测全部
            </VButton>
          </div>

          <div class="flex items-center gap-2">
            <!-- 视图模式切换 -->
            <div class="apple-segmented">
              <button
                type="button"
                :class="{ active: viewMode === 'grid' }"
                title="卡片网格视图"
                @click="viewMode = 'grid'"
              >
                <Icon icon="ri:apps-line" />
              </button>
              <button
                type="button"
                :class="{ active: viewMode === 'table' }"
                title="表格列表视图"
                @click="viewMode = 'table'"
              >
                <Icon icon="ri:list-check" />
              </button>
            </div>
          </div>
        </div>

        <VEmpty
          v-if="filteredLinks.length === 0"
          :title="searchQuery ? '未找到匹配的友链' : '暂无友链数据'"
          :message="searchQuery ? '换个搜索关键词试试' : '点击右上角「新增友链」或「导入友链」快速添加'"
        >
          <template #actions>
            <VButton v-if="!searchQuery" size="sm" type="primary" @click="openCreate('我的朋友')">
              新增首条友链
            </VButton>
          </template>
        </VEmpty>

        <!-- ============ 模式 1：卡片网格分组视图 ============ -->
        <div v-else-if="viewMode === 'grid'" class="space-y-6">
          <div v-for="g in grouped" :key="g.title" class="rounded-2xl border border-border bg-card p-5">
            <div class="mb-4 flex items-center justify-between">
              <div class="flex items-center gap-2.5">
                <span class="text-[15px] font-semibold text-foreground">{{ g.title }}</span>
                <span class="rounded-full border border-border bg-secondary px-2.5 py-0.5 text-xs text-text-tertiary">
                  {{ g.items.length }} 条
                </span>
              </div>
              <button
                type="button"
                class="flex items-center gap-1 rounded-lg border border-transparent px-2.5 py-1 text-xs font-medium text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-foreground"
                @click="openCreate(g.title)"
              >
                <Icon icon="ri:add-line" />
                新增至本组
              </button>
            </div>

            <div class="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-3">
              <div
                v-for="l in g.items"
                :key="l.id"
                class="group relative flex flex-col justify-between rounded-xl border border-border bg-secondary/30 p-4 transition-colors duration-150  hover:border-border-strong hover:bg-secondary/70 hover:shadow-sm"
                :class="{ 'border-primary ring-1 ring-primary': selectedIds.has(l.id) }"
              >
                <!-- 卡片顶部：选择框 + 头像 + 网站名/Tag + 操作组 -->
                <div>
                  <div class="flex items-start justify-between gap-3">
                    <div class="flex items-center gap-3 min-w-0 flex-1">
                      <input
                        type="checkbox"
                        :checked="selectedIds.has(l.id)"
                        class="h-4 w-4 rounded border-border-strong text-primary"
                        @change="toggleSelect(l.id)"
                      />

                      <img
                        v-if="l.avatar"
                        :src="l.avatar"
                        loading="lazy"
                        class="h-10 w-10 shrink-0 rounded-full border border-border bg-secondary object-cover"
                        alt=""
                      />
                      <div
                        v-else
                        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-border bg-secondary text-base text-text-tertiary avatar-gradient-1"
                      >
                        {{ l.name ? l.name.charAt(0).toUpperCase() : "🔗" }}
                      </div>

                      <div class="min-w-0 flex-1">
                        <div class="flex items-center gap-1.5">
                          <span class="truncate text-sm font-semibold text-foreground">{{ l.name }}</span>
                          <span
                            v-if="l.label"
                            class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold text-white"
                            :style="{ backgroundColor: normalizeColor(l.color) }"
                          >
                            {{ l.label }}
                          </span>
                        </div>
                        <a
                          :href="l.url"
                          target="_blank"
                          class="block truncate text-xs text-text-quaternary transition-colors hover:text-primary"
                          :title="l.url"
                        >
                          {{ l.url }}
                        </a>
                      </div>
                    </div>

                    <!-- 行内快捷操作按钮组 -->
                    <div class="flex shrink-0 items-center gap-1 opacity-80 transition-opacity group-hover:opacity-100">
                      <button type="button" class="action-icon-btn" title="编辑" @click="openEdit(l)">
                        <Icon icon="ri:pencil-line" />
                      </button>
                      <button
                        type="button"
                        class="action-icon-btn hover:text-[var(--color-state-error)]"
                        title="删除"
                        @click="handleDelete(l)"
                      >
                        <Icon icon="ri:delete-bin-line" />
                      </button>
                    </div>
                  </div>

                  <!-- 描述文本 -->
                  <p class="mt-2.5 line-clamp-2 text-xs leading-relaxed text-text-tertiary">
                    {{ l.description || "暂无网站描述信息" }}
                  </p>
                </div>

                <!-- 卡片底部：状态标签 + 检测时间 + RSS + 外链 -->
                <div
                  class="mt-3.5 flex items-center justify-between border-t border-border/60 pt-2.5 text-xs text-text-quaternary"
                >
                  <div class="flex items-center gap-2">
                    <span
                      v-if="linkState(l) === 'ok'"
                      class="inline-flex items-center gap-1 rounded-full bg-[#30D158]/10 px-2 py-0.5 text-[11px] font-medium text-[#30D158]"
                    >
                      <span class="h-1.5 w-1.5 rounded-full bg-[#30D158]"></span>
                      正常
                    </span>
                    <span
                      v-else-if="linkState(l) === 'error'"
                      :title="errorReason(l)"
                      class="inline-flex items-center gap-1 rounded-full bg-[#FF453A]/10 px-2 py-0.5 text-[11px] font-medium text-[#FF453A]"
                    >
                      <span class="h-1.5 w-1.5 rounded-full bg-[#FF453A]"></span>
                      {{ errorReason(l) }}
                    </span>
                    <span
                      v-else
                      class="inline-flex items-center gap-1 rounded-full bg-secondary px-2 py-0.5 text-[11px] text-text-quaternary"
                    >
                      未检测
                    </span>

                    <span v-if="l.last_checked_at" class="text-[11px]">
                      {{ checkedAtLabel(l.last_checked_at) }}
                    </span>
                  </div>

                  <div class="flex items-center gap-2">
                    <span v-if="l.rss" class="inline-flex items-center gap-0.5 text-[#FF9500]" title="已配置 RSS">
                      <Icon icon="ri:rss-line" />
                    </span>
                    <a
                      :href="l.url"
                      target="_blank"
                      class="inline-flex items-center gap-0.5 hover:text-primary"
                      title="打开网站"
                    >
                      <Icon icon="ri:external-link-line" />
                    </a>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- ============ 模式 2：表格列表视图 ============ -->
        <VCard v-else>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[760px] text-left">
              <thead>
                <tr class="border-b border-border text-xs font-medium text-text-quaternary">
                  <th class="w-12 px-4 py-3 text-center">
                    <input
                      v-if="filteredLinks.length > 0"
                      type="checkbox"
                      :checked="selectedIds.size === filteredLinks.length"
                      class="h-4 w-4 rounded border-border text-primary disabled:cursor-not-allowed disabled:opacity-40"
                      @change="selectedIds.size === filteredLinks.length ? clearSelection() : selectAll()"
                    />
                  </th>
                  <th class="px-4 py-3">网站 / 链接</th>
                  <th class="px-4 py-3">分组</th>
                  <th class="px-4 py-3">标签</th>
                  <th class="px-4 py-3">健康状态</th>
                  <th class="w-20 px-4 py-3 text-right">排序</th>
                  <th class="w-28 px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-border/60">
                <tr
                  v-for="l in filteredLinks"
                  :key="l.id"
                  class="transition-colors hover:bg-secondary/40"
                  :class="{ 'bg-primary/5': selectedIds.has(l.id) }"
                >
                  <td class="px-4 py-3 text-center">
                    <input
                      type="checkbox"
                      :checked="selectedIds.has(l.id)"
                      class="h-4 w-4 rounded border-border text-primary"
                      @change="toggleSelect(l.id)"
                    />
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex items-center gap-3">
                      <img
                        v-if="l.avatar"
                        :src="l.avatar"
                        class="h-8 w-8 shrink-0 rounded-full border border-border object-cover"
                        alt=""
                      />
                      <div
                        v-else
                        class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-xs"
                      >
                        🔗
                      </div>
                      <div class="min-w-0">
                        <div class="font-semibold text-foreground text-sm truncate">{{ l.name }}</div>
                        <a
                          :href="l.url"
                          target="_blank"
                          class="text-xs text-text-quaternary hover:text-primary truncate block max-w-[280px]"
                        >
                          {{ l.url }}
                        </a>
                      </div>
                    </div>
                  </td>
                  <td class="px-4 py-3 text-xs text-text-secondary">
                    <span class="rounded bg-secondary px-2 py-0.5 font-medium">
                      {{ l.group_title || "未分组" }}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-xs">
                    <span
                      v-if="l.label"
                      class="rounded-full px-2 py-0.5 text-[10px] font-semibold text-white"
                      :style="{ backgroundColor: normalizeColor(l.color) }"
                    >
                      {{ l.label }}
                    </span>
                    <span v-else class="text-text-quaternary">-</span>
                  </td>
                  <td class="px-4 py-3 text-xs">
                    <div class="flex items-center gap-1.5">
                      <span
                        v-if="linkState(l) === 'ok'"
                        class="inline-flex items-center gap-1 text-[#30D158] font-medium"
                      >
                        <Icon icon="ri:checkbox-circle-fill" /> 正常
                      </span>
                      <span
                        v-else-if="linkState(l) === 'error'"
                        class="inline-flex items-center gap-1 text-[#FF453A] font-medium"
                        :title="errorReason(l)"
                      >
                        <Icon icon="ri:error-warning-fill" /> {{ errorReason(l) }}
                      </span>
                      <span v-else class="text-text-quaternary">未检测</span>
                    </div>
                  </td>
                  <td class="px-4 py-3 text-right font-mono text-xs text-text-tertiary">
                    {{ l.sort }}
                  </td>
                  <td class="px-4 py-3 text-right">
                    <div class="flex justify-end gap-1">
                      <button type="button" class="action-icon-btn" title="编辑" @click="openEdit(l)">
                        <Icon icon="ri:pencil-line" />
                      </button>
                      <button
                        type="button"
                        class="action-icon-btn hover:text-[var(--color-state-error)]"
                        title="删除"
                        @click="handleDelete(l)"
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
      </template>
    </div>
    <FriendSettings v-else />

    <!-- ============ 新增 / 编辑友链弹窗 ============ -->
    <VModal v-model:visible="showModal" :title="editing.id ? '编辑友链' : '新增友链'" :width="560">
      <form class="space-y-4" @submit.prevent="handleSave">
        <p v-if="error" class="rounded-lg bg-state-error/10 p-2.5 text-xs text-state-error">
          {{ error }}
        </p>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1.5 block text-xs font-medium text-text-tertiary">分组名称 *</label>
            <input
              v-model="editing.group_title"
              class="apple-input"
              list="group-options"
              placeholder="例如：我的朋友"
              required
            />
            <datalist id="group-options">
              <option v-for="g in existingGroups" :key="g" :value="g"></option>
            </datalist>
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-text-tertiary">网站名称 *</label>
            <input v-model="editing.name" class="apple-input" placeholder="例如：Stellar 博客" required />
          </div>
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">网站链接 *</label>
          <input v-model="editing.url" class="apple-input" placeholder="https://" required />
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">网站头像 / Favicon</label>
          <AttachmentField v-model="editing.avatar" placeholder="图片 URL，留空展示文字占位" show-thumb />
        </div>

        <!-- 配色与标签 -->
        <div class="rounded-xl border border-border bg-secondary/30 p-3.5">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-text-tertiary">角标文案（可选）</label>
              <input v-model="editing.label" class="apple-input" placeholder="如：推荐 / 技术" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-text-tertiary">排序权重（大在前）</label>
              <input v-model.number="editing.sort" type="number" class="apple-input font-mono" />
            </div>
          </div>

          <div class="mt-3">
            <div class="mb-2 flex items-center justify-between">
              <label class="text-xs font-medium text-text-tertiary">角标配色</label>
              <button
                v-if="editing.color"
                type="button"
                class="text-xs text-text-quaternary hover:text-foreground"
                @click="editing.color = ''"
              >
                清除颜色
              </button>
            </div>

            <!-- Apple 预设色板 -->
            <div class="flex flex-wrap items-center gap-2">
              <button
                v-for="preset in APPLE_PRESET_COLORS"
                :key="preset.value"
                type="button"
                class="color-preset-dot"
                :class="{ active: (editing.color || '').replace(/^#/, '').toLowerCase() === preset.value.toLowerCase(), }"
                :style="{ background: `#${preset.value}` }"
                :title="preset.label"
                @click="editing.color = preset.value"
              ></button>
              <input
                v-model="editing.color"
                class="apple-input ml-2 h-7 w-24 font-mono text-xs"
                placeholder="Hex 色值"
                maxlength="7"
              />
            </div>
          </div>
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">网站描述</label>
          <textarea
            v-model="editing.description"
            class="apple-textarea"
            rows="2"
            placeholder="简短的一句话介绍"
          ></textarea>
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">RSS / Atom 地址（可选）</label>
          <input v-model="editing.rss" class="apple-input" placeholder="https://example.com/atom.xml" />
        </div>

        <!-- 网站截图 -->
        <div v-if="editing.id" class="rounded-xl border border-border bg-secondary/30 p-3.5">
          <div class="mb-2 flex items-center justify-between">
            <label class="text-xs font-medium text-text-tertiary">网站快照截图</label>
            <VButton type="secondary" size="sm" :loading="snapshotting" @click="takeSnapshot">
              <template #icon><Icon icon="ri:camera-line" /></template>
              立即截图
            </VButton>
          </div>
          <img
            v-if="editing.snapshot"
            :src="editing.snapshot"
            class="mb-2 aspect-video w-full rounded-lg border border-border bg-card object-cover"
            alt="网站截图"
          />
          <p v-else class="mb-2 text-xs text-text-quaternary">暂无缓存截图</p>
          <AttachmentField v-model="editing.snapshot" placeholder="可手动指定截图 URL" show-thumb />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="showModal = false">取消</VButton>
          <VButton type="primary" :loading="saving" @click="handleSave">保存</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 导入友链弹窗 ============ -->
    <VModal v-model:visible="showImportModal" title="导入友链" :width="500">
      <div class="space-y-4">
        <!-- 来源切换 -->
        <div class="apple-segmented">
          <button type="button" :class="{ active: importSource === 'github' }" @click="switchImportSource('github')">
            <Icon icon="ri:github-fill" class="mr-1.5 inline-block" />
            GitHub Issues 仓库
          </button>
          <button type="button" :class="{ active: importSource === 'text' }" @click="switchImportSource('text')">
            <Icon icon="ri:clipboard-line" class="mr-1.5 inline-block" />
            粘贴 JSON / YAML
          </button>
        </div>

        <p v-if="importSource === 'github'" class="rounded-lg bg-secondary px-3 py-2.5 text-xs text-text-tertiary">
          将自动从 GitHub Issues 拉取全部友链申请记录，解析格式并批量写入，已存在的友链将自动跳过。
        </p>
        <p v-else class="rounded-lg bg-secondary px-3 py-2.5 text-xs text-text-tertiary">
          粘贴 JSON 数组或 YAML 列表（包含 name、url、avatar、description、rss 等字段），已存在的链接自动跳过。
        </p>

        <p v-if="importError" class="rounded-lg bg-state-error/10 p-2.5 text-xs text-state-error">
          {{ importError }}
        </p>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">导入至分组</label>
          <input v-model="importGroup" class="apple-input" :placeholder="importGroupDefaults[importSource]" />
        </div>

        <div v-if="importSource === 'text'">
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">友链内容</label>
          <textarea
            v-model="importContent"
            class="apple-textarea code-style font-mono text-xs"
            rows="6"
            placeholder='[{"name": "示例博客", "url": "https://example.com", "avatar": "https://example.com/avatar.png"}]'
          ></textarea>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="showImportModal = false">取消</VButton>
          <VButton type="primary" :loading="importing" @click="handleImport">开始导入</VButton>
        </div>
      </template>
    </VModal>

    <!-- ============ 底部批量操作栏 (VFloatingDock) ============ -->
    <VFloatingDock :visible="selectedIds.size > 0" :count="selectedIds.size" title="已选友链" @clear="clearSelection">
      <div class="flex items-center gap-2">
        <input
          v-model="batchTargetGroup"
          class="apple-input h-8 w-36 text-xs"
          placeholder="迁移至分组..."
          list="group-options"
        />
        <VButton size="sm" :loading="batchMoving" @click="batchMoveGroup"> 迁移分组 </VButton>
        <VButton size="sm" type="danger" @click="batchDelete"> 批量删除 </VButton>
      </div>
    </VFloatingDock>
  </div>
</template>

<style scoped>
.friends-tabs {
  display: flex;
  width: min(1400px, calc(100% - 32px));
  margin: 16px auto 0;
  overflow-x: auto;
  scrollbar-width: none;
}
.friends-tabs::-webkit-scrollbar { display: none; }
.friends-tabs button { min-height: 44px; white-space: nowrap; }

/* 操作小按钮 */
.action-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  background: transparent;
  border: 1px solid transparent;
  transition: background-color var(--duration-fast) var(--ease-standard), border-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-icon-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

/* 调色板预设小圆点 */
.color-preset-dot {
  width: 22px;
  height: 22px;
  border-radius: var(--radius-full);
  border: 2px solid transparent;
  cursor: pointer;
  transition:
    transform var(--duration-fast) var(--ease-standard),
    box-shadow var(--duration-fast) var(--ease-standard);
}
.color-preset-dot:hover {
  transform: scale(1.15);
}
.color-preset-dot.active {
  border-color: #fff;
  transform: scale(1.15);
}

/* 移动端适配 */
@media (max-width: 640px) {
}
</style>
