<script setup lang="ts">
import { onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
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
    <VPageHeader title="主题设置"><template #icon><Icon icon="ri:palette-line" class="text-primary" /></template><template #subtitle>Stellar 2 上游主题配置与独立 Blog 扩展</template></VPageHeader>
    <main class="appearance-container"><CanonicalSettingsNotice fields="上游：theme.appearance、theme.layout、theme.menus、theme.widgets、theme.features" /></main>
  </div>
</template>

<style scoped>
.appearance-page {
  min-height: 100%;
  background: var(--color-background);
}
.appearance-container {
  width: 100%;
  max-width: 1100px;
  margin: 0 auto;
  padding: 24px;
  box-sizing: border-box;
}
.appearance-content {
  width: 100%;
}
.appearance-tabs button:focus-visible {
  outline: 2px solid var(--color-ring, var(--color-primary, #2e8dff));
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
    padding: 12px;
  }
}
</style>
