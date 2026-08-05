<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { usersApi, type UserItem } from "@/api";
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
  VTag,
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

function roleLabel(role: string): string {
  return role === "admin" ? "管理员" : "作者";
}

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const labelCls = "block text-xs text-gray-500 mb-1 dark:text-gray-400";
</script>

<template>
  <div>
    <VPageHeader title="用户">
      <template #icon>
        <Icon icon="ri:user-star-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <VButton type="primary" size="md" @click="openCreate">
          <Icon icon="ri:user-add-line" class="mr-1" /> 创建用户
        </VButton>
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
      <VCard :body-class="['!p-0']">
        <!-- 搜索头部 -->
        <template #header>
          <div class="block w-full bg-gray-50 px-4 py-3 dark:bg-gray-700/50">
            <div class="flex items-center justify-between gap-3">
              <div class="relative flex-1 max-w-xs">
                <Icon
                  icon="ri:search-line"
                  class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
                />
                <input
                  v-model="keyword"
                  class="w-full rounded-lg border border-gray-200 bg-white py-2 pl-9 pr-3 text-sm focus:border-primary focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  placeholder="搜索用户名 / 显示名 / 邮箱"
                />
              </div>
              <div
                class="group cursor-pointer rounded p-1 hover:bg-gray-200 dark:hover:bg-gray-600"
                @click="refetch()"
              >
                <Icon icon="ri:refresh-line" class="h-4 w-4 text-gray-600 group-hover:text-gray-900 dark:text-gray-300 dark:group-hover:text-gray-100" />
              </div>
            </div>
          </div>
        </template>

        <VLoading v-if="isLoading" />
        <VEmpty v-else-if="!users.length" title="暂无用户" />

        <VEntityContainer v-else>
          <VEntity v-for="u in users" :key="u.id">
            <template #start>
              <VEntityField>
                <template #title>
                  <span class="flex items-center gap-2">
                    {{ u.username }}
                    <VTag :color="u.role === 'admin' ? 'primary' : 'default'" size="sm">
                      {{ roleLabel(u.role) }}
                    </VTag>
                  </span>
                </template>
                <template #description>
                  {{ u.display_name || "—" }}
                </template>
              </VEntityField>
              <VEntityField title="邮箱" :description="u.email || '—'" />
              <VEntityField title="创建时间" :description="u.created_at || '—'" />
            </template>
            <template #end>
              <div class="flex items-center gap-2">
                <VButton size="xs" @click="openEdit(u)">
                  <Icon icon="ri:edit-line" class="mr-1" /> 编辑
                </VButton>
                <VButton size="xs" @click="openResetPwd(u)">
                  <Icon icon="ri:key-2-line" class="mr-1" /> 重置密码
                </VButton>
                <VButton size="xs" type="danger" @click="confirmDelete(u)">
                  <Icon icon="ri:delete-bin-line" class="mr-1" /> 删除
                </VButton>
              </div>
            </template>
          </VEntity>
        </VEntityContainer>

        <div v-if="total > size" class="border-t border-gray-100 p-4 dark:border-gray-700">
          <VPagination :page="page" :size="size" :total="total" @update:page="page = $event" />
        </div>
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
        <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">
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
