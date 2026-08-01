<script setup lang="ts">
import { ref, computed } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { queryKeys } from "@/api/query-client";
import { linkApi } from "@/api";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VButton,
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

// 连通性检测状态（批注：对齐 Halo“检测全部”）：存 {ok, status}，status 为 HTTP 状态码（0=无法访问）
const checkStatus = ref<Record<number, { ok: boolean; status: number }>>({});
const checking = ref(false);
async function runCheck() {
  checking.value = true;
  try {
    const { data } = await linkApi.check();
    const map: Record<number, { ok: boolean; status: number }> = {};
    let okN = 0;
    let errN = 0;
    for (const r of data.items || []) {
      map[r.id] = { ok: r.ok, status: r.status };
      r.ok ? okN++ : errN++;
    }
    checkStatus.value = map;
    Toast.success("检测完成", { description: `正常 ${okN} · 异常 ${errN}` });
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "检测失败");
  } finally {
    checking.value = false;
  }
}

/** 异常原因提示：HTTP 状态码或“无法访问” */
function errorReason(id: number): string {
  const s = checkStatus.value[id]?.status;
  return s && s > 0 ? `HTTP ${s}` : "无法访问";
}

// 状态筛选：全部 / 正常 / 异常（仅在检测后有意义）
const statusFilter = ref<"all" | "ok" | "error">("all");

// 按 group_title 分组展示（保留服务端返回顺序），并应用状态筛选
const grouped = computed(() => {
  const map: Record<string, LinkRow[]> = {};
  const order: string[] = [];
  for (const l of links.value) {
    if (statusFilter.value !== "all" && (checkStatus.value[l.id]?.ok ? "ok" : checkStatus.value[l.id] ? "error" : undefined) !== statusFilter.value) continue;
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
  "w-full rounded-base border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500";

function closeMenus() {
  menuOpenId.value = null;
}
</script>

<template>
  <div @click="closeMenus">
    <VPageHeader title="链接">
      <template #icon>
        <Icon icon="ri:links-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <router-link
          to="/subscriptions"
          class="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
        >
          <Icon icon="ri:rss-line" />链接订阅
        </router-link>
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
        <!-- 工具条（对齐 Halo 链接页） -->
        <div class="flex flex-wrap items-center gap-2 rounded-xl border border-gray-200 bg-white px-3 py-2.5 dark:border-gray-700 dark:bg-gray-800">
          <button class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700" @click="openCreate">新建分组</button>
          <button class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700" @click="openCreate">新建链接</button>
          <button
            :disabled="checking"
            class="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
            @click="runCheck"
          >
            <Icon :icon="checking ? 'ri:loader-4-line' : 'ri:pulse-line'" :class="checking && 'animate-spin'" />检测全部
          </button>
          <span class="flex-1"></span>
          <span class="text-sm text-gray-400">状态：</span>
          <select v-model="statusFilter" class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm text-gray-600 outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300">
            <option value="all">全部</option>
            <option value="ok">正常</option>
            <option value="error">异常</option>
          </select>
        </div>

        <!-- 分组卡片网格 -->
        <div v-for="g in grouped" :key="g.title" class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
          <div class="mb-3 flex items-center justify-between">
            <span class="text-sm font-semibold text-gray-700 dark:text-gray-200">{{ g.title }} <span class="text-gray-400">({{ g.items.length }})</span></span>
            <button class="rounded-lg bg-secondary px-3 py-1 text-xs text-white hover:opacity-90" @click="openCreate">新建</button>
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <div
              v-for="l in g.items"
              :key="l.id"
              class="group relative rounded-lg border border-gray-200 p-3 transition-shadow hover:shadow-sm dark:border-gray-700"
            >
              <div class="flex items-center gap-2.5">
                <img v-if="l.avatar" :src="l.avatar" class="h-9 w-9 shrink-0 rounded-full bg-gray-100 object-cover dark:bg-gray-700" alt="" />
                <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-300 dark:bg-gray-700 dark:text-gray-600"><Icon icon="ri:link" /></span>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-1.5">
                    <span class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">{{ l.name }}</span>
                    <span v-if="l.label" class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium leading-none text-white" :style="{ backgroundColor: l.color ? '#' + l.color : '#9ca3af' }">{{ l.label }}</span>
                  </div>
                  <div class="truncate text-xs text-gray-400">{{ l.url }}</div>
                </div>
                <!-- 悬停操作 -->
                <div class="relative shrink-0 opacity-0 transition-opacity group-hover:opacity-100">
                  <div class="cursor-pointer rounded p-1 hover:bg-gray-100 dark:hover:bg-gray-700" @click.stop="menuOpenId = menuOpenId === l.id ? null : l.id">
                    <Icon icon="ri:more-fill" class="text-gray-400" />
                  </div>
                  <Transition name="fade">
                    <div v-if="menuOpenId === l.id" class="absolute right-0 top-full z-30 mt-1 w-28 rounded-lg border border-gray-100 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800" @click.stop>
                      <VButton block type="secondary" size="sm" @click="openEdit(l)">编辑</VButton>
                      <div class="mt-2"><VButton block type="danger" size="sm" @click="handleDelete(l)">删除</VButton></div>
                    </div>
                  </Transition>
                </div>
              </div>
              <!-- 底部徐标行：状态（药丸）/ RSS / 访问 -->
              <div class="mt-2.5 flex items-center gap-2 text-xs">
                <span v-if="checkStatus[l.id]?.ok" class="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-green-700 dark:bg-green-500/15 dark:text-green-400"><Icon icon="ri:checkbox-circle-fill" />正常</span>
                <span v-else-if="checkStatus[l.id]" :title="errorReason(l.id)" class="inline-flex items-center gap-1 rounded-full bg-red-100 px-2 py-0.5 text-red-600 dark:bg-red-500/15 dark:text-red-400"><Icon icon="ri:error-warning-fill" />异常·{{ errorReason(l.id) }}</span>
                <span v-else class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-gray-400 dark:bg-gray-700 dark:text-gray-500"><Icon icon="ri:pulse-line" />未检测</span>
                <span v-if="l.rss" class="inline-flex items-center gap-1 text-gray-400"><Icon icon="ri:rss-line" />RSS</span>
                <a :href="l.url" target="_blank" class="inline-flex items-center gap-1 text-gray-400 hover:text-primary"><Icon icon="ri:external-link-line" />访问</a>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- Edit/Create modal -->
    <VModal v-model:visible="showModal" :title="editing.id ? '编辑友链' : '新增友链'" :width="560">
      <p v-if="error" class="mb-3 rounded-base bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-500/10 dark:text-red-400">
        {{ error }}
      </p>
      <div class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">分组</label>
            <input v-model="editing.group_title" :class="inputCls" placeholder="我的朋友" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">名称 *</label>
            <input v-model="editing.name" :class="inputCls" />
          </div>
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">链接 *</label>
          <input v-model="editing.url" :class="inputCls" placeholder="https://" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">头像 URL</label>
          <input v-model="editing.avatar" :class="inputCls" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">描述</label>
          <input v-model="editing.description" :class="inputCls" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">RSS/Atom（动态聚合用）</label>
          <input v-model="editing.rss" :class="inputCls" />
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">标签</label>
            <input v-model="editing.label" :class="inputCls" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">配色（不带 #）</label>
            <input v-model="editing.color" :class="inputCls" placeholder="1BCDFC" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">排序（大在前）</label>
            <input v-model.number="editing.sort" type="number" :class="inputCls" />
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
