<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VPageHeader } from "@/components/ui";
import CanonicalSettingsNotice from "@/modules/settings/CanonicalSettingsNotice.vue";
</script>

<template>
  <div class="authors-page">
    <VPageHeader title="作者"><template #icon><Icon icon="ri:user-star-line" class="text-primary" /></template><template #subtitle>公开作者资料：blog_extensions.data.authors</template></VPageHeader>
    <main class="authors-container"><CanonicalSettingsNotice tab="community" fields="公开作者资料：blog_extensions.data.authors" /></main>
  </div>
</template>

<style scoped>
.authors-page { min-height: 100%; background: var(--color-background); }
.authors-container { width: 100%; max-width: 1100px; margin: 0 auto; padding: 24px; box-sizing: border-box; }
.authors-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--color-text-tertiary); font-size: 13px; }
.authors-error { color: var(--color-state-error); }
@media (max-width: 640px) { .authors-container { padding: 12px; } }
</style>
