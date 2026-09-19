<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VButton, VSwitch } from "@/components/ui";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
const textareaCls = "apple-textarea";

function updateSkipSearch(event: Event) {
  props.data.search.local_search.skip_search = (event.target as HTMLTextAreaElement).value
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function handleSubmit() {
  emit("save", "search");
}
</script>

<template>
  <form class="settings-search-form" @submit.prevent="handleSubmit">
    <section class="settings-section settings-section-first">
      <div class="settings-section-heading">
        <h2>本地搜索引擎</h2>
        <p>控制本地索引生成范围、索引路径与正文检索能力；修改后会立即使本地索引失效并重建。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">搜索正文内容</div>
            <p class="settings-help">
              开启后将文章完整正文写入索引；关闭后仅索引标题，减小 search.json 体积并提升检索响应速度。
            </p>
          </div>
          <VSwitch v-model="data.search.local_search.content" aria-label="搜索正文内容" />
        </div>

        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">首次聚焦再加载</div>
            <p class="settings-help">
              开启后仅在用户第一次聚焦搜索框时加载索引，减少页面初始请求；关闭后页面加载时立即读取索引。
            </p>
          </div>
          <VSwitch v-model="data.search.local_search.lazy_load" aria-label="首次聚焦再加载" />
        </div>

        <div class="settings-form-grid settings-form-grid-2">
          <div class="settings-field">
            <label for="settings-search-service">搜索服务</label>
            <select id="settings-search-service" v-model="data.search.service" class="apple-select">
              <option value="local_search">本地静态索引 (local_search)</option>
              <option value="todo">待扩展 (todo)</option>
            </select>
          </div>

          <div class="settings-field">
            <label for="settings-search-field">索引范围</label>
            <select id="settings-search-field" v-model="data.search.local_search.field" class="apple-select">
              <option value="all">全部内容 (all)</option>
              <option value="post">仅文章 (post)</option>
              <option value="page">仅独立页面 (page)</option>
            </select>
          </div>
        </div>

        <div class="settings-field">
          <label for="settings-search-path">索引文件生成路径</label>
          <input
            id="settings-search-path"
            v-model="data.search.local_search.path"
            :class="inputCls"
            placeholder="/search.json"
          />
        </div>

        <div class="settings-field">
          <label for="settings-search-cache-ttl">搜索缓存时长（秒）</label>
          <input
            id="settings-search-cache-ttl"
            v-model.number="data.search.local_search.cache_ttl"
            type="number"
            min="0"
            :class="inputCls"
            placeholder="86400（0=不缓存）"
          />
          <p class="settings-help">浏览器本地搜索索引的缓存时长；设为 0 可关闭搜索缓存。</p>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>排除路径</h2>
        <p>配置不参与搜索索引生成的路径黑名单（每行一个路径）。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-field">
          <label for="settings-search-skip">排除路径列表</label>
          <textarea
            id="settings-search-skip"
            :value="data.search.local_search.skip_search.join('\n')"
            rows="4"
            :class="textareaCls"
            placeholder="/post/private&#10;/draft"
            @input="updateSkipSearch"
          ></textarea>
        </div>
      </div>
    </section>

    <div class="settings-search-actions">
      <div class="settings-search-actions-inner">
        <VButton type="primary" size="md" native-type="submit" :disabled="Boolean(saving)" :loading="saving === 'search'">
          <template #icon><Icon icon="ri:save-line" /></template>
          保存搜索设置
        </VButton>
      </div>
    </div>
  </form>
</template>

<style scoped>
.settings-search-form {
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

/* 内嵌分组面板：消除多余嵌套卡片背景、边框、圆角与阴影，由外层 VCard 统一提供卡片边界 */
.settings-section-body {
  max-width: 860px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 0;
  box-shadow: none;
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

.settings-form-grid {
  display: grid;
  gap: 16px;
  margin: 0 0 20px;
  padding: 0 0 20px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
}

.settings-form-grid .settings-field {
  margin: 0;
  padding: 0;
  border-bottom: 0;
}

.settings-form-grid-2 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
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

/* ================= 操作底栏 ================= */
.settings-search-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  padding: 16px 28px calc(16px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-card) 92%, transparent);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.settings-search-actions-inner {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

@media (max-width: 767px) {
  .settings-section {
    padding-left: 16px;
    padding-right: 16px;
  }
}

@media (max-width: 640px) {
  .settings-section {
    padding-top: 24px;
    padding-bottom: 24px;
  }

  .settings-form-grid-2 {
    grid-template-columns: 1fr;
  }

  .settings-switch {
    gap: 12px;
  }

  .settings-search-actions {
    padding: 14px 16px calc(14px + env(safe-area-inset-bottom));
  }
}
</style>
