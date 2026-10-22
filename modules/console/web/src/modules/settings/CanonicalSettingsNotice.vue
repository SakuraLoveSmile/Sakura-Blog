<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { getApiErrorMessage, readStellarSettingsProblem, stellarSettingsApi } from "@/api";
import { VButton, VCard } from "@/components/ui";

defineProps<{ fields?: string }>();
const router = useRouter();
const state = ref("loading");
const message = ref("");
async function load() {
  state.value = "loading";
  try {
    await stellarSettingsApi.get();
    state.value = "ready";
  } catch (error) {
    state.value = readStellarSettingsProblem(error)?.type === "stellar_migration_required" ? "migration" : "error";
    message.value = getApiErrorMessage(error, "无法读取 Stellar 2 配置");
  }
}
onMounted(() => void load());
</script>

<template>
  <VCard title="Stellar 2 配置" class="canonical-settings-notice">
    <p v-if="state === 'loading'" role="status">正在检查配置版本…</p>
    <p v-else-if="state === 'migration'">旧配置尚未迁移。请先审阅完整迁移预览，再执行迁移；旧分段现在只用于迁移预览。</p>
    <p v-else-if="state === 'ready'">此处的旧分段已停止保存。请在 Stellar 2 中编辑完整上游配置与独立 Blog 扩展。</p>
    <p v-else role="alert">{{ message }}</p>
    <p v-if="fields" class="canonical-fields">{{ fields }}</p>
    <VButton type="primary" @click="router.push({ path: '/settings', query: { tab: 'stellar2' } })">打开 Stellar 2 配置</VButton>
    <VButton v-if="state === 'error'" type="secondary" @click="load">重试</VButton>
  </VCard>
</template>

<style scoped>
.canonical-settings-notice p { margin: 0 0 16px; color: var(--color-text-secondary); line-height: 1.6; }
.canonical-settings-notice .canonical-fields { color: var(--color-text-tertiary); overflow-wrap: anywhere; }
</style>
