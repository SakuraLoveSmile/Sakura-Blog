<script setup lang="ts">
import { ref, computed } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { queryKeys } from "@/api/query-client";
import { linkApi } from "@/api";
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
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";

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
}

const queryClient = useQueryClient();

const { data: queryData, isLoading } = useQuery({
  queryKey: queryKeys.friends(),
  queryFn: () => linkApi.list().then(({ data }) => data),
});

const links = computed<LinkRow[]>(() => queryData.value?.items || []);

const showModal = ref(false);
const saving = ref(false);
const error = ref("");
const editing = ref<Partial<LinkRow>>({});
const menuOpenId = ref<number | null>(null);

// 按 group_title 分组展示（保留服务端返回顺序）
const grouped = computed(() => {
  const map: Record<string, LinkRow[]> = {};
  const order: string[] = [];
  for (const l of links.value) {
    const g = l.group_title || "未分组";
    if (!map[g]) {
      map[g] = [];
      order.push(g);
    }
    map[g].push(l);
  }
  return order.map((title) => ({ title, items: map[title] }));
});

function openCreate() {
  editing.value = { group_title: "", name: "", url: "", avatar: "", description: "", rss: "", label: "", color: "", sort: 0 };
  error.value = "";
  showModal.value = true;
}
function openEdit(l: LinkRow) {
  menuOpenId.value = null;
  editing.value = { ...l };
  error.value = "";
  showModal.value = true;
}

async function handleSave() {
  saving.value = true;
  error.value = "";
  try {
    const payload = { ...editing.value };
    if (editing.value.id) {
      await linkApi.update(editing.value.id, payload);
    } else {
      await linkApi.create(payload);
    }
    showModal.value = false;
    queryClient.invalidateQueries({ queryKey: ["friends"] });
    Toast.success("保存成功");
  } catch (e: any) {
    error.value = e?.response?.data?.error || "保存失败";
  } finally {
    saving.value = false;
  }
}

function handleDelete(l: LinkRow) {
  menuOpenId.value = null;
  Dialog.warning({
    title: `确定删除友链「${l.name}」？`,
    confirmType: "danger",
    onConfirm: async () => {
      await linkApi.delete(l.id);
      queryClient.invalidateQueries({ queryKey: ["friends"] });
      Toast.success("删除成功");
    },
  });
}

const inputCls =
  "w-full border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500";

function closeMenus() {
  menuOpenId.value = null;
}
</script>

<template>
  <div @click="closeMenus">
    <VPageHeader title="友链">
      <template #icon>
        <Icon icon="ri:links-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <VButton type="secondary" @click="openCreate">
          <template #icon>
            <Icon icon="ri:add-circle-line" class="h-full w-full" />
          </template>
          新增
        </VButton>
      </template>
    </VPageHeader>

    <div class="m-0 space-y-4 md:m-4">
      <VLoading v-if="isLoading" />

      <VEmpty v-else-if="links.length === 0" message="点击右上角新增第一条友链" title="当前没有友链">
        <template #actions>
          <VButton type="primary" @click="openCreate">
            <template #icon>
              <Icon icon="ri:add-circle-line" class="h-full w-full" />
            </template>
            新增友链
          </VButton>
        </template>
      </VEmpty>

      <template v-else>
        <VCard v-for="g in grouped" :key="g.title" :body-class="['!p-0']">
          <template #header>
            <span class="text-base font-bold text-gray-900 dark:text-gray-100">{{ g.title }}</span>
          </template>
          <VEntityContainer>
              <VEntity v-for="l in g.items" :key="l.id">
                <template #start>
                  <VEntityField max-width="30rem">
                    <template #title>
                      <div class="flex items-center gap-2">
                        <img
                          v-if="l.avatar"
                          :src="l.avatar"
                          class="h-8 w-8 shrink-0 rounded-full bg-gray-100 object-cover dark:bg-gray-700"
                          alt=""
                        />
                        <span
                          v-else
                          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-300 dark:bg-gray-700 dark:text-gray-600"
                        >
                          <Icon icon="ri:link" />
                        </span>
                        <a
                          :href="l.url"
                          target="_blank"
                          class="truncate text-sm font-medium text-gray-900 hover:text-blue-600 dark:text-gray-100 dark:hover:text-blue-400"
                        >
                          {{ l.name }}
                        </a>
                        <span
                          v-if="l.label"
                          class="shrink-0 self-center rounded px-1.5 py-0.5 text-[10px] font-medium leading-none text-white"
                          style="border-radius: 4px"
                          :style="{ backgroundColor: l.color ? '#' + l.color : '#9ca3af' }"
                        >
                          {{ l.label }}
                        </span>
                      </div>
                    </template>
                    <template #description>
                      <span class="truncate text-xs text-gray-400 dark:text-gray-500">{{ l.description }}</span>
                    </template>
                  </VEntityField>
                </template>
                <template #dropdownItems>
                  <div class="relative">
                    <div
                      class="cursor-pointer rounded p-1 transition-all hover:bg-gray-200/60 hover:text-blue-600"
                      @click.stop="menuOpenId = menuOpenId === l.id ? null : l.id"
                    >
                      <Icon icon="ri:more-fill" />
                    </div>
                    <Transition name="fade">
                      <div
                        v-if="menuOpenId === l.id"
                        class="absolute right-0 top-full z-30 mt-1 w-32 rounded border border-gray-100 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800"
                        style="border-radius: 4px"
                        @click.stop
                      >
                        <VButton block type="secondary" size="sm" @click="openEdit(l)">编辑</VButton>
                        <div class="mt-2">
                          <VButton block type="danger" size="sm" @click="handleDelete(l)">删除</VButton>
                        </div>
                      </div>
                    </Transition>
                  </div>
                </template>
              </VEntity>
          </VEntityContainer>
        </VCard>
      </template>
    </div>

    <!-- Edit/Create modal -->
    <VModal v-model:visible="showModal" :title="editing.id ? '编辑友链' : '新增友链'" :width="560">
      <p v-if="error" class="mb-3 rounded bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-500/10 dark:text-red-400" style="border-radius: 4px">
        {{ error }}
      </p>
      <div class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">分组</label>
            <input v-model="editing.group_title" :class="inputCls" style="border-radius: 4px" placeholder="我的朋友" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">名称 *</label>
            <input v-model="editing.name" :class="inputCls" style="border-radius: 4px" />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">链接 *</label>
          <input v-model="editing.url" :class="inputCls" style="border-radius: 4px" placeholder="https://" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">头像 URL</label>
          <input v-model="editing.avatar" :class="inputCls" style="border-radius: 4px" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">描述</label>
          <input v-model="editing.description" :class="inputCls" style="border-radius: 4px" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">RSS/Atom（动态聚合用）</label>
          <input v-model="editing.rss" :class="inputCls" style="border-radius: 4px" />
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">标签</label>
            <input v-model="editing.label" :class="inputCls" style="border-radius: 4px" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">配色（不带 #）</label>
            <input v-model="editing.color" :class="inputCls" style="border-radius: 4px" placeholder="1BCDFC" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">排序（大在前）</label>
            <input v-model.number="editing.sort" type="number" :class="inputCls" style="border-radius: 4px" />
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="showModal = false">取消</VButton>
          <VButton type="secondary" :loading="saving" @click="handleSave">保存</VButton>
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
