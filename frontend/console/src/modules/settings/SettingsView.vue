<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRoute } from "vue-router";
import { settingsApi } from "@/api";
import { Icon } from "@iconify/vue";
import MenubarEditor from "./MenubarEditor.vue";
import AppearanceTab from "./AppearanceTab.vue";
import WidgetsTab from "./WidgetsTab.vue";
import PluginsTab from "./PluginsTab.vue";
import CoverTab from "./CoverTab.vue";
import NavTabsTab from "./NavTabsTab.vue";
import AuthorsTab from "./AuthorsTab.vue";
import LangTab from "./LangTab.vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { VSwitch, VCard, VButton, Toast } from "@/components/ui";

// 全部段结构化编辑；「高级·JSON」保留 JSON 兜底（一般无需使用）。
// 两级分组导航：第一级为左侧子导航（大分组），第二级为右侧内容内的小分段。
const groups = [
  {
    key: "site",
    label: "站点",
    icon: "ri:home-4-line",
    desc: "配置站点基本信息、菜单与文案",
    tabs: [
      { key: "site", label: "站点" },
      { key: "menubar", label: "菜单" },
      { key: "rightbar", label: "右栏" },
      { key: "footer", label: "页脚" },
      { key: "lang", label: "文案" },
    ],
  },
  {
    key: "appearance",
    label: "外观",
    icon: "ri:palette-line",
    desc: "自定义主题、Hero 与组件编排",
    tabs: [
      { key: "appearance", label: "外观" },
      { key: "cover", label: "首页 Hero" },
      { key: "nav_tabs", label: "导航标签" },
      { key: "widgets", label: "组件编排" },
      { key: "plugins", label: "插件" },
    ],
  },
  {
    key: "content",
    label: "内容",
    icon: "ri:article-line",
    desc: "管理文章、评论、文库与作者",
    tabs: [
      { key: "article", label: "文章" },
      { key: "comments", label: "评论" },
      { key: "wiki", label: "文库" },
      { key: "authors", label: "作者" },
    ],
  },
  {
    key: "system",
    label: "系统",
    icon: "ri:settings-4-line",
    desc: "SEO、代码注入、AI 与安全",
    tabs: [
      { key: "seo", label: "SEO" },
      { key: "inject", label: "代码注入" },
      { key: "ai", label: "AI" },
      { key: "security", label: "安全" },
      { key: "advanced", label: "高级·JSON" },
    ],
  },
];
const allSections = ["site","menubar","rightbar","footer","comments","article","wiki","friends","seo","inject","dashboard","style","widgets","ai","cover","nav_tabs","authors","plugins","lang"];
const platformOptions = ["wechat","weibo","email","link","twitter","telegram"];

const route = useRoute();
const active = ref("site");
const activeGroup = ref(groups[0].key);
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(true);
const loadError = ref("");
const savingKey = ref("");

const showAvatarPicker = ref(false);
const showAvatarBgPicker = ref(false);
function handleAvatarSelect(url: string) { data.value.site.avatar = url; }
function handleAvatarBgSelect(url: string) { data.value.site.avatar_bg = url; }

const showFaviconPicker = ref(false);
const showAppleIconPicker = ref(false);
function handleFaviconSelect(url: string) { data.value.site.favicon_url = url; }
function handleAppleIconSelect(url: string) { data.value.site.apple_touch_icon = url; }

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
  d.friends.posts_per_link = d.friends.posts_per_link || 20;
  d.seo = d.seo || {};
  d.inject = d.inject || {};
  d.ai = d.ai || {};
  d.ai.summary_enabled = d.ai.summary_enabled ?? true;
  d.ai.allow_user_api = d.ai.allow_user_api ?? true;
  const rawWiki = Array.isArray(d.wiki) ? d.wiki : [];
  d.wiki = rawWiki.map((w: any) => {
    const tags = Array.isArray(w?.tags) ? w.tags : String(w?.tags ?? "").split(/[,，]/).map((s: string) => s.trim()).filter(Boolean);
    return { ...w, _tagsText: tags.join(",") };
  });
  d.style = d.style || {};
  d.style.color = d.style.color || {};
  d.style.font_size = d.style.font_size || {};
  d.style.font_family = d.style.font_family || {};
  d.style.leftbar = d.style.leftbar || {};
  d.style.site = d.style.site || {};
  d.style.codeblock = d.style.codeblock || {};
  d.style.header_prefix = d.style.header_prefix || {};
  d.widgets = d.widgets || {};
  d.widgets.library = d.widgets.library || {};
  d.widgets.pages = d.widgets.pages || {};
  d.plugins = d.plugins || {};
  d.plugins.preload = d.plugins.preload || {};
  d.plugins.scrollreveal = d.plugins.scrollreveal || {};
  d.plugins.heti = d.plugins.heti || {};
  d.plugins.fancybox = d.plugins.fancybox || {};
  d.plugins.pjax = d.plugins.pjax || {};
  d.cover = d.cover || {};
  d.cover.features = d.cover.features || [];
  d.nav_tabs = d.nav_tabs || {};
  d.nav_tabs.items = d.nav_tabs.items || [];
  d.authors = d.authors || {};
  d.lang = d.lang || {};
  return d;
}

function sortWikiProjects(list: any[]): any[] {
  return [...list].sort((a, b) => {
    const sa = Number(a?.sort) || 0;
    const sb = Number(b?.sort) || 0;
    if (sa !== sb) return sb - sa;
    return String(a?.id ?? "").localeCompare(String(b?.id ?? ""));
  });
}

function wikiPayload() {
  return (data.value.wiki as any[]).map(({ _tagsText, ...w }) => ({
    ...w,
    sort: Number(w.sort) || 0,
    tags: String(_tagsText || "").split(/[,，]/).map((s) => s.trim()).filter(Boolean),
  }));
}

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data: res } = await settingsApi.all();
    data.value = ensureDefaults({ ...res });
    data.value.wiki = sortWikiProjects(data.value.wiki);
    for (const s of allSections) {
      rawText.value[s] = JSON.stringify(res[s] ?? null, null, 2);
    }
  } catch (e: any) {
    loadError.value = e?.response?.data?.error || e?.message || "加载设置失败";
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

async function saveJson(section: string) {
  savingKey.value = "json:" + section;
  try {
    const payload = JSON.parse(rawText.value[section]);
    await settingsApi.updateSection(section, payload);
    if (section === "wiki") {
      const list = Array.isArray(payload) ? payload : [];
      data.value.wiki = sortWikiProjects(list.map((w: any) => {
        const tags = Array.isArray(w?.tags) ? w.tags : String(w?.tags ?? "").split(/[,，]/).map((s: string) => s.trim()).filter(Boolean);
        return { ...w, _tagsText: tags.join(",") };
      }));
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

function removeAt(arr: any[], i: number) { arr.splice(i, 1); }
function moveItem(arr: any[], i: number, dir: -1 | 1) {
  const j = i + dir;
  if (j < 0 || j >= arr.length) return;
  [arr[i], arr[j]] = [arr[j], arr[i]];
}

const dragWikiIndex = ref<number | null>(null);
function onWikiDragStart(i: number) { dragWikiIndex.value = i; }
function onWikiDragOver(e: DragEvent, i: number) {
  e.preventDefault();
  if (dragWikiIndex.value === null || dragWikiIndex.value === i) return;
  const arr = data.value.wiki;
  const [moved] = arr.splice(dragWikiIndex.value, 1);
  arr.splice(i, 0, moved);
  dragWikiIndex.value = i;
  syncWikiSort();
}
function onWikiDragEnd() { dragWikiIndex.value = null; }
function syncWikiSort() {
  const arr = data.value.wiki || [];
  arr.forEach((w: any, idx: number) => { w.sort = arr.length - idx; });
}
function moveWikiItem(i: number, dir: -1 | 1) {
  moveItem(data.value.wiki, i, dir);
  syncWikiSort();
}

function togglePlatform(p: string) {
  const arr = data.value.article.share.platforms as string[];
  const i = arr.indexOf(p);
  if (i >= 0) arr.splice(i, 1);
  else arr.push(p);
}
function addRightbarLink() { data.value.rightbar.welcome_linklist.items.push({ title: "", url: "", icon: "" }); }
function addSocial() { data.value.footer.social.push({ title: "", url: "", icon: "", icon_key: "" }); }
function addSitemapGroup() { data.value.footer.sitemap.push({ title: "", items: [] }); }
function addSitemapItem(g: any) {
  g.items = g.items || [];
  g.items.push({ title: "", url: "" });
}
function addWikiProject() {
  data.value.wiki.push({ id: "", name: "", title: "", description: "", icon: "", tags: [], url: "", repo: "", preview: "auto", sort: 0, _tagsText: "" });
}

const inputCls = "apple-input";
const textareaCls = "apple-textarea";
const codeAreaCls = "apple-textarea code-style";
const selectCls = "apple-select";

function selectGroup(key: string) {
  activeGroup.value = key;
  const group = groups.find((g) => g.key === key);
  if (group) active.value = group.tabs[0].key;
}

const currentGroupTabs = computed(() => groups.find((g) => g.key === activeGroup.value)?.tabs ?? []);
const currentGroup = computed(() => groups.find((g) => g.key === activeGroup.value) ?? groups[0]);
const currentTab = computed(() => currentGroupTabs.value.find((t) => t.key === active.value) ?? currentGroupTabs.value[0]);

function saveActive() {
  switch (active.value) {
    case "menubar": save("menubar"); break;
    case "rightbar": save("rightbar"); break;
    case "footer": save("footer"); break;
    case "wiki": save("wiki", wikiPayload()); break;
    case "comments": save("comments"); break;
    case "article": save("article"); break;
    case "seo": save("seo"); break;
    case "inject": save("inject"); break;
    case "ai": save("ai"); break;
    case "appearance": save("style"); break;
    case "cover": save("cover"); break;
    case "nav_tabs": save("nav_tabs"); break;
    case "widgets": save("widgets"); break;
    case "plugins": save("plugins"); break;
    case "authors": save("authors"); break;
    case "lang": save("lang"); break;
    case "site": save("site"); break;
    default: break;
  }
}

const saveDisabled = computed(() => loading.value || !!loadError.value || active.value === "advanced" || savingKey.value !== "");
const activeSaving = computed(() => savingKey.value === active.value || savingKey.value === "json:" + active.value);

onMounted(() => {
  const tab = route.query.tab;
  if (typeof tab === "string") {
    const group = groups.find((g) => g.tabs.some((t) => t.key === tab));
    if (group) {
      activeGroup.value = group.key;
      active.value = tab;
    }
  }
  load();
});
</script>

<template>
  <div class="settings-view">
    <header class="settings-page-header">
      <div class="settings-page-header-title">
        <Icon icon="ri:settings-3-line" class="settings-page-header-icon" />
        <span>设置</span>
      </div>
      <VButton type="primary" size="sm" :disabled="saveDisabled" :loading="activeSaving" @click="saveActive">
        <template #icon><Icon icon="ri:save-3-line" /></template>
        保存更改
      </VButton>
    </header>

    <div class="settings-container">
      <nav class="settings-nav">
        <div class="settings-nav-list">
          <button
            v-for="g in groups"
            :key="g.key"
            type="button"
            class="settings-nav-item"
            :class="{ active: activeGroup === g.key }"
            @click="selectGroup(g.key)"
          >
            <span class="settings-nav-item-icon"><Icon :icon="g.icon" width="16" height="16" /></span>
            <span>{{ g.label }}</span>
          </button>
        </div>
      </nav>

      <div class="settings-content">
        <div>
          <div class="settings-section-title">{{ currentTab.label }}</div>
          <div class="settings-section-desc">{{ currentGroup.desc }}</div>
        </div>

        <div v-if="currentGroupTabs.length > 1" class="apple-segmented-rect settings-subtabs">
          <button
            v-for="t in currentGroupTabs"
            :key="t.key"
            type="button"
            :class="{ active: active === t.key }"
            @click="active = t.key"
          >
            {{ t.label }}
          </button>
        </div>

        <div v-if="loading" class="settings-state">
          <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" />
          <span>加载中...</span>
        </div>
        <div v-else-if="loadError" class="settings-state settings-state-error">
          <Icon icon="ri:error-warning-line" width="22" height="22" />
          <span>设置加载失败：{{ loadError }}</span>
          <VButton type="secondary" size="sm" class="mt-2" @click="load">重试</VButton>
        </div>

        <template v-else>
          <VCard v-show="active === 'site'" title="站点信息">
            <div class="form-grid form-grid-2">
              <div class="field"><label>站点标题</label><input v-model="data.site.title" :class="inputCls" /></div>
              <div class="field"><label>副标题</label><input v-model="data.site.subtitle" :class="inputCls" /></div>
            </div>
            <div class="field"><label>站点描述</label><textarea v-model="data.site.description" rows="2" :class="textareaCls"></textarea></div>
            <div class="field"><label>站点 URL（canonical/OG/sitemap 用）</label><input v-model="data.site.url" :class="inputCls" placeholder="https://example.com" /></div>

            <div class="form-grid form-grid-2">
              <div class="field">
                <label>头像 URL</label>
                <div class="field-inline">
                  <input v-model="data.site.avatar" :class="inputCls" placeholder="输入图片 URL" />
                  <button type="button" class="icon-btn" title="从附件选择" @click="showAvatarPicker = true"><Icon icon="ri:image-upload-line" /></button>
                </div>
              </div>
              <div class="field">
                <label>头像悬停背景图</label>
                <div class="field-inline">
                  <input v-model="data.site.avatar_bg" :class="inputCls" placeholder="输入图片 URL" />
                  <button type="button" class="icon-btn" title="从附件选择" @click="showAvatarBgPicker = true"><Icon icon="ri:image-upload-line" /></button>
                </div>
              </div>
            </div>

            <div v-if="data.site.avatar || data.site.avatar_bg" class="preview-box">
              <p class="preview-label">实时预览</p>
              <div class="preview-row">
                <div v-if="data.site.avatar" class="preview-item">
                  <p class="preview-item-label">导航栏头像 32×32</p>
                  <div class="avatar-32"><img :src="data.site.avatar" alt="" /></div>
                </div>
                <div v-if="data.site.avatar" class="preview-item">
                  <p class="preview-item-label">分享卡片 128×128</p>
                  <div class="avatar-128"><img :src="data.site.avatar" alt="" /></div>
                </div>
                <div v-if="data.site.avatar && data.site.avatar_bg" class="preview-item">
                  <p class="preview-item-label">悬停效果</p>
                  <div class="avatar-hover">
                    <img :src="data.site.avatar_bg" class="avatar-hover-bg" alt="" />
                    <img :src="data.site.avatar" class="avatar-hover-fg" alt="" />
                  </div>
                </div>
              </div>
            </div>

            <div class="form-grid form-grid-2">
              <div class="field"><label>Logo 主标题</label><input v-model="data.site.logo_title" :class="inputCls" /></div>
              <div class="field"><label>Logo 副标题（含 | 拆滚动）</label><input v-model="data.site.logo_sub" :class="inputCls" /></div>
            </div>

            <div class="form-grid form-grid-2">
              <div class="field">
                <label>Favicon</label>
                <div class="field-inline">
                  <img :src="data.site.favicon_url || '/favicon.ico'" class="favicon-img" alt="" />
                  <input v-model="data.site.favicon_url" :class="inputCls" placeholder="留空用默认 /favicon.ico" />
                  <button type="button" class="icon-btn" title="从附件选择" @click="showFaviconPicker = true"><Icon icon="ri:image-upload-line" /></button>
                </div>
              </div>
              <div class="field">
                <label>Apple Touch Icon</label>
                <div class="field-inline">
                  <img :src="data.site.apple_touch_icon || '/apple-touch-icon.png'" class="touch-img" alt="" />
                  <input v-model="data.site.apple_touch_icon" :class="inputCls" placeholder="留空用默认" />
                  <button type="button" class="icon-btn" title="从附件选择" @click="showAppleIconPicker = true"><Icon icon="ri:image-upload-line" /></button>
                </div>
              </div>
            </div>

            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'site'" @click="save('site')">保存站点信息</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'menubar'" title="顶部菜单">
            <MenubarEditor
              :items="data.menubar.items"
              :columns="data.menubar.columns"
              @update:columns="data.menubar.columns = $event"
              @save="save('menubar')"
            />
          </VCard>

          <VCard v-show="active === 'rightbar'" title="右栏设置">
            <div class="field"><label>欢迎语正文（留空用默认文案，Spec Task 5）</label><textarea v-model="data.rightbar.intro" rows="3" :class="textareaCls" placeholder="输入右栏欢迎语正文，换行会保留显示"></textarea></div>
            <p class="hint">首页右栏「欢迎语」组件内的链接列表（welcome_linklist）。</p>
            <div class="field field-narrow"><label>列数（0=自动）</label><input v-model.number="data.rightbar.welcome_linklist.columns" type="number" min="0" :class="inputCls" /></div>
            <div class="editable-list">
              <div v-for="(it, i) in data.rightbar.welcome_linklist.items" :key="i" class="editable-row">
                <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-3" />
                <input v-model="it.url" placeholder="/url 或 https://..." :class="inputCls" class="col-4" />
                <input v-model="it.icon" placeholder="icon（图片 URL 或 icons.yml key）" :class="inputCls" class="col-3" />
                <div class="row-actions">
                  <button type="button" :disabled="Number(i) === 0" class="row-btn" @click="moveItem(data.rightbar.welcome_linklist.items, Number(i), -1)"><Icon icon="ri:arrow-up-s-line" /></button>
                  <button type="button" :disabled="Number(i) === data.rightbar.welcome_linklist.items.length - 1" class="row-btn" @click="moveItem(data.rightbar.welcome_linklist.items, Number(i), 1)"><Icon icon="ri:arrow-down-s-line" /></button>
                  <button type="button" class="row-btn row-btn-danger" @click="removeAt(data.rightbar.welcome_linklist.items, Number(i))"><Icon icon="ri:delete-bin-line" /></button>
                </div>
              </div>
            </div>
            <button type="button" class="add-btn" @click="addRightbarLink"><Icon icon="ri:add-line" /> 添加链接</button>
            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'rightbar'" @click="save('rightbar')">保存右栏</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'footer'" title="页脚设置">
            <div class="subsection">
              <p class="hint">社交图标（icon 填 raw HTML 如 &lt;img src="..."&gt;，icon_key 填 icons.yml 的 key，二选一，icon 优先）</p>
              <div v-for="(s, i) in data.footer.social" :key="i" class="editable-row">
                <input v-model="s.title" placeholder="标题" :class="inputCls" class="col-2" />
                <input v-model="s.url" placeholder="链接" :class="inputCls" class="col-3" />
                <input v-model="s.icon_key" placeholder="icon_key" :class="inputCls" class="col-2" />
                <input v-model="s.icon" placeholder="icon raw HTML（优先）" :class="inputCls" class="col-3" />
                <div class="row-actions">
                  <button type="button" :disabled="Number(i) === 0" class="row-btn" @click="moveItem(data.footer.social, Number(i), -1)"><Icon icon="ri:arrow-up-s-line" /></button>
                  <button type="button" :disabled="Number(i) === data.footer.social.length - 1" class="row-btn" @click="moveItem(data.footer.social, Number(i), 1)"><Icon icon="ri:arrow-down-s-line" /></button>
                  <button type="button" class="row-btn row-btn-danger" @click="removeAt(data.footer.social, Number(i))"><Icon icon="ri:delete-bin-line" /></button>
                </div>
              </div>
              <button type="button" class="add-btn" @click="addSocial"><Icon icon="ri:add-line" /> 添加社交图标</button>
            </div>
            <div class="subsection">
              <p class="hint">站点地图分组（页脚多列链接）</p>
              <div v-for="(g, gi) in data.footer.sitemap" :key="gi" class="nested-group">
                <div class="nested-group-head">
                  <input v-model="g.title" placeholder="组名" :class="inputCls" class="flex-1" />
                  <button type="button" :disabled="Number(gi) === 0" class="row-btn" @click="moveItem(data.footer.sitemap, Number(gi), -1)"><Icon icon="ri:arrow-up-s-line" /></button>
                  <button type="button" :disabled="Number(gi) === data.footer.sitemap.length - 1" class="row-btn" @click="moveItem(data.footer.sitemap, Number(gi), 1)"><Icon icon="ri:arrow-down-s-line" /></button>
                  <button type="button" class="row-btn row-btn-danger" @click="removeAt(data.footer.sitemap, Number(gi))"><Icon icon="ri:delete-bin-line" /></button>
                </div>
                <div v-for="(it, ii) in g.items" :key="ii" class="editable-row nested-row">
                  <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-4" />
                  <input v-model="it.url" placeholder="/url" :class="inputCls" class="col-6" />
                  <div class="row-actions">
                    <button type="button" :disabled="Number(ii) === 0" class="row-btn" @click="moveItem(g.items, Number(ii), -1)"><Icon icon="ri:arrow-up-s-line" /></button>
                    <button type="button" :disabled="Number(ii) === g.items.length - 1" class="row-btn" @click="moveItem(g.items, Number(ii), 1)"><Icon icon="ri:arrow-down-s-line" /></button>
                    <button type="button" class="row-btn row-btn-danger" @click="removeAt(g.items, Number(ii))"><Icon icon="ri:delete-bin-line" /></button>
                  </div>
                </div>
                <button type="button" class="add-btn add-btn-indent" @click="addSitemapItem(g)"><Icon icon="ri:add-line" /> 添加链接</button>
              </div>
              <button type="button" class="add-btn" @click="addSitemapGroup"><Icon icon="ri:add-line" /> 添加分组</button>
            </div>
            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'footer'" @click="save('footer')">保存页脚</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'wiki'" title="文库项目">
            <p class="hint">/wiki 页项目卡片。拖拽排序；tags 逗号分隔。</p>
            <div
              v-for="(w, i) in data.wiki"
              :key="i"
              draggable="true"
              class="wiki-card"
              :class="{ 'wiki-card-dragging': dragWikiIndex === Number(i) }"
              @dragstart="onWikiDragStart(Number(i))"
              @dragover="onWikiDragOver($event, Number(i))"
              @dragend="onWikiDragEnd"
            >
              <div class="wiki-card-head">
                <Icon icon="ri:drag-move-2-line" class="drag-handle" />
                <div v-if="w.icon" class="wiki-icon"><img :src="w.icon" alt="" @error="($event.target as HTMLImageElement).style.display = 'none'" /></div>
                <div v-else class="wiki-icon wiki-icon-empty"><Icon icon="ri:booklet-line" /></div>
                <span class="wiki-title">{{ w.title || w.name || w.id || "新项目" }}</span>
                <div class="row-actions">
                  <button type="button" :disabled="Number(i) === 0" class="row-btn" title="上移" @click="moveWikiItem(Number(i), -1)"><Icon icon="ri:arrow-up-s-line" /></button>
                  <button type="button" :disabled="Number(i) === data.wiki.length - 1" class="row-btn" title="下移" @click="moveWikiItem(Number(i), 1)"><Icon icon="ri:arrow-down-s-line" /></button>
                  <button type="button" class="row-btn row-btn-danger" title="删除" @click="removeAt(data.wiki, Number(i))"><Icon icon="ri:delete-bin-line" /></button>
                </div>
              </div>
              <div class="form-grid form-grid-3">
                <div class="field"><label>id</label><input v-model="w.id" :class="inputCls" /></div>
                <div class="field"><label>标题</label><input v-model="w.title" :class="inputCls" /></div>
                <div class="field"><label>排序（大在前）</label><input v-model.number="w.sort" type="number" :class="inputCls" /></div>
              </div>
              <div class="field"><label>描述</label><input v-model="w.description" :class="inputCls" /></div>
              <div class="form-grid form-grid-3">
                <div class="field"><label>图标（图片 URL）</label><input v-model="w.icon" :class="inputCls" placeholder="https://..." /></div>
                <div class="field"><label>GitHub 仓库</label><input v-model="w.repo" :class="inputCls" placeholder="owner/repo" /></div>
                <div class="field">
                  <label>预览布局</label>
                  <select v-model="w.preview" :class="selectCls">
                    <option value="auto">auto（自动）</option>
                    <option value="wide">wide（宽版）</option>
                    <option value="none">none（无预览）</option>
                  </select>
                </div>
              </div>
              <div class="field">
                <label>链接 URL</label>
                <div class="field-inline">
                  <input v-model="w.url" :class="inputCls" placeholder="/posts/xxx 或 https://..." />
                  <button type="button" class="icon-btn" :disabled="!w.id" title="链接到 wiki 项目文档" @click="w.url = '/wiki/' + w.id"><Icon icon="ri:link-m" /></button>
                </div>
              </div>
              <div class="field"><label>标签（逗号分隔）</label><input v-model="w._tagsText" :class="inputCls" placeholder="Go, 开源" /></div>
            </div>
            <button type="button" class="add-btn" @click="addWikiProject"><Icon icon="ri:add-line" /> 添加项目</button>
            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'wiki'" @click="save('wiki', wikiPayload())">保存文库</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'comments'" title="评论设置">
            <div class="switch-row">
              <div class="switch-row-label">
                <div class="switch-row-name">启用评论</div>
                <div class="switch-row-desc">关闭需重启才断开路由</div>
              </div>
              <VSwitch v-model="data.comments.enabled" />
            </div>
            <div class="switch-row">
              <div class="switch-row-label">
                <div class="switch-row-name">先审后发</div>
                <div class="switch-row-desc">新评论需审核后才显示</div>
              </div>
              <VSwitch v-model="data.comments.moderation" />
            </div>
            <div class="form-grid form-grid-2">
              <div class="field"><label>评论区标题</label><input v-model="data.comments.title" :class="inputCls" /></div>
              <div class="field"><label>博主徽章文案</label><input v-model="data.comments.admin_badge" :class="inputCls" /></div>
            </div>
            <div class="form-grid form-grid-2">
              <div class="field"><label>博主邮箱</label><input v-model="data.comments.admin_email" :class="inputCls" /></div>
              <div class="field"><label>Gravatar 镜像</label><input v-model="data.comments.gravatar_mirror" :class="inputCls" /></div>
            </div>
            <div class="subsection">
              <p class="subsection-title">邮件通知（SMTP，主机留空则关闭）</p>
              <div class="form-grid form-grid-2">
                <div class="field"><label>SMTP 主机</label><input v-model="data.comments.notify.smtp_host" :class="inputCls" /></div>
                <div class="field"><label>端口</label><input v-model.number="data.comments.notify.smtp_port" type="number" :class="inputCls" /></div>
                <div class="field"><label>用户名</label><input v-model="data.comments.notify.username" :class="inputCls" /></div>
                <div class="field"><label>密码</label><input v-model="data.comments.notify.password" type="password" :class="inputCls" /></div>
                <div class="field"><label>发件人（留空用用户名）</label><input v-model="data.comments.notify.from" :class="inputCls" /></div>
              </div>
            </div>
            <div class="card-actions card-actions-inline">
              <VButton type="primary" size="sm" :loading="savingKey === 'comments'" @click="save('comments')">保存评论设置</VButton>
              <VButton type="secondary" size="sm" :disabled="testing" @click="testMail">{{ testing ? "发送中..." : "发送测试邮件" }}</VButton>
              <span v-if="testResult" class="test-result" :class="testOk ? 'is-ok' : 'is-err'">{{ testResult }}</span>
            </div>
          </VCard>

          <VCard v-show="active === 'article'" title="文章设置">
            <div class="subsection">
              <p class="subsection-title">版权声明</p>
              <div class="switch-row">
                <div class="switch-row-label"><div class="switch-row-name">显示版权声明</div></div>
                <VSwitch v-model="data.article.copyright.enabled" />
              </div>
              <div class="form-grid form-grid-2">
                <div class="field"><label>协议名</label><input v-model="data.article.copyright.license" :class="inputCls" /></div>
                <div class="field"><label>协议链接</label><input v-model="data.article.copyright.license_url" :class="inputCls" /></div>
              </div>
            </div>
            <div class="subsection">
              <p class="subsection-title">分享平台</p>
              <div class="chip-group">
                <label v-for="p in platformOptions" :key="p" class="chip" :class="{ active: data.article.share.platforms.includes(p) }">
                  <input type="checkbox" :checked="data.article.share.platforms.includes(p)" @change="togglePlatform(p)" />
                  <span>{{ p }}</span>
                </label>
              </div>
              <div class="switch-row">
                <div class="switch-row-label"><div class="switch-row-name">启用分享</div></div>
                <VSwitch v-model="data.article.share.enabled" />
              </div>
            </div>
            <div class="subsection">
              <p class="subsection-title">打赏</p>
              <div class="switch-row">
                <div class="switch-row-label"><div class="switch-row-name">启用打赏</div></div>
                <VSwitch v-model="data.article.reward.enabled" />
              </div>
              <div class="form-grid form-grid-3">
                <div class="field"><label>按钮文案</label><input v-model="data.article.reward.text" :class="inputCls" /></div>
                <div class="field"><label>微信收款码 URL</label><input v-model="data.article.reward.wechat" :class="inputCls" /></div>
                <div class="field"><label>支付宝收款码 URL</label><input v-model="data.article.reward.alipay" :class="inputCls" /></div>
              </div>
            </div>
            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'article'" @click="save('article')">保存文章设置</VButton>
            </div>
            <div class="subsection friends-subsection">
              <p class="subsection-title">友链 RSS 聚合（friends 段，独立保存）</p>
              <div class="form-grid form-grid-2">
                <div class="field"><label>刷新间隔（分钟，≤0 用默认 60）</label><input v-model.number="data.friends.refresh_interval" type="number" :class="inputCls" /></div>
                <div class="field"><label>每友链文章数量（≤0 用默认 20）</label><input v-model.number="data.friends.posts_per_link" type="number" min="1" :class="inputCls" /></div>
              </div>
              <VButton type="secondary" size="sm" :loading="savingKey === 'friends'" @click="save('friends')">保存 RSS 设置</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'seo'" title="SEO 设置">
            <div class="field"><label>默认 Meta Description（文章未单独设置时使用）</label><textarea v-model="data.seo.default_description" rows="2" :class="textareaCls"></textarea></div>
            <div class="field"><label>默认 Meta Keywords（逗号分隔）</label><input v-model="data.seo.default_keywords" :class="inputCls" placeholder="博客, Go, 前端" /></div>
            <div class="field"><label>robots.txt 附加规则（原样追加到默认规则之后）</label><textarea v-model="data.seo.robots_extra" rows="4" :class="codeAreaCls" placeholder="User-agent: GPTBot&#10;Disallow: /"></textarea></div>
            <div class="switch-row">
              <div class="switch-row-label">
                <div class="switch-row-name">禁止搜索引擎收录</div>
                <div class="switch-row-desc">全站输出 noindex meta，请谨慎开启</div>
              </div>
              <VSwitch v-model="data.seo.no_index" />
            </div>
            <div class="subsection">
              <p class="subsection-title">站点验证（留空不输出对应 meta 标签）</p>
              <div class="form-grid form-grid-3">
                <div class="field"><label>Google 验证码</label><input v-model="data.seo.google_verification" :class="inputCls" /></div>
                <div class="field"><label>Bing 验证码</label><input v-model="data.seo.bing_verification" :class="inputCls" /></div>
                <div class="field"><label>百度验证码</label><input v-model="data.seo.baidu_verification" :class="inputCls" /></div>
              </div>
            </div>
            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'seo'" @click="save('seo')">保存 SEO 设置</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'inject'" title="代码注入">
            <div class="warning-hint">
              <Icon icon="ri:error-warning-line" width="14" height="14" />
              <span>注入内容将不经转义原样输出到前台所有页面，可执行任意脚本。请只粘贴你完全信任的代码（统计、字体、自定义样式等），错误或恶意代码可能导致页面故障与安全风险，由此产生的后果由站长自行承担。</span>
            </div>
            <div class="field"><label>头部注入（输出在 &lt;/head&gt; 之前）</label><textarea v-model="data.inject.inject_head" rows="6" :class="codeAreaCls" placeholder="<script>...</script> 或 <link ...>"></textarea></div>
            <div class="field"><label>页脚注入（输出在 &lt;/body&gt; 之前）</label><textarea v-model="data.inject.inject_footer" rows="6" :class="codeAreaCls" placeholder="<script>...</script>"></textarea></div>
            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'inject'" @click="save('inject')">保存代码注入</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'ai'" title="AI 属性生成">
            <p class="hint">编辑器「AI 属性生成」使用的 OpenAI 兼容接口（DeepSeek / 通义 / Ollama 等均可）。</p>
            <div class="field"><label>API 地址（如 https://api.deepseek.com/v1）</label><input v-model="data.ai.base_url" :class="inputCls" placeholder="https://api.openai.com/v1" /></div>
            <div class="field"><label>API Key（本地模型可留空；已保存时显示 ****，留空保存不覆盖）</label><input v-model="data.ai.api_key" type="password" autocomplete="off" :class="inputCls" /></div>
            <div class="field"><label>模型名（如 deepseek-chat / gpt-4o-mini）</label><input v-model="data.ai.model" :class="inputCls" /></div>
            <div class="subsection">
              <p class="subsection-title">AI 文章摘要</p>
              <div class="switch-row">
                <div class="switch-row-label"><div class="switch-row-desc">前台展示 AI 摘要（关闭后文章页不渲染摘要块）</div></div>
                <VSwitch v-model="data.ai.summary_enabled" />
              </div>
              <div class="switch-row">
                <div class="switch-row-label"><div class="switch-row-desc">允许读者临时接入自己的 API 生成摘要（Key 不落本站）</div></div>
                <VSwitch v-model="data.ai.allow_user_api" />
              </div>
            </div>
            <div class="card-actions">
              <VButton type="primary" size="sm" :loading="savingKey === 'ai'" @click="save('ai')">保存 AI 设置</VButton>
            </div>
          </VCard>

          <VCard v-show="active === 'security'" title="安全设置">
            <div class="warning-hint">
              <Icon icon="ri:information-line" width="14" height="14" />
              <span>密码管理已迁移至 <router-link to="/profile" class="link">个人资料页</router-link>。</span>
            </div>
            <p class="hint">忘记密码：删除数据库 settings 表 admin 段后重启，回落 site.yml 初始密码。</p>
          </VCard>

          <VCard v-show="active === 'advanced'" title="高级 · JSON 兜底编辑">
            <p class="hint">全部段 JSON 兜底编辑，一般无需使用（结构化表单已覆盖）。</p>
            <div v-for="s in allSections" :key="s" class="json-block">
              <div class="json-block-head">
                <span class="json-block-title">{{ s }}</span>
                <VButton type="secondary" size="xs" :loading="savingKey === 'json:' + s" @click="saveJson(s)">保存 {{ s }}</VButton>
              </div>
              <textarea v-model="rawText[s]" rows="8" :class="codeAreaCls"></textarea>
            </div>
          </VCard>

          <VCard v-show="active === 'appearance'" title="外观设置">
            <AppearanceTab :data="data" :saving="savingKey" @save="save" />
          </VCard>
          <VCard v-show="active === 'widgets'" title="组件编排">
            <WidgetsTab :data="data" :saving="savingKey" @save="save" />
          </VCard>
          <VCard v-show="active === 'plugins'" title="插件">
            <PluginsTab :data="data.plugins" :saving="savingKey" @save="() => save('plugins')" />
          </VCard>
          <VCard v-show="active === 'cover'" title="首页 Hero">
            <CoverTab :data="data.cover" :saving="savingKey" @save="() => save('cover')" />
          </VCard>
          <VCard v-show="active === 'nav_tabs'" title="导航标签">
            <NavTabsTab :data="data.nav_tabs" :saving="savingKey" @save="() => save('nav_tabs')" />
          </VCard>
          <VCard v-show="active === 'authors'" title="作者管理">
            <AuthorsTab :data="data.authors" :saving="savingKey" @save="() => save('authors')" />
          </VCard>
          <VCard v-show="active === 'lang'" title="前台文案覆盖">
            <LangTab :data="data.lang" :saving="savingKey" @save="() => save('lang')" />
          </VCard>
        </template>
      </div>
    </div>

    <AttachmentPickerModal :open="showAvatarPicker" @select="handleAvatarSelect" @close="showAvatarPicker = false" />
    <AttachmentPickerModal :open="showAvatarBgPicker" @select="handleAvatarBgSelect" @close="showAvatarBgPicker = false" />
    <AttachmentPickerModal :open="showFaviconPicker" @select="handleFaviconSelect" @close="showFaviconPicker = false" />
    <AttachmentPickerModal :open="showAppleIconPicker" @select="handleAppleIconSelect" @close="showAppleIconPicker = false" />
  </div>
</template>

<style scoped>
.settings-view { min-height: 100%; background: var(--color-background); }
.settings-page-header { height: 56px; padding: 0 28px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--color-border); background: rgba(0,0,0,0.6); backdrop-filter: saturate(180%) blur(20px); -webkit-backdrop-filter: saturate(180%) blur(20px); position: sticky; top: 0; z-index: 20; }
.settings-page-header-title { display: flex; align-items: center; gap: 10px; font-size: 19px; font-weight: 600; letter-spacing: -0.2px; color: var(--color-foreground); }
.settings-page-header-icon { color: var(--color-primary); font-size: 20px; }
.settings-container { display: flex; gap: 28px; padding: 28px; max-width: 1100px; margin: 0 auto; width: 100%; box-sizing: border-box; }
.settings-nav { width: 200px; flex-shrink: 0; padding-top: 4px; }
.settings-nav-list { display: flex; flex-direction: column; gap: 2px; position: sticky; top: 84px; }
.settings-nav-item { display: flex; align-items: center; gap: 10px; padding: 9px 12px; border-radius: 8px; font-size: 13.5px; color: var(--color-text-tertiary); cursor: pointer; transition: all 0.15s ease; position: relative; border-left: 3px solid transparent; margin-left: -3px; text-align: left; background: transparent; }
.settings-nav-item:hover { background: rgba(255,255,255,0.04); color: var(--color-foreground); }
.settings-nav-item.active { color: var(--color-primary); font-weight: 500; border-left-color: var(--color-primary); background: rgba(10,132,255,0.08); }
.settings-nav-item-icon { display: inline-flex; align-items: center; justify-content: center; opacity: 0.8; }
.settings-nav-item.active .settings-nav-item-icon { opacity: 1; }
.settings-content { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 18px; }
.settings-section-title { font-size: 22px; font-weight: 600; color: var(--color-foreground); letter-spacing: -0.3px; }
.settings-section-desc { font-size: 13px; color: var(--color-text-tertiary); margin-top: 4px; }
.settings-subtabs { align-self: flex-start; flex-wrap: wrap; }
.settings-state { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; padding: 60px 0; font-size: 13px; color: var(--color-text-tertiary); }
.settings-state-error { color: var(--color-state-error); }
.settings-spin { animation: settings-spin 1s linear infinite; }
@keyframes settings-spin { to { transform: rotate(360deg); } }
.field { margin-bottom: 14px; }
.field:last-child { margin-bottom: 0; }
.field label { display: block; font-size: 12px; color: var(--color-text-tertiary); margin-bottom: 6px; }
.field-narrow { max-width: 160px; }
.field-inline { display: flex; gap: 8px; align-items: center; }
.field-inline .apple-input { flex: 1; }
.form-grid { display: grid; gap: 14px; margin-bottom: 14px; }
.form-grid:last-child { margin-bottom: 0; }
.form-grid-2 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.form-grid-3 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.hint { font-size: 12px; color: var(--color-text-quaternary); margin: 0 0 12px; }
.subsection { margin-top: 18px; padding-top: 18px; border-top: 1px solid var(--color-border); }
.subsection-title { font-size: 12px; font-weight: 600; color: var(--color-text-secondary); margin: 0 0 12px; }
.card-actions { display: flex; align-items: center; gap: 10px; margin-top: 18px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.card-actions-inline { flex-wrap: wrap; }
.friends-subsection { margin-top: 22px; }
.icon-btn { flex-shrink: 0; display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; border-radius: var(--radius-sm); background: rgba(10,132,255,0.12); color: var(--color-primary); border: none; cursor: pointer; transition: background 0.15s ease; }
.icon-btn:hover:not(:disabled) { background: rgba(10,132,255,0.2); }
.icon-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.add-btn { display: inline-flex; align-items: center; gap: 4px; font-size: 13px; color: var(--color-primary); background: transparent; border: none; cursor: pointer; padding: 6px 0; transition: opacity 0.15s ease; }
.add-btn:hover { opacity: 0.8; }
.add-btn-indent { padding-left: 16px; }
.editable-list { display: flex; flex-direction: column; gap: 8px; margin-bottom: 10px; }
.editable-row { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); gap: 8px; align-items: center; }
.col-2 { grid-column: span 2; }
.col-3 { grid-column: span 3; }
.col-4 { grid-column: span 4; }
.col-6 { grid-column: span 6; }
.row-actions { grid-column: span 2; display: flex; align-items: center; gap: 4px; justify-content: flex-end; }
.row-btn { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; border-radius: 6px; color: var(--color-text-quaternary); background: transparent; border: none; cursor: pointer; transition: all 0.15s ease; }
.row-btn:hover:not(:disabled) { background: var(--color-secondary); color: var(--color-foreground); }
.row-btn:disabled { opacity: 0.25; cursor: not-allowed; }
.row-btn-danger { color: var(--color-state-error); }
.row-btn-danger:hover:not(:disabled) { background: rgba(255,69,58,0.12); color: var(--color-state-error); }
.nested-group { border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 14px; margin-bottom: 10px; display: flex; flex-direction: column; gap: 8px; background: var(--color-muted); }
.nested-group-head { display: flex; align-items: center; gap: 6px; }
.preview-box { border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 16px; margin-bottom: 14px; }
.preview-label { font-size: 12px; font-weight: 500; color: var(--color-text-tertiary); margin: 0 0 12px; }
.preview-row { display: flex; flex-wrap: wrap; gap: 24px; align-items: flex-end; }
.preview-item { text-align: center; }
.preview-item-label { font-size: 10px; color: var(--color-text-tertiary); margin: 0 0 8px; }
.avatar-32 { width: 32px; height: 32px; border-radius: 9999px; overflow: hidden; border: 1px solid var(--color-border); }
.avatar-128 { width: 128px; height: 128px; border-radius: 9999px; overflow: hidden; border: 1px solid var(--color-border); }
.avatar-32 img, .avatar-128 img { width: 100%; height: 100%; object-fit: cover; }
.avatar-hover { position: relative; width: 96px; height: 96px; border-radius: 9999px; overflow: hidden; border: 2px solid transparent; transition: border-color 0.2s ease; }
.avatar-hover:hover { border-color: var(--color-primary); }
.avatar-hover-bg { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
.avatar-hover-fg { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; transition: transform 0.3s ease; }
.avatar-hover:hover .avatar-hover-fg { transform: scale(1.1); }
.favicon-img { width: 24px; height: 24px; border-radius: 4px; flex-shrink: 0; }
.touch-img { width: 32px; height: 32px; border-radius: var(--radius-sm); flex-shrink: 0; }
.switch-row { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 12px 0; border-bottom: 1px solid var(--color-border); }
.switch-row:last-child { border-bottom: none; }
.switch-row-name { font-size: 13.5px; font-weight: 500; color: var(--color-foreground); }
.switch-row-desc { font-size: 12px; color: var(--color-text-tertiary); margin-top: 3px; line-height: 1.4; max-width: 460px; }
.warning-hint { display: flex; align-items: flex-start; gap: 8px; font-size: 12px; line-height: 1.6; color: var(--color-state-warning); background: rgba(255,159,10,0.08); border: 1px solid rgba(255,159,10,0.25); border-radius: var(--radius-md); padding: 12px 14px; margin-bottom: 14px; }
.warning-hint .link { color: var(--color-primary); text-decoration: none; font-weight: 500; }
.warning-hint .link:hover { text-decoration: underline; }
.chip-group { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.chip { display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; font-size: 12.5px; color: var(--color-text-secondary); background: var(--color-secondary); border: 1px solid var(--color-border); border-radius: 9999px; cursor: pointer; transition: all 0.15s ease; user-select: none; }
.chip input { position: absolute; opacity: 0; pointer-events: none; }
.chip:hover { border-color: var(--color-border-strong, var(--color-border)); color: var(--color-foreground); }
.chip.active { background: rgba(10,132,255,0.12); border-color: rgba(10,132,255,0.4); color: var(--color-primary); }
.test-result { font-size: 12px; }
.test-result.is-ok { color: var(--color-state-success); }
.test-result.is-err { color: var(--color-state-error); }
.json-block { margin-bottom: 18px; }
.json-block:last-child { margin-bottom: 0; }
.json-block-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.json-block-title { font-size: 12px; font-weight: 600; color: var(--color-text-secondary); font-family: var(--font-mono, ui-monospace, monospace); }
.wiki-card { border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 16px; margin-bottom: 12px; transition: all 0.2s ease; }
.wiki-card:hover { border-color: var(--color-border-strong, var(--color-border)); }
.wiki-card-dragging { opacity: 0.4; }
.wiki-card-head { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.drag-handle { color: var(--color-text-quaternary); font-size: 18px; cursor: grab; flex-shrink: 0; }
.drag-handle:active { cursor: grabbing; }
.wiki-icon { width: 32px; height: 32px; border-radius: var(--radius-sm); overflow: hidden; border: 1px solid var(--color-border); background: var(--color-secondary); flex-shrink: 0; display: flex; align-items: center; justify-content: center; }
.wiki-icon img { width: 100%; height: 100%; object-fit: cover; }
.wiki-icon-empty { color: var(--color-text-quaternary); }
.wiki-title { flex: 1; font-size: 14px; font-weight: 500; color: var(--color-text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 860px) {
  .settings-container { flex-direction: column; padding: 18px; }
  .settings-nav { width: 100%; }
  .settings-nav-list { position: static; flex-direction: row; flex-wrap: wrap; }
  .settings-nav-item { border-left: none; border-bottom: 3px solid transparent; margin-left: 0; margin-bottom: -3px; }
  .settings-nav-item.active { border-left: none; border-bottom-color: var(--color-primary); background: transparent; }
  .form-grid-2, .form-grid-3 { grid-template-columns: 1fr; }
}
</style>
