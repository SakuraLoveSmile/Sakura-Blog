<script setup lang="ts">
import { Icon } from "@iconify/vue";
import IconPicker from "@/components/IconPicker.vue";
import { VButton, VCard } from "@/components/ui";
import WidgetsTab from "@/modules/settings/WidgetsTab.vue";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const inputCls = "apple-input";

function moveItem(array: any[], index: number, direction: -1 | 1) {
  const target = index + direction;
  if (target < 0 || target >= array.length) return;
  [array[index], array[target]] = [array[target], array[index]];
}
function removeAt(array: any[], index: number) { array.splice(index, 1); }
function addRightbarLink() { props.data.rightbar.welcome_linklist.items.push({ title: "", url: "", icon: "" }); }
function addSocial() { props.data.footer.social.push({ title: "", url: "", icon: "", icon_key: "" }); }
function addSitemapGroup() { props.data.footer.sitemap.push({ title: "", items: [] }); }
function addSitemapItem(group: any) { group.items = group.items || []; group.items.push({ title: "", url: "" }); }
</script>

<template>
  <VCard title="右栏与页脚">
    <section>
      <h3 class="settings-subsection-title">右栏设置</h3>
      <div class="settings-field"><label>欢迎语正文（留空用默认文案）</label><textarea v-model="data.rightbar.intro" rows="3" class="apple-textarea" placeholder="输入右栏欢迎语正文，换行会保留显示"></textarea></div>
      <p class="settings-hint">首页右栏「欢迎语」组件内的链接列表（welcome_linklist）。</p>
      <div class="settings-field" style="max-width: 160px"><label>列数（0=自动）</label><input v-model.number="data.rightbar.welcome_linklist.columns" type="number" min="0" :class="inputCls" /></div>
      <div class="settings-editable-list"><div v-for="(item, index) in data.rightbar.welcome_linklist.items" :key="index" class="settings-editable-row"><input v-model="item.title" placeholder="标题" :class="inputCls" class="settings-col-3" /><input v-model="item.url" placeholder="/url 或 https://..." :class="inputCls" class="settings-col-4" /><IconPicker v-model="item.icon" compact placeholder="图标 key 或图片 URL" class="settings-col-3" /><div class="settings-row-actions"><button type="button" class="settings-row-btn" :disabled="Number(index) === 0" @click="moveItem(data.rightbar.welcome_linklist.items, Number(index), -1)"><Icon icon="ri:arrow-up-s-line" /></button><button type="button" class="settings-row-btn" :disabled="Number(index) === data.rightbar.welcome_linklist.items.length - 1" @click="moveItem(data.rightbar.welcome_linklist.items, Number(index), 1)"><Icon icon="ri:arrow-down-s-line" /></button><button type="button" class="settings-row-btn settings-row-btn-danger" @click="removeAt(data.rightbar.welcome_linklist.items, Number(index))"><Icon icon="ri:delete-bin-line" /></button></div></div></div>
      <button type="button" class="settings-add-btn" @click="addRightbarLink"><Icon icon="ri:add-line" />添加链接</button>
      <div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'rightbar'" @click="emit('save', 'rightbar')">保存右栏</VButton></div>
    </section>
    <section class="settings-subsection"><h3 class="settings-subsection-title">页脚</h3><p class="settings-hint">社交图标（icon 填 raw HTML，icon_key 填 icons.yml 的 key，二选一）。</p><div class="settings-editable-list"><div v-for="(social, index) in data.footer.social" :key="index" class="settings-editable-row"><input v-model="social.title" placeholder="标题" :class="inputCls" class="settings-col-2" /><input v-model="social.url" placeholder="链接" :class="inputCls" class="settings-col-3" /><IconPicker v-model="social.icon_key" compact placeholder="icon_key" class="settings-col-2" /><input v-model="social.icon" placeholder="icon raw HTML（优先）" :class="inputCls" class="settings-col-3" /><div class="settings-row-actions"><button type="button" class="settings-row-btn" :disabled="Number(index) === 0" @click="moveItem(data.footer.social, Number(index), -1)"><Icon icon="ri:arrow-up-s-line" /></button><button type="button" class="settings-row-btn" :disabled="Number(index) === data.footer.social.length - 1" @click="moveItem(data.footer.social, Number(index), 1)"><Icon icon="ri:arrow-down-s-line" /></button><button type="button" class="settings-row-btn settings-row-btn-danger" @click="removeAt(data.footer.social, Number(index))"><Icon icon="ri:delete-bin-line" /></button></div></div></div><button type="button" class="settings-add-btn" @click="addSocial"><Icon icon="ri:add-line" />添加社交图标</button><div class="settings-subsection"><p class="settings-subsection-title">站点地图分组</p><div v-for="(group, groupIndex) in data.footer.sitemap" :key="groupIndex" class="settings-nested-group"><div class="settings-nested-head"><input v-model="group.title" placeholder="组名" :class="inputCls" /><button type="button" class="settings-row-btn" @click="removeAt(data.footer.sitemap, Number(groupIndex))"><Icon icon="ri:delete-bin-line" /></button></div><div v-for="(item, itemIndex) in group.items" :key="itemIndex" class="settings-editable-row"><input v-model="item.title" placeholder="标题" :class="inputCls" class="settings-col-4" /><input v-model="item.url" placeholder="/url" :class="inputCls" class="settings-col-6" /><div class="settings-row-actions"><button type="button" class="settings-row-btn" :disabled="Number(itemIndex) === 0" @click="moveItem(group.items, Number(itemIndex), -1)"><Icon icon="ri:arrow-up-s-line" /></button><button type="button" class="settings-row-btn" :disabled="Number(itemIndex) === group.items.length - 1" @click="moveItem(group.items, Number(itemIndex), 1)"><Icon icon="ri:arrow-down-s-line" /></button><button type="button" class="settings-row-btn settings-row-btn-danger" @click="removeAt(group.items, Number(itemIndex))"><Icon icon="ri:delete-bin-line" /></button></div></div><button type="button" class="settings-add-btn" @click="addSitemapItem(group)"><Icon icon="ri:add-line" />添加链接</button></div><button type="button" class="settings-add-btn" @click="addSitemapGroup"><Icon icon="ri:add-line" />添加分组</button></div><div class="settings-field"><label>ICP 备案号（beian）</label><input v-model="data.footer.beian" :class="inputCls" placeholder="如 京ICP备2024xxxxxx号-1，留空不渲染" /></div><div class="settings-field"><label>自定义页脚文案（Markdown，空=内置 Powered-by 行）</label><textarea v-model="data.footer.content" rows="3" class="apple-textarea"></textarea></div><div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'footer'" @click="emit('save', 'footer')">保存页脚</VButton></div></section>
  </VCard>
  <VCard title="组件编排"><WidgetsTab :data="data" :saving="saving" @save="emit('save', $event)" /></VCard>
</template>
