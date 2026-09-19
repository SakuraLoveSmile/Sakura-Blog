<script setup lang="ts">
import { computed, ref } from "vue";
import { Icon } from "@iconify/vue";
import { VButton, VSwitch } from "@/components/ui";
import { getApiErrorMessage, settingsApi } from "@/api";
import type { FeedbackCheckResponse } from "@/api";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";

// ---- 连接自测：提交草稿 api_base，后端探测其 /healthz ----
const checking = ref(false);
const checkResult = ref<FeedbackCheckResponse | null>(null);
// 草稿变化后旧结果即失效，避免把上一个地址的结论挂到新输入上
const checkedBase = ref("");

const apiBaseValid = computed(() => /^https?:\/\/\S+$/.test(props.data.feedback.api_base.trim()));
const checkStale = computed(
  () => checkResult.value !== null && checkedBase.value !== props.data.feedback.api_base.trim(),
);

async function checkConnection() {
  const base = props.data.feedback.api_base.trim();
  if (checking.value || !apiBaseValid.value) return;
  checking.value = true;
  try {
    const { data: resp } = await settingsApi.checkFeedback(base);
    checkResult.value = resp;
    checkedBase.value = base;
  } catch (error) {
    checkResult.value = { ok: false, status: 0, latency_ms: 0, endpoint: "", error: getApiErrorMessage(error, "请求失败") };
    checkedBase.value = base;
  } finally {
    checking.value = false;
  }
}

// ---- 入口位置示意：CSS-only 迷你视口，随 side/launcher_bottom/launcher_mode 实时变化 ----
const normalizedBottom = computed(() => {
  const raw = String(props.data.feedback.launcher_bottom || "").trim();
  const m = raw.match(/^(\d+(?:\.\d+)?)%$/);
  if (m) return `${Math.min(90, Math.max(0, Number(m[1])))}%`;
  const px = raw.match(/^(\d+(?:\.\d+)?)px$/);
  if (px) return `${Math.min(72, Math.max(0, Number(px[1]) / 4))}px`;
  return "25%";
});

function handleSubmit() {
  emit("save", "feedback");
}
</script>

<template>
  <form class="settings-feedback-form" @submit.prevent="handleSubmit">
    <section class="settings-section settings-section-first">
      <div class="settings-section-heading">
        <h2>Feedback 反馈组件</h2>
        <p>
          接入自建 Feedback 服务的 <code>&lt;feedback-widget&gt;</code>：前台页面悬浮入口，支持截图与文字反馈。
          启用后需同时填写服务地址与软件标识才会渲染组件；提交成功仅表示服务已接收，归档进度由 Feedback
          管理页处理。
        </p>
      </div>
      <div class="settings-section-body">
        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">启用反馈组件</div>
            <p class="settings-help">在前台页面显示反馈入口（默认右侧灵感球）。</p>
          </div>
          <VSwitch v-model="data.feedback.enabled" aria-label="启用反馈组件" />
        </div>

        <div class="settings-form-grid settings-form-grid-2">
          <div class="settings-field">
            <label for="settings-feedback-api-base">服务地址（api_base）</label>
            <input
              id="settings-feedback-api-base"
              v-model="data.feedback.api_base"
              :class="inputCls"
              placeholder="https://feedback.example.com"
            />
          </div>
          <div class="settings-field">
            <label for="settings-feedback-app-id">软件标识（app_id）</label>
            <input
              id="settings-feedback-app-id"
              v-model="data.feedback.app_id"
              :class="inputCls"
              placeholder="com.example.blog"
            />
          </div>
          <div class="settings-field">
            <label for="settings-feedback-app-name">显示名（可选）</label>
            <input
              id="settings-feedback-app-name"
              v-model="data.feedback.app_name"
              :class="inputCls"
              placeholder="后台未配置时使用的名称"
            />
          </div>
          <div class="settings-field">
            <label for="settings-feedback-app-version">版本号（可选）</label>
            <input
              id="settings-feedback-app-version"
              v-model="data.feedback.app_version"
              :class="inputCls"
              placeholder="随提交上报，如 1.0.0"
            />
          </div>
        </div>

        <div class="settings-field feedback-check-field">
          <div class="feedback-check-row">
            <VButton
              type="secondary"
              size="sm"
              :disabled="!apiBaseValid || checking"
              :loading="checking"
              @click="checkConnection"
            >
              <template #icon><Icon icon="ri:pulse-line" /></template>连接测试
            </VButton>
            <span
              v-if="checkResult && !checkStale"
              class="feedback-check-badge"
              :class="checkResult.ok ? 'is-ok' : 'is-bad'"
              role="status"
            >
              <Icon :icon="checkResult.ok ? 'ri:checkbox-circle-line' : 'ri:error-warning-line'" />
              <template v-if="checkResult.ok">服务可达 · HTTP {{ checkResult.status }} · {{ checkResult.latency_ms }}ms</template>
              <template v-else-if="checkResult.status">服务响应异常 · HTTP {{ checkResult.status }}</template>
              <template v-else>无法连接 · {{ checkResult.error }}</template>
            </span>
            <span v-else-if="checkStale" class="feedback-check-badge is-stale">地址已修改，请重新测试</span>
          </div>
          <p class="settings-help feedback-check-hint">探测 <code>{{ apiBaseValid ? data.feedback.api_base.trim().replace(/\/+$/, "") + "/healthz" : "<api_base>/healthz" }}</code>，仅返回状态码与延迟。</p>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>入口与截图</h2>
        <p>悬浮入口的形态、停靠位置与呼出时的截图行为。</p>
      </div>
      <div class="settings-section-body">
        <div class="feedback-launcher-layout">
          <div class="settings-form-grid settings-form-grid-2 feedback-launcher-fields">
            <div class="settings-field">
              <label for="settings-feedback-launcher-mode">入口形态</label>
              <select id="settings-feedback-launcher-mode" v-model="data.feedback.launcher_mode" class="apple-select">
                <option value="orb">灵感球（可拖拽指出问题位置）</option>
                <option value="tab">贴边标签</option>
              </select>
            </div>
            <div class="settings-field">
              <label for="settings-feedback-capture-mode">呼出时截图</label>
              <select id="settings-feedback-capture-mode" v-model="data.feedback.capture_mode" class="apple-select">
                <option value="viewport">截取当前视口</option>
                <option value="off">不自动截图</option>
              </select>
            </div>
            <div class="settings-field">
              <label for="settings-feedback-side">停靠侧</label>
              <select id="settings-feedback-side" v-model="data.feedback.side" class="apple-select">
                <option value="right">右侧</option>
                <option value="left">左侧</option>
              </select>
            </div>
            <div class="settings-field">
              <label for="settings-feedback-launcher-bottom">距底部位置</label>
              <input
                id="settings-feedback-launcher-bottom"
                v-model="data.feedback.launcher_bottom"
                :class="inputCls"
                placeholder="25%"
              />
            </div>
          </div>

          <div class="feedback-stage" aria-hidden="true">
            <div class="feedback-stage-viewport">
              <span class="feedback-stage-url">{{ data.feedback.enabled ? "前台页面" : "未启用" }}</span>
              <span
                class="feedback-stage-launcher"
                :class="[`is-${data.feedback.side === 'left' ? 'left' : 'right'}`, `is-${data.feedback.launcher_mode === 'tab' ? 'tab' : 'orb'}`]"
                :style="{ bottom: normalizedBottom }"
              ></span>
            </div>
            <p class="feedback-stage-caption">入口位置示意</p>
          </div>
        </div>
        <p class="settings-help">
          面板主题固定深色，与站点一致；截图遮挡区域可在模板中以 <code>data-feedback-capture-mask</code> 标注。
        </p>
      </div>
      <div class="settings-card-actions">
        <VButton type="primary" size="sm" native-type="submit" :disabled="Boolean(saving)" :loading="saving === 'feedback'">
          <template #icon><Icon icon="ri:save-line" /></template>保存反馈组件设置
        </VButton>
      </div>
    </section>
  </form>
</template>

<style scoped>
.settings-section {
  margin: 0;
  padding: 28px;
  border-bottom: 1px solid var(--color-border);
}
.settings-section:last-child {
  border-bottom: 0;
}
.settings-section-first {
  padding-top: 28px;
}
.settings-section-heading {
  max-width: 860px;
  margin-bottom: 18px;
}
.settings-section-heading h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--color-foreground);
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.01em;
}
.settings-section-heading h2::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 16px;
  border-radius: 2px;
  background: var(--color-primary);
  flex-shrink: 0;
}
.settings-section-heading p,
.settings-help {
  margin: 6px 0 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}
.settings-section-body {
  max-width: 860px;
}
.settings-section-body > :last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: 0;
}

.feedback-check-field {
  margin-bottom: 0;
}
.feedback-check-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.feedback-check-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 550;
}
.feedback-check-badge.is-ok {
  color: var(--color-state-success, #30d158);
  background: color-mix(in srgb, var(--color-state-success, #30d158) 14%, transparent);
}
.feedback-check-badge.is-bad {
  color: var(--color-state-error, #ff453a);
  background: color-mix(in srgb, var(--color-state-error, #ff453a) 14%, transparent);
}
.feedback-check-badge.is-stale {
  color: var(--color-text-tertiary);
  background: color-mix(in srgb, var(--color-text-tertiary) 12%, transparent);
}
.feedback-check-hint {
  margin-top: 8px;
}

/* 入口位置示意：迷你视口 + 按 side/launcher_bottom/mode 定位的入口标记 */
.feedback-launcher-layout {
  display: flex;
  align-items: flex-start;
  gap: 24px;
  margin-bottom: 20px;
  padding-bottom: 20px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
}
.feedback-launcher-fields {
  flex: 1;
  min-width: 0;
  margin: 0;
  padding: 0;
  border-bottom: 0;
}
.feedback-stage {
  flex: 0 0 200px;
}
.feedback-stage-viewport {
  position: relative;
  width: 200px;
  height: 128px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg, 12px);
  background:
    linear-gradient(color-mix(in srgb, var(--color-border) 45%, transparent) 1px, transparent 1px) 0 0 / 100% 32px,
    color-mix(in srgb, var(--color-background) 60%, var(--color-card));
  overflow: hidden;
}
.feedback-stage-url {
  position: absolute;
  top: 6px;
  left: 10px;
  color: var(--color-text-tertiary);
  font-size: 10px;
  letter-spacing: 0.02em;
}
.feedback-stage-launcher {
  position: absolute;
  bottom: 25%;
  transition:
    left 250ms cubic-bezier(0.32, 0.72, 0, 1),
    right 250ms cubic-bezier(0.32, 0.72, 0, 1),
    bottom 250ms cubic-bezier(0.32, 0.72, 0, 1),
    border-radius 150ms;
}
.feedback-stage-launcher.is-orb {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--color-primary);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--color-primary) 45%, transparent);
}
.feedback-stage-launcher.is-tab {
  width: 7px;
  height: 30px;
  border-radius: 3px;
  background: var(--color-primary);
}
.feedback-stage-launcher.is-right {
  right: 8px;
}
.feedback-stage-launcher.is-left {
  left: 8px;
}
.feedback-stage-launcher.is-tab.is-right {
  right: 0;
  border-radius: 3px 0 0 3px;
}
.feedback-stage-launcher.is-tab.is-left {
  left: 0;
  border-radius: 0 3px 3px 0;
}
.feedback-stage-caption {
  margin: 8px 0 0;
  color: var(--color-text-tertiary);
  font-size: 11px;
  text-align: center;
}

@media (max-width: 640px) {
  .settings-section {
    padding: 24px 16px;
  }
  .feedback-launcher-layout {
    flex-direction: column;
  }
  .feedback-stage {
    flex-basis: auto;
    align-self: center;
  }
}
</style>
