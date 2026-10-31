<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { getApiErrorMessage, readStellarSettingsProblem, stellarSettingsApi } from "@/api";
import { VButton } from "@/components/ui";

const props = withDefaults(defineProps<{ fields?: string; tab?: string }>(), { tab: "advanced" });
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
  <section class="canonical-settings-notice" aria-labelledby="canonical-settings-notice-title">
    <h2 id="canonical-settings-notice-title">Stellar 2 配置</h2>
    <p v-if="state === 'loading'" role="status">正在检查配置版本…</p>
    <p v-else-if="state === 'migration'">旧配置尚未迁移。请先审阅完整迁移预览，再执行迁移；旧分段现在只用于迁移预览。</p>
    <p v-else-if="state === 'ready'">此处的旧分段已停止保存。请在系统设置的对应分类中修改配置。</p>
    <p v-else role="alert">{{ message }}</p>
    <p v-if="fields" class="canonical-fields">{{ fields }}</p>
    <div class="canonical-settings-actions">
      <VButton type="primary" @click="router.push({ path: '/settings', query: { tab: props.tab } })">打开设置</VButton>
      <VButton v-if="state === 'error'" type="secondary" @click="load">重试</VButton>
    </div>
  </section>
</template>

<style scoped>
.canonical-settings-notice {
  padding: 8px 0 24px;
  border-bottom: 1px solid var(--color-border);
}
.canonical-settings-notice h2 {
  margin: 0 0 12px;
  color: var(--color-foreground);
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.canonical-settings-notice p {
  margin: 0 0 16px;
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}
.canonical-settings-notice .canonical-fields { color: var(--color-text-tertiary); overflow-wrap: anywhere; }
.canonical-settings-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
</style>
