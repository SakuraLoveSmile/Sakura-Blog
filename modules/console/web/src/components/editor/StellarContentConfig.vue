<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { http } from "@blog/api-client/transport";
import { editorImportApi } from "@/api";
import { Toast, VButton } from "@/components/ui";
import { getApiErrorMessage } from "@/api";

const props = withDefaults(
  defineProps<{
    syncError?: string;
    version: number;
    config: string;
    blog: string;
    profile: string;
    collection?: boolean;
    content?: string;
  }>(),
  { collection: false, content: "" },
);
const emit = defineEmits<{ "update:config": [string]; "update:blog": [string] }>();
const busy = ref(false);
const diagnostic = ref("");
const rootEl = ref<HTMLDetailsElement>();
const open = ref(false);

// 只有出现明确问题才抢占视线：基本表单同步失败，或旧内容尚未迁移到 2.0。
// 平时它只是表单末尾的一行入口，展开后才是完整 JSON 原文。
const needsAttention = computed(() => Boolean(props.syncError) || props.version !== 2);

// 直接改 DOM 属性，避免 :open 绑定与用户手动折叠互相覆盖；条件成立时自动展开，
// 条件消失后不强制收起，保留用户自己的开合选择。
watch(
  needsAttention,
  (attention) => {
    if (attention && rootEl.value) {
      rootEl.value.open = true;
      open.value = true;
    }
  },
  { immediate: true, flush: "post" },
);

function onToggle(event: Event) {
  open.value = (event.target as HTMLDetailsElement).open;
}

async function validate() {
  busy.value = true;
  diagnostic.value = "";
  try {
    const { data } = await http.post<{ issues: { path: string; code: string }[] }>("/editor/stellar/validate", {
      stellar_version: props.version,
      stellar_config: props.config,
      blog_extensions: props.blog,
      profile: props.profile,
      collection: props.collection,
    });
    diagnostic.value = data.issues.length
      ? data.issues.map((issue) => `${issue.path}: ${issue.code}`).join("\n")
      : "校验通过";
  } catch (error) {
    diagnostic.value = getApiErrorMessage(error);
  } finally {
    busy.value = false;
  }
}

async function exportDocument() {
  busy.value = true;
  try {
    const { data } = await editorImportApi.exportStellar({
      front_matter: props.config,
      blog_extensions: props.blog,
      content_md: props.content,
    });
    const url = URL.createObjectURL(new Blob([data.content], { type: "text/markdown;charset=utf-8" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "stellar-document.md";
    a.click();
    URL.revokeObjectURL(url);
  } catch (error) {
    Toast.error(getApiErrorMessage(error));
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <details ref="rootEl" class="stellar-raw-config" @toggle="onToggle">
    <summary>
      <Icon
        :icon="open ? 'ri:arrow-up-s-line' : 'ri:arrow-down-s-line'"
        class="stellar-raw-config__chevron"
        aria-hidden="true"
      />
      <span class="stellar-raw-config__label">高级：配置原文</span>
      <span v-if="needsAttention" class="stellar-raw-config__dot" role="img" aria-label="配置需要检查" />
    </summary>
    <div class="stellar-raw-config__body">
      <p v-if="syncError" role="alert" class="text-sm text-destructive">{{ syncError }}</p>
      <p v-if="version !== 2" class="text-sm text-destructive" role="status">
        这条旧内容尚未迁移，仅可预览。请先审查迁移候选；保存不会自动转换旧字段。
      </p>
      <p class="stellar-raw-config__hint">
        完整 JSON 按原文保存；Stellar 字段与 Blog 扩展分开。合法原文修改会同步基本控件，基本控件修改会同步对应 JSON
        字段；仅编辑的字段被重写。未知字段和归属冲突会明确报错。
      </p>
      <label class="stellar-raw-config__field"
        >Stellar {{ collection ? "集合" : "内容" }}配置
        <textarea
          :value="config"
          @input="emit('update:config', ($event.target as HTMLTextAreaElement).value)"
          rows="9"
          spellcheck="false"
          class="apple-textarea stellar-raw-config__code"
        />
      </label>
      <label class="stellar-raw-config__field"
        >Blog 扩展
        <textarea
          :value="blog"
          @input="emit('update:blog', ($event.target as HTMLTextAreaElement).value)"
          rows="5"
          spellcheck="false"
          class="apple-textarea stellar-raw-config__code"
        />
      </label>
      <div class="flex flex-wrap gap-2">
        <VButton :loading="busy" :disabled="!!syncError" @click="validate">只校验</VButton>
        <VButton
          v-if="!collection && (profile === 'post' || profile === 'page')"
          :disabled="busy || version !== 2 || !!syncError"
          @click="exportDocument"
          >导出 Markdown</VButton
        >
      </div>
      <pre v-if="diagnostic" class="whitespace-pre-wrap break-words text-xs text-text-secondary" role="status">{{
        diagnostic
      }}</pre>
    </div>
  </details>
</template>

<style scoped>
/* 平时只是表单末尾的一行入口，不带卡片皮肤；展开后才是完整的 JSON 原文区域。
   字号与颜色低于正文控件，避免与用户的常规编辑抢注意力。 */
.stellar-raw-config {
  min-width: 0;
  border-top: 1px solid var(--color-border);
}

.stellar-raw-config > summary {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  min-height: 44px;
  padding: 8px 0;
  list-style: none;
  color: var(--color-text-quaternary);
  font-size: 12px;
  line-height: 1.5;
  cursor: pointer;
}

/* Safari 仍会渲染默认三角，颜色由上面的 color 继承。 */
.stellar-raw-config > summary::-webkit-details-marker {
  display: none;
}

.stellar-raw-config > summary:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
  border-radius: var(--radius);
}

.stellar-raw-config__chevron {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
}

.stellar-raw-config__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stellar-raw-config__dot {
  width: 6px;
  height: 6px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--color-state-warning);
}

.stellar-raw-config__body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  padding-bottom: 16px;
}

.stellar-raw-config__hint {
  color: var(--color-text-quaternary);
  font-size: 12px;
  line-height: 1.6;
}

.stellar-raw-config__field {
  display: block;
  min-width: 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
}

.stellar-raw-config__code {
  width: 100%;
  margin-top: 6px;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
}

.stellar-raw-config > summary:hover .stellar-raw-config__label {
  color: var(--color-text-tertiary);
}
</style>
