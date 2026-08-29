<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VButton, VCard } from "@/components/ui";
import PluginsTab from "@/modules/settings/PluginsTab.vue";
import IconPicker from "@/components/IconPicker.vue";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const colors = ["red", "orange", "yellow", "green", "cyan", "blue", "purple"];
const inputCls = "apple-input";

function saveTagPlugins() { emit("save", "tag_plugins"); }
</script>

<template>
  <VCard title="前端增强"><PluginsTab :data="data.plugins" :saving="saving" @save="emit('save', 'plugins')" /></VCard>
  <VCard title="内容组件默认参数">
    <p class="settings-hint">对齐 Stellar tag_plugins 段：未在具体标签中指定的参数使用这里的默认值；留空=内置默认。</p>
    <div class="settings-subsection settings-subsection-first"><p class="settings-subsection-title">默认配色</p><div class="settings-form-grid settings-form-grid-3"><div v-for="item in [['note','note 提示框'],['mark','mark 荧光标记'],['button','button 按钮'],['icon','icon 图标'],['hashtag','hashtag 话题']]" :key="item[0]" class="settings-field"><label>{{ item[1] }}</label><select v-model="data.tag_plugins[item[0]].default_color" class="apple-select"><option value="">{{ item[0] === 'hashtag' ? '循环' : '默认' }}</option><option v-for="color in colors" :key="color" :value="color">{{ color }}</option></select></div></div></div>
    <div class="settings-subsection"><p class="settings-subsection-title">quot 小标题默认图标</p><div class="settings-form-grid settings-form-grid-2"><div class="settings-field"><label>前缀</label><IconPicker v-model="data.tag_plugins.quot.prefix" placeholder="如 quot:quote-left" /></div><div class="settings-field"><label>后缀</label><IconPicker v-model="data.tag_plugins.quot.suffix" placeholder="如 quot:quote-right" /></div></div></div>
    <div class="settings-subsection"><p class="settings-subsection-title">gallery 默认布局</p><div class="settings-form-grid settings-form-grid-3"><div class="settings-field"><label>布局</label><select v-model="data.tag_plugins.gallery.layout" class="apple-select"><option value="">默认</option><option value="grid">grid</option><option value="flow">flow</option></select></div><div class="settings-field"><label>尺寸</label><input v-model="data.tag_plugins.gallery.size" :class="inputCls" placeholder="mix" /></div><div class="settings-field"><label>宽高比</label><input v-model="data.tag_plugins.gallery.ratio" :class="inputCls" placeholder="origin" /></div></div></div>
    <div class="settings-subsection"><p class="settings-subsection-title">其它</p><div class="settings-form-grid settings-form-grid-2"><div class="settings-field"><label>timeline 限高（如 80vh，空=不限）</label><input v-model="data.tag_plugins.timeline.max_height" :class="inputCls" /></div><div class="settings-field"><label>copy 成功提示文案</label><input v-model="data.tag_plugins.copy.toast" :class="inputCls" placeholder="复制成功" /></div></div></div>
    <div class="settings-subsection"><p class="settings-subsection-title">emoji 表情源（{name} 占位）</p><div class="settings-form-grid settings-form-grid-2"><div v-for="name in ['default','aru','tieba','blobcat','twemoji']" :key="name" class="settings-field"><label>{{ name === 'default' ? 'default / qq' : name }}</label><input v-model="data.tag_plugins.emoji[name]" :class="inputCls" /></div></div></div>
    <div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'tag_plugins'" @click="saveTagPlugins"><template #icon><Icon icon="ri:save-line" /></template>保存内容组件设置</VButton></div>
  </VCard>
</template>
