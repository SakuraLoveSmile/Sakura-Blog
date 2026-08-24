<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { assetsApi, type AssetIconItem } from "@/api";
import { Toast, VLoading } from "@/components/ui";
import { emojiSources } from "./assets-data";

// 素材速查（插件库扩展）：
// - 图标画廊：枚举后端 icons.yml 全量 key（含 default:*/example:*/quot:*/chat:*/github:*/share:*），
//   点击复制 {% icon key %}；key 可直接填入分类/标签图标配置（article.category_icon/tag_icon）。
// - 表情画廊：按来源浏览可用表情名，点击复制 {% emoji [来源] 名称 %}。

const subTab = ref<"icons" | "emoji">("icons");

// ---------- 图标 ----------

const icons = ref<AssetIconItem[]>([]);
const iconsLoading = ref(false);
const iconFilter = ref("");

async function loadIcons() {
  if (icons.value.length) return;
  iconsLoading.value = true;
  try {
    const { data } = await assetsApi.icons();
    icons.value = data.items || [];
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "加载图标失败");
  } finally {
    iconsLoading.value = false;
  }
}

const iconGroups = computed(() => {
  const kw = iconFilter.value.trim().toLowerCase();
  const list = kw ? icons.value.filter((i) => i.key.toLowerCase().includes(kw)) : icons.value;
  const groups: { prefix: string; items: AssetIconItem[] }[] = [];
  const byPrefix = new Map<string, AssetIconItem[]>();
  for (const item of list) {
    const prefix = item.key.split(":")[0] || "other";
    if (!byPrefix.has(prefix)) byPrefix.set(prefix, []);
    byPrefix.get(prefix)!.push(item);
  }
  for (const [prefix, items] of byPrefix) groups.push({ prefix, items });
  return groups;
});

function isUrl(v: string): boolean {
  return v.startsWith("http://") || v.startsWith("https://") || v.startsWith("/");
}

// ---------- 表情 ----------

const activeEmojiSource = ref(emojiSources[0].id);
const emojiFilter = ref("");

const currentEmojiSource = computed(
  () => emojiSources.find((s) => s.id === activeEmojiSource.value) || emojiSources[0],
);

const filteredEmojiNames = computed(() => {
  const kw = emojiFilter.value.trim().toLowerCase();
  if (!kw) return currentEmojiSource.value.names;
  return currentEmojiSource.value.names.filter((n) => n.toLowerCase().includes(kw));
});

function emojiUrl(name: string): string {
  return currentEmojiSource.value.urlTemplate.replace("{name}", name);
}

// ---------- 复制 ----------

const copiedKey = ref("");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    document.body.removeChild(ta);
  }
  copiedKey.value = text;
  clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => (copiedKey.value = ""), 1500);
  Toast.success(`已复制：${text}`);
}

function copyIcon(key: string) {
  copyText(`{% icon ${key} %}`);
}

function copyEmoji(name: string) {
  const src = currentEmojiSource.value.id;
  copyText(src === "default" ? `{% emoji ${name} %}` : `{% emoji ${src} ${name} %}`);
}

onMounted(loadIcons);
</script>

<template>
  <div>
    <!-- 子分类切换 -->
    <div class="apple-segmented mb-5" style="max-width: 320px">
      <button
        type="button"
        :class="{ active: subTab === 'icons' }"
        @click="
          subTab = 'icons';
          loadIcons();
        "
      >
        图标（icons.yml）
      </button>
      <button type="button" :class="{ active: subTab === 'emoji' }" @click="subTab = 'emoji'">表情（Emoji）</button>
    </div>

    <!-- ============ 图标画廊 ============ -->
    <section v-if="subTab === 'icons'">
      <div class="mb-4 flex items-center gap-3">
        <div class="apple-search" style="width: 260px">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="iconFilter" placeholder="搜索图标 key..." />
        </div>
        <p class="text-xs text-text-quaternary">
          点击复制 <code class="rounded bg-accent px-1 py-0.5 font-mono">{% icon key %}</code>；key
          也可直接填入「分类与标签」页的图标配置。
        </p>
      </div>

      <VLoading v-if="iconsLoading" />
      <div v-else class="space-y-6">
        <div v-for="g in iconGroups" :key="g.prefix">
          <h3 class="mb-2 text-[13px] font-semibold tracking-tight text-text-secondary">
            {{ g.prefix }}:*
            <span class="ml-1 font-normal text-text-quaternary">{{ g.items.length }} 个</span>
          </h3>
          <div class="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8">
            <button
              v-for="item in g.items"
              :key="item.key"
              type="button"
              :title="`点击复制 {% icon ${item.key} %}`"
              class="asset-cell group flex flex-col items-center gap-1.5 rounded-xl border border-border bg-card px-2 py-3 transition-all duration-150 hover:-translate-y-px hover:border-primary hover:shadow-[var(--shadow-md)]"
              @click="copyIcon(item.key)"
            >
              <span class="asset-svg flex h-8 w-8 items-center justify-center text-foreground">
                <img v-if="isUrl(item.value)" :src="item.value" class="h-7 w-7 object-contain" alt="" />
                <span v-else v-html="item.value"></span>
              </span>
              <span
                class="w-full truncate text-center font-mono text-[10px] text-text-tertiary group-hover:text-foreground"
              >
                {{ copiedKey === `{% icon ${item.key} %}` ? "已复制 ✓" : item.key }}
              </span>
            </button>
          </div>
        </div>
        <p v-if="iconGroups.length === 0" class="text-sm text-text-quaternary">没有匹配的图标。</p>
      </div>
    </section>

    <!-- ============ 表情画廊 ============ -->
    <section v-else>
      <div class="mb-4 flex flex-wrap items-center gap-2">
        <button
          v-for="s in emojiSources"
          :key="s.id"
          type="button"
          :class="[
            'rounded-full border px-3 py-1 text-xs font-medium transition-colors',
            activeEmojiSource === s.id
              ? 'border-primary bg-primary text-foreground'
              : 'border-border bg-card text-text-tertiary hover:border-primary hover:text-foreground',
          ]"
          @click="activeEmojiSource = s.id"
        >
          {{ s.label }}（{{ s.names.length }}）
        </button>
      </div>
      <div class="mb-4 flex items-center gap-3">
        <div class="apple-search" style="width: 260px">
          <Icon icon="ri:search-line" class="text-text-quaternary" />
          <input v-model="emojiFilter" placeholder="搜索表情名称..." />
        </div>
        <p class="text-xs text-text-quaternary">
          点击复制
          <code class="rounded bg-accent px-1 py-0.5 font-mono"
            >{% emoji {{ currentEmojiSource.id === "default" ? "名称" : currentEmojiSource.id + " 名称" }} %}</code
          >
        </p>
      </div>

      <div class="grid grid-cols-4 gap-2 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10">
        <button
          v-for="name in filteredEmojiNames"
          :key="name"
          type="button"
          class="asset-cell group flex flex-col items-center gap-1 rounded-xl border border-border bg-card px-1 py-2.5 transition-all duration-150 hover:-translate-y-px hover:border-primary hover:shadow-[var(--shadow-md)]"
          @click="copyEmoji(name)"
        >
          <img :src="emojiUrl(name)" class="h-8 w-8 object-contain" loading="lazy" :alt="name" />
          <span
            class="w-full truncate text-center font-mono text-[10px] text-text-tertiary group-hover:text-foreground"
          >
            {{ name }}
          </span>
        </button>
      </div>
      <p v-if="filteredEmojiNames.length === 0" class="text-sm text-text-quaternary">没有匹配的表情。</p>
    </section>
  </div>
</template>

<style scoped>
/* icons.yml 内联 SVG 自带 width/height 属性（32px），统一收敛到 24px 并继承前景色 */
.asset-svg :deep(svg) {
  width: 24px;
  height: 24px;
  color: inherit;
}
</style>
