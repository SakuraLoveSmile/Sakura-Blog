<script setup lang="ts">
import { computed, ref, useId } from "vue";
import { patchJSONField, readJSONField } from "./stellar-form-json";

const props = defineProps<{ config: string; version: number; syncError?: string }>();
const emit = defineEmits<{ "update:config": [string] }>();
const id = useId();
const editError = ref("");
const configError = computed(() => {
  try {
    readJSONField(props.config, "");
    return "";
  } catch {
    return "配置原文尚未完成，请在下方“配置原文（JSON）”中修正后再调整设置。";
  }
});
const disabled = computed(() => props.version !== 2 || !!props.syncError || !!configError.value);

type Choice = { value: string | boolean; label: string };
type Field = { path: string; label: string; choices?: Choice[]; placeholder?: string };
const booleanChoices: Choice[] = [
  { value: true, label: "显示" },
  { value: false, label: "隐藏" },
];
const enabledChoices: Choice[] = [
  { value: true, label: "开启" },
  { value: false, label: "关闭" },
];
const groups: { title: string; fields: Field[] }[] = [
  {
    title: "文章显示",
    fields: [
      {
        path: "article.style",
        label: "文章风格",
        choices: [
          { value: "tech", label: "技术文章" },
          { value: "story", label: "叙事文章" },
        ],
      },
      {
        path: "article.paragraph_indent",
        label: "段落缩进",
        choices: [
          { value: "auto", label: "自动" },
          { value: "always", label: "始终缩进" },
          { value: "never", label: "不缩进" },
        ],
      },
      {
        path: "article.ai_label",
        label: "AI 创作声明",
        choices: [
          { value: "manual", label: "人工创作" },
          { value: "reviewed", label: "AI 审阅" },
          { value: "polished", label: "AI 润色" },
          { value: "generated", label: "AI 生成" },
        ],
      },
      { path: "visibility.listed", label: "在文章列表显示", choices: booleanChoices },
      { path: "visibility.searchable", label: "允许站内搜索", choices: enabledChoices },
    ],
  },
  {
    title: "阅读与评论",
    fields: [
      { path: "comments.enabled", label: "文章评论", choices: enabledChoices },
      { path: "comments.title", label: "评论区标题", placeholder: "留空跟随全站设置" },
      { path: "comments.id", label: "评论标识", placeholder: "留空使用默认标识" },
    ],
  },
  {
    title: "内容渲染",
    fields: [
      {
        path: "render.math",
        label: "数学公式",
        choices: [
          { value: false, label: "关闭" },
          { value: "katex", label: "KaTeX" },
          { value: "mathjax", label: "MathJax" },
        ],
      },
      {
        path: "render.diagrams",
        label: "流程图",
        choices: [
          { value: false, label: "关闭" },
          { value: "mermaid", label: "Mermaid" },
        ],
      },
    ],
  },
  {
    title: "页面区域",
    fields: [
      { path: "breadcrumb", label: "面包屑导航", choices: booleanChoices },
      { path: "topbar.enabled", label: "顶部导航", choices: booleanChoices },
      { path: "leftbar.enabled", label: "左侧栏", choices: booleanChoices },
      { path: "rightbar.enabled", label: "右侧栏", choices: booleanChoices },
      { path: "banner.enabled", label: "文章横幅", choices: booleanChoices },
      { path: "banner.headline", label: "横幅标题", placeholder: "留空使用文章标题" },
      { path: "banner.tagline", label: "横幅副标题", placeholder: "填写横幅副标题" },
      { path: "banner.avatar", label: "横幅头像", placeholder: "图片 URL" },
      { path: "banner.background", label: "横幅背景", choices: booleanChoices },
    ],
  },
];
const sharePlatforms = [
  { value: "qrcode", label: "二维码" },
  { value: "weibo", label: "微博" },
  { value: "x", label: "X" },
  { value: "telegram", label: "Telegram" },
  { value: "whatsapp", label: "WhatsApp" },
  { value: "email", label: "邮件" },
];
function read(path: string): unknown {
  return configError.value ? undefined : readJSONField(props.config, path);
}
function setField(path: string, value: unknown) {
  if (disabled.value) return;
  try {
    emit("update:config", patchJSONField(props.config, path, value));
    editError.value = "";
  } catch (problem) {
    editError.value = problem instanceof Error ? problem.message : "无法更新文章设置";
  }
}
function selected(field: Field): string {
  const value = read(field.path);
  if (value === undefined || value === null) return "";
  return field.choices?.some((choice) => choice.value === value) ? String(value) : "custom";
}
function selectField(field: Field, value: string) {
  if (value === "") setField(field.path, undefined);
  else {
    const choice = field.choices?.find((choice) => String(choice.value) === value);
    if (choice) setField(field.path, choice.value);
  }
}
function text(path: string): string {
  const value = read(path);
  return typeof value === "string" ? value : "";
}
function setText(path: string, value: string) {
  setField(path, value.trim() ? value : undefined);
}
const references = computed(() => read("footer.references"));
const complexReferences = computed(
  () => Array.isArray(references.value) && references.value.some((item) => typeof item !== "string"),
);
const referencesText = computed(() =>
  Array.isArray(references.value) && !complexReferences.value ? references.value.join("\n") : "",
);
function setReferences(value: string) {
  setField(
    "footer.references",
    value
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean),
  );
}
const license = computed(() => read("footer.license"));
const licenseMode = computed(() =>
  typeof license.value === "string" ? "custom" : license.value === false ? "off" : license.value === true ? "site" : "",
);
function setLicenseMode(value: string) {
  if (value !== "custom") setField("footer.license", value === "off" ? false : value === "site" ? true : undefined);
}
const share = computed(() => read("footer.share"));
const shareMode = computed(() =>
  Array.isArray(share.value) ? "custom" : share.value === false ? "off" : share.value === true ? "site" : "",
);
function setShareMode(value: string) {
  setField("footer.share", value === "custom" ? [] : value === "off" ? false : value === "site" ? true : undefined);
}
function toggleShare(platform: string, checked: boolean) {
  const current = Array.isArray(share.value) ? share.value : [];
  setField(
    "footer.share",
    checked ? [...current.filter((item) => item !== platform), platform] : current.filter((item) => item !== platform),
  );
}
const tagsField: Field = { path: "footer.show_tags", label: "底部标签", choices: booleanChoices };
</script>

<template>
  <div class="min-w-0 space-y-3">
    <p v-if="configError || editError" role="alert" class="px-1 text-xs text-destructive">
      {{ configError || editError }}
    </p>
    <p v-if="version !== 2" role="status" class="px-1 text-xs text-destructive">旧内容仅可预览，请先完成迁移。</p>
    <details
      v-for="(group, index) in groups"
      :key="group.title"
      :open="index === 0"
      class="min-w-0 rounded-[14px] border border-border bg-card"
    >
      <summary class="cursor-pointer px-4 py-3.5 text-[12.5px] font-medium text-text-secondary">
        {{ group.title }}
      </summary>
      <fieldset :disabled="disabled" class="min-w-0 space-y-3.5 px-4 pb-4 disabled:opacity-50">
        <div v-for="field in group.fields" :key="field.path" class="min-w-0">
          <label :for="id + field.path" class="mb-1.5 block text-[12.5px] font-medium text-text-secondary">{{
            field.label
          }}</label>
          <select
            v-if="field.choices"
            :id="id + field.path"
            :value="selected(field)"
            class="apple-select w-full"
            @change="selectField(field, ($event.target as HTMLSelectElement).value)"
          >
            <option value="">跟随全站设置</option>
            <option v-for="choice in field.choices" :key="String(choice.value)" :value="String(choice.value)">
              {{ choice.label }}
            </option>
            <option v-if="selected(field) === 'custom'" value="custom" disabled>现有自定义配置（原文中编辑）</option>
          </select>
          <input
            v-else
            :id="id + field.path"
            :value="text(field.path)"
            :placeholder="field.placeholder"
            class="apple-input w-full"
            @input="setText(field.path, ($event.target as HTMLInputElement).value)"
          />
        </div>
        <p v-if="index === 0" class="text-[11px] text-text-quaternary">隐藏列表或搜索结果不会限制通过文章链接访问。</p>
        <p v-if="group.title === '内容渲染'" class="text-[11px] text-text-quaternary">
          已有 Mermaid 参数会保留，可在配置原文中调整。
        </p>
      </fieldset>
    </details>

    <details class="min-w-0 rounded-[14px] border border-border bg-card">
      <summary class="cursor-pointer px-4 py-3.5 text-[12.5px] font-medium text-text-secondary">文章底部</summary>
      <fieldset :disabled="disabled" class="min-w-0 space-y-3.5 px-4 pb-4 disabled:opacity-50">
        <div>
          <label :for="id + 'footer.show_tags'" class="mb-1.5 block text-[12.5px] font-medium text-text-secondary"
            >底部标签</label
          >
          <select
            :id="id + 'footer.show_tags'"
            :value="selected(tagsField)"
            class="apple-select w-full"
            @change="selectField(tagsField, ($event.target as HTMLSelectElement).value)"
          >
            <option value="">跟随全站设置</option>
            <option value="true">显示</option>
            <option value="false">隐藏</option>
          </select>
        </div>
        <div>
          <label :for="id + 'license'" class="mb-1.5 block text-[12.5px] font-medium text-text-secondary"
            >版权声明</label
          >
          <select
            :id="id + 'license'"
            :value="licenseMode"
            class="apple-select w-full"
            @change="setLicenseMode(($event.target as HTMLSelectElement).value)"
          >
            <option value="">跟随全站设置</option>
            <option value="site">使用站点协议</option>
            <option value="off">关闭</option>
            <option v-if="licenseMode === 'custom'" value="custom">自定义内容</option>
          </select>
        </div>
        <div>
          <label :for="id + 'license-text'" class="mb-1.5 block text-[12.5px] font-medium text-text-secondary"
            >自定义版权内容</label
          >
          <textarea
            :id="id + 'license-text'"
            :value="text('footer.license')"
            rows="3"
            class="apple-textarea w-full resize-y"
            placeholder="填写后覆盖站点协议，清空恢复继承"
            @input="setText('footer.license', ($event.target as HTMLTextAreaElement).value)"
          ></textarea>
        </div>
        <div>
          <label :for="id + 'share'" class="mb-1.5 block text-[12.5px] font-medium text-text-secondary">文章分享</label>
          <select
            :id="id + 'share'"
            :value="shareMode"
            class="apple-select w-full"
            @change="setShareMode(($event.target as HTMLSelectElement).value)"
          >
            <option value="">跟随全站设置</option>
            <option value="site">使用站点平台</option>
            <option value="off">关闭</option>
            <option value="custom">自定义平台</option>
          </select>
          <div v-if="shareMode === 'custom'" class="mt-2 grid grid-cols-2 gap-x-2">
            <label
              v-for="platform in sharePlatforms"
              :key="platform.value"
              class="flex min-h-11 cursor-pointer items-center gap-2 text-xs text-text-secondary"
            >
              <input
                type="checkbox"
                :checked="Array.isArray(share) && share.includes(platform.value)"
                class="apple-checkbox"
                @change="toggleShare(platform.value, ($event.target as HTMLInputElement).checked)"
              />
              {{ platform.label }}
            </label>
            <p class="col-span-2 text-[11px] text-text-quaternary">未选择平台时不显示分享入口。</p>
          </div>
        </div>
        <div>
          <div class="mb-1.5 flex items-center justify-between gap-2">
            <label :for="id + 'references'" class="text-[12.5px] font-medium text-text-secondary">参考资料</label>
            <button
              type="button"
              class="min-h-11 shrink-0 text-[11px] text-primary hover:underline"
              @click="setField('footer.references', undefined)"
            >
              恢复继承
            </button>
          </div>
          <textarea
            :id="id + 'references'"
            :value="referencesText"
            :disabled="complexReferences"
            rows="4"
            class="apple-textarea w-full resize-y"
            placeholder="每行一条，支持 Markdown 链接"
            @input="setReferences(($event.target as HTMLTextAreaElement).value)"
          ></textarea>
          <p class="mt-1 text-[11px] text-text-quaternary">
            {{
              complexReferences
                ? "现有资料包含复杂对象，已完整保留，请在配置原文中编辑。"
              : "清空可隐藏参考资料；恢复继承移除本文覆盖。"
            }}
          </p>
        </div>
      </fieldset>
    </details>
  </div>
</template>
