<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VCard, VSwitch } from "@/components/ui";

const data = ref<Record<string, any>>({});
const loading = ref(true);
const saving = ref(false);

async function load() {
  loading.value = true;
  try {
    const { data: response } = await settingsApi.all();
    data.value = { ...(response.ai || {}) };
    data.value.embedding_model = data.value.embedding_model || "";
    data.value.summary_enabled = data.value.summary_enabled ?? true;
    data.value.allow_user_api = data.value.allow_user_api ?? true;
    data.value.qa_enabled = data.value.qa_enabled ?? false;
    data.value.qa_allow_user_llm = data.value.qa_allow_user_llm ?? true;
  } catch {
    Toast.error("加载 AI 设置失败");
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    await settingsApi.updateSection("ai", data.value);
    Toast.success("AI 设置已保存");
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "AI 设置保存失败"));
  } finally {
    saving.value = false;
  }
}

onMounted(() => void load());
</script>

<template>
  <div class="ai-settings-page">
    <VCard title="AI 服务">
      <p class="ai-settings-hint">编辑器摘要、知识库索引和 AI 助手使用的 OpenAI 兼容接口（DeepSeek / 通义 / Ollama 等均可）。</p>
      <div v-if="loading" class="ai-settings-loading"><Icon icon="ri:loader-4-line" class="ai-settings-spin" />加载中...</div>
      <template v-else>
        <div class="ai-settings-field"><label>API 地址（如 https://api.deepseek.com/v1）</label><input v-model="data.base_url" class="apple-input" placeholder="https://api.openai.com/v1" /></div>
        <div class="ai-settings-field"><label>API Key（本地模型可留空；已保存时显示 ****，留空保存不覆盖）</label><input v-model="data.api_key" type="password" autocomplete="off" class="apple-input" /></div>
        <div class="ai-settings-field"><label>模型名（如 deepseek-chat / gpt-4o-mini）</label><input v-model="data.model" class="apple-input" /></div>
        <div class="ai-settings-field"><label>Embedding 模型（启用知识库时必填）</label><input v-model="data.embedding_model" class="apple-input" placeholder="如 text-embedding-3-small" /></div>
        <div class="ai-settings-subsection"><h3>文章摘要</h3><div class="ai-settings-switch"><div><strong>启用 AI 文章摘要</strong><p>前台展示 AI 摘要，关闭后文章页不渲染摘要块。</p></div><VSwitch v-model="data.summary_enabled" /></div><div class="ai-settings-switch"><div><strong>允许读者临时接入自己的 API</strong><p>读者可使用自己的模型生成摘要，Key 不落本站。</p></div><VSwitch v-model="data.allow_user_api" /></div></div>
        <div class="ai-settings-subsection"><h3>RAG 知识库问答</h3><p class="ai-settings-hint">开启 Embedding 模型后，可在「知识库」中构建公开文章知识库；前台问答默认关闭。</p><div class="ai-settings-switch"><div><strong>开启前台 AI 问答</strong><p>仅使用已发布公开文章和页面。</p></div><VSwitch v-model="data.qa_enabled" /></div><div class="ai-settings-switch"><div><strong>允许读者使用自己的 AI 模型生成最终回答</strong><p>Embedding 仍使用本站配置。</p></div><VSwitch v-model="data.qa_allow_user_llm" /></div></div>
        <div class="ai-settings-actions"><VButton type="primary" :loading="saving" @click="save"><template #icon><Icon icon="ri:save-line" /></template>保存 AI 设置</VButton></div>
      </template>
    </VCard>
  </div>
</template>

<style scoped>
.ai-settings-page { width: 100%; max-width: 900px; margin: 0 auto; }
.ai-settings-hint { margin: 0 0 14px; color: var(--color-text-tertiary); font-size: 12px; line-height: 1.5; }
.ai-settings-field { margin-bottom: 16px; }
.ai-settings-field label { display: block; margin-bottom: 6px; color: var(--color-text-tertiary); font-size: 12px; }
.ai-settings-subsection { margin-top: 24px; }
.ai-settings-subsection h3 { margin: 0 0 12px; color: var(--color-text-secondary); font-size: 13px; }
.ai-settings-switch { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 12px 0; border-bottom: 1px solid var(--color-border); }
.ai-settings-switch strong { color: var(--color-foreground); font-size: 13px; font-weight: 500; }
.ai-settings-switch p { margin: 4px 0 0; color: var(--color-text-tertiary); font-size: 12px; }
.ai-settings-actions { display: flex; justify-content: flex-end; margin-top: 24px; }
.ai-settings-loading { display: flex; align-items: center; gap: 8px; padding: 32px 0; color: var(--color-text-tertiary); font-size: 13px; }
.ai-settings-spin { animation: ai-settings-spin 1s linear infinite; }
@keyframes ai-settings-spin { to { transform: rotate(360deg); } }
</style>
