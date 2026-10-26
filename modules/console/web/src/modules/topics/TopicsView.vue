<script setup lang="ts">
import { useStellarForm } from "@/components/editor/useStellarForm";
import { collectionFields } from "@/components/editor/stellar-form-fields";
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { topicApi, type TopicItem } from "@/api";
import StellarContentConfig from "@/components/editor/StellarContentConfig.vue";
import AttachmentField from "@/components/AttachmentField.vue";
import IconPicker from "@/components/IconPicker.vue";
import { Dialog, Toast, VButton, VCard, VEmpty, VLoading, VModal, VPageHeader } from "@/components/ui";

const topics = ref<TopicItem[]>([]);
const loading = ref(false);
const loadFailed = ref(false);
const saving = ref(false);
const deletingId = ref<number | null>(null);
const search = ref("");
const modalVisible = ref(false);
const editingId = ref<number | null>(null);

// 视图模式：卡片网格 vs 数据表格
const viewMode = ref<"grid" | "table">("grid");
const sortBy = ref<"sort" | "post_count">("sort");
const sortOrder = ref<"desc" | "asc">("desc");

const form = ref({
  stellar_version: 2,
  stellar_config: "{}",
  blog_extensions: "{}",
  name: "",
  slug: "",
  description: "",
  icon: "",
  cover: "",
  sort: 0,
});

const stellarFormError = useStellarForm(() => form.value, collectionFields);
// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = topics.value;
  const totalPosts = all.reduce((sum, t) => sum + (t.post_count || 0), 0);
  const withCover = all.filter((t) => !!t.cover?.trim()).length;
  let maxTopic = all.length > 0 ? all[0] : null;
  for (const t of all) {
    if (!maxTopic || (t.post_count || 0) > (maxTopic.post_count || 0)) {
      maxTopic = t;
    }
  }

  return {
    total: all.length,
    totalPosts,
    withCover,
    maxName: maxTopic?.name || "-",
    maxCount: maxTopic?.post_count || 0,
  };
});

const filteredTopics = computed(() => {
  const query = search.value.trim().toLowerCase();
  let list = topics.value;
  if (query) {
    list = list.filter((topic) => topic.name.toLowerCase().includes(query) || topic.slug.toLowerCase().includes(query));
  }

  return [...list].sort((a, b) => {
    if (sortBy.value === "post_count") {
      const diff = (b.post_count || 0) - (a.post_count || 0);
      return sortOrder.value === "desc" ? diff : -diff;
    } else {
      const diff = (b.sort || 0) - (a.sort || 0);
      return sortOrder.value === "desc" ? diff : -diff;
    }
  });
});

function toggleSort(by: "sort" | "post_count") {
  if (sortBy.value === by) {
    sortOrder.value = sortOrder.value === "desc" ? "asc" : "desc";
  } else {
    sortBy.value = by;
    sortOrder.value = "desc";
  }
}

function resetForm(topic?: TopicItem) {
  editingId.value = topic?.id ?? null;
  form.value = {
    stellar_version: topic?.stellar_version ?? 2,
    stellar_config: topic?.stellar_config || "{}",
    blog_extensions: topic?.blog_extensions || "{}",
    name: topic?.name ?? "",
    slug: topic?.slug ?? "",
    description: topic?.description ?? "",
    icon: topic?.icon ?? "",
    cover: topic?.cover ?? "",
    sort: topic?.sort ?? 0,
  };
}

function openCreate() {
  resetForm();
  modalVisible.value = true;
}

function openEdit(topic: TopicItem) {
  resetForm(topic);
  modalVisible.value = true;
}

async function load() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const { data } = await topicApi.list();
    topics.value = data.items || [];
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

async function save() {
  const name = form.value.name.trim();
  if (!name) {
    Toast.warning("请输入专栏名称");
    return;
  }
  saving.value = true;
  const payload = {
    stellar_version: form.value.stellar_version,
    stellar_config: form.value.stellar_config,
    blog_extensions: form.value.blog_extensions,
    name,
    slug: form.value.slug.trim(),
    description: form.value.description.trim(),
    icon: form.value.icon.trim(),
    cover: form.value.cover.trim(),
    sort: Number.isFinite(form.value.sort) ? form.value.sort : 0,
  };
  try {
    if (editingId.value == null) {
      await topicApi.create(payload);
      Toast.success("专栏已创建");
    } else {
      await topicApi.update(editingId.value, payload);
      Toast.success("专栏已更新");
    }
    modalVisible.value = false;
    await load();
  } catch {
  } finally {
    saving.value = false;
  }
}

function remove(topic: TopicItem) {
  Dialog.warning({
    title: `确定删除专栏「${topic.name}」吗？`,
    description: topic.post_count
      ? `该专栏仍关联 ${topic.post_count} 篇文章，请先在文章编辑器中解除关联。`
      : "删除后不可恢复。",
    confirmType: "danger",
    onConfirm: async () => {
      deletingId.value = topic.id;
      try {
        await topicApi.delete(topic.id);
        Toast.success("专栏已删除");
        await load();
      } catch {
      } finally {
        deletingId.value = null;
      }
    },
  });
}

onMounted(load);
</script>

<template>
  <div class="min-h-full">
    <VPageHeader title="专栏">
      <template #icon>
        <Icon icon="ri:book-shelf-line" class="text-primary" />
      </template>
      <template #subtitle>维护文章聚合专栏与海报展示信息，支持自定义封面与前台图标</template>
      <template #actions>
        <div class="apple-search min-w-[200px] max-w-[280px]">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="search" aria-label="搜索专栏" placeholder="搜索专栏名称或 Slug..." />
          <button v-if="search" type="button" class="text-text-quaternary hover:text-foreground" @click="search = ''">
            <Icon icon="ri:close-line" />
          </button>
        </div>

        <VButton type="primary" @click="openCreate">
          <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
          新建专栏
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="loading" />

      <template v-else-if="loadFailed">
        <div class="flex flex-col items-center gap-3 py-16 text-center">
          <p class="text-sm text-text-tertiary">专栏列表加载失败</p>
          <VButton size="sm" @click="load">重新加载</VButton>
        </div>
      </template>

      <template v-else>
        <!-- 当前数据摘要 -->
        <div class="console-summary" role="group" aria-label="概览摘要">
          <div class="stats-card">
            <div>
              <div class="stats-label">全部专栏</div>
              <div class="stats-value">
                {{ stats.total }}
                <span class="stats-sub">个专题合辑</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div>
              <div class="stats-label">归属文章</div>
              <div class="stats-value">
                {{ stats.totalPosts }}
                <span class="stats-sub">篇收录文章</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div>
              <div class="stats-label">海报封面</div>
              <div class="stats-value">
                {{ stats.withCover }}
                <span class="stats-sub">个已设封面</span>
              </div>
            </div>
          </div>

          <div class="stats-card">
            <div>
              <div class="stats-label">文章最多的专栏</div>
              <div class="stats-value truncate max-w-[140px]" :title="stats.maxName">
                {{ stats.maxName }}
                <span class="stats-sub">{{ stats.maxCount }} 篇收录</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 视图切换与排序工具栏 -->
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <span class="text-sm font-semibold text-foreground">全部专栏 ({{ filteredTopics.length }})</span>
          </div>

          <div class="flex items-center gap-2">
            <button type="button" class="apple-sort-btn" title="按排序权重排列" @click="toggleSort('sort')">
              <Icon icon="ri:arrow-up-down-line" />
              排序权重 {{ sortBy === "sort" ? (sortOrder === "desc" ? "↓" : "↑") : "" }}
            </button>
            <button type="button" class="apple-sort-btn" title="按文章数排列" @click="toggleSort('post_count')">
              <Icon icon="ri:article-line" />
              文章数 {{ sortBy === "post_count" ? (sortOrder === "desc" ? "↓" : "↑") : "" }}
            </button>

            <!-- 视图模式切换 -->
            <div class="apple-segmented">
              <button
                type="button"
                :class="{ active: viewMode === 'grid' }"
                title="海报卡片视图"
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
          v-if="filteredTopics.length === 0"
          :title="search ? '未找到匹配的专栏' : '暂无专栏'"
          :message="search ? '换个名称或 Slug 试试' : '创建第一个专栏后，它会显示在这里'"
        >
          <template #actions>
            <VButton v-if="!search" size="sm" type="primary" @click="openCreate">新建首个专栏</VButton>
          </template>
        </VEmpty>

        <!-- ============ 模式 1：海报卡片视图 ============ -->
        <div v-else-if="viewMode === 'grid'" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="topic in filteredTopics"
            :key="topic.id"
            class="topic-hero-card group relative flex flex-col justify-between overflow-hidden rounded-2xl border border-border bg-card transition-colors duration-150 hover:border-border-strong"
          >
            <!-- 卡片海报封面区 -->
            <div class="relative aspect-video w-full overflow-hidden bg-secondary">
              <img
                v-if="topic.cover"
                :src="topic.cover"
                :alt="topic.name"
                class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
              />
              <div
                v-else
                class="flex h-full w-full items-center justify-center bg-secondary/80 text-4xl text-text-quaternary"
              >
                <Icon :icon="topic.icon || 'ri:book-shelf-line'" />
              </div>

              <!-- 顶部浮层：文章数 Badge + 排序值 -->
              <div class="absolute inset-x-3 top-3 flex items-center justify-between">
                <span class="rounded-full bg-black/60 px-2.5 py-0.5 text-xs font-semibold text-white">
                  {{ topic.post_count }} 篇收录
                </span>
                <span v-if="topic.sort" class="rounded bg-black/50 px-2 py-0.5 font-mono text-[10px] text-white/80">
                  权重 {{ topic.sort }}
                </span>
              </div>

              <!-- 底部遮罩渐变 -->
              <div
                class="absolute inset-0 bg-gradient-to-t from-background via-transparent to-transparent opacity-80"
              ></div>
            </div>

            <!-- 卡片内容区 -->
            <div class="flex flex-1 flex-col justify-between p-4 pt-2">
              <div>
                <div class="flex items-center gap-2">
                  <div
                    v-if="topic.icon"
                    class="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary text-sm"
                  >
                    <Icon :icon="topic.icon" />
                  </div>
                  <h3 class="truncate text-base font-bold text-foreground">{{ topic.name }}</h3>
                </div>

                <div class="mt-1 flex items-center gap-2 font-mono text-xs text-text-quaternary">
                  <span class="truncate">/topics/{{ topic.slug }}</span>
                </div>

                <p class="mt-2 line-clamp-2 text-xs leading-relaxed text-text-tertiary">
                  {{ topic.description || "未填写专栏描述" }}
                </p>
              </div>

              <!-- 底部操作按钮 -->
              <div class="mt-4 flex items-center justify-end gap-2 border-t border-border/50 pt-3">
                <VButton size="sm" @click="openEdit(topic)">
                  <template #icon><Icon icon="ri:pencil-line" /></template>
                  编辑
                </VButton>
                <VButton size="sm" type="danger" :loading="deletingId === topic.id" @click="remove(topic)">
                  <template #icon><Icon icon="ri:delete-bin-line" /></template>
                  删除
                </VButton>
              </div>
            </div>
          </div>
        </div>

        <!-- ============ 模式 2：表格列表视图 ============ -->
        <VCard v-else>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[800px] text-left">
              <thead>
                <tr class="border-b border-border text-xs font-medium text-text-quaternary">
                  <th class="w-20 px-4 py-3">封面</th>
                  <th class="px-4 py-3">专栏名称</th>
                  <th class="px-4 py-3">Slug 路径</th>
                  <th class="px-4 py-3">简介</th>
                  <th class="w-28 px-4 py-3 text-right">文章数</th>
                  <th class="w-24 px-4 py-3 text-right">排序权重</th>
                  <th class="w-28 px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-border/60">
                <tr v-for="topic in filteredTopics" :key="topic.id" class="transition-colors hover:bg-secondary/40">
                  <td class="px-4 py-3">
                    <div
                      class="flex h-12 w-16 items-center justify-center overflow-hidden rounded-lg border border-border bg-secondary"
                    >
                      <img v-if="topic.cover" :src="topic.cover" :alt="topic.name" class="h-full w-full object-cover" />
                      <Icon v-else :icon="topic.icon || 'ri:book-shelf-line'" class="text-xl text-text-tertiary" />
                    </div>
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex items-center gap-1.5">
                      <Icon v-if="topic.icon" :icon="topic.icon" class="text-primary text-sm shrink-0" />
                      <span class="font-semibold text-foreground text-sm truncate">{{ topic.name }}</span>
                    </div>
                  </td>
                  <td class="px-4 py-3 font-mono text-xs text-text-secondary">
                    {{ topic.slug }}
                  </td>
                  <td class="px-4 py-3 text-xs text-text-tertiary max-w-[280px]">
                    <span class="line-clamp-2">{{ topic.description || "未填写描述" }}</span>
                  </td>
                  <td class="px-4 py-3 text-right font-mono text-xs">
                    <span class="rounded bg-secondary px-2 py-0.5 font-bold text-foreground">
                      {{ topic.post_count }}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-right font-mono text-xs text-text-tertiary">
                    {{ topic.sort }}
                  </td>
                  <td class="px-4 py-3 text-right">
                    <div class="flex justify-end gap-1">
                      <button type="button" class="action-icon-btn" title="编辑专栏" @click="openEdit(topic)">
                        <Icon icon="ri:pencil-line" />
                      </button>
                      <button
                        type="button"
                        class="action-icon-btn hover:text-[var(--color-state-error)]"
                        title="删除专栏"
                        :disabled="deletingId === topic.id"
                        @click="remove(topic)"
                      >
                        <Icon
                          :icon="deletingId === topic.id ? 'ri:loader-4-line' : 'ri:delete-bin-line'"
                          :class="deletingId === topic.id && 'animate-spin'"
                        />
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

    <!-- ============ 新建 / 编辑专栏弹窗 ============ -->
    <VModal v-model:visible="modalVisible" :title="editingId == null ? '新建专栏' : '编辑专栏'" :width="580">
      <form class="space-y-4" @submit.prevent="save">
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label class="mb-1.5 block text-xs font-medium text-text-tertiary">专栏名称 *</label>
            <input v-model="form.name" class="apple-input" required placeholder="例如：前端深度探索" />
          </div>

          <div>
            <label class="mb-1.5 block text-xs font-medium text-text-tertiary">Slug 路径（可选）</label>
            <input v-model="form.slug" class="apple-input font-mono text-xs" placeholder="留空按名称自动生成" />
            <p class="apple-form-hint">已有文章或未发布草稿归属时不能修改 ID；请先解除所有归属。</p>
          </div>

          <div>
            <label class="mb-1.5 block text-xs font-medium text-text-tertiary">排序权重（大在前）</label>
            <input v-model.number="form.sort" class="apple-input font-mono text-xs" type="number" step="1" />
          </div>
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">专栏描述</label>
          <textarea
            v-model="form.description"
            class="apple-textarea"
            rows="3"
            placeholder="简短的专栏介绍，用于前台海报与详情展示"
          ></textarea>
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">专栏图标</label>
          <IconPicker v-model="form.icon" placeholder="选择 icons.yml 图标或图片 URL" />
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-text-tertiary">专栏封面海报</label>
          <AttachmentField v-model="form.cover" placeholder="输入图片 URL 或从附件库选择" show-thumb />
        </div>
        <StellarContentConfig
          :sync-error="stellarFormError"
          v-model:config="form.stellar_config"
          v-model:blog="form.blog_extensions"
          :version="form.stellar_version"
          profile="topic"
          collection
        />
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="modalVisible = false">取消</VButton>
          <VButton type="primary" :loading="saving" @click="save">保存</VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>

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
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.apple-sort-btn:hover {
  color: var(--color-foreground);
  border-color: var(--color-border-strong);
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
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-icon-btn:hover:not(:disabled) {
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

</style>
