<script setup lang="ts">
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import IconPicker from "@/components/IconPicker.vue";
import { VButton, VSwitch } from "@/components/ui";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
const selectCls = "apple-select";
const platformOptions = ["wechat", "weibo", "email", "link", "twitter", "telegram"];

function handleSubmit() {
  emit("save", "article");
}

function togglePlatform(platform: string) {
  props.data.article = props.data.article || {};
  props.data.article.share = props.data.article.share || {};
  props.data.article.share.platforms = props.data.article.share.platforms || [];
  const platforms = props.data.article.share.platforms as string[];
  const index = platforms.indexOf(platform);
  if (index >= 0) platforms.splice(index, 1);
  else platforms.push(platform);
}

type MapTarget = "category_color" | "category_icon" | "tag_icon" | "edit_this_page";

function addMapEntry(target: MapTarget) {
  props.data.article[target] = props.data.article[target] || {};
  const defaultKey = target === "edit_this_page" ? "路径前缀" : "新名称";
  let key = defaultKey;
  let index = 2;
  while (Object.prototype.hasOwnProperty.call(props.data.article[target], key)) key = `${defaultKey}${index++}`;
  props.data.article[target][key] = "";
}

function renameMapEntry(target: MapTarget, oldKey: string, event: Event) {
  const nextKey = (event.target as HTMLInputElement).value;
  if (nextKey === oldKey) return;
  const map = props.data.article[target] || {};
  const value = map[oldKey];
  delete map[oldKey];
  map[nextKey || oldKey] = value;
}

function removeMapEntry(target: MapTarget, key: string) {
  delete props.data.article[target]?.[key];
}

const aiLabelTiers = [
  { key: "manual", label: "人工创作" },
  { key: "reviewed", label: "AI 生成·已审核" },
  { key: "polished", label: "人工写作·AI 润色" },
  { key: "generated", label: "AI 生成" },
] as const;

</script>

<template>
  <form class="settings-post-form" @submit.prevent="handleSubmit">
    <section class="settings-post-section settings-post-section-first">
      <div class="settings-post-section-heading">
        <h2>文章展示</h2>
        <p>控制文章列表、文章卡片和文章页的内容呈现方式。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-post-field">
          <label for="settings-post-type">文章布局类型</label>
          <select id="settings-post-type" v-model="data.article.type" :class="selectCls">
            <option value="tech">tech（技术）</option>
            <option value="story">story（图文）</option>
          </select>
          <p class="settings-post-help">技术布局强调正文阅读，图文布局适合有封面和横幅的文章。</p>
        </div>

        <div class="settings-post-field">
          <label for="settings-post-card-style">有封面时的文章卡片样式</label>
          <select id="settings-post-card-style" v-model="data.article.card_style" :class="selectCls">
            <option value="hero">hero（全图文字）</option>
            <option value="classic">classic（摘要卡）</option>
          </select>
        </div>

        <div class="settings-post-field">
          <label for="settings-post-pin-style">置顶文章样式</label>
          <select id="settings-post-pin-style" v-model="data.article.pin_style" :class="selectCls">
            <option value="carousel">轮播</option>
            <option value="flat">平铺到列表顶部</option>
          </select>
        </div>

        <div class="settings-post-field">
          <label for="settings-post-cover-ratio">卡片封面宽高比</label>
          <input
            id="settings-post-cover-ratio"
            v-model.number="data.article.cover_ratio"
            type="number"
            step="0.1"
            min="0"
            :class="inputCls"
            placeholder="0（使用主题默认值，如 2）"
          />
          <p class="settings-post-help">填写 0 使用主题默认比例。</p>
        </div>

        <div class="settings-post-field">
          <label for="settings-post-banner-ratio">文首横幅宽高比</label>
          <input
            id="settings-post-banner-ratio"
            v-model.number="data.article.banner_ratio"
            type="number"
            step="0.1"
            min="0"
            :class="inputCls"
            placeholder="0（使用主题默认值，如 2.5）"
          />
        </div>

        <div class="settings-post-field">
          <label for="settings-post-auto-excerpt">自动摘要长度</label>
          <input
            id="settings-post-auto-excerpt"
            v-model.number="data.article.auto_excerpt"
            type="number"
            min="0"
            :class="inputCls"
            placeholder="0（关闭）"
          />
          <p class="settings-post-help">按字符数截取摘要，填写 0 关闭自动摘要。</p>
        </div>

        <div class="settings-post-field">
          <label for="settings-post-related-posts">相关文章数上限</label>
          <input
            id="settings-post-related-posts"
            v-model.number="data.article.related_posts"
            type="number"
            min="0"
            :class="inputCls"
            placeholder="0（关闭）"
          />
        </div>

        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">显示字数与阅读时长</div>
            <p class="settings-post-help">在文章卡片和文章页显示正文统计信息。</p>
          </div>
          <VSwitch v-model="data.article.reading_time" aria-label="显示字数与阅读时长" />
        </div>

        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">文章卡片显示标签</div>
            <p class="settings-post-help">文章列表卡片最多显示 5 个标签。</p>
          </div>
          <VSwitch v-model="data.article.card_tags" aria-label="文章卡片显示标签" />
        </div>
      </div>
    </section>

    <section class="settings-post-section">
      <div class="settings-post-section-heading">
        <h2>阅读体验</h2>
        <p>控制正文排版、上下篇导航和文章时效提示。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">正文段落首行缩进两字符</div>
            <p class="settings-post-help">适合中文长文阅读；关闭后正文保持常规左对齐。</p>
          </div>
          <VSwitch v-model="data.article.indent" aria-label="正文段落首行缩进两字符" />
        </div>

        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">上下篇导航</div>
            <p class="settings-post-help">在文章页底部显示较新与较早文章卡片。</p>
          </div>
          <VSwitch v-model="data.article.read_next" aria-label="上下篇导航" />
        </div>

        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">文章末尾显示标签</div>
            <p class="settings-post-help">在正文后、页脚前显示文章标签行。</p>
          </div>
          <VSwitch v-model="data.article.tags" aria-label="文章末尾显示标签" />
        </div>

        <div class="settings-post-field">
          <label for="settings-post-expire-days">过期提醒阈值</label>
          <input
            id="settings-post-expire-days"
            v-model.number="data.article.expire_days"
            type="number"
            min="0"
            :class="inputCls"
            placeholder="0（使用默认 180 天）"
          />
          <p class="settings-post-help">文章发布超过指定天数后显示提醒，填写 0 使用默认值 180 天。</p>
        </div>
      </div>
    </section>

    <section class="settings-post-section">
      <div class="settings-post-section-heading">
        <h2>分类与标签映射</h2>
        <p>为分类设置颜色或图标，为标签设置图标。修改名称会同步更新对应映射键。</p>
      </div>

      <div class="settings-section-body">
        <div
          v-for="target in ['category_color', 'category_icon', 'tag_icon'] as const"
          :key="target"
          class="settings-post-mapping-block"
        >
          <div class="settings-post-mapping-head">
            <div>
              <h3>
                {{ target === "category_color" ? "分类颜色" : target === "category_icon" ? "分类图标" : "标签图标" }}
              </h3>
              <p v-if="target === 'category_color'" class="settings-post-help">填写 CSS 颜色值，例如 #2e8dff。</p>
              <p v-else class="settings-post-help">输入图标 key，或从当前图标库中选择。</p>
            </div>
            <button type="button" class="settings-post-add-btn" @click="addMapEntry(target)">
              <Icon icon="ri:add-line" />
              <span>添加</span>
            </button>
          </div>

          <div class="settings-post-mapping-list">
            <div v-for="key in Object.keys(data.article?.[target] || {})" :key="key" class="settings-post-mapping-row">
              <input
                :value="key"
                :class="inputCls"
                placeholder="名称"
                aria-label="映射名称"
                @change="renameMapEntry(target, key, $event)"
              />
              <input
                v-if="target === 'category_color'"
                v-model="data.article[target][key]"
                :class="inputCls"
                placeholder="#2e8dff"
                aria-label="分类颜色"
              />
              <IconPicker
                v-else
                v-model="data.article[target][key]"
                compact
                placeholder="图标 key"
                class="settings-post-icon-picker"
              />
              <button
                type="button"
                class="settings-post-row-btn settings-post-row-btn-danger"
                title="删除"
                aria-label="删除映射"
                @click="removeMapEntry(target, key)"
              >
                <Icon icon="ri:delete-bin-line" />
              </button>
            </div>
            <p v-if="Object.keys(data.article?.[target] || {}).length === 0" class="settings-post-empty">
              暂无映射，可点击右上角添加。
            </p>
          </div>
        </div>
      </div>
    </section>

    <section class="settings-post-section">
      <div class="settings-post-section-heading">
        <h2>AI 成分标签</h2>
        <p>文章 front-matter 写 ai_label 后，在文章页顶部显示 AI 成分标记；各档可配置文字颜色与图标。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-post-field">
          <label for="settings-post-ai-default">未标注文章的默认档</label>
          <select id="settings-post-ai-default" v-model="data.article.ai_label.default" :class="selectCls">
            <option value="">不显示（历史文章零变化）</option>
            <option value="manual">人工创作</option>
            <option value="reviewed">AI 生成·已审核</option>
            <option value="polished">人工写作·AI 润色</option>
            <option value="generated">AI 生成</option>
          </select>
          <p class="settings-post-help">仅对未写 ai_label 的文章生效；填写后所有未标注文章都会显示该档标签。</p>
        </div>

        <div
          v-for="tier in aiLabelTiers"
          :key="tier.key"
          class="settings-post-field"
        >
          <span class="settings-post-field-label">{{ tier.label }}（{{ tier.key }}）</span>
          <div class="settings-post-ai-tier">
            <input
              v-model="data.article.ai_label[tier.key].color"
              :class="inputCls"
              placeholder="留空=内置默认色"
              aria-label="标签文字颜色"
            />
            <IconPicker
              v-model="data.article.ai_label[tier.key].icon"
              compact
              placeholder="图标 key（可选）"
              class="settings-post-icon-picker"
            />
          </div>
        </div>
      </div>
    </section>

    <section class="settings-post-section">
      <div class="settings-post-section-heading">
        <h2>编辑本文</h2>
        <p>源路径前缀 → 仓库编辑页 URL 前缀映射，命中时文章页显示「编辑本文」按钮与贡献者列表。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-post-mapping-block">
          <div class="settings-post-mapping-head">
            <div>
              <h3>路径映射</h3>
              <p class="settings-post-help">
                键为文章源路径前缀（如 posts/），值为仓库 blob URL 前缀（如
                https://github.com/user/repo/blob/main/content/posts/）。
              </p>
            </div>
            <button type="button" class="settings-post-add-btn" @click="addMapEntry('edit_this_page')">
              <Icon icon="ri:add-line" />
              <span>添加</span>
            </button>
          </div>

          <div class="settings-post-mapping-list">
            <div v-for="key in Object.keys(data.article?.edit_this_page || {})" :key="key" class="settings-post-mapping-row">
              <input
                :value="key"
                :class="inputCls"
                placeholder="源路径前缀"
                aria-label="源路径前缀"
                @change="renameMapEntry('edit_this_page', key, $event)"
              />
              <input
                v-model="data.article.edit_this_page[key]"
                :class="inputCls"
                placeholder="https://github.com/user/repo/blob/main/"
                aria-label="仓库编辑页 URL 前缀"
              />
              <button
                type="button"
                class="settings-post-row-btn settings-post-row-btn-danger"
                title="删除"
                aria-label="删除映射"
                @click="removeMapEntry('edit_this_page', key)"
              >
                <Icon icon="ri:delete-bin-line" />
              </button>
            </div>
            <p v-if="Object.keys(data.article?.edit_this_page || {}).length === 0" class="settings-post-empty">
              暂无映射，可点击右上角添加。
            </p>
          </div>
        </div>
      </div>
    </section>

    <section class="settings-post-section">
      <div class="settings-post-section-heading">
        <h2>版权与分享</h2>
        <p>设置文章页底部的版权信息和分享入口。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">显示版权声明</div>
            <p class="settings-post-help">在文章末尾显示许可协议和版权提示。</p>
          </div>
          <VSwitch v-model="data.article.copyright.enabled" aria-label="显示版权声明" />
        </div>

        <div class="settings-post-field">
          <label for="settings-post-license">协议名</label>
          <input
            id="settings-post-license"
            v-model="data.article.copyright.license"
            :class="inputCls"
            placeholder="CC BY-NC 4.0"
          />
        </div>

        <div class="settings-post-field">
          <label for="settings-post-license-url">协议链接</label>
          <input
            id="settings-post-license-url"
            v-model="data.article.copyright.license_url"
            :class="inputCls"
            placeholder="https://creativecommons.org/licenses/by-nc/4.0/"
          />
        </div>

        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">启用分享</div>
            <p class="settings-post-help">在文章页显示所选分享平台。</p>
          </div>
          <VSwitch v-model="data.article.share.enabled" aria-label="启用分享" />
        </div>

        <div class="settings-post-field settings-post-platform-field">
          <span class="settings-post-field-label">分享平台</span>
          <div class="settings-post-platforms" role="group" aria-label="分享平台">
            <label
              v-for="platform in platformOptions"
              :key="platform"
              class="settings-post-platform"
              :class="{ active: data.article?.share?.platforms?.includes(platform) }"
            >
              <input
                type="checkbox"
                :checked="data.article?.share?.platforms?.includes(platform)"
                @change="togglePlatform(platform)"
              />
              <span>{{ platform }}</span>
            </label>
          </div>
        </div>
      </div>
    </section>

    <section class="settings-post-section">
      <div class="settings-post-section-heading">
        <h2>打赏</h2>
        <p>在文章页提供可选的打赏入口和收款码。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-post-switch">
          <div class="settings-post-switch-copy">
            <div class="settings-post-switch-name">启用打赏</div>
            <p class="settings-post-help">开启后，在文章页显示打赏按钮和收款码。</p>
          </div>
          <VSwitch v-model="data.article.reward.enabled" aria-label="启用打赏" />
        </div>

        <div class="settings-post-field">
          <label for="settings-post-reward-text">按钮文案</label>
          <input id="settings-post-reward-text" v-model="data.article.reward.text" :class="inputCls" placeholder="赞赏" />
        </div>

        <div class="settings-post-field">
          <span class="settings-post-field-label">微信收款码</span>
          <AttachmentField v-model="data.article.reward.wechat" placeholder="输入图片 URL 或从附件库选择" />
        </div>

        <div class="settings-post-field">
          <span class="settings-post-field-label">支付宝收款码</span>
          <AttachmentField v-model="data.article.reward.alipay" placeholder="输入图片 URL 或从附件库选择" />
        </div>
      </div>
    </section>

    <div class="settings-post-actions">
      <div class="settings-post-actions-inner">
        <VButton type="primary" size="md" native-type="submit" :disabled="Boolean(saving)" :loading="saving === 'article'">
          <template #icon><Icon icon="ri:save-line" /></template>
          保存文章设置
        </VButton>
      </div>
    </div>
  </form>
</template>

<style scoped>
.settings-post-form {
  background: var(--color-card);
}

.settings-post-section {
  margin: 0;
  padding: 28px;
  border-bottom: 1px solid var(--color-border);
}

.settings-post-section-first {
  padding-top: 28px;
}

.settings-post-section-heading {
  max-width: 860px;
  margin-bottom: 18px;
}

.settings-post-section-heading h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--color-foreground);
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.settings-post-section-heading h2::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 16px;
  border-radius: 2px;
  background: var(--color-primary);
  flex-shrink: 0;
}

.settings-post-section-heading p,
.settings-post-help {
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

.settings-post-field,
.settings-post-switch {
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

.settings-post-field > label,
.settings-post-field-label {
  display: block;
  margin-bottom: 10px;
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
}

.settings-post-field > .apple-input,
.settings-post-field > .apple-select {
  display: block;
  width: min(100%, 860px);
  max-width: 860px;
}

.settings-post-field > :deep(.attachment-field),
.settings-post-field > :deep(.ip-field) {
  width: min(100%, 860px);
}

.settings-post-switch {
  display: flex;
  min-height: 44px;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.settings-post-switch-copy {
  min-width: 0;
}

.settings-post-switch-name {
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
  line-height: 1.4;
}

.settings-post-switch .settings-post-help {
  margin-top: 6px;
}

.settings-post-switch :deep(button[role="switch"]) {
  flex: 0 0 auto;
  margin-top: 2px;
}

.settings-post-mapping-block {
  max-width: 860px;
  margin: 0 0 24px;
}

.settings-post-mapping-block:last-child {
  margin-bottom: 0;
}

.settings-post-mapping-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.settings-post-mapping-head h3 {
  margin: 0;
  color: var(--color-foreground);
  font-size: 13.5px;
  font-weight: 600;
}

.settings-post-mapping-head .settings-post-help {
  margin-top: 4px;
}

.settings-post-add-btn {
  display: inline-flex;
  min-height: 44px;
  flex: 0 0 auto;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  border: 0;
  background: transparent;
  color: var(--color-primary);
  cursor: pointer;
  font-size: 13px;
}

.settings-post-add-btn:hover {
  color: var(--color-foreground);
}

.settings-post-add-btn:focus-visible,
.settings-post-row-btn:focus-visible,
.settings-post-platform:focus-within {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.settings-post-mapping-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.settings-post-mapping-row {
  display: grid;
  grid-template-columns: minmax(150px, 0.65fr) minmax(240px, 1.35fr) 44px;
  align-items: start;
  gap: 12px;
}

.settings-post-icon-picker {
  min-width: 0;
}

.settings-post-ai-tier {
  display: grid;
  grid-template-columns: minmax(200px, 0.5fr) minmax(240px, 1fr);
  align-items: start;
  gap: 12px;
}

@media (max-width: 640px) {
  .settings-post-ai-tier {
    grid-template-columns: 1fr;
  }
}

.settings-post-row-btn {
  display: inline-flex;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-text-quaternary);
  cursor: pointer;
}

.settings-post-row-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}

.settings-post-row-btn-danger {
  color: var(--color-state-error);
}

.settings-post-empty {
  margin: 0;
  color: var(--color-text-quaternary);
  font-size: 12px;
}

.settings-post-platform-field {
  margin-bottom: 0;
}

.settings-post-platforms {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.settings-post-platform {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  gap: 7px;
  padding: 0 13px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-full);
  background: var(--color-secondary);
  color: var(--color-text-secondary);
  cursor: pointer;
  font-size: 12.5px;
}

.settings-post-platform input {
  position: absolute;
  pointer-events: none;
  opacity: 0;
}

.settings-post-platform.active {
  border-color: color-mix(in srgb, var(--color-primary) 40%, transparent);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
}

/* ================= 操作底栏 ================= */
.settings-post-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  padding: 16px 28px calc(16px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: var(--color-card);
}

.settings-post-actions-inner {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

@media (max-width: 767px) {
  .settings-post-section {
    padding-left: 16px;
    padding-right: 16px;
  }
}

@media (max-width: 640px) {
  .settings-post-section {
    padding-top: 24px;
    padding-bottom: 24px;
  }

  .settings-post-switch {
    gap: 12px;
  }

  .settings-post-mapping-head {
    align-items: center;
  }

  .settings-post-mapping-row {
    grid-template-columns: 1fr 44px;
  }

  .settings-post-mapping-row > :nth-child(1),
  .settings-post-mapping-row > :nth-child(2) {
    grid-column: 1 / -1;
  }

  .settings-post-mapping-row > .settings-post-row-btn {
    grid-column: 1;
    grid-row: 3;
  }

  .settings-post-actions {
    padding: 14px 16px calc(14px + env(safe-area-inset-bottom));
  }
}
</style>
