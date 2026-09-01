<script setup lang="ts">
import { ref } from "vue";
import { Icon } from "@iconify/vue";
import { VButton, VSwitch } from "@/components/ui";
import { ALL_SETTINGS_SECTIONS } from "../settings-state";

defineProps<{ data: Record<string, any>; rawText: Record<string, string>; saving: string }>();
const emit = defineEmits<{ save: [section: string]; saveJson: [section: string] }>();
const codeAreaCls = "apple-textarea code-style";
const rawSection = ref<string>("site");
const cacheServices = [
  ["giscus", "Giscus 最新评论"],
  ["waline", "Waline 最新评论"],
  ["artalk", "Artalk 最新评论"],
  ["memos", "Memos"],
  ["memos-user", "Memos 用户"],
  ["sites", "站点卡片"],
  ["friends", "友链"],
  ["friends_and_posts", "友链文章"],
  ["siteinfo", "站点信息"],
] as const;
</script>

<template>
  <div class="settings-advanced-form">
    <section class="settings-section settings-section-first">
      <div class="settings-section-heading">
        <h2>代码注入</h2>
        <p>在前台所有页面的关键生命周期注入自定义脚本、样式或统计代码。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-warning">
          <Icon icon="ri:error-warning-line" class="settings-warning-icon" />
          <span
            >注入内容将不经转义原样输出到前台所有页面，可执行任意脚本。请只粘贴你完全信任的代码，错误或恶意代码可能导致页面故障与安全风险。</span
          >
        </div>

        <div class="settings-field">
          <label for="settings-inject-head">头部注入（输出在 &lt;/head&gt; 之前）</label>
          <textarea
            id="settings-inject-head"
            v-model="data.inject.inject_head"
            rows="5"
            :class="codeAreaCls"
            placeholder='<script>...</script> 或 <link rel="stylesheet" href="...">'
          ></textarea>
        </div>

        <div class="settings-field">
          <label for="settings-inject-footer">页脚注入（输出在 &lt;/body&gt; 之前）</label>
          <textarea
            id="settings-inject-footer"
            v-model="data.inject.inject_footer"
            rows="5"
            :class="codeAreaCls"
            placeholder="<script>...</script>"
          ></textarea>
        </div>

        <div class="settings-section-inline-action">
          <VButton type="secondary" size="sm" :loading="saving === 'inject'" @click="emit('save', 'inject')">
            <template #icon><Icon icon="ri:save-line" /></template>
            保存代码注入
          </VButton>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>数据缓存</h2>
        <p>友链、站点卡片和其他动态数据的浏览器本地缓存策略；TTL 设为 0 表示对应服务不缓存。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">启用本地数据缓存</div>
            <p class="settings-help">关闭后访客每次访问都重新向后端拉取动态数据（TTL=0）。</p>
          </div>
          <VSwitch v-model="data.data_cache.enable" aria-label="启用本地数据缓存" />
        </div>

        <div class="settings-field">
          <label for="settings-cache-ttl">默认缓存时长（秒）</label>
          <input
            id="settings-cache-ttl"
            v-model.number="data.data_cache.default_ttl"
            type="number"
            min="0"
            class="apple-input"
            placeholder="3600"
          />
        </div>

        <div class="settings-field">
          <label for="settings-cache-max-entries">缓存条目上限</label>
          <input
            id="settings-cache-max-entries"
            v-model.number="data.data_cache.max_entries"
            type="number"
            min="1"
            step="1"
            class="apple-input"
            placeholder="200"
          />
          <p class="settings-help">仅淘汰以 <code>dc:</code> 开头的动态数据缓存，按写入时间移除最旧条目。</p>
        </div>

        <div class="settings-subsection">
          <p class="settings-subsection-title">服务级缓存时长（秒）</p>
          <div class="settings-form-grid settings-form-grid-3">
            <div v-for="[name, label] in cacheServices" :key="name" class="settings-field">
              <label :for="`settings-cache-${name}`">{{ label }}（{{ name }}）</label>
              <input
                :id="`settings-cache-${name}`"
                v-model.number="data.data_cache.ttl[name]"
                type="number"
                min="0"
                step="1"
                class="apple-input"
                placeholder="沿用默认"
              />
            </div>
          </div>
        </div>

        <div class="settings-section-inline-action">
          <VButton type="secondary" size="sm" :loading="saving === 'data_cache'" @click="emit('save', 'data_cache')">
            <template #icon><Icon icon="ri:save-line" /></template>
            保存数据缓存设置
          </VButton>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>原始 JSON 配置（高级兜底）</h2>
        <p>结构化表单未覆盖的底层高级字段可在此直接编辑并保存；保存前请确认 JSON 语法格式正确。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-field">
          <label for="settings-raw-section">配置段选择</label>
          <select id="settings-raw-section" v-model="rawSection" class="apple-select">
            <option v-for="section in ALL_SETTINGS_SECTIONS" :key="section" :value="section">{{ section }}</option>
          </select>
        </div>

        <div class="settings-field">
          <label for="settings-raw-content">{{ rawSection }} JSON 内容</label>
          <textarea id="settings-raw-content" v-model="rawText[rawSection]" rows="12" :class="codeAreaCls"></textarea>
        </div>

        <div class="settings-section-inline-action">
          <VButton
            type="secondary"
            size="sm"
            :loading="saving === 'json:' + rawSection"
            @click="emit('saveJson', rawSection)"
          >
            <template #icon><Icon icon="ri:save-line" /></template>
            保存 {{ rawSection }} JSON
          </VButton>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.settings-advanced-form {
  background: var(--color-card);
}

.settings-section {
  margin: 0;
  padding: 28px;
  border-bottom: 1px solid var(--color-border);
}

.settings-section-first {
  padding-top: 28px;
}

.settings-section:last-of-type {
  border-bottom: 0;
  padding-bottom: 36px;
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

/* 方案 1：内嵌分组面板 */
.settings-section-body {
  max-width: 860px;
  padding: 22px 24px;
  background: color-mix(in srgb, var(--color-background) 38%, var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
}

.settings-warning {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  max-width: 860px;
  margin: 0 0 20px;
  padding: 14px 16px;
  border: 1px solid color-mix(in srgb, var(--color-state-warning) 30%, transparent);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--color-state-warning) 10%, transparent);
  color: var(--color-state-warning);
  font-size: 12.5px;
  line-height: 1.6;
}

.settings-warning-icon {
  flex-shrink: 0;
  font-size: 16px;
  margin-top: 2px;
}

.settings-field,
.settings-switch {
  max-width: 860px;
  margin: 0 0 20px;
  padding: 0 0 20px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
}

.settings-section-body > :last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: 0;
}

.settings-field > label {
  display: block;
  margin-bottom: 10px;
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
}

.settings-field > .apple-input,
.settings-field > .apple-select,
.settings-field > .apple-textarea {
  display: block;
  width: min(100%, 860px);
  max-width: 860px;
}

.settings-switch {
  display: flex;
  min-height: 44px;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.settings-switch-copy {
  min-width: 0;
}

.settings-switch-name {
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
  line-height: 1.4;
}

.settings-switch .settings-help {
  margin-top: 6px;
}

.settings-switch :deep(button[role="switch"]) {
  flex: 0 0 auto;
  margin-top: 2px;
}

.settings-section-inline-action {
  display: flex;
  align-items: center;
  padding-top: 4px;
}

@media (max-width: 640px) {
  .settings-section {
    padding: 24px 16px;
  }

  .settings-section-body {
    padding: 18px 16px;
  }

  .settings-switch {
    gap: 12px;
  }
}
</style>
