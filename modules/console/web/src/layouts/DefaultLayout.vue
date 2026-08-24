<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useQuery } from "@tanstack/vue-query";
import { useAuthStore } from "@/stores/auth";
import { useModulesStore, type ModuleNav } from "@/stores/modules";
import { Icon } from "@iconify/vue";
import { VTag, Dialog } from "@/components/ui";
import { searchApi, notificationApi, type SearchResultItem } from "@/api";
import { queryKeys } from "@/api/query-client";

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();
const modulesStore = useModulesStore();

const moreMenuVisible = ref(false);
const profileDropdownOpen = ref(false);
const searchOpen = ref(false);
const searchQuery = ref("");

// 未读通知数（铃铛角标，Spec 62：raw axios 轮询改 useQuery + refetchInterval，
// 与 NotificationsView 共用查询体系——已读操作 invalidate 后角标即时更新）
const { data: unreadData } = useQuery({
  queryKey: queryKeys.unread(),
  queryFn: () => notificationApi.unreadCount().then(({ data }) => data.count),
  refetchInterval: 60_000,
});
const unreadCount = computed(() => unreadData.value || 0);

// 侧边栏收起状态（批注：左侧边栏希望可以收起来），收起后仅图标模式，状态持久化
const SIDEBAR_KEY = "console:sidebar-collapsed";
const sidebarCollapsed = ref(localStorage.getItem(SIDEBAR_KEY) === "1");
function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value;
  localStorage.setItem(SIDEBAR_KEY, sidebarCollapsed.value ? "1" : "0");
}

const isMac = /macintosh|mac os x/i.test(navigator.userAgent);

// 页脚控制：编辑器等全屏页面通过路由 meta.hideFooter 隐藏
const showFooter = computed(() => !route.meta.hideFooter);

type NavItem = ModuleNav;

function roleAllowed(item: NavItem): boolean {
  const current = auth.isAdmin ? "admin" : auth.user.role || auth.role;
  return !item.roles?.length || item.roles.includes(current) || (auth.isAdmin && item.roles.includes("author"));
}

function visibleNav(item: NavItem): NavItem | null {
  if (!roleAllowed(item)) return null;
  const children = item.children?.map(visibleNav).filter((child): child is NavItem => child !== null);
  return children ? { ...item, children } : item;
}

// 菜单真值来自后端 /api/v1/modules；前端模块只声明契约，不自行决定显隐。
const allNavItems = computed<NavItem[]>(() =>
  modulesStore
    .enabledNav()
    .map(visibleNav)
    .filter((item): item is NavItem => item !== null),
);
const navGroups = computed(() => ["", ...new Set(allNavItems.value.map((item) => item.group).filter(Boolean))]);
const minimenus = computed(() => allNavItems.value.filter((item) => item.mobile));

// 搜索用扁平列表（展开子菜单，保证 ⌘K 仍能搜到分享链接）
const flatNavItems = computed<NavItem[]>(() =>
  allNavItems.value.flatMap((i) => (i.children ? [i, ...i.children] : [i])),
);

const searchResults = computed(() => {
  if (!searchQuery.value.trim()) return flatNavItems.value;
  const q = searchQuery.value.toLowerCase();
  return flatNavItems.value.filter((item) => item.label.toLowerCase().includes(q));
});

// --- 内容搜索（Spec 39：⌘K 接入后端 /api/v1/search，含草稿/回收站）---
const contentResults = ref<SearchResultItem[]>([]);
const contentSearching = ref(false);
const contentSearchFailed = ref(false); // 接口异常时静默降级为仅菜单搜索
let searchDebounce: ReturnType<typeof setTimeout> | null = null;
let searchSeq = 0; // 丢弃乱序响应（慢请求覆盖新结果）
const activeIndex = ref(0); // ↑↓ 统一导航的当前选中项（菜单区+内容区平铺索引）

// 内容结果区是否启用：输入 ≥2 字符且未降级
const contentActive = computed(() => searchQuery.value.trim().length >= 2 && !contentSearchFailed.value);
const postResults = computed(() => contentResults.value.filter((i) => i.type === "post"));
const pageResults = computed(() => contentResults.value.filter((i) => i.type === "page"));

watch(searchQuery, () => {
  activeIndex.value = 0;
  if (searchDebounce) clearTimeout(searchDebounce);
  const q = searchQuery.value.trim();
  if (q.length < 2) {
    searchSeq++;
    contentResults.value = [];
    contentSearching.value = false;
    contentSearchFailed.value = false;
    return;
  }
  contentSearching.value = true;
  searchDebounce = setTimeout(async () => {
    const seq = ++searchSeq;
    try {
      const { data } = await searchApi.query(q);
      if (seq !== searchSeq) return;
      contentResults.value = data.items || [];
      contentSearchFailed.value = false;
    } catch {
      if (seq !== searchSeq) return;
      contentResults.value = [];
      contentSearchFailed.value = true;
    } finally {
      if (seq === searchSeq) contentSearching.value = false;
    }
  }, 300);
});

// --- 统一键盘导航：菜单区 + 文章组 + 页面组拼成平铺列表，↑↓ 跨区移动 ---
type SearchEntry = { kind: "menu"; item: NavItem } | { kind: "content"; item: SearchResultItem };

const searchEntries = computed<SearchEntry[]>(() => [
  ...searchResults.value.map((item): SearchEntry => ({ kind: "menu", item })),
  ...(contentActive.value
    ? [...postResults.value, ...pageResults.value].map((item): SearchEntry => ({ kind: "content", item }))
    : []),
]);

// 分组渲染时各区在平铺列表中的起始偏移
const postOffset = computed(() => searchResults.value.length);
const pageOffset = computed(() => postOffset.value + postResults.value.length);

function moveActive(delta: number) {
  const n = searchEntries.value.length;
  if (!n) return;
  activeIndex.value = (activeIndex.value + delta + n) % n;
  nextTick(() => {
    document.querySelector(`[data-search-index="${activeIndex.value}"]`)?.scrollIntoView({ block: "nearest" });
  });
}

function confirmActive() {
  const entry = searchEntries.value[activeIndex.value];
  if (!entry) return;
  if (entry.kind === "menu") {
    navigateTo(entry.item.path);
  } else {
    openContentResult(entry.item);
  }
}

// 内容结果跳转：文章/页面 → Console 编辑器；回收站项 → 对应列表页带筛选参数
function openContentResult(item: SearchResultItem) {
  searchOpen.value = false;
  if (item.status === "trashed") {
    router.push({ path: item.type === "post" ? "/posts" : "/pages", query: { status: "trashed" } });
    return;
  }
  router.push(item.type === "post" ? `/posts/edit/${item.slug}` : `/pages/edit/${item.slug}`);
}

const statusBadges: Record<string, { label: string; cls: string }> = {
  draft: { label: "草稿", cls: "bg-state-warning/15 text-state-warning" },
  trashed: { label: "回收站", cls: "bg-state-error/15 text-state-error" },
};

function openSearch() {
  searchOpen.value = true;
  searchQuery.value = "";
  activeIndex.value = 0;
  contentResults.value = [];
  contentSearching.value = false;
  contentSearchFailed.value = false;
}

function navigateTo(path: string) {
  searchOpen.value = false;
  moreMenuVisible.value = false;
  profileDropdownOpen.value = false;
  router.push(path);
}

// 菜单项激活/展开判定：当前路径匹配该项或其任一子项（子项路径未必以父路径开头，如笔记页在页面下）
function isNavActive(item: NavItem): boolean {
  if (route.path.startsWith(item.path)) return true;
  return !!item.children?.some((c) => route.path.startsWith(c.path));
}

function onKeydown(e: KeyboardEvent) {
  const { key, ctrlKey, metaKey } = e;
  if (key === "k" && ((ctrlKey && !isMac) || metaKey)) {
    e.preventDefault();
    searchOpen.value ? (searchOpen.value = false) : openSearch();
  }
  if (key === "Escape") searchOpen.value = false;
  // 搜索弹窗内：↑↓ 在菜单区与内容结果区间统一导航，回车打开选中项
  if (searchOpen.value) {
    if (key === "ArrowDown") {
      e.preventDefault();
      moveActive(1);
    } else if (key === "ArrowUp") {
      e.preventDefault();
      moveActive(-1);
    } else if (key === "Enter") {
      e.preventDefault();
      confirmActive();
    }
  }
}

onMounted(() => {
  document.addEventListener("keydown", onKeydown);
  // 拉取功能模块 manifest（驱动动态菜单）；已加载则不重复拉
  if (!modulesStore.loaded) modulesStore.fetch();
});
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
  <div class="flex h-screen bg-background text-foreground" @click="closeProfileDropdown">
    <!-- Sidebar（对齐 Halo BasicLayout navbar：w-64 bg-card；支持收起为仅图标的 w-16） -->
    <aside
      :class="sidebarCollapsed ? 'sidebar-collapsed w-16' : 'w-64'"
      class="fixed inset-y-0 left-0 z-[999] hidden h-full flex-col overflow-y-auto overflow-x-hidden border-r border-border transition-[width] duration-150 md:flex"
      style="background: var(--color-sidebar); backdrop-filter: saturate(180%) blur(24px)"
    >
      <!-- Logo -->
      <div class="flex justify-center pb-7 pt-5">
        <a
          href="/"
          target="_blank"
          title="访问首页"
          class="flex items-center gap-2 transition-all hover:brightness-125"
        >
          <img src="/favicon.svg" alt="Logo" class="h-8 w-8 cursor-pointer select-none" />
          <span v-if="!sidebarCollapsed" class="text-lg font-bold text-foreground">Console</span>
        </a>
      </div>

      <div class="flex-1 overflow-y-auto">
        <!-- 搜索框（对齐 Halo：bg-secondary rounded p-2；收起时仅图标） -->
        <div class="px-3">
          <div
            class="flex cursor-pointer items-center rounded-base bg-secondary p-2 text-text-quaternary transition-all hover:text-foreground hover:bg-accent"
            :class="{ 'justify-center': sidebarCollapsed }"
            :title="sidebarCollapsed ? '搜索 (⌘+K)' : ''"
            @click="openSearch"
          >
            <span :class="{ 'mr-3': !sidebarCollapsed }"><Icon icon="ri:search-line" /></span>
            <template v-if="!sidebarCollapsed">
              <span class="flex-1 select-none text-base font-normal">搜索</span>
              <div class="text-sm">{{ isMac ? "⌘" : "Ctrl" }}+K</div>
            </template>
          </div>
        </div>

        <!-- 分组菜单（对齐 VMenu/VMenuItem/VMenuLabel；收起时分组标题改分割线） -->
        <ul class="menu-list space-y-1 p-3">
          <template v-for="group in navGroups" :key="group">
            <li v-if="group && !sidebarCollapsed" class="menu-label">{{ group }}</li>
            <li v-else-if="group" class="mx-1 my-2 border-t border-border"></li>
            <li
              v-for="item in allNavItems.filter((i) => i.group === group)"
              :key="item.path"
              class="menu-item group/item relative"
            >
              <div
                :class="{ active: isNavActive(item), 'justify-center': sidebarCollapsed }"
                class="menu-item-title"
                :title="sidebarCollapsed ? item.label : ''"
                @click="navigateTo(item.path)"
              >
                <span class="menu-icon self-center" :class="{ 'mr-3': !sidebarCollapsed }">
                  <Icon :icon="item.icon" width="20" height="20" />
                </span>
                <template v-if="!sidebarCollapsed">
                  <span class="menu-title flex-1 self-center">{{ item.label }}</span>
                  <span
                    v-if="item.children?.length"
                    class="self-center text-text-quaternary transition-transform text-text-tertiary"
                    :class="{ '-rotate-90': !isNavActive(item) }"
                  >
                    <Icon icon="ri:arrow-down-s-line" />
                  </span>
                </template>
              </div>
              <!-- 收起模式下的 Hover Floating Tooltip -->
              <div
                v-if="sidebarCollapsed"
                class="pointer-events-none absolute left-full top-1/2 z-50 ml-2.5 hidden -translate-y-1/2 whitespace-nowrap rounded-md border border-border bg-card px-2.5 py-1 text-xs font-medium text-foreground shadow-lg backdrop-blur-md group-hover/item:block"
              >
                {{ item.label }}
              </div>
              <ul v-if="!sidebarCollapsed && item.children?.length && isNavActive(item)" class="mt-1 space-y-1">
                <li v-for="child in item.children" :key="child.path" class="menu-item">
                  <div
                    :class="{ active: route.path.startsWith(child.path) }"
                    class="menu-subitem"
                    @click="navigateTo(child.path)"
                  >
                    <span class="menu-icon mr-3 self-center">
                      <Icon :icon="child.icon" width="18" height="18" />
                    </span>
                    <span class="menu-title flex-1 self-center">{{ child.label }}</span>
                  </div>
                </li>
              </ul>
            </li>
          </template>
        </ul>
      </div>

      <!-- 收起/展开切换 -->
      <div class="px-3 pb-1">
        <button
          class="flex w-full items-center rounded-base p-2 text-text-quaternary transition-all hover:text-foreground hover:bg-accent"
          :class="{ 'justify-center': sidebarCollapsed }"
          :title="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'"
          @click="toggleSidebar"
        >
          <span :class="{ 'mr-3': !sidebarCollapsed }">
            <Icon :icon="sidebarCollapsed ? 'ri:menu-unfold-line' : 'ri:menu-fold-line'" width="20" height="20" />
          </span>
          <span v-if="!sidebarCollapsed" class="select-none text-sm">收起侧边栏</span>
        </button>
      </div>

      <!-- 底部用户资料区（对齐 Halo profile 70px；收起时仅头像） -->
      <div class="profile-placeholder">
        <div class="current-profile" :class="sidebarCollapsed ? 'w-16 justify-center gap-0 !p-0' : 'w-64'">
          <!-- 铃铛通知按钮（Wave 3b）：收起时隐藏，避免 w-16 宽度放不下两个图标 -->
          <div v-if="!sidebarCollapsed" class="relative self-center">
            <div
              class="cursor-pointer rounded p-1 transition-all hover:bg-accent"
              @click="router.push('/notifications')"
            >
              <Icon icon="ri:notification-3-line" class="text-text-tertiary" />
              <span
                v-if="unreadCount > 0"
                class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-state-error px-1 text-[10px] font-bold text-foreground"
                >{{ unreadCount > 99 ? "99+" : unreadCount }}</span
              >
            </div>
          </div>
          <div class="profile-avatar">
            <img
              v-if="auth.user.avatar"
              :src="auth.user.avatar"
              alt="avatar"
              class="h-9 w-9 rounded-full object-cover"
            />
            <span v-else class="flex h-9 w-9 items-center justify-center rounded-full bg-primary/15">
              <Icon icon="ri:user-3-line" class="text-lg text-primary" />
            </span>
          </div>
          <!-- 收起时：未读徽标叠在头像右上角，保持通知入口可见 -->
          <div
            v-if="sidebarCollapsed && unreadCount > 0"
            class="pointer-events-none absolute left-1/2 top-1/2 ml-4 -mt-5 flex h-4 min-w-4 items-center justify-center rounded-full bg-state-error px-1 text-[10px] font-bold text-foreground"
          >
            {{ unreadCount > 99 ? "99+" : unreadCount }}
          </div>
          <div v-if="!sidebarCollapsed" class="profile-name">
            <div class="flex text-sm font-medium text-foreground">{{ auth.user.display_name || "管理员" }}</div>
            <div class="flex">
              <VTag>
                <template #leftIcon>
                  <Icon icon="ri:user-settings-line" class="text-xs" />
                </template>
                {{ auth.user.username || "超级管理员" }}
              </VTag>
            </div>
          </div>
          <div v-if="!sidebarCollapsed" class="relative self-center">
            <div
              class="cursor-pointer rounded p-1 transition-all hover:bg-accent"
              @click.stop="profileDropdownOpen = !profileDropdownOpen"
            >
              <Icon icon="ri:more-fill" class="text-text-tertiary" />
            </div>
            <Transition name="dropdown">
              <div
                v-if="profileDropdownOpen"
                class="absolute bottom-full right-0 z-30 mb-2 w-48 rounded-base border border-border bg-card p-2 shadow-lg"
                @click.stop
              >
                <button
                  class="flex w-full items-center gap-2 rounded px-3 py-2 text-sm text-text-secondary transition-colors hover:bg-accent"
                  @click="navigateTo('/profile')"
                >
                  <Icon icon="ri:user-settings-line" />
                  个人资料
                </button>
                <button
                  class="flex w-full items-center gap-2 rounded px-3 py-2 text-sm text-text-secondary transition-colors hover:bg-accent"
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
    <main
      class="content w-full min-w-0 overflow-y-auto pb-16 md:pb-0"
      :class="sidebarCollapsed ? 'md:ml-16' : 'md:ml-64'"
    >
      <router-view />
      <!-- 页脚（对齐 Halo：Powered by ...） -->
      <footer
        v-if="showFooter"
        class="border-t border-border px-4 py-3 text-center text-xs text-text-quaternary text-text-tertiary"
      >
        Powered by
        <a href="/" target="_blank" class="font-medium text-text-tertiary transition-colors hover:text-primary"
          >Stellar Blog</a
        >
      </footer>
    </main>

    <!-- 移动端底部导航条（对齐 Halo bottom-nav-bar：bg-secondary） -->
    <div
      class="fixed bottom-0 left-0 right-0 z-40 grid grid-cols-7 border-t-2 border-black bg-secondary drop-shadow-2xl md:hidden"
    >
      <div
        v-for="menu in minimenus"
        :key="menu.path"
        :class="{ 'bg-black': route.path.startsWith(menu.path) }"
        class="nav-item min-w-0"
        @click="router.push(menu.path)"
      >
        <div class="flex w-full cursor-pointer items-center justify-center p-1 text-foreground">
          <div class="flex h-10 w-10 flex-col items-center justify-center">
            <div class="text-base"><Icon :icon="menu.icon" /></div>
            <div class="mt-0.5 text-xs">{{ menu.label }}</div>
          </div>
        </div>
      </div>
      <div class="nav-item min-w-0" @click="moreMenuVisible = true">
        <div class="flex w-full cursor-pointer items-center justify-center p-1 text-foreground">
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
          <div class="absolute inset-0 bg-background/60 transition-opacity" @click="moreMenuVisible = false"></div>
        </Transition>
        <Transition name="slide-up" appear>
          <div
            class="relative flex h-3/4 w-screen flex-col items-stretch overflow-y-auto rounded-t-md bg-card shadow-xl"
          >
            <ul class="menu-list space-y-1 p-3">
              <template v-for="group in navGroups" :key="group">
                <li v-if="group" class="menu-label">{{ group }}</li>
                <li v-for="item in allNavItems.filter((i) => i.group === group)" :key="item.path" class="menu-item">
                  <div :class="{ active: isNavActive(item) }" class="menu-item-title" @click="navigateTo(item.path)">
                    <span class="menu-icon mr-3 self-center">
                      <Icon :icon="item.icon" width="20" height="20" />
                    </span>
                    <span class="menu-title flex-1 self-center">{{ item.label }}</span>
                    <span
                      v-if="item.children?.length"
                      class="self-center text-text-quaternary transition-transform text-text-tertiary"
                      :class="{ '-rotate-90': !isNavActive(item) }"
                    >
                      <Icon icon="ri:arrow-down-s-line" />
                    </span>
                  </div>
                  <ul v-if="item.children?.length && isNavActive(item)" class="mt-1 space-y-1">
                    <li v-for="child in item.children" :key="child.path" class="menu-item">
                      <div
                        :class="{ active: route.path.startsWith(child.path) }"
                        class="menu-subitem"
                        @click="navigateTo(child.path)"
                      >
                        <span class="menu-icon mr-3 self-center">
                          <Icon :icon="child.icon" width="18" height="18" />
                        </span>
                        <span class="menu-title flex-1 self-center">{{ child.label }}</span>
                      </div>
                    </li>
                  </ul>
                </li>
              </template>
            </ul>
          </div>
        </Transition>
      </div>
    </Teleport>

    <!-- 全局搜索弹窗（⌘K） -->
    <Transition name="fade">
      <div
        v-if="searchOpen"
        class="fixed inset-0 z-[9999] flex items-start justify-center pt-[15vh] p-4"
        @click.self="searchOpen = false"
      >
        <div
          class="absolute inset-0 backdrop-blur-md"
          style="background: color-mix(in srgb, #000 65%, transparent)"
          @click="searchOpen = false"
        />
        <div
          class="relative w-full max-w-lg overflow-hidden rounded-xl border border-border bg-card shadow-2xl transition-all"
          style="box-shadow: var(--shadow-2xl)"
        >
          <!-- 搜索输入头 -->
          <div class="flex items-center gap-3 border-b border-border px-4 py-3.5 bg-card">
            <Icon icon="ri:search-line" class="text-lg text-primary shrink-0" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜索菜单、文章、页面..."
              class="flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-text-quaternary"
              autofocus
            />
            <kbd class="rounded border border-border bg-secondary px-2 py-0.5 text-[10px] font-mono text-text-tertiary"
              >ESC</kbd
            >
          </div>

          <!-- 搜索结果列表 -->
          <div class="max-h-80 overflow-y-auto py-2">
            <!-- 菜单区 -->
            <template v-if="searchResults.length">
              <p class="px-4 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wider text-text-quaternary">
                系统菜单
              </p>
              <button
                v-for="(item, i) in searchResults"
                :key="item.path"
                :data-search-index="i"
                :class="{ 'bg-accent text-foreground': activeIndex === i, 'text-text-secondary': activeIndex !== i }"
                class="group flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors hover:bg-accent hover:text-foreground"
                @click="navigateTo(item.path)"
                @mouseenter="activeIndex = i"
              >
                <div
                  class="flex h-6 w-6 items-center justify-center rounded bg-secondary text-text-tertiary group-hover:text-primary"
                >
                  <Icon :icon="item.icon" class="text-sm" />
                </div>
                <span class="font-medium">{{ item.label }}</span>
                <span class="ml-auto rounded-full bg-secondary/80 px-2 py-0.5 text-[10px] text-text-tertiary">{{
                  item.group || "概览"
                }}</span>
              </button>
            </template>

            <!-- 内容结果区（≥2 字符启用；接口异常时静默降级为仅菜单搜索） -->
            <template v-if="contentActive">
              <template v-if="postResults.length">
                <p class="px-4 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wider text-text-quaternary">
                  文章内容
                </p>
                <button
                  v-for="(item, i) in postResults"
                  :key="'post-' + item.id"
                  :data-search-index="postOffset + i"
                  :class="{ 'bg-accent': activeIndex === postOffset + i }"
                  class="group flex w-full items-start gap-3 px-4 py-2.5 text-left transition-colors hover:bg-accent"
                  @click="openContentResult(item)"
                  @mouseenter="activeIndex = postOffset + i"
                >
                  <div
                    class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded bg-secondary text-text-tertiary group-hover:text-primary"
                  >
                    <Icon icon="ri:book-read-line" class="text-sm" />
                  </div>
                  <span class="min-w-0 flex-1">
                    <span class="flex items-center gap-2">
                      <span class="truncate text-sm font-medium text-foreground">{{ item.title }}</span>
                      <span
                        v-if="statusBadges[item.status]"
                        :class="statusBadges[item.status].cls"
                        class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold"
                        >{{ statusBadges[item.status].label }}</span
                      >
                    </span>
                    <span
                      v-if="item.snippet"
                      class="search-snippet mt-0.5 block truncate text-xs text-text-tertiary"
                      v-html="item.snippet"
                    />
                  </span>
                </button>
              </template>
              <template v-if="pageResults.length">
                <p class="px-4 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wider text-text-quaternary">
                  独立页面
                </p>
                <button
                  v-for="(item, i) in pageResults"
                  :key="'page-' + item.id"
                  :data-search-index="pageOffset + i"
                  :class="{ 'bg-accent': activeIndex === pageOffset + i }"
                  class="group flex w-full items-start gap-3 px-4 py-2.5 text-left transition-colors hover:bg-accent"
                  @click="openContentResult(item)"
                  @mouseenter="activeIndex = pageOffset + i"
                >
                  <div
                    class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded bg-secondary text-text-tertiary group-hover:text-primary"
                  >
                    <Icon icon="ri:pages-line" class="text-sm" />
                  </div>
                  <span class="min-w-0 flex-1">
                    <span class="flex items-center gap-2">
                      <span class="truncate text-sm font-medium text-foreground">{{ item.title }}</span>
                      <span
                        v-if="statusBadges[item.status]"
                        :class="statusBadges[item.status].cls"
                        class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold"
                        >{{ statusBadges[item.status].label }}</span
                      >
                    </span>
                    <span
                      v-if="item.snippet"
                      class="search-snippet mt-0.5 block truncate text-xs text-text-tertiary"
                      v-html="item.snippet"
                    />
                  </span>
                </button>
              </template>
              <p
                v-if="contentSearching && !contentResults.length"
                class="px-4 py-3 text-center text-xs text-text-tertiary"
              >
                搜索中…
              </p>
              <p
                v-else-if="!contentSearching && !contentResults.length"
                class="px-4 py-3 text-center text-xs text-text-tertiary"
              >
                无匹配内容
              </p>
            </template>

            <p
              v-if="searchResults.length === 0 && !contentActive"
              class="px-4 py-6 text-center text-sm text-text-quaternary"
            >
              无匹配结果
            </p>
          </div>

          <!-- 底部快捷键 Footer 状态栏 -->
          <div
            class="flex items-center justify-between border-t border-border bg-secondary/40 px-4 py-2 text-[11px] text-text-tertiary"
          >
            <div class="flex items-center gap-3">
              <span class="flex items-center gap-1.5"
                ><kbd class="rounded border border-border bg-card px-1.5 py-0.5 text-[9px] font-mono">↑↓</kbd>
                导航</span
              >
              <span class="flex items-center gap-1.5"
                ><kbd class="rounded border border-border bg-card px-1.5 py-0.5 text-[9px] font-mono">↵</kbd> 打开</span
              >
            </div>
            <span class="flex items-center gap-1.5"
              ><kbd class="rounded border border-border bg-card px-1.5 py-0.5 text-[9px] font-mono">ESC</kbd> 关闭</span
            >
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
  color: var(--color-text-quaternary);
  user-select: none;
}

/* 对齐 Halo VMenuItem */
.menu-item {
  cursor: pointer;
  list-style: none;
}

.menu-item-title {
  transition: all var(--duration-fast) var(--ease-standard);
  font-size: 0.9rem;
  display: flex;
  user-select: none;
  position: relative;
  padding: 0.5rem;
  font-weight: 400;
  border-radius: 8px;
  color: var(--color-text-secondary);
}

.menu-item-title:hover,
.menu-item-title.active {
  background: color-mix(in srgb, var(--color-accent) 60%, transparent);
  color: var(--color-foreground);
  font-weight: 500;
}

/* 激活项左侧竖条（Apple 蓝） */
.menu-item-title.active::after {
  position: absolute;
  top: calc(50% - 13px);
  left: -8px;
  width: 3px;
  height: 26px;
  content: "";
  background: var(--color-primary);
  border-radius: 6px;
}

/* 侧边栏收起模式：隐藏左侧竖条，改用居中胶囊微光高光 */
.sidebar-collapsed .menu-item-title.active::after {
  display: none;
}
.sidebar-collapsed .menu-item-title.active {
  background: color-mix(in srgb, var(--color-primary) 18%, transparent);
  color: var(--color-primary);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-primary) 30%, transparent);
}

/* 缩进子菜单项 */
.menu-subitem {
  transition: all var(--duration-fast) var(--ease-standard);
  font-size: 0.85rem;
  display: flex;
  user-select: none;
  position: relative;
  padding: 0.4rem 0.5rem 0.4rem 2.25rem;
  font-weight: 400;
  border-radius: 8px;
  color: var(--color-text-tertiary);
}
.menu-subitem:hover,
.menu-subitem.active {
  background: color-mix(in srgb, var(--color-accent) 60%, transparent);
  color: var(--color-foreground);
  font-weight: 500;
}

/* 底部用户资料区 */
.profile-placeholder {
  height: 70px;
}

.current-profile {
  height: 70px;
  position: fixed;
  left: 0;
  bottom: 0;
  display: flex;
  gap: 0.75rem;
  background: var(--color-sidebar);
  backdrop-filter: saturate(180%) blur(24px);
  padding: 0.75rem;
  z-index: 10;
  border-top: 1px solid var(--color-border);
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

/* 搜索 snippet 命中词高亮 */
.search-snippet :deep(mark) {
  background: transparent;
  color: var(--color-primary);
  font-weight: 600;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--duration-normal) var(--ease-standard);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition:
    transform var(--duration-slow) var(--ease-standard),
    opacity var(--duration-slow) var(--ease-standard);
}
.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
  opacity: 0;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all var(--duration-normal) var(--ease-standard);
}
.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(4px);
}
</style>
