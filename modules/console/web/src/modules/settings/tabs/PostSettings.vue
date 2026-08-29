<script setup lang="ts">
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import IconPicker from "@/components/IconPicker.vue";
import { VButton, VCard, VSwitch } from "@/components/ui";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";
const selectCls = "apple-select";
const platformOptions = ["wechat", "weibo", "email", "link", "twitter", "telegram"];

function togglePlatform(platform: string) {
  const platforms = props.data.article.share.platforms as string[];
  const index = platforms.indexOf(platform);
  if (index >= 0) platforms.splice(index, 1);
  else platforms.push(platform);
}

function addMapEntry(target: "category_color" | "category_icon" | "tag_icon") {
  props.data.article[target] = props.data.article[target] || {};
  let key = "新名称";
  let index = 2;
  while (Object.prototype.hasOwnProperty.call(props.data.article[target], key)) key = `新名称${index++}`;
  props.data.article[target][key] = "";
}

function renameMapEntry(target: "category_color" | "category_icon" | "tag_icon", oldKey: string, event: Event) {
  const nextKey = (event.target as HTMLInputElement).value;
  if (nextKey === oldKey) return;
  const map = props.data.article[target] || {};
  const value = map[oldKey];
  delete map[oldKey];
  map[nextKey || oldKey] = value;
}

function removeMapEntry(target: "category_color" | "category_icon" | "tag_icon", key: string) {
  delete props.data.article[target]?.[key];
}
</script>

<template>
  <VCard title="文章设置">
    <div class="settings-subsection settings-subsection-first">
      <p class="settings-subsection-title">排版（对齐原版 article 段）</p>
      <div class="settings-switch-row"><div><div class="settings-switch-name">正文段落首行缩进两字符</div></div><VSwitch v-model="data.article.indent" /></div>
      <div class="settings-switch-row"><div><div class="settings-switch-name">上下篇导航（read_next）</div><div class="settings-switch-desc">文章页底部「接下来阅读」较新/较早文章卡片</div></div><VSwitch v-model="data.article.read_next" /></div>
      <div class="settings-form-grid settings-form-grid-3">
        <div class="settings-field"><label>置顶文章样式</label><select v-model="data.article.pin_style" :class="selectCls"><option value="carousel">轮播</option><option value="flat">平铺到列表顶部</option></select></div>
        <div class="settings-switch-row"><div><div class="settings-switch-name">显示字数与阅读时长</div></div><VSwitch v-model="data.article.reading_time" /></div>
        <div class="settings-switch-row"><div><div class="settings-switch-name">文章卡片显示标签</div><div class="settings-switch-desc">最多显示 5 个</div></div><VSwitch v-model="data.article.card_tags" /></div>
        <div class="settings-switch-row"><div><div class="settings-switch-name">文章末尾显示标签</div><div class="settings-switch-desc">正文后、页脚前的标签行</div></div><VSwitch v-model="data.article.tags" /></div>
      </div>
      <div class="settings-form-grid settings-form-grid-3">
        <div class="settings-field"><label>卡片封面宽高比（0=主题默认，如 2）</label><input v-model.number="data.article.cover_ratio" type="number" step="0.1" min="0" :class="inputCls" /></div>
        <div class="settings-field"><label>相关文章数上限（0=关）</label><input v-model.number="data.article.related_posts" type="number" min="0" :class="inputCls" /></div>
        <div class="settings-field"><label>过期提醒阈值（天，0=缺省 180）</label><input v-model.number="data.article.expire_days" type="number" min="0" :class="inputCls" /></div>
      </div>
      <div class="settings-form-grid settings-form-grid-3">
        <div class="settings-field"><label>文章布局类型</label><select v-model="data.article.type" :class="selectCls"><option value="tech">tech（技术）</option><option value="story">story（图文）</option></select></div>
        <div class="settings-field"><label>文首横幅宽高比</label><input v-model.number="data.article.banner_ratio" type="number" step="0.1" min="0" :class="inputCls" placeholder="2.5" /></div>
        <div class="settings-field"><label>自动摘要长度（字符，0=关闭）</label><input v-model.number="data.article.auto_excerpt" type="number" min="0" :class="inputCls" placeholder="128" /></div>
        <div class="settings-field"><label>有封面时的文章卡片样式</label><select v-model="data.article.card_style" :class="selectCls"><option value="hero">hero（全图文字）</option><option value="classic">classic（摘要卡）</option></select></div>
      </div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">分类与标签映射</p>
      <p class="settings-hint">分类颜色使用 CSS 颜色；图标字段可输入图标 key 或从图标库选择。</p>
      <div v-for="target in ['category_color', 'category_icon', 'tag_icon'] as const" :key="target" class="settings-mapping-block">
        <div class="settings-mapping-head"><span>{{ target === 'category_color' ? '分类颜色' : target === 'category_icon' ? '分类图标' : '标签图标' }}</span><button type="button" class="settings-add-btn" @click="addMapEntry(target)"><Icon icon="ri:add-line" />添加</button></div>
        <div v-for="key in Object.keys(data.article[target])" :key="key" class="settings-editable-row">
          <input :value="key" :class="inputCls" class="settings-col-4" placeholder="名称" @change="renameMapEntry(target, key, $event)" />
          <input v-if="target === 'category_color'" v-model="data.article[target][key]" :class="inputCls" class="settings-col-6" placeholder="#1BCDFC" />
          <IconPicker v-else v-model="data.article[target][key]" compact placeholder="图标 key" class="settings-col-6" />
          <button type="button" class="settings-row-btn settings-row-btn-danger" title="删除" @click="removeMapEntry(target, key)"><Icon icon="ri:delete-bin-line" /></button>
        </div>
      </div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">版权声明</p>
      <div class="settings-switch-row"><div><div class="settings-switch-name">显示版权声明</div></div><VSwitch v-model="data.article.copyright.enabled" /></div>
      <div class="settings-form-grid settings-form-grid-2"><div class="settings-field"><label>协议名</label><input v-model="data.article.copyright.license" :class="inputCls" /></div><div class="settings-field"><label>协议链接</label><input v-model="data.article.copyright.license_url" :class="inputCls" /></div></div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">分享平台</p>
      <div class="settings-chip-group"><label v-for="platform in platformOptions" :key="platform" class="settings-chip" :class="{ active: data.article.share.platforms.includes(platform) }"><input type="checkbox" :checked="data.article.share.platforms.includes(platform)" @change="togglePlatform(platform)" /><span>{{ platform }}</span></label></div>
      <div class="settings-switch-row"><div><div class="settings-switch-name">启用分享</div></div><VSwitch v-model="data.article.share.enabled" /></div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">打赏</p>
      <div class="settings-switch-row"><div><div class="settings-switch-name">启用打赏</div></div><VSwitch v-model="data.article.reward.enabled" /></div>
      <div class="settings-form-grid settings-form-grid-3"><div class="settings-field"><label>按钮文案</label><input v-model="data.article.reward.text" :class="inputCls" /></div><div class="settings-field"><label>微信收款码 URL</label><AttachmentField v-model="data.article.reward.wechat" /></div><div class="settings-field"><label>支付宝收款码 URL</label><AttachmentField v-model="data.article.reward.alipay" /></div></div>
    </div>

    <div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'article'" @click="emit('save', 'article')"><template #icon><Icon icon="ri:save-line" /></template>保存文章设置</VButton></div>
  </VCard>
</template>
