<script setup lang="ts">
import { ref, watch } from"vue";
import { Icon } from"@iconify/vue";

// 多作者体系（site.yml authors 段入库版）：key → {name, avatar, description, url, hidden}。
// 段是 map 结构，表单用行数组代理，变更时同步回 data（保存提交 data.authors）。
const props = defineProps<{
 data: any;
 saving: string;
}>();

const emit = defineEmits<{ (e:"save"): void }>();

interface AuthorRow {
 key: string;
 name: string;
 avatar: string;
 description: string;
 url: string;
 hidden: boolean;
}

const inputCls = "apple-input";
const btnCls =
"px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg border border-secondary-700/30 hover:opacity-90 disabled:opacity-50 shadow-sm";
const addBtnCls =
"inline-flex items-center gap-1 text-sm text-secondary hover:text-primary transition-colors text-primary hover:text-primary/80";

const rows = ref<AuthorRow[]>([]);

function syncBack() {
 const m: Record<string, any> = {};
 for (const r of rows.value) {
 if (!r.key) continue;
 m[r.key] = {
 name: r.name,
 avatar: r.avatar,
 description: r.description,
 url: r.url,
 hidden: r.hidden,
 };
 }
 // 就地替换对象内容（保持 data.authors 引用不变）
 Object.keys(props.data).forEach((k) => delete props.data[k]);
 Object.assign(props.data, m);
}

watch(rows, syncBack, { deep: true });

// 初始化：从 map 展开为行
rows.value = Object.entries(props.data || {}).map(([key, v]: [string, any]) => ({
 key,
 name: v?.name ||"",
 avatar: v?.avatar ||"",
 description: v?.description ||"",
 url: v?.url ||"",
 hidden: !!v?.hidden,
}));

function addRow() {
 rows.value.push({ key:"", name:"", avatar:"", description:"", url:"", hidden: false });
}
function removeRow(i: number) {
 rows.value.splice(i, 1);
}

// watch 为异步 flush，保存前强制同步回 data，避免提交旧值
function doSave() {
 syncBack();
 emit("save");
}
</script>

<template>
 <div class="space-y-4">
 <p class="text-xs text-text-quaternary">
 多作者体系：key 为作者 ID（文章 front matter 的 author 字段引用），name 非空即生效；留空站点回退站长。
 </p>
 <div
 v-for="(r, i) in rows"
 :key="i"
 class="border border-border rounded-xl p-4 space-y-3"
 >
 <div class="flex items-center gap-3">
 <Icon icon="ri:user-3-line" class="text-text-quaternary text-lg shrink-0" />
 <span class="flex-1 text-sm font-medium text-text-secondary truncate">
 {{ r.name || r.key ||"新作者" }}
 </span>
 <label class="flex items-center gap-1.5 text-xs text-text-tertiary">
 <input type="checkbox" v-model="r.hidden" class="rounded" /> 隐藏
 </label>
 <div class="flex items-center gap-1.5 shrink-0">
 <button @click="removeRow(Number(i))" class="text-state-error hover:text-state-error text-state-error hover:text-state-error" title="删除"><Icon icon="ri:delete-bin-line" /></button>
 </div>
 </div>
 <div class="grid grid-cols-3 gap-3">
 <div><label class="block text-xs text-text-tertiary mb-1">作者 ID（key）</label><input v-model="r.key" :class="inputCls" placeholder="sakura" /></div>
 <div><label class="block text-xs text-text-tertiary mb-1">显示名</label><input v-model="r.name" :class="inputCls" placeholder="Sakura" /></div>
 <div><label class="block text-xs text-text-tertiary mb-1">主页链接</label><input v-model="r.url" :class="inputCls" placeholder="https://..." /></div>
 </div>
 <div class="grid grid-cols-2 gap-3">
 <div><label class="block text-xs text-text-tertiary mb-1">头像 URL</label><input v-model="r.avatar" :class="inputCls" placeholder="https://..." /></div>
 <div><label class="block text-xs text-text-tertiary mb-1">简介</label><input v-model="r.description" :class="inputCls" placeholder="博客作者" /></div>
 </div>
 </div>
 <button @click="addRow" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加作者</button>
 <div><button @click="doSave" :disabled="saving === 'authors'" :class="btnCls">保存作者</button></div>
 </div>
</template>
