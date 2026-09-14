<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VButton, VSwitch } from "@/components/ui";

defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
const textareaCls = "apple-textarea";

function handleSubmit() {
  emit('save', 'seo');
}
</script>

<template>
  <form class="settings-seo-form" @submit.prevent="handleSubmit">
    <section class="settings-section settings-section-first">
      <div class="settings-section-heading">
        <h2>站点元信息</h2>
        <p>设置站点的全局描述与访问 URL，影响搜索引擎收录与分享卡片基准。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-field">
          <label for="settings-site-desc">站点描述</label>
          <textarea
            id="settings-site-desc"
            v-model="data.site.description"
            rows="2"
            :class="textareaCls"
            placeholder="用于首页和未单独设置描述的页面"
          ></textarea>
        </div>

        <div class="settings-field">
          <label for="settings-site-url">站点 URL</label>
          <input id="settings-site-url" v-model="data.site.url" :class="inputCls" placeholder="https://example.com" />
        </div>

        <div class="settings-section-inline-action">
          <VButton type="secondary" size="sm" :disabled="Boolean(saving)" :loading="saving === 'site'" @click="emit('save', 'site')">
            <template #icon><Icon icon="ri:save-line" /></template>
            保存站点元信息
          </VButton>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>社交卡片与标准域名</h2>
        <p>规范标准链接（Canonical）、Open Graph 协议与 Twitter 社交卡片。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">启用 Open Graph 协议</div>
            <p class="settings-help">输出 og:title、og:description、og:image 与 Twitter 站点卡片标签。</p>
          </div>
          <VSwitch v-model="data.seo.open_graph.enable" aria-label="启用 Open Graph" />
        </div>

        <div class="settings-form-grid settings-form-grid-2">
          <div class="settings-field">
            <label for="settings-seo-original-host">源站主机（originalHost）</label>
            <input id="settings-seo-original-host" v-model="data.seo.canonical.originalHost" :class="inputCls" placeholder="example.com" />
          </div>

          <div class="settings-field">
            <label for="settings-seo-twitter-id">Twitter ID</label>
            <input id="settings-seo-twitter-id" v-model="data.seo.open_graph.twitter_id" :class="inputCls" placeholder="username（不含 @）" />
          </div>
        </div>

        <div class="settings-field">
          <label for="settings-seo-official-hosts">官方主机列表（每行一个）</label>
          <textarea
            id="settings-seo-official-hosts"
            :value="data.seo.canonical.officialHosts.join('\n')"
            rows="3"
            :class="textareaCls"
            placeholder="localhost&#10;example.com"
            @input="
              data.seo.canonical.officialHosts = ($event.target as HTMLTextAreaElement).value
                .split(/\r?\n/)
                .map((s: string) => s.trim())
                .filter(Boolean)
            "
          ></textarea>
        </div>

        <div class="settings-field">
          <label for="settings-seo-preconnect">预连接域名（每行一个）</label>
          <textarea
            id="settings-seo-preconnect"
            :value="data.seo.preconnect.join('\n')"
            rows="3"
            :class="textareaCls"
            placeholder="https://cdn.jsdelivr.net"
            @input="
              data.seo.preconnect = ($event.target as HTMLTextAreaElement).value
                .split(/\r?\n/)
                .map((s: string) => s.trim())
                .filter(Boolean)
            "
          ></textarea>
        </div>

        <div class="settings-field">
          <label for="settings-seo-structured-links">结构化数据社交链接（每行一个）</label>
          <textarea
            id="settings-seo-structured-links"
            :value="data.seo.structured_data.links.join('\n')"
            rows="3"
            :class="textareaCls"
            placeholder="https://github.com/username"
            @input="
              data.seo.structured_data.links = ($event.target as HTMLTextAreaElement).value
                .split(/\r?\n/)
                .map((s: string) => s.trim())
                .filter(Boolean)
            "
          ></textarea>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>搜索引擎与爬虫规则</h2>
        <p>控制全局 Meta 描述、关键字、爬虫抓取规则与站点收录权限。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">禁止搜索引擎收录（noindex）</div>
            <p class="settings-help">全站输出 &lt;meta name="robots" content="noindex"&gt;，请谨慎开启。</p>
          </div>
          <VSwitch v-model="data.seo.no_index" aria-label="禁止搜索引擎收录" />
        </div>

        <div class="settings-field">
          <label for="settings-seo-default-desc">默认 Meta Description（文章未单独设置时使用）</label>
          <textarea id="settings-seo-default-desc" v-model="data.seo.default_description" rows="2" :class="textareaCls"></textarea>
        </div>

        <div class="settings-field">
          <label for="settings-seo-default-keywords">默认 Meta Keywords（逗号分隔）</label>
          <input id="settings-seo-default-keywords" v-model="data.seo.default_keywords" :class="inputCls" placeholder="博客, Go, 前端" />
        </div>

        <div class="settings-field">
          <label for="settings-seo-robots-extra">robots.txt 附加规则（原样追加到默认规则之后）</label>
          <textarea
            id="settings-seo-robots-extra"
            v-model="data.seo.robots_extra"
            rows="4"
            :class="textareaCls"
            placeholder="User-agent: GPTBot&#10;Disallow: /"
          ></textarea>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>站点所有权验证</h2>
        <p>填入主流搜索引擎站长平台的验证代码（留空则不输出对应 Meta 标签）。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-form-grid settings-form-grid-3">
          <div class="settings-field">
            <label for="settings-seo-google">Google 验证码</label>
            <input id="settings-seo-google" v-model="data.seo.google_verification" :class="inputCls" placeholder="验证代码" />
          </div>

          <div class="settings-field">
            <label for="settings-seo-bing">Bing 验证码</label>
            <input id="settings-seo-bing" v-model="data.seo.bing_verification" :class="inputCls" placeholder="验证代码" />
          </div>

          <div class="settings-field">
            <label for="settings-seo-baidu">百度验证码</label>
            <input id="settings-seo-baidu" v-model="data.seo.baidu_verification" :class="inputCls" placeholder="验证代码" />
          </div>
        </div>
      </div>
    </section>

    <div class="settings-seo-actions">
      <div class="settings-seo-actions-inner">
        <VButton type="primary" size="md" native-type="submit" :disabled="Boolean(saving)" :loading="saving === 'seo'">
          <template #icon><Icon icon="ri:save-line" /></template>
          保存 SEO 设置
        </VButton>
      </div>
    </div>
  </form>
</template>

<style scoped>
.settings-seo-form {
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

.settings-form-grid-3 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
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

/* ================= 操作底栏 ================= */
.settings-seo-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  padding: 16px 28px calc(16px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-card) 92%, transparent);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.settings-seo-actions-inner {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

@media (max-width: 640px) {
  .settings-section {
    padding: 24px 16px;
  }

  .settings-form-grid-2,
  .settings-form-grid-3 {
    grid-template-columns: 1fr;
  }

  .settings-switch {
    gap: 12px;
  }

  .settings-seo-actions {
    padding: 14px 16px calc(14px + env(safe-area-inset-bottom));
  }
}
</style>
