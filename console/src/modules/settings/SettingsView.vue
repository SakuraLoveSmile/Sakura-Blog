<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { settingsApi, authApi } from "@/api";
import { useAuthStore } from "@/stores/auth";
import { Icon } from "@iconify/vue";

// 全部段结构化编辑；「高级·JSON」保留 8 段 JSON 兜底（一般无需使用）。
const tabs = [
  { key: "site", label: "站点" },
  { key: "menubar", label: "菜单" },
  { key: "rightbar", label: "右栏" },
  { key: "footer", label: "页脚" },
  { key: "wiki", label: "文库" },
  { key: "comments", label: "评论" },
  { key: "article", label: "文章" },
  { key: "security", label: "安全" },
  { key: "advanced", label: "高级·JSON" },
];
const allSections = ["site", "menubar", "rightbar", "footer", "comments", "article", "wiki", "friends"];
const platformOptions = ["wechat", "weibo", "email", "link", "twitter", "telegram"];

const router = useRouter();
const auth = useAuthStore();

const active = ref("site");
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(true);
const savingKey = ref("");
const message = ref("");
const isError = ref(false);

function ensureDefaults(d: Record<string, any>) {
  d.site = d.site || {};
  d.menubar = d.menubar || {};
  d.menubar.items = d.menubar.items || [];
  d.rightbar = d.rightbar || {};
  d.rightbar.welcome_linklist = d.rightbar.welcome_linklist || {};
  d.rightbar.welcome_linklist.columns = d.rightbar.welcome_linklist.columns || 0;
  d.rightbar.welcome_linklist.items = d.rightbar.welcome_linklist.items || [];
  d.footer = d.footer || {};
  d.footer.social = d.footer.social || [];
  d.footer.sitemap = d.footer.sitemap || [];
  d.comments = d.comments || {};
  d.comments.notify = d.comments.notify || {};
  d.article = d.article || {};
  d.article.copyright = d.article.copyright || {};
  d.article.reward = d.article.reward || {};
  d.article.share = d.article.share || {};
  d.article.share.platforms = d.article.share.platforms || [];
  d.friends = d.friends || {};
  d.friends.refresh_interval = d.friends.refresh_interval || 0;
  // wiki 段是数组；行内编辑用 _tagsText 字符串代理 tags 数组（逗号分隔）
  d.wiki = (d.wiki || []).map((w: any) => ({ ...w, _tagsText: (w.tags || []).join(", ") }));
  return d;
}

// wiki 保存 payload：去掉 _tagsText 代理字段，逗号分隔转数组
function wikiPayload() {
  return (data.value.wiki as any[]).map(({ _tagsText, ...w }) => ({
    ...w,
    sort: Number(w.sort) || 0,
    tags: String(_tagsText || "").split(/[,，]/).map((s) => s.trim()).filter(Boolean),
  }));
}

async function load() {
  loading.value = true;
  try {
    const { data: res } = await settingsApi.all();
    data.value = ensureDefaults({ ...res });
    for (const s of allSections) {
      rawText.value[s] = JSON.stringify(res[s] ?? null, null, 2);
    }
  } finally {
    loading.value = false;
  }
}

async function save(section: string, payload?: unknown) {
  savingKey.value = section;
  message.value = "";
  isError.value = false;
  try {
    const body = payload !== undefined ? payload : data.value[section];
    await settingsApi.updateSection(section, body);
    rawText.value[section] = JSON.stringify(body, null, 2);
    message.value = "已保存，前台已生效";
  } catch (e: any) {
    isError.value = true;
    message.value = e?.response?.data?.error || e?.message || "保存失败";
  } finally {
    savingKey.value = "";
  }
}

// 高级 JSON tab：裸编辑兜底，保存后同步回结构化表单
async function saveJson(section: string) {
  savingKey.value = "json:" + section;
  message.value = "";
  isError.value = false;
  try {
    const payload = JSON.parse(rawText.value[section]);
    await settingsApi.updateSection(section, payload);
    if (section === "wiki") {
      data.value.wiki = (payload || []).map((w: any) => ({ ...w, _tagsText: (w.tags || []).join(", ") }));
    } else {
      data.value[section] = payload;
      ensureDefaults(data.value);
    }
    message.value = "已保存，前台已生效";
  } catch (e: any) {
    isError.value = true;
    message.value = e?.response?.data?.error || e?.message || "保存失败（检查 JSON 格式）";
  } finally {
    savingKey.value = "";
  }
}

// 列表行公共操作：增删 + 上下移
function removeAt(arr: any[], i: number) {
  arr.splice(i, 1);
}
function moveItem(arr: any[], i: number, dir: -1 | 1) {
  const j = i + dir;
  if (j < 0 || j >= arr.length) return;
  [arr[i], arr[j]] = [arr[j], arr[i]];
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
function addRightbarLink() {
  data.value.rightbar.welcome_linklist.items.push({ title: "", url: "", icon: "" });
}
function addSocial() {
  data.value.footer.social.push({ title: "", url: "", icon: "", icon_key: "" });
}
function addSitemapGroup() {
  data.value.footer.sitemap.push({ title: "", items: [] });
}
function addSitemapItem(g: any) {
  g.items = g.items || [];
  g.items.push({ title: "", url: "" });
}
function addWikiProject() {
  data.value.wiki.push({ id: "", name: "", title: "", description: "", icon: "", tags: [], url: "", sort: 0, _tagsText: "" });
}

// 安全 tab：修改管理密码
const pwdCurrent = ref("");
const pwdNew = ref("");
const pwdConfirm = ref("");
const savingPwd = ref(false);
const pwdMsg = ref("");
const pwdErr = ref(false);

async function savePassword() {
  pwdMsg.value = "";
  pwdErr.value = false;
  if (pwdNew.value !== pwdConfirm.value) {
    pwdErr.value = true;
    pwdMsg.value = "两次输入的新密码不一致";
    return;
  }
  if (pwdNew.value === "") {
    if (!confirm("新密码为空将关闭 Console 登录验证，任何人都可访问管理后台。确定继续？")) return;
  }
  savingPwd.value = true;
  try {
    const { data: res } = await authApi.changePassword(pwdCurrent.value, pwdNew.value);
    pwdCurrent.value = pwdNew.value = pwdConfirm.value = "";
    if (res.passwordRequired) {
      alert("密码已修改，请使用新密码重新登录");
      auth.authenticated = false;
      router.push("/login");
    } else {
      auth.passwordRequired = false;
      pwdMsg.value = "已关闭登录验证（新密码为空）";
    }
  } catch (e: any) {
    pwdErr.value = true;
    pwdMsg.value = e?.response?.data?.error || "修改失败";
  } finally {
    savingPwd.value = false;
  }
}

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const btnCls =
  "px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50";
const rowBtnCls = "text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 disabled:opacity-30";
const addBtnCls = "inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300";

onMounted(load);
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">站点设置</h2>
      <span v-if="message" :class="['text-sm px-3 py-1.5 rounded-lg', isError ? 'text-red-600 bg-red-50 dark:text-red-400 dark:bg-red-500/10' : 'text-green-700 bg-green-50 dark:text-green-400 dark:bg-green-500/10']">{{ message }}</span>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 border-b border-gray-200 mb-6 dark:border-gray-700 flex-wrap">
      <button v-for="t in tabs" :key="t.key" @click="active = t.key"
        :class="['px-4 py-2 text-sm font-medium -mb-px border-b-2 transition-colors', active === t.key ? 'border-indigo-600 text-indigo-700 dark:border-indigo-400 dark:text-indigo-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200']">
        {{ t.label }}
      </button>
    </div>

    <div v-if="loading" class="text-sm text-gray-400 py-12 text-center dark:text-gray-500">加载中...</div>

    <div v-else class="max-w-3xl">
      <!-- 站点 -->
      <div v-show="active === 'site'" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">站点标题</label><input v-model="data.site.title" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">副标题</label><input v-model="data.site.subtitle" :class="inputCls" /></div>
        </div>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">站点描述</label><textarea v-model="data.site.description" rows="2" :class="inputCls"></textarea></div>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">站点 URL（canonical/OG/sitemap 用）</label><input v-model="data.site.url" :class="inputCls" placeholder="https://example.com" /></div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">头像 URL</label><input v-model="data.site.avatar" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">头像悬停背景图</label><input v-model="data.site.avatar_bg" :class="inputCls" /></div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Logo 主标题</label><input v-model="data.site.logo_title" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Logo 副标题（含 | 拆滚动）</label><input v-model="data.site.logo_sub" :class="inputCls" /></div>
        </div>
        <button @click="save('site')" :disabled="savingKey === 'site'" :class="btnCls">保存站点信息</button>
      </div>

      <!-- 菜单 -->
      <div v-show="active === 'menubar'" class="space-y-4">
        <div class="w-40"><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">菜单列数</label><input v-model.number="data.menubar.columns" type="number" min="1" :class="inputCls" /></div>
        <div class="space-y-2">
          <div v-for="(it, i) in data.menubar.items" :key="i" class="grid grid-cols-12 gap-2 items-center">
            <input v-model="it.id" placeholder="id" :class="inputCls" class="col-span-2" />
            <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-span-2" />
            <input v-model="it.url" placeholder="/url" :class="inputCls" class="col-span-3" />
            <input v-model="it.icon" placeholder="icon key" :class="inputCls" class="col-span-3" />
            <input v-model="it.theme" placeholder="#色值" :class="inputCls" class="col-span-1" />
            <button @click="removeAt(data.menubar.items, Number(i))" class="col-span-1 text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
          </div>
        </div>
        <button @click="addMenuItem" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加菜单项</button>
        <div><button @click="save('menubar')" :disabled="savingKey === 'menubar'" :class="btnCls">保存菜单</button></div>
      </div>

      <!-- 右栏 -->
      <div v-show="active === 'rightbar'" class="space-y-4">
        <p class="text-xs text-gray-400 dark:text-gray-500">首页右栏「欢迎语」组件内的链接列表（welcome_linklist）。</p>
        <div class="w-40"><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">列数（0=自动）</label><input v-model.number="data.rightbar.welcome_linklist.columns" type="number" min="0" :class="inputCls" /></div>
        <div class="space-y-2">
          <div v-for="(it, i) in data.rightbar.welcome_linklist.items" :key="i" class="grid grid-cols-12 gap-2 items-center">
            <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-span-3" />
            <input v-model="it.url" placeholder="/url 或 https://..." :class="inputCls" class="col-span-4" />
            <input v-model="it.icon" placeholder="icon（图片 URL 或 icons.yml key）" :class="inputCls" class="col-span-3" />
            <div class="col-span-2 flex items-center gap-1.5 justify-end">
              <button @click="moveItem(data.rightbar.welcome_linklist.items, Number(i), -1)" :disabled="Number(i) === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
              <button @click="moveItem(data.rightbar.welcome_linklist.items, Number(i), 1)" :disabled="Number(i) === data.rightbar.welcome_linklist.items.length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
              <button @click="removeAt(data.rightbar.welcome_linklist.items, Number(i))" class="text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
            </div>
          </div>
        </div>
        <button @click="addRightbarLink" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加链接</button>
        <div><button @click="save('rightbar')" :disabled="savingKey === 'rightbar'" :class="btnCls">保存右栏</button></div>
      </div>

      <!-- 页脚 -->
      <div v-show="active === 'footer'" class="space-y-6">
        <div class="space-y-2">
          <p class="text-xs text-gray-400 dark:text-gray-500">社交图标（icon 填 raw HTML 如 &lt;img src="..."&gt;，icon_key 填 icons.yml 的 key，二选一，icon 优先）</p>
          <div v-for="(s, i) in data.footer.social" :key="i" class="grid grid-cols-12 gap-2 items-center">
            <input v-model="s.title" placeholder="标题" :class="inputCls" class="col-span-2" />
            <input v-model="s.url" placeholder="链接" :class="inputCls" class="col-span-3" />
            <input v-model="s.icon_key" placeholder="icon_key" :class="inputCls" class="col-span-2" />
            <input v-model="s.icon" placeholder="icon raw HTML（优先）" :class="inputCls" class="col-span-3" />
            <div class="col-span-2 flex items-center gap-1.5 justify-end">
              <button @click="moveItem(data.footer.social, Number(i), -1)" :disabled="Number(i) === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
              <button @click="moveItem(data.footer.social, Number(i), 1)" :disabled="Number(i) === data.footer.social.length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
              <button @click="removeAt(data.footer.social, Number(i))" class="text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
            </div>
          </div>
          <button @click="addSocial" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加社交图标</button>
        </div>

        <div class="space-y-3">
          <p class="text-xs text-gray-400 dark:text-gray-500">站点地图分组（页脚多列链接）</p>
          <div v-for="(g, gi) in data.footer.sitemap" :key="gi" class="border border-gray-200 rounded-xl p-4 space-y-2 dark:border-gray-700">
            <div class="flex items-center gap-2">
              <input v-model="g.title" placeholder="组名" :class="inputCls" class="flex-1" />
              <button @click="moveItem(data.footer.sitemap, Number(gi), -1)" :disabled="Number(gi) === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
              <button @click="moveItem(data.footer.sitemap, Number(gi), 1)" :disabled="Number(gi) === data.footer.sitemap.length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
              <button @click="removeAt(data.footer.sitemap, Number(gi))" class="text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
            </div>
            <div v-for="(it, ii) in g.items" :key="ii" class="grid grid-cols-12 gap-2 items-center pl-4">
              <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-span-4" />
              <input v-model="it.url" placeholder="/url" :class="inputCls" class="col-span-6" />
              <div class="col-span-2 flex items-center gap-1.5 justify-end">
                <button @click="moveItem(g.items, Number(ii), -1)" :disabled="Number(ii) === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
                <button @click="moveItem(g.items, Number(ii), 1)" :disabled="Number(ii) === g.items.length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
                <button @click="removeAt(g.items, Number(ii))" class="text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
              </div>
            </div>
            <button @click="addSitemapItem(g)" :class="addBtnCls" class="pl-4 text-xs"><Icon icon="ri:add-line" /> 添加链接</button>
          </div>
          <button @click="addSitemapGroup" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加分组</button>
        </div>
        <button @click="save('footer')" :disabled="savingKey === 'footer'" :class="btnCls">保存页脚</button>
      </div>

      <!-- 文库 -->
      <div v-show="active === 'wiki'" class="space-y-4">
        <p class="text-xs text-gray-400 dark:text-gray-500">/wiki 页项目卡片。sort 越大越靠前；tags 逗号分隔。</p>
        <div v-for="(w, i) in data.wiki" :key="i" class="border border-gray-200 rounded-xl p-4 space-y-3 dark:border-gray-700">
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ w.title || w.name || w.id || "新项目" }}</span>
            <div class="flex items-center gap-1.5">
              <button @click="moveItem(data.wiki, Number(i), -1)" :disabled="Number(i) === 0" :class="rowBtnCls"><Icon icon="ri:arrow-up-s-line" /></button>
              <button @click="moveItem(data.wiki, Number(i), 1)" :disabled="Number(i) === data.wiki.length - 1" :class="rowBtnCls"><Icon icon="ri:arrow-down-s-line" /></button>
              <button @click="removeAt(data.wiki, Number(i))" class="text-red-500 hover:text-red-700"><Icon icon="ri:delete-bin-line" /></button>
            </div>
          </div>
          <div class="grid grid-cols-3 gap-3">
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">id</label><input v-model="w.id" :class="inputCls" /></div>
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">标题</label><input v-model="w.title" :class="inputCls" /></div>
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">排序（大在前）</label><input v-model.number="w.sort" type="number" :class="inputCls" /></div>
          </div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">描述</label><input v-model="w.description" :class="inputCls" /></div>
          <div class="grid grid-cols-2 gap-3">
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">图标（图片 URL）</label><input v-model="w.icon" :class="inputCls" /></div>
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">链接 URL</label><input v-model="w.url" :class="inputCls" placeholder="/posts/xxx 或 https://..." /></div>
          </div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">标签（逗号分隔）</label><input v-model="w._tagsText" :class="inputCls" placeholder="Go, 开源" /></div>
        </div>
        <button @click="addWikiProject" :class="addBtnCls"><Icon icon="ri:add-line" /> 添加项目</button>
        <div><button @click="save('wiki', wikiPayload())" :disabled="savingKey === 'wiki'" :class="btnCls">保存文库</button></div>
      </div>

      <!-- 评论 -->
      <div v-show="active === 'comments'" class="space-y-4">
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><input type="checkbox" v-model="data.comments.enabled" /> 启用评论（关闭需重启才断开路由）</label>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><input type="checkbox" v-model="data.comments.moderation" /> 先审后发</label>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">评论区标题</label><input v-model="data.comments.title" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">博主徽章文案</label><input v-model="data.comments.admin_badge" :class="inputCls" /></div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">博主邮箱</label><input v-model="data.comments.admin_email" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Gravatar 镜像</label><input v-model="data.comments.gravatar_mirror" :class="inputCls" /></div>
        </div>
        <p class="text-xs text-gray-400 dark:text-gray-500 pt-2">邮件通知（SMTP，主机留空则关闭）</p>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">SMTP 主机</label><input v-model="data.comments.notify.smtp_host" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">端口</label><input v-model.number="data.comments.notify.smtp_port" type="number" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">用户名</label><input v-model="data.comments.notify.username" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">密码</label><input v-model="data.comments.notify.password" type="password" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">发件人（留空用用户名）</label><input v-model="data.comments.notify.from" :class="inputCls" /></div>
        </div>
        <button @click="save('comments')" :disabled="savingKey === 'comments'" :class="btnCls">保存评论设置</button>
      </div>

      <!-- 文章 -->
      <div v-show="active === 'article'" class="space-y-4">
        <p class="text-xs text-gray-400 dark:text-gray-500">版权声明</p>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><input type="checkbox" v-model="data.article.copyright.enabled" /> 显示版权声明</label>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">协议名</label><input v-model="data.article.copyright.license" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">协议链接</label><input v-model="data.article.copyright.license_url" :class="inputCls" /></div>
        </div>
        <p class="text-xs text-gray-400 dark:text-gray-500 pt-2">分享平台</p>
        <div class="flex flex-wrap gap-3">
          <label v-for="p in platformOptions" :key="p" class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-300">
            <input type="checkbox" :checked="data.article.share.platforms.includes(p)" @change="togglePlatform(p)" /> {{ p }}
          </label>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><input type="checkbox" v-model="data.article.share.enabled" /> 启用分享</label>
        <p class="text-xs text-gray-400 dark:text-gray-500 pt-2">打赏</p>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><input type="checkbox" v-model="data.article.reward.enabled" /> 启用打赏</label>
        <div class="grid grid-cols-3 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">按钮文案</label><input v-model="data.article.reward.text" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">微信收款码 URL</label><input v-model="data.article.reward.wechat" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">支付宝收款码 URL</label><input v-model="data.article.reward.alipay" :class="inputCls" /></div>
        </div>
        <button @click="save('article')" :disabled="savingKey === 'article'" :class="btnCls">保存文章设置</button>

        <div class="border-t border-gray-200 pt-4 mt-6 space-y-3 dark:border-gray-700">
          <p class="text-xs text-gray-400 dark:text-gray-500">友链 RSS 聚合（friends 段，独立保存）</p>
          <div class="w-56"><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">刷新间隔（分钟，≤0 用默认 60）</label><input v-model.number="data.friends.refresh_interval" type="number" :class="inputCls" /></div>
          <button @click="save('friends')" :disabled="savingKey === 'friends'" :class="btnCls">保存 RSS 设置</button>
        </div>
      </div>

      <!-- 安全 -->
      <div v-show="active === 'security'" class="space-y-4 max-w-md">
        <p class="text-xs text-gray-400 dark:text-gray-500">修改 Console 管理密码。修改后所有已登录会话失效，需用新密码重新登录。</p>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">当前密码</label><input v-model="pwdCurrent" type="password" autocomplete="current-password" :class="inputCls" /></div>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">新密码</label><input v-model="pwdNew" type="password" autocomplete="new-password" :class="inputCls" /></div>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">确认新密码</label><input v-model="pwdConfirm" type="password" autocomplete="new-password" :class="inputCls" /></div>
        <p v-if="pwdNew === '' && pwdConfirm === ''" class="text-xs text-red-500 dark:text-red-400">⚠ 新密码留空 = 关闭登录验证，任何人都可访问管理后台，请谨慎。</p>
        <span v-if="pwdMsg" :class="['block text-sm', pwdErr ? 'text-red-600 dark:text-red-400' : 'text-green-700 dark:text-green-400']">{{ pwdMsg }}</span>
        <button @click="savePassword" :disabled="savingPwd" :class="btnCls">修改密码</button>
        <p class="text-xs text-gray-400 dark:text-gray-500 pt-2">忘记密码：删除数据库 settings 表 admin 段后重启，回落 site.yml 初始密码。</p>
      </div>

      <!-- 高级 JSON -->
      <div v-show="active === 'advanced'" class="space-y-6">
        <p class="text-xs text-gray-400 dark:text-gray-500">全部段 JSON 兜底编辑，一般无需使用（结构化表单已覆盖）。</p>
        <div v-for="s in allSections" :key="s" class="space-y-2">
          <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">{{ s }}</label>
          <textarea v-model="rawText[s]" rows="8" class="w-full px-4 py-3 text-sm font-mono border border-gray-200 rounded-xl resize-y focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"></textarea>
          <button @click="saveJson(s)" :disabled="savingKey === 'json:' + s" :class="btnCls">保存 {{ s }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
