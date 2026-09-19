<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import AttachmentField from "@/components/AttachmentField.vue";
import { VPageHeader, VButton, VCard, VEmpty, VLoading, Dialog, Toast } from "@/components/ui";
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
    animationType: "" | "galaxy";
    animationParamsText: string;
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
    id,
    name: "",
    title: "",
    headline: "",
    subtitle: "",
    description: "",
    icon: "",
    cover: "",
    available: "",
    url: "",
    repo: "",
    branch: "",
    sort: 0,
    pin: false,
    visible: true,
    tagsText: "",
    hero: {
      background: "",
      title: "",
      description: "",
      start: "",
      animationType: "",
      animationParamsText: "",
      previewType: "",
      previewImage: "",
      commandsText: "",
      actions: [],
    },
  };
}

function formFromProject(p: WikiProject): ProjectForm {
  const hero = p.hero || {};
  const animation = hero.animation || {};
  const preview = hero.preview || {};
  const commands = (preview.commands || []).flatMap((c) => c.codes || []);
  return {
    id: p.id,
    name: p.name || p.id,
    title: p.title || "",
    headline: p.headline || "",
    subtitle: p.subtitle || "",
    description: p.description || "",
    icon: p.icon || "",
    cover: p.cover || "",
    available: p.available || "",
    url: p.url || "",
    repo: p.repo || "",
    branch: p.branch || "",
    sort: Number(p.sort) || 0,
    pin: Boolean(p.pin),
    visible: p.visible !== false,
    tagsText: (p.tags || []).join(", "),
    hero: {
      background: hero.background || "",
      title: hero.title || "",
      description: hero.description || "",
      start: hero.start || "",
      animationType: animation.type === "galaxy" || hero.background === "galaxy" ? "galaxy" : "",
      animationParamsText: animation.params ? JSON.stringify(animation.params, null, 2) : "",
      previewType: preview.type === "terminal" || preview.type === "image" ? preview.type : "",
      previewImage: preview.image || "",
      commandsText: commands.join("\n"),
      actions: (hero.actions || []).map((a) => ({
        title: a.title || "",
        url: a.url || "",
        icon: a.icon || "",
      })),
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

// ---------- 统计数据 ----------
const stats = computed(() => {
  const all = projects.value;
  let totalDocs = 0;
  let pinCount = 0;
  let visibleCount = 0;
  for (const p of all) {
    totalDocs += p.doc_count || 0;
    if (p.pin) pinCount++;
    if (p.visible !== false) visibleCount++;
  }
  return {
    totalProjects: all.length,
    totalDocs,
    pinCount,
    visibleCount,
  };
});

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
  } catch {
    error.value = "加载 Wiki 项目失败";
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
  const commands = h.commandsText
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  const actions = h.actions
    .map((a) => ({
      title: (a.title || "").trim(),
      url: (a.url || "").trim(),
      icon: (a.icon || "").trim(),
    }))
    .filter((a) => a.title || a.url);
  const preview = h.previewType
    ? {
        type: h.previewType,
        ...(h.previewImage.trim() ? { image: h.previewImage.trim() } : {}),
        ...(commands.length ? { commands: commands.map((code) => ({ codes: [code] })) } : {}),
      }
    : undefined;
  let animation: { type: "galaxy"; params?: Record<string, unknown> } | undefined;
  if (h.animationType === "galaxy") {
    const raw = h.animationParamsText.trim();
    if (raw) {
      let parsed: unknown;
      try {
        parsed = JSON.parse(raw);
      } catch {
        throw new Error("Galaxy 参数必须是合法 JSON");
      }
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error("Galaxy 参数必须是 JSON 对象");
      }
      animation = { type: "galaxy", params: parsed as Record<string, unknown> };
    } else {
      animation = { type: "galaxy" };
    }
  }
  const hero = {
    background: h.background.trim() === "galaxy" ? "" : h.background.trim(),
    title: h.title.trim(),
    description: h.description.trim(),
    start: h.start.trim(),
    ...(animation ? { animation } : {}),
    ...(preview ? { preview } : {}),
    ...(actions.length ? { actions } : {}),
  };
  return Object.values(hero).some((value) => value && (typeof value !== "object" || Object.keys(value).length))
    ? hero
    : undefined;
}

function payload() {
  const f = form.value;
  return {
    id: f.id.trim(),
    name: f.name.trim(),
    title: f.title.trim(),
    headline: f.headline.trim(),
    subtitle: f.subtitle.trim(),
    description: f.description.trim(),
    icon: f.icon.trim(),
    cover: f.cover.trim(),
    available: f.available.trim(),
    url: f.url.trim(),
    repo: f.repo.trim(),
    branch: f.branch.trim(),
    sort: Number(f.sort) || 0,
    pin: f.pin,
    visible: f.visible,
    tags: f.tagsText
      .split(/[,，]/)
      .map((tag) => tag.trim())
      .filter(Boolean),
    hero: buildHero(),
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
  let body;
  try {
    body = payload();
  } catch (err) {
    Toast.error(err instanceof Error ? err.message : "Hero 参数格式错误");
    return;
  }
  saving.value = true;
  try {
    if (isEditing.value) await wikiApi.updateProject(selectedId.value, body);
    else await wikiApi.createProject(body as any);
    Toast.success(isEditing.value ? "Wiki 项目已保存" : "Wiki 项目已创建");
    await loadProjects(id);
  } catch {
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
      } catch {
      } finally {
        deleting.value = false;
      }
    },
  });
}

onMounted(() => loadProjects());
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="文库项目">
      <template #icon>
        <Icon icon="ri:stack-line" class="text-primary" />
      </template>
      <template #subtitle>管理文库项目的多版本元数据、封面海报、标签与 Hero 介绍</template>
      <template #actions>
        <VButton :route="{ path: '/wiki' }">
          <template #icon><Icon icon="ri:file-list-3-line" /></template>
          管理文档
        </VButton>
        <VButton type="primary" @click="startCreate">
          <template #icon><Icon icon="ri:add-line" class="h-full w-full" /></template>
          新建项目
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
        <div class="stats-card">
          <div class="stats-icon text-primary bg-primary/10">
            <Icon icon="ri:stack-line" />
          </div>
          <div>
            <div class="stats-label">文库项目</div>
            <div class="stats-value">{{ stats.totalProjects }} <span class="stats-sub">个项目合辑</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
            <Icon icon="ri:file-text-line" />
          </div>
          <div>
            <div class="stats-label">文档收录</div>
            <div class="stats-value text-[#30D158]">
              {{ stats.totalDocs }} <span class="stats-sub">篇在册文档</span>
            </div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#FF9500] bg-[#FF9500]/10">
            <Icon icon="ri:pushpin-line" />
          </div>
          <div>
            <div class="stats-label">置顶推荐</div>
            <div class="stats-value text-[#FF9500]">{{ stats.pinCount }} <span class="stats-sub">个精选项目</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
            <Icon icon="ri:global-line" />
          </div>
          <div>
            <div class="stats-label">公开展示</div>
            <div class="stats-value">{{ stats.visibleCount }} <span class="stats-sub">个前台可见</span></div>
          </div>
        </div>
      </div>

      <VLoading v-if="loading" />

      <div v-else class="grid grid-cols-1 gap-5 lg:grid-cols-12 lg:items-start">
        <!-- 左侧项目列表 -->
        <div class="space-y-3 lg:col-span-4">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold uppercase tracking-wider text-text-tertiary"
              >项目列表 ({{ projects.length }})</span
            >
            <button type="button" class="text-xs text-primary hover:underline" @click="startCreate">+ 新增</button>
          </div>

          <VEmpty v-if="!projects.length" title="暂无 Wiki 项目" message="点击上方按钮创建你的第一个项目" />

          <div v-else class="space-y-2.5">
            <div
              v-for="project in projects"
              :key="project.id"
              class="group relative flex cursor-pointer items-center gap-3 rounded-2xl border border-border bg-card p-3.5 transition-all hover:border-border-strong"
              :class="{ 'border-primary ring-1 ring-primary bg-primary/5': selectedId === project.id }"
              @click="selectProject(project)"
            >
              <!-- 图标 -->
              <div
                class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-border bg-secondary"
              >
                <img v-if="project.icon" :src="project.icon" alt="" class="h-full w-full object-cover" />
                <Icon v-else icon="ri:book-2-line" class="text-xl text-text-tertiary" />
              </div>

              <!-- 标题与信息 -->
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5">
                  <span class="truncate text-sm font-semibold text-foreground">
                    {{ project.title || project.name || project.id }}
                  </span>
                  <Icon v-if="project.pin" icon="ri:pushpin-fill" class="shrink-0 text-xs text-primary" />
                </div>
                <div class="mt-0.5 flex items-center gap-2 text-xs text-text-quaternary">
                  <span class="font-mono">/{{ project.id }}</span>
                  <span>·</span>
                  <span>{{ project.doc_count || 0 }} 篇文档</span>
                </div>
              </div>

              <Icon
                icon="ri:arrow-right-s-line"
                class="text-text-quaternary transition-transform group-hover:translate-x-0.5"
              />
            </div>
          </div>
        </div>

        <!-- 右侧项目表单与 Hero 设置 -->
        <div class="space-y-5 lg:col-span-8">
          <VCard :title="isEditing ? `编辑项目：${selectedProject?.title || form.id}` : '新建 Wiki 项目'">
            <div class="space-y-4">
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary"
                    >项目 ID / Slug <span class="text-state-error">*</span></label
                  >
                  <input v-model="form.id" :class="inputCls" :disabled="isEditing" placeholder="stellar" />
                  <small class="mt-1 block text-[11px] text-text-quaternary">用于 /wiki/:project，创建后不可更改</small>
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">内部名称</label>
                  <input v-model="form.name" :class="inputCls" placeholder="Stellar Docs" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">排序权重（大在前）</label>
                  <input v-model.number="form.sort" type="number" :class="inputCls" />
                </div>
              </div>

              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">卡片主标题</label>
                  <input v-model="form.title" :class="inputCls" placeholder="如：Stellar 使用指南" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">卡片副标题</label>
                  <input v-model="form.subtitle" :class="inputCls" placeholder="如：全场景多功能知识库" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">Headline 标签</label>
                  <input v-model="form.headline" :class="inputCls" placeholder="可选的短标题/亮点" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">适用范围 / 版本</label>
                  <input v-model="form.available" :class="inputCls" placeholder="如：适用于所有独立博客" />
                </div>
              </div>

              <div>
                <label class="mb-1 block text-xs font-medium text-text-tertiary">项目简介</label>
                <textarea
                  v-model="form.description"
                  rows="2"
                  :class="textareaCls"
                  placeholder="关于该文库项目的简短说明..."
                ></textarea>
              </div>

              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">项目图标</label>
                  <AttachmentField v-model="form.icon" placeholder="选择或输入图标 URL" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">卡片封面</label>
                  <AttachmentField v-model="form.cover" placeholder="选择或输入封面 URL" />
                </div>
              </div>

              <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">GitHub 仓库</label>
                  <input v-model="form.repo" :class="inputCls" placeholder="owner/repo" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">分支</label>
                  <input v-model="form.branch" :class="inputCls" placeholder="main" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">外部链接</label>
                  <input v-model="form.url" :class="inputCls" placeholder="https://..." />
                </div>
              </div>

              <div>
                <label class="mb-1 block text-xs font-medium text-text-tertiary">标签（逗号分隔）</label>
                <input v-model="form.tagsText" :class="inputCls" placeholder="开源, 知识库, 工具" />
              </div>

              <div class="flex flex-wrap items-center gap-5 border-t border-border/60 pt-3">
                <label class="inline-flex items-center gap-2 text-xs font-medium text-foreground cursor-pointer">
                  <input v-model="form.visible" type="checkbox" class="h-4 w-4 rounded border-border text-primary" />
                  前台公开展示
                </label>
                <label class="inline-flex items-center gap-2 text-xs font-medium text-foreground cursor-pointer">
                  <input v-model="form.pin" type="checkbox" class="h-4 w-4 rounded border-border text-primary" />
                  置顶推荐项目
                </label>
              </div>
            </div>
          </VCard>

          <!-- Hero 配置卡 -->
          <VCard title="项目 Hero 横幅（可选）">
            <p class="mb-4 text-xs text-text-tertiary leading-relaxed">
              详情页配置 Hero 时将以全宽横幅呈现；背景图片与 Galaxy 动画可独立或叠加。
            </p>

            <div class="space-y-4">
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">背景图片</label>
                  <AttachmentField v-model="form.hero.background" placeholder="图片 URL（可留空）" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">背景动画</label>
                  <select v-model="form.hero.animationType" :class="selectCls">
                    <option value="">无动画</option>
                    <option value="galaxy">Galaxy 星空</option>
                  </select>
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">Hero 标题</label>
                  <input v-model="form.hero.title" :class="inputCls" placeholder="留空使用项目名" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">Hero 描述</label>
                  <input v-model="form.hero.description" :class="inputCls" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">开始阅读按钮文案</label>
                  <input v-model="form.hero.start" :class="inputCls" placeholder="开始阅读" />
                </div>
              </div>

              <div v-if="form.hero.animationType === 'galaxy'">
                <label class="mb-1 block text-xs font-medium text-text-tertiary">Galaxy 参数（可选 JSON）</label>
                <textarea
                  v-model="form.hero.animationParamsText"
                  rows="5"
                  :class="textareaCls"
                  spellcheck="false"
                  placeholder='{"density": 2, "speed": 0.5, "transparent": true}'
                ></textarea>
                <small class="mt-1 block text-[11px] text-text-quaternary">支持 focal、density、speed、hueShift 等上游参数；非法值会回退默认值。</small>
              </div>

              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">交互预览模式</label>
                  <select v-model="form.hero.previewType" :class="selectCls">
                    <option value="">无预览</option>
                    <option value="terminal">终端命令窗口</option>
                    <option value="image">海报图片</option>
                  </select>
                </div>
                <div v-if="form.hero.previewType === 'image'">
                  <label class="mb-1 block text-xs font-medium text-text-tertiary">预览图片</label>
                  <AttachmentField v-model="form.hero.previewImage" placeholder="图片 URL" />
                </div>
              </div>

              <div v-if="form.hero.previewType === 'terminal'">
                <label class="mb-1 block text-xs font-medium text-text-tertiary">终端命令（每行一条）</label>
                <textarea
                  v-model="form.hero.commandsText"
                  rows="3"
                  :class="textareaCls"
                  placeholder="npm install stellar\nmake dev"
                ></textarea>
              </div>

              <!-- 快捷行动按钮组 -->
              <div class="border-t border-border/60 pt-3">
                <div class="flex items-center justify-between mb-2">
                  <span class="text-xs font-medium text-text-tertiary">快捷行动按钮</span>
                  <button type="button" class="text-xs text-primary hover:underline" @click="addAction">
                    + 添加按钮
                  </button>
                </div>

                <div v-for="(action, index) in form.hero.actions" :key="index" class="flex items-center gap-2 mb-2">
                  <input v-model="action.title" :class="inputCls" placeholder="按钮文案" class="flex-1" />
                  <input v-model="action.url" :class="inputCls" placeholder="跳转 URL" class="flex-1" />
                  <input v-model="action.icon" :class="inputCls" placeholder="图标 key" class="w-28" />
                  <button
                    type="button"
                    class="rounded-lg p-2 text-text-quaternary hover:text-state-error hover:bg-secondary"
                    @click="removeAction(index)"
                  >
                    <Icon icon="ri:delete-bin-line" />
                  </button>
                </div>
              </div>
            </div>

            <template #footer>
              <div class="flex items-center justify-between w-full">
                <div class="flex items-center gap-2">
                  <VButton type="primary" :loading="saving" @click="saveProject">
                    {{ isEditing ? "保存修改" : "创建项目" }}
                  </VButton>
                  <VButton
                    v-if="isEditing"
                    type="danger"
                    :disabled="Boolean(selectedProject?.has_docs)"
                    :loading="deleting"
                    @click="deleteProject"
                  >
                    删除项目
                  </VButton>
                </div>
                <span v-if="selectedProject?.has_docs" class="text-xs text-text-quaternary">
                  包含文档的项目不可直接删除
                </span>
              </div>
            </template>
          </VCard>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 统计卡片 */
.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-xs);
}
.stats-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 8%, transparent), transparent);
  pointer-events: none;
}
.stats-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  font-size: 20px;
  flex-shrink: 0;
}
.stats-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-quaternary);
  text-transform: uppercase;
  letter-spacing: 0.2px;
}
.stats-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.2;
}
.stats-sub {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-tertiary);
  margin-top: 1px;
}

@media (max-width: 640px) {
  .stats-card {
    padding: 10px 12px;
    gap: 8px;
  }
  .stats-icon {
    width: 32px;
    height: 32px;
    font-size: 16px;
  }
  .stats-value {
    font-size: 14px;
  }
}
</style>
