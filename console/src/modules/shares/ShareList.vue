<script setup lang="ts">
import { ref, computed } from "vue";
import { useQuery, useMutation, useQueryClient } from "@tanstack/vue-query";
import { shareApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEntity,
  VEntityField,
  VEmpty,
  VLoading,
  VPagination,
  Dialog,
  Toast,
} from "@/components/ui";

/** 分享链接响应模型 */
interface ShareRow {
  id: number;
  token: string;
  post_slug: string;
  post_title: string;
  share_type: string;
  expires_at: string;
  created_at: string;
  url: string;
}

const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);

/** 响应式查询参数：page/size 变化自动重取 */
const queryParams = computed(() => ({
  page: page.value,
  size: size.value,
}));

const { data: queryData, isLoading, isFetching, refetch } = useQuery({
  queryKey: computed(() => queryKeys.shares(queryParams.value)),
  queryFn: () => shareApi.list(queryParams.value).then(({ data }) => data),
});

const shares = computed<ShareRow[]>(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

/** 删除 mutation：成功后失效分享列表缓存 */
const deleteMutation = useMutation({
  mutationFn: (id: number) => shareApi.delete(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ["shares"] });
    Toast.success("删除成功");
  },
});

/** 删除前弹出确认对话框 */
function handleDelete(share: ShareRow) {
  Dialog.warning({
    title: "确定要删除该分享链接吗？",
    description: "删除后该链接将立即失效，无法再用于访问文章",
    confirmType: "danger",
    onConfirm: () => deleteMutation.mutate(share.id),
  });
}

/** 拼接完整可访问的分享 URL */
function buildFullUrl(url: string): string {
  return window.location.origin + url;
}

/** 复制分享链接到剪贴板 */
async function handleCopy(url: string) {
  const fullUrl = buildFullUrl(url);
  try {
    await navigator.clipboard.writeText(fullUrl);
    Toast.success("链接已复制到剪贴板");
  } catch {
    Toast.error("复制失败，请手动复制");
  }
}

/** 格式化日期时间：2026-07-27T14:27:00Z -> 2026-07-27 14:27 */
function formatDateTime(dateStr: string): string {
  if (!dateStr) return "";
  return dateStr.slice(0, 16).replace("T", " ");
}

/** 分享类型对应的中文标签 */
function shareTypeLabel(shareType: string): string {
  return shareType === "draft" ? "草稿" : "已发布";
}
</script>

<template>
  <div>
    <VPageHeader title="文章分享链接">
      <template #icon>
        <Icon icon="ri:share-forward-line" class="mr-2 self-center text-xl" />
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
      <VCard :body-class="['!p-0']">
        <!-- 筛选头部：刷新按钮 -->
        <template #header>
          <div class="block w-full bg-gray-50 px-4 py-3 dark:bg-gray-700/50">
            <div class="flex items-center justify-end">
              <div
                class="group cursor-pointer rounded p-1 hover:bg-gray-200 dark:hover:bg-gray-600"
                @click="refetch()"
              >
                <Icon
                  icon="ri:refresh-line"
                  :class="{ 'animate-spin text-gray-900 dark:text-gray-100': isFetching }"
                  class="h-4 w-4 text-gray-600 group-hover:text-gray-900 dark:text-gray-300 dark:group-hover:text-gray-100"
                />
              </div>
            </div>
          </div>
        </template>

        <!-- Loading -->
        <VLoading v-if="isLoading" />

        <!-- Empty -->
        <Transition v-else-if="!shares.length" appear name="fade">
          <VEmpty message="你可以在文章列表中为文章创建分享链接" title="暂无分享链接">
            <template #actions>
              <VButton @click="refetch">刷新</VButton>
            </template>
          </VEmpty>
        </Transition>

        <!-- Entity list -->
        <Transition v-else appear name="fade">
          <ul class="box-border h-full w-full divide-y divide-gray-100 dark:divide-gray-700" role="list">
            <li v-for="share in shares" :key="share.id">
              <VEntity>
                <template #start>
                  <VEntityField :title="share.post_title || share.post_slug" width="20rem">
                    <template #title>
                      <span class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
                        {{ share.post_title || share.post_slug }}
                      </span>
                    </template>
                    <template #description>
                      <span class="truncate text-xs text-gray-400 dark:text-gray-500">
                        /{{ share.post_slug }}
                      </span>
                    </template>
                  </VEntityField>
                </template>
                <template #end>
                  <!-- 分享链接 -->
                  <VEntityField width="22rem">
                    <template #description>
                      <div class="flex items-center gap-1.5">
                        <a
                          :href="buildFullUrl(share.url)"
                          target="_blank"
                          class="truncate text-xs text-blue-600 hover:underline dark:text-blue-400"
                          :title="buildFullUrl(share.url)"
                        >
                          {{ share.token }}
                        </a>
                        <span
                          class="cursor-pointer shrink-0 text-gray-400 transition-all hover:text-blue-600"
                          title="复制链接"
                          @click="handleCopy(share.url)"
                        >
                          <Icon icon="ri:file-copy-line" class="h-3.5 w-3.5" />
                        </span>
                      </div>
                    </template>
                  </VEntityField>

                  <!-- 分享类型 -->
                  <VEntityField>
                    <template #description>
                      <span
                        class="inline-block rounded px-1.5 py-0.5 text-xs font-medium"
                        :class="share.share_type === 'draft'
                          ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-400'
                          : 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'"
                        style="border-radius: 4px"
                      >
                        {{ shareTypeLabel(share.share_type) }}
                      </span>
                    </template>
                  </VEntityField>

                  <!-- 创建时间 -->
                  <VEntityField>
                    <template #description>
                      <span class="truncate text-xs tabular-nums text-gray-500 dark:text-gray-400">
                        {{ formatDateTime(share.created_at) }}
                      </span>
                    </template>
                  </VEntityField>

                  <!-- 操作：删除 -->
                  <VEntityField>
                    <template #description>
                      <span
                        class="cursor-pointer text-sm text-gray-400 transition-all hover:text-red-500"
                        title="删除"
                        @click="handleDelete(share)"
                      >
                        <Icon icon="ri:delete-bin-line" />
                      </span>
                    </template>
                  </VEntityField>
                </template>
              </VEntity>
            </li>
          </ul>
        </Transition>

        <!-- Footer: pagination -->
        <template #footer>
          <div class="bg-white sm:flex sm:items-center sm:justify-end dark:bg-gray-800">
            <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 30, 50, 100]" />
          </div>
        </template>
      </VCard>
    </div>
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
