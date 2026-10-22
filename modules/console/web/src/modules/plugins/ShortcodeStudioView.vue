<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VEmpty, VLoading } from "@/components/ui";
import {
  PLUGIN_CATEGORIES,
  previewInfoFor,
  type PluginMeta,
  type PluginCategory,
  type PluginPreviewInfo,
} from "./plugin-types";
import { CAPABILITY_LABELS, FIXTURE_LABELS } from "./preview/resources";
import { usePluginPreview } from "./preview/usePluginPreview";
import PluginParamForm from "./PluginParamForm.vue";
import { catalogError, catalogStatus, loadPluginCatalog, retryLoadCatalog, runtimePlugins } from "./usePluginCatalog";

// 标签插件 Studio 工作台（从 PluginLibrary.vue 的「插件」Tab 拆分出来）：
// 分类速查 + 实时渲染预览 + 一键复制。预览复用后端 POST /api/v1/preview/markdown
// （component-library Fixture 模式）渲染管线，共享预览运行时（preview/）构建 iframe。

const emptyPreview: PluginPreviewInfo = { capability: "static", services: [], fixture: undefined, fallback: "" };

const search = ref("");
const activeCategory = ref<PluginCategory | "全部">("全部");
const selected = ref<PluginMeta | null>(null);
const example = ref("");
const formValid = ref(true);
const formError = ref("");

watch(runtimePlugins, (next) => {
  const refreshed = next.find((plugin) => selected.value && plugin.name === selected.value.name);
  if (refreshed) {
    selected.value = refreshed;
  } else if (next[0] && !selected.value) {
    selected.value = next[0];
    example.value = next[0].example;
  }
});

const filtered = computed(() => {
  const kw = search.value.trim().toLowerCase();
  return runtimePlugins.value.filter((p) => {
    if (activeCategory.value !== "全部" && p.category !== activeCategory.value) return false;
    if (!kw) return true;
    return p.name.toLowerCase().includes(kw) || p.title.toLowerCase().includes(kw) || p.desc.toLowerCase().includes(kw);
  });
});

const categoryCount = computed(() => {
  const map: Record<string, number> = { 全部: runtimePlugins.value.length };
  for (const c of PLUGIN_CATEGORIES) map[c] = 0;
  for (const p of runtimePlugins.value) map[p.category]++;
  return map;
});

// 列表徽标：hybrid 标签两种写法都支持，用「混」区分于纯块级/纯行内
function typeBadge(type: PluginMeta["type"]): string {
  if (type === "block") return "块";
  if (type === "hybrid") return "混";
  return "行";
}

function selectPlugin(p: PluginMeta) {
  selected.value = p;
  example.value = p.example;
  // 无 form 时 PluginParamForm 不挂载、不会发出 validation，这里复位；有 form 时由初始化的 validation 重新判定
  formValid.value = true;
  formError.value = "";
  preview.resetHeight(); // 选中标签先应用推荐高度，再按 iframe 实际内容高度回传调整
  preview.scheduleRender(example.value, true);
}

// 结构化参数表单产出 shortcode（Spec 29）：同步驱动示例/预览/复制
function onFormCode(code: string) {
  example.value = code;
}

// 必填缺失时示例保留上一次有效值，但复制入口需禁用并提示原因
function onFormValidation(state: { valid: boolean; error?: string }) {
  formValid.value = state.valid;
  formError.value = state.error || "";
}

// ---------- 实时预览（Spec 63：共享预览运行时） ----------
// 防抖 500ms + 请求序列号（旧请求不覆盖新预览）；请求携带 preview_profile=component-library
// 让后端进入本地 Fixture 模式（GitHub/友链确定性数据、rating/vote 隔离 ID 不写库）。

const previewFrame = ref<HTMLIFrameElement | null>(null);
const preview = usePluginPreview(() => (selected.value ? previewInfoFor(selected.value) : emptyPreview));

const selectedPreview = computed<PluginPreviewInfo>(() =>
  selected.value ? previewInfoFor(selected.value) : emptyPreview,
);
const capabilityLabel = computed(() =>
  selectedPreview.value.capability ? CAPABILITY_LABELS[selectedPreview.value.capability] : "",
);
const fixtureLabel = computed(() =>
  selectedPreview.value.fixture ? FIXTURE_LABELS[selectedPreview.value.fixture] : "",
);

watch(example, () => {
  if (example.value) preview.scheduleRender(example.value);
});

function onPreviewMessage(event: MessageEvent) {
  preview.onMessage(event, previewFrame.value);
}
onMounted(() => {
  window.addEventListener("message", onPreviewMessage);
  void loadPluginCatalog().then((next) => {
    if (next[0] && !selected.value) {
      selected.value = next[0];
      example.value = next[0].example;
      preview.resetHeight();
      preview.scheduleRender(example.value, true);
    }
  });
});
onBeforeUnmount(() => {
  window.removeEventListener("message", onPreviewMessage);
});

// ---------- 复制 ----------

const copied = ref<"syntax" | "example" | "name" | "">("");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

async function copyText(kind: "syntax" | "example" | "name", text: string) {
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
  copied.value = kind;
  clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => (copied.value = ""), 1500);
}

onBeforeUnmount(() => clearTimeout(copiedTimer));
</script>

<template>
  <div>
    <VPageHeader title="标签插件" subtitle="语法速查 + 实时渲染预览（本地 Fixture 数据，不依赖公网）" />

    <div class="mx-auto max-w-[1400px] px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="catalogStatus === 'loading'" />
      <VEmpty v-else-if="catalogStatus === 'error'" title="无法加载标签插件" :message="catalogError">
        <template #actions>
          <button type="button" class="retry-button" @click="retryLoadCatalog">
            <Icon icon="ri:refresh-line" />
            重试
          </button>
        </template>
      </VEmpty>
      <VEmpty
        v-else-if="catalogStatus === 'success' && runtimePlugins.length === 0"
        title="暂无标签插件"
        message="服务端 catalog 尚未提供可用的标签插件。"
      />
      <VCard v-else class="rounded-2xl">
        <div class="grid grid-cols-1 lg:grid-cols-[290px_minmax(0,1fr)]">
          <!-- 左侧：分类 + 搜索 + 列表 -->
          <aside
            class="studio-sidebar lg:sticky lg:top-5 lg:max-h-[calc(100vh-2.5rem)] lg:overflow-y-auto lg:border-r border-border"
          >
            <div class="studio-sidebar-header">
              <div class="studio-search-wrapper">
                <Icon icon="ri:search-line" class="studio-search-icon" />
                <input
                  v-model="search"
                  placeholder="搜索插件..."
                  class="studio-search-input"
                />
                <button
                  v-if="search"
                  type="button"
                  class="studio-search-clear"
                  title="清空搜索"
                  @click="search = ''"
                >
                  <Icon icon="ri:close-circle-fill" />
                </button>
              </div>
              <div class="studio-category-rail">
                <button
                  type="button"
                  @click="activeCategory = '全部'"
                  :class="['studio-cat-pill', { active: activeCategory === '全部' }]"
                >
                  全部 <span class="cat-count">{{ categoryCount["全部"] }}</span>
                </button>
                <button
                  v-for="c in PLUGIN_CATEGORIES"
                  :key="c"
                  type="button"
                  @click="activeCategory = c"
                  :class="['studio-cat-pill', { active: activeCategory === c }]"
                >
                  {{ c }} <span class="cat-count">{{ categoryCount[c] }}</span>
                </button>
              </div>
            </div>

            <div class="studio-sidebar-list">
              <button
                v-for="p in filtered"
                :key="p.name"
                type="button"
                @click="selectPlugin(p)"
                :class="['studio-list-item', { active: selected?.name === p.name }]"
              >
                <span
                  :class="[
                    'studio-type-badge',
                    p.type === 'block' ? 'badge-block' : p.type === 'hybrid' ? 'badge-hybrid' : 'badge-inline',
                  ]"
                >
                  {{ typeBadge(p.type) }}
                </span>
                <span class="studio-item-title">{{ p.title }}</span>
                <span class="studio-item-name">{{ p.name }}</span>
              </button>
              <div v-if="filtered.length === 0" class="studio-empty-list">
                <Icon icon="ri:search-eye-line" width="28" height="28" />
                <p>无匹配插件</p>
              </div>
            </div>
          </aside>

          <!-- 右侧：详情 + 预览 -->
          <main class="studio-workbench">
            <template v-if="selected">
              <!-- 1. 顶部 Header 概览卡片 -->
              <div class="studio-hero-card">
                <div class="studio-hero-main">
                  <div class="studio-hero-title-row">
                    <h3 class="studio-hero-title">{{ selected.title }}</h3>
                    <button
                      type="button"
                      class="studio-hero-shortcode-tag"
                      :title="`点击复制 {% ${selected.name} %}`"
                      @click="copyText('name', `{% ${selected.name} %}`)"
                    >
                      <code>{{ "{%" }} {{ selected.name }} {{ "%}" }}</code>
                      <Icon :icon="copied === 'name' ? 'ri:check-line' : 'ri:file-copy-line'" class="tag-copy-icon" />
                    </button>
                    <span class="studio-pill-badge badge-category">{{ selected.category }}</span>
                    <span
                      :class="[
                        'studio-pill-badge',
                        selected.type === 'block' ? 'badge-block' : selected.type === 'hybrid' ? 'badge-hybrid' : 'badge-inline',
                      ]"
                    >
                      {{ selected.type === "block" ? "块级标签" : selected.type === "hybrid" ? "块级 / 行内" : "行内标签" }}
                    </span>
                  </div>
                  <p class="studio-hero-desc">{{ selected.desc }}</p>
                </div>
              </div>

              <!-- 2. 语法与参数规格 -->
              <section class="studio-section">
                <div class="studio-section-heading">
                  <h2>语法与参数规格</h2>
                  <p>查看 Shortcode 签名格式与支持的属性参数。</p>
                </div>
                <div class="studio-panel">
                  <!-- 语法卡片 -->
                  <div class="studio-syntax-box">
                    <div class="studio-box-header">
                      <span class="box-header-title">Shortcode 签名</span>
                      <button
                        type="button"
                        class="studio-copy-action"
                        @click="copyText('syntax', selected.signature)"
                      >
                        <Icon :icon="copied === 'syntax' ? 'ri:check-line' : 'ri:file-copy-line'" />
                        <span>{{ copied === "syntax" ? "已复制" : "复制语法" }}</span>
                      </button>
                    </div>
                    <pre class="studio-syntax-code"><code>{{ selected.signature }}</code></pre>
                  </div>

                  <!-- 参数表 -->
                  <div v-if="selected.params.length" class="studio-params-table-wrapper">
                    <div class="studio-params-header">
                      <span>参数列表</span>
                      <span class="params-count">{{ selected.params.length }} 个参数</span>
                    </div>
                    <div class="studio-table-container">
                      <table class="studio-table">
                        <thead>
                          <tr>
                            <th class="th-name">参数</th>
                            <th class="th-desc">说明</th>
                            <th class="th-default">默认值</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="param in selected.params" :key="param.name">
                            <td class="td-name">
                              <code class="param-code">{{ param.name }}</code>
                            </td>
                            <td class="td-desc">{{ param.desc }}</td>
                            <td class="td-default">
                              <code class="default-code">{{ param.default || "—" }}</code>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                  </div>
                </div>
              </section>

              <!-- 3. 结构化参数表单（Spec 29，有 form schema 的插件） -->
              <section v-if="selected.form" class="studio-section">
                <div class="studio-section-heading">
                  <h2>可视化配置</h2>
                  <p>调整参数将自动同步拼装 Shortcode 示例与下方渲染效果。</p>
                </div>
                <div class="studio-panel studio-form-panel">
                  <PluginParamForm
                    :plugin="selected"
                    :model-value="example"
                    @update:model-value="onFormCode"
                    @validation="onFormValidation"
                  />
                  <p v-if="formError" style="color: var(--color-state-error, #ff453a)">{{ formError }}</p>
                </div>
              </section>

              <!-- 4. 示例编辑 + 复制 -->
              <section class="studio-section">
                <div class="studio-section-heading">
                  <h2>Shortcode 示例</h2>
                  <p>支持直接手动编辑示例代码，修改后下方预览将实时重新渲染。</p>
                </div>
                <div class="studio-panel studio-editor-panel">
                  <div class="studio-editor-header">
                    <span class="editor-header-label">
                      <Icon icon="ri:code-s-slash-line" />
                      <span>Shortcode 代码</span>
                    </span>
                    <button
                      type="button"
                      class="studio-copy-action"
                      :disabled="!formValid"
                      @click="copyText('example', example)"
                    >
                      <Icon :icon="copied === 'example' ? 'ri:check-line' : 'ri:file-copy-line'" />
                      <span>{{ copied === "example" ? "已复制" : "复制示例" }}</span>
                    </button>
                  </div>
                  <textarea
                    v-model="example"
                    spellcheck="false"
                    class="studio-code-editor"
                    placeholder="输入 Shortcode 代码..."
                  ></textarea>
                </div>
              </section>

              <!-- 5. 实时渲染画布 -->
              <section class="studio-section">
                <div class="studio-section-heading">
                  <h2>实时渲染预览</h2>
                  <p>基于真实前台组件渲染管线（包含样式表与按需注入的交互服务）。</p>
                </div>
                <div class="studio-canvas-panel">
                  <!-- macOS Window Header -->
                  <div class="studio-canvas-header">
                    <div class="canvas-window-dots" aria-hidden="true">
                      <span class="dot dot-red"></span>
                      <span class="dot dot-yellow"></span>
                      <span class="dot dot-green"></span>
                    </div>
                    <div class="canvas-header-meta">
                      <span class="canvas-cap-badge">
                        <Icon icon="ri:cpu-line" />
                        <span>依赖：{{ capabilityLabel }}</span>
                      </span>
                      <template v-if="selectedPreview.services?.length">
                        <span class="meta-dot">·</span>
                        <span class="canvas-services-tag">{{ selectedPreview.services.join(" / ") }}</span>
                      </template>
                      <span
                        v-if="fixtureLabel"
                        class="canvas-fixture-badge"
                        :title="fixtureLabel"
                      >
                        <Icon icon="ri:database-2-line" />
                        <span>预览 Fixture</span>
                      </span>
                    </div>
                    <div class="canvas-header-actions">
                      <button
                        type="button"
                        title="重新渲染"
                        class="canvas-refresh-btn"
                        @click="preview.scheduleRender(example, true)"
                      >
                        <Icon
                          icon="ri:refresh-line"
                          :class="{ 'animate-spin': preview.rendering.value }"
                        />
                        <span>{{ preview.rendering.value ? "渲染中..." : "重新渲染" }}</span>
                      </button>
                    </div>
                  </div>

                  <!-- Iframe Canvas Body -->
                  <div class="studio-canvas-body">
                    <div v-if="preview.previewError.value" class="canvas-state canvas-error">
                      <Icon icon="ri:error-warning-line" width="28" height="28" />
                      <p>{{ preview.previewError.value }}</p>
                    </div>
                    <div v-else-if="!preview.previewHtml.value" class="canvas-state canvas-empty">
                      <Icon icon="ri:inbox-line" width="28" height="28" />
                      <p>预览为空：该插件示例未产生可见输出，请检查参数是否齐全。</p>
                    </div>
                    <iframe
                      v-else
                      ref="previewFrame"
                      :srcdoc="preview.srcdoc.value"
                      class="studio-preview-iframe"
                      :style="{ height: preview.frameHeight.value + 'px' }"
                      sandbox="allow-scripts"
                    ></iframe>
                  </div>

                  <!-- Fallback hint footer -->
                  <div
                    v-if="selectedPreview.fallback && !preview.previewError.value"
                    class="studio-canvas-footer"
                  >
                    <Icon icon="ri:information-line" />
                    <span>{{ selectedPreview.fallback }}</span>
                  </div>
                </div>
              </section>
            </template>

            <VEmpty
              v-else
              title="从左侧选择一个标签插件"
              message="目录加载完成后可在此查看语法、参数与实时渲染预览。"
            />
          </main>
        </div>
      </VCard>
    </div>
  </div>
</template>

<style scoped>
/* ================= 标签插件 Studio 布局 ================= */
/* 左侧边栏 */
.studio-sidebar {
  display: flex;
  flex-direction: column;
  background: var(--color-card);
  border-right: 1px solid var(--color-border);
}

.studio-sidebar-header {
  padding: 14px;
  border-bottom: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.studio-search-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.studio-search-icon {
  position: absolute;
  left: 10px;
  color: var(--color-text-quaternary);
  font-size: 15px;
  pointer-events: none;
}

.studio-search-input {
  width: 100%;
  box-sizing: border-box;
  padding: 7px 28px 7px 32px;
  font-size: 13px;
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 50%, var(--color-card));
  color: var(--color-foreground);
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease;
}

.studio-search-input:focus {
  outline: none;
  border-color: var(--color-primary);
}

.studio-search-clear {
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
.studio-search-clear:hover {
  color: var(--color-foreground);
}

.studio-category-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.studio-cat-pill {
  padding: 3px 8px;
  font-size: 11.5px;
  border-radius: 9999px;
  border: 1px solid transparent;
  background: color-mix(in srgb, var(--color-foreground) 5%, transparent);
  color: var(--color-text-tertiary);
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.studio-cat-pill:hover {
  background: color-mix(in srgb, var(--color-foreground) 10%, transparent);
  color: var(--color-foreground);
}

.studio-cat-pill.active {
  background: var(--color-primary);
  color: #fff;
  font-weight: 550;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.25);
}

.studio-cat-pill .cat-count {
  font-size: 10px;
  opacity: 0.85;
}

.studio-sidebar-list {
  padding: 6px 0;
}

.studio-list-item {
  width: calc(100% - 12px);
  margin: 2px 6px;
  padding: 8px 10px;
  border-radius: var(--radius-md, 8px);
  border: 1px solid transparent;
  background: transparent;
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.studio-list-item:hover {
  background: color-mix(in srgb, var(--color-foreground) 4%, transparent);
}

.studio-list-item.active {
  background: color-mix(in srgb, var(--color-primary) 12%, var(--color-card));
  border-color: color-mix(in srgb, var(--color-primary) 30%, transparent);
}

.studio-type-badge {
  font-family: var(--font-mono, monospace);
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 4px;
  font-weight: 600;
  flex-shrink: 0;
}

.badge-block {
  background: rgba(255, 159, 10, 0.15);
  color: #ff9f0a;
}

.badge-inline {
  background: rgba(10, 132, 255, 0.15);
  color: var(--color-primary);
}

.badge-hybrid {
  background: rgba(191, 90, 242, 0.15);
  color: #bf5af2;
}

.badge-category {
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  color: var(--color-primary);
}

.studio-item-title {
  font-size: 13px;
  color: var(--color-foreground);
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.studio-list-item.active .studio-item-title {
  color: var(--color-primary);
  font-weight: 600;
}

.studio-item-name {
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  color: var(--color-text-quaternary);
  flex-shrink: 0;
}

.studio-empty-list {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 16px;
  color: var(--color-text-quaternary);
  font-size: 13px;
}

/* ================= 右侧工作台 ================= */
.studio-workbench {
  min-width: 0;
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  box-sizing: border-box;
}

.retry-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 34px;
  padding: 0 14px;
  border-radius: var(--radius-full);
  color: var(--color-foreground);
  background: var(--color-primary);
  font-size: 13px;
  font-weight: 600;
}

.retry-button:hover {
  opacity: 0.9;
}

/* 顶部 Hero 概览卡片 */
.studio-hero-card {
  background: linear-gradient(135deg, color-mix(in srgb, var(--color-background) 40%, var(--color-card)), var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  padding: 20px 24px;
  box-shadow: var(--shadow-sm);
}

.studio-hero-title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 8px;
}

.studio-hero-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--color-foreground);
  letter-spacing: -0.01em;
  display: flex;
  align-items: center;
  gap: 8px;
}

.studio-hero-title::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 18px;
  border-radius: 2px;
  background: var(--color-primary);
}

.studio-hero-shortcode-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: 6px;
  border: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 50%, var(--color-card));
  color: var(--color-text-secondary);
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.studio-hero-shortcode-tag:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 8%, var(--color-card));
}

.tag-copy-icon {
  font-size: 13px;
  opacity: 0.7;
}

.studio-pill-badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 9999px;
  font-weight: 500;
}

.studio-hero-desc {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 13.5px;
  line-height: 1.6;
}

/* 区域分块（Option 1 Inset Group Panel） */
.studio-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.studio-section-heading {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.studio-section-heading h2 {
  margin: 0;
  font-size: 15.5px;
  font-weight: 650;
  color: var(--color-foreground);
  display: flex;
  align-items: center;
  gap: 8px;
}

.studio-section-heading h2::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 15px;
  border-radius: 2px;
  background: var(--color-primary);
}

.studio-section-heading p {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-tertiary);
  padding-left: 11.5px;
}

.studio-panel {
  background: color-mix(in srgb, var(--color-background) 38%, var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  overflow: hidden;
}

/* 语法卡片 */
.studio-syntax-box {
  background: var(--color-card);
  border-bottom: 1px solid var(--color-border);
}

.studio-panel > .studio-syntax-box:last-child {
  border-bottom: none;
}

.studio-box-header {
  padding: 10px 16px;
  background: color-mix(in srgb, var(--color-background) 30%, var(--color-card));
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.box-header-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.studio-copy-action {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--color-primary);
  background: transparent;
  border: 1px solid transparent;
  padding: 3px 8px;
  border-radius: 6px;
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.studio-copy-action:hover {
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border-color: color-mix(in srgb, var(--color-primary) 25%, transparent);
}

.studio-syntax-code {
  margin: 0;
  padding: 14px 16px;
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.5;
  overflow-x: auto;
}

/* 参数表 */
.studio-params-table-wrapper {
  background: var(--color-card);
}

.studio-params-header {
  padding: 10px 16px;
  background: color-mix(in srgb, var(--color-background) 30%, var(--color-card));
  border-bottom: 1px solid var(--color-border);
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.params-count {
  font-size: 11px;
  color: var(--color-text-quaternary);
  font-weight: normal;
}

.studio-table-container {
  overflow-x: auto;
}

.studio-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  text-align: left;
}

.studio-table th {
  padding: 10px 16px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--color-text-tertiary);
  border-bottom: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 15%, var(--color-card));
}

.studio-table td {
  padding: 11px 16px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
  vertical-align: top;
}

.studio-table tr:last-child td {
  border-bottom: none;
}

.param-code {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--color-foreground) 6%, transparent);
  color: var(--color-foreground);
  font-weight: 550;
}

.default-code {
  font-family: var(--font-mono, monospace);
  font-size: 11.5px;
  color: var(--color-text-tertiary);
}

.td-desc {
  color: var(--color-text-secondary);
  line-height: 1.5;
}

/* 参数表单面板 */
.studio-form-panel {
  padding: 20px 22px;
}

/* 示例编辑器面板 */
.studio-editor-panel {
  background: var(--color-card);
}

.studio-editor-header {
  padding: 10px 16px;
  background: color-mix(in srgb, var(--color-background) 30%, var(--color-card));
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.editor-header-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.studio-code-editor {
  width: 100%;
  box-sizing: border-box;
  padding: 14px 16px;
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  line-height: 1.6;
  background: transparent;
  color: var(--color-foreground);
  border: none;
  resize: vertical;
  min-height: 110px;
}

.studio-code-editor:focus {
  outline: none;
  background: color-mix(in srgb, var(--color-background) 20%, var(--color-card));
}

/* 画布预览面板（macOS Studio Style） */
.studio-canvas-panel {
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  overflow: hidden;
}

.studio-canvas-header {
  padding: 10px 16px;
  background: color-mix(in srgb, var(--color-background) 35%, var(--color-card));
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.canvas-window-dots {
  display: flex;
  align-items: center;
  gap: 6px;
}

.canvas-window-dots .dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.dot-red { background: #ff453a; }
.dot-yellow { background: #ff9f0a; }
.dot-green { background: #30d158; }

.canvas-header-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.canvas-cap-badge {
  font-size: 11px;
  color: var(--color-text-tertiary);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.meta-dot {
  color: var(--color-text-quaternary);
}

.canvas-services-tag {
  font-family: var(--font-mono, monospace);
  font-size: 10.5px;
  color: var(--color-text-tertiary);
  padding: 1px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--color-foreground) 6%, transparent);
}

.canvas-fixture-badge {
  font-size: 10.5px;
  padding: 2px 7px;
  border-radius: 9999px;
  background: rgba(48, 209, 88, 0.15);
  color: #30d158;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.canvas-refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 9999px;
  border: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-foreground) 5%, transparent);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.canvas-refresh-btn:hover {
  background: color-mix(in srgb, var(--color-foreground) 10%, transparent);
  color: var(--color-foreground);
  border-color: var(--color-primary);
}

.studio-canvas-body {
  position: relative;
  background: #101012;
  min-height: 120px;
}

.studio-preview-iframe {
  display: block;
  width: 100%;
  border: none;
  transition: height 0.2s ease;
}

.canvas-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 16px;
  font-size: 13px;
}

.canvas-error {
  color: var(--color-state-error);
}

.canvas-empty {
  color: var(--color-text-quaternary);
}

.studio-canvas-footer {
  padding: 8px 16px;
  border-top: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 25%, var(--color-card));
  font-size: 11px;
  color: var(--color-text-quaternary);
  display: flex;
  align-items: center;
  gap: 6px;
}

@media (max-width: 768px) {
  .studio-workbench {
    padding: 16px;
    gap: 16px;
  }
}
</style>
