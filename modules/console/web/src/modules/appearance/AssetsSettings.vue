<script setup lang="ts">
import AttachmentField from "@/components/AttachmentField.vue";
import { VButton } from "@/components/ui";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
</script>

<template>
  <div class="settings-basic-section settings-basic-section-first">
    <div class="settings-basic-section-heading">
      <h2>主题默认素材</h2>
      <p class="settings-hint">各场景占位图 URL；留空回退主题内置默认素材。</p>
    </div>
    <div class="settings-form-grid settings-form-grid-2">
      <div class="settings-field"><label>头像</label><AttachmentField v-model="data.default.avatar" /></div>
      <div class="settings-field"><label>链接卡片图标</label><AttachmentField v-model="data.default.link" /></div>
      <div class="settings-field"><label>封面</label><AttachmentField v-model="data.default.cover" /></div>
      <div class="settings-field"><label>正文插图</label><AttachmentField v-model="data.default.image" /></div>
      <div class="settings-field settings-field-wide"><label>图片加载失败占位图（image_onerror）</label><AttachmentField v-model="data.default.image_onerror" placeholder="可填图片 URL 或 data URI" /></div>
      <div class="settings-field"><label>项目卡片</label><AttachmentField v-model="data.default.project" /></div>
      <div class="settings-field"><label>横幅</label><AttachmentField v-model="data.default.banner" /></div>
      <div class="settings-field"><label>专栏</label><AttachmentField v-model="data.default.topic" /></div>
      <div class="settings-field"><label>加载图标</label><AttachmentField v-model="data.default.loading" /></div>
    </div>
    <div class="settings-card-actions"><VButton type="primary" size="sm" :disabled="Boolean(saving)" :loading="saving === 'default'" @click="emit('save', 'default')">保存素材</VButton></div>
  </div>
</template>
