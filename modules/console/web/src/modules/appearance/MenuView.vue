<script setup lang="ts">
import { VPageHeader } from "@/components/ui";
import CanonicalSettingsNotice from "@/modules/settings/CanonicalSettingsNotice.vue";
</script>

<template>
  <div class="menu-page">
    <VPageHeader title="菜单" />
    <main class="menu-container"><CanonicalSettingsNotice tab="navigation" fields="上游：theme.menus" /></main>
  </div>
</template>

<style scoped>
.menu-page { min-height: 100%; background: var(--color-background); }
.menu-container { width: 100%; max-width: 1080px; margin: 0; padding: 8px 32px 40px; box-sizing: border-box; }
.menu-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--color-text-tertiary); font-size: 13px; }
.menu-error { color: var(--color-state-error); }
.menu-spin { animation: menu-spin 1s linear infinite; }
@keyframes menu-spin { to { transform: rotate(360deg); } }
@media (max-width: 767px) { .menu-container { padding: 0 16px 32px; } }
</style>
