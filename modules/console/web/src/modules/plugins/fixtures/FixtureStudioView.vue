<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";
import { useFixtureStore } from "./useFixtureStore";
import type { FixtureItem, FixtureState } from "./fixture-types";
import { validateFixtureData } from "./fixture-validation";
import { usePluginPreview } from "../preview/usePluginPreview";
import type { PluginPreviewInfo } from "../plugin-types";

const store = useFixtureStore();

const search = ref("");
const localEditorData = ref("");
const validationErrors = ref<string[]>([]);
const copySuccessKey = ref("");
const fileInputRef = ref<HTMLInputElement | null>(null);

const filteredFixtures = computed(() => {
  const q = search.value.trim().toLowerCase();
  if (!q) return store.fixtures;
  return store.fixtures.filter(
    (f) =>
      f.title.toLowerCase().includes(q) ||
      f.endpoint.toLowerCase().includes(q) ||
      f.description.toLowerCase().includes(q) ||
      f.category.toLowerCase().includes(q),
  );
});

const currentFixture = computed<FixtureItem>(() => store.currentFixture.value);
const currentState = computed<FixtureState>(() => store.currentState.value);

// 映射当前选中 Fixture 为 Preview 运行时所需的能力描述
const currentPreviewInfo = computed<PluginPreviewInfo>(() => {
  const f = currentFixture.value;
  switch (f.id) {
    case "friends":
      return { capability: "service", services: ["friends"], height: f.previewHeight };
    case "sites":
      return { capability: "service", services: ["sites"], height: f.previewHeight };
    case "rating":
      return { capability: "service", services: ["rating"], height: f.previewHeight };
    case "vote":
      return { capability: "service", services: ["vote"], height: f.previewHeight };
    case "md":
      return { capability: "external", plugins: ["md"], height: f.previewHeight };
    default:
      return { capability: "static", height: f.previewHeight };
  }
});

const previewFrame = ref<HTMLIFrameElement | null>(null);
const preview = usePluginPreview(
  () => currentPreviewInfo.value,
  () => store.overrideMap.value,
);

// 切换 Fixture 时，同步编辑器内容并调度预览
watch(
  () => store.selectedId.value,
  () => {
    localEditorData.value = store.currentData.value;
    validateEditorData();
    preview.resetHeight();
    preview.scheduleRender(currentFixture.value.previewShortcode, true);
  },
  { immediate: true },
);

function onSelect(item: FixtureItem) {
  store.selectFixture(item.id);
}

function validateEditorData(): boolean {
  const result = validateFixtureData(currentFixture.value, localEditorData.value);
  validationErrors.value = result.errors;
  return result.valid;
}

function onEditorInput() {
  if (!validateEditorData()) return;
  store.updateData(currentFixture.value.id, localEditorData.value);
  preview.scheduleRender(currentFixture.value.previewShortcode, false);
}

function onChangeState(state: FixtureState) {
  store.updateState(currentFixture.value.id, state);
  preview.scheduleRender(currentFixture.value.previewShortcode, true);
}

function onPrettify() {
  if (currentFixture.value.format !== "json") return;
  try {
    const parsed = JSON.parse(localEditorData.value);
    localEditorData.value = JSON.stringify(parsed, null, 2);
    if (!validateEditorData()) return;
    store.updateData(currentFixture.value.id, localEditorData.value);
    preview.scheduleRender(currentFixture.value.previewShortcode, true);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : "无法解析 JSON";
    validationErrors.value = [`JSON syntax error: ${message}`];
  }
}

function onResetCurrent() {
  store.resetFixture(currentFixture.value.id);
  localEditorData.value = currentFixture.value.defaultData;
  validateEditorData();
  preview.scheduleRender(currentFixture.value.previewShortcode, true);
}

function onResetAll() {
  if (confirm("确定要将所有 Mock Fixtures 恢复为系统默认预设吗？")) {
    store.resetAll();
    localEditorData.value = currentFixture.value.defaultData;
    validateEditorData();
    preview.scheduleRender(currentFixture.value.previewShortcode, true);
  }
}

async function copyText(text: string, key: string) {
  try {
    await navigator.clipboard.writeText(text);
    copySuccessKey.value = key;
    setTimeout(() => {
      if (copySuccessKey.value === key) copySuccessKey.value = "";
    }, 1800);
  } catch (err) {
    console.error("Failed to copy", err);
  }
}

function triggerImport() {
  fileInputRef.value?.click();
}

function handleFileImport(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) return;
  const reader = new FileReader();
  reader.onload = (e) => {
    const content = e.target?.result as string;
    if (content) {
      const res = store.importJson(content);
      if (res.success) {
        localEditorData.value = store.currentData.value;
        validateEditorData();
        preview.scheduleRender(currentFixture.value.previewShortcode, true);
        alert("Mock Fixture 配置导入成功！");
      } else {
        alert(`导入失败：${res.error}`);
      }
    }
  };
  reader.readAsText(file);
  target.value = "";
}

function exportConfig() {
  const json = store.exportJson();
  const blob = new Blob([json], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `blog-mock-fixtures-${new Date().toISOString().slice(0, 10)}.json`;
  a.click();
  URL.revokeObjectURL(url);
}

function onFrameMessage(e: MessageEvent) {
  preview.onMessage(e, previewFrame.value);
}

onMounted(() => {
  window.addEventListener("message", onFrameMessage);
});

onBeforeUnmount(() => {
  window.removeEventListener("message", onFrameMessage);
});
</script>

<template>
  <div class="fixture-studio-layout">
    <!-- 左侧边栏：搜索 + Fixture 列表 -->
    <aside class="studio-sidebar">
      <div class="studio-sidebar-header">
        <div class="studio-search-wrapper">
          <Icon icon="ri:search-line" class="studio-search-icon" />
          <input
            v-model="search"
            placeholder="搜索 Mock 数据或端点..."
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

        <div class="studio-sidebar-summary">
          <span>共 {{ store.fixtures.length }} 个确定性数据源</span>
          <span v-if="store.customCount.value > 0" class="studio-summary-badge">
            已修改 {{ store.customCount.value }}
          </span>
        </div>
      </div>

      <div class="studio-sidebar-list">
        <button
          v-for="item in filteredFixtures"
          :key="item.id"
          type="button"
          :class="['studio-list-item', { active: store.selectedId.value === item.id }]"
          @click="onSelect(item)"
        >
          <span class="studio-type-badge badge-inline">{{ item.format.toUpperCase() }}</span>
          <div class="studio-item-content">
            <div class="studio-item-title-row">
              <span class="studio-item-title">{{ item.title }}</span>
              <span v-if="store.isModified(item.id)" class="studio-dirty-dot" title="已修改" />
            </div>
            <span class="studio-item-name">{{ item.endpoint }}</span>
          </div>
        </button>

        <div v-if="filteredFixtures.length === 0" class="studio-empty-list">
          <Icon icon="ri:search-eye-line" width="28" height="28" />
          <p>无匹配 Mock 数据源</p>
        </div>
      </div>
    </aside>

    <!-- 右侧工作台：详情 + 状态模拟 + 编辑器 + 实时预览 -->
    <main class="studio-workbench">
      <!-- 1. 顶部 Header 概览卡片 -->
      <div class="studio-hero-card">
        <div class="studio-hero-main">
          <div class="studio-hero-title-row">
            <h3 class="studio-hero-title">{{ currentFixture.title }}</h3>
            <span class="studio-pill-badge badge-category">{{ currentFixture.category }}</span>
            <button
              type="button"
              class="studio-hero-shortcode-tag"
              title="点击复制端点路径"
              @click="copyText(currentFixture.endpoint, 'endpoint')"
            >
              <code>GET {{ currentFixture.endpoint }}</code>
              <Icon :icon="copySuccessKey === 'endpoint' ? 'ri:check-line' : 'ri:file-copy-line'" class="tag-copy-icon" />
            </button>
            <div class="studio-hero-actions">
              <input ref="fileInputRef" type="file" accept=".json" class="hidden-file-input" @change="handleFileImport" />
              <button type="button" class="studio-header-btn" title="导入配置 JSON" @click="triggerImport">
                <Icon icon="ri:upload-2-line" />
                <span>导入</span>
              </button>
              <button type="button" class="studio-header-btn" title="导出当前配置" @click="exportConfig">
                <Icon icon="ri:download-2-line" />
                <span>导出</span>
              </button>
              <button
                v-if="store.customCount.value > 0"
                type="button"
                class="studio-header-btn studio-header-btn--reset"
                title="全部恢复默认"
                @click="onResetAll"
              >
                <Icon icon="ri:refresh-line" />
                <span>全部重置</span>
              </button>
            </div>
          </div>
          <p class="studio-hero-desc">{{ currentFixture.description }}</p>
          <div class="studio-hero-shortcodes">
            <span class="studio-shortcodes-label">消费短代码:</span>
            <span v-for="sc in currentFixture.sampleShortcodes" :key="sc" class="studio-shortcode-pill">
              {{ sc }}
            </span>
          </div>
        </div>
      </div>

      <!-- 2. 模拟响应状态 -->
      <section class="studio-section">
        <div class="studio-section-heading">
          <h2>响应状态模拟</h2>
          <p>切换正常返回、空数据或 500 异常，实时检验前端数据交互与降级提示。</p>
        </div>
        <div class="studio-panel studio-state-panel">
          <div class="studio-state-inner">
            <div class="studio-segmented">
              <button
                type="button"
                :class="['studio-segmented-btn', { active: currentState === 'normal' }]"
                @click="onChangeState('normal')"
              >
                <span class="state-dot state-dot--normal" />
                <span>正常响应 (Normal)</span>
              </button>
              <button
                type="button"
                :class="['studio-segmented-btn', { active: currentState === 'empty' }]"
                @click="onChangeState('empty')"
              >
                <span class="state-dot state-dot--empty" />
                <span>空数据 (Empty)</span>
              </button>
              <button
                type="button"
                :class="['studio-segmented-btn', { active: currentState === 'error' }]"
                @click="onChangeState('error')"
              >
                <span class="state-dot state-dot--error" />
                <span>500 异常 (Error)</span>
              </button>
            </div>
            <div class="studio-state-hint">
              <span v-if="currentState === 'normal'">🟢 当前模式：正常返回 Mock 数据，供前台脚本渲染真实结构</span>
              <span v-else-if="currentState === 'empty'">🟡 当前模式：模拟返回空数据（如空列表或 0 评分），触发空态提示</span>
              <span v-else>🔴 当前模式：模拟接口返回 500 异常，检验前台优雅降级机制</span>
            </div>
          </div>
        </div>
      </section>

      <!-- 3. 数据内容编辑 -->
      <section class="studio-section">
        <div class="studio-section-heading">
          <h2>数据内容编辑</h2>
          <p>实时编辑该 Fixture 的 JSON / Markdown 数据，修改即时同步至下方渲染画布。</p>
          <p v-if="currentFixture.schemaHint" class="studio-schema-hint">
            数据结构：<code>{{ currentFixture.schemaHint }}</code>
          </p>
        </div>
        <div class="studio-panel">
          <div class="studio-box-header">
            <div class="studio-window-dots">
              <span class="dot dot-red" />
              <span class="dot dot-yellow" />
              <span class="dot dot-green" />
              <span class="studio-window-filename">
                {{ currentFixture.id }}.fixture.{{ currentFixture.format === 'json' ? 'json' : 'md' }}
              </span>
              <span v-if="store.isModified(currentFixture.id)" class="studio-dirty-dot" title="已修改" />
            </div>
            <div class="studio-editor-actions">
              <button
                v-if="currentFixture.format === 'json'"
                type="button"
                class="studio-copy-action"
                title="格式化 JSON"
                @click="onPrettify"
              >
                <Icon icon="ri:sparkling-line" />
                <span>格式化</span>
              </button>
              <button
                type="button"
                class="studio-copy-action"
                title="复制内容"
                @click="copyText(localEditorData, 'editor')"
              >
                <Icon :icon="copySuccessKey === 'editor' ? 'ri:check-line' : 'ri:file-copy-line'" />
                <span>{{ copySuccessKey === 'editor' ? '已复制' : '复制' }}</span>
              </button>
              <button
                v-if="store.isModified(currentFixture.id)"
                type="button"
                class="studio-copy-action studio-copy-action--reset"
                title="恢复此项为系统预设"
                @click="onResetCurrent"
              >
                <Icon icon="ri:refresh-line" />
                <span>恢复默认</span>
              </button>
            </div>
          </div>

          <div class="studio-editor-body">
            <textarea
              v-model="localEditorData"
              class="studio-syntax-code studio-code-textarea"
              spellcheck="false"
              placeholder="在此输入自定义 Mock 数据..."
              @input="onEditorInput"
            />
          </div>

          <div class="studio-editor-footer">
            <div v-if="validationErrors.length > 0" class="studio-syntax-status is-error">
              <Icon icon="ri:error-warning-line" />
              <div>
                <div v-for="error in validationErrors" :key="error">✕ {{ error }}</div>
                <div>当前修改尚未应用到预览</div>
              </div>
            </div>
            <div v-else class="studio-syntax-status is-valid">
              <Icon icon="ri:checkbox-circle-line" />
              <div>
                <div>{{ currentFixture.format === 'json' ? '✓ JSON 格式正确' : '✓ Markdown 内容有效' }}</div>
                <div v-if="currentFixture.format === 'json'">✓ Fixture 数据结构正确</div>
                <div>✓ 修改已自动保存到当前浏览器</div>
              </div>
            </div>
            <div class="studio-editor-length">{{ localEditorData.length }} 字符</div>
          </div>
        </div>
      </section>

      <!-- 4. 实时效果预览 -->
      <section class="studio-section">
        <div class="studio-section-heading">
          <h2>实时效果预览</h2>
          <p>消费当前 Mock 数据的 Shortcode 真实渲染画布，支持交互测试。</p>
        </div>
        <div class="studio-panel studio-preview-panel">
          <div class="studio-box-header">
            <div class="studio-window-dots">
              <span class="dot dot-red" />
              <span class="dot dot-yellow" />
              <span class="dot dot-green" />
              <span class="studio-window-filename">
                <code>{{ currentFixture.previewShortcode }}</code>
              </span>
            </div>
            <div class="studio-editor-actions">
              <button
                type="button"
                class="studio-copy-action"
                title="刷新预览"
                :disabled="preview.rendering.value"
                @click="preview.scheduleRender(currentFixture.previewShortcode, true)"
              >
                <Icon icon="ri:refresh-line" :class="{ 'animate-spin': preview.rendering.value }" />
                <span>{{ preview.rendering.value ? '渲染中...' : '重新渲染' }}</span>
              </button>
              <button
                type="button"
                class="studio-copy-action"
                title="复制短代码"
                @click="copyText(currentFixture.previewShortcode, 'shortcode')"
              >
                <Icon :icon="copySuccessKey === 'shortcode' ? 'ri:check-line' : 'ri:file-copy-line'" />
                <span>{{ copySuccessKey === 'shortcode' ? '已复制' : '复制语法' }}</span>
              </button>
            </div>
          </div>

          <div class="studio-preview-canvas">
            <div v-if="preview.previewError.value" class="canvas-state canvas-error">
              <Icon icon="ri:error-warning-line" width="28" height="28" />
              <p>{{ preview.previewError.value }}</p>
            </div>
            <iframe
              ref="previewFrame"
              class="studio-preview-iframe"
              :srcdoc="preview.srcdoc.value"
              sandbox="allow-same-origin allow-scripts"
              :style="{ height: preview.frameHeight.value + 'px' }"
              title="Fixture Live Preview"
            />
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<style scoped>
.fixture-studio-layout {
  display: flex;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.hidden-file-input {
  display: none;
}

/* 左侧边栏 */
.studio-sidebar {
  width: 300px;
  flex-shrink: 0;
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
  transition: all 0.2s ease;
}

.studio-search-input:focus {
  outline: none;
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-primary) 20%, transparent);
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

.studio-sidebar-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11.5px;
  color: var(--color-text-tertiary);
  padding: 0 2px;
}

.studio-summary-badge {
  font-size: 10.5px;
  padding: 1px 6px;
  border-radius: 9999px;
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  color: var(--color-primary);
  font-weight: 500;
}

.studio-sidebar-list {
  flex: 1;
  overflow-y: auto;
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
  gap: 10px;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
  box-sizing: border-box;
}

.studio-list-item:hover {
  background: color-mix(in srgb, var(--color-foreground) 4%, transparent);
}

.studio-list-item.active {
  background: color-mix(in srgb, var(--color-primary) 12%, var(--color-card));
  border-color: color-mix(in srgb, var(--color-primary) 30%, transparent);
}

.studio-item-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.studio-item-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.studio-item-title {
  font-size: 13px;
  color: var(--color-foreground);
  font-weight: 500;
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
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.studio-type-badge {
  font-family: var(--font-mono, monospace);
  font-size: 10px;
  padding: 2px 5px;
  border-radius: 4px;
  font-weight: 600;
  flex-shrink: 0;
}

.badge-inline {
  background: rgba(10, 132, 255, 0.15);
  color: var(--color-primary);
}

.badge-category {
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  color: var(--color-primary);
}

.studio-dirty-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #30d158;
  box-shadow: 0 0 6px rgba(48, 209, 88, 0.6);
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

/* 右侧工作台 */
.studio-workbench {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  box-sizing: border-box;
}

/* 顶部 Hero 概览卡片 */
.studio-hero-card {
  background: linear-gradient(135deg, color-mix(in srgb, var(--color-background) 40%, var(--color-card)), var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl, 14px);
  padding: 20px 24px;
  box-shadow: var(--shadow-sm);
}

.studio-hero-main {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.studio-hero-title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
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
  transition: all 0.15s ease;
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

.studio-hero-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}

.studio-header-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 10px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--color-background) 40%, var(--color-card));
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.studio-header-btn:hover {
  background: color-mix(in srgb, var(--color-foreground) 8%, var(--color-card));
  color: var(--color-foreground);
  border-color: var(--color-border);
}

.studio-header-btn--reset {
  color: #ff453a;
  border-color: rgba(255, 69, 58, 0.25);
  background: rgba(255, 69, 58, 0.08);
}

.studio-header-btn--reset:hover {
  background: rgba(255, 69, 58, 0.16);
  color: #ff453a;
}

.studio-hero-desc {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 13.5px;
  line-height: 1.6;
}

.studio-hero-shortcodes {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  margin-top: 4px;
}

.studio-shortcodes-label {
  color: var(--color-text-tertiary);
  font-weight: 500;
}

.studio-shortcode-pill {
  font-family: var(--font-mono, monospace);
  font-size: 11.5px;
  padding: 2px 8px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--color-background) 40%, var(--color-card));
  border: 1px solid var(--color-border);
  color: var(--color-primary);
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

.studio-schema-hint code {
  color: var(--color-primary);
  font-family: var(--font-mono, monospace);
}

.studio-panel {
  background: color-mix(in srgb, var(--color-background) 38%, var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl, 14px);
  overflow: hidden;
}

/* 状态模拟面板 */
.studio-state-panel {
  padding: 16px 20px;
  background: var(--color-card);
}

.studio-state-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
}

.studio-segmented {
  display: inline-flex;
  align-items: center;
  background: color-mix(in srgb, var(--color-background) 60%, var(--color-card));
  padding: 3px;
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--color-border);
}

.studio-segmented-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border-radius: 6px;
  background: transparent;
  border: none;
  color: var(--color-text-secondary);
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.studio-segmented-btn:hover {
  color: var(--color-foreground);
}

.studio-segmented-btn.active {
  background: var(--color-card);
  color: var(--color-foreground);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.25);
  font-weight: 600;
}

.state-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.state-dot--normal {
  background: #30d158;
}

.state-dot--empty {
  background: #ffd60a;
}

.state-dot--error {
  background: #ff453a;
}

.studio-state-hint {
  font-size: 12.5px;
  color: var(--color-text-tertiary);
}

/* macOS 风格 Window Bar */
.studio-box-header {
  padding: 10px 16px;
  background: color-mix(in srgb, var(--color-background) 30%, var(--color-card));
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.studio-window-dots {
  display: flex;
  align-items: center;
  gap: 6px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.dot-red {
  background: #ff5f56;
}

.dot-yellow {
  background: #ffbd2e;
}

.dot-green {
  background: #27c93f;
}

.studio-window-filename {
  margin-left: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  font-family: var(--font-mono, monospace);
}

.studio-editor-actions {
  display: flex;
  align-items: center;
  gap: 6px;
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
  transition: all 0.15s ease;
}

.studio-copy-action:hover {
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border-color: color-mix(in srgb, var(--color-primary) 25%, transparent);
}

.studio-copy-action--reset {
  color: #ff453a;
}

.studio-copy-action--reset:hover {
  background: rgba(255, 69, 58, 0.12);
  border-color: rgba(255, 69, 58, 0.25);
}

/* 编辑器区域 */
.studio-editor-body {
  background: var(--color-card);
}

.studio-syntax-code {
  margin: 0;
  padding: 14px 16px;
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  color: var(--color-foreground);
  line-height: 1.6;
  tab-size: 2;
}

.studio-code-textarea {
  width: 100%;
  box-sizing: border-box;
  min-height: 240px;
  max-height: 380px;
  background: transparent;
  border: none;
  outline: none;
  resize: vertical;
}

.studio-editor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: color-mix(in srgb, var(--color-background) 30%, var(--color-card));
  border-top: 1px solid var(--color-border);
  font-size: 11.5px;
}

.studio-syntax-status {
  display: flex;
  align-items: center;
  gap: 6px;
}

.studio-syntax-status.is-valid {
  color: #30d158;
}

.studio-syntax-status.is-error {
  color: #ff453a;
  font-weight: 500;
}

.studio-editor-length {
  color: var(--color-text-quaternary);
  font-family: var(--font-mono, monospace);
}

/* 实时预览区域 */
.studio-preview-panel {
  background: var(--color-card);
}

.studio-preview-canvas {
  background: #1c1c1e;
  min-height: 120px;
  position: relative;
}

.canvas-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 20px;
  text-align: center;
  font-size: 13px;
}

.canvas-error {
  color: #ff453a;
  background: rgba(255, 69, 58, 0.08);
}

.studio-preview-iframe {
  display: block;
  width: 100%;
  border: none;
  background: transparent;
  transition: height 0.2s ease;
}
</style>
