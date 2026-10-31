<script setup lang="ts">
import { onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { VPageHeader } from "@/components/ui";
import CanonicalSettingsNotice from "@/modules/settings/CanonicalSettingsNotice.vue";
const route = useRoute();
const router = useRouter();
onMounted(() => {
  if (route.query.tab === "feedback") void router.replace({ path: "/settings", query: { tab: "feedback" } });
});
</script>

<template>
  <div class="appearance-page">
    <VPageHeader title="主题设置" />
    <main class="appearance-container"><CanonicalSettingsNotice tab="appearance" fields="上游：theme.appearance、theme.layout、theme.menus、theme.widgets、theme.features" /></main>
  </div>
</template>

<style scoped>
.appearance-page {
  min-height: 100%;
  background: var(--color-background);
}
.appearance-container {
  width: 100%;
  max-width: 1080px;
  margin: 0;
  padding: 8px 32px 40px;
  box-sizing: border-box;
}
/* 外观页只保留一段说明，不再套第二层卡片。 */
.appearance-container :deep(.card-wrapper) {
  background: transparent;
  border: 0;
  border-top: 1px solid var(--color-border);
  border-radius: 0;
  box-shadow: none;
}
.appearance-container :deep(.card-header) {
  padding: 16px 0 8px;
  border-bottom: 0;
}
.appearance-container :deep(.card-body) {
  padding: 0 0 16px;
}
.appearance-content {
  width: 100%;
}
.appearance-tabs button:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}
.appearance-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 60px 0;
  color: var(--color-text-tertiary);
  font-size: 13px;
}
.appearance-error {
  color: var(--color-state-error);
}
.appearance-spin {
  animation: appearance-spin 1s linear infinite;
}
@keyframes appearance-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (max-width: 640px) {
  .appearance-container {
    padding: 0 16px 32px;
  }
}
</style>
