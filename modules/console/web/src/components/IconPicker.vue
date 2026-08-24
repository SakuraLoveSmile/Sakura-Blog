<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { Icon } from "@iconify/vue";
import { assetsApi, type AssetIconItem } from "@/api";

// 图标选择器：数据源硬限定 icons.yml（GET /api/v1/assets/icons，key + 内联 SVG 或 URL）。
// - 当前值在清单 → 内联预览（v-html 只渲染 API 返回的 SVG）；值是 URL → <img>；
//   其他非空值 → 警告「不在图标库中，前台将显示占位」（对齐前台 icon helper 的 icon-missing 行为）。
// - 手填输入保留为兜底（emit 前 trim，与 matchedItem/warnValue 的 trim 匹配保持一致）；
//   「选择」按钮打开遮罩弹窗，搜索（35 项客户端过滤）+ 按前缀分组网格。
// - 弹窗用 Teleport 挂到 body：避免 TaxonomyList 分类卡片等 transform 祖先
//   （hover:-translate-y-px）约束 fixed 定位导致遮罩只覆盖卡片、点外部无法关闭。
// - 数据模块级缓存：多实例（菜单/右栏/页脚等多行）共享一次请求。
const props = defineProps<{
  modelValue: string;
  placeholder?: string;
  compact?: boolean;
  hint?: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: string): void;
}>();

// ---------- 图标数据（模块级缓存） ----------
let cachedIcons: AssetIconItem[] | null = null;
let iconsPromise: Promise<AssetIconItem[]> | null = null;

async function fetchIcons(): Promise<AssetIconItem[]> {
  if (cachedIcons) return cachedIcons;
  if (!iconsPromise) {
    iconsPromise = (async () => {
      const { data } = await assetsApi.icons();
      cachedIcons = data.items || [];
      return cachedIcons;
    })();
    // 失败时清空 promise，允许下次重试
    try {
      await iconsPromise;
    } catch (e) {
      iconsPromise = null;
      throw e;
    }
  }
  return iconsPromise;
}

const icons = ref<AssetIconItem[]>([]);
const loading = ref(false);
const open = ref(false);
// 清单加载状态：成功/失败分别标记，失败允许重试（弹窗内提供重试入口）
const iconsLoaded = ref(false);
const iconsFailed = ref(false);

async function ensureLoaded() {
  if (iconsLoaded.value) return;
  loading.value = true;
  try {
    icons.value = await fetchIcons();
    iconsLoaded.value = true;
    iconsFailed.value = false;
  } catch {
    // 加载失败：保持未加载状态，不触发误报警告；弹窗内可重试
    iconsFailed.value = true;
  } finally {
    loading.value = false;
  }
}

function openPicker() {
  open.value = true;
  ensureLoaded();
}

function selectIcon(key: string) {
  emit("update:modelValue", key);
  open.value = false;
}

// ---------- Escape 关闭弹窗 ----------
function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && open.value) open.value = false;
}

watch(open, (v) => {
  if (v) window.addEventListener("keydown", onKeydown);
  else window.removeEventListener("keydown", onKeydown);
});

onMounted(() => {
  // 挂载即预加载清单：模块级 cachedIcons 保证多实例仅发一次请求，
  // 避免首屏渲染时清单未加载而对有效 key 误报「不在图标库」警告
  ensureLoaded();
});

onUnmounted(() => {
  window.removeEventListener("keydown", onKeydown);
});

// ---------- 当前值状态 ----------
// 值是否为 URL（对齐前台 icon helper：以 /、http://、https:// 开头渲染 <img>）
function isUrl(v: string): boolean {
  return v.startsWith("http://") || v.startsWith("https://") || v.startsWith("/");
}

const matchedItem = computed(() => icons.value.find((i) => i.key === (props.modelValue || "").trim()) || null);

// 非空、非 URL、清单加载完成且不在清单 → 前台将渲染 icon-missing 占位
// 注意：加载中/加载失败（iconsLoaded=false）时不显示警告，避免误报
const warnValue = computed(() => {
  const v = (props.modelValue || "").trim();
  return !!v && iconsLoaded.value && !isUrl(v) && !matchedItem.value;
});

// 预览：清单命中且 value 为 URL → img；命中且为内联 SVG → v-html；未命中但值是 URL → img
const previewUrl = computed(() => {
  if (matchedItem.value && isUrl(matchedItem.value.value)) return matchedItem.value.value;
  if (!matchedItem.value && isUrl(props.modelValue || "")) return props.modelValue;
  return "";
});

const previewSvg = computed(() => {
  if (matchedItem.value && !isUrl(matchedItem.value.value)) return matchedItem.value.value;
  return "";
});

// ---------- 弹窗内搜索与分组 ----------
const filter = ref("");

const groups = computed(() => {
  const kw = filter.value.trim().toLowerCase();
  const list = kw ? icons.value.filter((i) => i.key.toLowerCase().includes(kw)) : icons.value;
  const byPrefix = new Map<string, AssetIconItem[]>();
  for (const item of list) {
    const prefix = item.key.split(":")[0] || "other";
    if (!byPrefix.has(prefix)) byPrefix.set(prefix, []);
    byPrefix.get(prefix)!.push(item);
  }
  const out: { prefix: string; items: AssetIconItem[] }[] = [];
  for (const [prefix, items] of byPrefix) out.push({ prefix, items });
  return out;
});
</script>

<template>
  <div class="ip-field" :class="{ 'ip-compact': compact }">
    <div class="ip-row">
      <span v-if="previewSvg" class="ip-preview" v-html="previewSvg"></span>
      <img
        v-else-if="previewUrl"
        :src="previewUrl"
        class="ip-preview-img"
        alt=""
        @error="($event.target as HTMLImageElement).style.display = 'none'"
      />
      <span v-else-if="warnValue" class="ip-warn-badge" title="该值不在图标库中，前台将显示占位">
        <Icon icon="ri:alert-line" />
      </span>
      <input
        :value="modelValue"
        class="apple-input ip-input"
        :placeholder="placeholder"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value.trim())"
      />
      <button type="button" class="ip-btn" title="选择图标" @click="openPicker">
        <Icon icon="ri:shape-line" />
      </button>
    </div>
    <p v-if="warnValue" class="ip-warn-text">不在图标库中，前台将显示占位</p>
    <p v-if="hint" class="ip-hint">{{ hint }}</p>

    <!-- 选择弹窗：Teleport 到 body，避免 TaxonomyList 分类卡片等 transform 祖先
         （hover:-translate-y-px）约束 fixed 定位导致遮罩只覆盖卡片、点外部无法关闭；
         模式对齐 components/ui/VModal.vue；scoped 样式随 data-v 属性照常生效 -->
    <Teleport to="body">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center bg-background/40 p-4"
        @click.self="open = false"
      >
        <div class="ip-modal w-full max-w-2xl max-h-[80vh] flex flex-col bg-card rounded-xl shadow-xl">
          <div class="flex items-center justify-between gap-3 px-5 py-3 border-b border-border">
            <h3 class="text-sm font-medium text-foreground">选择图标（icons.yml）</h3>
            <div class="apple-search" style="width: 240px">
              <Icon icon="ri:search-line" class="text-text-quaternary" />
              <input v-model="filter" placeholder="搜索图标 key..." />
            </div>
            <button type="button" @click="open = false" class="text-text-quaternary hover:text-foreground">
              <Icon icon="ri:close-line" class="text-xl" />
            </button>
          </div>

          <div class="ip-modal-body flex-1 overflow-y-auto p-5">
            <p v-if="loading" class="text-sm text-text-tertiary">加载中...</p>
            <p v-else-if="iconsFailed" class="flex items-center gap-2 text-sm text-text-quaternary">
              图标清单加载失败
              <button type="button" class="ip-retry" @click="ensureLoaded">重试</button>
            </p>
            <template v-else>
              <div v-for="g in groups" :key="g.prefix" class="mb-5">
                <h4 class="ip-group-title">
                  {{ g.prefix }}:*
                  <span class="ip-group-count">{{ g.items.length }} 个</span>
                </h4>
                <div class="grid grid-cols-4 gap-2 sm:grid-cols-6 md:grid-cols-8">
                  <button
                    v-for="item in g.items"
                    :key="item.key"
                    type="button"
                    class="ip-cell"
                    :class="{ active: item.key === (modelValue || '').trim() }"
                    :title="item.key"
                    @click="selectIcon(item.key)"
                  >
                    <span class="ip-cell-svg">
                      <img v-if="isUrl(item.value)" :src="item.value" class="h-6 w-6 object-contain" alt="" />
                      <span v-else v-html="item.value"></span>
                    </span>
                    <span class="ip-cell-key">{{ item.key.split(":")[1] || item.key }}</span>
                  </button>
                </div>
              </div>
              <p v-if="groups.length === 0" class="text-sm text-text-quaternary">没有匹配的图标。</p>
            </template>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
/* 组件自包含：不依赖外部 scoped 类；颜色/圆角/动效只消费语义 token */
.ip-field {
  min-width: 0;
}
.ip-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.ip-input {
  flex: 1;
  min-width: 0;
}
.ip-preview {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--color-text-secondary);
}
.ip-preview-img {
  width: 22px;
  height: 22px;
  object-fit: contain;
  flex-shrink: 0;
}
.ip-warn-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--color-state-warning);
  font-size: 16px;
}
.ip-warn-text {
  margin: 4px 0 0;
  font-size: 11px;
  color: var(--color-state-warning);
}
.ip-hint {
  margin: 4px 0 0;
  font-size: 11px;
  color: var(--color-text-quaternary);
}
.ip-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
  border: none;
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-standard);
}
.ip-btn:hover {
  background: color-mix(in srgb, var(--color-primary) 20%, transparent);
}

/* compact：行内紧凑场景（表格行/卡片网格），按钮与预览缩小 */
.ip-compact .ip-btn {
  width: 30px;
  height: 30px;
}
.ip-compact .ip-input {
  font-size: 12px;
}
.ip-compact .ip-preview-img,
.ip-compact .ip-preview :deep(svg) {
  width: 18px;
  height: 18px;
}

/* icons.yml 内联 SVG 自带 32px，统一收敛到 24px 并继承前景色 */
.ip-preview :deep(svg) {
  width: 22px;
  height: 22px;
  color: inherit;
}

/* 弹窗内网格 */
.ip-group-title {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  letter-spacing: -0.1px;
}
.ip-group-count {
  margin-left: 4px;
  font-weight: 400;
  color: var(--color-text-quaternary);
}
.ip-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 10px 4px 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  background: var(--color-card);
  color: var(--color-foreground);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-standard);
}
.ip-cell:hover {
  border-color: var(--color-primary);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md, 0 4px 8px -2px rgba(0, 0, 0, 0.06));
}
.ip-cell.active {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
}
.ip-cell-svg {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 24px;
  color: inherit;
}
.ip-cell-svg :deep(svg) {
  width: 24px;
  height: 24px;
  color: inherit;
}
.ip-cell-key {
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 10px;
  color: var(--color-text-tertiary);
}
.ip-modal-body {
  min-height: 0;
}
.ip-retry {
  padding: 2px 10px;
  font-size: 12px;
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border: none;
  border-radius: var(--radius-full);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-standard);
}
.ip-retry:hover {
  background: color-mix(in srgb, var(--color-primary) 20%, transparent);
}
</style>
