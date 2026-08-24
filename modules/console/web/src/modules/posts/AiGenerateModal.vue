<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Icon } from "@iconify/vue";
import { aiApi } from "@/api";
import { VModal, VButton, Toast } from "@/components/ui";

// AI 属性生成弹窗（对照 Halo AI 插件）：勾选字段 → 单项/一键生成 → 保存回填。
// 生成走后端 /ai/generate 代理（OpenAI 兼容，配置在 设置 → AI）。
interface PostForm {
  slug: string;
  title: string;
  excerpt: string;
  category: string;
  tags: string;
  content_md: string;
}

const props = defineProps<{
  open: boolean;
  form: PostForm;
  categories: string[];
  tags: string[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "apply", values: Record<string, string>): void;
}>();

type FieldKey = "title" | "slug" | "category" | "tags" | "excerpt" | "summary";

const fieldDefs: { key: FieldKey; label: string; kind: "input" | "select" | "textarea" }[] = [
  { key: "title", label: "标题", kind: "input" },
  { key: "slug", label: "别名", kind: "input" },
  { key: "category", label: "分类目录", kind: "select" },
  { key: "tags", label: "标签", kind: "input" },
  { key: "excerpt", label: "摘要", kind: "textarea" },
  { key: "summary", label: "AI 摘要（前台导读）", kind: "textarea" },
];

const checked = ref<Record<FieldKey, boolean>>({
  title: true,
  slug: true,
  category: true,
  tags: true,
  excerpt: true,
  summary: false,
});
const values = ref<Record<FieldKey, string>>({ title: "", slug: "", category: "", tags: "", excerpt: "", summary: "" });
const loadingFields = ref<Set<FieldKey>>(new Set());
const batchLoading = ref(false);

// 打开时以当前表单值初始化
watch(
  () => props.open,
  (open) => {
    if (!open) return;
    values.value = {
      title: props.form.title,
      slug: props.form.slug,
      category: props.form.category,
      tags: props.form.tags,
      excerpt: props.form.excerpt,
      summary: "",
    };
  },
);

const checkedCount = computed(() => fieldDefs.filter((f) => checked.value[f.key]).length);

// 分类下拉选项：现有分类并入当前值（AI 可能生成新分类）
const categoryOptions = computed(() => {
  const set = new Set(props.categories);
  if (values.value.category) set.add(values.value.category);
  return [...set];
});

function setAll(v: boolean) {
  for (const f of fieldDefs) checked.value[f.key] = v;
}

// 把 AI 结果写入本地值（tags 数组转逗号串）
function applyResult(data: Record<string, unknown>, fields: FieldKey[]) {
  for (const f of fields) {
    const v = data[f];
    if (v === undefined || v === null) continue;
    values.value[f] = Array.isArray(v) ? v.join(",") : String(v);
  }
}

async function generate(fields: FieldKey[]) {
  if (fields.length === 0) return;
  try {
    const { data } = await aiApi.generate({
      fields,
      title: props.form.title,
      content: props.form.content_md,
      categories: props.categories,
      tags: props.tags,
    });
    applyResult(data, fields);
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "生成失败");
  }
}

async function generateOne(f: FieldKey) {
  loadingFields.value.add(f);
  loadingFields.value = new Set(loadingFields.value);
  try {
    await generate([f]);
  } finally {
    loadingFields.value.delete(f);
    loadingFields.value = new Set(loadingFields.value);
  }
}

async function generateAll() {
  const fields = fieldDefs.filter((f) => checked.value[f.key]).map((f) => f.key);
  batchLoading.value = true;
  try {
    await generate(fields);
  } finally {
    batchLoading.value = false;
  }
}

/** 保存：把勾选字段的当前值回填到文章表单 */
function save() {
  const out: Record<string, string> = {};
  for (const f of fieldDefs) {
    if (checked.value[f.key]) out[f.key] = values.value[f.key];
  }
  emit("apply", out);
  emit("close");
}

const fieldCls = "apple-input";
const textareaCls = "apple-textarea";
const selectCls = "apple-select";
</script>

<template>
  <VModal :visible="open" title="AI 属性生成" :width="620" @close="emit('close')">
    <div class="space-y-4">
      <!-- 全选控制 -->
      <div class="flex gap-2">
        <button
          type="button"
          class="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary hover:bg-accent"
          @click="setAll(true)"
        >
          选择所有
        </button>
        <button
          type="button"
          class="rounded-lg border border-border px-3 py-1.5 text-xs text-text-secondary hover:bg-accent"
          @click="setAll(false)"
        >
          取消选择所有
        </button>
      </div>

      <!-- 字段行：复选框 + 输入控件 + 单项生成 -->
      <div v-for="f in fieldDefs" :key="f.key" class="space-y-1.5">
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="checked[f.key]" type="checkbox" class="rounded" />
          {{ f.label }}
        </label>
        <div class="flex items-start gap-1.5">
          <select v-if="f.kind === 'select'" v-model="values[f.key]" :class="selectCls + ' flex-1 min-w-0'">
            <option value=""></option>
            <option v-for="c in categoryOptions" :key="c" :value="c">{{ c }}</option>
          </select>
          <textarea
            v-else-if="f.kind === 'textarea'"
            v-model="values[f.key]"
            rows="3"
            :class="textareaCls + ' flex-1 min-w-0 resize-y'"
          ></textarea>
          <input
            v-else
            v-model="values[f.key]"
            :class="fieldCls + ' flex-1 min-w-0'"
            :placeholder="f.key === 'tags' ? '逗号分隔' : ''"
          />
          <button
            type="button"
            :title="'生成' + f.label"
            :disabled="loadingFields.has(f.key) || batchLoading"
            class="shrink-0 rounded-lg border border-border p-2 text-text-tertiary transition-colors hover:bg-accent hover:text-primary disabled:cursor-not-allowed disabled:opacity-50"
            @click="generateOne(f.key)"
          >
            <Icon
              :icon="loadingFields.has(f.key) ? 'ri:loader-4-line' : 'ri:magic-line'"
              :class="['text-base', loadingFields.has(f.key) && 'animate-spin']"
            />
          </button>
        </div>
      </div>

      <!-- 一键生成 -->
      <button
        type="button"
        :disabled="batchLoading || checkedCount === 0"
        class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-2 text-sm text-text-secondary transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
        @click="generateAll"
      >
        <Icon :icon="batchLoading ? 'ri:loader-4-line' : 'ri:magic-line'" :class="batchLoading && 'animate-spin'" />
        一键生成 {{ checkedCount }} 项
      </button>
    </div>

    <template #footer>
      <div class="flex gap-2">
        <VButton type="secondary" :disabled="checkedCount === 0" @click="save">保存 {{ checkedCount }} 项</VButton>
        <VButton @click="emit('close')">关闭</VButton>
      </div>
    </template>
  </VModal>
</template>
