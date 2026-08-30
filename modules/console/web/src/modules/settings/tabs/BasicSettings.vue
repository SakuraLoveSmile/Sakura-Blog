<script setup lang="ts">
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import { Toast, VButton } from "@/components/ui";
import { headingStyleMode as styleMode } from "../style-mode";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";

function handleSubmit() {
  const title = (props.data?.site?.title || "").trim();
  if (!title) {
    Toast.error("站点标题不能为空");
    document.getElementById("settings-site-title")?.focus();
    return;
  }
  emit('save', 'site');
}
</script>

<template>
  <form class="settings-basic-form" @submit.prevent="handleSubmit">
    <div :class="`settings-style-${styleMode}`">
      <div class="settings-style-preview-toolbar">
      <div class="settings-style-toolbar-header">
        <span class="settings-style-toolbar-title">
          <Icon icon="ri:layout-grid-line" />
          二级标题与分组方案实时对比
        </span>
        <span class="settings-style-toolbar-hint">点击以下按钮可实时切换整页排版样式</span>
      </div>
      <div class="settings-style-switcher" role="radiogroup" aria-label="二级标题样式切换">
        <button
          type="button"
          class="settings-style-btn"
          :class="{ active: styleMode === 'panel' }"
          @click="styleMode = 'panel'"
        >
          <span class="settings-style-btn-tag">方案 1</span>
          <span class="settings-style-btn-name">胶囊指示条 + 内嵌面板（推荐）</span>
        </button>
        <button
          type="button"
          class="settings-style-btn"
          :class="{ active: styleMode === 'card' }"
          @click="styleMode = 'card'"
        >
          <span class="settings-style-btn-tag">方案 2</span>
          <span class="settings-style-btn-name">独立次级卡片（Sub-Card）</span>
        </button>
        <button
          type="button"
          class="settings-style-btn"
          :class="{ active: styleMode === 'tree' }"
          @click="styleMode = 'tree'"
        >
          <span class="settings-style-btn-tag">方案 3</span>
          <span class="settings-style-btn-name">标题色带 + 层级引导线</span>
        </button>
      </div>
    </div>

    <section class="settings-basic-field settings-basic-field-first">
      <label for="settings-site-title">站点标题 <span class="settings-required" aria-hidden="true">*</span></label>
      <input id="settings-site-title" v-model="data.site.title" :class="[inputCls, 'settings-basic-input']" required />
    </section>

    <section class="settings-basic-field">
      <label for="settings-site-subtitle">站点副标题</label>
      <input id="settings-site-subtitle" v-model="data.site.subtitle" :class="[inputCls, 'settings-basic-input']" />
    </section>

    <section class="settings-basic-section">
      <div class="settings-basic-section-heading">
        <h2>Logo</h2>
        <p>站点头像和 Logo 文案会显示在前台导航区域。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-basic-asset">
          <div class="settings-basic-asset-controls">
            <div class="settings-basic-label">Logo 主图</div>
            <AttachmentField v-model="data.site.avatar" placeholder="输入图片 URL" show-thumb />
            <p class="settings-basic-help">当前主题没有独立 Logo 图标时使用站点头像作为 Logo。</p>
          </div>
        </div>

        <div class="settings-basic-asset">
          <div class="settings-basic-asset-controls">
            <div class="settings-basic-label">Logo 悬停背景图（可选）</div>
            <AttachmentField v-model="data.site.avatar_bg" placeholder="输入图片 URL" show-thumb />
            <p class="settings-basic-help">鼠标悬停在头像上时使用的背景图，留空则不启用。</p>
          </div>
        </div>

        <div class="settings-basic-subfields">
          <div class="settings-basic-subfield">
            <label for="settings-logo-title">Logo 主标题</label>
            <input id="settings-logo-title" v-model="data.site.logo_title" :class="[inputCls, 'settings-basic-input']" />
          </div>
          <div class="settings-basic-subfield">
            <label for="settings-logo-sub">Logo 副标题</label>
            <input
              id="settings-logo-sub"
              v-model="data.site.logo_sub"
              :class="[inputCls, 'settings-basic-input']"
              placeholder="含 | 时拆分为悬停前后的两段文案"
            />
          </div>
        </div>

        <div v-if="data.site.avatar || data.site.avatar_bg" class="settings-preview-box settings-basic-preview-box">
          <p class="settings-preview-label">Logo 使用预览</p>
          <div class="settings-preview-row">
            <div v-if="data.site.avatar" class="settings-preview-item">
              <p>导航栏头像 32×32</p>
              <div class="settings-avatar-32"><img :src="data.site.avatar" alt="" /></div>
            </div>
            <div v-if="data.site.avatar" class="settings-preview-item">
              <p>分享卡片 128×128</p>
              <div class="settings-avatar-128"><img :src="data.site.avatar" alt="" /></div>
            </div>
            <div v-if="data.site.avatar && data.site.avatar_bg" class="settings-preview-item">
              <p>悬停效果</p>
              <div class="settings-avatar-hover">
                <img :src="data.site.avatar_bg" class="settings-avatar-bg" alt="" /><img
                  :src="data.site.avatar"
                  class="settings-avatar-fg"
                  alt=""
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="settings-basic-section">
      <div class="settings-basic-section-heading">
        <h2>Favicon</h2>
        <p>浏览器标签页和收藏夹使用的站点图标。</p>
      </div>
      <div class="settings-section-body">
        <div class="settings-basic-asset">
          <div class="settings-basic-asset-controls">
            <div class="settings-basic-label">Favicon</div>
            <AttachmentField
              v-model="data.site.favicon_url"
              placeholder="留空使用默认 /favicon.ico"
              show-thumb
              fallback-src="/favicon.ico"
            />
          </div>
        </div>
      </div>
    </section>

    <section class="settings-basic-section">
      <div class="settings-basic-section-heading">
        <h2>Apple Touch Icon</h2>
        <p>添加到 iOS/iPadOS 主屏幕时使用的图标。</p>
      </div>
      <div class="settings-section-body">
        <div class="settings-basic-asset">
          <div class="settings-basic-asset-controls">
            <div class="settings-basic-label">Apple Touch Icon</div>
            <AttachmentField
              v-model="data.site.apple_touch_icon"
              placeholder="留空使用默认 /apple-touch-icon.png"
              show-thumb
              fallback-src="/apple-touch-icon.png"
            />
          </div>
        </div>
      </div>
    </section>
    </div>

    <div class="settings-basic-actions">
      <div class="settings-basic-actions-inner">
        <VButton type="primary" size="md" native-type="submit" :loading="saving === 'site'">
          <template #icon><Icon icon="ri:save-line" /></template>保存
        </VButton>
      </div>
    </div>
  </form>
</template>

<style scoped>
.settings-basic-form {
  background: var(--color-card);
}

/* 顶部方案预览对比工具条 */
.settings-style-preview-toolbar {
  max-width: 860px;
  margin: 24px 28px 12px;
  padding: 16px 20px;
  background: color-mix(in srgb, var(--color-primary) 8%, var(--color-card));
  border: 1px dashed color-mix(in srgb, var(--color-primary) 35%, var(--color-border));
  border-radius: var(--radius-xl);
}

.settings-style-toolbar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.settings-style-toolbar-title {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13.5px;
  font-weight: 650;
  color: var(--color-primary);
}

.settings-style-toolbar-hint {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.settings-style-switcher {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.settings-style-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  color: var(--color-text-secondary);
  font-size: 12.5px;
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-standard);
}

.settings-style-btn:hover {
  border-color: var(--color-primary);
  color: var(--color-foreground);
}

.settings-style-btn.active {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #ffffff;
  font-weight: 600;
  box-shadow: 0 2px 10px color-mix(in srgb, var(--color-primary) 35%, transparent);
}

.settings-style-btn-tag {
  display: inline-block;
  padding: 1px 6px;
  border-radius: var(--radius-xs, 4px);
  background: color-mix(in srgb, var(--color-foreground) 10%, transparent);
  font-size: 11px;
  font-family: var(--font-mono);
}

.settings-style-btn.active .settings-style-btn-tag {
  background: rgba(255, 255, 255, 0.22);
  color: #ffffff;
}

/* ================= 通用字段与说明文本基础排版 ================= */
.settings-basic-field {
  margin: 0;
  padding: 24px 28px;
  border-bottom: 1px solid var(--color-border);
}

.settings-basic-field-first {
  padding-top: 24px;
}

.settings-basic-field label,
.settings-basic-subfield label {
  display: block;
  margin-bottom: 10px;
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
}

.settings-required {
  color: var(--color-state-error);
}

.settings-basic-input {
  display: block;
  width: min(100%, 860px);
}

.settings-basic-section-heading p,
.settings-basic-help {
  margin: 6px 0 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}

.settings-basic-asset {
  display: block;
  max-width: 860px;
  margin-top: 20px;
}

.settings-basic-asset:first-child {
  margin-top: 0;
}

.settings-basic-asset-controls {
  min-width: 0;
  max-width: 860px;
}

.settings-basic-label {
  margin-bottom: 10px;
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 600;
}

.settings-basic-subfields {
  max-width: 860px;
  margin-top: 24px;
}

.settings-basic-subfield + .settings-basic-subfield {
  margin-top: 20px;
}

.settings-basic-preview-box {
  margin-top: 24px;
  margin-bottom: 0;
}

/* ================= 方案 1: 胶囊指示条 + 内嵌面板 (Panel) ================= */
.settings-style-panel .settings-basic-section {
  margin: 0;
  padding: 28px;
  border-bottom: 1px solid var(--color-border);
}

.settings-style-panel .settings-basic-section-heading {
  max-width: 860px;
  margin-bottom: 18px;
}

.settings-style-panel .settings-basic-section-heading h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--color-foreground);
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.settings-style-panel .settings-basic-section-heading h2::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 16px;
  border-radius: 2px;
  background: var(--color-primary);
  flex-shrink: 0;
}

.settings-style-panel .settings-section-body {
  max-width: 860px;
  padding: 22px 24px;
  background: color-mix(in srgb, var(--color-background) 38%, var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
}

/* ================= 方案 2: 独立次级卡片 (Card) ================= */
.settings-style-card .settings-basic-section {
  max-width: 860px;
  margin: 24px 28px 0;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  background: var(--color-card);
  overflow: hidden;
  box-shadow: var(--shadow-sm);
}

.settings-style-card .settings-basic-section:last-of-type {
  margin-bottom: 28px;
}

.settings-style-card .settings-basic-section-heading {
  margin-bottom: 0;
  padding: 16px 24px;
  background: color-mix(in srgb, var(--color-foreground) 4%, transparent);
  border-bottom: 1px solid var(--color-border);
}

.settings-style-card .settings-basic-section-heading h2 {
  margin: 0;
  color: var(--color-foreground);
  font-size: 15.5px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.settings-style-card .settings-section-body {
  padding: 24px;
}

/* ================= 方案 3: 标题色带 + 层级引导线 (Tree) ================= */
.settings-style-tree .settings-basic-section {
  margin: 0;
  padding: 26px 28px;
  border-bottom: 1px solid var(--color-border);
}

.settings-style-tree .settings-basic-section-heading {
  max-width: 860px;
  margin-bottom: 20px;
  padding: 12px 18px;
  background: color-mix(in srgb, var(--color-primary) 9%, transparent);
  border-left: 3.5px solid var(--color-primary);
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
}

.settings-style-tree .settings-basic-section-heading h2 {
  margin: 0;
  color: var(--color-foreground);
  font-size: 15.5px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.settings-style-tree .settings-section-body {
  max-width: 860px;
  margin-left: 12px;
  padding-left: 24px;
  border-left: 2px solid color-mix(in srgb, var(--color-primary) 32%, var(--color-border));
}

/* ================= 操作底栏 ================= */
.settings-basic-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  padding: 16px 28px calc(16px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-card) 92%, transparent);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.settings-basic-actions-inner {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

@media (max-width: 640px) {
  .settings-style-preview-toolbar {
    margin: 16px 16px 8px;
    padding: 14px 16px;
  }

  .settings-basic-field {
    padding: 20px 16px;
  }

  .settings-style-panel .settings-basic-section,
  .settings-style-tree .settings-basic-section {
    padding: 24px 16px;
  }

  .settings-style-card .settings-basic-section {
    margin: 16px 16px 0;
  }

  .settings-style-panel .settings-section-body,
  .settings-style-card .settings-section-body {
    padding: 18px 16px;
  }

  .settings-style-tree .settings-section-body {
    margin-left: 6px;
    padding-left: 16px;
  }

  .settings-basic-actions {
    padding: 14px 16px calc(14px + env(safe-area-inset-bottom));
  }
}
</style>
