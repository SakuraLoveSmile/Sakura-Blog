<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { usersApi, type UserItem } from "@/api";
import { Icon } from "@iconify/vue";
import {
  VButton,
  VCard,
  VEmpty,
  VLoading,
  VPagination,
  VStatusDot,
  VModal,
  Dialog,
  Toast,
} from "@/components/ui";

const queryClient = useQueryClient();

const page = ref(1);
const size = ref(20);
const keyword = ref("");
const debouncedKeyword = ref("");
let debounceTimer: ReturnType<typeof setTimeout> | null = null;
watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    debouncedKeyword.value = keyword.value;
    page.value = 1;
  }, 300);
});

const queryParams = computed(() => ({
  page: page.value,
  size: size.value,
  keyword: debouncedKeyword.value,
}));

const { data: queryData, isLoading, refetch } = useQuery({
  queryKey: computed(() => ["users", queryParams.value] as const),
  queryFn: () => usersApi.list(queryParams.value).then(({ data }) => data),
});

const users = computed<UserItem[]>(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

// 角色分段筛选（本地过滤当前页；后端暂无 role 查询参数）
const roleFilter = ref<"all" | "admin" | "author" | "disabled">("all");
const filteredUsers = computed(() => {
  if (roleFilter.value === "all") return users.value;
  if (roleFilter.value === "disabled") return [];
  return users.value.filter((u) => u.role === roleFilter.value);
});

function invalidate() {
  queryClient.invalidateQueries({ queryKey: ["users"] });
}

function errMsg(e: any, fallback: string): string {
  return e?.response?.data?.error || e?.response?.data?.detail || fallback;
}

// ---------- 创建用户 ----------
const createVisible = ref(false);
const createSaving = ref(false);
const createForm = ref({
  username: "",
  display_name: "",
  email: "",
  avatar: "",
  password: "",
  role: "author" as "admin" | "author",
});

function openCreate() {
  createForm.value = { username: "", display_name: "", email: "", avatar: "", password: "", role: "author" };
  createVisible.value = true;
}

async function submitCreate() {
  if (!createForm.value.username.trim()) return Toast.error("用户名不能为空");
  if (!createForm.value.password || createForm.value.password.length < 6)
    return Toast.error("密码至少 6 位");
  createSaving.value = true;
  try {
    await usersApi.create({
      username: createForm.value.username.trim(),
      display_name: createForm.value.display_name.trim(),
      email: createForm.value.email.trim(),
      avatar: createForm.value.avatar.trim(),
      password: createForm.value.password,
      role: createForm.value.role,
    });
    createVisible.value = false;
    invalidate();
    Toast.success("用户创建成功");
  } catch (e: any) {
    Toast.error(errMsg(e, "创建失败"));
  } finally {
    createSaving.value = false;
  }
}

// ---------- 编辑用户 ----------
const editVisible = ref(false);
const editSaving = ref(false);
const editTarget = ref<UserItem | null>(null);
const editForm = ref({ display_name: "", email: "", avatar: "", role: "author" as "admin" | "author" });

function openEdit(u: UserItem) {
  editTarget.value = u;
  editForm.value = { display_name: u.display_name, email: u.email, avatar: u.avatar, role: u.role };
  editVisible.value = true;
}

async function submitEdit() {
  if (!editTarget.value) return;
  editSaving.value = true;
  try {
    await usersApi.update(editTarget.value.id, editForm.value);
    editVisible.value = false;
    invalidate();
    Toast.success("已保存");
  } catch (e: any) {
    Toast.error(errMsg(e, "保存失败"));
  } finally {
    editSaving.value = false;
  }
}

// ---------- 重置密码 ----------
const pwdVisible = ref(false);
const pwdSaving = ref(false);
const pwdTarget = ref<UserItem | null>(null);
const pwdForm = ref({ password: "" });

function openResetPwd(u: UserItem) {
  pwdTarget.value = u;
  pwdForm.value = { password: "" };
  pwdVisible.value = true;
}

async function submitResetPwd() {
  if (!pwdTarget.value) return;
  if (!pwdForm.value.password || pwdForm.value.password.length < 6)
    return Toast.error("密码至少 6 位");
  pwdSaving.value = true;
  try {
    await usersApi.resetPassword(pwdTarget.value.id, pwdForm.value.password);
    pwdVisible.value = false;
    Toast.success("密码已重置，该用户旧登录态已失效");
  } catch (e: any) {
    Toast.error(errMsg(e, "重置失败"));
  } finally {
    pwdSaving.value = false;
  }
}

// ---------- 删除用户 ----------
function confirmDelete(u: UserItem) {
  Dialog.warning({
    title: `确定要删除用户「${u.username}」吗？`,
    description: "删除后该用户将无法登录，其名下文章的作者署名不受影响",
    confirmText: "删除",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await usersApi.remove(u.id);
        invalidate();
        Toast.success("已删除");
      } catch (e: any) {
        Toast.error(errMsg(e, "删除失败"));
      }
    },
  });
}

// ---------- 展示辅助 ----------
function roleLabel(role: string): string {
  return role === "admin" ? "管理员" : "作者";
}

type RoleTone = "admin" | "author";
function roleTone(role: string): RoleTone {
  return role === "admin" ? "admin" : "author";
}

// 按用户名 hash 取 1-8 的渐变头像类
function avatarGradient(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash * 31 + name.charCodeAt(i)) & 0xffffffff;
  }
  const n = (Math.abs(hash) % 8) + 1;
  return `avatar-gradient-${n}`;
}

function avatarInitial(u: UserItem): string {
  const source = u.display_name?.trim() || u.username || "?";
  return source.charAt(0).toUpperCase();
}

// 统计卡数据（从当前页用户列表派生；总数取后端 total）
const stats = computed(() => {
  const list = users.value;
  const adminCount = list.filter((u) => u.role === "admin").length;
  const authorCount = list.filter((u) => u.role === "author").length;
  return { total: total.value, adminCount, authorCount };
});

const inputCls = "apple-input";
const labelCls = "block text-xs text-text-tertiary mb-1";
</script>

<template>
  <div class="apple-ambient-glow min-h-full">
    <!-- ===== Page Header ===== -->
    <header
      class="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-3 px-5 py-5 backdrop-blur-xl md:px-8"
      style="background: linear-gradient(180deg, rgba(0,0,0,0.9) 0%, rgba(0,0,0,0.6) 80%, transparent 100%);"
    >
      <div class="flex flex-col gap-1">
        <h1 class="text-[28px] font-bold leading-tight tracking-tight text-white">用户管理</h1>
        <p class="text-[13px] text-text-tertiary">管理所有用户账户、角色和权限</p>
      </div>
      <div class="flex items-center gap-2.5">
        <VButton size="sm">
          <template #icon>
            <Icon icon="ri:download-2-line" class="h-full w-full" />
          </template>
          导出
        </VButton>
        <VButton type="primary" size="sm" @click="openCreate">
          <template #icon>
            <Icon icon="ri:user-add-line" class="h-full w-full" />
          </template>
          添加用户
        </VButton>
      </div>
    </header>

    <div class="px-5 pb-8 md:px-8">
      <!-- ===== Stat Cards ===== -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <!-- 总用户数 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">总用户数</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-[#0A84FF]"
              style="background-color: rgba(10,132,255,0.1);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:team-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-white"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ stats.total }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span
              class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[12px] font-semibold"
              style="background-color: rgba(48,209,88,0.1); color: var(--color-state-success);"
            >
              <Icon icon="ri:arrow-up-line" class="text-[11px]" />
              全部
            </span>
            <span class="text-[12px] text-text-tertiary">注册账户</span>
          </div>
        </div>

        <!-- 管理员 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">管理员</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-[#FF375F]"
              style="background-color: rgba(255,55,95,0.1);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:shield-user-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-white"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ stats.adminCount }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span
              class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[12px] font-semibold"
              style="background-color: rgba(255,55,95,0.1); color: #FF375F;"
            >
              <Icon icon="ri:star-line" class="text-[11px]" />
              最高权限
            </span>
          </div>
        </div>

        <!-- 作者 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">作者</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-[#BF5AF2]"
              style="background-color: rgba(191,90,242,0.1);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:quill-pen-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-white"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ stats.authorCount }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span
              class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[12px] font-semibold"
              style="background-color: rgba(191,90,242,0.1); color: #BF5AF2;"
            >
              <Icon icon="ri:edit-2-line" class="text-[11px]" />
              内容编辑
            </span>
          </div>
        </div>

        <!-- 活跃用户 -->
        <div
          class="apple-card-highlight rounded-[14px] border border-border bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[var(--shadow-lg)]"
        >
          <div class="flex items-start justify-between">
            <span class="text-[13px] font-medium text-text-secondary">当前页用户</span>
            <div
              class="relative flex h-10 w-10 items-center justify-center rounded-[12px] text-[#30D158]"
              style="background-color: rgba(48,209,88,0.1);"
            >
              <span
                class="pointer-events-none absolute inset-0 rounded-[13px]"
                style="background: currentColor; opacity: 0.12;"
              ></span>
              <Icon icon="ri:user-star-line" class="relative z-[1] text-[19px]" />
            </div>
          </div>
          <div
            class="mt-3 text-[32px] font-bold leading-none tracking-tight text-white"
            style="font-variant-numeric: tabular-nums;"
          >
            {{ users.length }}
          </div>
          <div class="mt-2.5 flex items-center gap-1.5">
            <span
              class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[12px] font-semibold"
              style="background-color: rgba(48,209,88,0.1); color: var(--color-state-success);"
            >
              <Icon icon="ri:eye-line" class="text-[11px]" />
              本页展示
            </span>
            <span class="text-[12px] text-text-tertiary">共 {{ total }} 条</span>
          </div>
        </div>
      </div>

      <!-- ===== User Table Card ===== -->
      <VCard class="mt-5" :body-class="['!p-0']">
        <!-- Card Header: 分段筛选 + 搜索 -->
        <template #header>
          <div class="flex flex-wrap items-center justify-between gap-3 p-4">
            <div class="apple-segmented">
              <button
                type="button"
                :class="{ active: roleFilter === 'all' }"
                @click="roleFilter = 'all'"
              >
                全部
              </button>
              <button
                type="button"
                :class="{ active: roleFilter === 'admin' }"
                @click="roleFilter = 'admin'"
              >
                管理员
              </button>
              <button
                type="button"
                :class="{ active: roleFilter === 'author' }"
                @click="roleFilter = 'author'"
              >
                作者
              </button>
              <button
                type="button"
                :class="{ active: roleFilter === 'disabled' }"
                @click="roleFilter = 'disabled'"
              >
                已禁用
              </button>
            </div>
            <div class="flex items-center gap-2.5">
              <div class="apple-search" style="width: 220px;">
                <Icon icon="ri:search-line" class="text-text-quaternary" />
                <input v-model="keyword" placeholder="搜索用户..." />
              </div>
              <button
                type="button"
                title="刷新"
                class="inline-flex h-9 items-center gap-1.5 rounded-[10px] border border-border bg-card px-3.5 text-[13px] font-medium text-text-secondary transition-colors hover:border-border-strong hover:bg-secondary hover:text-white"
                @click="refetch()"
              >
                <Icon icon="ri:refresh-line" />
                刷新
              </button>
            </div>
          </div>
        </template>

        <VLoading v-if="isLoading" />
        <VEmpty v-else-if="!filteredUsers.length" title="暂无用户" />

        <template v-else>
          <div class="overflow-x-auto">
            <table class="w-full border-collapse">
              <thead>
                <tr>
                  <th
                    class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                  >
                    用户
                  </th>
                  <th
                    class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                  >
                    角色
                  </th>
                  <th
                    class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                  >
                    状态
                  </th>
                  <th
                    class="whitespace-nowrap border-b border-border px-5 py-3 text-left text-[12px] font-semibold tracking-wide text-text-tertiary"
                  >
                    注册时间
                  </th>
                  <th
                    class="whitespace-nowrap border-b border-border px-5 py-3 text-right text-[12px] font-semibold tracking-wide text-text-tertiary"
                  >
                    操作
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="u in filteredUsers"
                  :key="u.id"
                  class="transition-colors duration-150 hover:bg-white/[0.03]"
                >
                  <td class="border-b border-border px-5 py-3.5 align-middle">
                    <div class="flex items-center gap-3">
                      <img
                        v-if="u.avatar"
                        :src="u.avatar"
                        :alt="u.username"
                        class="h-[38px] w-[38px] shrink-0 rounded-full object-cover"
                      />
                      <div
                        v-else
                        :class="avatarGradient(u.username)"
                        class="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-full text-[14px] font-semibold text-white"
                      >
                        {{ avatarInitial(u) }}
                      </div>
                      <div class="flex min-w-0 flex-col gap-0.5">
                        <span class="text-[14px] font-semibold leading-tight text-foreground">
                          {{ u.display_name || u.username }}
                        </span>
                        <span
                          class="max-w-[220px] truncate text-[12px] leading-tight text-text-tertiary"
                        >
                          {{ u.email || u.username }}
                        </span>
                      </div>
                    </div>
                  </td>
                  <td class="border-b border-border px-5 py-3.5 align-middle">
                    <span
                      class="inline-flex items-center rounded-full px-2.5 py-0.5 text-[12px] font-medium"
                      :style="
                        roleTone(u.role) === 'admin'
                          ? { backgroundColor: 'rgba(255,55,95,0.15)', color: '#FF375F' }
                          : { backgroundColor: 'rgba(191,90,242,0.15)', color: '#BF5AF2' }
                      "
                    >
                      {{ roleLabel(u.role) }}
                    </span>
                  </td>
                  <td class="border-b border-border px-5 py-3.5 align-middle">
                    <span
                      class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-medium"
                      style="background-color: rgba(44,44,46,0.6); color: var(--color-state-success);"
                    >
                      <VStatusDot state="success" />
                      活跃
                    </span>
                  </td>
                  <td
                    class="border-b border-border px-5 py-3.5 align-middle text-[13px] text-text-secondary"
                    style="font-variant-numeric: tabular-nums;"
                  >
                    {{ u.created_at || "—" }}
                  </td>
                  <td class="border-b border-border px-5 py-3.5 text-right align-middle">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        title="编辑"
                        class="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-foreground"
                        @click="openEdit(u)"
                      >
                        <Icon icon="ri:pencil-line" class="text-[16px]" />
                      </button>
                      <button
                        type="button"
                        title="重置密码"
                        class="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-foreground"
                        @click="openResetPwd(u)"
                      >
                        <Icon icon="ri:key-2-line" class="text-[16px]" />
                      </button>
                      <button
                        type="button"
                        title="删除"
                        class="flex h-[30px] w-[30px] items-center justify-center rounded-lg border border-transparent text-text-tertiary transition-colors hover:border-border hover:bg-secondary hover:text-[var(--color-state-error)]"
                        @click="confirmDelete(u)"
                      >
                        <Icon icon="ri:delete-bin-line" class="text-[16px]" />
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="total > size" class="border-t border-border p-4">
            <VPagination :page="page" :size="size" :total="total" @update:page="page = $event" />
          </div>
        </template>
      </VCard>
    </div>

    <!-- 创建用户 -->
    <VModal v-model:visible="createVisible" title="创建用户" :width="520">
      <div class="space-y-4">
        <div>
          <label :class="labelCls">用户名 <span class="text-red-500">*</span></label>
          <input v-model="createForm.username" :class="inputCls" placeholder="登录用户名，唯一" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label :class="labelCls">显示名</label>
            <input v-model="createForm.display_name" :class="inputCls" placeholder="前台展示名" />
          </div>
          <div>
            <label :class="labelCls">邮箱</label>
            <input v-model="createForm.email" :class="inputCls" placeholder="user@example.com" />
          </div>
        </div>
        <div>
          <label :class="labelCls">头像 URL</label>
          <input v-model="createForm.avatar" :class="inputCls" placeholder="https://...（可选）" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label :class="labelCls">初始密码 <span class="text-red-500">*</span></label>
            <input v-model="createForm.password" type="password" :class="inputCls" placeholder="至少 6 位" />
          </div>
          <div>
            <label :class="labelCls">角色</label>
            <select v-model="createForm.role" :class="inputCls">
              <option value="author">作者（可编辑内容）</option>
              <option value="admin">管理员（全部权限）</option>
            </select>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="createVisible = false">取消</VButton>
          <VButton type="primary" :loading="createSaving" @click="submitCreate">创建</VButton>
        </div>
      </template>
    </VModal>

    <!-- 编辑用户 -->
    <VModal v-model:visible="editVisible" title="编辑用户" :width="520">
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label :class="labelCls">显示名</label>
            <input v-model="editForm.display_name" :class="inputCls" />
          </div>
          <div>
            <label :class="labelCls">邮箱</label>
            <input v-model="editForm.email" :class="inputCls" />
          </div>
        </div>
        <div>
          <label :class="labelCls">头像 URL</label>
          <input v-model="editForm.avatar" :class="inputCls" />
        </div>
        <div>
          <label :class="labelCls">角色</label>
          <select v-model="editForm.role" :class="inputCls">
            <option value="author">作者（可编辑内容）</option>
            <option value="admin">管理员（全部权限）</option>
          </select>
          <p v-if="editTarget?.role === 'admin' && editForm.role === 'author'" class="mt-1 text-xs text-amber-500">
            注意：不能降级最后一个管理员
          </p>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="editVisible = false">取消</VButton>
          <VButton type="primary" :loading="editSaving" @click="submitEdit">保存</VButton>
        </div>
      </template>
    </VModal>

    <!-- 重置密码 -->
    <VModal v-model:visible="pwdVisible" title="重置密码" :width="420">
      <div>
        <label :class="labelCls">新密码（{{ pwdTarget?.username }}）<span class="text-red-500">*</span></label>
        <input v-model="pwdForm.password" type="password" :class="inputCls" placeholder="至少 6 位" />
        <p class="mt-2 text-xs text-text-quaternary">
          重置后该用户所有已登录会话将立即失效
        </p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="pwdVisible = false">取消</VButton>
          <VButton type="primary" :loading="pwdSaving" @click="submitResetPwd">重置</VButton>
        </div>
      </template>
    </VModal>
  </div>
</template>
