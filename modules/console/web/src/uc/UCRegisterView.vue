<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useUCAuthStore } from "./stores/auth";

const router = useRouter();
const auth = useUCAuthStore();
const form = ref({ username: "", password: "", display_name: "", email: "", captcha_answer: "" });
const challengeToken = ref("");
const question = ref("加载中…");
const error = ref("");
const loading = ref(false);

async function loadChallenge() {
  try {
    const challenge = await auth.challenge();
    challengeToken.value = challenge.token;
    question.value = challenge.question;
    form.value.captcha_answer = "";
  } catch (err) {
    error.value = err instanceof Error ? err.message : "验证码加载失败";
  }
}

async function submit() {
  error.value = "";
  loading.value = true;
  try {
    await auth.register({ ...form.value, captcha_token: challengeToken.value });
    await router.replace("/upgrade");
  } catch (err) {
    error.value = err instanceof Error ? err.message : "注册失败";
    await loadChallenge();
  } finally {
    loading.value = false;
  }
}

onMounted(loadChallenge);
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-background px-4 py-10 text-foreground">
    <div class="w-full max-w-md">
      <div class="mb-7 text-center">
        <p class="mb-2 text-xs font-medium text-text-tertiary">注册账户</p>
        <h1 class="text-3xl font-semibold tracking-tight">创建读者账号</h1>
        <p class="mt-2 text-sm text-text-tertiary">注册后可升级为作者，开始管理自己的文章。</p>
      </div>
      <form class="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-7" @submit.prevent="submit">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="uc-field"
            ><span>用户名</span
            ><input v-model="form.username" required minlength="3" maxlength="20" autocomplete="username"
          /></label>
          <label class="uc-field"><span>昵称</span><input v-model="form.display_name" maxlength="32" /></label>
        </div>
        <label class="uc-field mt-4"
          ><span>邮箱</span><input v-model="form.email" type="email" required autocomplete="email"
        /></label>
        <label class="uc-field mt-4"
          ><span>密码</span
          ><input v-model="form.password" type="password" required minlength="6" autocomplete="new-password"
        /></label>
        <div class="mt-4 flex items-end gap-2">
          <label class="uc-field min-w-0 flex-1"
            ><span>验证码：{{ question }}</span
            ><input v-model="form.captcha_answer" inputmode="numeric" required
          /></label>
          <button type="button" class="uc-secondary-button mb-0.5 shrink-0" @click="loadChallenge">换一题</button>
        </div>
        <p
          v-if="error"
          class="mt-3 rounded-xl border border-state-error/30 bg-state-error/10 px-3 py-2 text-sm text-state-error"
        >
          {{ error }}
        </p>
        <button type="submit" class="uc-primary-button mt-5 w-full" :disabled="loading">
          {{ loading ? "创建中…" : "创建账号" }}
        </button>
        <p class="mt-5 text-center text-sm text-text-tertiary">
          已有账号？<RouterLink class="text-primary hover:underline" to="/login">返回登录</RouterLink>
        </p>
      </form>
    </div>
  </div>
</template>

<style scoped>
.uc-field {
  display: block;
  font-size: 0.8rem;
  color: var(--color-text-secondary);
}
.uc-field > span {
  display: block;
  margin-bottom: 0.45rem;
}
.uc-field input {
  width: 100%;
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
  background: var(--color-input);
  padding: 0.7rem 0.8rem;
  color: var(--color-foreground);
  outline: none;
}
.uc-field input:focus {
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
