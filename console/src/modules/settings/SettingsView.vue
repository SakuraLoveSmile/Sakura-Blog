<script setup lang="ts">
import { ref, onMounted } from "vue";
import { settingsApi } from "@/api";
import { Icon } from "@iconify/vue";

// 结构化表单段：site / menubar / comments / article；其余段用 JSON 编辑器（保真、少出错）。
const tabs = [
  { key: "site", label: "站点" },
  { key: "menubar", label: "菜单" },
  { key: "comments", label: "评论" },
  { key: "article", label: "文章" },
  { key: "footer", label: "页脚·JSON" },
  { key: "advanced", label: "高级·JSON" },
];
const jsonSections = ["footer", "rightbar", "wiki", "friends"];
const platformOptions = ["wechat", "weibo", "email", "link", "twitter", "telegram"];

const active = ref("site");
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(false);
const savingKey = ref("");
const message = ref("");
const isError = ref(false);

function ensureDefaults(d: Record<string, any>) {
  d.site = d.site || {};
  d.menubar = d.menubar || {};
  d.menubar.items = d.menubar.items || [];
  d.comments = d.comments || {};
  d.comments.notify = d.comments.notify || {};
  d.article = d.article || {};
  d.article.copyright = d.article.copyright || {};
  d.article.reward = d.article.reward || {};
  d.article.share = d.article.share || {};
  d.article.share.platforms = d.article.share.platforms || [];
  return d;
}

async function load() {
  loading.value = true;
  try {
    const { data: res } = await settingsApi.all();
    data.value = ensureDefaults({ ...res });
    for (const s of jsonSections) {
      rawText.value[s] = JSON.stringify(res[s] ?? null, null, 2);
    }
  } finally {
    loading.value = false;
  }
}

async function save(section: string) {
  savingKey.value = section;
  message.value = "";
  isError.value = false;
  try {
    const payload = jsonSections.includes(section)
      ? JSON.parse(rawText.value[section])
      : data.value[section];
    await settingsApi.updateSection(section, payload);
    message.value = "已保存，前台已生效";
  } catch (e: any) {
    isError.value = true;
    message.value =
      e?.response?.data?.error || e?.message || "保存失败（检查 JSON 格式）";
  } finally {
    savingKey.value = "";
  }
}

function togglePlatform(p: string) {
  const arr = data.value.article.share.platforms as string[];
  const i = arr.indexOf(p);
  if (i >= 0) arr.splice(i, 1);
  else arr.push(p);
}
function addMenuItem() {
  data.value.menubar.items.push({ id: "", title: "", url: "", icon: "", theme: "" });
}
function removeMenuItem(i: number) {
  data.value.menubar.items.splice(i, 1);
}

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500";

onMounted(load);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800">站点设置</h2>
      <span v-if="message" :class="['text-sm px-3 py-1.5 rounded-lg', isError ? 'text-red-600 bg-red-50' : 'text-green-700 bg-green-50']">{{ message }}</span>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 border-b border-gray-200 mb-6">
      <button v-for="t in tabs" :key="t.key" @click="active = t.key"
        :class="['px-4 py-2 text-sm font-medium -mb-px border-b-2 transition-colors', active === t.key ? 'border-indigo-600 text-indigo-700' : 'border-transparent text-gray-500 hover:text-gray-800']">
        {{ t.label }}
      </button>
    </div>

    <div v-if="loading" class="text-sm text-gray-400 py-12 text-center">加载中...</div>

    <div v-else class="max-w-3xl">
      <!-- 站点 -->
      <div v-show="active === 'site'" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">站点标题</label><input v-model="data.site.title" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">副标题</label><input v-model="data.site.subtitle" :class="inputCls" /></div>
        </div>
        <div><label class="block text-xs text-gray-500 mb-1">站点描述</label><textarea v-model="data.site.description" rows="2" :class="inputCls"></textarea></div>
        <div><label class="block text-xs text-gray-500 mb-1">站点 URL（canonical/OG/sitemap 用）</label><input v-model="data.site.url" :class="inputCls" placeholder="https://example.com" /></div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">头像 URL</label><input v-model="data.site.avatar" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">头像悬停背景图</label><input v-model="data.site.avatar_bg" :class="inputCls" /></div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">Logo 主标题</label><input v-model="data.site.logo_title" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">Logo 副标题（含 | 拆滚动）</label><input v-model="data.site.logo_sub" :class="inputCls" /></div>
        </div>
        <button @click="save('site')" :disabled="savingKey === 'site'" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">保存站点信息</button>
      </div>

      <!-- 菜单 -->
      <div v-show="active === 'menubar'" class="space-y-4">
        <div class="w-40"><label class="block text-xs text-gray-500 mb-1">菜单列数</label><input v-model.number="data.menubar.columns" type="number" min="1" :class="inputCls" /></div>
        <div class="space-y-2">
          <div v-for="(it, i) in data.menubar.items" :key="i" class="grid grid-cols-12 gap-2 items-center">
            <input v-model="it.id" placeholder="id" :class="inputCls" class="col-span-2" />
            <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-span-2" />
            <input v-model="it.url" placeholder="/url" :class="inputCls" class="col-span-3" />
            <input v-model="it.icon" placeholder="icon key" :class="inputCls" class="col-span-3" />
            <input v-model="it.theme" placeholder="#色值" :class="inputCls" class="col-span-1" />
            <button @click="removeMenuItem(Number(i))" class="col-span-1 text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
          </div>
        </div>
        <button @click="addMenuItem" class="inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-800"><Icon icon="ri:add-line" /> 添加菜单项</button>
        <div><button @click="save('menubar')" :disabled="savingKey === 'menubar'" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">保存菜单</button></div>
      </div>

      <!-- 评论 -->
      <div v-show="active === 'comments'" class="space-y-4">
        <label class="flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" v-model="data.comments.enabled" /> 启用评论（关闭需重启才断开路由）</label>
        <label class="flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" v-model="data.comments.moderation" /> 先审后发</label>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">评论区标题</label><input v-model="data.comments.title" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">博主徽章文案</label><input v-model="data.comments.admin_badge" :class="inputCls" /></div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">博主邮箱</label><input v-model="data.comments.admin_email" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">Gravatar 镜像</label><input v-model="data.comments.gravatar_mirror" :class="inputCls" /></div>
        </div>
        <p class="text-xs text-gray-400 pt-2">邮件通知（SMTP，主机留空则关闭）</p>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">SMTP 主机</label><input v-model="data.comments.notify.smtp_host" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">端口</label><input v-model.number="data.comments.notify.smtp_port" type="number" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">用户名</label><input v-model="data.comments.notify.username" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">密码</label><input v-model="data.comments.notify.password" type="password" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">发件人（留空用用户名）</label><input v-model="data.comments.notify.from" :class="inputCls" /></div>
        </div>
        <button @click="save('comments')" :disabled="savingKey === 'comments'" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">保存评论设置</button>
      </div>

      <!-- 文章 -->
      <div v-show="active === 'article'" class="space-y-4">
        <p class="text-xs text-gray-400">版权声明</p>
        <label class="flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" v-model="data.article.copyright.enabled" /> 显示版权声明</label>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">协议名</label><input v-model="data.article.copyright.license" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">协议链接</label><input v-model="data.article.copyright.license_url" :class="inputCls" /></div>
        </div>
        <p class="text-xs text-gray-400 pt-2">分享平台</p>
        <div class="flex flex-wrap gap-3">
          <label v-for="p in platformOptions" :key="p" class="flex items-center gap-1.5 text-sm text-gray-700">
            <input type="checkbox" :checked="data.article.share.platforms.includes(p)" @change="togglePlatform(p)" /> {{ p }}
          </label>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" v-model="data.article.share.enabled" /> 启用分享</label>
        <p class="text-xs text-gray-400 pt-2">打赏</p>
        <label class="flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" v-model="data.article.reward.enabled" /> 启用打赏</label>
        <div class="grid grid-cols-3 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1">按钮文案</label><input v-model="data.article.reward.text" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">微信收款码 URL</label><input v-model="data.article.reward.wechat" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1">支付宝收款码 URL</label><input v-model="data.article.reward.alipay" :class="inputCls" /></div>
        </div>
        <button @click="save('article')" :disabled="savingKey === 'article'" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">保存文章设置</button>
      </div>

      <!-- 页脚 JSON -->
      <div v-show="active === 'footer'" class="space-y-3">
        <p class="text-xs text-gray-400">页脚（social 社交图标 + sitemap 站点地图分组），直接编辑 JSON。</p>
        <textarea v-model="rawText.footer" rows="18" class="w-full px-4 py-3 text-sm font-mono border border-gray-200 rounded-xl resize-y focus:outline-none focus:ring-2 focus:ring-indigo-500"></textarea>
        <button @click="save('footer')" :disabled="savingKey === 'footer'" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">保存页脚</button>
      </div>

      <!-- 高级 JSON -->
      <div v-show="active === 'advanced'" class="space-y-6">
        <div v-for="s in ['rightbar', 'wiki', 'friends']" :key="s" class="space-y-2">
          <label class="block text-xs font-medium text-gray-600">{{ s }}</label>
          <textarea v-model="rawText[s]" rows="8" class="w-full px-4 py-3 text-sm font-mono border border-gray-200 rounded-xl resize-y focus:outline-none focus:ring-2 focus:ring-indigo-500"></textarea>
          <button @click="save(s)" :disabled="savingKey === s" class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">保存 {{ s }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
