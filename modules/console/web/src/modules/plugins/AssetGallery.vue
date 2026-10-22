<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { assetsApi, type AssetIconItem } from "@/api";
import { Toast, VLoading, VTabbar, VPageHeader } from "@/components/ui";
import { emojiSources } from "./assets-data";

// 素材速查（插件库扩展）：
// - 图标画廊：枚举后端 icons.yml 全量 key（含 default:*/example:*/quot:*/chat:*/github:*/share:*），
//   点击复制 {% icon key %}；key 可直接填入分类/标签图标配置（article.category_icon/tag_icon）。
// - 表情画廊：按来源浏览可用表情名，点击复制 {% emoji [来源] 名称 %}。

const subTab = ref<"icons" | "emoji">("icons");
const subTabs = [
  { id: "icons", label: "图标画廊 (icons.yml)", icon: "ri:remixicon-line" },
  { id: "emoji", label: "表情画廊 (Emoji)", icon: "ri:emotion-happy-line" },
];

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
  } catch {
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
    <VPageHeader
      title="素材速查"
      subtitle="图标与表情速查：点击即复制 shortcode，key 可直接用于分类 / 标签图标配置。"
    />
    <div class="mx-auto max-w-[1100px] px-4 py-5 sm:px-6 md:px-8">
      <div class="asset-gallery-view">
    <!-- 子分类切换 -->
    <div class="asset-subtabs-wrapper">
      <VTabbar
        v-model:active-id="subTab"
        :items="subTabs"
        type="segmented"
        @change="(id) => { if (id === 'icons') loadIcons(); }"
      />
    </div>

    <!-- ============ 图标画廊 ============ -->
    <section v-if="subTab === 'icons'" class="asset-main-section">
      <!-- 搜索与辅助栏 -->
      <div class="asset-search-bar">
        <div class="asset-search-input-wrap">
          <Icon icon="ri:search-line" class="asset-search-icon" />
          <input
            v-model="iconFilter"
            placeholder="搜索图标 key（如 github、mail、link）..."
            class="asset-search-input"
          />
          <button
            v-if="iconFilter"
            type="button"
            class="asset-search-clear"
            title="清空"
            @click="iconFilter = ''"
          >
            <Icon icon="ri:close-circle-fill" />
          </button>
        </div>
        <p class="asset-search-hint">
          点击图标卡片可直接复制 <code class="hint-code">{{ "{%" }} icon key {{ "%}" }}</code>，key 亦可直接用于分类与标签图标配置。
        </p>
      </div>

      <VLoading v-if="iconsLoading" />
      <div v-else class="asset-groups-list">
        <div v-for="g in iconGroups" :key="g.prefix" class="asset-group-card">
          <div class="asset-group-header">
            <h3 class="asset-group-title">
              <code>{{ g.prefix }}:*</code>
            </h3>
            <span class="asset-group-count">{{ g.items.length }} 个图标</span>
          </div>
          <div class="asset-grid asset-grid-icons">
            <button
              v-for="item in g.items"
              :key="item.key"
              type="button"
              :title="`点击复制 {% icon ${item.key} %}`"
              :class="['asset-cell', { 'is-copied': copiedKey === `{% icon ${item.key} %}` }]"
              @click="copyIcon(item.key)"
            >
              <span class="asset-icon-display">
                <img v-if="isUrl(item.value)" :src="item.value" class="asset-img" alt="" />
                <span v-else class="asset-svg" v-html="item.value"></span>
              </span>
              <span class="asset-key-label">
                {{ copiedKey === `{% icon ${item.key} %}` ? "已复制 ✓" : item.key }}
              </span>
            </button>
          </div>
        </div>
        <div v-if="iconGroups.length === 0" class="asset-empty-state">
          <Icon icon="ri:search-eye-line" width="32" height="32" />
          <p>没有找到匹配的图标</p>
        </div>
      </div>
    </section>

    <!-- ============ 表情画廊 ============ -->
    <section v-else class="asset-main-section">
      <!-- 来源选择胶囊 -->
      <div class="emoji-sources-rail">
        <button
          v-for="s in emojiSources"
          :key="s.id"
          type="button"
          :class="['emoji-source-pill', { active: activeEmojiSource === s.id }]"
          @click="activeEmojiSource = s.id"
        >
          <span>{{ s.label }}</span>
          <span class="source-count">{{ s.names.length }}</span>
        </button>
      </div>

      <!-- 搜索与提示 -->
      <div class="asset-search-bar">
        <div class="asset-search-input-wrap">
          <Icon icon="ri:search-line" class="asset-search-icon" />
          <input
            v-model="emojiFilter"
            placeholder="搜索表情名称..."
            class="asset-search-input"
          />
          <button
            v-if="emojiFilter"
            type="button"
            class="asset-search-clear"
            title="清空"
            @click="emojiFilter = ''"
          >
            <Icon icon="ri:close-circle-fill" />
          </button>
        </div>
        <p class="asset-search-hint">
          点击表情卡片可直接复制
          <code class="hint-code">{{ "{%" }} emoji {{ currentEmojiSource.id === "default" ? "名称" : currentEmojiSource.id + " 名称" }} {{ "%}" }}</code>
        </p>
      </div>

      <div class="asset-group-card">
        <div class="asset-grid asset-grid-emojis">
          <button
            v-for="name in filteredEmojiNames"
            :key="name"
            type="button"
            :title="`点击复制 {% emoji ${currentEmojiSource.id === 'default' ? name : currentEmojiSource.id + ' ' + name} %}`"
            :class="['asset-cell', 'asset-cell-emoji', { 'is-copied': copiedKey.includes(name) }]"
            @click="copyEmoji(name)"
          >
            <img :src="emojiUrl(name)" class="emoji-img" loading="lazy" :alt="name" />
            <span class="asset-key-label">
              {{ copiedKey.includes(name) ? "已复制 ✓" : name }}
            </span>
          </button>
        </div>
        <div v-if="filteredEmojiNames.length === 0" class="asset-empty-state">
          <Icon icon="ri:search-eye-line" width="32" height="32" />
          <p>没有找到匹配的表情</p>
        </div>
      </div>
    </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.asset-gallery-view {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.asset-subtabs-wrapper {
  max-width: 360px;
}

.asset-main-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.asset-search-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.asset-search-input-wrap {
  position: relative;
  width: 280px;
  display: flex;
  align-items: center;
}

.asset-search-icon {
  position: absolute;
  left: 10px;
  color: var(--color-text-quaternary);
  font-size: 15px;
  pointer-events: none;
}

.asset-search-input {
  width: 100%;
  box-sizing: border-box;
  padding: 8px 28px 8px 32px;
  font-size: 13px;
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 50%, var(--color-card));
  color: var(--color-foreground);
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease;
}

.asset-search-input:focus {
  outline: none;
  border-color: var(--color-primary);
}

.asset-search-clear {
  position: absolute;
  right: 8px;
  background: transparent;
  border: none;
  color: var(--color-text-quaternary);
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
}
.asset-search-clear:hover {
  color: var(--color-foreground);
}

.asset-search-hint {
  margin: 0;
  font-size: 12.5px;
  color: var(--color-text-tertiary);
}

.hint-code {
  font-family: var(--font-mono, monospace);
  padding: 2px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--color-foreground) 8%, transparent);
  color: var(--color-foreground);
  font-size: 12px;
}

.asset-groups-list {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.asset-group-card {
  background: color-mix(in srgb, var(--color-background) 38%, var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  padding: 16px 20px;
}

.asset-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
}

.asset-group-title {
  margin: 0;
  font-size: 14px;
  font-weight: 650;
  color: var(--color-foreground);
  display: flex;
  align-items: center;
  gap: 8px;
}

.asset-group-title::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 14px;
  border-radius: 2px;
  background: var(--color-primary);
}

.asset-group-count {
  font-size: 11.5px;
  color: var(--color-text-quaternary);
}

.asset-grid {
  display: grid;
  gap: 8px;
}

.asset-grid-icons {
  grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
}

.asset-grid-emojis {
  grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
}

.asset-cell {
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg, 12px);
  padding: 12px 8px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  transition: background-color 0.2s cubic-bezier(0.16, 1, 0.3, 1), border-color 0.2s cubic-bezier(0.16, 1, 0.3, 1), color 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  text-align: center;
}

.asset-cell:hover {
  transform: translateY(-2px);
  border-color: var(--color-primary);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  background: color-mix(in srgb, var(--color-primary) 6%, var(--color-card));
}

.asset-cell.is-copied {
  border-color: #30d158;
  background: rgba(48, 209, 88, 0.12);
}

.asset-icon-display {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  color: var(--color-foreground);
}

.asset-img {
  width: 26px;
  height: 26px;
  object-fit: contain;
}

.asset-svg :deep(svg) {
  width: 24px;
  height: 24px;
  color: inherit;
}

.emoji-img {
  width: 36px;
  height: 36px;
  object-fit: contain;
}

.asset-key-label {
  width: 100%;
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  color: var(--color-text-tertiary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 0 4px;
}

.asset-cell:hover .asset-key-label {
  color: var(--color-foreground);
}

.asset-cell.is-copied .asset-key-label {
  color: #30d158;
  font-weight: 600;
}

/* 表情来源药丸栏 */
.emoji-sources-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.emoji-source-pill {
  padding: 5px 12px;
  font-size: 12px;
  border-radius: 9999px;
  border: 1px solid var(--color-border);
  background: var(--color-card);
  color: var(--color-text-secondary);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.emoji-source-pill:hover {
  border-color: var(--color-primary);
  color: var(--color-foreground);
}

.emoji-source-pill.active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
  font-weight: 550;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.25);
}

.source-count {
  font-size: 10px;
  opacity: 0.8;
}

.asset-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 16px;
  color: var(--color-text-quaternary);
  font-size: 13px;
}
</style>
