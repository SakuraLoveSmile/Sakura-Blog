<script setup lang="ts">
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import { VButton } from "@/components/ui";

defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
</script>

<template>
  <div class="settings-basic-form">
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
    </section>

    <section class="settings-basic-section">
      <div class="settings-basic-section-heading">
        <h2>Favicon</h2>
        <p>浏览器标签页和收藏夹使用的站点图标。</p>
      </div>
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
    </section>

    <section class="settings-basic-section">
      <div class="settings-basic-section-heading">
        <h2>Apple Touch Icon</h2>
        <p>添加到 iOS/iPadOS 主屏幕时使用的图标。</p>
      </div>
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
    </section>

    <div class="settings-basic-actions">
      <div class="settings-basic-actions-inner">
        <VButton type="primary" size="md" :loading="saving === 'site'" @click="emit('save', 'site')">
          <template #icon><Icon icon="ri:save-line" /></template>保存
        </VButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-basic-form {
  background: var(--color-card);
}
.settings-basic-field,
.settings-basic-section {
  margin: 0;
  padding: 26px 28px;
  border-bottom: 1px solid var(--color-border);
}
.settings-basic-field-first {
  padding-top: 30px;
}
.settings-basic-field label,
.settings-basic-subfield label {
  display: block;
  margin-bottom: 12px;
  color: var(--color-foreground);
  font-size: 15px;
  font-weight: 600;
}
.settings-required {
  color: var(--color-state-error);
}
.settings-basic-input {
  display: block;
  width: min(100%, 860px);
}
.settings-basic-section-heading {
  margin-bottom: 22px;
}
.settings-basic-section-heading h2 {
  margin: 0;
  color: var(--color-foreground);
  font-size: 17px;
  font-weight: 650;
  letter-spacing: -0.01em;
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
  margin-top: 24px;
}
.settings-basic-section-heading + .settings-basic-asset {
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
  margin-top: 28px;
}
.settings-basic-subfield + .settings-basic-subfield {
  margin-top: 22px;
}
.settings-basic-preview-box {
  margin-top: 28px;
  margin-bottom: 0;
}
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
  .settings-basic-field,
  .settings-basic-section {
    padding: 22px 16px;
  }
  .settings-basic-field-first {
    padding-top: 24px;
  }
  .settings-basic-actions {
    padding: 14px 16px calc(14px + env(safe-area-inset-bottom));
  }
}
</style>
