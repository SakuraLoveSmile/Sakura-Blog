<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { onBeforeRouteLeave } from "vue-router";
import { VButton, Toast } from "@/components/ui";
import { settingsApi, getApiErrorMessage } from "@/api";

// 样式 YAML 编辑器：直接编辑 site.yml style 段（对齐 Stellar _config.yml style 面），
// 保存经后端 yaml 解析校验 + SaveSection(style) + RebuildSite 热更新前台。
const yamlText = ref("");
const savedYaml = ref("");
const loading = ref(true);
const saving = ref(false);
const loadError = ref("");

const isDirty = computed(() => !loading.value && yamlText.value !== savedYaml.value);

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data } = await settingsApi.getStyleYAML();
    yamlText.value = data.yaml || "";
    savedYaml.value = yamlText.value;
  } catch (error) {
    loadError.value = getApiErrorMessage(error, "加载样式 YAML 失败");
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (saving.value) return;
  saving.value = true;
  const currentPayload = yamlText.value;
  try {
    await settingsApi.updateStyleYAML(currentPayload);
    // 仅当保存成功且当前内容仍等于已保存内容时更新快照
    if (yamlText.value === currentPayload) {
      savedYaml.value = currentPayload;
    }
    Toast.success("样式 YAML 已保存，前台已热更新");
  } catch (error) {
    // 失败保留用户输入，提供明确错误明细
    Toast.error(getApiErrorMessage(error, "保存样式 YAML 失败，请检查语法或网络"));
  } finally {
    saving.value = false;
  }
}

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (isDirty.value) {
    e.preventDefault();
    e.returnValue = "";
  }
}

onMounted(() => {
  void load();
  window.addEventListener("beforeunload", handleBeforeUnload);
});

onBeforeUnmount(() => {
  window.removeEventListener("beforeunload", handleBeforeUnload);
});

onBeforeRouteLeave((to, from, next) => {
  if (to.path === from.path) {
    next();
    return;
  }
  if (isDirty.value) {
    const confirmed = window.confirm("样式 YAML 有未保存修改，离开后将丢失，是否确认离开？");
    if (confirmed) {
      next();
    } else {
      next(false);
    }
    return;
  }
  next();
});
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <p class="text-xs text-text-quaternary">
        直接编辑 Stellar 主题 <code>style</code> 段
        YAML（颜色/字号/圆角/渐变/左栏与全站背景/分页/标题前缀等），保存即热更新前台样式。键名对齐上游
        <code>_config.yml</code>。
      </p>
      <span v-if="isDirty" class="text-xs text-state-warning whitespace-nowrap">（有未保存修改）</span>
    </div>
    <div v-if="loading" class="text-xs text-text-tertiary">正在加载样式 YAML...</div>
    <div v-else-if="loadError" class="text-sm text-state-error flex items-center gap-2">
      <span>{{ loadError }}</span>
      <VButton size="sm" type="secondary" @click="load">重试</VButton>
    </div>
    <textarea
      v-else
      v-model="yamlText"
      :disabled="loading || saving"
      spellcheck="false"
      class="apple-textarea code-style w-full font-mono text-xs"
      style="min-height: 420px"
      placeholder="prefers_theme: auto&#10;color:&#10;  theme: 'hsl(192 98% 55%)'"
    ></textarea>
    <div>
      <VButton type="primary" :disabled="loading || saving" @click="save">
        {{ saving ? "保存中..." : "保存样式 YAML" }}
      </VButton>
    </div>
  </div>
</template>
