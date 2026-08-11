<script setup lang="ts">
import { ref, onUnmounted, nextTick } from "vue";
import { useRouter, useRoute } from "vue-router";
import { Icon } from "@iconify/vue";
import { useAuthStore } from "@/stores/auth";
import { authApi } from "@/api";

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();

// 第一步：用户名 + 密码
const username = ref("");
const password = ref("");
const showPassword = ref(false);
const rememberMe = ref(true);
const error = ref("");
const loading = ref(false);

// 第二步：TOTP 验证码（require_2fa 时进入）
const step = ref<1 | 2>(1);
const pendingToken = ref("");
const code = ref("");
const codeError = ref("");
const codeLoading = ref(false);

// 气泡提示
const activeTooltip = ref<string | null>(null);
const tooltipPos = ref({ x: 0, y: 0 });
const btnRefs = ref<Record<string, HTMLButtonElement | null>>({});
let tooltipTimer: ReturnType<typeof setTimeout> | null = null;

const funnyMessages: Record<string, string> = {
  Apple: "🍎 Apple 登录还在和库克谈判中，再等等~",
  Google: "🔍 Google 登录在墙外面壁思过，暂时回不来 🏃",
  GitHub: "🐙 GitHub 登录的 PR 还在 review，老板说别急 😅",
};

function showTooltip(provider: string) {
  const btn = btnRefs.value[provider];
  if (btn) {
    const rect = btn.getBoundingClientRect();
    tooltipPos.value = {
      x: rect.left + rect.width / 2,
      y: rect.top,
    };
  }
  if (tooltipTimer) clearTimeout(tooltipTimer);
  activeTooltip.value = provider;
  tooltipTimer = setTimeout(() => {
    activeTooltip.value = null;
  }, 2500);
}

function hideTooltip() {
  if (tooltipTimer) {
    clearTimeout(tooltipTimer);
    tooltipTimer = null;
  }
  activeTooltip.value = null;
}

function handleSocialLogin(provider: string, e: MouseEvent) {
  e.stopPropagation();
  nextTick(() => showTooltip(provider));
}

// 点击页面其他地方关闭气泡
function handlePageClick(e: MouseEvent) {
  const target = e.target as HTMLElement;
  if (!target.closest(".social-btn-wrap") && !target.closest(".bubble-tip") && activeTooltip.value) {
    hideTooltip();
  }
}

onUnmounted(() => {
  if (tooltipTimer) clearTimeout(tooltipTimer);
});

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

function setBtnRef(el: any, provider: string) {
  btnRefs.value[provider] = el;
}
</script>

<template>
  <div class="login-page" @click="handlePageClick">
    <div class="login-card">
      <!-- 第一步：用户名 + 密码 -->
      <template v-if="step === 1">
        <!-- 顶部 Logo 区 -->
        <div class="login-header">
          <div class="logo-icon">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/>
              <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>
              <path d="M8 7h6"/>
              <path d="M8 11h8"/>
            </svg>
          </div>
          <h1 class="login-title">欢迎使用</h1>
          <p class="login-subtitle">登录以管理你的博客</p>
        </div>

        <!-- 登录表单 -->
        <form class="login-form" @submit.prevent="handleLogin">
          <div class="form-group">
            <label class="form-label">账号</label>
            <input
              v-model="username"
              type="text"
              class="apple-input"
              placeholder="请输入用户名"
              autocomplete="username"
              autofocus
            />
          </div>

          <div class="form-group">
            <label class="form-label">密码</label>
            <div class="password-wrapper">
              <input
                v-model="password"
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

          <!-- 保持登录 -->
          <div class="remember-me">
            <label class="checkbox-label">
              <input v-model="rememberMe" type="checkbox" class="apple-checkbox" />
              <span>保持登录</span>
            </label>
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
            <Icon v-if="loading" icon="line-md:loading-twotone-loop" :width="20" :height="20" class="loading-icon" />
            {{ loading ? '登录中...' : '登 录' }}
          </button>
        </form>

        <!-- 分割线 -->
        <div class="divider">
          <span>或</span>
        </div>

        <!-- 第三方登录 -->
        <div class="social-buttons">
          <div v-for="provider in ['Apple', 'Google', 'GitHub']" :key="provider" class="social-btn-wrap">
            <button
              :ref="(el: any) => setBtnRef(el, provider)"
              type="button"
              class="social-btn"
              :aria-label="`使用 ${provider} 登录`"
              @click="handleSocialLogin(provider, $event)"
            >
              <svg v-if="provider === 'Apple'" width="20" height="20" viewBox="0 0 24 24" fill="var(--color-foreground)">
                <path d="M17.05 20.28c-.98.95-2.05.8-3.08.35-1.09-.46-2.09-.48-3.24 0-1.44.62-2.2.44-3.06-.35C2.79 15.25 3.51 7.59 9.05 7.31c1.35.07 2.29.74 3.08.8 1.18-.24 2.31-.93 3.57-.84 1.51.12 2.65.72 3.4 1.8-3.12 1.87-2.38 5.98.48 7.13-.57 1.5-1.31 2.99-2.54 4.09zM12.03 7.25c-.15-2.23 1.66-4.07 3.74-4.25.29 2.58-2.34 4.5-3.74 4.25z"/>
              </svg>
              <svg v-else-if="provider === 'Google'" width="20" height="20" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
              </svg>
              <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="var(--color-foreground)">
                <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/>
              </svg>
            </button>
          </div>
        </div>
      </template>

      <!-- 第二步：TOTP 验证码 -->
      <template v-else>
        <div class="login-header">
          <div class="logo-icon totp-icon">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
              <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
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
            <Icon v-if="codeLoading" icon="line-md:loading-twotone-loop" :width="20" :height="20" class="loading-icon" />
            {{ codeLoading ? '验证中...' : '验证并登录' }}
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

    <!-- 气泡提示（Teleport 到 body 避免被裁剪） -->
    <Teleport to="body">
      <Transition name="bubble">
        <div
          v-if="activeTooltip"
          class="bubble-tip-global"
          :style="{
            left: tooltipPos.x + 'px',
            top: (tooltipPos.y - 12) + 'px',
          }"
        >
          {{ funnyMessages[activeTooltip] }}
          <div class="bubble-arrow-global"></div>
        </div>
      </Transition>
    </Teleport>
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
  border-radius: 12px;
  box-shadow: 0 1px 2px 0 color-mix(in srgb, #000 36%, transparent), 0 1px 3px -1px color-mix(in srgb, #000 36%, transparent);
  padding: 40px;
  overflow: visible;
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.logo-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  background: linear-gradient(180deg, #66ABFF 0%, #007AFF 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 20px;
}

.logo-icon.totp-icon {
  background: var(--color-state-success);
}

.login-title {
  font-size: 24px;
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
  border-radius: 12px;
  font-size: 14px;
  color: var(--color-foreground);
  transition: all 0.2s cubic-bezier(0.32, 0.72, 0, 1);
  outline: none;
  box-sizing: border-box;
}

.apple-input::placeholder {
  color: var(--color-muted-foreground);
}

.apple-input:focus {
  border-color: #007AFF;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-ring) 20%, transparent);
}

.password-wrapper {
  position: relative;
}

.password-wrapper .apple-input {
  padding-right: 48px;
}

.password-toggle {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 20px;
  height: 20px;
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

.remember-me {
  display: flex;
  align-items: center;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.apple-checkbox {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  accent-color: #007AFF;
  cursor: pointer;
  margin: 0;
}

.checkbox-label span {
  font-size: 14px;
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
  border-radius: 9999px;
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

.divider {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 28px 0;
}

.divider::before,
.divider::after {
  content: "";
  flex: 1;
  height: 1px;
  background-color: #3a3a3c;
}

.divider span {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.social-buttons {
  display: flex;
  justify-content: center;
  gap: 12px;
}

.social-btn-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.social-btn {
  width: 44px;
  height: 44px;
  border-radius: 9999px;
  border: 1px solid var(--color-border);
  background-color: var(--color-card);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all var(--duration-normal) var(--ease-standard);
  padding: 0;
}

.social-btn:hover {
  background-color: #2c2c2e;
  border-color: #48484a;
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
</style>

<style>
/* 全局气泡样式（Teleport 到 body，非 scoped） */
.bubble-tip-global {
  position: fixed;
  transform: translateX(-50%) translateY(-100%);
  background: color-mix(in srgb, var(--color-accent) 95%, transparent);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid color-mix(in srgb, #fff 8%, transparent);
  border-radius: 12px;
  padding: 10px 16px;
  font-size: 13px;
  color: var(--color-foreground);
  white-space: nowrap;
  box-shadow: 0 8px 32px color-mix(in srgb, #000 50%, transparent);
  z-index: 100000;
  pointer-events: none;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
}

.bubble-arrow-global {
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 7px solid transparent;
  border-right: 7px solid transparent;
  border-top: 7px solid color-mix(in srgb, var(--color-accent) 95%, transparent);
}

/* 气泡动画 */
.bubble-enter-active {
  animation: bubble-in 0.35s cubic-bezier(0.32, 0.72, 0, 1);
}
.bubble-leave-active {
  animation: bubble-out 0.2s cubic-bezier(0.32, 0.72, 0, 1) forwards;
}
@keyframes bubble-in {
  0% {
    opacity: 0;
    transform: translateX(-50%) translateY(calc(-100% + 6px)) scale(0.85);
  }
  100% {
    opacity: 1;
    transform: translateX(-50%) translateY(-100%) scale(1);
  }
}
@keyframes bubble-out {
  0% {
    opacity: 1;
    transform: translateX(-50%) translateY(-100%) scale(1);
  }
  100% {
    opacity: 0;
    transform: translateX(-50%) translateY(calc(-100% + 4px)) scale(0.9);
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

  .divider {
    margin: 20px 0;
    gap: 12px;
  }

  .social-buttons {
    gap: 10px;
  }

  .social-btn {
    width: 40px;
    height: 40px;
  }
  .social-btn svg {
    width: 18px;
    height: 18px;
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
