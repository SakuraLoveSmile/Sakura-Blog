<script setup lang="ts">
import { ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { VButton } from "@/components/ui";

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();

const password = ref("");
const error = ref("");
const loading = ref(false);

async function handleLogin() {
  if (!password.value) {
    error.value = "请输入密码";
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    await auth.login(password.value);
    const redirect = (route.query.redirect as string) || "/dashboard";
    router.push(redirect);
  } catch {
    error.value = "密码错误，请重试";
  } finally {
    loading.value = false;
  }
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

      <!-- 表单 -->
      <form class="flex flex-col gap-3" @submit.prevent="handleLogin">
        <div>
          <input
            v-model="password"
            type="password"
            placeholder="请输入管理密码"
            autocomplete="current-password"
            autofocus
            class="w-full border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            style="border-radius: 4px"
          />
        </div>

        <Transition name="shake-fade">
          <p v-if="error" class="text-xs text-red-600 dark:text-red-400">{{ error }}</p>
        </Transition>

        <VButton type="secondary" block :loading="loading" @click="handleLogin">
          登录
        </VButton>
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
