<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useQueryClient } from "@tanstack/vue-query";
import { postApi, shareApi, taxonomyApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { usePagedList } from "@/composables/usePagedList";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEmpty,
  VLoading,
  VFloatingDock,
  VPagination,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";
import { formatRelative } from "@/lib/format";

interface PostItem {
  id?: number;
  slug: string;
  title: string;
  cover?: string;
  summary?: string;
  category?: string;
  tags?: string[];
  status: "published" | "draft" | "trashed";
  pin?: boolean;
  views?: number;
  created_at: string;
  updated_at: string;
}

const router = useRouter();
const route = useRoute();
const queryClient = useQueryClient();

const validStatuses = ["published", "draft", "trashed"];
function statusFromQuery(): string | undefined {
  const s = route.query.status;
  return typeof s === "string" && validStatuses.includes(s) ? s : undefined;
}

const selectedStatus = ref<string | undefined>(statusFromQuery());
const selectedSort = ref<string>("newest");
const selectedCategory = ref<string | undefined>(undefined);
const selectedTag = ref<string | undefined>(undefined);
const viewMode = ref<"grid" | "table">("grid");

const { page, size, keyword, items: rawItems, total, isLoading, refetch } = usePagedList<PostItem>({
  apiFn: (params: Record<string, unknown>) => postApi.list(params),
  keyFn: (params: Record<string, unknown>) => queryKeys.posts(params),
  keywordParam: "search",
  extraParams: () => {
    const params: Record<string, unknown> = {};
    if (selectedStatus.value) params.status = selectedStatus.value;
    if (selectedCategory.value) params.category = selectedCategory.value;
    if (selectedTag.value) params.tag = selectedTag.value;
    return params;
  },
});

watch(
  () => route.query.status,
  () => {
    const s = statusFromQuery();
    if (s && s !== selectedStatus.value) {
      selectedStatus.value = s;
      page.value = 1;
    }
  }
);

// 分类与标签选项
const categories = ref<string[]>([]);
const tags = ref<string[]>([]);

onMounted(async () => {
  try {
    const [cRes, tRes] = await Promise.all([taxonomyApi.categories(), taxonomyApi.tags()]);
    categories.value = (cRes.data.items || []).map((i: any) => i.name);
    tags.value = (tRes.data.items || []).map((i: any) => i.name);
  } catch {}
});

// 客户端排序
const posts = computed(() => {
  const items = [...rawItems.value];
  if (selectedSort.value === "newest") {
    items.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  } else if (selectedSort.value === "oldest") {
    items.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
  } else if (selectedSort.value === "updated") {
    items.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());
  }
  return items;
});

// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = posts.value;
  let pubCount = 0;
  let draftCount = 0;
  let pinCount = 0;
  for (const p of all) {
    if (p.status === "published") pubCount++;
    else if (p.status === "draft") draftCount++;
    if (p.pin) pinCount++;
  }
  return {
    total: total.value,
    pubCount,
    draftCount,
    pinCount,
  };
});

// 批量选择
const selectedSlugs = ref<Set<string>>(new Set());

function toggleSelect(slug: string) {
  if (selectedSlugs.value.has(slug)) selectedSlugs.value.delete(slug);
  else selectedSlugs.value.add(slug);
}

function selectAll() {
  for (const p of posts.value) selectedSlugs.value.add(p.slug);
}

function clearSelection() {
  selectedSlugs.value.clear();
}

// ---------- 操作流 ----------
function handleDelete(p: PostItem) {
  Dialog.warning({
    title: "确定将文章移入回收站吗？",
    description: `「${p.title}」将被移入回收站，可在回收站中随时恢复。`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await postApi.delete(p.slug);
        selectedSlugs.value.delete(p.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("已移入回收站");
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      }
    },
  });
}

function handleRestore(p: PostItem) {
  Dialog.warning({
    title: "确定恢复该文章吗？",
    description: `「${p.title}」将恢复至草稿/发布状态。`,
    onConfirm: async () => {
      try {
        await postApi.restore(p.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("文章已恢复");
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "恢复失败");
      }
    },
  });
}

function handlePurge(p: PostItem) {
  Dialog.warning({
    title: "确定彻底删除该文章吗？",
    description: `「${p.title}」将被永久物理删除，不可恢复！`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await postApi.purge(p.slug);
        selectedSlugs.value.delete(p.slug);
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("已彻底删除");
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "删除失败");
      }
    },
  });
}

async function handleBatchDelete() {
  const slugs = Array.from(selectedSlugs.value);
  if (!slugs.length) return;
  Dialog.warning({
    title: `确定将选中的 ${slugs.length} 篇文章移入回收站吗？`,
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await Promise.all(slugs.map((s) => postApi.delete(s)));
        clearSelection();
        await queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
        await refetch();
        Toast.success("批量操作成功");
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "批量删除失败");
      }
    },
  });
}

// ---------- 分享链接创建弹窗 ----------
const shareModalOpen = ref(false);
const shareTarget = ref<PostItem | null>(null);
const shareDuration = ref<"1" | "7" | "30" | "0">("7");
const shareCreating = ref(false);
const generatedShareUrl = ref("");

function openShareModal(p: PostItem) {
  shareTarget.value = p;
  shareDuration.value = "7";
  generatedShareUrl.value = "";
  shareModalOpen.value = true;
}

async function createShare() {
  if (!shareTarget.value) return;
  shareCreating.value = true;
  try {
    const days = Number(shareDuration.value);
    const expiresAt =
      days > 0
        ? new Date(Date.now() + days * 86400000).toISOString()
        : "";
    const { data } = await shareApi.create(shareTarget.value.slug, {
      share_type: shareTarget.value.status === "draft" ? "draft" : "published",
      expires_at: expiresAt,
    });
    generatedShareUrl.value = window.location.origin + data.url;
    Toast.success("分享链接已生成");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "生成分享链接失败");
  } finally {
    shareCreating.value = false;
  }
}

function copyShareUrl() {
  navigator.clipboard.writeText(generatedShareUrl.value).then(() => {
    Toast.success("已复制到剪贴板");
  });
}

function openPostPreview(slug: string) {
  window.open(`/${slug}`, "_blank");
}

function statusBadge(status: string) {
  switch (status) {
    case "published":
      return { cls: "bg-[#30D158]/10 text-[#30D158]", label: "已发布" };
    case "draft":
      return { cls: "bg-[#FF9500]/10 text-[#FF9500]", label: "草稿" };
    case "trashed":
      return { cls: "bg-[#FF453A]/10 text-[#FF453A]", label: "回收站" };
    default:
      return { cls: "bg-secondary text-text-tertiary", label: status };
  }
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="文章管理">
      <template #icon>
        <Icon icon="ri:article-line" class="text-primary" />
      </template>
      <template #subtitle>撰写与管理博客博文，支持分类标签筛选、版本追踪与临时分享链接</template>
      <template #actions>
        <div class="apple-search min-w-[200px] max-w-[280px]">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="keyword" placeholder="搜索文章标题或 Slug..." />
          <button v-if="keyword" type="button" class="text-text-quaternary hover:text-foreground" @click="keyword = ''">
            <Icon icon="ri:close-line" />
          </button>
        </div>

        <VButton :route="{ path: '/posts/shares' }">
          <template #icon><Icon icon="ri:share-forward-line" /></template>
          分享链接
        </VButton>

        <VButton type="primary" :route="{ path: '/posts/new' }">
          <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
          撰写文章
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
        <div class="stats-card">
          <div class="stats-icon text-primary bg-primary/10">
            <Icon icon="ri:article-line" />
          </div>
          <div>
            <div class="stats-label">全部文章</div>
            <div class="stats-value">{{ stats.total }} <span class="stats-sub">篇博文</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
            <Icon icon="ri:checkbox-circle-line" />
          </div>
          <div>
            <div class="stats-label">已公开发布</div>
            <div class="stats-value text-[#30D158]">{{ stats.pubCount }} <span class="stats-sub">篇在线查阅</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#FF9500] bg-[#FF9500]/10">
            <Icon icon="ri:draft-line" />
          </div>
          <div>
            <div class="stats-label">草稿箱</div>
            <div class="stats-value text-[#FF9500]">{{ stats.draftCount }} <span class="stats-sub">篇未发布</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
            <Icon icon="ri:pushpin-line" />
          </div>
          <div>
            <div class="stats-label">置顶推荐</div>
            <div class="stats-value">{{ stats.pinCount }} <span class="stats-sub">篇精选置顶</span></div>
          </div>
        </div>
      </div>

      <!-- 工具栏：状态分段 + 筛选 + 视图模式 -->
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-border bg-card p-3">
        <!-- 状态分段 -->
        <div class="apple-segmented">
          <button
            type="button"
            :class="{ active: selectedStatus === undefined }"
            @click="selectedStatus = undefined; page = 1"
          >
            全部
          </button>
          <button
            type="button"
            :class="{ active: selectedStatus === 'published' }"
            @click="selectedStatus = 'published'; page = 1"
          >
            已发布
          </button>
          <button
            type="button"
            :class="{ active: selectedStatus === 'draft' }"
            @click="selectedStatus = 'draft'; page = 1"
          >
            草稿
          </button>
          <button
            type="button"
            :class="{ active: selectedStatus === 'trashed' }"
            @click="selectedStatus = 'trashed'; page = 1"
          >
            回收站
          </button>
        </div>

        <!-- 筛选器与视图 -->
        <div class="flex flex-wrap items-center gap-2">
          <!-- 分类下拉 -->
          <select v-model="selectedCategory" class="apple-select text-xs h-8">
            <option :value="undefined">全部分类</option>
            <option v-for="c in categories" :key="c" :value="c">{{ c }}</option>
          </select>

          <!-- 标签下拉 -->
          <select v-model="selectedTag" class="apple-select text-xs h-8">
            <option :value="undefined">全部标签</option>
            <option v-for="t in tags" :key="t" :value="t">{{ t }}</option>
          </select>

          <!-- 排序 -->
          <select v-model="selectedSort" class="apple-select text-xs h-8">
            <option value="newest">最近创建</option>
            <option value="updated">最近更新</option>
            <option value="oldest">最早创建</option>
          </select>

          <button
            type="button"
            class="rounded-lg border border-border px-2.5 py-1 text-xs text-text-tertiary hover:bg-secondary hover:text-foreground"
            @click="selectedSlugs.size === posts.length ? clearSelection() : selectAll()"
          >
            {{ selectedSlugs.size === posts.length ? '取消全选' : '全选本页' }}
          </button>

          <div class="apple-segmented">
            <button
              type="button"
              :class="{ active: viewMode === 'grid' }"
              title="海报卡片视图"
              @click="viewMode = 'grid'"
            >
              <Icon icon="ri:layout-grid-line" />
            </button>
            <button
              type="button"
              :class="{ active: viewMode === 'table' }"
              title="表格视图"
              @click="viewMode = 'table'"
            >
              <Icon icon="ri:table-line" />
            </button>
          </div>
        </div>
      </div>

      <VLoading v-if="isLoading" />

      <VEmpty
        v-else-if="posts.length === 0"
        :title="keyword ? '未找到匹配的文章' : '暂无对应文章'"
        :message="keyword ? '换个搜索关键词试试' : '点击右上角「撰写文章」创作你的第一篇博文'"
      />

      <!-- ============ 模式 1：海报卡片网格 ============ -->
      <div
        v-else-if="viewMode === 'grid'"
        class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      >
        <div
          v-for="p in posts"
          :key="p.slug"
          class="group relative flex flex-col justify-between overflow-hidden rounded-2xl border border-border bg-card transition-all hover:border-border-strong sm:p-0"
          :class="{ 'border-primary ring-1 ring-primary bg-primary/5': selectedSlugs.has(p.slug) }"
        >
          <!-- 封面图 / 渐变区域 -->
          <div class="relative aspect-video w-full overflow-hidden bg-secondary">
            <img
              v-if="p.cover"
              :src="p.cover"
              alt=""
              loading="lazy"
              class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
            <div
              v-else
              class="avatar-gradient-3 flex h-full w-full items-center justify-center text-4xl text-white/30"
            >
              <Icon icon="ri:article-line" />
            </div>

            <div class="absolute inset-0 bg-gradient-to-t from-black/80 via-black/20 to-transparent"></div>

            <!-- 顶部多选与置顶徽章 -->
            <div class="absolute left-2.5 top-2.5 flex items-center gap-1.5" @click.stop>
              <input
                type="checkbox"
                :checked="selectedSlugs.has(p.slug)"
                class="h-4 w-4 rounded border-border text-primary cursor-pointer"
                @change="toggleSelect(p.slug)"
              />
              <span v-if="p.pin" class="rounded bg-primary px-1.5 py-0.5 text-[9px] font-bold text-white flex items-center gap-0.5">
                <Icon icon="ri:pushpin-fill" /> 置顶
              </span>
            </div>

            <!-- 右上角状态 -->
            <span
              class="absolute right-2.5 top-2.5 rounded-full px-2 py-0.5 text-[10px] font-medium backdrop-blur-md"
              :class="statusBadge(p.status).cls"
            >
              {{ statusBadge(p.status).label }}
            </span>

            <!-- 底部浮层标题 -->
            <div class="absolute bottom-2.5 left-3 right-3 text-white">
              <h3
                class="truncate text-sm font-bold transition-colors group-hover:text-primary cursor-pointer"
                :title="p.title"
                @click="router.push(`/posts/edit/${p.slug}`)"
              >
                {{ p.title }}
              </h3>
            </div>
          </div>

          <!-- 卡片正文信息 -->
          <div class="p-3.5 flex flex-col justify-between flex-1">
            <div>
              <p v-if="p.summary" class="line-clamp-2 text-xs text-text-tertiary leading-relaxed mb-2.5">
                {{ p.summary }}
              </p>

              <div class="flex flex-wrap items-center gap-1.5 mb-3">
                <span v-if="p.category" class="rounded bg-secondary px-2 py-0.5 text-[10px] text-text-secondary">
                  📁 {{ p.category }}
                </span>
                <span
                  v-for="t in (p.tags || []).slice(0, 2)"
                  :key="t"
                  class="rounded bg-secondary/60 px-1.5 py-0.5 text-[10px] text-text-quaternary"
                >
                  #{{ t }}
                </span>
              </div>
            </div>

            <!-- 底部时间与操作 -->
            <div class="flex items-center justify-between border-t border-border/60 pt-2.5 text-xs text-text-quaternary">
              <span>{{ formatRelative(p.created_at) }}</span>

              <div class="flex items-center gap-1">
                <button
                  type="button"
                  class="action-icon-btn"
                  title="生成临时分享链接"
                  @click="openShareModal(p)"
                >
                  <Icon icon="ri:share-forward-line" />
                </button>

                <button
                  v-if="p.status === 'published'"
                  type="button"
                  class="action-icon-btn"
                  title="前台查阅"
                  @click="openPostPreview(p.slug)"
                >
                  <Icon icon="ri:external-link-line" />
                </button>

                <button
                  v-if="p.status !== 'trashed'"
                  type="button"
                  class="action-icon-btn"
                  title="编辑博文"
                  @click="router.push(`/posts/edit/${p.slug}`)"
                >
                  <Icon icon="ri:edit-line" />
                </button>

                <button
                  v-if="p.status === 'trashed'"
                  type="button"
                  class="action-icon-btn text-primary"
                  title="恢复博文"
                  @click="handleRestore(p)"
                >
                  <Icon icon="ri:restart-line" />
                </button>

                <button
                  v-if="p.status !== 'trashed'"
                  type="button"
                  class="action-icon-btn hover:text-state-error"
                  title="移入回收站"
                  @click="handleDelete(p)"
                >
                  <Icon icon="ri:delete-bin-line" />
                </button>

                <button
                  v-if="p.status === 'trashed'"
                  type="button"
                  class="action-icon-btn text-state-error"
                  title="彻底删除"
                  @click="handlePurge(p)"
                >
                  <Icon icon="ri:delete-bin-7-line" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ============ 模式 2：表格列表视图 ============ -->
      <VCard v-else>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[800px] text-left">
            <thead>
              <tr class="border-b border-border text-xs font-medium uppercase tracking-wide text-text-quaternary">
                <th class="w-12 px-4 py-3 text-center">
                  <input
                    type="checkbox"
                    :checked="selectedSlugs.size > 0 && selectedSlugs.size === posts.length"
                    class="h-4 w-4 rounded border-border text-primary"
                    @change="selectedSlugs.size === posts.length ? clearSelection() : selectAll()"
                  />
                </th>
                <th class="px-4 py-3">文章标题</th>
                <th class="px-4 py-3">分类 / 标签</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3">更新时间</th>
                <th class="w-40 px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border/60">
              <tr
                v-for="p in posts"
                :key="p.slug"
                class="cursor-pointer transition-colors hover:bg-secondary/40"
                :class="{ 'bg-primary/5': selectedSlugs.has(p.slug) }"
                @click="router.push(`/posts/edit/${p.slug}`)"
              >
                <td class="px-4 py-3.5 text-center" @click.stop>
                  <input
                    type="checkbox"
                    :checked="selectedSlugs.has(p.slug)"
                    class="h-4 w-4 rounded border-border text-primary"
                    @change="toggleSelect(p.slug)"
                  />
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-3">
                    <img
                      v-if="p.cover"
                      :src="p.cover"
                      class="h-10 w-14 shrink-0 rounded-lg border border-border object-cover bg-secondary"
                      alt=""
                    />
                    <div class="min-w-0">
                      <div class="font-semibold text-foreground text-sm flex items-center gap-1.5">
                        <span class="truncate max-w-[320px]">{{ p.title }}</span>
                        <span v-if="p.pin" class="rounded bg-primary px-1.5 py-0.2 text-[9px] font-bold text-white">置顶</span>
                      </div>
                      <div class="text-xs font-mono text-text-quaternary">/{{ p.slug }}</div>
                    </div>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs text-text-secondary">
                  <div class="flex flex-wrap items-center gap-1">
                    <span v-if="p.category" class="rounded bg-secondary px-1.5 py-0.5 text-[10px]">
                      {{ p.category }}
                    </span>
                    <span v-for="t in (p.tags || []).slice(0, 2)" :key="t" class="text-text-quaternary text-[11px]">
                      #{{ t }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs">
                  <span
                    class="rounded-full px-2 py-0.5 text-[10px] font-medium"
                    :class="statusBadge(p.status).cls"
                  >
                    {{ statusBadge(p.status).label }}
                  </span>
                </td>
                <td class="px-4 py-3.5 text-xs text-text-quaternary">
                  {{ formatRelative(p.updated_at) }}
                </td>
                <td class="px-4 py-3.5 text-right" @click.stop>
                  <div class="flex items-center justify-end gap-1">
                    <button
                      type="button"
                      class="action-icon-btn"
                      title="分享链接"
                      @click="openShareModal(p)"
                    >
                      <Icon icon="ri:share-forward-line" />
                    </button>
                    <button
                      v-if="p.status === 'published'"
                      type="button"
                      class="action-icon-btn"
                      title="前台查阅"
                      @click="openPostPreview(p.slug)"
                    >
                      <Icon icon="ri:external-link-line" />
                    </button>
                    <button
                      v-if="p.status !== 'trashed'"
                      type="button"
                      class="action-icon-btn"
                      title="编辑"
                      @click="router.push(`/posts/edit/${p.slug}`)"
                    >
                      <Icon icon="ri:edit-line" />
                    </button>
                    <button
                      v-if="p.status === 'trashed'"
                      type="button"
                      class="action-icon-btn text-primary"
                      title="恢复"
                      @click="handleRestore(p)"
                    >
                      <Icon icon="ri:restart-line" />
                    </button>
                    <button
                      v-if="p.status !== 'trashed'"
                      type="button"
                      class="action-icon-btn hover:text-state-error"
                      title="移入回收站"
                      @click="handleDelete(p)"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                    <button
                      v-if="p.status === 'trashed'"
                      type="button"
                      class="action-icon-btn text-state-error"
                      title="彻底删除"
                      @click="handlePurge(p)"
                    >
                      <Icon icon="ri:delete-bin-7-line" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </VCard>

      <!-- 分页 -->
      <div v-if="total > size" class="flex justify-center pt-4">
        <VPagination
          :page="page"
          :size="size"
          :total="total"
          @update:page="page = $event"
          @update:size="size = $event"
        />
      </div>
    </div>

    <!-- ============ 底部批量操作浮动栏 ============ -->
    <VFloatingDock
      :visible="selectedSlugs.size > 0"
      :count="selectedSlugs.size"
      title="已选文章"
      @clear="clearSelection"
    >
      <div class="flex items-center gap-2">
        <VButton size="sm" type="danger" @click="handleBatchDelete">
          批量移入回收站
        </VButton>
      </div>
    </VFloatingDock>

    <!-- ============ 生成分享链接弹窗 ============ -->
    <VModal
      v-model:visible="shareModalOpen"
      title="生成临时分享链接"
      :width="460"
    >
      <div class="space-y-4 text-xs">
        <div class="rounded-xl border border-border bg-secondary/40 p-3">
          <div class="text-text-quaternary">目标文章：</div>
          <div class="font-bold text-foreground text-sm mt-0.5">{{ shareTarget?.title }}</div>
        </div>

        <div>
          <label class="mb-1.5 block font-medium text-text-tertiary">链接有效期</label>
          <select v-model="shareDuration" class="apple-select w-full">
            <option value="1">1 天有效</option>
            <option value="7">7 天有效</option>
            <option value="30">30 天有效</option>
            <option value="0">永久有效</option>
          </select>
        </div>

        <div v-if="generatedShareUrl" class="space-y-1.5">
          <label class="block font-medium text-text-tertiary">已生成的分享直链</label>
          <div class="flex items-center gap-2">
            <input :value="generatedShareUrl" readonly class="apple-input font-mono text-xs" />
            <VButton size="sm" @click="copyShareUrl">复制</VButton>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="shareModalOpen = false">关闭</VButton>
          <VButton
            v-if="!generatedShareUrl"
            type="primary"
            :loading="shareCreating"
            @click="createShare"
          >
            生成链接
          </VButton>
        </div>
      </template>
    </VModal>
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
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-icon-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

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
}
</style>
