<script setup lang="ts">
import { ref } from "vue";
import { Icon } from "@iconify/vue";
import { VButton, VCard, VSwitch } from "@/components/ui";
import { ALL_SETTINGS_SECTIONS } from "../settings-state";

const props = defineProps<{ data: Record<string, any>; rawText: Record<string, string>; saving: string }>();
const emit = defineEmits<{ save: [section: string]; saveJson: [section: string] }>();
const codeAreaCls = "apple-textarea code-style";
const rawSection = ref<string>("site");
</script>

<template>
  <VCard title="高级设置">
    <section>
      <h3 class="settings-subsection-title">代码注入</h3>
      <div class="settings-warning"><Icon icon="ri:error-warning-line" width="14" height="14" /><span>注入内容将不经转义原样输出到前台所有页面，可执行任意脚本。请只粘贴你完全信任的代码，错误或恶意代码可能导致页面故障与安全风险。</span></div>
      <div class="settings-field"><label>头部注入（输出在 &lt;/head&gt; 之前）</label><textarea v-model="data.inject.inject_head" rows="6" :class="codeAreaCls" placeholder="<script>...</script> 或 <link ...>"></textarea></div>
      <div class="settings-field"><label>页脚注入（输出在 &lt;/body&gt; 之前）</label><textarea v-model="data.inject.inject_footer" rows="6" :class="codeAreaCls" placeholder="<script>...</script>"></textarea></div>
      <div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'inject'" @click="emit('save', 'inject')"><template #icon><Icon icon="ri:save-line" /></template>保存代码注入</VButton></div>
    </section>

    <section class="settings-subsection"><h3 class="settings-subsection-title">数据缓存</h3><p class="settings-hint">友链、时间线、GitHub 信息等动态数据的浏览器本地缓存。按服务覆盖缓存时长（ttl）请通过下方原始 JSON 编辑。</p><div class="settings-switch-row"><div><div class="settings-switch-name">启用本地缓存</div><div class="settings-switch-desc">关闭后每次访问都重新拉取动态数据（TTL=0）</div></div><VSwitch v-model="data.data_cache.enable" /></div><div class="settings-form-grid settings-form-grid-2"><div class="settings-field"><label>默认缓存时长（秒）</label><input v-model.number="data.data_cache.default_ttl" type="number" min="0" class="apple-input" placeholder="3600" /></div></div><div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'data_cache'" @click="emit('save', 'data_cache')"><template #icon><Icon icon="ri:save-line" /></template>保存数据缓存设置</VButton></div></section>

    <details class="settings-json-block"><summary>原始 JSON 配置（高级兜底）</summary><p class="settings-hint">结构化表单未覆盖的字段可在此编辑；默认折叠，保存前请确认 JSON 格式正确。</p><div class="settings-field"><label>配置段</label><select v-model="rawSection" class="apple-select"><option v-for="section in ALL_SETTINGS_SECTIONS" :key="section" :value="section">{{ section }}</option></select></div><textarea v-model="rawText[rawSection]" rows="12" :class="codeAreaCls"></textarea><div class="settings-card-actions"><VButton type="secondary" size="sm" :loading="saving === 'json:' + rawSection" @click="emit('saveJson', rawSection)">保存 {{ rawSection }} JSON</VButton></div></details>
  </VCard>
</template>
