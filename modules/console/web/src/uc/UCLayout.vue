<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { useUCAuthStore } from "./stores/auth";

const router = useRouter();
const auth = useUCAuthStore();
const displayName = computed(() => auth.user?.nickname || auth.user?.username || "用户");

async function logout() {
  await auth.logout();
  await router.replace("/login");
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <header class="sticky top-0 z-30 border-b border-border bg-sidebar/95 backdrop-blur-xl">
      <div class="mx-auto flex min-h-16 max-w-6xl items-center gap-5 px-4 sm:px-6">
        <RouterLink to="/posts" class="flex min-w-0 items-center gap-2.5 text-sm font-semibold tracking-tight">
            <span class="flex h-9 w-9 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Icon icon="ri:user-3-line" class="h-5 w-5" />
          </span>
          <span class="truncate">用户中心</span>
        </RouterLink>
        <nav class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto" aria-label="用户中心">
          <template v-if="auth.user?.role === 'author' || auth.user?.role === 'admin'">
            <RouterLink to="/posts" class="uc-nav-link"><Icon icon="ri:article-line" />文章</RouterLink>
            <RouterLink to="/attachments" class="uc-nav-link"><Icon icon="ri:folder-2-line" />附件</RouterLink>
          </template>
          <RouterLink v-else to="/upgrade" class="uc-nav-link"><Icon icon="ri:edit-line" />升级作者</RouterLink>
          <RouterLink to="/notifications" class="uc-nav-link"><Icon icon="ri:notification-3-line" />通知</RouterLink>
          <RouterLink to="/profile" class="uc-nav-link"><Icon icon="ri:user-3-line" />资料</RouterLink>
        </nav>
        <div class="flex shrink-0 items-center gap-2">
          <span class="hidden max-w-28 truncate text-xs text-text-secondary sm:inline">{{ displayName }}</span>
          <button type="button" class="uc-quiet-button" title="退出登录" @click="logout">
            <Icon icon="ri:logout-box-r-line" class="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>
    <main class="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6 sm:py-8">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.uc-nav-link {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  gap: 0.4rem;
  white-space: nowrap;
  border-radius: 9999px;
  padding: 0.55rem 0.8rem;
  font-size: 0.8rem;
  color: var(--color-text-tertiary);
  transition:
    color 150ms var(--ease-standard),
    background 150ms var(--ease-standard);
}
.uc-nav-link:hover,
.uc-nav-link.router-link-active {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
.uc-nav-link :deep(svg) {
  height: 1rem;
  width: 1rem;
}
.uc-quiet-button {
  display: inline-flex;
  min-height: 44px;
  min-width: 44px;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  color: var(--color-text-tertiary);
  transition:
    color 150ms var(--ease-standard),
    background 150ms var(--ease-standard);
}
.uc-quiet-button:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
</style>
