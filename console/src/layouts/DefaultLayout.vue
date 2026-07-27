<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useThemeStore, type ThemeMode } from "@/stores/theme";
import { Icon } from "@iconify/vue";
import { VTag, Dialog } from "@/components/ui";

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();
const theme = useThemeStore();

const moreMenuVisible = ref(false);
const profileDropdownOpen = ref(false);
const searchOpen = ref(false);
const searchQuery = ref("");

const isMac = /macintosh|mac os x/i.test(navigator.userAgent);

// 页脚控制：编辑器等全屏页面通过路由 meta.hideFooter 隐藏
const showFooter = computed(() => !route.meta.hideFooter);

const themeOptions: { value: ThemeMode; icon: string; label: string }[] = [
  { value: "light", icon: "ri:sun-line", label: "亮色" },
  { value: "dark", icon: "ri:moon-line", label: "暗色" },
  { value: "system", icon: "ri:computer-line", label: "跟随系统" },
];

interface NavItem {
  path: string;
  label: string;
  icon: string;
  group: string;
  mobile?: boolean;
}

const navGroups = ["", "内容", "互动", "媒体", "系统"];

// 对齐 Halo：仪表盘无分组标签，内容/互动/媒体/系统分组
const navItems: NavItem[] = [
  { path: "/dashboard", label: "仪表盘", icon: "ri:dashboard-3-line", group: "", mobile: true },
  { path: "/posts", label: "文章", icon: "ri:book-read-line", group: "内容", mobile: true },
  { path: "/shares", label: "分享链接", icon: "ri:share-forward-line", group: "内容" },
  { path: "/pages", label: "页面", icon: "ri:pages-line", group: "内容", mobile: true },
  { path: "/moments", label: "瞬间", icon: "ri:quill-pen-line", group: "内容" },
  { path: "/notes-page", label: "笔记页", icon: "ri:booklet-line", group: "内容" },
  { path: "/comments", label: "评论", icon: "ri:chat-3-line", group: "互动", mobile: true },
  { path: "/friends", label: "友链", icon: "ri:links-line", group: "互动" },
  { path: "/attachments", label: "附件", icon: "ri:folder-2-line", group: "媒体", mobile: true },
  { path: "/taxonomy", label: "分类标签", icon: "ri:price-tag-3-line", group: "系统" },
  { path: "/plugins", label: "插件", icon: "ri:puzzle-line", group: "系统" },
  { path: "/settings", label: "设置", icon: "ri:settings-3-line", group: "系统" },
  { path: "/backup", label: "备份", icon: "ri:archive-line", group: "系统" },
];

const minimenus = navItems.filter((i) => i.mobile);

const searchResults = computed(() => {
  if (!searchQuery.value.trim()) return navItems;
  const q = searchQuery.value.toLowerCase();
  return navItems.filter((item) => item.label.toLowerCase().includes(q));
});

function openSearch() {
  searchOpen.value = true;
  searchQuery.value = "";
}

function navigateTo(path: string) {
  searchOpen.value = false;
  moreMenuVisible.value = false;
  router.push(path);
}

function onKeydown(e: KeyboardEvent) {
  const { key, ctrlKey, metaKey } = e;
  if (key === "k" && ((ctrlKey && !isMac) || metaKey)) {
    e.preventDefault();
    searchOpen.value ? (searchOpen.value = false) : openSearch();
  }
  if (key === "Escape") searchOpen.value = false;
}

onMounted(() => document.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => document.removeEventListener("keydown", onKeydown));

function handleLogout() {
  profileDropdownOpen.value = false;
  Dialog.warning({
    title: "确定要退出登录吗？",
    onConfirm: async () => {
      await auth.logout();
      router.push("/login");
    },
  });
}

function closeProfileDropdown() {
  profileDropdownOpen.value = false;
}
</script>

<template>
  <div class="flex h-screen bg-gray-50 dark:bg-gray-900" @click="closeProfileDropdown">
    <!-- Sidebar（对齐 Halo BasicLayout navbar：w-64 bg-white） -->
    <aside class="navbar fixed inset-y-0 left-0 z-[999] hidden h-full w-64 flex-col overflow-y-auto bg-white shadow-sm md:flex dark:bg-gray-800">
      <!-- Logo -->
      <div class="flex justify-center pb-7 pt-5">
        <a href="/" target="_blank" title="访问首页" class="flex items-center gap-2 transition-all hover:brightness-125">
          <img src="/favicon.svg" alt="Logo" class="h-8 w-8 cursor-pointer select-none" />
          <span class="text-lg font-bold text-gray-900 dark:text-gray-100">Console</span>
        </a>
      </div>

      <div class="flex-1 overflow-y-auto">
        <!-- 搜索框（对齐 Halo：bg-gray-100 rounded p-2） -->
        <div class="px-3">
          <div
            class="flex cursor-pointer items-center rounded bg-gray-100 p-2 text-gray-400 transition-all hover:text-gray-900 dark:bg-gray-700 dark:hover:text-gray-100"
            style="border-radius: 4px"
            @click="openSearch"
          >
            <span class="mr-3"><Icon icon="ri:search-line" /></span>
            <span class="flex-1 select-none text-base font-normal">搜索</span>
            <div class="text-sm">{{ isMac ? "⌘" : "Ctrl" }}+K</div>
          </div>
        </div>

        <!-- 分组菜单（对齐 VMenu/VMenuItem/VMenuLabel） -->
        <ul class="menu-list space-y-1 p-3">
          <template v-for="group in navGroups" :key="group">
            <li v-if="group" class="menu-label">{{ group }}</li>
            <li v-for="item in navItems.filter((i) => i.group === group)" :key="item.path" class="menu-item">
              <div
                :class="{ active: route.path.startsWith(item.path) }"
                class="menu-item-title"
                @click="navigateTo(item.path)"
              >
                <span class="menu-icon mr-3 self-center">
                  <Icon :icon="item.icon" width="20" height="20" />
                </span>
                <span class="menu-title flex-1 self-center">{{ item.label }}</span>
              </div>
            </li>
          </template>
        </ul>
      </div>

      <!-- 底部用户资料区（对齐 Halo profile 70px） -->
      <div class="profile-placeholder">
        <div class="current-profile">
          <div class="profile-avatar">
            <span class="flex h-9 w-9 items-center justify-center rounded-full bg-primary/15">
              <Icon icon="ri:user-3-line" class="text-lg text-primary" />
            </span>
          </div>
          <div class="profile-name">
            <div class="flex text-sm font-medium text-gray-900 dark:text-gray-100">管理员</div>
            <div class="flex">
              <VTag>
                <template #leftIcon>
                  <Icon icon="ri:user-settings-line" class="text-xs" />
                </template>
                超级管理员
              </VTag>
            </div>
          </div>
          <div class="relative self-center">
            <div
              class="cursor-pointer rounded p-1 transition-all hover:bg-gray-100 dark:hover:bg-gray-700"
              @click.stop="profileDropdownOpen = !profileDropdownOpen"
            >
              <Icon icon="ri:more-fill" class="text-gray-500 dark:text-gray-400" />
            </div>
            <Transition name="dropdown">
              <div
                v-if="profileDropdownOpen"
                class="absolute bottom-full right-0 z-30 mb-2 w-48 rounded border border-gray-100 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800"
                style="border-radius: 4px"
                @click.stop
              >
                <!-- 主题切换 -->
                <div class="mb-2 flex items-center gap-1 rounded bg-gray-100 p-1 dark:bg-gray-700">
                  <button
                    v-for="opt in themeOptions"
                    :key="opt.value"
                    :title="opt.label"
                    :class="[
                      'flex flex-1 items-center justify-center rounded py-1.5 text-base transition-colors',
                      theme.mode === opt.value
                        ? 'bg-white text-primary shadow-sm dark:bg-gray-800'
                        : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200',
                    ]"
                    @click="theme.setMode(opt.value)"
                  >
                    <Icon :icon="opt.icon" />
                  </button>
                </div>
                <button
                  class="flex w-full items-center gap-2 rounded px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
                  @click="handleLogout"
                >
                  <Icon icon="ri:logout-box-r-line" />
                  退出登录
                </button>
              </div>
            </Transition>
          </div>
        </div>
      </div>
    </aside>

    <!-- Main content -->
    <main class="content w-full overflow-y-auto pb-16 md:ml-64 md:pb-0">
      <router-view />
      <!-- 页脚（对齐 Halo：Powered by ...） -->
      <footer
        v-if="showFooter"
        class="border-t border-gray-100 px-4 py-3 text-center text-xs text-gray-400 dark:border-gray-700 dark:text-gray-500"
      >
        Powered by <a href="/" target="_blank" class="font-medium text-gray-500 transition-colors hover:text-primary dark:text-gray-400">Stellar Blog</a>
      </footer>
    </main>

    <!-- 移动端底部导航条（对齐 Halo bottom-nav-bar：bg-secondary） -->
    <div class="fixed bottom-0 left-0 right-0 z-40 grid grid-cols-6 border-t-2 border-black bg-secondary drop-shadow-2xl md:hidden">
      <div
        v-for="menu in minimenus"
        :key="menu.path"
        :class="{ 'bg-black': route.path.startsWith(menu.path) }"
        class="nav-item"
        @click="router.push(menu.path)"
      >
        <div class="flex w-full cursor-pointer items-center justify-center p-1 text-white">
          <div class="flex h-10 w-10 flex-col items-center justify-center">
            <div class="text-base"><Icon :icon="menu.icon" /></div>
            <div class="mt-0.5 text-xs">{{ menu.label }}</div>
          </div>
        </div>
      </div>
      <div class="nav-item" @click="moreMenuVisible = true">
        <div class="flex w-full cursor-pointer items-center justify-center p-1 text-white">
          <div class="flex h-10 w-10 flex-col items-center justify-center">
            <div class="text-base"><Icon icon="ri:more-fill" /></div>
            <div class="mt-0.5 text-xs">更多</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 移动端"更多"底部抽屉（对齐 Halo drawer） -->
    <Teleport to="body">
      <div v-if="moreMenuVisible" class="fixed inset-0 z-[99999] flex flex-row items-end justify-center">
        <Transition name="fade" appear>
          <div class="absolute inset-0 bg-gray-500/75 transition-opacity" @click="moreMenuVisible = false"></div>
        </Transition>
        <Transition name="slide-up" appear>
          <div class="relative flex h-3/4 w-screen flex-col items-stretch overflow-y-auto rounded-t-md bg-white shadow-xl dark:bg-gray-800">
            <ul class="menu-list space-y-1 p-3">
              <template v-for="group in navGroups" :key="group">
                <li v-if="group" class="menu-label">{{ group }}</li>
                <li v-for="item in navItems.filter((i) => i.group === group)" :key="item.path" class="menu-item">
                  <div
                    :class="{ active: route.path.startsWith(item.path) }"
                    class="menu-item-title"
                    @click="navigateTo(item.path)"
                  >
                    <span class="menu-icon mr-3 self-center">
                      <Icon :icon="item.icon" width="20" height="20" />
                    </span>
                    <span class="menu-title flex-1 self-center">{{ item.label }}</span>
                  </div>
                </li>
              </template>
            </ul>
          </div>
        </Transition>
      </div>
    </Teleport>

    <!-- 全局搜索弹窗（⌘K） -->
    <Transition name="fade">
      <div v-if="searchOpen" class="fixed inset-0 z-[9999] flex items-start justify-center pt-[20vh]" @click.self="searchOpen = false">
        <div class="absolute inset-0 bg-gray-500/75" @click="searchOpen = false" />
        <div
          class="relative mx-4 w-full max-w-md overflow-hidden rounded border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-800"
          style="border-radius: 4px"
        >
          <div class="flex items-center gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-700">
            <Icon icon="ri:search-line" class="text-gray-400" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜索菜单..."
              class="flex-1 bg-transparent text-sm text-gray-800 outline-none placeholder-gray-400 dark:text-gray-100"
              autofocus
            />
            <kbd class="rounded border border-gray-200 px-1.5 py-0.5 text-[10px] text-gray-400 dark:border-gray-600">ESC</kbd>
          </div>
          <div class="max-h-64 overflow-y-auto py-2">
            <button
              v-for="item in searchResults"
              :key="item.path"
              class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm text-gray-700 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              @click="navigateTo(item.path)"
            >
              <Icon :icon="item.icon" class="text-base text-gray-400" />
              <span>{{ item.label }}</span>
              <span class="ml-auto text-[10px] text-gray-400">{{ item.group || "概览" }}</span>
            </button>
            <p v-if="searchResults.length === 0" class="px-4 py-6 text-center text-sm text-gray-400">无匹配结果</p>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
/* 对齐 Halo VMenuLabel */
.menu-label {
  padding: 0.5rem;
  font-size: 0.75rem;
  color: #9ca3af;
  user-select: none;
}

/* 对齐 Halo VMenuItem */
.menu-item {
  cursor: pointer;
  list-style: none;
}

.menu-item-title {
  transition: all 0.15s ease;
  font-size: 1rem;
  display: flex;
  user-select: none;
  position: relative;
  padding: 0.5rem;
  font-weight: 400;
  border-radius: 4px;
  color: #374151;
}
.dark .menu-item-title {
  color: #d1d5db;
}

.menu-item-title:hover,
.menu-item-title.active {
  background: #f3f4f6;
  font-weight: 500;
}
.dark .menu-item-title:hover,
.dark .menu-item-title.active {
  background: #374151;
}

/* 激活项左侧竖条（对齐 Halo：#242e41 3px x 26px） */
.menu-item-title.active::after {
  position: absolute;
  top: calc(50% - 13px);
  left: -8px;
  width: 3px;
  height: 26px;
  content: "";
  background: #242e41;
  border-radius: 6px;
}
.dark .menu-item-title.active::after {
  background: #4ccba0;
}

/* 底部用户资料区（对齐 Halo 70px） */
.profile-placeholder {
  height: 70px;
}

.current-profile {
  height: 70px;
  position: fixed;
  left: 0;
  bottom: 0;
  display: flex;
  width: 16rem;
  gap: 0.75rem;
  background: #fff;
  padding: 0.75rem;
  z-index: 10;
}
.dark .current-profile {
  background: #1f2937;
}

.profile-avatar {
  display: flex;
  align-items: center;
  align-self: center;
}

.profile-name {
  flex: 1;
  align-self: center;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}
.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(4px);
}

.slide-up-enter-active {
  transition: transform 0.4s ease-in-out;
}
.slide-up-leave-active {
  transition: transform 0.3s ease-in-out;
}
.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
}
</style>
