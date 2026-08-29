<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VButton, VCard, VSwitch } from "@/components/ui";

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
</script>

<template>
  <VCard title="搜索设置">
    <p class="settings-hint">对齐 Stellar search.local_search；修改后会立即使本地索引失效并重建。</p>
    <div class="settings-form-grid settings-form-grid-3"><div class="settings-field"><label>搜索服务</label><select v-model="data.search.service" class="apple-select"><option value="local_search">local_search</option><option value="todo">todo</option></select></div><div class="settings-field"><label>索引范围</label><select v-model="data.search.local_search.field" class="apple-select"><option value="all">全部</option><option value="post">文章</option><option value="page">页面</option></select></div><div class="settings-field"><label>索引路径</label><input v-model="data.search.local_search.path" :class="inputCls" placeholder="/search.json" /></div></div>
    <div class="settings-switch-row"><div><div class="settings-switch-name">搜索正文内容</div><div class="settings-switch-desc">关闭后仍可搜索标题，但不会把正文写入索引</div></div><VSwitch v-model="data.search.local_search.content" /></div>
    <div class="settings-field"><label>排除路径（每行一个）</label><textarea :value="data.search.local_search.skip_search.join('\n')" rows="4" :class="textareaCls" placeholder="/post/private" @input="updateSkipSearch"></textarea></div>
    <div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'search'" @click="emit('save', 'search')"><template #icon><Icon icon="ri:save-line" /></template>保存搜索设置</VButton></div>
  </VCard>
</template>
