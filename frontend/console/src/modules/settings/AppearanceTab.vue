<script setup lang="ts">
import { computed } from "vue";
import { VButton, VSwitch, Dialog, Toast } from "@/components/ui";
import AttachmentField from "@/components/AttachmentField.vue";

const props = defineProps<{
  data: Record<string, any>;
  saving: string;
}>();
const emit = defineEmits<{
  save: [section: string];
}>();

const inputCls = "apple-input";

/** 外观段初始（默认）结构：全部留空 = 不注入任何覆盖，回退主题内置值 */
function defaultStyle(): Record<string, any> {
  return {
    prefers_theme: "",
    color: { theme: "", accent: "", link: "" },
    // 后端模型仅支持 root/body 字号，无 code/codeblock
    font_size: { root: "", body: "" },
    font_family: { body: "", code: "", codeblock: "" },
    // blur_px 为 px 字符串（后端 string 字段，直接注入 CSS）；留空 = 回退主题内置值
    leftbar: { background_color_dark: "", background_image: "", blur_px: "", background_opacity: "" },
    site: { background_image: "", blur_px: "", blur_sat: "", blur_bg: "" },
    codeblock: { scrollbar: "" },
    header_prefix: { h2: "", h3: "", h4: "", h5: "" },
    // smooth_scroll 语义：null=未配置（默认开），false=显式关闭；UI 代理见 smoothScrollOn
    smooth_scroll: null,
    text_align: "",
    border_radius: { card: "", block: "", navbar: "", image: "" },
    gradient: { button: "", searchbar: "" },
    paginator: { prev: "", next: "" },
  };
}

// 容错：加载数据缺键时补齐，保证 header_prefix 的 v-model 绑定路径安全；非 h2-h5 存量键保留不动。
// 类型守卫：存量数据经「高级 JSON」保存后 header_prefix 可能为非对象 primitive（如字符串），
// 对 primitive 赋属性会抛 TypeError 导致设置页白屏 → 非对象（含数组）整体重置为 {}
const hpRaw = props.data.style.header_prefix;
if (!hpRaw || typeof hpRaw !== "object" || Array.isArray(hpRaw)) {
  props.data.style.header_prefix = {};
}
const hpSafe = props.data.style.header_prefix as Record<string, any>;
for (const k of ["h2", "h3", "h4", "h5"] as const) {
  if (hpSafe[k] === undefined) hpSafe[k] = "";
}
// 容错：存量 blur_px 可能为历史遗留数值（后端字段为 string，数字无法保存），统一规范化为 px 字符串
if (typeof props.data.style.leftbar?.blur_px === "number") {
  const n = props.data.style.leftbar.blur_px as number;
  props.data.style.leftbar.blur_px = n > 0 ? numPx(n) : "";
}

// 容错：存量 style 可能缺 border_radius/gradient/paginator 键，补齐保证 v-model 绑定路径安全
const styleRef = props.data.style as Record<string, any>;
styleRef.border_radius = styleRef.border_radius || {};
styleRef.gradient = styleRef.gradient || {};
styleRef.paginator = styleRef.paginator || {};

// smooth_scroll 开关代理：后端 *bool（null=未配置默认开），开→null、关→false
const smoothScrollOn = computed({
  get: () => styleRef.smooth_scroll !== false,
  set: (v: boolean) => { styleRef.smooth_scroll = v ? null : false; },
});

/** 恢复默认初始值（内存重置，需点保存才生效） */
function resetStyle() {
  Dialog.warning({
    title: "恢复外观默认值？",
    description: "将清空所有外观自定义（主题色/字号/字体/背景等），回退到主题内置值。需点击保存后生效。",
    confirmType: "danger",
    onConfirm: () => {
      props.data.style = defaultStyle();
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
          <label>主题色 (theme)</label>
          <div class="field-inline">
            <input type="color" v-model="data.style.color.theme" class="apple-color" />
            <input v-model="data.style.color.theme" :class="inputCls" placeholder="#2E8DFF" />
          </div>
        </div>
        <div class="field">
          <label>强调色 (accent)</label>
          <div class="field-inline">
            <input type="color" v-model="data.style.color.accent" class="apple-color" />
            <input v-model="data.style.color.accent" :class="inputCls" placeholder="#ff9800" />
          </div>
        </div>
        <div class="field">
          <label>链接色 (link)</label>
          <div class="field-inline">
            <input type="color" v-model="data.style.color.link" class="apple-color" />
            <input v-model="data.style.color.link" :class="inputCls" placeholder="#2196f3" />
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
              type="range" min="12" max="20" step="1" class="apple-range"
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
              type="range" min="12" max="20" step="1" class="apple-range"
              :value="pxNum(data.style.font_size?.body, 15)"
              :style="{ background: rangeFill(pxNum(data.style.font_size?.body, 15), 12, 20) }"
              @input="data.style.font_size.body = numPx(Number(($event.target as HTMLInputElement).value))"
            />
            <span class="apple-range-value">{{ data.style.font_size.body || "15px" }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 3. 字体族 -->
    <div class="subsection">
      <p class="subsection-title">字体族</p>
      <div class="form-grid form-grid-3">
        <div class="field">
          <label>正文字体 (body)</label>
          <input v-model="data.style.font_family.body" :class="inputCls" list="font-family-candidates" placeholder="留空使用默认字体" />
        </div>
        <div class="field">
          <label>行内代码字体 (code)</label>
          <input v-model="data.style.font_family.code" :class="inputCls" list="font-family-candidates" placeholder="留空使用默认字体" />
        </div>
        <div class="field">
          <label>代码块字体 (codeblock)</label>
          <input v-model="data.style.font_family.codeblock" :class="inputCls" list="font-family-candidates" placeholder="留空使用默认字体" />
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
        <label>背景色</label>
        <div class="field-inline">
          <input type="color" v-model="data.style.leftbar.background_color_dark" class="apple-color" />
          <input v-model="data.style.leftbar.background_color_dark" :class="inputCls" />
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
              type="range" min="0" max="20" step="1" class="apple-range"
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
        <label>高亮主题 URL (scrollbar)</label>
        <input v-model="data.style.codeblock.scrollbar" :class="inputCls" placeholder="代码块高亮主题 CSS URL" />
      </div>
    </div>

    <!-- 7. 杂项 -->
    <div class="subsection">
      <p class="subsection-title">杂项</p>
      <div class="field">
        <label>标题前缀 (header_prefix)</label>
        <p class="hint">各标题级别前缀字符（如 h2="#"）；非 h2-h5 的存量键保留不动，可在「高级 JSON」兜底中查看。</p>
        <div class="form-grid form-grid-2">
          <div class="field"><label>h2</label><input v-model="data.style.header_prefix.h2" :class="inputCls" placeholder="如 #" /></div>
          <div class="field"><label>h3</label><input v-model="data.style.header_prefix.h3" :class="inputCls" placeholder="如 =" /></div>
          <div class="field"><label>h4</label><input v-model="data.style.header_prefix.h4" :class="inputCls" placeholder="如 ~" /></div>
          <div class="field"><label>h5</label><input v-model="data.style.header_prefix.h5" :class="inputCls" placeholder="如 ^" /></div>
        </div>
      </div>
    </div>

    <!-- 8. 布局与效果 -->
    <div class="subsection">
      <p class="subsection-title">布局与效果</p>
      <div class="switch-row">
        <div class="switch-row-label">
          <div class="switch-row-name">平滑滚动 (smooth_scroll)</div>
          <div class="switch-row-desc">关闭后页面锚点/返回顶部为瞬移（默认开启）</div>
        </div>
        <VSwitch v-model="smoothScrollOn" />
      </div>
      <div class="form-grid form-grid-2">
        <div class="field">
          <label>正文对齐 (text_align)</label>
          <select v-model="data.style.text_align" class="apple-select">
            <option value="">默认（left）</option>
            <option value="left">left</option>
            <option value="justify">justify（两端对齐）</option>
          </select>
        </div>
      </div>
    </div>

    <!-- 9. 圆角 -->
    <div class="subsection">
      <p class="subsection-title">圆角 (border_radius)</p>
      <p class="hint">四档圆角，留空回退主题内置值；值为 CSS 长度，如 16px。</p>
      <div class="form-grid form-grid-2">
        <div class="field"><label>卡片 (card)</label><input v-model="data.style.border_radius.card" :class="inputCls" placeholder="16px" /></div>
        <div class="field"><label>块级元素 (block)</label><input v-model="data.style.border_radius.block" :class="inputCls" placeholder="8px" /></div>
        <div class="field"><label>导航栏 (navbar)</label><input v-model="data.style.border_radius.navbar" :class="inputCls" placeholder="100px" /></div>
        <div class="field"><label>图片 (image)</label><input v-model="data.style.border_radius.image" :class="inputCls" placeholder="8px" /></div>
      </div>
    </div>

    <!-- 10. 渐变 -->
    <div class="subsection">
      <p class="subsection-title">渐变 (gradient)</p>
      <p class="hint">任意 CSS gradient 表达式，留空回退主题内置值。</p>
      <div class="form-grid form-grid-2">
        <div class="field"><label>按钮背景 (button)</label><input v-model="data.style.gradient.button" :class="inputCls" placeholder="linear-gradient(...)" /></div>
        <div class="field"><label>搜索栏背景 (searchbar)</label><input v-model="data.style.gradient.searchbar" :class="inputCls" placeholder="linear-gradient(...)" /></div>
      </div>
    </div>

    <!-- 11. 分页箭头 -->
    <div class="subsection">
      <p class="subsection-title">分页箭头 (paginator)</p>
      <p class="hint">自定义分页器 prev/next 箭头图 URL，留空用主题内置箭头。</p>
      <div class="form-grid form-grid-2">
        <div class="field"><label>上一篇箭头 (prev)</label><AttachmentField v-model="data.style.paginator.prev" placeholder="图片 URL" /></div>
        <div class="field"><label>下一篇箭头 (next)</label><AttachmentField v-model="data.style.paginator.next" placeholder="图片 URL" /></div>
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
