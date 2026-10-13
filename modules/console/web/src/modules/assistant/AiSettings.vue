<script setup lang="ts">
import CanonicalSettingsNotice from "@/modules/settings/CanonicalSettingsNotice.vue";
</script>

<template>
  <div class="ai-settings-page"><CanonicalSettingsNotice fields="本地 AI：blog_extensions.ai；摘要开关：ai.summary；问答开关：ai.ask。**** 保留密钥，空字符串或 null 清除。" /></div>
</template>

<style scoped>
.ai-settings-page { width: 100%; max-width: 900px; margin: 0 auto; }
</style>
