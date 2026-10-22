<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { usersApi, type UserItem } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import {
  VPageHeader,
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
import ConsoleStatsCard from "@/components/ConsoleStatsCard.vue";

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

const {
  data: queryData,
  isLoading,
  refetch,
} = useQuery({
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
  queryClient.invalidateQueries({ queryKey: queryKeys.users() });
}

// ---------- 注册邀请码 ----------
const invitationVisible = ref(false);
const invitationSaving = ref(false);
const invitationDays = ref<1 | 7 | 30>(7);
const invitationCode = ref("");
const invitationExpiresAt = ref("");
let invitationRequest = 0;
function clearInvitation() {
  invitationRequest++;
  invitationCode.value = "";
  invitationExpiresAt.value = "";
  invitationSaving.value = false;
}
watch(invitationVisible, (visible) => {
  if (!visible) clearInvitation();
});
onBeforeUnmount(clearInvitation);
function openInvitation() {
  clearInvitation();
  invitationDays.value = 7;
  invitationVisible.value = true;
}
async function generateInvitation() {
  if (invitationSaving.value || invitationCode.value) return;
  const request = ++invitationRequest;
  invitationSaving.value = true;
  try {
    const { data } = await usersApi.createInvitation({ expires_in_days: invitationDays.value });
    if (request !== invitationRequest || !invitationVisible.value) return;
    invitationCode.value = data.code;
    invitationExpiresAt.value = new Date(data.expires_at).toLocaleString();
  } catch {
    // API transport 已显示具体错误。
  } finally {
    if (request === invitationRequest) invitationSaving.value = false;
  }
}
async function copyInvitation() {
  try {
    await navigator.clipboard.writeText(invitationCode.value);
    Toast.success("邀请码已复制");
  } catch {
    Toast.error("复制失败，请选择邀请码手动复制");
  }
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
  if (!createForm.value.password || createForm.value.password.length < 6) return Toast.error("密码至少 6 位");
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
  } catch {
  } finally {
    createSaving.value = false;
  }
}

// ---------- 编辑用户 ----------
const editVisible = ref(false);
const editSaving = ref(false);
const editTarget = ref<UserItem | null>(null);
const editForm = ref({ display_name: "", email: "", avatar: "", role: "author" as "admin" | "author" | "reader" });

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
  } catch {
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
  if (!pwdForm.value.password || pwdForm.value.password.length < 6) return Toast.error("密码至少 6 位");
  pwdSaving.value = true;
  try {
    await usersApi.resetPassword(pwdTarget.value.id, pwdForm.value.password);
    pwdVisible.value = false;
    Toast.success("密码已重置，该用户旧登录态已失效");
  } catch {
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
      } catch {}
    },
  });
}

// ---------- 展示辅助 ----------
function roleLabel(role: string): string {
  if (role === "admin") return "管理员";
  if (role === "author") return "作者";
  if (role === "reader") return "读者";
  if (role === "commenter") return "评论者";
  return role;
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
  <div class="min-h-full">
    <!-- ===== Page Header ===== -->
    <VPageHeader title="用户管理" subtitle="管理所有用户账户、角色和权限">
      <template #icon>
        <Icon icon="ri:user-star-line" class="text-xl text-primary sm:text-2xl" />
      </template>
      <template #actions>
        <VButton v-permission="'users:manage'" size="sm" @click="openInvitation">
          <template #icon><Icon icon="ri:key-2-line" class="h-4 w-4" /></template>
          生成邀请码
        </VButton>
        <VButton v-permission="'users:manage'" type="primary" size="sm" @click="openCreate">
          <template #icon>
            <Icon icon="ri:user-add-line" class="h-4 w-4" />
          </template>
          <span class="hidden sm:inline">添加用户</span>
        </VButton>
      </template>
    </VPageHeader>

    <div class="px-4 pb-8 sm:px-5 md:px-8">
      <!-- ===== Stat Cards ===== -->
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <ConsoleStatsCard
          label="总用户数"
          :value="stats.total"
          sub="注册账户"
          icon="ri:team-line"
          color="primary"
        />
        <ConsoleStatsCard
          label="系统管理员"
          :value="stats.adminCount"
          sub="管理权限"
          icon="ri:shield-user-line"
          color="primary"
        />
        <ConsoleStatsCard
          label="专栏作者"
          :value="stats.authorCount"
          sub="内容编辑"
          icon="ri:quill-pen-line"
          color="purple"
        />
        <ConsoleStatsCard
          label="当前页用户"
          :value="users.length"
          :sub="'共 ' + total + ' 条'"
          icon="ri:user-star-line"
          color="success"
          value-class="text-[#30D158]"
        />
      </div>

      <!-- ===== User Table Card ===== -->
      <VCard class="mt-5" :body-class="['!p-0']">
        <!-- Card Header: 分段筛选 + 搜索 -->
        <template #header>
          <div
            class="flex flex-col gap-3 p-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-3 sm:p-4"
          >
            <div class="tabs-scroll apple-segmented -mx-1 px-1 sm:mx-0 sm:px-0">
              <button type="button" :class="{ active: roleFilter === 'all' }" @click="roleFilter = 'all'">全部</button>
              <button type="button" :class="{ active: roleFilter === 'admin' }" @click="roleFilter = 'admin'">
                管理员
              </button>
              <button type="button" :class="{ active: roleFilter === 'author' }" @click="roleFilter = 'author'">
                作者
              </button>
              <button type="button" :class="{ active: roleFilter === 'disabled' }" @click="roleFilter = 'disabled'">
                已禁用
              </button>
            </div>
            <div class="flex w-full items-center gap-2 sm:w-auto">
              <div class="flex-1 sm:w-[220px] sm:flex-none">
                <div class="apple-search w-full">
                  <Icon icon="ri:search-line" class="text-text-quaternary" />
                  <input v-model="keyword" placeholder="搜索用户..." />
                </div>
              </div>
              <button
                type="button"
                title="刷新"
                class="hidden h-9 items-center gap-1.5 rounded-full border border-border bg-card px-3.5 text-[13px] font-semibold text-text-secondary transition-colors hover:border-border-strong hover:bg-secondary hover:text-foreground sm:inline-flex"
                @click="refetch()"
              >
                <Icon icon="ri:refresh-line" />
                刷新
              </button>
              <button
                type="button"
                title="刷新"
                class="inline-flex h-8 w-8 items-center justify-center rounded-full border border-border bg-card text-text-secondary transition-colors hover:border-border-strong hover:bg-secondary hover:text-foreground sm:hidden"
                @click="refetch()"
              >
                <Icon icon="ri:refresh-line" class="text-base" />
              </button>
            </div>
          </div>
        </template>

        <VLoading v-if="isLoading" />
        <VEmpty v-else-if="!filteredUsers.length" title="暂无用户" />

        <template v-else>
          <!-- 桌面端表格 -->
          <div class="hidden sm:block">
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
                          loading="lazy"
                          class="h-[38px] w-[38px] shrink-0 rounded-full object-cover"
                        />
                        <div
                          v-else
                          :class="avatarGradient(u.username)"
                          class="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-full text-[14px] font-semibold text-foreground"
                        >
                          {{ avatarInitial(u) }}
                        </div>
                        <div class="flex min-w-0 flex-col gap-0.5">
                          <span class="text-[14px] font-semibold leading-tight text-foreground">
                            {{ u.display_name || u.username }}
                          </span>
                          <span class="max-w-[220px] truncate text-[12px] leading-tight text-text-tertiary">
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
                            ? {
                                backgroundColor: 'color-mix(in srgb, var(--color-primary) 15%, transparent)',
                                color: 'var(--color-primary)',
                              }
                            : {
                                backgroundColor: 'color-mix(in srgb, var(--color-secondary) 80%, transparent)',
                                color: 'var(--color-text-secondary)',
                              }
                        "
                      >
                        {{ roleLabel(u.role) }}
                      </span>
                    </td>
                    <td class="border-b border-border px-5 py-3.5 align-middle">
                      <span
                        class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-medium"
                        style="
                          background-color: color-mix(in srgb, var(--color-accent) 60%, transparent);
                          color: var(--color-state-success);
                        "
                      >
                        <VStatusDot state="success" />
                        活跃
                      </span>
                    </td>
                    <td
                      class="border-b border-border px-5 py-3.5 align-middle text-[13px] text-text-secondary"
                      style="font-variant-numeric: tabular-nums"
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
          </div>

          <!-- 移动端卡片列表 -->
          <div class="sm:hidden user-card-list">
            <div
              v-for="u in filteredUsers"
              :key="u.id"
              class="user-card border-b border-border px-3 py-3.5 last:border-b-0"
            >
              <div class="flex items-start gap-3">
                <!-- 头像 -->
                <img
                  v-if="u.avatar"
                  :src="u.avatar"
                  :alt="u.username"
                  loading="lazy"
                  class="h-10 w-10 shrink-0 rounded-full object-cover"
                />
                <div
                  v-else
                  :class="avatarGradient(u.username)"
                  class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-foreground"
                >
                  {{ avatarInitial(u) }}
                </div>

                <!-- 主体 -->
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span class="truncate text-sm font-semibold text-foreground">
                      {{ u.display_name || u.username }}
                    </span>
                    <span
                      class="inline-flex items-center rounded-full px-1.5 py-0.5 text-[11px] font-medium"
                      :style="
                        roleTone(u.role) === 'admin'
                          ? {
                              backgroundColor: 'color-mix(in srgb, var(--color-primary) 15%, transparent)',
                              color: 'var(--color-primary)',
                            }
                          : {
                              backgroundColor: 'color-mix(in srgb, var(--color-secondary) 80%, transparent)',
                              color: 'var(--color-text-secondary)',
                            }
                      "
                    >
                      {{ roleLabel(u.role) }}
                    </span>
                    <span class="ml-auto flex items-center gap-1 text-[11px] text-text-tertiary">
                      <span class="h-1.5 w-1.5 rounded-full bg-state-success"></span>
                      活跃
                    </span>
                  </div>
                  <div class="mt-0.5 truncate text-[12px] text-text-tertiary">
                    {{ u.email || u.username }}
                  </div>
                  <div class="mt-1 text-[11px] text-text-quaternary">注册：{{ u.created_at || "—" }}</div>

                  <!-- 操作按钮 -->
                  <div class="mt-2.5 flex items-center gap-1">
                    <button
                      type="button"
                      title="编辑"
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-secondary/50 text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
                      @click="openEdit(u)"
                    >
                      <Icon icon="ri:pencil-line" class="text-sm" />
                    </button>
                    <button
                      type="button"
                      title="重置密码"
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-secondary/50 text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
                      @click="openResetPwd(u)"
                    >
                      <Icon icon="ri:key-2-line" class="text-sm" />
                    </button>
                    <button
                      type="button"
                      title="删除"
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-secondary/50 text-text-tertiary transition-colors hover:bg-secondary hover:text-state-error"
                      @click="confirmDelete(u)"
                    >
                      <Icon icon="ri:delete-bin-line" class="text-sm" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="total > size" class="border-t border-border px-3 py-3 sm:px-4 sm:py-4">
            <div class="flex flex-col items-center gap-2 sm:flex-row sm:items-center sm:justify-end">
              <VPagination :page="page" :size="size" :total="total" @update:page="page = $event" />
            </div>
          </div>
        </template>
      </VCard>
    </div>

    <!-- 注册邀请码：明文仅保留在当前弹窗中 -->
    <VModal v-model:visible="invitationVisible" title="生成注册邀请码" :width="520">
      <div class="space-y-4">
        <p class="text-sm text-text-secondary">邀请码只能注册一次，注册成功后自动失效。</p>
        <div v-if="!invitationCode">
          <label for="invitation-days" :class="labelCls">有效期</label>
          <select id="invitation-days" v-model="invitationDays" :class="inputCls" :disabled="invitationSaving">
            <option :value="1">1 天</option>
            <option :value="7">7 天（默认）</option>
            <option :value="30">30 天</option>
          </select>
        </div>
        <div v-else>
          <label for="invitation-code" :class="labelCls">邀请码</label>
          <input id="invitation-code" :value="invitationCode" :class="inputCls" readonly autocomplete="off" />
          <p class="mt-2 text-xs text-text-tertiary">到期时间：{{ invitationExpiresAt }}</p>
          <p class="mt-2 text-xs text-text-tertiary">请复制并交给注册者，关闭后不再显示。</p>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <VButton @click="invitationVisible = false">关闭</VButton>
          <VButton v-if="invitationCode" type="primary" @click="copyInvitation">复制邀请码</VButton>
          <VButton v-else type="primary" :loading="invitationSaving" @click="generateInvitation">生成</VButton>
        </div>
      </template>
    </VModal>

    <!-- 创建用户 -->
    <VModal v-model:visible="createVisible" title="创建用户" :width="520">
      <div class="space-y-4">
        <div>
          <label :class="labelCls">用户名 <span class="text-state-error">*</span></label>
          <input v-model="createForm.username" :class="inputCls" placeholder="登录用户名，唯一" />
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
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
          <AttachmentField v-model="createForm.avatar" placeholder="https://...（可选）" show-thumb />
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label :class="labelCls">初始密码 <span class="text-state-error">*</span></label>
            <input v-model="createForm.password" type="password" :class="inputCls" placeholder="至少 6 位" />
          </div>
          <div>
            <label :class="labelCls">角色</label>
            <select v-model="createForm.role" :class="inputCls">
              <option value="author">作者（可编辑内容）</option>
              <option value="admin">管理员（全部权限）</option>
              <option value="reader">读者（仅个人中心）</option>
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
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
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
          <AttachmentField v-model="editForm.avatar" placeholder="https://..." show-thumb />
        </div>
        <div>
          <label :class="labelCls">角色</label>
          <select v-model="editForm.role" :class="inputCls">
            <option value="author">作者（可编辑内容）</option>
            <option value="admin">管理员（全部权限）</option>
            <option value="reader">读者（仅个人中心）</option>
          </select>
          <p v-if="editTarget?.role === 'admin' && editForm.role === 'author'" class="mt-1 text-xs text-state-warning">
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
        <label :class="labelCls">新密码（{{ pwdTarget?.username }}）<span class="text-state-error">*</span></label>
        <input v-model="pwdForm.password" type="password" :class="inputCls" placeholder="至少 6 位" />
        <p class="mt-2 text-xs text-text-quaternary">重置后该用户所有已登录会话将立即失效</p>
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

<style scoped>
/* 分段 Tab 横向滚动，不显示滚动条（无条件生效：任何宽度下标签溢出都可横滚） */
.tabs-scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}
.tabs-scroll::-webkit-scrollbar {
  display: none;
}
</style>
