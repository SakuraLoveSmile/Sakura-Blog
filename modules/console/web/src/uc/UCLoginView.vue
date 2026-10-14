<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { useUCAuthStore } from "./stores/auth";

const router = useRouter();
const route = useRoute();
const auth = useUCAuthStore();
const username = ref("");
const password = ref("");
const code = ref("");
const error = ref("");
const loading = ref(false);

async function submit() {
  error.value = "";
  loading.value = true;
  try {
    const user = await auth.login(username.value, password.value, code.value.trim() || undefined);
    await router.replace(user.role === "reader" ? "/upgrade" : String(route.query.from || "/posts"));
  } catch (err) {
    error.value = err instanceof Error ? err.message : "登录失败";
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-background px-4 py-10 text-foreground">
    <div class="w-full max-w-md">
      <div class="mb-8 text-center">
        <div
          class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground"
        >
          <Icon icon="ri:user-3-line" class="h-7 w-7" />
        </div>
        <p class="mb-2 text-xs font-medium text-text-tertiary">用户中心</p>
        <h1 class="text-3xl font-semibold tracking-tight">欢迎回来</h1>
        <p class="mt-2 text-sm text-text-tertiary">登录后管理你的文章与附件。</p>
      </div>
      <form class="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-7" @submit.prevent="submit">
        <label class="uc-label" for="uc-username">用户名</label>
        <input id="uc-username" v-model="username" class="uc-input" autocomplete="username" required />
        <label class="uc-label mt-4" for="uc-password">密码</label>
        <input
          id="uc-password"
          v-model="password"
          class="uc-input"
          type="password"
          autocomplete="current-password"
          required
        />
        <label class="uc-label mt-4" for="uc-login-code">两步验证码或恢复码（已开启时填写）</label>
        <input
          id="uc-login-code"
          v-model="code"
          class="uc-input"
          pattern="[0-9]{6}|[A-Za-z0-9]{8}"
          maxlength="8"
          autocomplete="one-time-code"
        />
        <p
          v-if="error"
          class="mt-3 rounded-xl border border-state-error/30 bg-state-error/10 px-3 py-2 text-sm text-state-error"
        >
          {{ error }}
        </p>
        <button type="submit" class="uc-primary-button mt-5 w-full" :disabled="loading">
          {{ loading ? "登录中…" : "登录" }}
        </button>
        <p class="mt-5 text-center text-sm text-text-tertiary">
          还没有账号？<RouterLink class="text-primary hover:underline" to="/register">创建账号</RouterLink>
        </p>
      </form>
    </div>
  </div>
</template>

<style scoped>
.uc-label {
  display: block;
  margin-bottom: 0.45rem;
  font-size: 0.8rem;
  color: var(--color-text-secondary);
}
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
.uc-primary-button {
  min-height: 44px;
  border-radius: 9999px;
  background: var(--color-primary);
  color: var(--color-primary-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  transition: background 150ms var(--ease-standard);
}
.uc-primary-button:hover {
  background: var(--color-primary-hover);
}
.uc-primary-button:disabled {
  cursor: wait;
  opacity: 0.6;
}
</style>
