<script setup lang="ts">
import { ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { Icon } from "@iconify/vue";
import { useAuthStore } from "@/stores/auth";
import { authApi } from "@/api";
import { resolveSafeRedirect } from "@/utils/redirect";

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
  if (!username.value.trim()) {
    error.value = "请输入账号";
    return;
  }
  if (!password.value) {
    error.value = "请输入密码";
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const data = await auth.login(username.value.trim(), password.value);
    if (data.require_2fa && data.token) {
      pendingToken.value = data.token;
      step.value = 2;
      return;
    }
    const redirect = resolveSafeRedirect(route.query.redirect, route.query.from);
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
    await auth.checkAuth();
    const redirect = resolveSafeRedirect(route.query.redirect, route.query.from);
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
  <div class="login-page">
    <div class="login-card">
      <!-- 第一步：用户名 + 密码 -->
      <template v-if="step === 1">
        <!-- 顶部 Logo 区 -->
        <div class="login-header">
          <div class="logo-icon">
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="white"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
              <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
              <path d="M8 7h6" />
              <path d="M8 11h8" />
            </svg>
          </div>
          <h1 class="login-title">登录 Console</h1>
          <p class="login-subtitle">管理内容与站点设置</p>
        </div>

        <!-- 登录表单 -->
        <form class="login-form" @submit.prevent="handleLogin">
          <div class="form-group">
            <label class="form-label" for="console-username">账号</label>
            <input
              v-model="username"
              id="console-username"
              type="text"
              class="apple-input"
              placeholder="请输入用户名"
              autocomplete="username"
              autofocus
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="console-password">密码</label>
            <div class="password-wrapper">
              <input
                v-model="password"
                id="console-password"
                :type="showPassword ? 'text' : 'password'"
                class="apple-input"
                placeholder="请输入密码"
                autocomplete="current-password"
              />
              <button
                type="button"
                class="password-toggle"
                :title="showPassword ? '隐藏密码' : '显示密码'"
                @mousedown.prevent
                @click="showPassword = !showPassword"
              >
                <Icon v-if="showPassword" icon="ri:eye-off-line" :width="20" :height="20" />
                <Icon v-else icon="ri:eye-line" :width="20" :height="20" />
              </button>
            </div>
          </div>

          <!-- 错误提示 -->
          <Transition name="shake-fade">
            <p v-if="error" class="error-text">
              <Icon icon="ri:error-warning-fill" :width="16" :height="16" />
              {{ error }}
            </p>
          </Transition>

          <!-- 登录按钮 -->
          <button type="submit" class="btn-primary" :disabled="loading">
            <Icon v-if="loading" icon="ri:loader-4-line" :width="20" :height="20" class="loading-icon" />
            {{ loading ? "登录中..." : "登 录" }}
          </button>
        </form>
      </template>

      <!-- 第二步：TOTP 验证码 -->
      <template v-else>
        <div class="login-header">
          <div class="logo-icon totp-icon">
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="white"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </div>
          <h1 class="login-title">两步验证</h1>
          <p class="login-subtitle">请输入认证器 App 中的 6 位验证码</p>
        </div>

        <form class="login-form" @submit.prevent="handleVerify2FA">
          <div class="form-group">
            <label class="form-label sr-only">验证码</label>
            <input
              v-model="code"
              type="text"
              inputmode="numeric"
              pattern="[0-9]*"
              maxlength="6"
              class="apple-input totp-input"
              placeholder="6 位验证码"
              autocomplete="one-time-code"
              autofocus
            />
          </div>

          <Transition name="shake-fade">
            <p v-if="codeError" class="error-text">
              <Icon icon="ri:error-warning-fill" :width="16" :height="16" />
              {{ codeError }}
            </p>
          </Transition>

          <button type="submit" class="btn-primary" :disabled="codeLoading">
            <Icon v-if="codeLoading" icon="ri:loader-4-line" :width="20" :height="20" class="loading-icon" />
            {{ codeLoading ? "验证中..." : "验证并登录" }}
          </button>

          <button type="button" class="back-link" @click="backToStep1">
            <Icon icon="ri:arrow-left-line" :width="16" :height="16" />
            返回重新输入
          </button>
        </form>
      </template>
    </div>

    <!-- 返回网站 -->
    <div class="back-to-site">
      <a href="/" class="site-link">
        <Icon icon="ri:arrow-left-line" :width="16" :height="16" />
        返回网站
      </a>
    </div>

  </div>
</template>

<style scoped>
.login-page {
  position: fixed;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: var(--color-background);
  padding: 20px;
  box-sizing: border-box;
  z-index: 9999;
  overflow-y: auto;
}

.login-card {
  width: 100%;
  max-width: 360px;
  background-color: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: none;
  padding: 32px;
  overflow: visible;
}

.login-header {
  text-align: left;
  margin-bottom: 28px;
}

.logo-icon {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  background: var(--color-accent);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 0 20px;
}

.logo-icon.totp-icon {
  background: var(--color-state-success);
}

.login-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--color-foreground);
  letter-spacing: -0.01em;
  margin: 0;
}

.login-subtitle {
  font-size: 14px;
  color: var(--color-muted-foreground);
  margin: 8px 0 0 0;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
}

.form-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-muted-foreground);
  margin-bottom: 6px;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}

.apple-input {
  width: 100%;
  height: 44px;
  padding: 0 16px;
  background-color: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: 14px;
  color: var(--color-foreground);
  transition: border-color var(--duration-fast) var(--ease-standard), box-shadow var(--duration-fast) var(--ease-standard);
  outline: none;
  box-sizing: border-box;
}

.apple-input::placeholder {
  color: var(--color-muted-foreground);
}

.apple-input:focus {
  border-color: var(--color-primary-active);
}

.password-wrapper {
  position: relative;
}

.password-wrapper .apple-input {
  padding-right: 48px;
}

.password-toggle {
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 44px;
  height: 44px;
  color: var(--color-text-secondary);
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-standard);
  display: flex;
  align-items: center;
  justify-content: center;
}

.password-toggle:hover {
  color: var(--color-muted-foreground);
}

.error-text {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--color-state-error);
  margin: 0;
}

.btn-primary {
  width: 100%;
  height: 44px;
  background-color: var(--color-primary);
  color: var(--color-primary-foreground);
  font-size: 15px;
  font-weight: 600;
  border-radius: var(--radius-sm);
  border: none;
  cursor: pointer;
  transition: background-color var(--duration-normal) var(--ease-standard);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  letter-spacing: 0.02em;
  font-family: inherit;
}

.btn-primary:hover:not(:disabled) {
  filter: brightness(0.96);
}

.btn-primary:active:not(:disabled) {
  background-color: var(--color-primary-active);
}

.btn-primary:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.loading-icon {
  flex-shrink: 0;
}

.totp-input {
  text-align: center;
  font-size: 18px;
  letter-spacing: 0.5em;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.back-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: none;
  border: none;
  color: var(--color-muted-foreground);
  font-size: 13px;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-standard);
  padding: 8px;
  margin-top: -8px;
  font-family: inherit;
}

.back-link:hover {
  color: var(--color-text-secondary);
}

.back-to-site {
  margin-top: 32px;
  text-align: center;
}

.site-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--color-muted-foreground);
  text-decoration: none;
  font-size: 14px;
  transition: color var(--duration-fast) var(--ease-standard);
}

.site-link:hover {
  color: var(--color-text-secondary);
}

.shake-fade-enter-active {
  animation: shake 0.4s ease;
}
.shake-fade-leave-active {
  transition: opacity var(--duration-fast) var(--ease-standard);
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

/* ===== 移动端适配 ===== */
@media (max-width: 400px) {
  .login-page {
    padding: 12px;
  }

  .login-card {
    padding: 28px 22px;
    border-radius: 14px;
  }

  .login-header {
    margin-bottom: 24px;
  }

  .logo-icon {
    width: 48px;
    height: 48px;
    margin-bottom: 16px;
  }
  .logo-icon svg {
    width: 22px !important;
    height: 22px !important;
  }

  .login-title {
    font-size: 20px;
  }

  .login-subtitle {
    font-size: 13px;
    margin-top: 6px !important;
  }

  .login-form {
    gap: 16px;
  }

  .apple-input {
    height: 42px;
    font-size: 14px;
  }

  .btn-primary {
    height: 42px;
    font-size: 14px;
  }

  .totp-input {
    font-size: 16px;
    letter-spacing: 0.4em;
  }

  .back-to-site {
    margin-top: 20px;
  }
}
</style>
