<script setup lang="ts">
import AttachmentField from "@/components/AttachmentField.vue";
import { Toast } from "@/components/ui";
import { settingsSectionAnchor } from "../settings-pages";

// 站点信息页：站点标识、Logo、图标与站点描述/URL。独立保存（site 分段），保存栏由页面通过 actions 插槽提供。
const props = defineProps<{ data: Record<string, any>; saved?: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
const anchor = (id: string) => settingsSectionAnchor("site", id);
// 相对上次保存有改动的字段显示橙点；没有快照时不显示。
const changed = (key: string) =>
  props.saved !== undefined && JSON.stringify(props.data.site?.[key]) !== JSON.stringify(props.saved[key]);

function handleSubmit() {
  const title = (props.data?.site?.title || "").trim();
  if (!title) {
    Toast.error("站点标题不能为空");
    document.getElementById("settings-site-title")?.focus();
    return;
  }
  emit("save", "site");
}
</script>

<template>
  <form class="settings-basic-form" @submit.prevent="handleSubmit">
    <section :id="anchor('id')" class="settings-sec">
      <div class="settings-sec-head">
        <div class="settings-sec-title">
          <h2>站点标识</h2>
          <p>站点的核心名称与副标题，显示在前台首页、导航栏与浏览器标签中。</p>
        </div>
      </div>
      <div class="settings-rows">
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-site-title">站点标题 <span class="settings-required" aria-hidden="true">*</span></label>
              <span v-if="changed('title')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">必填；留空无法保存。</p>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-site-title"
              v-model="data.site.title"
              :class="[inputCls, 'settings-control-text']"
              required
            />
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-site-subtitle">站点副标题</label>
              <span v-if="changed('subtitle')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-site-subtitle"
              v-model="data.site.subtitle"
              :class="[inputCls, 'settings-control-text']"
              placeholder="例如：热爱生活，保持微笑"
            />
          </div>
        </div>
      </div>
    </section>

    <section :id="anchor('logo')" class="settings-sec">
      <div class="settings-sec-head">
        <div class="settings-sec-title">
          <h2>Logo</h2>
          <p>站点头像和 Logo 文案会显示在前台导航区域。</p>
        </div>
      </div>
      <div class="settings-rows">
        <div class="settings-row is-block">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <span class="settings-row-name">Logo 主图</span>
              <span v-if="changed('avatar')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">当前主题没有独立 Logo 图标时使用站点头像作为 Logo。</p>
          </div>
          <div class="settings-row-control">
            <AttachmentField v-model="data.site.avatar" placeholder="输入图片 URL" show-thumb />
          </div>
        </div>
        <div class="settings-row is-block">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <span class="settings-row-name">Logo 悬停背景图（可选）</span>
              <span v-if="changed('avatar_bg')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">鼠标悬停在头像上时使用的背景图，留空则不启用。</p>
          </div>
          <div class="settings-row-control">
            <AttachmentField v-model="data.site.avatar_bg" placeholder="输入图片 URL" show-thumb />
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-logo-title">Logo 主标题</label>
              <span v-if="changed('logo_title')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
          </div>
          <div class="settings-row-control">
            <input id="settings-logo-title" v-model="data.site.logo_title" :class="[inputCls, 'settings-control-text']" />
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-logo-sub">Logo 副标题</label>
              <span v-if="changed('logo_sub')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">含 | 时拆分为悬停前后的两段文案。</p>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-logo-sub"
              v-model="data.site.logo_sub"
              :class="[inputCls, 'settings-control-text']"
              placeholder="含 | 时拆分为悬停前后的两段文案"
            />
          </div>
        </div>
      </div>

      <div v-if="data.site.avatar || data.site.avatar_bg" class="settings-preview-box settings-basic-preview-box">
        <p class="settings-preview-label">Logo 使用预览</p>
        <div class="settings-preview-row">
          <div v-if="data.site.avatar" class="settings-preview-item">
            <p>导航栏头像 32×32</p>
            <div class="settings-avatar-32"><img :src="data.site.avatar" alt="" /></div>
          </div>
          <div v-if="data.site.avatar" class="settings-preview-item">
            <p>分享卡片 128×128</p>
            <div class="settings-avatar-128"><img :src="data.site.avatar" alt="" /></div>
          </div>
          <div v-if="data.site.avatar && data.site.avatar_bg" class="settings-preview-item">
            <p>悬停效果</p>
            <div class="settings-avatar-hover">
              <img :src="data.site.avatar_bg" class="settings-avatar-bg" alt="" /><img
                :src="data.site.avatar"
                class="settings-avatar-fg"
                alt=""
              />
            </div>
          </div>
        </div>
      </div>
    </section>

    <section :id="anchor('icon')" class="settings-sec">
      <div class="settings-sec-head">
        <div class="settings-sec-title">
          <h2>浏览器与设备图标</h2>
          <p>标签页、收藏夹与 iOS 主屏幕使用；留空时使用默认图标。</p>
        </div>
      </div>
      <div class="settings-rows">
        <div class="settings-row is-block">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <span class="settings-row-name">Favicon</span>
              <span v-if="changed('favicon_url')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">浏览器标签页和收藏夹使用的站点图标；留空时前台将自动使用默认的 /favicon.ico。</p>
          </div>
          <div class="settings-row-control">
            <AttachmentField v-model="data.site.favicon_url" placeholder="留空使用默认 /favicon.ico" show-thumb />
          </div>
        </div>
        <div class="settings-row is-block">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <span class="settings-row-name">Apple Touch Icon</span>
              <span v-if="changed('apple_touch_icon')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">
              添加到 iOS/iPadOS 主屏幕时使用的图标；留空时前台将自动使用默认的 /apple-touch-icon.png。
            </p>
          </div>
          <div class="settings-row-control">
            <AttachmentField
              v-model="data.site.apple_touch_icon"
              placeholder="留空使用默认 /apple-touch-icon.png"
              show-thumb
            />
          </div>
        </div>
      </div>
    </section>

    <section :id="anchor('desc')" class="settings-sec">
      <div class="settings-sec-head">
        <div class="settings-sec-title">
          <h2>描述与地址</h2>
          <p>用于首页与未单独设置描述的页面，也是搜索结果和分享卡片的来源。</p>
        </div>
      </div>
      <div class="settings-rows">
        <div class="settings-row is-block">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-site-desc">站点描述</label>
              <span v-if="changed('description')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
          </div>
          <div class="settings-row-control">
            <textarea
              id="settings-site-desc"
              v-model="data.site.description"
              rows="2"
              class="apple-textarea"
              placeholder="用于首页和未单独设置描述的页面"
            ></textarea>
          </div>
        </div>
        <div class="settings-row">
          <div class="settings-row-label">
            <div class="settings-row-title">
              <label for="settings-site-url">站点 URL</label>
              <span v-if="changed('url')" class="settings-dot" role="img" aria-label="有未保存修改"></span>
            </div>
            <p class="settings-help">站点对外访问的完整地址，用于生成绝对链接。</p>
          </div>
          <div class="settings-row-control">
            <input
              id="settings-site-url"
              v-model="data.site.url"
              :class="[inputCls, 'settings-control-text']"
              placeholder="https://example.com"
            />
          </div>
        </div>
      </div>
    </section>

    <slot name="actions" />
  </form>
</template>

<style scoped>
.settings-basic-form {
  background: transparent;
}
.settings-required {
  color: var(--color-state-error);
}
.settings-row-name {
  color: var(--color-foreground);
  font-size: 14px;
  font-weight: 500;
}
.settings-basic-preview-box {
  max-width: 860px;
  margin-top: 16px;
  margin-bottom: 8px;
}
</style>
