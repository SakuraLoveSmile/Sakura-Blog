<script setup lang="ts">
import { ref, watch, computed } from"vue";
import { Icon } from"@iconify/vue";

// 前台文案覆盖表（settings lang 段）：key → 文案，增量覆盖内置 zhCN。
// 表单用行数组代理，变更时同步回 data（保存提交 data.lang）。
const props = defineProps<{
 data: any;
 saving: string;
}>();

const emit = defineEmits<{ (e:"save"): void }>();

interface LangRow {
 key: string;
 value: string;
}

const inputCls = "apple-input";
const btnCls =
"px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg border border-secondary-700/30 hover:opacity-90 disabled:opacity-50 shadow-sm";
const addBtnCls =
"inline-flex items-center gap-1 text-sm text-secondary hover:text-primary transition-colors text-primary hover:text-primary/80";

// 常用内置 key 参考（与 backend/internal/template/lang.go 的 zhCN 分组一致），
// 帮助站长定位可覆盖文案；点击填入 key。
const COMMON_KEYS: [string, string][] = [
 ["btn.home","主页"],
 ["btn.recent_publish","近期发布"],
 ["btn.categories","分类"],
 ["btn.tags","标签"],
 ["btn.archives","归档"],
 ["btn.topic","专栏"],
 ["btn.all","全部"],
 ["btn.edit","编辑本文"],
 ["meta.recent_update","最近更新"],
 ["meta.toc","本文目录"],
 ["meta.read_next","接下来阅读"],
 ["meta.related_posts","您可能感兴趣的文章"],
 ["meta.license","许可协议"],
 ["meta.share","分享文章"],
 ["meta.contributors","本文贡献者"],
 ["date_suffix.just","刚刚"],
 ["search.search","站内搜索"],
 ["search.no_result","没有找到内容！"],
 ["message.copied","复制成功"],
 ["article.expire","本文最后更新于 %d 天前…"],
 ["article.copyright","版权声明"],
 ["article.reward","打赏"],
 ["share.wechat","微信分享"],
 ["share.copy_link","复制链接"],
 ["comment.reply","回复"],
 ["comment.placeholder","编写评论，支持 Markdown"],
 ["comment.submit","发布评论"],
 ["comment.count","%d 条评论"],
 ["comment.no_comment","暂无评论"],
 ["page.error.what","很抱歉，您访问的页面不存在"],
 ["page.error.action","返回主页"],
 ["error500.what","很抱歉，服务器遇到了内部错误"],
 ["notes.empty","暂无内容"],
 ["moments.empty","还没有瞬间，敬请期待。"],
 ["friend.empty","暂无友链"],
 ["wiki.empty","暂无文库项目"],
 ["widget.siteinfo","站点统计"],
 ["widget.recent","最近更新"],
 ["widget.popular","热门文章"],
 ["widget.welcome","欢迎语正文"],
 ["password.title","需要口令"],
 ["password.submit","解锁阅读"],
 ["ai.summary.title","AI 摘要"],
];

const rows = ref<LangRow[]>([]);
const filterText = ref("");

const filtered = computed(() => {
 const q = filterText.value.trim().toLowerCase();
 if (!q) return rows.value;
 return rows.value.filter((r) => r.key.toLowerCase().includes(q) || r.value.includes(q));
});

function syncBack() {
 const m: Record<string, string> = {};
 for (const r of rows.value) {
 if (!r.key) continue;
 m[r.key] = r.value;
 }
 Object.keys(props.data).forEach((k) => delete props.data[k]);
 Object.assign(props.data, m);
}

watch(rows, syncBack, { deep: true });

// 初始化：从覆盖表展开为行
rows.value = Object.entries(props.data || {}).map(([key, v]) => ({ key, value: String(v ||"") }));

function addRow() {
 rows.value.push({ key:"", value:"" });
}
function removeRow(i: number) {
 rows.value.splice(i, 1);
}
function pickKey(key: string) {
 // 已存在则聚焦现有行，否则追加新行
 const hit = rows.value.find((r) => r.key === key);
 if (!hit) rows.value.push({ key, value:"" });
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
 前台文案覆盖表：填写 key 与文案即覆盖内置文案（如 btn.home → 主页），留空保存清空覆盖。
 支持 %s / %d 占位符（与内置文案格式一致）。
 </p>

 <div class="flex flex-wrap gap-1.5 max-h-40 overflow-y-auto border border-border rounded-xl p-3">
 <button
 v-for="[k, v] in COMMON_KEYS"
 :key="k"
 @click="pickKey(k)"
 class="text-xs px-2 py-1 rounded-full bg-secondary text-text-tertiary hover:bg-primary/10 hover:text-secondary hover:text-primary transition-colors"
 :title="v"
 >
 {{ k }}
 </button>
 </div>

 <div class="flex items-center gap-2">
 <input v-model="filterText" :class="inputCls" placeholder="搜索 key / 文案…" />
 </div>

 <div class="space-y-2">
 <div v-for="(r, i) in filtered" :key="i" class="grid grid-cols-12 gap-2 items-center">
 <input v-model="r.key" placeholder="key（如 btn.home）" :class="inputCls + ' font-mono text-xs'" class="col-span-4" />
 <input v-model="r.value" placeholder="覆盖文案" :class="inputCls" class="col-span-6" />
 <div class="col-span-2 flex items-center gap-1.5 justify-end">
 <button @click="removeRow(Number(i))" class="text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
 </div>
 </div>
 <button @click="addRow" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加覆盖项</button>
 </div>

 <div><button @click="doSave" :disabled="saving === 'lang'" :class="btnCls">保存文案</button></div>
 </div>
</template>
