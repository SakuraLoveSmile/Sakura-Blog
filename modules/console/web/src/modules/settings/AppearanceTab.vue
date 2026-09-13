<script setup lang="ts">
import { VButton, VSwitch, Dialog, Toast } from "@/components/ui";
import AttachmentField from "@/components/AttachmentField.vue";
import { normalizeStyleSettings } from "./settings-state";

const props = defineProps<{
  data: Record<string, any>;
  saving: string;
}>();
const emit = defineEmits<{
  save: [section: string];
}>();

const inputCls = "apple-input";
const selectCls = "apple-select";

/** 外观段初始（默认）结构：全部留空 = 不注入任何覆盖，回退主题内置值 */
function defaultStyle(): Record<string, any> {
  return {
    prefers_theme: "",
    color: { theme: "", accent: "", link: "" },
    font_size: { root: "", body: "", code: "", codeblock: "" },
    font_family: { body: "", code: "", codeblock: "" },
    // blur_px 为 px 字符串（后端 string 字段，直接注入 CSS）；留空 = 回退主题内置值
    leftbar: { ui_style: "card", background_color_dark: "", background_image: "", blur_px: "", background_opacity: "" },
    site: { background_image: "", blur_px: "", blur_sat: "", blur_bg: "" },
    codeblock: { scrollbar: "", highlightjs_theme: "" },
    page_transition: { enable: true },
    header_prefix: { h2: "", h3: "", h4: "", h5: "" },
    text_align: "",
    border_radius: {
      card: "",
      block: "",
      navbar: "",
      image: "",
      card_l: "",
      card_s: "",
      bar: "",
      image_l: "",
      image_s: "",
    },
    gradient: { start: "", searchbar: "", avatar: "", angle: "" },
    corner_shape: "superellipse(1.25)",
    animated_avatar: { animate: "auto", background: "" },
    loading: { loading: "", error: "" },
    error_page: "",
  };
}

/** 恢复默认初始值（内存重置，需点保存才生效） */
function resetStyle() {
  Dialog.warning({
    title: "恢复外观默认值？",
    description: "将清空所有外观自定义（主题色/字号/字体/背景等），回退到主题内置值。需点击保存后生效。",
    confirmType: "danger",
    onConfirm: () => {
      props.data.style = normalizeStyleSettings(defaultStyle());
      Toast.success("已恢复默认，点击保存后生效");
    },
  });
}

/** 从 px 字符串提取数值，如 "16px" → 16 */
function pxNum(v: string | undefined, fallback = 16): number {
  if (!v) return fallback;
  return parseInt(String(v).replace("px", ""), 10) || fallback;
}
/** 数值转 px 字符串 */
function numPx(n: number): string {
  return n + "px";
}
/** 滑杆填充背景：已选部分主色、剩余部分浅灰，模拟 Apple 进度滑杆 */
function rangeFill(val: number, min: number, max: number): string {
  const pct = max > min ? ((val - min) / (max - min)) * 100 : 0;
  const track = "color-mix(in srgb, #fff 16%, transparent)";
  const fill = "var(--color-primary)";
  return `linear-gradient(to right, ${fill} 0%, ${fill} ${pct}%, ${track} ${pct}%, ${track} 100%)`;
}
</script>

<template>
  <div>
    <!-- 1. 主题色 -->
    <div class="subsection">
      <p class="subsection-title">主题色</p>
      <div class="form-grid form-grid-3">
        <div class="field">
          <div class="flex items-center justify-between mb-1">
            <label class="mb-0">主题色 (theme)</label>
            <span v-if="data.style.color?.theme" class="settings-color-badge settings-color-badge-override">
              已覆盖
              <button type="button" class="settings-color-reset-btn" @click="data.style.color.theme = ''">恢复默认</button>
            </span>
            <span v-else class="settings-color-badge settings-color-badge-inherit">继承默认值</span>
          </div>
          <div class="field-inline">
            <input
              type="color"
              :value="data.style.color?.theme || '#2E8DFF'"
              class="apple-color"
              @input="data.style.color.theme = ($event.target as HTMLInputElement).value"
            />
            <input v-model="data.style.color.theme" :class="inputCls" placeholder="#2E8DFF (默认)" />
          </div>
        </div>
        <div class="field">
          <div class="flex items-center justify-between mb-1">
            <label class="mb-0">强调色 (accent)</label>
            <span v-if="data.style.color?.accent" class="settings-color-badge settings-color-badge-override">
              已覆盖
              <button type="button" class="settings-color-reset-btn" @click="data.style.color.accent = ''">恢复默认</button>
            </span>
            <span v-else class="settings-color-badge settings-color-badge-inherit">继承默认值</span>
          </div>
          <div class="field-inline">
            <input
              type="color"
              :value="data.style.color?.accent || '#ff9800'"
              class="apple-color"
              @input="data.style.color.accent = ($event.target as HTMLInputElement).value"
            />
            <input v-model="data.style.color.accent" :class="inputCls" placeholder="#ff9800 (默认)" />
          </div>
        </div>
        <div class="field">
          <div class="flex items-center justify-between mb-1">
            <label class="mb-0">链接色 (link)</label>
            <span v-if="data.style.color?.link" class="settings-color-badge settings-color-badge-override">
              已覆盖
              <button type="button" class="settings-color-reset-btn" @click="data.style.color.link = ''">恢复默认</button>
            </span>
            <span v-else class="settings-color-badge settings-color-badge-inherit">继承默认值</span>
          </div>
          <div class="field-inline">
            <input
              type="color"
              :value="data.style.color?.link || '#2196f3'"
              class="apple-color"
              @input="data.style.color.link = ($event.target as HTMLInputElement).value"
            />
            <input v-model="data.style.color.link" :class="inputCls" placeholder="#2196f3 (默认)" />
          </div>
        </div>
      </div>
    </div>

    <!-- 2. 字体字号 -->
    <div class="subsection">
      <p class="subsection-title">字体字号</p>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>根字号 (root)</label>
          <div class="field-inline">
            <input
              type="range"
              min="12"
              max="20"
              step="1"
              class="apple-range"
              :value="pxNum(data.style.font_size?.root)"
              :style="{ background: rangeFill(pxNum(data.style.font_size?.root), 12, 20) }"
              @input="data.style.font_size.root = numPx(Number(($event.target as HTMLInputElement).value))"
            />
            <span class="apple-range-value">{{ data.style.font_size.root || "16px" }}</span>
          </div>
        </div>
        <div class="field">
          <label>正文字号 (body)</label>
          <div class="field-inline">
            <input
              type="range"
              min="12"
              max="20"
              step="1"
              class="apple-range"
              :value="pxNum(data.style.font_size?.body, 15)"
              :style="{ background: rangeFill(pxNum(data.style.font_size?.body, 15), 12, 20) }"
              @input="data.style.font_size.body = numPx(Number(($event.target as HTMLInputElement).value))"
            />
            <span class="apple-range-value">{{ data.style.font_size.body || "15px" }}</span>
          </div>
        </div>
      </div>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>行内代码字号 (code)</label>
          <input v-model="data.style.font_size.code" :class="inputCls" placeholder="85%" />
        </div>
        <div class="field">
          <label>代码块字号 (codeblock)</label>
          <input v-model="data.style.font_size.codeblock" :class="inputCls" placeholder="0.8125rem" />
        </div>
      </div>
      <p class="hint">code/codeblock 值为百分比或 rem（如 85% / 0.8125rem），留空回退主题内置值。</p>
    </div>

    <!-- 3. 字体族 -->
    <div class="subsection">
      <p class="subsection-title">字体族</p>
      <div class="form-grid form-grid-3">
        <div class="field">
          <label>正文字体 (body)</label>
          <input
            v-model="data.style.font_family.body"
            :class="inputCls"
            list="font-family-candidates"
            placeholder="留空使用默认字体"
          />
        </div>
        <div class="field">
          <label>行内代码字体 (code)</label>
          <input
            v-model="data.style.font_family.code"
            :class="inputCls"
            list="font-family-candidates"
            placeholder="留空使用默认字体"
          />
        </div>
        <div class="field">
          <label>代码块字体 (codeblock)</label>
          <input
            v-model="data.style.font_family.codeblock"
            :class="inputCls"
            list="font-family-candidates"
            placeholder="留空使用默认字体"
          />
        </div>
      </div>
      <!-- 候选列表：仅提示用，手填值照常写入 data.style.font_family.* -->
      <datalist id="font-family-candidates">
        <option value="LXGW WenKai Screen" />
        <option value="system-ui" />
        <option value="DM Sans" />
        <option value="JetBrains Mono" />
        <option value="Menlo" />
        <option value="Consolas" />
        <option value="monospace" />
        <option value="PingFang SC" />
        <option value="Microsoft YaHei" />
        <option value="sans-serif" />
      </datalist>
    </div>

    <!-- 4. 左侧栏背景 -->
    <div class="subsection">
      <p class="subsection-title">左侧栏背景</p>
      <div class="field">
        <label>左侧栏样式</label>
        <select v-model="data.style.leftbar.ui_style" :class="selectCls">
          <option value="card">纯色模式</option>
          <option value="glass">光晕模式（背景图毛玻璃）</option>
        </select>
        <p v-if="data.style.leftbar.ui_style === 'glass' && !data.style.leftbar.background_image" class="hint">
          未配置背景图片时使用主题内置默认背景图；配置后将替换为自定义图片。
        </p>
      </div>
      <div class="field">
        <div class="flex items-center justify-between mb-1">
          <label class="mb-0">背景色</label>
          <span v-if="data.style.leftbar?.background_color_dark" class="settings-color-badge settings-color-badge-override">
            已覆盖
            <button type="button" class="settings-color-reset-btn" @click="data.style.leftbar.background_color_dark = ''">恢复默认</button>
          </span>
          <span v-else class="settings-color-badge settings-color-badge-inherit">继承默认值</span>
        </div>
        <div class="field-inline">
          <input
            type="color"
            :value="data.style.leftbar?.background_color_dark || '#1c1c1e'"
            class="apple-color"
            @input="data.style.leftbar.background_color_dark = ($event.target as HTMLInputElement).value"
          />
          <input
            v-model="data.style.leftbar.background_color_dark"
            :class="inputCls"
            placeholder="#1c1c1e (默认)"
          />
        </div>
      </div>
      <div class="field">
        <label>背景图片 URL</label>
        <AttachmentField v-model="data.style.leftbar.background_image" placeholder="输入图片 URL" show-thumb />
      </div>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>模糊半径 (blur_px)</label>
          <div class="field-inline">
            <input
              type="range"
              min="0"
              max="20"
              step="1"
              class="apple-range"
              :value="pxNum(data.style.leftbar.blur_px, 0)"
              :style="{ background: rangeFill(pxNum(data.style.leftbar.blur_px, 0), 0, 20) }"
              @input="data.style.leftbar.blur_px = numPx(Number(($event.target as HTMLInputElement).value))"
            />
            <span class="apple-range-value">{{ data.style.leftbar.blur_px || "0px" }}</span>
          </div>
        </div>
        <div class="field">
          <label>背景不透明度</label>
          <input v-model="data.style.leftbar.background_opacity" :class="inputCls" placeholder="1.0" />
        </div>
      </div>
    </div>

    <!-- 5. 全站背景 -->
    <div class="subsection">
      <p class="subsection-title">全站背景</p>
      <div class="field">
        <label>背景图片 URL</label>
        <AttachmentField v-model="data.style.site.background_image" placeholder="输入图片 URL" show-thumb />
      </div>
      <div class="form-grid form-grid-3">
        <div class="field">
          <label>模糊半径 (blur_px)</label>
          <input v-model="data.style.site.blur_px" :class="inputCls" placeholder="0" />
        </div>
        <div class="field">
          <label>模糊饱和度 (blur_sat)</label>
          <input v-model="data.style.site.blur_sat" :class="inputCls" placeholder="1" />
        </div>
        <div class="field">
          <label>模糊背景 (blur_bg)</label>
          <input v-model="data.style.site.blur_bg" :class="inputCls" placeholder="0" />
        </div>
      </div>
    </div>

    <!-- 6. 代码块 -->
    <div class="subsection">
      <p class="subsection-title">代码块</p>
      <div class="field">
        <label>滚动条宽度 (scrollbar)</label>
        <input v-model="data.style.codeblock.scrollbar" :class="inputCls" placeholder="如 4px，留空回退主题内置值" />
      </div>
      <div class="field">
        <label>高亮主题 URL (highlightjs_theme)</label>
        <input
          v-model="data.style.codeblock.highlightjs_theme"
          :class="inputCls"
          placeholder="highlight.js 主题 CSS URL"
        />
        <p class="hint">配置对齐上游 style.codeblock.highlightjs_theme，由 head 注入 link 生效。</p>
      </div>
    </div>

    <!-- 7. 杂项 -->
    <div class="subsection">
      <p class="subsection-title">杂项</p>
      <div class="field">
        <label>标题前缀 (header_prefix)</label>
        <p class="hint">各标题级别前缀字符（如 h2="#"）；非 h2-h5 的存量键保留不动，可在「高级 JSON」兜底中查看。</p>
        <div class="form-grid form-grid-2">
          <div class="field">
            <label>h2</label><input v-model="data.style.header_prefix.h2" :class="inputCls" placeholder="如 #" />
          </div>
          <div class="field">
            <label>h3</label><input v-model="data.style.header_prefix.h3" :class="inputCls" placeholder="如 =" />
          </div>
          <div class="field">
            <label>h4</label><input v-model="data.style.header_prefix.h4" :class="inputCls" placeholder="如 ~" />
          </div>
          <div class="field">
            <label>h5</label><input v-model="data.style.header_prefix.h5" :class="inputCls" placeholder="如 ^" />
          </div>
        </div>
      </div>
    </div>

    <!-- 8. 布局与效果 -->
    <div class="subsection">
      <p class="subsection-title">布局与效果</p>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>正文对齐 (text_align)</label>
          <select v-model="data.style.text_align" class="apple-select">
            <option value="">默认（left）</option>
            <option value="left">left</option>
            <option value="justify">justify（两端对齐）</option>
          </select>
        </div>
        <div class="field">
          <label>整页过渡动画 (page_transition)</label>
          <p class="hint">同源导航使用原生 View Transition（Stellar 1.43 默认开启）。</p>
          <VSwitch v-model="data.style.page_transition.enable" aria-label="整页过渡动画" />
        </div>
      </div>
    </div>

    <!-- 9. 圆角 -->
    <div class="subsection">
      <p class="subsection-title">圆角 (border_radius)</p>
      <p class="hint">四档圆角，留空回退主题内置值；值为 CSS 长度，如 16px。</p>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>卡片 (card)</label
          ><input v-model="data.style.border_radius.card" :class="inputCls" placeholder="16px" />
        </div>
        <div class="field">
          <label>块级元素 (block)</label
          ><input v-model="data.style.border_radius.block" :class="inputCls" placeholder="8px" />
        </div>
        <div class="field">
          <label>导航栏 (navbar)</label
          ><input v-model="data.style.border_radius.navbar" :class="inputCls" placeholder="100px" />
        </div>
        <div class="field">
          <label>图片 (image)</label
          ><input v-model="data.style.border_radius.image" :class="inputCls" placeholder="8px" />
        </div>
        <div class="field">
          <label>大卡片 (card-l)</label
          ><input v-model="data.style.border_radius.card_l" :class="inputCls" placeholder="24px" />
        </div>
        <div class="field">
          <label>小卡片 (card-s)</label
          ><input v-model="data.style.border_radius.card_s" :class="inputCls" placeholder="12px" />
        </div>
        <div class="field">
          <label>横条 (bar)</label><input v-model="data.style.border_radius.bar" :class="inputCls" placeholder="12px" />
        </div>
        <div class="field">
          <label>大图 (image-l)</label
          ><input v-model="data.style.border_radius.image_l" :class="inputCls" placeholder="24px" />
        </div>
        <div class="field">
          <label>小图 (image-s)</label
          ><input v-model="data.style.border_radius.image_s" :class="inputCls" placeholder="8px" />
        </div>
      </div>
    </div>

    <div class="subsection">
      <p class="subsection-title">连续曲率与头像</p>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>corner-shape</label
          ><input v-model="data.style.corner_shape" :class="inputCls" placeholder="superellipse(1.25)" />
        </div>
        <div class="field">
          <label>头像光环渐变</label
          ><input v-model="data.style.gradient.avatar" :class="inputCls" placeholder="conic-gradient(...)" />
        </div>
        <div class="field">
          <label>头像动画</label
          ><select v-model="data.style.animated_avatar.animate" :class="selectCls">
            <option value="auto">auto</option>
            <option value="always">always</option>
          </select>
        </div>
        <div class="field">
          <label>头像动画背景图</label
          ><AttachmentField v-model="data.style.animated_avatar.background" placeholder="图片 URL" />
        </div>
      </div>
    </div>

    <div class="subsection">
      <p class="subsection-title">加载与错误页</p>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>加载文案</label><input v-model="data.style.loading.loading" :class="inputCls" placeholder="正在加载" />
        </div>
        <div class="field">
          <label>加载失败文案</label
          ><input v-model="data.style.loading.error" :class="inputCls" placeholder="加载失败，请稍后重试。" />
        </div>
        <div class="field field-wide">
          <label>错误页插图</label><AttachmentField v-model="data.style.error_page" placeholder="图片 URL" show-thumb />
        </div>
      </div>
    </div>

    <!-- 10. 渐变 -->
    <div class="subsection">
      <p class="subsection-title">渐变 (gradient)</p>
      <p class="hint">任意 CSS gradient 表达式，留空回退主题内置值。</p>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>按钮/起始背景 (start)</label
          ><input v-model="data.style.gradient.start" :class="inputCls" placeholder="linear-gradient(...)" />
        </div>
        <div class="field">
          <label>搜索栏背景 (searchbar)</label
          ><input v-model="data.style.gradient.searchbar" :class="inputCls" placeholder="linear-gradient(...)" />
        </div>
        <div class="field">
          <label>菜单渐变角度 (angle)</label
          ><input v-model="data.style.gradient.angle" :class="inputCls" placeholder="210deg" />
        </div>
      </div>
    </div>

    <!-- 保存 / 恢复默认 -->
    <div class="card-actions">
      <VButton type="primary" size="sm" :loading="saving === 'style'" @click="emit('save', 'style')">
        保存外观设置
      </VButton>
      <VButton size="sm" @click="resetStyle">恢复默认</VButton>
    </div>
  </div>
</template>
