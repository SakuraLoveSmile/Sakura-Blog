<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { onBeforeRouteLeave, useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VPageHeader } from "@/components/ui";
import BasicSettings from "./tabs/BasicSettings.vue";
import FeedbackSettings from "./tabs/FeedbackSettings.vue";
import CanonicalSettingsForm from "./CanonicalSettingsForm.vue";
import SettingsNav from "./SettingsNav.vue";
import SettingsRawEditor from "./SettingsRawEditor.vue";
import SettingsSaveBar from "./SettingsSaveBar.vue";
import StellarSettingsStatus from "./StellarSettingsStatus.vue";
import { useStellarSettings } from "./useStellarSettings";
import { ALL_SETTINGS_SECTIONS, ensureSettingsDefaults, sortWikiProjects, deepCloneJsonSafe } from "./settings-state";
import { settingsPreviewPages, useSettingsPreview } from "./useSettingsPreview";
import {
  DEFAULT_SETTINGS_PAGE,
  resolveSettingsTab,
  settingsGroups,
  settingsPageKeys,
  settingsPages,
  settingsSectionAnchor,
  settingsThemePages,
  type SettingsHostSection,
  type SettingsPageKey,
} from "./settings-pages";
import { changeCount, parseDocument, pathHasOverride, restorePage, themeChangeSummary } from "./settings-page-state";
import "./settings-form.css";

// Shared defaults retain the existing frontend enhancement safety boundary: card_hover.enable = false; card_hover.max_tilt = 3.

const route = useRoute();
const router = useRouter();
const rawOpen = ref(false);
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(true);
const loadError = ref("");
const hostSavingKey = ref("");
const stellar = useStellarSettings();
const {
  draft: stellarDraft,
  saved: stellarSaved,
  loading: stellarLoading,
  busy: stellarBusy,
  loadError: stellarLoadError,
  dirty: stellarDirty,
  parseError: stellarParseError,
  pendingInput,
  loadRevision,
} = stellar;
const savingKey = computed(() => hostSavingKey.value || (stellarBusy.value ? "stellar2" : ""));
const pendingTabs = ref<Record<string, boolean>>({});
// 放弃本页修改后重建该页表单，清空它本地未同步的输入。
const pageRevision = reactive<Record<string, number>>({});
watch(
  pendingTabs,
  (tabs) => {
    pendingInput.value = Object.values(tabs).some(Boolean);
  },
  { deep: true, flush: "sync" },
);
watch(loadRevision, () => {
  pendingTabs.value = {};
});
const savedSnapshot = ref<string>("");
const savedJsonBaseline = ref<Record<string, string>>({});

// ---- 路由：?tab= 决定当前页；旧版 Tab 与分段键经 resolveSettingsTab 落到新页面或原跳转目标 ----
const mobileQuery =
  typeof window !== "undefined" && typeof window.matchMedia === "function"
    ? window.matchMedia("(max-width: 767px)")
    : undefined;
const isMobile = ref(Boolean(mobileQuery?.matches));
function syncMobile() {
  isMobile.value = Boolean(mobileQuery?.matches);
  // 跨过手机断点时内容区底部留白会变，保存栏的占位也要重新量取。
  void nextTick(measureSticky);
}
const hasTab = computed(() => typeof route.query.tab === "string" && route.query.tab !== "");
const active = computed<SettingsPageKey>(() => {
  const target = resolveSettingsTab(route.query.tab);
  return target.kind === "page" ? target.page : DEFAULT_SETTINGS_PAGE;
});
// 手机端：没有 ?tab= 时先看分组列表，选择后进入详情。
const showMobileList = computed(() => isMobile.value && !hasTab.value);

async function normalizeRoute() {
  const queryTab = route.query.tab;
  if (typeof queryTab !== "string" || !queryTab) return;
  const target = resolveSettingsTab(queryTab);
  if (target.kind === "route") {
    await router.replace({ path: target.path, query: target.query });
    return;
  }
  if (target.raw) rawOpen.value = true;
  if (target.page !== queryTab) await router.replace({ query: { ...route.query, tab: target.page } });
}

const dirtySections = computed<Set<string>>(() => {
  if (loading.value || !savedSnapshot.value) return new Set();
  const set = new Set<string>();
  try {
    const original = JSON.parse(savedSnapshot.value);
    for (const key of ALL_SETTINGS_SECTIONS) {
      const isFormDirty = JSON.stringify(data.value[key]) !== JSON.stringify(original[key]);
      const isJsonDirty = (rawText.value[key] ?? "") !== (savedJsonBaseline.value[key] ?? "");
      if (isFormDirty || isJsonDirty) {
        set.add(key);
      }
    }
  } catch {}
  return set;
});

const isDirty = computed(() => {
  if (stellarDirty.value) return true;
  if (dirtySections.value.size > 0) return true;
  for (const key of ALL_SETTINGS_SECTIONS) {
    if ((rawText.value[key] ?? "") !== (savedJsonBaseline.value[key] ?? "")) {
      return true;
    }
  }
  return false;
});

// ---- 未保存计数：主题页共享一份草稿，按页统计；站点信息与访客反馈各自对照最近一次加载/保存 ----
const themeSummary = computed(() => themeChangeSummary(stellarDraft.value, stellarSaved.value));
const savedSections = computed<Record<string, any>>(() => {
  try {
    return savedSnapshot.value ? JSON.parse(savedSnapshot.value) : {};
  } catch {
    return {};
  }
});
function hostChangeCount(section: SettingsHostSection): number {
  return Math.max(
    changeCount(data.value[section], savedSections.value[section]),
    dirtySections.value.has(section) ? 1 : 0,
  );
}
function pageDirty(page: SettingsPageKey): boolean {
  const host = settingsPages[page].host;
  if (host) return dirtySections.value.has(host);
  if (pendingTabs.value[page]) return true;
  if ((themeSummary.value.pages[page] ?? 0) > 0) return true;
  // 目录之外的参数、以及只改了原文格式的改动，都归在“代码与原文”。
  return (
    page === "advanced" &&
    (themeSummary.value.other > 0 || (stellarDraft.value !== stellarSaved.value && themeSummary.value.total === 0))
  );
}
const navGroups = computed(() =>
  settingsGroups.map((group) => ({
    key: group.key,
    label: group.label,
    pages: group.pages.map((key) => ({
      key,
      label: settingsPages[key].label,
      summary: settingsPages[key].summary,
      dirty: pageDirty(key),
    })),
  })),
);
const themeBreakdown = computed(() => [
  ...settingsThemePages.map((key) => ({ label: settingsPages[key].label, count: themeSummary.value.pages[key] ?? 0 })),
  { label: "其他参数", count: themeSummary.value.other },
]);
const otherThemeCount = computed(() => themeSummary.value.total);
const formsReady = computed(() => !stellarLoading.value && !stellarLoadError.value && !stellarParseError.value);
const draftDocument = computed(() => parseDocument(stellarDraft.value));
// 页面级“恢复默认”（目前只有主题外观）：草稿里存在显式覆盖，或本页有尚未同步的输入时才提供。
function pageReset(page: SettingsPageKey) {
  const reset = settingsPages[page].reset;
  return reset && (pendingTabs.value[page] || pathHasOverride(draftDocument.value, reset.path)) ? reset : undefined;
}

function takeSnapshot() {
  savedSnapshot.value = JSON.stringify(data.value);
}

const previewBlocked = computed(
  () =>
    pendingInput.value || Boolean(stellarParseError.value) || stellarLoading.value || Boolean(stellarLoadError.value),
);
const {
  previewOpen,
  previewViewport,
  previewPage,
  previewHtml,
  previewLoading,
  previewError,
  previewFocus,
  refreshPreview,
} = useSettingsPreview(data, active, stellarDraft, previewBlocked);
const previewBody = ref<HTMLElement | null>(null);
const previewSize = ref({ width: 800, height: 400 });
let previewObserver: ResizeObserver | undefined;
watch(previewBody, (element) => {
  previewObserver?.disconnect();
  if (!element) return;
  previewObserver = new ResizeObserver(([entry]) => {
    previewSize.value = { width: entry.contentRect.width, height: entry.contentRect.height };
  });
  previewObserver.observe(element);
});
const previewFrameStyle = computed(() => {
  const width = previewViewport.value === "mobile" ? 390 : previewViewport.value === "tablet" ? 768 : 1200;
  const scale = Math.min(1, previewSize.value.width / width);
  return {
    width: `${width}px`,
    height: `${previewSize.value.height / Math.max(scale, 0.1)}px`,
    transform: `scale(${scale})`,
  };
});

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data: response } = await settingsApi.all();
    data.value = ensureSettingsDefaults({ ...response });
    data.value.wiki = sortWikiProjects(data.value.wiki);
    const initialBaselines: Record<string, string> = {};
    for (const section of ALL_SETTINGS_SECTIONS) {
      const jsonStr = JSON.stringify(response[section] ?? null, null, 2);
      rawText.value[section] = jsonStr;
      initialBaselines[section] = jsonStr;
    }
    savedJsonBaseline.value = initialBaselines;
    takeSnapshot();
  } catch {
    loadError.value = "加载设置失败";
  } finally {
    loading.value = false;
  }
}

function updateSectionSnapshot(section: string, savedData: any) {
  try {
    const snap = savedSnapshot.value ? JSON.parse(savedSnapshot.value) : {};
    snap[section] = JSON.parse(JSON.stringify(savedData));
    savedSnapshot.value = JSON.stringify(snap);
  } catch {}
}

async function save(section: string, payload?: unknown): Promise<boolean> {
  if (section !== "site" && section !== "feedback") {
    Toast.error("旧主题分段停止保存，请使用对应设置分类");
    return false;
  }
  if (savingKey.value) return false;
  hostSavingKey.value = section;
  try {
    const rawToSave = payload !== undefined ? payload : data.value[section];
    const body = deepCloneJsonSafe(rawToSave ?? {});
    const jsonAtStart = rawText.value[section];
    const hadNoIndependentDraft = jsonAtStart === savedJsonBaseline.value[section];

    await settingsApi.updateSection(section, body);

    const formatted = JSON.stringify(body, null, 2);
    // 普通保存前记录该分区 JSON 文本。成功后，只有文本仍等于请求开始时文本，且开始时没有独立 JSON 草稿，才自动同步为提交内容；否则保留用户草稿。
    if (rawText.value[section] === jsonAtStart && hadNoIndependentDraft) {
      rawText.value[section] = formatted;
      savedJsonBaseline.value[section] = formatted;
    } else {
      savedJsonBaseline.value[section] = formatted;
    }

    updateSectionSnapshot(section, body);
    Toast.success("设置已保存，前台已生效");
    return true;
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "保存失败，请检查参数或网络后重试"));
    return false;
  } finally {
    hostSavingKey.value = "";
  }
}

async function saveStellar() {
  if (savingKey.value) return;
  if (await stellar.submit("save")) Toast.success("主题设置已保存，前台已生效");
}

// ---- 放弃本页修改：主题页只还原本页字段（其他页草稿保留）；站点信息与访客反馈回到最近一次加载/保存值 ----
function discardPage(page: SettingsPageKey) {
  if (savingKey.value) return;
  const host = settingsPages[page].host;
  if (host) {
    try {
      const original = JSON.parse(savedSnapshot.value);
      data.value[host] = deepCloneJsonSafe(original[host] ?? {});
      rawText.value[host] = savedJsonBaseline.value[host];
    } catch {
      Toast.error("无法还原本页修改，请刷新页面后重试");
      return;
    }
  } else {
    if (stellarParseError.value) {
      Toast.error("完整配置有语法错误，请在“完整配置原文”中修正，或点击“重新读取”放弃草稿。");
      return;
    }
    try {
      stellarDraft.value = restorePage(stellarDraft.value, stellarSaved.value, page);
    } catch (error) {
      Toast.error(error instanceof Error ? error.message : "无法还原本页修改，请在完整配置原文中检查。");
      return;
    }
    pageRevision[page] = (pageRevision[page] ?? 0) + 1;
    pendingTabs.value[page] = false;
  }
  Toast.success("已放弃本页的未保存修改");
  void focusPageHead();
}

function resetPage(page: SettingsPageKey) {
  const reset = pageReset(page);
  if (savingKey.value || !reset || !stellar.updateField(reset.path, undefined)) return;
  pageRevision[page] = (pageRevision[page] ?? 0) + 1;
  void focusPageHead();
}

// ---- 页内导航 ----
const reduceMotion = () =>
  typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-motion: reduce)").matches === true;
function goSection(page: SettingsPageKey, sectionId: string) {
  if (page === "advanced" && sectionId === "raw") rawOpen.value = true;
  void nextTick(() =>
    document
      .getElementById(settingsSectionAnchor(page, sectionId))
      ?.scrollIntoView({ behavior: reduceMotion() ? "auto" : "smooth", block: "start" }),
  );
}
async function focusPageHead() {
  await nextTick();
  document.getElementById(`settings-page-title-${active.value}`)?.focus({ preventScroll: true });
}
function backToList() {
  void router.push({ path: route.path, query: { ...route.query, tab: undefined } });
}
// sticky 的页头与底部保存栏会盖住滚到它们下面的内容：分区跳转与键盘聚焦都要让出这两块。
// 手机上页头会随页面换行，保存栏文字多时也会变高，所以按实际尺寸量取，经 CSS 变量交给样式：
// --settings-sticky-offset 是页头高度，--settings-bar-offset 是保存栏顶边到内容区底边的距离。
const viewRoot = ref<HTMLElement | null>(null);
const stickyOffset = ref(72);
const barOffset = ref(72);
let stickyObserver: ResizeObserver | undefined;
function scrollParent(element: HTMLElement | null): HTMLElement | null {
  for (let parent = element?.parentElement ?? null; parent; parent = parent.parentElement) {
    const overflowY = getComputedStyle(parent).overflowY;
    if (overflowY === "auto" || overflowY === "scroll") return parent;
  }
  return null;
}
function measureSticky() {
  const root = viewRoot.value;
  if (!root) return;
  const header = root.querySelector<HTMLElement>(":scope > .page-header");
  if (header) stickyOffset.value = Math.ceil(header.getBoundingClientRect().height);
  // 各主题页的表单都已挂载，只有当前页的保存栏有高度（其余在 hidden 的页面里）。
  const bar = Array.from(root.querySelectorAll<HTMLElement>(".settings-save-bar")).find((el) => el.offsetHeight > 0);
  if (!bar) return;
  // 粘住时保存栏停在“内容区底边 − 内容区底部留白 − bottom”处：手机上留白让出底部导航，
  // 预览面板停靠时 bottom 让它停在面板上方。
  const reserved = parseFloat(getComputedStyle(scrollParent(root) ?? root).paddingBottom) || 0;
  const lift = parseFloat(getComputedStyle(bar).bottom) || 0;
  barOffset.value = Math.ceil(bar.getBoundingClientRect().height + lift + reserved);
}
function observeSticky() {
  stickyObserver?.disconnect();
  const root = viewRoot.value;
  if (!root || typeof ResizeObserver === "undefined") return;
  stickyObserver = new ResizeObserver(measureSticky);
  root
    .querySelectorAll<HTMLElement>(":scope > .page-header, .settings-save-bar")
    .forEach((element) => stickyObserver?.observe(element));
  measureSticky();
}
// 保存栏随页面挂载、随表单就绪出现，放弃本页修改与重新读取会重建表单（保存栏换成新元素）；
// 预览面板停靠会把它抬高却不改变它的尺寸，所以也要重新量取。
watch([active, formsReady, loading, previewOpen, loadRevision, () => pageRevision[active.value]], observeSticky, {
  flush: "post",
});
// 侧栏有自己的纵向滚动区：短屏下把当前页显露在可视范围内，只滚动侧栏，不带动整页。
const sidebar = ref<HTMLElement | null>(null);
function revealActiveNav() {
  const aside = sidebar.value;
  const item = aside?.querySelector<HTMLElement>('[aria-current="page"]');
  if (!aside || !item || aside.scrollHeight <= aside.clientHeight) return;
  const view = aside.getBoundingClientRect();
  const box = item.getBoundingClientRect();
  if (box.top < view.top) aside.scrollTop -= view.top - box.top;
  else if (box.bottom > view.bottom) aside.scrollTop += box.bottom - view.bottom;
}
// 回到内容起点。内容区在 Console 外壳里自己滚动，而且外壳切换路由时不重置滚动位置：沿用上一页（或从别的
// Console 页面带进来）的位置，或用 scrollIntoView(nearest) 对齐标题，都会让标题停在 sticky 页头后面。
function scrollToStart() {
  viewRoot.value?.scrollIntoView({ block: "start" });
}
// 换页（含手机上列表与详情切换）后回到内容起点。
watch([active, showMobileList], async () => {
  await nextTick();
  revealActiveNav();
  scrollToStart();
});
watch(showMobileList, (list, wasList) => {
  // 手机端从列表进入详情后，焦点移到页面标题，避免停留在已隐藏的列表上。
  if (wasList && !list) void focusPageHead();
});
watch(() => route.query.tab, () => void normalizeRoute());

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (isDirty.value) {
    e.preventDefault();
    e.returnValue = "";
  }
}

onMounted(() => {
  observeSticky();
  scrollToStart();
  void normalizeRoute().then(() => nextTick(revealActiveNav));
  void load();
  void stellar.load();
  mobileQuery?.addEventListener("change", syncMobile);
  window.addEventListener("beforeunload", handleBeforeUnload);
});

onBeforeUnmount(() => {
  previewObserver?.disconnect();
  stickyObserver?.disconnect();
  mobileQuery?.removeEventListener("change", syncMobile);
  window.removeEventListener("beforeunload", handleBeforeUnload);
});

onBeforeRouteLeave((to, from, next) => {
  if (to.path === from.path) {
    next();
    return;
  }
  if (isDirty.value) {
    const confirmed = window.confirm("您有未保存的设置更改，离开后修改将丢失，是否确认离开？");
    if (confirmed) {
      next();
    } else {
      next(false);
    }
    return;
  }
  next();
});
</script>

<template>
  <div
    ref="viewRoot"
    class="settings-view"
    :class="{ 'preview-docked': previewOpen }"
    :style="{ '--settings-sticky-offset': `${stickyOffset}px`, '--settings-bar-offset': `${barOffset}px` }"
  >
    <VPageHeader title="系统设置">
      <template #badge>
        <span v-if="isDirty" class="settings-dirty-badge"><span class="settings-dot" aria-hidden="true"></span>未保存修改</span>
      </template>
      <template #actions>
        <VButton
          v-if="previewFocus"
          size="sm"
          :type="previewOpen ? 'primary' : 'secondary'"
          @click="previewOpen = !previewOpen"
        >
          <template #icon><Icon icon="ri:eye-line" /></template><span>前端预览</span>
        </VButton>
      </template>
    </VPageHeader>

    <div class="settings-container">
      <div class="settings-layout">
        <aside v-show="!isMobile || showMobileList" ref="sidebar" class="settings-sidebar">
          <SettingsNav :groups="navGroups" :active="showMobileList ? '' : active" :variant="isMobile ? 'list' : 'nav'" />
        </aside>

        <main v-show="!showMobileList" class="settings-content">
          <button v-if="isMobile" type="button" class="settings-back" @click="backToList">
            <Icon icon="ri:arrow-left-s-line" aria-hidden="true" />返回设置
          </button>

          <div v-if="loading" class="settings-state">
            <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" /><span>加载中...</span>
          </div>
          <div v-else-if="loadError" class="settings-state settings-state-error">
            <Icon icon="ri:error-warning-line" width="22" height="22" /><span>设置加载失败：{{ loadError }}</span
            ><VButton type="secondary" size="sm" @click="load">重试</VButton>
          </div>
          <template v-else>
            <section
              v-for="key in settingsPageKeys"
              :id="`settings-page-${key}`"
              :key="key"
              class="settings-tab-panel"
              :aria-labelledby="`settings-page-title-${key}`"
              :hidden="active !== key"
            >
              <template v-if="active === key">
                <header class="settings-page-head">
                  <h2 :id="`settings-page-title-${key}`" tabindex="-1">{{ settingsPages[key].label }}</h2>
                  <p>{{ settingsPages[key].summary }}</p>
                  <nav class="settings-anchors" aria-label="本页分区">
                    <button
                      v-for="section in settingsPages[key].sections"
                      :key="section.id"
                      type="button"
                      class="settings-anchor"
                      @click="goSection(key, section.id)"
                    >
                      {{ section.title }}
                    </button>
                  </nav>
                </header>
                <div v-if="pageReset(key)" class="settings-page-tools" :data-setting-path="pageReset(key)!.path">
                  <VButton
                    size="sm"
                    type="secondary"
                    :disabled="Boolean(savingKey)"
                    :aria-label="`恢复${pageReset(key)!.label}默认`"
                    @click="resetPage(key)"
                    >恢复{{ pageReset(key)!.label }}默认</VButton
                  >
                  <span class="settings-help">移除本页全部显式覆盖，改回主题默认；仍需保存后才生效。</span>
                </div>
              </template>

              <div v-if="!settingsPages[key].host" class="settings-tab-content">
                <div v-if="active === key" class="canonical-settings-status">
                  <StellarSettingsStatus :editor="stellar" :saving="savingKey" />
                  <p v-if="stellarParseError" class="settings-error" role="alert">
                    完整配置暂时无法用于表单编辑：{{ stellarParseError }}。请展开「代码与原文」中的完整配置原文修正后继续。
                  </p>
                </div>
                <CanonicalSettingsForm
                  v-if="formsReady"
                  :key="`${key}-${loadRevision}-${pageRevision[key] ?? 0}`"
                  :page="key"
                  :document="stellarDraft"
                  :saved="stellarSaved"
                  :saving="savingKey || (pendingInput ? 'pending-input' : '')"
                  @change="stellar.updateField"
                  @pending="pendingTabs[key] = $event"
                  @save="saveStellar"
                >
                  <template v-if="key === 'advanced'" #after>
                    <SettingsRawEditor
                      :id="settingsSectionAnchor('advanced', 'raw')"
                      v-model:open="rawOpen"
                      :editor="stellar"
                      :saving="savingKey"
                    />
                  </template>
                  <template #actions="{ blocked }">
                    <SettingsSaveBar
                      scope="theme"
                      :dirty="stellarDirty"
                      :page-dirty="pageDirty(key)"
                      :total="themeSummary.total"
                      :breakdown="themeBreakdown"
                      :saving="savingKey === 'stellar2'"
                      :busy="Boolean(savingKey)"
                      :blocked="blocked || (pendingInput ? '部分设置还有未完成的输入，请修正后再保存。' : '')"
                      @discard="discardPage(key)"
                    />
                  </template>
                </CanonicalSettingsForm>
                <SettingsRawEditor
                  v-else-if="key === 'advanced'"
                  :id="settingsSectionAnchor('advanced', 'raw')"
                  v-model:open="rawOpen"
                  :editor="stellar"
                  :saving="savingKey"
                />
              </div>
              <template v-else-if="active === key">
                <BasicSettings
                  v-if="key === 'site'"
                  :data="data"
                  :saved="savedSections.site"
                  :saving="savingKey"
                  @save="save"
                >
                  <template #actions>
                    <SettingsSaveBar
                      scope="host"
                      :dirty="dirtySections.has('site')"
                      :page-dirty="dirtySections.has('site')"
                      :total="hostChangeCount('site')"
                      :other-theme-count="otherThemeCount"
                      :saving="savingKey === 'site'"
                      :busy="Boolean(savingKey)"
                      @discard="discardPage('site')"
                    />
                  </template>
                </BasicSettings>
                <div v-else-if="key === 'feedback'" class="settings-tab-content">
                  <FeedbackSettings :data="data" :saved="savedSections.feedback" :saving="savingKey" @save="save">
                    <template #actions>
                      <SettingsSaveBar
                        scope="host"
                        :dirty="dirtySections.has('feedback')"
                        :page-dirty="dirtySections.has('feedback')"
                        :total="hostChangeCount('feedback')"
                        :other-theme-count="otherThemeCount"
                        :saving="savingKey === 'feedback'"
                        :busy="Boolean(savingKey)"
                        @discard="discardPage('feedback')"
                      />
                    </template>
                  </FeedbackSettings>
                </div>
              </template>
            </section>
          </template>
        </main>
      </div>

      <aside v-if="previewOpen" class="preview-panel">
        <div class="preview-panel-header">
          <span class="preview-focus-label"
            ><Icon icon="ri:eye-line" />{{ previewFocus ? `预览：${previewFocus.label}` : "当前设置无固定预览" }}</span
          >
          <div class="preview-panel-actions">
            <button
              type="button"
              :aria-pressed="previewViewport === 'desktop'"
              :class="{ active: previewViewport === 'desktop' }"
              @click="previewViewport = 'desktop'"
            >
              桌面
            </button>
            <button
              type="button"
              :aria-pressed="previewViewport === 'tablet'"
              :class="{ active: previewViewport === 'tablet' }"
              @click="previewViewport = 'tablet'"
            >
              平板
            </button>
            <button
              type="button"
              :aria-pressed="previewViewport === 'mobile'"
              :class="{ active: previewViewport === 'mobile' }"
              @click="previewViewport = 'mobile'"
            >
              手机
            </button>
            <button type="button" aria-label="刷新预览" title="刷新预览" @click="refreshPreview">
              <Icon icon="ri:refresh-line" />
            </button>
            <button type="button" aria-label="关闭预览" title="关闭预览" @click="previewOpen = false">
              <Icon icon="ri:close-line" />
            </button>
          </div>
        </div>
        <div class="preview-context">
          <select v-model="previewPage" class="apple-select" aria-label="预览页面">
            <option v-for="page in settingsPreviewPages" :key="page.value" :value="page.value">{{ page.label }}</option>
          </select>
          <p>当前草稿 · 尚未保存到前台。静态预览不执行脚本，交互功能请保存后到前台检查；没有对应内容时显示列表页。</p>
        </div>
        <div ref="previewBody" class="preview-panel-body" :aria-busy="previewLoading">
          <div v-if="!previewFocus" class="preview-loading">
            <Icon icon="ri:information-line" width="20" height="20" /><span
              >当前设置项没有固定视觉预览，保存后请在前台查看效果。</span
            >
          </div>
          <div v-else-if="previewLoading" class="preview-loading" role="status">
            <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" /><span>生成预览中...</span>
          </div>
          <div v-else-if="previewError" class="preview-loading preview-error" role="alert">
            <Icon icon="ri:error-warning-line" width="22" height="22" />
            <span>{{ previewError }}</span>
            <VButton size="sm" type="secondary" @click="refreshPreview">重试预览</VButton>
          </div>
          <iframe
            v-else-if="previewHtml"
            :srcdoc="previewHtml"
            title="前台设置预览"
            class="preview-frame"
            sandbox=""
            :style="previewFrameStyle"
          />
          <div v-else class="preview-loading">暂无预览</div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  min-height: 100%;
  background: var(--color-background);
  /* 预览面板停靠在底部时，保存栏上移到它上方。 */
  --settings-dock-offset: 0px;
}
.settings-view.preview-docked {
  --settings-dock-offset: max(56vh, 320px);
}
.settings-container {
  display: flex;
  flex-direction: column;
  gap: 0;
  width: 100%;
  max-width: 1200px;
  margin: 0;
  padding: 0 32px 40px;
  box-sizing: border-box;
}
.settings-layout {
  display: flex;
  align-items: flex-start;
  gap: 32px;
  min-width: 0;
}
.settings-sidebar {
  position: sticky;
  top: 88px;
  flex: 0 0 216px;
  max-height: calc(100vh - 104px);
  /* 目录本身仍是 208px 宽；右侧另留 8px，目录高过视口出现滚动条时，选中项的底色不贴着滚动条。 */
  padding: 24px 8px 16px 0;
  overflow-y: auto;
}
.settings-content {
  display: flex;
  flex: 1 1 0;
  flex-direction: column;
  gap: 0;
  min-width: 0;
}
.settings-dirty-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: 8px;
  color: var(--color-state-warning, #ff9f0a);
  font-size: 12px;
  font-weight: 500;
}
.settings-tab-content {
  min-width: 0;
}
.canonical-settings-status {
  padding: 0;
}
.canonical-settings-status:has([role="status"]),
.canonical-settings-status:has([role="alert"]) {
  padding-top: 16px;
  padding-bottom: 4px;
}
.settings-page-head h2:focus {
  outline: none;
}
.settings-page-head h2:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 4px;
}
.settings-page-tools {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  padding: 12px 0 0;
}
.settings-back {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  gap: 2px;
  min-height: 44px;
  margin-left: -6px;
  padding: 0 8px 0 2px;
  border: 0;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 14px;
  cursor: pointer;
}
.settings-back:hover {
  color: var(--color-foreground);
}
.settings-back:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}
.settings-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 60px 0;
  color: var(--color-text-tertiary);
  font-size: 13px;
}
.settings-state-error {
  color: var(--color-state-error);
}
.settings-spin {
  animation: settings-spin 1s linear infinite;
}
@keyframes settings-spin {
  to {
    transform: rotate(360deg);
  }
}
.preview-panel {
  position: sticky;
  bottom: 0;
  z-index: 30;
  display: flex;
  flex-direction: column;
  height: 56vh;
  min-height: 320px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-bottom: none;
  border-radius: var(--radius-2xl) var(--radius-2xl) 0 0;
  background: var(--color-background);
  box-shadow: var(--shadow-lg);
}
.preview-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border);
  background: var(--color-background);
}
.preview-focus-label {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 500;
}
.preview-panel-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.preview-panel-actions button {
  min-height: 44px;
  padding: 0 8px;
  border: 0;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-tertiary);
  cursor: pointer;
}
.preview-panel-actions button.active,
.preview-panel-actions button:hover {
  background: var(--color-accent);
  color: var(--color-foreground);
}
.preview-panel-body {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: var(--color-background);
}
.preview-frame {
  display: block;
  max-width: none;
  transform-origin: top left;
  border: 0;
  background: var(--color-background);
}
.preview-loading {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-text-tertiary);
  font-size: 13px;
  text-align: center;
  padding: 20px;
}
.preview-context {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border);
}
.preview-context select {
  width: 180px;
  min-height: 40px;
  flex-shrink: 0;
}
.preview-context p {
  margin: 0;
  color: var(--color-text-tertiary);
  font-size: 11px;
  line-height: 1.5;
}
.preview-error {
  color: var(--color-state-error);
}
.preview-panel-actions button:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}
@media (max-width: 767px) {
  .settings-view.preview-docked {
    --settings-dock-offset: max(65vh, 320px);
  }
  .settings-container {
    padding: 0 16px 32px;
    gap: 0;
  }
  .settings-layout {
    display: block;
  }
  .settings-sidebar {
    position: static;
    max-height: none;
    padding: 8px 0 16px;
    overflow: visible;
  }
  .preview-panel {
    height: 65vh;
  }
  .preview-panel-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .preview-context {
    align-items: stretch;
    flex-direction: column;
    gap: 6px;
  }
  .preview-context select {
    width: 100%;
  }
}
</style>
