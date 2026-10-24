<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useUCAuthStore } from "./stores/auth";

const router = useRouter();
const auth = useUCAuthStore();
const password = ref("");
const error = ref("");
const loading = ref(false);

async function submit() {
  error.value = "";
  loading.value = true;
  try {
    await auth.upgrade(password.value);
    await router.replace("/posts");
  } catch (err) {
    error.value = err instanceof Error ? err.message : "升级失败";
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="mx-auto max-w-xl">
    <div class="rounded-2xl border border-border bg-card p-6 shadow-sm sm:p-9">
      <p class="mb-2 text-xs font-medium text-text-tertiary">申请作者权限</p>
      <h1 class="text-3xl font-semibold tracking-tight">升级为作者</h1>
      <p class="mt-3 max-w-lg text-sm leading-6 text-text-secondary">
        作者可以使用受限编辑器发布自己的文章，并管理自己上传的附件。请输入当前密码确认升级。
      </p>
      <form class="mt-7" @submit.prevent="submit">
        <label class="block text-sm text-text-secondary" for="upgrade-password">当前密码</label>
        <input
          id="upgrade-password"
          v-model="password"
          class="uc-input mt-2"
          type="password"
          required
          autocomplete="current-password"
        />
        <p
          v-if="error"
          class="mt-3 rounded-xl border border-state-error/30 bg-state-error/10 px-3 py-2 text-sm text-state-error"
        >
          {{ error }}
        </p>
        <div class="mt-6 flex flex-wrap gap-2">
          <button type="submit" class="uc-primary-button" :disabled="loading">
            {{ loading ? "升级中…" : "确认升级" }}
          </button>
          <button type="button" class="uc-secondary-button" @click="router.push('/profile')">稍后处理</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.uc-input {
  width: 100%;
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
  background: var(--color-input);
  padding: 0.7rem 0.8rem;
  color: var(--color-foreground);
  outline: none;
}
.uc-input:focus {
  border-color: var(--color-ring);
}
.uc-primary-button,
.uc-secondary-button {
  min-height: 44px;
  border-radius: 9999px;
  padding: 0.65rem 1rem;
  font-size: 0.875rem;
  font-weight: 600;
}
.uc-primary-button {
  background: var(--color-primary);
  color: var(--color-primary-foreground);
}
.uc-primary-button:hover {
  background: var(--color-primary-hover);
}
.uc-primary-button:disabled {
  cursor: wait;
  opacity: 0.6;
}
.uc-secondary-button {
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
}
.uc-secondary-button:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
</style>
