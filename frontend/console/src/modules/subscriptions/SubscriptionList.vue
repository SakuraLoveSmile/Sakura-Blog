<script setup lang="ts">
import { ref, computed, onMounted } from"vue";
import { Icon } from"@iconify/vue";
import { subscriptionApi } from"@/api";
import { VPageHeader, Toast } from"@/components/ui";

// 友链 RSS 订阅阅读器（对齐 Halo 链接订阅）：左栏订阅源 + 未读计数；右栏文章列表（全部/未读/已读）。
interface FeedSource {
 friend_title: string;
 friend_url: string;
 total: number;
 unread: number;
}
interface Article {
 id: number;
 friend_title: string;
 friend_url: string;
 title: string;
 link: string;
 summary: string;
 published: string;
 read: boolean;
 favorite: boolean;
 later: boolean;
}

const sources = ref<FeedSource[]>([]);
const totalUnread = ref(0);
const totalCount = ref(0);
const articles = ref<Article[]>([]);
const articleTotal = ref(0);
// Spec 62：文章列表分页（原全量拉取，随订阅累积无界渲染）
const articlePage = ref(1);
const articleSize = 50;
const loading = ref(false);
const refreshing = ref(false);

// 当前选中源（"" = 全部动态）与状态过滤（all | unread | read | favorite | later）
const activeFeed = ref("");
const filter = ref<"all" |"unread" |"read" |"favorite" |"later">("all");

const activeSourceTitle = computed(() => {
 if (!activeFeed.value) return"全部动态";
 return sources.value.find((s) => s.friend_url === activeFeed.value)?.friend_title ||"全部动态";
});

async function loadSources() {
 try {
 const { data } = await subscriptionApi.sources();
 sources.value = data.sources || [];
 totalUnread.value = data.unread || 0;
 totalCount.value = data.total || 0;
 } catch {
 /* 静默 */
 }
}

async function loadArticles(append = false) {
 loading.value = true;
 try {
 const { data } = await subscriptionApi.articles({
 feed: activeFeed.value || undefined,
 filter: filter.value,
 page: articlePage.value,
 size: articleSize,
 });
 articleTotal.value = data.total || 0;
 articles.value = append ? [...articles.value, ...(data.items || [])] : (data.items || []);
 } catch (e: any) {
 Toast.error(e.response?.data?.error ||"加载失败");
 } finally {
 loading.value = false;
 }
}

function loadMore() {
 if (articles.value.length >= articleTotal.value) return;
 articlePage.value += 1;
 loadArticles(true);
}

function selectFeed(url: string) {
 activeFeed.value = url;
 articlePage.value = 1;
 loadArticles();
}

function setFilter(f: typeof filter.value) {
 filter.value = f;
 articlePage.value = 1;
 loadArticles();
}

async function refreshAll() {
 refreshing.value = true;
 try {
 await subscriptionApi.refresh();
 Toast.success("已触发抓取，稍后刷新列表查看新文章");
 // 抓取是异步的，给后端一点时间再拉取
 setTimeout(async () => {
 await loadSources();
 await loadArticles();
 refreshing.value = false;
 }, 2500);
 } catch (e: any) {
 Toast.error(e.response?.data?.error ||"刷新失败");
 refreshing.value = false;
 }
}

async function reloadList() {
 await loadSources();
 await loadArticles();
}

async function markRead(a: Article, read = true) {
 if (a.read === read) return;
 a.read = read;
 try {
 await subscriptionApi.markRead(a.id, read);
 await loadSources();
 } catch {
 a.read = !read;
 Toast.error("操作失败");
 }
}

async function markAllRead() {
 try {
 await subscriptionApi.markAllRead(activeFeed.value || undefined);
 articles.value.forEach((a) => (a.read = true));
 await loadSources();
 Toast.success("已全部标为已读");
 } catch (e: any) {
 Toast.error(e.response?.data?.error ||"操作失败");
 }
}

async function toggleFlag(a: Article, flag:"favorite" |"later") {
 const next = !a[flag];
 a[flag] = next;
 try {
 await subscriptionApi.toggleFlag(a.id, flag, next);
 } catch {
 a[flag] = !next;
 Toast.error("操作失败");
 }
}

onMounted(reloadList);

const filterTabs: { key: typeof filter.value; label: string }[] = [
 { key:"all", label:"全部" },
 { key:"unread", label:"未读" },
 { key:"read", label:"已读" },
];
</script>

<template>
 <div>
 <VPageHeader title="链接订阅">
 <template #icon>
 <Icon icon="ri:rss-line" class="mr-2 self-center text-xl" />
 </template>
 <template #actions>
 <div class="flex items-center gap-2">
 <button
 class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary hover:bg-accent"
 @click="setFilter('later')"
 >
 <Icon icon="ri:bookmark-line" />稍后阅读
 </button>
 <button
 class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary hover:bg-accent"
 @click="setFilter('favorite')"
 >
 <Icon icon="ri:star-line" />收藏
 </button>
 <router-link
 to="/friends"
 class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-text-tertiary hover:bg-secondary hover:bg-accent"
 >
 <Icon icon="ri:arrow-left-line" />返回链接
 </router-link>
 </div>
 </template>
 </VPageHeader>

 <div class="m-0 grid grid-cols-1 gap-4 md:m-4 lg:grid-cols-[300px_1fr]">
 <!-- 左栏：订阅源 -->
 <aside class="rounded-xl border border-border bg-card p-3">
 <div class="mb-2 flex items-center justify-between px-1">
 <span class="flex items-center gap-1.5 text-sm font-semibold text-text-secondary">
 <Icon icon="ri:rss-line" class="text-primary" />RSS 订阅
 </span>
 <span class="rounded-full bg-secondary px-2 py-0.5 text-xs text-text-tertiary">{{ sources.length }}</span>
 </div>
 <ul class="space-y-0.5">
 <li>
 <button
 class="flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left transition-colors"
 :class="!activeFeed ? 'bg-primary/15 text-primary' : 'hover:bg-secondary hover:bg-accent'"
 @click="selectFeed('')"
 >
 <Icon icon="ri:stack-line" class="text-lg text-text-quaternary" />
 <span class="flex-1">
 <span class="block text-sm font-medium">全部动态</span>
 <span class="block text-xs text-text-quaternary">所有订阅源</span>
 </span>
 <span v-if="totalUnread" class="rounded-full bg-primary/15 px-1.5 text-xs text-primary">{{ totalUnread }}</span>
 </button>
 </li>
 <li v-for="s in sources" :key="s.friend_url">
 <button
 class="flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left transition-colors"
 :class="activeFeed === s.friend_url ? 'bg-primary/15 text-primary' : 'hover:bg-secondary hover:bg-accent'"
 @click="selectFeed(s.friend_url)"
 >
 <Icon icon="ri:rss-fill" class="text-lg text-state-warning" />
 <span class="min-w-0 flex-1">
 <span class="block truncate text-sm font-medium">{{ s.friend_title }}</span>
 <span class="block truncate text-xs text-text-quaternary">{{ s.friend_url }}</span>
 </span>
 <span v-if="s.unread" class="shrink-0 rounded-full bg-primary/15 px-1.5 text-xs text-primary">{{ s.unread }}</span>
 </button>
 </li>
 </ul>
 <p v-if="sources.length === 0" class="px-2 py-8 text-center text-sm text-text-quaternary">
 暂无订阅源。请在友链中配置带 RSS 的链接，再点「刷新列表」抓取。
 </p>
 </aside>

 <!-- 右栏：RSS 动态 -->
 <section class="space-y-4">
 <!-- 概览卡 -->
 <div class="rounded-xl border border-border bg-card p-4">
 <div class="mb-3 flex items-center gap-2">
 <Icon icon="ri:rss-line" class="text-lg text-primary" />
 <span class="text-base font-semibold text-foreground">{{ activeSourceTitle }}</span>
 </div>
 <p class="mb-3 text-sm text-text-tertiary">
 {{ sources.length }} 个订阅 / {{ totalCount }} 篇缓存 / {{ totalUnread }} 篇未读
 </p>
 <div class="mb-3 flex gap-0.5 rounded-lg bg-secondary p-0.5 filter-tabs">
 <button
  v-for="t in filterTabs"
  :key="t.key"
  class="flex-1 rounded-md py-1.5 text-sm transition-colors"
  :class="filter === t.key ? 'bg-card font-medium text-foreground shadow-sm ' : 'text-text-tertiary hover:text-text-secondary '"
  @click="setFilter(t.key)"
 >{{ t.label }}</button>
</div>
 <div class="flex flex-wrap gap-2">
 <button
 :disabled="refreshing"
 class="flex items-center gap-1.5 rounded-lg border border-border px-2.5 py-1.5 text-xs text-text-tertiary hover:bg-secondary disabled:opacity-50 hover:bg-accent"
 @click="refreshAll"
 >
 <Icon :icon="refreshing ? 'ri:loader-4-line' : 'ri:download-cloud-2-line'" :class="refreshing && 'animate-spin'" />获取全部
 </button>
 <button
 class="flex items-center gap-1.5 rounded-lg border border-border px-2.5 py-1.5 text-xs text-text-tertiary hover:bg-secondary hover:bg-accent"
 @click="markAllRead"
 >
 <Icon icon="ri:mail-open-line" />全部标为已读
 </button>
 <button
 class="flex items-center gap-1.5 rounded-lg border border-border px-2.5 py-1.5 text-xs text-text-tertiary hover:bg-secondary hover:bg-accent"
 @click="reloadList"
 >
 <Icon icon="ri:refresh-line" />刷新列表
 </button>
 </div>
 </div>

 <!-- 文章列表 -->
 <div v-if="loading" class="py-12 text-center text-sm text-text-quaternary">加载中…</div>
 <p v-else-if="articles.length === 0" class="rounded-xl border border-dashed border-border py-12 text-center text-sm text-text-quaternary">
 暂无文章。点「获取全部」抓取 RSS，或调整筛选。
 </p>
 <div
 v-for="a in articles"
 v-else
 :key="a.id"
 class="rounded-xl border border-border bg-card p-4 transition-shadow hover:shadow-sm"
 >
 <div class="mb-1 flex items-center gap-2 text-xs text-text-quaternary">
 <span v-if="!a.read" class="h-2 w-2 rounded-full bg-primary"></span>
 <span class="font-medium text-text-tertiary">{{ a.friend_title }}</span>
 <span>{{ a.published }}</span>
 <span class="flex-1"></span>
 <button title="收藏" class="rounded p-1 hover:bg-secondary hover:bg-accent" :class="a.favorite ? 'text-state-warning' : 'text-text-quaternary'" @click="toggleFlag(a, 'favorite')">
 <Icon :icon="a.favorite ? 'ri:star-fill' : 'ri:star-line'" />
 </button>
 <button title="稍后阅读" class="rounded p-1 hover:bg-secondary hover:bg-accent" :class="a.later ? 'text-primary' : 'text-text-quaternary'" @click="toggleFlag(a, 'later')">
 <Icon :icon="a.later ? 'ri:bookmark-fill' : 'ri:bookmark-line'" />
 </button>
 <button :title="a.read ? '标为未读' : '标为已读'" class="rounded p-1 text-text-quaternary hover:bg-secondary hover:bg-accent" @click="markRead(a, !a.read)">
 <Icon :icon="a.read ? 'ri:mail-line' : 'ri:mail-open-line'" />
 </button>
 </div>
 <a
 :href="a.link"
 target="_blank"
 class="flex items-start gap-1 text-base font-semibold text-foreground hover:text-primary"
 @click="markRead(a, true)"
 >
 <span>{{ a.title }}</span>
 <Icon icon="ri:external-link-line" class="mt-1 shrink-0 text-sm text-text-quaternary" />
 </a>
 <p v-if="a.summary" class="mt-1.5 line-clamp-2 text-sm text-text-tertiary">{{ a.summary }}</p>
 <div class="mt-2">
 <span
  class="rounded px-1.5 py-0.5 text-xs feed-badge"
  :class="a.read ? 'bg-secondary text-text-quaternary ' : 'bg-primary/15 text-primary'"
 >{{ a.read ?"已读" :"未读" }}</span>
 </div>
 </div>
 <!-- Spec 62：分页加载更多（原全量渲染，随订阅累积无界） -->
 <div v-if="articles.length < articleTotal" class="mt-4 flex justify-center">
 <button class="rounded-full border border-border px-5 py-2 text-sm text-text-tertiary transition-colors hover:bg-secondary hover:text-foreground" :disabled="loading" @click="loadMore">
 {{ loading ?"加载中…" :"加载更多" }}
 </button>
 </div>
 </section>
 </div>
 </div>
</template>

<style scoped>
/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  /* 订阅源侧边栏更紧凑 */
  aside.rounded-xl {
    padding: 10px !important;
  }
  aside .text-sm.font-semibold {
    font-size: 13px;
  }
  aside .rounded-lg {
    padding: 8px !important;
  }
  aside .text-sm {
    font-size: 12.5px;
  }
  aside .text-xs {
    font-size: 11px;
  }
}

@media (max-width: 640px) {
  .grid {
    gap: 12px !important;
  }

  /* 概览卡更紧凑 */
  section > .rounded-xl:first-child {
    padding: 12px !important;
  }
  .text-base.font-semibold {
    font-size: 14px !important;
  }
  .text-sm.text-text-tertiary {
    font-size: 12px;
  }
  /* 分段筛选条（全部/未读/已读） */
  .filter-tabs {
    margin-bottom: 8px !important;
  }
  .filter-tabs button {
    font-size: 12px;
    padding: 6px 8px !important;
  }

  /* 操作按钮：文字更小 */
  .flex.flex-wrap.gap-2 button {
    font-size: 11px;
    padding: 5px 10px !important;
    height: 32px;
  }

  /* 文章列表项更紧凑 */
  section .space-y-4 {
    gap: 10px !important;
  }
  section .space-y-4 .rounded-xl {
    padding: 12px !important;
  }
  section .space-y-4 .text-base {
    font-size: 13.5px !important;
  }
  section .space-y-4 .text-sm.text-text-tertiary {
    font-size: 12px;
  }

  /* 已读/未读徽章 */
  .feed-badge {
    font-size: 10.5px !important;
    padding: 2px 8px !important;
  }
}
</style>
