<script setup lang="ts">
import { ref, onMounted } from "vue";
import { settingsApi } from "@/api";
import { Icon } from "@iconify/vue";
import MenubarEditor from "./MenubarEditor.vue";
import AppearanceTab from "./AppearanceTab.vue";
import WidgetsTab from "./WidgetsTab.vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { VPageHeader, Toast } from "@/components/ui";

// 全部段结构化编辑；「高级·JSON」保留 8 段 JSON 兜底（一般无需使用）。
const tabs = [
  { key: "site", label: "站点" },
  { key: "menubar", label: "菜单" },
  { key: "rightbar", label: "右栏" },
  { key: "footer", label: "页脚" },
  { key: "wiki", label: "文库" },
  { key: "comments", label: "评论" },
  { key: "article", label: "文章" },
  { key: "seo", label: "SEO" },
  { key: "inject", label: "代码注入" },
  { key: "ai", label: "AI" },
  { key: "security", label: "安全" },
  { key: "appearance", label: "外观" },
  { key: "widgets", label: "组件编排" },
  { key: "advanced", label: "高级·JSON" },
];
const allSections = ["site", "menubar", "rightbar", "footer", "comments", "article", "wiki", "friends", "seo", "inject", "dashboard", "style", "widgets", "ai"];
const platformOptions = ["wechat", "weibo", "email", "link", "twitter", "telegram"];

const active = ref("site");
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(true);
const savingKey = ref("");

// 头像选择器：复用附件选择弹窗，选中后回填 URL
const showAvatarPicker = ref(false);
const showAvatarBgPicker = ref(false);

/** 头像 URL 选中回调 */
function handleAvatarSelect(url: string) {
  data.value.site.avatar = url;
}

/** 头像悬停背景图 URL 选中回调 */
function handleAvatarBgSelect(url: string) {
  data.value.site.avatar_bg = url;
}

// Favicon / Apple Touch Icon 选择器（Spec Task 9）
const showFaviconPicker = ref(false);
const showAppleIconPicker = ref(false);

/** Favicon URL 选中回调 */
function handleFaviconSelect(url: string) {
  data.value.site.favicon_url = url;
}

/** Apple Touch Icon URL 选中回调 */
function handleAppleIconSelect(url: string) {
  data.value.site.apple_touch_icon = url;
}

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
  d.seo = d.seo || {};
  d.inject = d.inject || {};
  d.ai = d.ai || {};
  // wiki 段是数组；行内编辑用 _tagsText 字符串代理 tags 数组（逗号分隔）
  d.wiki = (d.wiki || []).map((w: any) => ({ ...w, _tagsText: (w.tags || []).join(", ") }));
  // style 段默认值（Spec 47）
  d.style = d.style || {};
  d.style.color = d.style.color || {};
  d.style.font_size = d.style.font_size || {};
  d.style.font_family = d.style.font_family || {};
  d.style.leftbar = d.style.leftbar || {};
  d.style.site = d.style.site || {};
  d.style.codeblock = d.style.codeblock || {};
  d.style.header_prefix = d.style.header_prefix || {};
  // widgets 段默认值（Spec 47）
  d.widgets = d.widgets || {};
  d.widgets.library = d.widgets.library || {};
  d.widgets.pages = d.widgets.pages || {};
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
  try {
    const body = payload !== undefined ? payload : data.value[section];
    await settingsApi.updateSection(section, body);
    rawText.value[section] = JSON.stringify(body, null, 2);
    Toast.success("已保存，前台已生效");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "保存失败");
  } finally {
    savingKey.value = "";
  }
}

// 高级 JSON tab：裸编辑兖底，保存后同步回结构化表单
async function saveJson(section: string) {
  savingKey.value = "json:" + section;
  try {
    const payload = JSON.parse(rawText.value[section]);
    await settingsApi.updateSection(section, payload);
    if (section === "wiki") {
      data.value.wiki = (payload || []).map((w: any) => ({ ...w, _tagsText: (w.tags || []).join(", ") }));
    } else {
      data.value[section] = payload;
      ensureDefaults(data.value);
    }
    Toast.success("已保存，前台已生效");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "保存失败（检查 JSON 格式）");
  } finally {
    savingKey.value = "";
  }
}

// 测试发信（Spec 26）
const testing = ref(false);
const testResult = ref("");
const testOk = ref(false);
async function testMail() {
  testing.value = true;
  testResult.value = "";
  try {
    const { data: res } = await settingsApi.testNotify();
    if (res.ok) {
      testOk.value = true;
      testResult.value = "测试邮件已发送，请查收";
    } else {
      testOk.value = false;
      testResult.value = res.error || "发送失败";
    }
  } catch (e: any) {
    testOk.value = false;
    testResult.value = e?.response?.data?.error || e?.message || "请求失败";
  } finally {
    testing.value = false;
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

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const btnCls =
  "px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg hover:opacity-90 disabled:opacity-50";
const rowBtnCls = "text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 disabled:opacity-30";
const addBtnCls = "inline-flex items-center gap-1 text-sm text-secondary hover:text-primary transition-colors dark:text-primary dark:hover:text-primary/80";

onMounted(load);
</script>

<template>
  <div>
    <VPageHeader title="站点设置">
      <template #icon>
        <Icon icon="ri:settings-3-line" class="mr-2 self-center text-xl" />
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
    <!-- Tabs -->
    <div class="flex gap-1 border-b border-gray-200 mb-6 dark:border-gray-700 flex-wrap">
      <button v-for="t in tabs" :key="t.key" @click="active = t.key"
        :class="['px-4 py-2 text-sm font-medium -mb-px border-b-2 transition-colors', active === t.key ? 'border-secondary text-secondary dark:border-primary dark:text-primary' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200']">
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
        <!-- 头像设置 -->
        <div class="space-y-3">
          <div class="grid grid-cols-2 gap-4">
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">头像 URL</label>
              <div class="flex gap-2">
                <input v-model="data.site.avatar" :class="inputCls" placeholder="输入图片 URL" />
                <button @click="showAvatarPicker = true" title="从附件选择"
                  class="shrink-0 px-3 py-2 text-sm bg-primary/10 text-secondary rounded-lg hover:bg-primary/20 dark:bg-primary/15 dark:text-primary">
                  <Icon icon="ri:image-upload-line" />
                </button>
              </div>
            </div>
            <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">头像悬停背景图</label>
              <div class="flex gap-2">
                <input v-model="data.site.avatar_bg" :class="inputCls" placeholder="输入图片 URL" />
                <button @click="showAvatarBgPicker = true" title="从附件选择"
                  class="shrink-0 px-3 py-2 text-sm bg-primary/10 text-secondary rounded-lg hover:bg-primary/20 dark:bg-primary/15 dark:text-primary">
                  <Icon icon="ri:image-upload-line" />
                </button>
              </div>
            </div>
          </div>

          <!-- 头像实时预览 -->
          <div v-if="data.site.avatar || data.site.avatar_bg" class="border border-gray-200 rounded-xl p-4 dark:border-gray-700">
            <p class="text-xs font-medium text-gray-600 mb-3 dark:text-gray-400">实时预览</p>
            <div class="flex flex-wrap gap-6 items-center">
              <!-- 导航栏头像 (32x32) -->
              <div v-if="data.site.avatar" class="text-center">
                <p class="text-[10px] text-gray-500 mb-2">导航栏头像 (32×32)</p>
                <div class="w-8 h-8 rounded-full overflow-hidden border border-gray-200">
                  <img :src="data.site.avatar" class="w-full h-full object-cover" alt="" />
                </div>
              </div>

              <!-- 社交分享卡片头像 (128x128) -->
              <div v-if="data.site.avatar" class="text-center">
                <p class="text-[10px] text-gray-500 mb-2">分享卡片 (128×128)</p>
                <div class="w-32 h-32 rounded-full overflow-hidden border border-gray-200 shadow-sm">
                  <img :src="data.site.avatar" class="w-full h-full object-cover" alt="" />
                </div>
              </div>

              <!-- 悬停效果预览 -->
              <div v-if="data.site.avatar && data.site.avatar_bg" class="text-center">
                <p class="text-[10px] text-gray-500 mb-2">悬停效果</p>
                <div class="group relative w-24 h-24 rounded-full overflow-hidden transition-transform duration-300 cursor-pointer border-2 border-transparent hover:border-primary">
                  <img :src="data.site.avatar_bg" class="absolute inset-0 w-full h-full object-cover" alt="" />
                  <img :src="data.site.avatar" class="absolute inset-0 w-full h-full object-cover group-hover:scale-110 transition-transform duration-300" alt="" />
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Logo 主标题</label><input v-model="data.site.logo_title" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Logo 副标题（含 | 拆滚动）</label><input v-model="data.site.logo_sub" :class="inputCls" /></div>
        </div>
        <!-- Favicon / Apple Touch Icon 设置（Spec Task 9） -->
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Favicon</label>
            <div class="flex gap-2 items-center">
              <img :src="data.site.favicon_url || '/favicon.ico'" class="w-6 h-6 rounded shrink-0" alt="" />
              <input v-model="data.site.favicon_url" :class="inputCls" placeholder="留空用默认 /favicon.ico" />
              <button @click="showFaviconPicker = true"
                class="shrink-0 px-3 py-2 bg-primary/10 text-secondary rounded-lg hover:bg-primary/20 dark:bg-primary/15 dark:text-primary">
                <Icon icon="ri:image-upload-line" />
              </button>
            </div>
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Apple Touch Icon</label>
            <div class="flex gap-2 items-center">
              <img :src="data.site.apple_touch_icon || '/apple-touch-icon.png'" class="w-8 h-8 rounded-lg shrink-0" alt="" />
              <input v-model="data.site.apple_touch_icon" :class="inputCls" placeholder="留空用默认" />
              <button @click="showAppleIconPicker = true"
                class="shrink-0 px-3 py-2 bg-primary/10 text-secondary rounded-lg hover:bg-primary/20 dark:bg-primary/15 dark:text-primary">
                <Icon icon="ri:image-upload-line" />
              </button>
            </div>
          </div>
        </div>
        <button @click="save('site')" :disabled="savingKey === 'site'" :class="btnCls">保存站点信息</button>
      </div>

      <!-- 菜单 -->
      <div v-show="active === 'menubar'" class="space-y-4">
        <MenubarEditor
          :items="data.menubar.items"
          :columns="data.menubar.columns"
          @update:columns="data.menubar.columns = $event"
          @save="save('menubar')"
        />
      </div>

      <!-- 右栏 -->
      <div v-show="active === 'rightbar'" class="space-y-4">
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">欢迎语正文（留空用默认文案，Spec Task 5）</label>
          <textarea v-model="data.rightbar.intro" rows="3" :class="inputCls" placeholder="输入右栏欢迎语正文，换行会保留显示"></textarea>
        </div>
        <p class="text-xs text-gray-400 dark:text-gray-500">首页右栏「欢迎语」 组件内的链接列表（welcome_linklist）。</p>
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
        <button @click="testMail" :disabled="testing" class="ml-2 px-3 py-1.5 text-xs rounded-lg border border-gray-300 text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700">
          {{ testing ? '发送中...' : '发送测试邮件' }}
        </button>
        <span v-if="testResult" :class="['text-xs ml-2', testOk ? 'text-green-600' : 'text-red-500']">{{ testResult }}</span>
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

      <!-- SEO（Spec 42） -->
      <div v-show="active === 'seo'" class="space-y-4">
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">默认 Meta Description（文章未单独设置时使用）</label><textarea v-model="data.seo.default_description" rows="2" :class="inputCls"></textarea></div>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">默认 Meta Keywords（逗号分隔）</label><input v-model="data.seo.default_keywords" :class="inputCls" placeholder="博客, Go, 前端" /></div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">robots.txt 附加规则（原样追加到默认规则之后）</label>
          <textarea v-model="data.seo.robots_extra" rows="4" :class="inputCls + ' font-mono text-xs'" placeholder="User-agent: GPTBot&#10;Disallow: /"></textarea>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><input type="checkbox" v-model="data.seo.no_index" /> 禁止搜索引擎收录（全站输出 noindex meta，请谨慎开启）</label>
        <p class="text-xs text-gray-400 dark:text-gray-500 pt-2">站点验证（留空不输出对应 meta 标签）</p>
        <div class="grid grid-cols-3 gap-4">
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Google 验证码</label><input v-model="data.seo.google_verification" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Bing 验证码</label><input v-model="data.seo.bing_verification" :class="inputCls" /></div>
          <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">百度验证码</label><input v-model="data.seo.baidu_verification" :class="inputCls" /></div>
        </div>
        <button @click="save('seo')" :disabled="savingKey === 'seo'" :class="btnCls">保存 SEO 设置</button>
      </div>

      <!-- 代码注入（Spec 42） -->
      <div v-show="active === 'inject'" class="space-y-4">
        <p class="text-xs text-red-500 dark:text-red-400">⚠ 注入内容将不经转义原样输出到前台所有页面，可执行任意脚本。请只粘贴你完全信任的代码（统计、字体、自定义样式等），错误或恶意代码可能导致页面故障与安全风险，由此产生的后果由站长自行承担。</p>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">头部注入（输出在 &lt;/head&gt; 之前）</label>
          <textarea v-model="data.inject.inject_head" rows="6" :class="inputCls + ' font-mono text-xs'" placeholder="<script>...</script> 或 <link ...>"></textarea>
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">页脚注入（输出在 &lt;/body&gt; 之前）</label>
          <textarea v-model="data.inject.inject_footer" rows="6" :class="inputCls + ' font-mono text-xs'" placeholder="<script>...</script>"></textarea>
        </div>
        <button @click="save('inject')" :disabled="savingKey === 'inject'" :class="btnCls">保存代码注入</button>
      </div>

      <!-- AI 属性生成 -->
      <div v-show="active === 'ai'" class="space-y-4 max-w-md">
        <p class="text-xs text-gray-400 dark:text-gray-500">编辑器「AI 属性生成」使用的 OpenAI 兼容接口（DeepSeek / 通义 / Ollama 等均可）。</p>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">API 地址（如 https://api.deepseek.com/v1）</label><input v-model="data.ai.base_url" placeholder="https://api.openai.com/v1" :class="inputCls" /></div>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">API Key（本地模型可留空；已保存时显示 ****，留空保存不覆盖）</label><input v-model="data.ai.api_key" type="password" autocomplete="off" :class="inputCls" /></div>
        <div><label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">模型名（如 deepseek-chat / gpt-4o-mini）</label><input v-model="data.ai.model" :class="inputCls" /></div>
        <button @click="save('ai')" :disabled="savingKey === 'ai'" :class="btnCls">保存 AI 设置</button>
      </div>
      </div>

      <!-- 安全 -->
      <div v-show="active === 'security'" class="space-y-4 max-w-md">
        <div class="rounded-lg bg-yellow-50 p-4 dark:bg-yellow-900/20">
          <p class="text-sm text-yellow-800 dark:text-yellow-200">
            密码管理已迁移至
            <router-link to="/profile" class="font-medium text-blue-600 hover:underline dark:text-blue-400">个人资料页</router-link>
          </p>
        </div>
        <p class="text-xs text-gray-400 dark:text-gray-500">忘记密码：删除数据库 settings 表 admin 段后重启，回落 site.yml 初始密码。</p>
      </div>

      <!-- 高级 JSON -->
      <div v-show="active === 'advanced'" class="space-y-6">
        <p class="text-xs text-gray-400 dark:text-gray-500">全部段 JSON 兜底编辑，一般无需使用（结构化表单已覆盖）。</p>
        <div v-for="s in allSections" :key="s" class="space-y-2">
          <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">{{ s }}</label>
          <textarea v-model="rawText[s]" rows="8" class="w-full px-4 py-3 text-sm font-mono border border-gray-200 rounded-xl resize-y focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"></textarea>
          <button @click="saveJson(s)" :disabled="savingKey === 'json:' + s" :class="btnCls">保存 {{ s }}</button>
        </div>
      </div>

      <AppearanceTab v-show="active === 'appearance'" :data="data" :saving="savingKey" @save="save" />
      <WidgetsTab v-show="active === 'widgets'" :data="data" :saving="savingKey" @save="save" />

      <!-- 头像选择器 -->
      <AttachmentPickerModal
        :open="showAvatarPicker"
        @select="handleAvatarSelect"
        @close="showAvatarPicker = false"
      />

      <!-- 头像背景选择器 -->
      <AttachmentPickerModal
        :open="showAvatarBgPicker"
        @select="handleAvatarBgSelect"
        @close="showAvatarBgPicker = false"
      />

      <!-- Favicon 选择器 -->
      <AttachmentPickerModal
        :open="showFaviconPicker"
        @select="handleFaviconSelect"
        @close="showFaviconPicker = false"
      />

      <!-- Apple Touch Icon 选择器 -->
      <AttachmentPickerModal
        :open="showAppleIconPicker"
        @select="handleAppleIconSelect"
        @close="showAppleIconPicker = false"
      />
    </div>
    </div>
  </div>
</template>
