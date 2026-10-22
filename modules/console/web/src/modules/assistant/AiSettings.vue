<script setup lang="ts">
import { onBeforeUnmount, onMounted } from "vue";
import { onBeforeRouteLeave, useRouter } from "vue-router";
import { Toast, VButton, VCard } from "@/components/ui";
import CanonicalSettingsForm from "@/modules/settings/CanonicalSettingsForm.vue";
import { useStellarSettings } from "@/modules/settings/useStellarSettings";

// AI 配置是独立 Blog 扩展 blog_extensions.ai，读写与系统设置的 Stellar 2 原文共用同一套保存协议。
const router = useRouter();
const stellar = useStellarSettings();
const { draft, loading, busy, loadError, migration, parseError, dirty, loadRevision } = stellar;

async function save() {
  if (await stellar.submit("save")) Toast.success("AI 设置已保存");
}

function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (!dirty.value) return;
  event.preventDefault();
  event.returnValue = "";
}

onMounted(() => {
  void stellar.load();
  window.addEventListener("beforeunload", handleBeforeUnload);
});

onBeforeUnmount(() => {
  window.removeEventListener("beforeunload", handleBeforeUnload);
});

onBeforeRouteLeave((to, from, next) => {
  if (to.path === from.path || !dirty.value) next();
  else if (window.confirm("您有未保存的 AI 设置，离开后修改将丢失，是否确认离开？")) next();
  else next(false);
});
</script>

<template>
  <div class="ai-settings-page">
    <VCard title="AI 设置">
      <div v-if="loading" class="ai-settings-state" role="status">正在读取 AI 配置…</div>
      <div v-else-if="migration" class="ai-settings-state" role="status">
        <p>旧配置尚未迁移。请先在 Stellar 2 标签中审阅完整迁移预览并确认迁移，然后再配置 AI。</p>
        <VButton type="primary" @click="router.push({ path: '/settings', query: { tab: 'stellar2' } })"
          >打开 Stellar 2 配置</VButton
        >
      </div>
      <div v-else-if="loadError" class="ai-settings-state" role="alert">
        <p class="text-sm break-words text-state-error">{{ loadError }}</p>
        <VButton type="secondary" @click="stellar.load()">重新读取</VButton>
      </div>
      <div v-else-if="parseError" class="ai-settings-state" role="alert">
        <p class="text-sm break-words text-state-error">
          完整配置暂时无法用于表单编辑：{{ parseError }}请在 Stellar 2 标签修正后继续。
        </p>
        <VButton type="primary" @click="router.push({ path: '/settings', query: { tab: 'stellar2' } })"
          >打开 Stellar 2 配置</VButton
        >
      </div>
      <CanonicalSettingsForm
        v-else
        :key="loadRevision"
        tab="ai"
        :document="draft"
        :saving="busy ? 'stellar2' : ''"
        save-label="保存 AI 设置"
        save-help="保存后，之后发起的请求将使用新配置；更换接口地址或向量模型后，请在「知识库」中点击「重建知识库」。"
        invalid-help="请先修正上方输入问题，再保存 AI 设置。"
        @change="stellar.updateField"
        @save="save"
      />
    </VCard>
  </div>
</template>

<style scoped>
.ai-settings-page {
  width: 100%;
  max-width: 900px;
  margin: 0 auto;
}
.ai-settings-state {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
  padding: 24px 28px;
  color: var(--color-text-secondary);
  line-height: 1.6;
}
</style>
