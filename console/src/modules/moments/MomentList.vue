<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { momentApi, settingsApi } from "@/api";
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

// 站长头像（发布框与列表项展示）
const ownerAvatar = ref("");
onMounted(async () => {
  try {
    const { data } = await settingsApi.all();
    ownerAvatar.value = data?.site?.avatar || "";
  } catch {
    /* 静默 */
  }
});

// 发布框状态：聚焦后展开工具条；字数统计（非空白 rune）
const composerFocused = ref(false);
const charCount = computed(() => {
  let n = 0;
  for (const ch of newContent.value) {
    if (!/\s/.test(ch)) n++;
  }
  return n;
});

// Cmd/Ctrl+Enter 快捷发布
function onComposerKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
    e.preventDefault();
    handlePublish();
  }
}

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

// 相对时间：列表项展示用（超过 30 天回退绝对时间）
function formatRelative(s: string): string {
  if (!s) return "";
  const t = new Date(s).getTime();
  if (isNaN(t)) return formatTime(s);
  const diff = Date.now() - t;
  const min = Math.floor(diff / 60000);
  if (min < 1) return "刚刚";
  if (min < 60) return `${min} 分钟前`;
  const hour = Math.floor(min / 60);
  if (hour < 24) return `${hour} 小时前`;
  const day = Math.floor(hour / 24);
  if (day <= 30) return `${day} 天前`;
  return formatTime(s);
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
      <!-- 发布框（Halo Moments 风格：头像 + 无边框输入 + 底部工具条） -->
      <VCard :body-class="['!p-4']">
        <div class="flex gap-3">
          <img
            v-if="ownerAvatar"
            :src="ownerAvatar"
            alt=""
            class="h-9 w-9 shrink-0 rounded-full border border-gray-100 object-cover dark:border-gray-700"
          />
          <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/15">
            <Icon icon="ri:user-3-line" class="text-lg text-primary" />
          </span>
          <div class="min-w-0 flex-1">
            <textarea
              v-model="newContent"
              :rows="composerFocused || newContent ? 4 : 2"
              placeholder="分享此刻的想法…"
              class="w-full resize-none border-0 bg-transparent px-0 py-1.5 text-sm text-gray-900 outline-none transition-all placeholder:text-gray-400 dark:text-gray-100 dark:placeholder-gray-500"
              @focus="composerFocused = true"
              @blur="composerFocused = false"
              @keydown="onComposerKeydown"
            ></textarea>
            <div class="flex items-center justify-between border-t border-gray-100 pt-2.5 dark:border-gray-700">
              <div class="flex items-center gap-2 text-gray-400 dark:text-gray-500">
                <Icon icon="ri:markdown-line" class="text-base" title="支持 Markdown 语法" />
                <span v-if="charCount" class="text-xs tabular-nums">{{ charCount }} 字</span>
                <span v-else class="text-xs">支持 Markdown</span>
              </div>
              <div class="flex items-center gap-3">
                <span class="hidden text-xs text-gray-300 sm:inline dark:text-gray-600">⌘ + Enter 发布</span>
                <VButton
                  type="secondary"
                  size="sm"
                  :loading="publishing"
                  :disabled="!newContent.trim()"
                  @click="handlePublish"
                >
                  <template #icon>
                    <Icon icon="ri:send-plane-fill" class="h-full w-full" />
                  </template>
                  发布
                </VButton>
              </div>
            </div>
          </div>
        </div>
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
          <VEntityContainer>
              <VEntity v-for="m in moments" :key="m.id">
                <template #start>
                  <VEntityField max-width="40rem">
                    <template #description>
                      <div class="flex gap-3">
                        <img
                          v-if="ownerAvatar"
                          :src="ownerAvatar"
                          alt=""
                          class="mt-0.5 h-7 w-7 shrink-0 rounded-full border border-gray-100 object-cover dark:border-gray-700"
                        />
                        <div class="flex min-w-0 flex-col gap-1.5">
                          <span class="text-xs text-gray-400 dark:text-gray-500" :title="formatTime(m.created_at)">
                            {{ formatRelative(m.created_at) }}
                          </span>
                          <div class="prose prose-sm max-w-none text-gray-700 dark:text-gray-300" v-html="m.content"></div>
                        </div>
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
          </VEntityContainer>
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
