<script setup lang="ts">
import { ref, computed } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { momentApi } from "@/api";
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

const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);
const newContent = ref("");
const publishing = ref(false);
const menuOpenId = ref<number | null>(null);

const queryParams = computed(() => ({ page: page.value, size: size.value }));

const { data: queryData, isLoading, refetch } = useQuery({
  queryKey: computed(() => queryKeys.moments(queryParams.value)),
  queryFn: () => momentApi.list(queryParams.value).then(({ data }) => data),
});

const moments = computed(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

async function handlePublish() {
  if (!newContent.value.trim()) return;
  publishing.value = true;
  try {
    await momentApi.create({ content_md: newContent.value });
    newContent.value = "";
    queryClient.invalidateQueries({ queryKey: ["moments"] });
    Toast.success("发布成功");
  } catch {} finally {
    publishing.value = false;
  }
}

function handleDelete(m: any) {
  menuOpenId.value = null;
  Dialog.warning({
    title: "确定要删除该瞬间吗？",
    description: "该操作不可恢复",
    confirmType: "danger",
    onConfirm: async () => {
      await momentApi.delete(m.id);
      queryClient.invalidateQueries({ queryKey: ["moments"] });
      Toast.success("删除成功");
    },
  });
}

function formatTime(s: string): string {
  if (!s) return "";
  return s.slice(0, 16).replace("T", " ");
}

function closeMenus() {
  menuOpenId.value = null;
}
</script>

<template>
  <div @click="closeMenus">
    <VPageHeader title="瞬间">
      <template #icon>
        <Icon icon="ri:quill-pen-line" class="mr-2 self-center text-xl" />
      </template>
    </VPageHeader>

    <div class="m-0 space-y-4 md:m-4">
      <!-- Publish box -->
      <VCard>
        <template #header>
          <span class="text-base font-bold text-gray-900 dark:text-gray-100">发布瞬间</span>
        </template>
        <textarea
          v-model="newContent"
          rows="3"
          placeholder="记录此刻想法...（支持 Markdown）"
          class="w-full resize-y border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
          style="border-radius: 4px"
        ></textarea>
        <template #footer>
          <div class="flex justify-end">
            <VButton type="secondary" :loading="publishing" :disabled="!newContent.trim()" @click="handlePublish">
              <template #icon>
                <Icon icon="ri:send-plane-fill" class="h-full w-full" />
              </template>
              发布
            </VButton>
          </div>
        </template>
      </VCard>

      <!-- Moment list -->
      <VCard :body-class="['!p-0']">
        <VLoading v-if="isLoading" />

        <Transition v-else-if="!moments.length" appear name="fade">
          <VEmpty message="发布第一条瞬间吧" title="当前没有瞬间">
            <template #actions>
              <VButton @click="refetch">刷新</VButton>
            </template>
          </VEmpty>
        </Transition>

        <Transition v-else appear name="fade">
          <ul class="box-border h-full w-full divide-y divide-gray-100 dark:divide-gray-700" role="list">
            <li v-for="m in moments" :key="m.id">
              <VEntity>
                <template #start>
                  <VEntityField width="40rem">
                    <template #description>
                      <div class="flex flex-col gap-1.5">
                        <span class="text-xs text-gray-400 dark:text-gray-500">{{ formatTime(m.created_at) }}</span>
                        <div class="prose prose-sm max-w-none text-gray-700 dark:text-gray-300" v-html="m.content"></div>
                      </div>
                    </template>
                  </VEntityField>
                </template>
                <template #dropdownItems>
                  <div class="relative">
                    <div
                      class="cursor-pointer rounded p-1 transition-all hover:bg-gray-200/60 hover:text-blue-600"
                      @click.stop="menuOpenId = menuOpenId === m.id ? null : m.id"
                    >
                      <Icon icon="ri:more-fill" />
                    </div>
                    <Transition name="fade">
                      <div
                        v-if="menuOpenId === m.id"
                        class="absolute right-0 top-full z-30 mt-1 w-32 rounded border border-gray-100 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800"
                        style="border-radius: 4px"
                        @click.stop
                      >
                        <VButton block type="danger" size="sm" @click="handleDelete(m)">删除</VButton>
                      </div>
                    </Transition>
                  </div>
                </template>
              </VEntity>
            </li>
          </ul>
        </Transition>

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
