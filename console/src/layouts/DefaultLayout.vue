<script setup lang="ts">
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useThemeStore, type ThemeMode } from "@/stores/theme";
import { Icon } from "@iconify/vue";

const router = useRouter();
const auth = useAuthStore();
const theme = useThemeStore();

const themeOptions: { value: ThemeMode; icon: string; label: string }[] = [
  { value: "light", icon: "ri:sun-line", label: "亮色" },
  { value: "dark", icon: "ri:moon-line", label: "暗色" },
  { value: "system", icon: "ri:computer-line", label: "跟随系统" },
];

const navItems = [
  { path: "/dashboard", label: "仪表盘", icon: "ri:dashboard-line" },
  { path: "/posts", label: "文章", icon: "ri:article-line" },
  { path: "/comments", label: "评论", icon: "ri:chat-3-line" },
  { path: "/attachments", label: "附件", icon: "ri:image-2-line" },
  { path: "/pages", label: "页面", icon: "ri:pages-line" },
  { path: "/moments", label: "瞬间", icon: "ri:quill-pen-line" },
  { path: "/notes-page", label: "笔记页", icon: "ri:booklet-line" },
  { path: "/taxonomy", label: "分类标签", icon: "ri:price-tag-3-line" },
  { path: "/friends", label: "友链", icon: "ri:links-line" },
  { path: "/settings", label: "设置", icon: "ri:settings-3-line" },
  { path: "/backup", label: "备份", icon: "ri:archive-line" },
];

async function handleLogout() {
  await auth.logout();
  router.push("/login");
}
</script>

<template>
  <div class="flex h-screen bg-gray-50 dark:bg-gray-900">
    <!-- Sidebar -->
    <aside class="w-56 bg-white border-r border-gray-200 flex flex-col dark:bg-gray-800 dark:border-gray-700">
      <div class="h-14 flex items-center px-5 border-b border-gray-100 dark:border-gray-700">
        <h1 class="text-lg font-semibold text-gray-800 dark:text-gray-100">Stellar Console</h1>
      </div>
      <nav class="flex-1 py-4 space-y-1 px-3">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-gray-600 hover:bg-gray-100 hover:text-gray-900 transition-colors dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-100"
          active-class="!bg-indigo-50 !text-indigo-700 dark:!bg-indigo-500/15 dark:!text-indigo-400 font-medium"
        >
          <Icon :icon="item.icon" class="text-lg" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
      <div class="p-3 border-t border-gray-100 dark:border-gray-700 space-y-2">
        <!-- 主题切换（亮/暗/跟随系统） -->
        <div class="flex items-center gap-1 p-1 bg-gray-100 rounded-lg dark:bg-gray-700/50">
          <button
            v-for="opt in themeOptions"
            :key="opt.value"
            @click="theme.setMode(opt.value)"
            :title="opt.label"
            :class="[
              'flex-1 flex items-center justify-center py-1.5 rounded-md text-base transition-colors',
              theme.mode === opt.value
                ? 'bg-white text-indigo-600 shadow-sm dark:bg-gray-800 dark:text-indigo-400'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200',
            ]"
          >
            <Icon :icon="opt.icon" />
          </button>
        </div>
        <button
          @click="handleLogout"
          class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-gray-500 hover:bg-red-50 hover:text-red-600 w-full transition-colors dark:text-gray-400 dark:hover:bg-red-500/10 dark:hover:text-red-400"
        >
          <Icon icon="ri:logout-box-r-line" class="text-lg" />
          <span>退出登录</span>
        </button>
      </div>
    </aside>

    <!-- Main content -->
    <main class="flex-1 overflow-auto">
      <router-view />
    </main>
  </div>
</template>
