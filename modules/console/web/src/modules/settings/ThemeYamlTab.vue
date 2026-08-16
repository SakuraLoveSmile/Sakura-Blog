<script setup lang="ts">
import { ref, onMounted } from "vue";
import { VButton, Toast } from "@/components/ui";
import { settingsApi } from "@/api";

// Spec 60 主题 YAML 编辑器：直接编辑 site.yml style 段（对齐 Stellar _config.yml style 面），
// 保存经后端 yaml 解析校验 + SaveSection(style) + RebuildSite 热更新前台。
// 与结构化 AppearanceTab / 高级·JSON 同源（style 段）。
const yamlText = ref("");
const loading = ref(true);
const loadError = ref("");

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data } = await settingsApi.getStyleYAML();
    yamlText.value = data.yaml || "";
  } catch (e: any) {
    loadError.value = e?.response?.data?.error || e?.message || "加载主题 YAML 失败";
  } finally {
    loading.value = false;
  }
}

async function save() {
  try {
    await settingsApi.updateStyleYAML(yamlText.value);
    Toast.success("主题已保存，前台已热更新");
    await load();
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "保存失败（检查 YAML 格式）");
  }
}

onMounted(load);
</script>

<template>
  <div class="space-y-4">
    <p class="text-xs text-text-quaternary">
      直接编辑 Stellar 主题 <code>style</code> 段 YAML（颜色/字号/圆角/渐变/左栏与全站背景/分页/标题前缀等），保存即热更新前台样式。键名对齐上游 <code>_config.yml</code>。
    </p>
    <div v-if="loadError" class="text-sm text-state-error">{{ loadError }}</div>
    <textarea
      v-else
      v-model="yamlText"
      :disabled="loading"
      spellcheck="false"
      class="apple-textarea code-style w-full font-mono text-xs"
      style="min-height: 420px"
      placeholder="prefers_theme: auto&#10;color:&#10;  theme: 'hsl(192 98% 55%)'"
    ></textarea>
    <div>
      <VButton type="primary" :disabled="loading" @click="save">保存主题 YAML</VButton>
    </div>
  </div>
</template>
