<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { commentApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEntityContainer,
  VEntity,
  VEntityField,
  VEmpty,
  VLoading,
  VPagination,
  VStatusDot,
  VModal,
  FilterDropdown,
  FilterTag,
  Dialog,
  Toast,
} from "@/components/ui";

const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);

const selectedStatus = ref<string | undefined>(undefined);
const statusItems = [
  { label: "全部", value: undefined },
  { label: "待审核", value: "pending" },
  { label: "已通过", value: "approved" },
];

const menuOpenId = ref<number | null>(null);
const replyVisible = ref(false);
const replyTarget = ref<any>(null);
const replyContent = ref("");

const hasFilters = computed(() => selectedStatus.value !== undefined);
const statusLabel = computed(() => statusItems.find((i) => i.value === selectedStatus.value)?.label || "");

// 响应式查询参数：page/size/status 变化自动重取
const queryParams = computed(() => {
  const params: Record<string, unknown> = { page: page.value, size: size.value };
  if (selectedStatus.value) params.status = selectedStatus.value;
  return params;
});

const { data: queryData, isLoading, isFetching, refetch } = useQuery({
  queryKey: computed(() => queryKeys.comments(queryParams.value)),
  queryFn: () => commentApi.list(queryParams.value).then(({ data }) => data),
});

const comments = computed(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

// 批量选择（对齐 PostList 既有模式）
const checkedAll = ref(false);
const selectedIds = ref<number[]>([]);

watch(selectedIds, (val) => {
  checkedAll.value = val.length === comments.value.length && val.length > 0;
});

// 翻页/筛选变化后清空选中，全选只作用于当前筛选结果当前页
watch(queryParams, () => {
  selectedIds.value = [];
});

function handleCheckAllChange(e: Event) {
  const { checked } = e.target as HTMLInputElement;
  selectedIds.value = checked ? comments.value.map((cm: any) => cm.id) : [];
}

function checkSelection(id: number) {
  return selectedIds.value.includes(id);
}

function toggleSelection(id: number) {
  if (checkSelection(id)) {
    selectedIds.value = selectedIds.value.filter((i) => i !== id);
  } else {
    selectedIds.value = [...selectedIds.value, id];
  }
}

// 批量通过：不触发邮件通知（后端批量端点不接 notify）
async function handleApproveInBatch() {
  try {
    const { data } = await commentApi.batch({ ids: selectedIds.value, action: "approve" });
    selectedIds.value = [];
    queryClient.invalidateQueries({ queryKey: ["comments"] });
    Toast.success(`批量通过完成：成功 ${data.succeeded} 条，失败 ${data.failed} 条`);
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "批量通过失败");
  }
}

function handleDeleteInBatch() {
  const count = selectedIds.value.length;
  Dialog.warning({
    title: `确定要删除选中的 ${count} 条评论吗？`,
    description: "该操作不可恢复",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        const { data } = await commentApi.batch({ ids: selectedIds.value, action: "delete" });
        selectedIds.value = [];
        queryClient.invalidateQueries({ queryKey: ["comments"] });
        Toast.success(`批量删除完成：成功 ${data.succeeded} 条，失败 ${data.failed} 条`);
      } catch (e: any) {
        Toast.error(e?.response?.data?.error || "批量删除失败");
      }
    },
  });
}

function handleFilterChange() {
  page.value = 1;
}

function handleClearFilters() {
  selectedStatus.value = undefined;
  page.value = 1;
}

function handleApprove(cm: any) {
  menuOpenId.value = null;
  Dialog.warning({
    title: "确定要通过该评论吗？",
    confirmType: "secondary",
    onConfirm: async () => {
      await commentApi.approve(cm.id);
      queryClient.invalidateQueries({ queryKey: ["comments"] });
      Toast.success("已通过");
    },
  });
}

function handleDelete(cm: any) {
  menuOpenId.value = null;
  Dialog.warning({
    title: "确定要删除该评论吗？",
    description: "该操作不可恢复",
    confirmType: "danger",
    onConfirm: async () => {
      await commentApi.delete(cm.id);
      queryClient.invalidateQueries({ queryKey: ["comments"] });
      Toast.success("删除成功");
    },
  });
}

function openReply(cm: any) {
  menuOpenId.value = null;
  replyTarget.value = cm;
  replyContent.value = "";
  replyVisible.value = true;
}

async function handleReply() {
  if (!replyTarget.value || !replyContent.value.trim()) return;
  await commentApi.reply({ parent_id: replyTarget.value.id, content: replyContent.value });
  replyVisible.value = false;
  replyTarget.value = null;
  replyContent.value = "";
  queryClient.invalidateQueries({ queryKey: ["comments"] });
  Toast.success("回复成功");
}

function closeMenus() {
  menuOpenId.value = null;
}
</script>

<template>
  <div @click="closeMenus">
    <VPageHeader title="评论">
      <template #icon>
        <Icon icon="ri:chat-3-line" class="mr-2 self-center text-xl" />
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
      <VCard :body-class="['!p-0']">
        <template #header>
          <div class="block w-full bg-gray-50 px-4 py-3 dark:bg-gray-700/50">
            <div class="relative flex flex-col items-start sm:flex-row sm:items-center">
              <div class="mr-4 hidden items-center sm:flex">
                <input
                  :checked="checkedAll"
                  class="h-4 w-4 rounded border-gray-300 text-primary"
                  type="checkbox"
                  @change="handleCheckAllChange"
                />
              </div>
              <div class="flex w-full flex-1 items-center sm:w-auto">
                <!-- Normal mode -->
                <div v-if="!selectedIds.length" class="flex items-center gap-2">
                  <FilterTag v-if="selectedStatus !== undefined" @close="selectedStatus = undefined; handleFilterChange()">
                    状态：{{ statusLabel }}
                  </FilterTag>
                  <button
                    v-if="hasFilters"
                    class="text-xs text-gray-400 underline hover:text-gray-600 dark:hover:text-gray-300"
                    @click="handleClearFilters"
                  >
                    清除筛选
                  </button>
                </div>
                <!-- Batch mode -->
                <div v-else class="flex items-center gap-3">
                  <VButton type="secondary" size="sm" @click="handleApproveInBatch">批量通过</VButton>
                  <VButton type="danger" size="sm" @click="handleDeleteInBatch">批量删除</VButton>
                  <span class="text-sm text-gray-500 dark:text-gray-400">已选择 {{ selectedIds.length }} 项</span>
                </div>
              </div>
              <div class="mt-4 flex sm:mt-0">
                <div class="flex items-center gap-6">
                  <FilterDropdown v-model="selectedStatus" :items="statusItems" label="状态" @change="handleFilterChange" />
                  <div class="group cursor-pointer rounded p-1 hover:bg-gray-200 dark:hover:bg-gray-600" @click="refetch()">
                    <Icon
                      icon="ri:refresh-line"
                      :class="{ 'animate-spin text-gray-900 dark:text-gray-100': isFetching }"
                      class="h-4 w-4 text-gray-600 group-hover:text-gray-900 dark:text-gray-300 dark:group-hover:text-gray-100"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </template>

        <VLoading v-if="isLoading" />

        <Transition v-else-if="!comments.length" appear name="fade">
          <VEmpty message="你可以尝试刷新或者查看评论筛选" title="当前没有评论">
            <template #actions>
              <VButton @click="refetch">刷新</VButton>
            </template>
          </VEmpty>
        </Transition>

        <Transition v-else appear name="fade">
          <VEntityContainer>
              <VEntity v-for="cm in comments" :key="cm.id" :is-selected="checkSelection(cm.id)">
                <template #checkbox>
                  <input
                    :checked="checkSelection(cm.id)"
                    class="h-4 w-4 rounded border-gray-300 text-primary"
                    type="checkbox"
                    @change="toggleSelection(cm.id)"
                  />
                </template>
                <template #start>
                  <VEntityField max-width="30rem">
                    <template #title>
                      <div class="flex items-center gap-2">
                        <img :src="cm.avatar" class="h-8 w-8 shrink-0 rounded-full" alt="" />
                        <span class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">{{ cm.nick }}</span>
                        <span
                          v-if="cm.is_admin"
                          class="rounded bg-secondary/10 px-1.5 py-0.5 text-xs text-secondary dark:bg-primary/15 dark:text-primary"
                          style="border-radius: 4px"
                        >
                          博主
                        </span>
                      </div>
                    </template>
                    <template #description>
                      <div class="flex flex-col gap-1">
                        <p v-if="cm.reply_nick" class="text-xs text-gray-400 dark:text-gray-500">回复 @{{ cm.reply_nick }}</p>
                        <p class="line-clamp-2 text-sm text-gray-600 dark:text-gray-400">{{ cm.content }}</p>
                        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                          <span class="text-xs text-gray-400 dark:text-gray-500">{{ cm.date }}</span>
                          <span v-if="cm.page_key" class="text-xs text-gray-400 dark:text-gray-500">{{ cm.page_key }}</span>
                        </div>
                      </div>
                    </template>
                  </VEntityField>
                </template>
                <template #end>
                  <VEntityField v-if="cm.status === 'approved'">
                    <template #description>
                      <VStatusDot state="success" text="已通过" />
                    </template>
                  </VEntityField>
                  <VEntityField v-else>
                    <template #description>
                      <VStatusDot state="warning" text="待审核" />
                    </template>
                  </VEntityField>
                </template>
                <template #dropdownItems>
                  <div class="relative">
                    <div
                      class="cursor-pointer rounded p-1 transition-all hover:bg-gray-200/60 hover:text-blue-600"
                      @click.stop="menuOpenId = menuOpenId === cm.id ? null : cm.id"
                    >
                      <Icon icon="ri:more-fill" />
                    </div>
                    <Transition name="fade">
                      <div
                        v-if="menuOpenId === cm.id"
                        class="absolute right-0 top-full z-30 mt-1 w-32 rounded border border-gray-100 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800"
                        style="border-radius: 4px"
                        @click.stop
                      >
                        <VButton v-if="cm.status === 'pending'" block type="secondary" size="sm" @click="handleApprove(cm)">
                          通过
                        </VButton>
                        <div v-if="cm.status === 'pending'" class="mt-2">
                          <VButton block size="sm" @click="openReply(cm)">回复</VButton>
                        </div>
                        <div class="mt-2">
                          <VButton block type="danger" size="sm" @click="handleDelete(cm)">删除</VButton>
                        </div>
                      </div>
                    </Transition>
                  </div>
                </template>
              </VEntity>
          </VEntityContainer>
        </Transition>

        <template #footer>
          <div class="bg-white sm:flex sm:items-center sm:justify-end dark:bg-gray-800">
            <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 30, 50, 100]" />
          </div>
        </template>
      </VCard>
    </div>

    <!-- Reply modal -->
    <VModal v-model:visible="replyVisible" :title="`回复 @${replyTarget?.nick || ''}`" :width="480">
      <textarea
        v-model="replyContent"
        rows="4"
        placeholder="回复内容..."
        class="w-full resize-y border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
        style="border-radius: 4px"
      ></textarea>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="replyVisible = false">取消</VButton>
          <VButton type="secondary" @click="handleReply">发送</VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
