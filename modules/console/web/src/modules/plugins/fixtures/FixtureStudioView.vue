<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";
import { useFixtureStore } from "./useFixtureStore";
import type { FixtureItem, FixtureState } from "./fixture-types";
import { usePluginPreview } from "../preview/usePluginPreview";
import type { PluginPreviewInfo } from "../plugin-types";

const store = useFixtureStore();

const search = ref("");
const localEditorData = ref("");
const syntaxError = ref("");
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
    validateSyntax();
    preview.resetHeight();
    preview.scheduleRender(currentFixture.value.previewShortcode, true);
  },
  { immediate: true },
);

function onSelect(item: FixtureItem) {
  store.selectFixture(item.id);
}

function validateSyntax() {
  if (currentFixture.value.format === "json") {
    try {
      JSON.parse(localEditorData.value);
      syntaxError.value = "";
    } catch (err: unknown) {
      syntaxError.value = err instanceof Error ? err.message : "JSON 格式非法";
    }
  } else {
    syntaxError.value = "";
  }
}

function onEditorInput() {
  validateSyntax();
  if (!syntaxError.value) {
    store.updateData(currentFixture.value.id, localEditorData.value);
    preview.scheduleRender(currentFixture.value.previewShortcode, false);
  }
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
    syntaxError.value = "";
    store.updateData(currentFixture.value.id, localEditorData.value);
    preview.scheduleRender(currentFixture.value.previewShortcode, true);
  } catch (err: unknown) {
    syntaxError.value = err instanceof Error ? err.message : "JSON 格式非法，无法格式化";
  }
}

function onResetCurrent() {
  store.resetFixture(currentFixture.value.id);
  localEditorData.value = currentFixture.value.defaultData;
  syntaxError.value = "";
  preview.scheduleRender(currentFixture.value.previewShortcode, true);
}

function onResetAll() {
  if (confirm("确定要将所有 Mock Fixtures 恢复为系统默认预设吗？")) {
    store.resetAll();
    localEditorData.value = currentFixture.value.defaultData;
    syntaxError.value = "";
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
        validateSyntax();
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
  <div class="fixture-studio">
    <!-- Top Hero Overview -->
    <div class="studio-hero-card">
      <div class="studio-hero-content">
        <div class="studio-hero-icon">
          <Icon icon="ri:database-2-line" />
        </div>
        <div class="studio-hero-info">
          <div class="studio-hero-title-row">
            <h1>Mock Fixture 数据工作台</h1>
            <span class="studio-hero-badge">Spec 63 / 64 确定性隔离</span>
            <span v-if="store.customCount.value > 0" class="studio-hero-badge studio-hero-badge--active">
              已自定义 {{ store.customCount.value }} 项
            </span>
          </div>
          <p class="studio-hero-desc">
            为标签插件与侧边栏组件提供本地确定性数据源，完全与生产数据库及外部网络隔离。支持直接修改 JSON / Markdown
            数据并实时联动预览画布，快速验证不同数据量与异常降级表现。
          </p>
        </div>
      </div>
      <div class="studio-hero-actions">
        <input ref="fileInputRef" type="file" accept=".json" class="hidden-file-input" @change="handleFileImport" />
        <button class="studio-action-btn" title="导入配置 JSON" @click="triggerImport">
          <Icon icon="ri:upload-2-line" />
          <span>导入</span>
        </button>
        <button class="studio-action-btn" title="导出当前配置" @click="exportConfig">
          <Icon icon="ri:download-2-line" />
          <span>导出</span>
        </button>
        <button
          v-if="store.customCount.value > 0"
          class="studio-action-btn studio-action-btn--reset"
          title="全部恢复默认"
          @click="onResetAll"
        >
          <Icon icon="ri:refresh-line" />
          <span>全部重置</span>
        </button>
      </div>
    </div>

    <!-- Two-Column Workbench Split View -->
    <div class="studio-workbench">
      <!-- Left Sidebar: Fixture List -->
      <aside class="studio-sidebar">
        <div class="studio-sidebar-header">
          <div class="studio-search-bar">
            <Icon icon="ri:search-line" class="studio-search-icon" />
            <input v-model="search" type="text" placeholder="搜索 Mock 数据或端点..." />
            <button v-if="search" class="studio-search-clear" @click="search = ''">
              <Icon icon="ri:close-circle-fill" />
            </button>
          </div>
        </div>

        <div class="studio-fixture-list">
          <button
            v-for="item in filteredFixtures"
            :key="item.id"
            class="studio-fixture-card"
            :class="{ 'is-active': store.selectedId.value === item.id }"
            @click="onSelect(item)"
          >
            <div class="studio-fixture-card-icon">
              <Icon :icon="item.icon" />
            </div>
            <div class="studio-fixture-card-body">
              <div class="studio-fixture-card-title-row">
                <span class="studio-fixture-card-title">{{ item.title }}</span>
                <span class="studio-fixture-card-format">{{ item.format.toUpperCase() }}</span>
              </div>
              <div class="studio-fixture-card-endpoint">{{ item.endpoint }}</div>
            </div>
            <div v-if="store.isCustom(item.id)" class="studio-fixture-custom-dot" title="已自定义修改" />
          </button>
        </div>
      </aside>

      <!-- Right Main: Editor & Live Preview -->
      <main class="studio-main">
        <!-- Fixture Detail Header & Controls -->
        <div class="studio-panel studio-fixture-header-panel">
          <div class="studio-fixture-meta">
            <div class="studio-fixture-title-line">
              <h2>{{ currentFixture.title }}</h2>
              <span class="studio-tag studio-tag--category">{{ currentFixture.category }}</span>
              <button
                class="studio-tag studio-tag--endpoint"
                title="点击复制端点路径"
                @click="copyText(currentFixture.endpoint, 'endpoint')"
              >
                <code>GET {{ currentFixture.endpoint }}</code>
                <Icon :icon="copySuccessKey === 'endpoint' ? 'ri:check-line' : 'ri:file-copy-line'" />
              </button>
            </div>
            <p class="studio-fixture-desc">{{ currentFixture.description }}</p>
            <div class="studio-fixture-shortcodes">
              <span class="studio-shortcodes-label">消费短代码:</span>
              <span v-for="sc in currentFixture.sampleShortcodes" :key="sc" class="studio-shortcode-pill">
                {{ sc }}
              </span>
            </div>
          </div>

          <!-- Status Simulator -->
          <div class="studio-state-controller">
            <span class="studio-state-label">模拟状态:</span>
            <div class="studio-segmented">
              <button
                class="studio-segmented-btn"
                :class="{ 'is-active': currentState === 'normal' }"
                @click="onChangeState('normal')"
              >
                <span class="state-indicator state-indicator--normal" />
                <span>正常 (Normal)</span>
              </button>
              <button
                class="studio-segmented-btn"
                :class="{ 'is-active': currentState === 'empty' }"
                @click="onChangeState('empty')"
              >
                <span class="state-indicator state-indicator--empty" />
                <span>空数据 (Empty)</span>
              </button>
              <button
                class="studio-segmented-btn"
                :class="{ 'is-active': currentState === 'error' }"
                @click="onChangeState('error')"
              >
                <span class="state-indicator state-indicator--error" />
                <span>500 异常 (Error)</span>
              </button>
            </div>
          </div>
        </div>

        <!-- Data Editor Panel -->
        <div class="studio-panel studio-editor-panel">
          <div class="studio-panel-topbar">
            <div class="studio-window-dots">
              <span class="dot dot-red" />
              <span class="dot dot-yellow" />
              <span class="dot dot-green" />
            </div>
            <div class="studio-window-title">
              <Icon :icon="currentFixture.format === 'json' ? 'ri:code-box-line' : 'ri:markdown-line'" />
              <span>{{ currentFixture.id }}.fixture.{{ currentFixture.format === 'json' ? 'json' : 'md' }}</span>
              <span v-if="store.isCustom(currentFixture.id)" class="studio-dirty-dot" title="包含未保存到后端的自定义修改" />
            </div>
            <div class="studio-editor-actions">
              <button
                v-if="currentFixture.format === 'json'"
                class="studio-tool-btn"
                title="格式化 JSON"
                @click="onPrettify"
              >
                <Icon icon="ri:sparkling-line" />
                <span>格式化</span>
              </button>
              <button class="studio-tool-btn" title="复制内容" @click="copyText(localEditorData, 'editor')">
                <Icon :icon="copySuccessKey === 'editor' ? 'ri:check-line' : 'ri:file-copy-line'" />
                <span>{{ copySuccessKey === 'editor' ? '已复制' : '复制' }}</span>
              </button>
              <button
                v-if="store.isCustom(currentFixture.id)"
                class="studio-tool-btn studio-tool-btn--reset"
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
              class="studio-code-textarea"
              spellcheck="false"
              placeholder="在此输入自定义 Mock 数据..."
              @input="onEditorInput"
            />
          </div>

          <div class="studio-editor-footer">
            <div v-if="syntaxError" class="studio-syntax-status is-error">
              <Icon icon="ri:error-warning-line" />
              <span>{{ syntaxError }}</span>
            </div>
            <div v-else class="studio-syntax-status is-valid">
              <Icon icon="ri:checkbox-circle-line" />
              <span>{{ currentFixture.format === 'json' ? '✓ JSON 语法有效' : '✓ Markdown 内容有效' }}</span>
            </div>
            <div class="studio-editor-length">{{ localEditorData.length }} 字符</div>
          </div>
        </div>

        <!-- Live Preview Canvas -->
        <div class="studio-panel studio-canvas-panel">
          <div class="studio-panel-topbar">
            <div class="studio-window-dots">
              <span class="dot dot-red" />
              <span class="dot dot-yellow" />
              <span class="dot dot-green" />
            </div>
            <div class="studio-window-title">
              <Icon icon="ri:play-circle-line" />
              <span>实时效果预览 · <code>{{ currentFixture.previewShortcode }}</code></span>
            </div>
            <div class="studio-canvas-actions">
              <button
                class="studio-tool-btn"
                title="刷新预览"
                :disabled="preview.rendering.value"
                @click="preview.scheduleRender(currentFixture.previewShortcode, true)"
              >
                <Icon icon="ri:refresh-line" :class="{ 'animate-spin': preview.rendering.value }" />
                <span>{{ preview.rendering.value ? '渲染中...' : '重新渲染' }}</span>
              </button>
              <button
                class="studio-tool-btn"
                title="复制短代码"
                @click="copyText(currentFixture.previewShortcode, 'shortcode')"
              >
                <Icon :icon="copySuccessKey === 'shortcode' ? 'ri:check-line' : 'ri:file-copy-line'" />
                <span>{{ copySuccessKey === 'shortcode' ? '已复制' : '复制语法' }}</span>
              </button>
            </div>
          </div>

          <div class="studio-canvas-body">
            <div v-if="preview.previewError.value" class="studio-canvas-error">
              <Icon icon="ri:error-warning-line" />
              <p>{{ preview.previewError.value }}</p>
            </div>
            <iframe
              ref="previewFrame"
              class="studio-preview-iframe"
              :srcdoc="preview.srcdoc.value"
              sandbox="allow-same-origin allow-scripts"
              :style="{ height: `${preview.frameHeight.value}px` }"
              title="Fixture Live Preview"
            />
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.fixture-studio {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.hidden-file-input {
  display: none;
}

/* Hero Card */
.studio-hero-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 24px 28px;
  background: var(--card, #1e1e20);
  border: 1px solid var(--block-border, rgba(255, 255, 255, 0.08));
  border-radius: 18px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.18);
}

.studio-hero-content {
  display: flex;
  align-items: flex-start;
  gap: 18px;
}

.studio-hero-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(74, 108, 247, 0.22), rgba(168, 85, 247, 0.22));
  color: var(--accent-primary, #4a6cf7);
  font-size: 26px;
  flex-shrink: 0;
  border: 1px solid rgba(74, 108, 247, 0.3);
}

.studio-hero-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.studio-hero-title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.studio-hero-title-row h1 {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-p1, #fff);
  margin: 0;
  letter-spacing: -0.01em;
}

.studio-hero-badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-p3, #8e8e93);
  font-family: var(--font-mono, monospace);
}

.studio-hero-badge--active {
  background: rgba(74, 108, 247, 0.15);
  color: var(--accent-primary, #4a6cf7);
  border: 1px solid rgba(74, 108, 247, 0.3);
}

.studio-hero-desc {
  font-size: 13px;
  line-height: 1.55;
  color: var(--text-p2, #aeaeb2);
  margin: 0;
  max-width: 820px;
}

.studio-hero-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.studio-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding: 0 14px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--block-border, rgba(255, 255, 255, 0.08));
  color: var(--text-p1, #fff);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.studio-action-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.16);
}

.studio-action-btn--reset {
  color: #ff453a;
  border-color: rgba(255, 69, 58, 0.25);
  background: rgba(255, 69, 58, 0.08);
}

.studio-action-btn--reset:hover {
  background: rgba(255, 69, 58, 0.18);
  border-color: rgba(255, 69, 58, 0.4);
}

/* Workbench Split View */
.studio-workbench {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 20px;
  align-items: start;
}

@media (max-width: 1024px) {
  .studio-workbench {
    grid-template-columns: 1fr;
  }
}

/* Left Sidebar */
.studio-sidebar {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.studio-search-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--card, #1e1e20);
  border: 1px solid var(--block-border, rgba(255, 255, 255, 0.08));
  border-radius: 12px;
  padding: 0 12px;
  height: 40px;
}

.studio-search-icon {
  font-size: 16px;
  color: var(--text-p3, #8e8e93);
}

.studio-search-bar input {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--text-p1, #fff);
  font-size: 13px;
  outline: none;
}

.studio-search-bar input::placeholder {
  color: var(--text-p3, #8e8e93);
}

.studio-search-clear {
  background: transparent;
  border: none;
  color: var(--text-p3, #8e8e93);
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
}

.studio-fixture-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.studio-fixture-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--card, #1e1e20);
  border: 1px solid var(--block-border, rgba(255, 255, 255, 0.08));
  border-radius: 14px;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s ease;
}

.studio-fixture-card:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.16);
  transform: translateY(-1px);
}

.studio-fixture-card.is-active {
  background: rgba(74, 108, 247, 0.1);
  border-color: var(--accent-primary, #4a6cf7);
  box-shadow: 0 0 0 1px var(--accent-primary, #4a6cf7);
}

.studio-fixture-card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--accent-primary, #4a6cf7);
  font-size: 20px;
  flex-shrink: 0;
}

.studio-fixture-card.is-active .studio-fixture-card-icon {
  background: var(--accent-primary, #4a6cf7);
  color: #fff;
}

.studio-fixture-card-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.studio-fixture-card-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.studio-fixture-card-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-p1, #fff);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.studio-fixture-card-format {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-p2, #aeaeb2);
  font-family: var(--font-mono, monospace);
}

.studio-fixture-card-endpoint {
  font-size: 11px;
  color: var(--text-p3, #8e8e93);
  font-family: var(--font-mono, monospace);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.studio-fixture-custom-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #30d158;
  box-shadow: 0 0 6px rgba(48, 209, 88, 0.6);
  flex-shrink: 0;
}

/* Right Main Studio */
.studio-main {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.studio-panel {
  background: var(--card, #1e1e20);
  border: 1px solid var(--block-border, rgba(255, 255, 255, 0.08));
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.12);
}

/* Fixture Header Panel */
.studio-fixture-header-panel {
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.studio-fixture-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.studio-fixture-title-line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.studio-fixture-title-line h2 {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-p1, #fff);
  margin: 0;
}

.studio-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 6px;
}

.studio-tag--category {
  background: rgba(74, 108, 247, 0.15);
  color: var(--accent-primary, #4a6cf7);
  font-weight: 600;
}

.studio-tag--endpoint {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-p2, #aeaeb2);
  cursor: pointer;
  transition: all 0.15s ease;
}

.studio-tag--endpoint:hover {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}

.studio-tag--endpoint code {
  font-family: var(--font-mono, monospace);
  font-size: 11px;
}

.studio-fixture-desc {
  font-size: 13px;
  color: var(--text-p2, #aeaeb2);
  margin: 0;
  line-height: 1.5;
}

.studio-fixture-shortcodes {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
}

.studio-shortcodes-label {
  color: var(--text-p3, #8e8e93);
  font-weight: 500;
}

.studio-shortcode-pill {
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--accent-primary, #4a6cf7);
}

/* State Controller */
.studio-state-controller {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.studio-state-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-p2, #aeaeb2);
}

.studio-segmented {
  display: inline-flex;
  align-items: center;
  background: rgba(0, 0, 0, 0.3);
  padding: 3px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.studio-segmented-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border-radius: 8px;
  background: transparent;
  border: none;
  color: var(--text-p2, #aeaeb2);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.studio-segmented-btn:hover {
  color: var(--text-p1, #fff);
}

.studio-segmented-btn.is-active {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.2);
}

.state-indicator {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.state-indicator--normal {
  background: #30d158;
}

.state-indicator--empty {
  background: #ffd60a;
}

.state-indicator--error {
  background: #ff453a;
}

/* Panel Topbar */
.studio-panel-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  background: rgba(0, 0, 0, 0.25);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
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

.studio-window-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-p2, #aeaeb2);
  font-family: var(--font-mono, monospace);
}

.studio-dirty-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #4a6cf7;
}

.studio-editor-actions,
.studio-canvas-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.studio-tool-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 26px;
  padding: 0 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: var(--text-p1, #fff);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.studio-tool-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.16);
}

.studio-tool-btn--reset {
  color: #ff453a;
}

/* Editor Body */
.studio-editor-body {
  position: relative;
  min-height: 240px;
  max-height: 380px;
  display: flex;
}

.studio-code-textarea {
  width: 100%;
  min-height: 240px;
  max-height: 380px;
  padding: 16px;
  background: transparent;
  border: none;
  outline: none;
  resize: vertical;
  color: var(--text-p1, #fff);
  font-family: "JetBrains Mono", Menlo, Monaco, Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.6;
  tab-size: 2;
  white-space: pre;
}

.studio-editor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: rgba(0, 0, 0, 0.2);
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  font-size: 11px;
}

.studio-syntax-status {
  display: flex;
  align-items: center;
  gap: 5px;
}

.studio-syntax-status.is-valid {
  color: #30d158;
}

.studio-syntax-status.is-error {
  color: #ff453a;
  font-weight: 500;
}

.studio-editor-length {
  color: var(--text-p3, #8e8e93);
  font-family: var(--font-mono, monospace);
}

/* Preview Canvas Body */
.studio-canvas-body {
  position: relative;
  background: var(--background, #141416);
  min-height: 160px;
}

.studio-canvas-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: rgba(255, 69, 58, 0.1);
  border-bottom: 1px solid rgba(255, 69, 58, 0.2);
  color: #ff453a;
  font-size: 13px;
}

.studio-preview-iframe {
  display: block;
  width: 100%;
  border: none;
  transition: height 0.2s ease;
}
</style>
