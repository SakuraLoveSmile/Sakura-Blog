<script setup lang="ts">
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import { VButton, VCard } from "@/components/ui";

defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
const textareaCls = "apple-textarea";
</script>

<template>
  <VCard title="站点信息">
    <div class="settings-form-grid settings-form-grid-2">
      <div class="settings-field"><label>站点标题</label><input v-model="data.site.title" :class="inputCls" /></div>
      <div class="settings-field"><label>副标题</label><input v-model="data.site.subtitle" :class="inputCls" /></div>
    </div>
    <div class="settings-field">
      <label>站点描述</label><textarea v-model="data.site.description" rows="2" :class="textareaCls"></textarea>
    </div>
    <div class="settings-field">
      <label>站点 URL（canonical/OG/sitemap 用）</label>
      <input v-model="data.site.url" :class="inputCls" placeholder="https://example.com" />
    </div>
    <div class="settings-form-grid settings-form-grid-2">
      <div class="settings-field">
        <label>头像 URL</label><AttachmentField v-model="data.site.avatar" placeholder="输入图片 URL" />
      </div>
      <div class="settings-field">
        <label>头像悬停背景图</label><AttachmentField v-model="data.site.avatar_bg" placeholder="输入图片 URL" />
      </div>
    </div>
    <div v-if="data.site.avatar || data.site.avatar_bg" class="settings-preview-box">
      <p class="settings-preview-label">实时预览</p>
      <div class="settings-preview-row">
        <div v-if="data.site.avatar" class="settings-preview-item">
          <p>导航栏头像 32×32</p><div class="settings-avatar-32"><img :src="data.site.avatar" alt="" /></div>
        </div>
        <div v-if="data.site.avatar" class="settings-preview-item">
          <p>分享卡片 128×128</p><div class="settings-avatar-128"><img :src="data.site.avatar" alt="" /></div>
        </div>
        <div v-if="data.site.avatar && data.site.avatar_bg" class="settings-preview-item">
          <p>悬停效果</p>
          <div class="settings-avatar-hover"><img :src="data.site.avatar_bg" class="settings-avatar-bg" alt="" /><img :src="data.site.avatar" class="settings-avatar-fg" alt="" /></div>
        </div>
      </div>
    </div>
    <div class="settings-form-grid settings-form-grid-2">
      <div class="settings-field"><label>Logo 主标题</label><input v-model="data.site.logo_title" :class="inputCls" /></div>
      <div class="settings-field"><label>Logo 副标题（含 | 拆滚动）</label><input v-model="data.site.logo_sub" :class="inputCls" /></div>
    </div>
    <div class="settings-form-grid settings-form-grid-2">
      <div class="settings-field">
        <label>Favicon</label><AttachmentField v-model="data.site.favicon_url" placeholder="留空用默认 /favicon.ico" show-thumb fallback-src="/favicon.ico" />
      </div>
      <div class="settings-field">
        <label>Apple Touch Icon</label><AttachmentField v-model="data.site.apple_touch_icon" placeholder="留空用默认" show-thumb fallback-src="/apple-touch-icon.png" />
      </div>
    </div>
    <div class="settings-card-actions">
      <VButton type="primary" size="sm" :loading="saving === 'site'" @click="emit('save', 'site')">
        <template #icon><Icon icon="ri:save-line" /></template>保存站点信息
      </VButton>
    </div>
  </VCard>
</template>
