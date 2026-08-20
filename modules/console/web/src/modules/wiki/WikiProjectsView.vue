<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import { Dialog, Toast, VButton, VCard } from "@/components/ui";
import { wikiApi, type WikiHeroAction, type WikiProject } from "@/api";

interface ProjectForm {
 id: string;
 name: string;
 title: string;
 headline: string;
 subtitle: string;
 description: string;
 icon: string;
 cover: string;
 available: string;
 url: string;
 repo: string;
 branch: string;
 sort: number;
 pin: boolean;
 visible: boolean;
 tagsText: string;
 hero: {
  background: string;
  title: string;
  description: string;
  start: string;
  previewType: "" | "terminal" | "image";
  previewImage: string;
  commandsText: string;
  actions: WikiHeroAction[];
 };
}

const inputCls = "apple-input";
const textareaCls = "apple-textarea";
const selectCls = "apple-select";

function emptyForm(id = ""): ProjectForm {
 return {
  id, name: "", title: "", headline: "", subtitle: "", description: "", icon: "", cover: "",
  available: "", url: "", repo: "", branch: "", sort: 0, pin: false, visible: true, tagsText: "",
  hero: { background: "", title: "", description: "", start: "", previewType: "", previewImage: "", commandsText: "", actions: [] },
 };
}

function formFromProject(p: WikiProject): ProjectForm {
 const hero = p.hero || {};
 const preview = hero.preview || {};
 const commands = (preview.commands || []).flatMap((c) => c.codes || []);
 return {
  id: p.id, name: p.name || p.id, title: p.title || "", headline: p.headline || "", subtitle: p.subtitle || "",
  description: p.description || "", icon: p.icon || "", cover: p.cover || "", available: p.available || "",
  url: p.url || "", repo: p.repo || "", branch: p.branch || "", sort: Number(p.sort) || 0,
  pin: Boolean(p.pin), visible: p.visible !== false, tagsText: (p.tags || []).join(", "),
  hero: {
   background: hero.background || "", title: hero.title || "", description: hero.description || "", start: hero.start || "",
   previewType: preview.type === "terminal" || preview.type === "image" ? preview.type : "",
   previewImage: preview.image || "", commandsText: commands.join("\n"),
   actions: (hero.actions || []).map((a) => ({ title: a.title || "", url: a.url || "", icon: a.icon || "" })),
  },
 };
}

const projects = ref<WikiProject[]>([]);
const selectedId = ref("");
const form = ref<ProjectForm>(emptyForm());
const loading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const error = ref("");

const selectedProject = computed(() => projects.value.find((p) => p.id === selectedId.value) || null);
const isEditing = computed(() => Boolean(selectedId.value));
const projectCountLabel = computed(() => `${projects.value.length} 个项目`);

async function loadProjects(selectID = "") {
 loading.value = true;
 error.value = "";
 try {
  const { data } = await wikiApi.listProjects();
  projects.value = Array.isArray(data.items) ? data.items : [];
  const target = selectID || selectedId.value;
  const project = projects.value.find((p) => p.id === target) || projects.value[0];
  if (project) selectProject(project);
  else {
   selectedId.value = "";
   form.value = emptyForm();
  }
 } catch (e: any) {
  error.value = e?.response?.data?.error || e?.message || "加载 Wiki 项目失败";
 } finally {
  loading.value = false;
 }
}

function selectProject(project: WikiProject) {
 selectedId.value = project.id;
 form.value = formFromProject(project);
 error.value = "";
}

function startCreate() {
 selectedId.value = "";
 form.value = emptyForm();
 error.value = "";
}

function addAction() {
 form.value.hero.actions.push({ title: "", url: "", icon: "" });
}

function removeAction(index: number) {
 form.value.hero.actions.splice(index, 1);
}

function buildHero() {
 const h = form.value.hero;
 const commands = h.commandsText.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
 const actions = h.actions
  .map((a) => ({ title: (a.title || "").trim(), url: (a.url || "").trim(), icon: (a.icon || "").trim() }))
  .filter((a) => a.title || a.url);
 const preview = h.previewType
  ? {
    type: h.previewType,
    ...(h.previewImage.trim() ? { image: h.previewImage.trim() } : {}),
    ...(commands.length ? { commands: commands.map((code) => ({ codes: [code] })) } : {}),
   }
  : undefined;
 const hero = {
  background: h.background.trim(), title: h.title.trim(), description: h.description.trim(), start: h.start.trim(),
  ...(preview ? { preview } : {}), ...(actions.length ? { actions } : {}),
 };
 return Object.values(hero).some((value) => value && (typeof value !== "object" || Object.keys(value).length)) ? hero : undefined;
}

function payload() {
 const f = form.value;
 return {
  id: f.id.trim(), name: f.name.trim(), title: f.title.trim(), headline: f.headline.trim(), subtitle: f.subtitle.trim(),
  description: f.description.trim(), icon: f.icon.trim(), cover: f.cover.trim(), available: f.available.trim(),
  url: f.url.trim(), repo: f.repo.trim(), branch: f.branch.trim(), sort: Number(f.sort) || 0, pin: f.pin,
  visible: f.visible, tags: f.tagsText.split(/[,，]/).map((tag) => tag.trim()).filter(Boolean), hero: buildHero(),
 };
}

async function saveProject() {
 const id = form.value.id.trim();
 if (!id) {
  Toast.error("请输入项目 ID");
  return;
 }
 if (!/^[A-Za-z0-9][A-Za-z0-9_-]*$/.test(id)) {
  Toast.error("项目 ID 只能包含字母、数字、下划线和短横线");
  return;
 }
 saving.value = true;
 try {
  if (isEditing.value) await wikiApi.updateProject(selectedId.value, payload());
  else await wikiApi.createProject(payload() as any);
  Toast.success(isEditing.value ? "Wiki 项目已保存" : "Wiki 项目已创建");
  await loadProjects(id);
 } catch (e: any) {
  Toast.error(e?.response?.data?.error || e?.message || "保存失败");
 } finally {
  saving.value = false;
 }
}

function deleteProject() {
 const project = selectedProject.value;
 if (!project || project.has_docs) return;
 Dialog.warning({
  title: "确定删除这个 Wiki 项目吗？",
  description: `「${project.title || project.name || project.id}」删除后不可恢复。`,
  confirmType: "danger",
  onConfirm: async () => {
   deleting.value = true;
   try {
    await wikiApi.deleteProject(project.id);
    Toast.success("Wiki 项目已删除");
    selectedId.value = "";
    await loadProjects();
   } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "删除失败");
   } finally {
    deleting.value = false;
   }
  },
 });
}

onMounted(() => loadProjects());
</script>

<template>
 <div class="wiki-projects-view">
  <header class="wiki-projects-header">
   <div class="min-w-0">
    <h1 class="wiki-projects-title"><Icon icon="ri:stack-line" /> Wiki 项目</h1>
    <p class="wiki-projects-subtitle">管理项目卡片、标签和文档页 Hero · {{ projectCountLabel }}</p>
   </div>
   <div class="flex shrink-0 gap-2">
    <VButton type="secondary" size="sm" :route="{ path: '/wiki' }">
     <template #icon><Icon icon="ri:file-list-3-line" /></template>文档
    </VButton>
    <VButton type="primary" size="sm" @click="startCreate">
     <template #icon><Icon icon="ri:add-line" /></template>新建项目
    </VButton>
   </div>
  </header>

  <div v-if="error" class="wiki-projects-error"><Icon icon="ri:error-warning-line" />{{ error }}<VButton type="secondary" size="sm" @click="loadProjects">重试</VButton></div>
  <div v-if="loading" class="wiki-projects-state"><Icon icon="ri:loader-4-line" class="spin" />加载项目中...</div>

  <div v-else class="wiki-projects-layout">
   <aside class="wiki-project-list">
    <div class="wiki-project-list-head"><span>项目列表</span><span class="text-xs text-text-tertiary">{{ projects.length }}</span></div>
    <button v-for="project in projects" :key="project.id" type="button" class="wiki-project-list-item" :class="{ active: selectedId === project.id }" @click="selectProject(project)">
     <span class="wiki-project-list-icon"><img v-if="project.icon" :src="project.icon" alt="" /><Icon v-else icon="ri:book-2-line" /></span>
     <span class="min-w-0 flex-1 text-left"><strong>{{ project.title || project.name || project.id }}</strong><small>{{ project.id }} · {{ project.doc_count || 0 }} 篇文档</small></span>
     <Icon v-if="project.pin" icon="ri:pushpin-2-fill" class="text-primary" />
    </button>
    <div v-if="!projects.length" class="wiki-project-empty">还没有 Wiki 项目<br /><button type="button" @click="startCreate">创建第一个项目</button></div>
   </aside>

   <main class="wiki-project-editor">
    <VCard :title="isEditing ? '编辑项目' : '新建项目'">
     <div class="form-grid form-grid-3">
      <div class="field"><label>项目 ID <span class="required">*</span></label><input v-model="form.id" :class="inputCls" :disabled="isEditing" placeholder="stellar" /><small class="field-hint">创建后不可修改，用于 /wiki/:project</small></div>
      <div class="field"><label>内部名称</label><input v-model="form.name" :class="inputCls" placeholder="Stellar" /></div>
      <div class="field"><label>排序（大在前）</label><input v-model.number="form.sort" type="number" :class="inputCls" /></div>
     </div>
     <div class="form-grid form-grid-2">
      <div class="field"><label>卡片标题</label><input v-model="form.title" :class="inputCls" /></div>
      <div class="field"><label>卡片副标题</label><input v-model="form.subtitle" :class="inputCls" /></div>
      <div class="field"><label>Headline</label><input v-model="form.headline" :class="inputCls" placeholder="可选的短标题" /></div>
      <div class="field"><label>适用范围</label><input v-model="form.available" :class="inputCls" placeholder="适用于独立博主" /></div>
     </div>
     <div class="field"><label>项目简介</label><textarea v-model="form.description" rows="2" :class="textareaCls" /></div>
     <div class="form-grid form-grid-2">
      <div class="field"><label>项目图标</label><AttachmentField v-model="form.icon" placeholder="图片 URL" /></div>
      <div class="field"><label>卡片封面</label><AttachmentField v-model="form.cover" placeholder="图片 URL（留空为无封面）" /></div>
     </div>
     <div class="form-grid form-grid-3">
      <div class="field"><label>GitHub 仓库</label><input v-model="form.repo" :class="inputCls" placeholder="owner/repo" /></div>
      <div class="field"><label>分支</label><input v-model="form.branch" :class="inputCls" placeholder="main" /></div>
      <div class="field"><label>外部链接</label><input v-model="form.url" :class="inputCls" placeholder="https://..." /></div>
     </div>
     <div class="field"><label>标签（逗号分隔）</label><input v-model="form.tagsText" :class="inputCls" placeholder="开源, 工具" /><small class="field-hint">前台会以 # 标签形式显示，并支持按标签筛选。</small></div>
     <div class="toggle-row">
      <label><input v-model="form.visible" type="checkbox" /> <span>在前台显示</span></label>
      <label><input v-model="form.pin" type="checkbox" /> <span>置顶项目</span></label>
      <span v-if="selectedProject?.has_docs" class="doc-count"><Icon icon="ri:file-text-line" /> 已有 {{ selectedProject?.doc_count || 0 }} 篇文档</span>
     </div>
    </VCard>

    <VCard title="项目 Hero（可选）" class="mt-4">
     <p class="section-hint">详情页有 Hero 配置时显示在文档标题上方；背景填 galaxy 可启用动态星空，浏览器不支持动效时会自然降级。</p>
     <div class="form-grid form-grid-2">
      <div class="field"><label>背景（图片 URL 或 galaxy）</label><input v-model="form.hero.background" :class="inputCls" placeholder="galaxy / https://..." /></div>
      <div class="field"><label>Hero 标题</label><input v-model="form.hero.title" :class="inputCls" placeholder="留空使用项目名" /></div>
      <div class="field"><label>Hero 描述</label><input v-model="form.hero.description" :class="inputCls" /></div>
      <div class="field"><label>开始按钮文案</label><input v-model="form.hero.start" :class="inputCls" placeholder="开始阅读" /></div>
     </div>
     <div class="hero-preview-form">
      <div class="field"><label>预览类型</label><select v-model="form.hero.previewType" :class="selectCls"><option value="">无预览</option><option value="terminal">终端命令</option><option value="image">图片预览</option></select></div>
      <div v-if="form.hero.previewType === 'image'" class="field"><label>预览图片</label><AttachmentField v-model="form.hero.previewImage" placeholder="图片 URL" /></div>
      <div v-if="form.hero.previewType === 'terminal'" class="field hero-command-field"><label>命令（每行一条，可复制）</label><textarea v-model="form.hero.commandsText" rows="4" :class="textareaCls" placeholder="npm install stellar\nmake dev" /></div>
     </div>
     <div class="actions-editor">
      <div class="actions-head"><label>操作按钮</label><button type="button" class="text-link" @click="addAction"><Icon icon="ri:add-line" /> 添加按钮</button></div>
      <div v-for="(action, index) in form.hero.actions" :key="index" class="action-row">
       <input v-model="action.title" :class="inputCls" placeholder="按钮文案" />
       <input v-model="action.url" :class="inputCls" placeholder="链接 URL" />
       <input v-model="action.icon" :class="inputCls" placeholder="图标 key（可选）" />
       <button type="button" class="remove-action" title="删除按钮" @click="removeAction(index)"><Icon icon="ri:delete-bin-line" /></button>
      </div>
      <p v-if="!form.hero.actions.length" class="field-hint">暂无按钮。</p>
     </div>
     <div class="card-actions">
      <VButton type="primary" size="sm" :loading="saving" @click="saveProject">{{ isEditing ? '保存项目' : '创建项目' }}</VButton>
      <VButton v-if="isEditing" type="danger" size="sm" :disabled="Boolean(selectedProject?.has_docs)" :loading="deleting" :title="selectedProject?.has_docs ? '有文档的项目不能删除' : '删除项目'" @click="deleteProject">删除项目</VButton>
      <span v-if="selectedProject?.has_docs" class="delete-hint">项目包含文档时不能删除，请先移除文档。</span>
     </div>
    </VCard>
   </main>
  </div>
 </div>
</template>

<style scoped>
.wiki-projects-view { max-width: 1440px; margin: 0 auto; padding: 24px; }
.wiki-projects-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 20px; }
.wiki-projects-title { display: flex; align-items: center; gap: 10px; margin: 0; color: var(--color-foreground); font-size: 24px; font-weight: 700; }
.wiki-projects-title svg { color: var(--color-primary); }
.wiki-projects-subtitle, .section-hint, .field-hint { color: var(--color-text-tertiary); font-size: 12px; }
.wiki-projects-subtitle { margin: 7px 0 0; }
.wiki-projects-layout { display: grid; grid-template-columns: 280px minmax(0, 1fr); gap: 16px; align-items: start; }
.wiki-project-list { overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-lg); background: var(--color-card); box-shadow: var(--shadow-sm); }
.wiki-project-list-head { display: flex; justify-content: space-between; padding: 14px 16px; border-bottom: 1px solid var(--color-border); color: var(--color-foreground); font-size: 13px; font-weight: 600; }
.wiki-project-list-item { display: flex; width: 100%; align-items: center; gap: 9px; padding: 12px 14px; border: 0; border-bottom: 1px solid color-mix(in srgb, var(--color-border) 55%, transparent); background: transparent; color: var(--color-foreground); text-align: left; cursor: pointer; transition: background 150ms ease, border-color 150ms ease; }
.wiki-project-list-item:last-of-type { border-bottom: 0; }
.wiki-project-list-item:hover, .wiki-project-list-item.active { background: color-mix(in srgb, var(--color-primary) 10%, var(--color-card)); }
.wiki-project-list-item.active { box-shadow: inset 3px 0 var(--color-primary); }
.wiki-project-list-item strong, .wiki-project-list-item small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.wiki-project-list-item strong { font-size: 13px; font-weight: 600; }
.wiki-project-list-item small { margin-top: 3px; color: var(--color-text-tertiary); font-size: 11px; }
.wiki-project-list-icon { display: inline-flex; width: 32px; height: 32px; flex: 0 0 32px; align-items: center; justify-content: center; overflow: hidden; border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-secondary); color: var(--color-text-tertiary); }
.wiki-project-list-icon img { width: 100%; height: 100%; object-fit: cover; }
.wiki-project-empty { padding: 28px 16px; color: var(--color-text-tertiary); font-size: 12px; line-height: 1.8; text-align: center; }
.wiki-project-empty button, .text-link { border: 0; background: transparent; color: var(--color-primary); cursor: pointer; }
.wiki-project-editor { min-width: 0; }
.wiki-project-editor :deep(.card-body) { padding: 20px; }
.required { color: var(--color-state-error); }
.field-hint { display: block; margin-top: 5px; }
.section-hint { margin: -4px 0 16px; line-height: 1.6; }
.toggle-row { display: flex; flex-wrap: wrap; align-items: center; gap: 18px; margin-top: 16px; color: var(--color-text-secondary); font-size: 13px; }
.toggle-row label { display: inline-flex; align-items: center; gap: 7px; cursor: pointer; }
.toggle-row input { accent-color: var(--color-primary); }
.doc-count { display: inline-flex; align-items: center; gap: 5px; color: var(--color-text-tertiary); font-size: 12px; }
.hero-preview-form { display: grid; grid-template-columns: 180px minmax(0, 1fr); gap: 12px; align-items: start; margin-top: 14px; }
.hero-command-field { grid-column: 1 / -1; }
.actions-editor { margin-top: 18px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.actions-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; color: var(--color-text-secondary); font-size: 13px; font-weight: 600; }
.text-link { display: inline-flex; align-items: center; gap: 4px; padding: 3px; font-size: 12px; font-weight: 500; }
.action-row { display: grid; grid-template-columns: 1fr 1.5fr 1fr 32px; gap: 8px; margin-bottom: 8px; }
.remove-action { display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: transparent; color: var(--color-state-error); cursor: pointer; }
.card-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 20px; }
.delete-hint { color: var(--color-text-tertiary); font-size: 12px; }
.wiki-projects-state, .wiki-projects-error { display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 180px; color: var(--color-text-tertiary); }
.wiki-projects-error { justify-content: flex-start; min-height: 0; margin-bottom: 14px; color: var(--color-state-error); }
.wiki-projects-error .btn { margin-left: 4px; }
.spin { animation: wiki-spin 1s linear infinite; }
@keyframes wiki-spin { to { transform: rotate(360deg); } }
@media (max-width: 900px) {
 .wiki-projects-view { padding: 16px; }
 .wiki-projects-layout { grid-template-columns: 1fr; }
 .wiki-project-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
 .wiki-project-list-head { grid-column: 1 / -1; }
 .wiki-project-list-item { border-bottom: 0; }
 .wiki-project-list-item.active { box-shadow: inset 0 -3px var(--color-primary); }
}
@media (max-width: 600px) {
 .wiki-projects-header { align-items: flex-start; flex-direction: column; }
 .wiki-project-list { display: block; }
 .hero-preview-form, .action-row { grid-template-columns: 1fr; }
 .hero-command-field { grid-column: auto; }
}
</style>
