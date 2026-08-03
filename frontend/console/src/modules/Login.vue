<script setup lang="ts">
import { ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { Icon } from "@iconify/vue";
import { useAuthStore } from "@/stores/auth";
import { authApi } from "@/api";
import { VButton } from "@/components/ui";

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();

// 第一步：用户名 + 密码
const username = ref("");
const password = ref("");
const showPassword = ref(false);
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

// 输入框通用样式：44px 高、聚焦时 primary 绿边 + 淡绿光环（对齐 Halo 登录页焦点态）
const inputCls =
  "h-11 w-full rounded-lg border border-gray-300 bg-white px-3 text-sm text-gray-900 outline-none transition-[border-color,box-shadow] focus:border-primary focus:ring-2 focus:ring-primary/20 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500";
const labelCls = "mb-2 block text-[13px] font-medium text-gray-700 dark:text-gray-300";
</script>

<template>
  <!-- Halo 登录风格：白色背景 + 居中卡片 -->
  <div
    class="flex min-h-screen flex-col items-center justify-center bg-white px-4 py-10 dark:bg-background"
  >
    <div class="w-full max-w-[420px]">
      <!-- Logo -->
      <div class="mb-9 flex justify-center">
        <a href="/" target="_blank" title="访问首页" class="flex items-center gap-2.5">
          <img src="/favicon.svg" alt="Logo" class="h-11 w-11 select-none" />
          <span class="text-3xl font-bold tracking-tight text-gray-900 dark:text-gray-100">
            Console
          </span>
        </a>
      </div>

      <!-- 登录卡片 -->
      <div
        class="rounded-xl border border-gray-200 bg-white p-7 shadow-sm dark:border-border dark:bg-card"
      >
        <!-- 第一步：用户名 + 密码 -->
        <form v-if="step === 1" class="flex flex-col gap-5" @submit.prevent="handleLogin">
          <div>
            <label for="login-username" :class="labelCls">账号</label>
            <input
              id="login-username"
              v-model="username"
              type="text"
              placeholder="用户名"
              autocomplete="username"
              autofocus
              :class="inputCls"
            />
          </div>
          <div>
            <label for="login-password" :class="labelCls">密码</label>
            <div class="relative">
              <input
                id="login-password"
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="请输入管理密码"
                autocomplete="current-password"
                :class="inputCls + ' pr-10'"
              />
              <!-- mousedown.prevent 避免点击眼睛图标时输入框失焦 -->
              <button
                type="button"
                :title="showPassword ? '隐藏密码' : '显示密码'"
                class="absolute top-1/2 right-3 -translate-y-1/2 text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-300"
                @mousedown.prevent
                @click="showPassword = !showPassword"
              >
                <Icon :icon="showPassword ? 'ri:eye-off-line' : 'ri:eye-line'" class="text-lg" />
              </button>
            </div>
          </div>

          <Transition name="shake-fade">
            <p
              v-if="error"
              class="-my-2 flex items-center gap-1 text-xs text-red-600 dark:text-red-400"
            >
              <Icon icon="ri:error-warning-line" class="text-sm" />{{ error }}
            </p>
          </Transition>

          <VButton
            type="secondary"
            size="lg"
            block
            :loading="loading"
            class="login-submit"
            @click="handleLogin"
          >
            登录
          </VButton>
        </form>

        <!-- 第二步：TOTP 验证码 -->
        <form v-else class="flex flex-col gap-5" @submit.prevent="handleVerify2FA">
          <div class="text-center">
            <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">两步验证</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              请输入认证器 App 中的 6 位验证码
            </p>
          </div>
          <div>
            <label for="login-totp" class="sr-only">验证码</label>
            <input
              id="login-totp"
              v-model="code"
              type="text"
              inputmode="numeric"
              pattern="[0-9]*"
              maxlength="6"
              placeholder="6 位验证码"
              autocomplete="one-time-code"
              autofocus
              :class="inputCls + ' text-center text-lg tracking-[0.5em]'"
            />
          </div>

          <Transition name="shake-fade">
            <p
              v-if="codeError"
              class="-my-2 flex items-center justify-center gap-1 text-xs text-red-600 dark:text-red-400"
            >
              <Icon icon="ri:error-warning-line" class="text-sm" />{{ codeError }}
            </p>
          </Transition>

          <VButton
            type="secondary"
            size="lg"
            block
            :loading="codeLoading"
            class="login-submit"
            @click="handleVerify2FA"
          >
            验证并登录
          </VButton>
          <button
            type="button"
            class="self-center text-xs text-gray-400 transition-colors hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300"
            @click="backToStep1"
          >
            返回重新输入
          </button>
        </form>
      </div>

      <!-- 返回网站 -->
      <div class="mt-8 text-center">
        <a
          href="/"
          class="inline-flex items-center gap-1.5 text-sm text-gray-500 transition-colors hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
        >
          <Icon icon="ri:arrow-left-line" class="text-base" />
          返回网站
        </a>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* VButton lg 默认 18px 偏大，贴近参照图按钮字号 */
.login-submit {
  font-size: 0.9375rem;
}

/* 深色模式下 secondary 深蓝黑 (#0e1731) 与卡片底色过近，换用 primary 绿保证可辨识。
   特异性 (0,3,0) 压过 VButton 的 .btn-secondary[data-v] (0,2,0) */
:global(.dark .login-submit.btn-secondary) {
  background: var(--color-primary) !important;
}

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
