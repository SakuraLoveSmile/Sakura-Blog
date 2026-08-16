<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Icon } from "@iconify/vue";
import { topicApi, type TopicItem } from "@/api";
import AttachmentField from "@/components/AttachmentField.vue";
import IconPicker from "@/components/IconPicker.vue";
import {
  Dialog,
  Toast,
  VButton,
  VCard,
  VEmpty,
  VLoading,
  VModal,
  VPageHeader,
} from "@/components/ui";

const topics = ref<TopicItem[]>([]);
const loading = ref(false);
const loadFailed = ref(false);
const saving = ref(false);
const deletingId = ref<number | null>(null);
const search = ref("");
const modalVisible = ref(false);
const editingId = ref<number | null>(null);

const form = reactive({
  name: "",
  slug: "",
  description: "",
  icon: "",
  cover: "",
  sort: 0,
});

const filteredTopics = computed(() => {
  const query = search.value.trim().toLowerCase();
  if (!query) return topics.value;
  return topics.value.filter(
    (topic) => topic.name.toLowerCase().includes(query) || topic.slug.toLowerCase().includes(query)
  );
});

function resetForm(topic?: TopicItem) {
  editingId.value = topic?.id ?? null;
  form.name = topic?.name ?? "";
  form.slug = topic?.slug ?? "";
  form.description = topic?.description ?? "";
  form.icon = topic?.icon ?? "";
  form.cover = topic?.cover ?? "";
  form.sort = topic?.sort ?? 0;
}

function openCreate() {
  resetForm();
  modalVisible.value = true;
}

function openEdit(topic: TopicItem) {
  resetForm(topic);
  modalVisible.value = true;
}

function errorMessage(error: any, fallback: string) {
  return error?.response?.data?.error || error?.message || fallback;
}

async function load() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const { data } = await topicApi.list();
    topics.value = data.items || [];
  } catch (error: any) {
    loadFailed.value = true;
    Toast.error(errorMessage(error, "专栏加载失败"));
  } finally {
    loading.value = false;
  }
}

async function save() {
  const name = form.name.trim();
  if (!name) {
    Toast.warning("请输入专栏名称");
    return;
  }
  saving.value = true;
  const payload = {
    name,
    slug: form.slug.trim(),
    description: form.description.trim(),
    icon: form.icon.trim(),
    cover: form.cover.trim(),
    sort: Number.isFinite(form.sort) ? form.sort : 0,
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
  } catch (error: any) {
    // 保持弹窗打开，便于修正重复名称/Slug或网络错误。
    Toast.error(errorMessage(error, "保存失败"));
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
      } catch (error: any) {
        Toast.error(errorMessage(error, "删除失败"));
      } finally {
        deletingId.value = null;
      }
    },
  });
}

onMounted(load);
</script>

<template>
  <div class="min-w-0">
    <VPageHeader title="专栏">
      <template #icon>
        <Icon icon="ri:book-shelf-line" class="text-primary" />
      </template>
      <template #subtitle>维护文章归属与专栏展示信息</template>
      <template #actions>
        <VButton type="primary" @click="openCreate">
          <template #icon><Icon icon="ri:add-line" /></template>
          新建专栏
        </VButton>
      </template>
    </VPageHeader>

    <main class="topics-page mx-auto max-w-[1400px] space-y-4 p-4 sm:p-6">
      <div class="topics-toolbar flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card p-3">
        <div class="apple-search min-w-[220px] max-w-[360px] flex-1">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="search" aria-label="搜索专栏" placeholder="搜索名称或 Slug" />
          <button v-if="search" type="button" title="清除搜索" @click="search = ''">
            <Icon icon="ri:close-line" />
          </button>
        </div>
        <span class="text-sm text-text-tertiary">{{ filteredTopics.length }} 个专栏</span>
      </div>

      <VCard>
        <VLoading v-if="loading" />
        <div v-else-if="loadFailed" class="flex flex-col items-center gap-3 py-16">
          <p class="text-sm text-text-tertiary">专栏列表加载失败</p>
          <VButton size="sm" @click="load">重新加载</VButton>
        </div>
        <VEmpty
          v-else-if="filteredTopics.length === 0"
          :title="search ? '未找到匹配的专栏' : '暂无专栏'"
          :message="search ? '换个名称或 Slug 试试' : '创建第一个专栏后，它会显示在这里'"
        >
          <template #actions>
            <VButton v-if="!search" size="sm" type="primary" @click="openCreate">新建专栏</VButton>
          </template>
        </VEmpty>
        <div v-else class="topics-table-wrap overflow-x-auto">
          <table class="topics-table w-full min-w-[880px] text-left">
            <thead>
              <tr class="border-b border-border text-xs font-medium uppercase tracking-wide text-text-quaternary">
                <th class="w-20 px-4 py-3">封面</th>
                <th class="px-4 py-3">专栏</th>
                <th class="px-4 py-3">Slug</th>
                <th class="px-4 py-3">描述</th>
                <th class="w-24 px-4 py-3 text-right">文章数</th>
                <th class="w-20 px-4 py-3 text-right">排序</th>
                <th class="w-28 px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr v-for="topic in filteredTopics" :key="topic.id" class="transition-colors hover:bg-secondary/60">
                <td class="px-4 py-3">
                  <div class="topic-thumb flex h-12 w-16 items-center justify-center overflow-hidden rounded-lg border border-border bg-secondary">
                    <img v-if="topic.cover" :src="topic.cover" :alt="topic.name" class="h-full w-full object-cover" />
                    <Icon v-else :icon="topic.icon || 'ri:book-shelf-line'" class="text-xl text-text-tertiary" />
                  </div>
                </td>
                <td class="max-w-[220px] px-4 py-3">
                  <div class="truncate text-sm font-semibold text-foreground">{{ topic.name }}</div>
                  <div v-if="topic.icon" class="mt-1 flex items-center gap-1 text-xs text-text-quaternary">
                    <Icon icon="ri:shape-line" />{{ topic.icon }}
                  </div>
                </td>
                <td class="px-4 py-3 font-mono text-xs text-text-secondary">{{ topic.slug }}</td>
                <td class="max-w-[300px] px-4 py-3 text-sm text-text-tertiary">
                  <span class="line-clamp-2">{{ topic.description || "未填写描述" }}</span>
                </td>
                <td class="px-4 py-3 text-right font-mono text-sm text-foreground">{{ topic.post_count }}</td>
                <td class="px-4 py-3 text-right font-mono text-sm text-text-secondary">{{ topic.sort }}</td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-1">
                    <button
                      type="button"
                      class="topic-action"
                      title="编辑专栏"
                      @click="openEdit(topic)"
                    >
                      <Icon icon="ri:pencil-line" />
                    </button>
                    <button
                      type="button"
                      class="topic-action topic-action-danger"
                      title="删除专栏"
                      :disabled="deletingId === topic.id"
                      @click="remove(topic)"
                    >
                      <Icon :icon="deletingId === topic.id ? 'ri:loader-4-line' : 'ri:delete-bin-line'" :class="deletingId === topic.id && 'animate-spin'" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </VCard>
    </main>

    <VModal v-model:visible="modalVisible" :title="editingId == null ? '新建专栏' : '编辑专栏'" :width="620">
      <form class="space-y-4" @submit.prevent="save">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="topic-field sm:col-span-2">
            <span>名称 <b>*</b></span>
            <input v-model="form.name" class="apple-input" required placeholder="例如：前端实践" />
          </label>
          <label class="topic-field">
            <span>Slug</span>
            <input v-model="form.slug" class="apple-input font-mono" placeholder="留空按名称生成" />
          </label>
          <label class="topic-field">
            <span>排序值</span>
            <input v-model.number="form.sort" class="apple-input font-mono" type="number" step="1" />
          </label>
        </div>
        <label class="topic-field">
          <span>描述</span>
          <textarea v-model="form.description" class="apple-textarea min-h-20" rows="3" placeholder="用于前台专栏列表和详情页" />
        </label>
        <div class="topic-field">
          <span>图标</span>
          <IconPicker v-model="form.icon" placeholder="选择 icons.yml 图标或填写图片 URL" />
        </div>
        <div class="topic-field">
          <span>封面</span>
          <AttachmentField v-model="form.cover" placeholder="输入图片 URL或从附件中选择" show-thumb />
        </div>
        <div class="flex justify-end gap-2 border-t border-border pt-4">
          <VButton type="default" :disabled="saving" @click="modalVisible = false">取消</VButton>
          <VButton type="primary" :loading="saving" @click="save">保存</VButton>
        </div>
      </form>
    </VModal>
  </div>
</template>

<style scoped>
.topics-toolbar {
  box-shadow: var(--shadow-sm);
}

.topic-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.topic-field > span {
  font-weight: 500;
}

.topic-field b {
  color: var(--color-state-error);
  font-weight: 500;
}

.topic-action {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  transition: color var(--duration-fast) var(--ease-standard), background var(--duration-fast) var(--ease-standard), border-color var(--duration-fast) var(--ease-standard);
}

.topic-action:hover:not(:disabled) {
  border-color: var(--color-border);
  background: var(--color-secondary);
  color: var(--color-foreground);
}

.topic-action-danger:hover:not(:disabled) {
  color: var(--color-state-error);
}

.topic-action:disabled {
  cursor: wait;
  opacity: 0.55;
}

@media (max-width: 640px) {
  .topics-table th,
  .topics-table td {
    padding-left: 12px;
    padding-right: 12px;
  }
}
</style>
