<script setup lang="ts">
import { computed, ref } from "vue";
import { Icon } from "@iconify/vue";
import { VButton, VSwitch } from "@/components/ui";
import { getApiErrorMessage, settingsApi } from "@/api";
import type { FeedbackCheckResponse } from "@/api";
import { settingsSectionAnchor } from "../settings-pages";

// 访客反馈页：Feedback 服务接入与悬浮入口外观。独立保存（feedback 分段），保存栏由页面通过 actions 插槽提供。
const props = defineProps<{ data: Record<string, any>; saved?: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
const anchor = (id: string) => settingsSectionAnchor("feedback", id);
// 相对上次保存有改动的字段显示橙点；没有快照时不显示。
const changed = (key: string) =>
  props.saved !== undefined && JSON.stringify(props.data.feedback?.[key]) !== JSON.stringify(props.saved[key]);

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
    <section :id="anchor('service')" class="settings-sec">
      <div class="settings-sec-head">
        <div class="settings-sec-title">
          <h2>Feedback 服务接入</h2>
          <p>
            接入自建 Feedback 服务的 <code>&lt;feedback-widget&gt;</code>：前台页面悬浮入口，支持截图与文字反馈。
            启用后需同时填写服务地址与软件标识才会渲染组件；提交成功仅表示服务已接收，归档进度由 Feedback
            管理页处理。
          </p>
        </div>
      </div>
      <div class="settings-rows">
        <div class="settings-row is-switch is-compact">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <span class="settings-row-name">启用反馈组件</span>
              <span v-if="changed('enabled')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">在前台页面显示反馈入口（默认右侧灵感球）。</p>
          </div>
          <div class="settings-row-control">
            <VSwitch v-model="data.feedback.enabled" aria-label="启用反馈组件" />
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-feedback-api-base">服务地址（api_base）</label>
              <span v-if="changed('api_base')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-feedback-api-base"
              v-model="data.feedback.api_base"
              :class="[inputCls, 'settings-control-text']"
              placeholder="https://feedback.example.com"
            />
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-feedback-app-id">软件标识（app_id）</label>
              <span v-if="changed('app_id')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-feedback-app-id"
              v-model="data.feedback.app_id"
              :class="[inputCls, 'settings-control-text']"
              placeholder="com.example.blog"
            />
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-feedback-app-name">显示名（可选）</label>
              <span v-if="changed('app_name')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-feedback-app-name"
              v-model="data.feedback.app_name"
              :class="[inputCls, 'settings-control-text']"
              placeholder="后台未配置时使用的名称"
            />
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-feedback-app-version">版本号（可选）</label>
              <span v-if="changed('app_version')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-feedback-app-version"
              v-model="data.feedback.app_version"
              :class="[inputCls, 'settings-control-text']"
              placeholder="随提交上报，如 1.0.0"
            />
          </div>
        </div>
        <div class="settings-row feedback-check-field">
          <div class="settings-row-label">
            <div class="settings-row-title"><span class="settings-row-name">连接测试</span></div>
            <p class="settings-help feedback-check-hint">
              探测
              <code>{{
                apiBaseValid ? data.feedback.api_base.trim().replace(/\/+$/, "") + "/healthz" : "<api_base>/healthz"
              }}</code
              >，仅返回状态码与延迟。
            </p>
          </div>
          <div class="settings-row-control">
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
            <VButton
              type="secondary"
              size="sm"
              :disabled="!apiBaseValid || checking"
              :loading="checking"
              @click="checkConnection"
            >
              <template #icon><Icon icon="ri:pulse-line" /></template>连接测试
            </VButton>
          </div>
        </div>
      </div>
    </section>

    <section :id="anchor('launcher')" class="settings-sec">
      <div class="settings-sec-head">
        <div class="settings-sec-title">
          <h2>入口与截图</h2>
          <p>悬浮入口的形态、停靠位置与呼出时的截图行为。</p>
        </div>
      </div>
      <div class="feedback-launcher-layout">
        <div class="settings-rows feedback-launcher-fields">
          <div class="settings-row">
            <div class="settings-row-label">
              <div class="settings-row-title">
                <label for="settings-feedback-display">显示范围</label>
                <span v-if="changed('display')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
              </div>
              <p class="settings-help">前台挂在页面右侧悬浮；选 Console 时入口显示在后台右下角。</p>
            </div>
            <div class="settings-row-control">
              <select id="settings-feedback-display" v-model="data.feedback.display" class="apple-select settings-control-select">
                <option value="front">仅博客前台</option>
                <option value="console">仅 Console 后台</option>
                <option value="both">前台 + Console 都显示</option>
              </select>
            </div>
          </div>
          <div class="settings-row">
            <div class="settings-row-label">
              <div class="settings-row-title">
                <label for="settings-feedback-launcher-mode">入口形态</label>
                <span v-if="changed('launcher_mode')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
              </div>
            </div>
            <div class="settings-row-control">
              <select
                id="settings-feedback-launcher-mode"
                v-model="data.feedback.launcher_mode"
                class="apple-select settings-control-select"
              >
                <option value="orb">灵感球（可拖拽指出问题位置）</option>
                <option value="tab">贴边标签</option>
              </select>
            </div>
          </div>
          <div class="settings-row">
            <div class="settings-row-label">
              <div class="settings-row-title">
                <label for="settings-feedback-capture-mode">呼出时截图</label>
                <span v-if="changed('capture_mode')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
              </div>
            </div>
            <div class="settings-row-control">
              <select
                id="settings-feedback-capture-mode"
                v-model="data.feedback.capture_mode"
                class="apple-select settings-control-select"
              >
                <option value="viewport">截取当前视口</option>
                <option value="off">不自动截图</option>
              </select>
            </div>
          </div>
          <div class="settings-row">
            <div class="settings-row-label">
              <div class="settings-row-title">
                <label for="settings-feedback-side">停靠侧</label>
                <span v-if="changed('side')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
              </div>
            </div>
            <div class="settings-row-control">
              <select id="settings-feedback-side" v-model="data.feedback.side" class="apple-select settings-control-select">
                <option value="right">右侧</option>
                <option value="left">左侧</option>
              </select>
            </div>
          </div>
          <div class="settings-row">
            <div class="settings-row-label">
              <div class="settings-row-title">
                <label for="settings-feedback-launcher-bottom">距底部位置</label>
                <span v-if="changed('launcher_bottom')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
              </div>
            </div>
            <div class="settings-row-control">
              <input
                id="settings-feedback-launcher-bottom"
                v-model="data.feedback.launcher_bottom"
                :class="[inputCls, 'settings-control-text']"
                placeholder="25%"
              />
            </div>
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
      <p class="settings-help feedback-panel-note">
        面板主题固定深色，与站点一致；截图遮挡区域可在模板中以 <code>data-feedback-capture-mask</code> 标注。
      </p>
    </section>

    <slot name="actions" />
  </form>
</template>

<style scoped>
.settings-row-name {
  color: var(--color-foreground);
  font-size: 14px;
  font-weight: 500;
}
.feedback-check-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  padding: 0 10px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 500;
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
.feedback-panel-note {
  max-width: 860px;
}

/* 入口位置示意：迷你视口 + 按 side/launcher_bottom/mode 定位的入口标记 */
.feedback-launcher-layout {
  display: flex;
  align-items: flex-start;
  gap: 24px;
  max-width: 860px;
}
.feedback-launcher-fields {
  flex: 1;
  min-width: 0;
}
.feedback-stage {
  flex: 0 0 200px;
  padding-top: 14px;
}
.feedback-stage-viewport {
  position: relative;
  width: 200px;
  height: 128px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg, 8px);
  background: transparent;
  overflow: hidden;
}
.feedback-stage-url {
  position: absolute;
  top: 6px;
  left: 10px;
  color: var(--color-text-tertiary);
  font-size: 11px;
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
}
.feedback-stage-launcher.is-tab {
  width: 7px;
  height: 30px;
  border-radius: var(--radius-sm);
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
  border-radius: var(--radius-sm) 0 0 var(--radius-sm);
}
.feedback-stage-launcher.is-tab.is-left {
  left: 0;
  border-radius: 0 6px 6px 0;
}
.feedback-stage-caption {
  margin: 8px 0 0;
  color: var(--color-text-tertiary);
  font-size: 11px;
  text-align: center;
}

@media (max-width: 767px) {
  .feedback-launcher-layout {
    flex-direction: column;
    gap: 8px;
  }
  .feedback-stage {
    flex-basis: auto;
    align-self: center;
  }
}
</style>
