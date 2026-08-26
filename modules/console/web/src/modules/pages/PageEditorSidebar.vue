<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { settingsApi } from "@/api";
import AttachmentField from "@/components/AttachmentField.vue";
import { Toast } from "@/components/ui";

// 页面编辑器右侧辅助面板（参考 PostEditor 的 EditorSidebar）：
// 大纲：解析 Markdown 标题可点击跳转；详情：slug/status + 侧边栏组件参数编辑（快速入口）
interface PageForm {
  slug: string;
  title: string;
  content_md: string;
  status: string;
}

type SidebarWidget = { name: string; params: Record<string, any> };

const props = defineProps<{
  form: PageForm;
  leftbarWidgets: SidebarWidget[] | null;
  rightbarWidgets: SidebarWidget[] | null;
  widgetLibrary: { name: string; label: string; hint?: string }[];
  knownWidgetFields: Record<string, { key: string; label: string; type: string; placeholder?: string }[]>;
}>();

const emit = defineEmits<{
  (e: "jump", line: number, index: number): void;
  (e: "openSidebarConfig"): void;
}>();

const activeTab = ref<"outline" | "detail">("outline");

// ---------- 大纲：解析 ATX 标题（跳过代码围栏），点击跳转到对应行 ----------
interface OutlineItem {
  level: number;
  text: string;
  line: number;
}

const outline = computed<OutlineItem[]>(() => {
  const items: OutlineItem[] = [];
  let inFence = false;
  const lines = (props.form.content_md || "").split("\n");
  lines.forEach((raw, i) => {
    const line = raw.trim();
    if (/^(```|~~~)/.test(line)) {
      inFence = !inFence;
      return;
    }
    if (inFence) return;
    const m = /^(#{1,6})\s+(.+?)\s*#*\s*$/.exec(line);
    if (m) items.push({ level: m[1].length, text: m[2], line: i });
  });
  return items;
});

const minLevel = computed(() => (outline.value.length ? Math.min(...outline.value.map((i) => i.level)) : 1));

function widgetLabelOf(name: string) {
  return props.widgetLibrary.find((w) => w.name === name)?.label ?? name;
}

// ---------- 友链页专属设置（slug=friends 时右侧栏显示，存 settings pages.friends 段） ----------
const isFriendsPage = computed(() => props.form.slug === "friends");
const friendsCfg = ref({
  hero: { title: "", description: "", background: "" },
  refresh_interval: 0,
  posts_per_link: 0,
});
const friendsSaving = ref(false);

async function loadFriendsCfg() {
  if (!isFriendsPage.value) return;
  try {
    const { data } = await settingsApi.all();
    const f = data?.pages?.friends || {};
    friendsCfg.value = {
      hero: {
        title: f.hero?.title || "",
        description: f.hero?.description || "",
        background: f.hero?.background || "",
      },
      refresh_interval: f.refresh_interval || 0,
      posts_per_link: f.posts_per_link || 0,
    };
  } catch {
    // 设置加载失败不阻断编辑器
  }
}

async function saveFriendsCfg() {
  friendsSaving.value = true;
  try {
    const hero = friendsCfg.value.hero;
    const hasHero = Boolean(hero.title || hero.description || hero.background);
    await settingsApi.updateSection("pages", {
      friends: {
        hero: hasHero ? hero : undefined,
        refresh_interval: Number(friendsCfg.value.refresh_interval) || 0,
        posts_per_link: Number(friendsCfg.value.posts_per_link) || 0,
      },
    });
    Toast.success("已保存，前台 /friends 已生效");
  } catch {
  } finally {
    friendsSaving.value = false;
  }
}

onMounted(loadFriendsCfg);
</script>

<template>
  <div
    class="flex h-full flex-col overflow-hidden rounded-xl border border-border bg-white dark:border-border-strong dark:bg-secondary"
  >
    <!-- Tabs -->
    <div class="grid grid-cols-2 gap-0.5 rounded-t-xl bg-secondary p-1 dark:bg-accent/50">
      <button
        :class="[
          'rounded-lg px-2 py-1.5 text-xs font-medium transition-colors',
          activeTab === 'outline'
            ? 'bg-white text-text-quaternary shadow-sm dark:bg-secondary dark:text-foreground'
            : 'text-text-quaternary hover:text-text-quaternary dark:text-muted-foreground dark:hover:text-foreground',
        ]"
        @click="activeTab = 'outline'"
      >
        <span class="inline-flex items-center gap-1">
          <Icon icon="ri:list-unordered" class="h-3.5 w-3.5" />
          大纲
          <span
            class="ml-1 rounded-full bg-accent px-1.5 text-[10px] text-text-quaternary dark:bg-accent dark:text-text-secondary"
            >{{ outline.length }}</span
          >
        </span>
      </button>
      <button
        :class="[
          'rounded-lg px-2 py-1.5 text-xs font-medium transition-colors',
          activeTab === 'detail'
            ? 'bg-white text-text-quaternary shadow-sm dark:bg-secondary dark:text-foreground'
            : 'text-text-quaternary hover:text-text-quaternary dark:text-muted-foreground dark:hover:text-foreground',
        ]"
        @click="activeTab = 'detail'"
      >
        <span class="inline-flex items-center gap-1">
          <Icon icon="ri:settings-4-line" class="h-3.5 w-3.5" />
          详情
        </span>
      </button>
    </div>

    <!-- Outline Tab -->
    <div v-if="activeTab === 'outline'" class="flex-1 overflow-y-auto p-3">
      <div v-if="!outline.length" class="flex flex-col items-center justify-center py-8 text-center">
        <Icon icon="ri:article-line" class="mb-2 h-8 w-8 text-text-secondary dark:text-text-quaternary" />
        <p class="text-xs text-muted-foreground">暂无标题，添加 # 标题后可在此跳转</p>
      </div>
      <ol v-else class="space-y-0.5">
        <li
          v-for="(it, idx) in outline"
          :key="idx"
          class="group cursor-pointer rounded-md px-2 py-1 text-xs hover:bg-secondary dark:hover:bg-accent/40"
          :style="{ paddingLeft: `${8 + (it.level - minLevel) * 12}px` }"
          @click="emit('jump', it.line, idx)"
        >
          <span
            class="truncate text-text-quaternary group-hover:text-foreground dark:text-text-secondary dark:group-hover:text-foreground"
            >{{ it.text }}</span
          >
        </li>
      </ol>
    </div>

    <!-- Detail Tab -->
    <div v-else class="flex-1 overflow-y-auto p-3 space-y-4">
      <div class="space-y-2">
        <h3 class="text-xs font-semibold uppercase tracking-wide text-text-quaternary dark:text-muted-foreground">
          页面信息
        </h3>
        <div class="space-y-2 rounded-lg bg-secondary p-3 text-xs dark:bg-accent/40">
          <div class="flex items-center justify-between gap-2">
            <span class="text-text-quaternary dark:text-muted-foreground">标题</span>
            <span class="truncate font-medium text-text-quaternary dark:text-foreground">{{
              form.title || "(未填)"
            }}</span>
          </div>
          <div class="flex items-center justify-between gap-2">
            <span class="text-text-quaternary dark:text-muted-foreground">Slug</span>
            <span class="truncate font-mono text-[11px] text-text-quaternary dark:text-foreground">{{
              form.slug || "(未填)"
            }}</span>
          </div>
          <div class="flex items-center justify-between gap-2">
            <span class="text-text-quaternary dark:text-muted-foreground">状态</span>
            <span
              :class="[
                'rounded-full px-2 py-0.5 text-[11px]',
                form.status === 'published'
                  ? 'bg-state-success/10 text-state-success dark:bg-state-success/100/20 dark:text-state-success'
                  : 'bg-state-warning/10 text-state-warning dark:bg-state-warning/20 dark:text-state-warning',
              ]"
              >{{ form.status === "published" ? "已发布" : "草稿" }}</span
            >
          </div>
          <div class="flex items-center justify-between gap-2">
            <span class="text-text-quaternary dark:text-muted-foreground">字数</span>
            <span class="font-mono text-[11px] text-text-quaternary dark:text-text-secondary">
              {{ form.content_md ? [...form.content_md].filter((c) => !/\s/.test(c)).length : 0 }}
            </span>
          </div>
        </div>
      </div>

      <!-- 友链页专属设置（slug=friends 时显示，存 settings pages.friends 段） -->
      <div v-if="isFriendsPage" class="space-y-2">
        <h3 class="text-xs font-semibold uppercase tracking-wide text-text-quaternary dark:text-muted-foreground">
          友链页设置
        </h3>
        <div class="space-y-2 rounded-lg bg-secondary p-3 text-xs dark:bg-accent/40">
          <div>
            <label class="mb-1 block text-[10px] text-muted-foreground">hero 副标题（banner 签名档）</label>
            <input
              v-model="friendsCfg.hero.description"
              class="w-full rounded-base border border-border bg-background px-2 py-1 text-xs outline-none focus:border-primary dark:border-border-strong dark:bg-secondary"
            />
          </div>
          <div>
            <label class="mb-1 block text-[10px] text-muted-foreground">hero 背景图 URL</label>
            <AttachmentField v-model="friendsCfg.hero.background" placeholder="https://..." show-thumb />
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div>
              <label class="mb-1 block text-[10px] text-muted-foreground">刷新间隔（分钟）</label>
              <input
                v-model.number="friendsCfg.refresh_interval"
                type="number"
                class="w-full rounded-base border border-border bg-background px-2 py-1 text-xs outline-none focus:border-primary dark:border-border-strong dark:bg-secondary"
                placeholder="0=缺省 60"
              />
            </div>
            <div>
              <label class="mb-1 block text-[10px] text-muted-foreground">每链文章数</label>
              <input
                v-model.number="friendsCfg.posts_per_link"
                type="number"
                class="w-full rounded-base border border-border bg-background px-2 py-1 text-xs outline-none focus:border-primary dark:border-border-strong dark:bg-secondary"
                placeholder="0=缺省 20"
              />
            </div>
          </div>
          <p class="text-[10px] text-muted-foreground">
            RSS 文章列表开关在正文 shortcode：{% friends posts:true|false %}
          </p>
          <button
            class="w-full rounded-md bg-primary px-2 py-1.5 text-[11px] font-medium text-primary-foreground transition-colors hover:bg-primary-hover disabled:opacity-50"
            :disabled="friendsSaving"
            @click="saveFriendsCfg"
          >
            {{ friendsSaving ? "保存中…" : "保存友链页设置" }}
          </button>
        </div>
      </div>

      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <h3 class="text-xs font-semibold uppercase tracking-wide text-text-quaternary dark:text-muted-foreground">
            侧边栏配置
          </h3>
          <button
            class="rounded-md px-2 py-1 text-[11px] text-primary hover:bg-primary/10"
            @click="emit('openSidebarConfig')"
          >
            <Icon icon="ri:arrow-up-down-line" class="mr-0.5 inline h-3 w-3 align-middle" />
            打开完整配置
          </button>
        </div>
        <div class="space-y-2 text-xs">
          <div class="rounded-lg bg-secondary p-3 dark:bg-accent/40">
            <div class="mb-1 flex items-center justify-between">
              <span class="font-medium text-text-quaternary dark:text-text-secondary">左栏</span>
              <span class="text-[10px] text-muted-foreground">
                {{ leftbarWidgets === null ? "继承全局" : `自定义 · ${leftbarWidgets.length} 组件` }}
              </span>
            </div>
            <div v-if="leftbarWidgets?.length" class="mt-1 flex flex-wrap gap-1">
              <span
                v-for="w in leftbarWidgets"
                :key="w.name"
                class="inline-flex items-center gap-1 rounded-full bg-white px-1.5 py-0.5 text-[10px] text-text-quaternary ring-1 ring-border dark:bg-secondary dark:text-foreground dark:ring-border"
              >
                <Icon icon="ri-layout-left-line" class="h-3 w-3 text-muted-foreground" />
                {{ widgetLabelOf(w.name) }}
                <span
                  v-if="w.params && Object.keys(w.params).length"
                  class="ml-0.5 rounded-full bg-primary/10 px-1 text-primary"
                >
                  {{ Object.keys(w.params).length }}
                </span>
              </span>
            </div>
            <p
              v-else-if="Array.isArray(leftbarWidgets) && !leftbarWidgets.length"
              class="mt-1 text-[10px] text-muted-foreground"
            >
              （显式空左栏）
            </p>
          </div>
          <div class="rounded-lg bg-secondary p-3 dark:bg-accent/40">
            <div class="mb-1 flex items-center justify-between">
              <span class="font-medium text-text-quaternary dark:text-text-secondary">右栏</span>
              <span class="text-[10px] text-muted-foreground">
                {{ rightbarWidgets === null ? "继承全局" : `自定义 · ${rightbarWidgets.length} 组件` }}
              </span>
            </div>
            <div v-if="rightbarWidgets?.length" class="mt-1 flex flex-wrap gap-1">
              <span
                v-for="w in rightbarWidgets"
                :key="w.name"
                class="inline-flex items-center gap-1 rounded-full bg-white px-1.5 py-0.5 text-[10px] text-text-quaternary ring-1 ring-border dark:bg-secondary dark:text-foreground dark:ring-border"
              >
                <Icon icon="ri-layout-right-line" class="h-3 w-3 text-muted-foreground" />
                {{ widgetLabelOf(w.name) }}
                <span
                  v-if="w.params && Object.keys(w.params).length"
                  class="ml-0.5 rounded-full bg-primary/10 px-1 text-primary"
                >
                  {{ Object.keys(w.params).length }}
                </span>
              </span>
            </div>
            <p
              v-else-if="Array.isArray(rightbarWidgets) && !rightbarWidgets.length"
              class="mt-1 text-[10px] text-muted-foreground"
            >
              （显式空右栏）
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
