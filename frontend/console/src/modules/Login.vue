<script setup lang="ts">
import { ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { authApi } from "@/api";
import { VButton } from "@/components/ui";

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();

// 第一步：用户名 + 密码
const username = ref("");
const password = ref("");
const error = ref("");
const loading = ref(false);

// 第二步：TOTP 验证码（require_2fa 时进入）
const step = ref<1 | 2>(1);
const pendingToken = ref("");
const code = ref("");
const codeError = ref("");
const codeLoading = ref(false);

async function handleLogin() {
  if (!password.value) {
    error.value = "请输入密码";
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const data = await auth.login(username.value.trim(), password.value);
    if (data.require_2fa && data.token) {
      // 进入第二步：TOTP 验证
      pendingToken.value = data.token;
      step.value = 2;
      return;
    }
    // 旧格式（无 require_2fa）或直接 ok：登录成功
    const redirect = (route.query.redirect as string) || "/dashboard";
    router.push(redirect);
  } catch (e: any) {
    error.value = e?.response?.data?.error || "用户名或密码错误，请重试";
  } finally {
    loading.value = false;
  }
}

async function handleVerify2FA() {
  if (!/^\d{6}$/.test(code.value.trim())) {
    codeError.value = "请输入 6 位验证码";
    return;
  }
  codeLoading.value = true;
  codeError.value = "";
  try {
    await authApi.verify2FA(pendingToken.value, code.value.trim());
    // 刷新登录态与用户资料后跳转
    await auth.checkAuth();
    const redirect = (route.query.redirect as string) || "/dashboard";
    router.push(redirect);
  } catch (e: any) {
    codeError.value = e?.response?.data?.error || "验证码错误，请重试";
  } finally {
    codeLoading.value = false;
  }
}

function backToStep1() {
  step.value = 1;
  code.value = "";
  codeError.value = "";
  pendingToken.value = "";
}
</script>

<template>
  <!-- Halo 登录风格：浅灰背景 + 居中窄卡片 -->
  <div class="flex h-screen flex-col items-center justify-center bg-white/50 dark:bg-gray-900">
    <div class="flex w-72 flex-col items-stretch justify-center">
      <!-- Logo -->
      <div class="mb-8 flex justify-center">
        <a href="/" target="_blank" title="访问首页" class="flex items-center gap-2.5">
          <img src="/favicon.svg" alt="Logo" class="h-10 w-10 select-none" />
          <span class="text-2xl font-bold text-gray-900 dark:text-gray-100">Console</span>
        </a>
      </div>

      <!-- 第一步：用户名 + 密码 -->
      <form v-if="step === 1" class="flex flex-col gap-3" @submit.prevent="handleLogin">
        <div>
          <input
            v-model="username"
            type="text"
            placeholder="用户名"
            autocomplete="username"
            class="w-full rounded-base border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
          />
        </div>
        <div>
          <input
            v-model="password"
            type="password"
            placeholder="请输入管理密码"
            autocomplete="current-password"
            :autofocus="!username"
            class="w-full rounded-base border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
          />
        </div>

        <Transition name="shake-fade">
          <p v-if="error" class="text-xs text-red-600 dark:text-red-400">{{ error }}</p>
        </Transition>

        <VButton type="secondary" block :loading="loading" @click="handleLogin">
          登录
        </VButton>
      </form>

      <!-- 第二步：TOTP 验证码 -->
      <form v-else class="flex flex-col gap-3" @submit.prevent="handleVerify2FA">
        <p class="text-sm text-gray-600 dark:text-gray-300">请输入认证器 App 中的 6 位验证码</p>
        <div>
          <input
            v-model="code"
            type="text"
            inputmode="numeric"
            pattern="[0-9]*"
            maxlength="6"
            placeholder="6 位验证码"
            autocomplete="one-time-code"
            autofocus
            class="w-full rounded-base border border-gray-300 bg-white px-3 py-2.5 text-center text-lg tracking-[0.5em] text-gray-900 outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
          />
        </div>

        <Transition name="shake-fade">
          <p v-if="codeError" class="text-xs text-red-600 dark:text-red-400">{{ codeError }}</p>
        </Transition>

        <VButton type="secondary" block :loading="codeLoading" @click="handleVerify2FA">
          验证并登录
        </VButton>
        <button
          type="button"
          class="text-xs text-gray-400 transition-colors hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300"
          @click="backToStep1"
        >
          返回重新输入
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.shake-fade-enter-active {
  animation: shake 0.4s ease;
}
.shake-fade-leave-active {
  transition: opacity 0.15s ease;
}
.shake-fade-leave-to {
  opacity: 0;
}
@keyframes shake {
  0%,
  100% {
    transform: translateX(0);
  }
  25% {
    transform: translateX(-4px);
  }
  75% {
    transform: translateX(4px);
  }
}
</style>
