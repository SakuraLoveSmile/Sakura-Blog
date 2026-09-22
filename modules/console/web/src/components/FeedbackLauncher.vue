<script setup lang="ts">
// Feedback 组件的 Console 侧挂载点（feedback.display: console|both）。
// 复用 Stellar 前台同一份 <feedback-widget> Custom Element：
// ESM 入口与前台一致 /assets/vendor/feedback/feedback-web.js（同源静态资源），
// 用 <script type="module"> 注入而非 import()——构建期不打包，避免 SW/路径差异。
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { settingsApi } from "@/api";

const WIDGET_SRC = "/assets/vendor/feedback/feedback-web.js";

const route = useRoute();
const cfg = ref<Record<string, any> | null>(null);
const scriptReady = ref(false);

// 前端缺省回落与 internal/settings/model.go 的 Effective* 一致
const showOnConsole = computed(() => {
  const f = cfg.value;
  if (!f || !f.enabled || !f.api_base || !f.app_id) return false;
  const d = f.display || "front";
  return d === "console" || d === "both";
});
const apiBase = computed(() => String(cfg.value?.api_base || "").trim());
const appId = computed(() => String(cfg.value?.app_id || "").trim());
const appName = computed(() => String(cfg.value?.app_name || "").trim());
const appVersion = computed(() => String(cfg.value?.app_version || "").trim());
const side = computed(() => (cfg.value?.side === "left" ? "left" : "right"));
const launcherMode = computed(() => (cfg.value?.launcher_mode === "tab" ? "tab" : "orb"));
const captureMode = computed(() => (cfg.value?.capture_mode === "off" ? "off" : "viewport"));
const launcherBottom = computed(() => String(cfg.value?.launcher_bottom || "").trim() || "25%");
const pageLabel = computed(() => `console:${route.path}`);

function ensureScript() {
  if (scriptReady.value) return;
  if (customElements.get("feedback-widget")) {
    scriptReady.value = true;
    return;
  }
  const script = document.createElement("script");
  script.type = "module";
  script.src = WIDGET_SRC;
  script.onload = () => {
    scriptReady.value = true;
  };
  document.head.appendChild(script);
}

onMounted(async () => {
  try {
    const { data } = await settingsApi.all();
    cfg.value = (data as any)?.feedback ?? null;
    if (showOnConsole.value) ensureScript();
  } catch {
    // 设置读取失败静默降级：反馈入口不是后台核心功能
  }
});
</script>

<template>
  <feedback-widget
    v-if="showOnConsole && scriptReady"
    :api-base="apiBase"
    :app-id="appId"
    :app-name="appName || undefined"
    :app-version="appVersion || undefined"
    :page-label="pageLabel"
    :side="side"
    theme="dark"
    :launcher-mode="launcherMode"
    :capture-mode="captureMode"
    :launcher-bottom="launcherBottom"
  />
</template>
