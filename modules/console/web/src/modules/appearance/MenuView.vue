<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VPageHeader } from "@/components/ui";
import CanonicalSettingsNotice from "@/modules/settings/CanonicalSettingsNotice.vue";
</script>

<template>
  <div class="menu-page">
    <VPageHeader title="菜单"><template #icon><Icon icon="ri:menu-line" class="text-primary" /></template><template #subtitle>上游：theme.menus</template></VPageHeader>
    <main class="menu-container"><CanonicalSettingsNotice tab="navigation" fields="上游：theme.menus" /></main>
  </div>
</template>

<style scoped>
.menu-page { min-height: 100%; background: var(--color-background); }
.menu-container { width: 100%; max-width: 1100px; margin: 0 auto; padding: 24px; box-sizing: border-box; }
.menu-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--color-text-tertiary); font-size: 13px; }
.menu-error { color: var(--color-state-error); }
.menu-spin { animation: menu-spin 1s linear infinite; }
@keyframes menu-spin { to { transform: rotate(360deg); } }
@media (max-width: 640px) { .menu-container { padding: 12px; } }
</style>
