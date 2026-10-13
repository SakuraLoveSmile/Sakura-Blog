<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { VButton, Toast } from "@/components/ui";
import { getApiErrorMessage, readStellarSettingsProblem, stellarSettingsApi } from "@/api";
import type { StellarSettingsIssue, StellarSettingsMigration } from "@/api";

const props = defineProps<{ saving?: string }>();
const emit = defineEmits<{ dirty: [value: boolean]; busy: [value: boolean] }>();
const draft = ref('{\n  "version": 2,\n  "theme": {},\n  "blog_extensions": {}\n}');
const saved = ref(draft.value);
const loading = ref(true);
const busy = ref<"" | "save" | "validate">("");
const loadError = ref("");
const migration = ref<StellarSettingsMigration>();
const checkedDraft = ref("");
const validationMessage = ref("");
const issues = ref<StellarSettingsIssue[]>([]);
const valid = ref(false);

const dirty = computed(() => !loading.value && draft.value !== saved.value);
const disabled = computed(() => loading.value || Boolean(busy.value) || Boolean(props.saving));
const showValidation = computed(() => checkedDraft.value === draft.value && Boolean(validationMessage.value));
watch(dirty, (value) => emit("dirty", value));

async function load() {
  if (dirty.value && !window.confirm("重新读取将替换未保存的 Stellar 2 配置，是否继续？")) return;
  loading.value = true;
  loadError.value = "";
  migration.value = undefined;
  checkedDraft.value = "";
  try {
    const { data } = await stellarSettingsApi.get();
    draft.value = data;
    saved.value = data;
  } catch (error) {
    const problem = readStellarSettingsProblem(error);
    migration.value = problem?.migration;
    loadError.value = problem?.detail || getApiErrorMessage(error, "读取 Stellar 2 配置失败");
  } finally {
    loading.value = false;
  }
}

async function submit(action: "save" | "validate") {
  if (disabled.value || (action === "save" && loadError.value)) return;
  busy.value = action;
  emit("busy", true);
  const payload = draft.value;
  try {
    if (action === "validate") {
      const { data } = await stellarSettingsApi.validate(payload);
      checkedDraft.value = payload;
      valid.value = data.valid;
      issues.value = data.issues;
      validationMessage.value = "配置符合 Stellar 2.0.0 规则。校验不会修改已保存配置。";
    } else {
      const { data } = await stellarSettingsApi.save(payload);
      saved.value = data;
      if (draft.value === payload) draft.value = data;
      checkedDraft.value = "";
      Toast.success("Stellar 2 配置已保存，前台已热更新");
    }
  } catch (error) {
    const problem = readStellarSettingsProblem(error);
    if (problem?.migration) {
      migration.value = problem.migration;
      loadError.value = problem.detail;
    }
    checkedDraft.value = payload;
    valid.value = false;
    issues.value = problem?.issues || [];
    validationMessage.value = problem?.detail || getApiErrorMessage(error, "配置操作失败，请检查语法或网络");
  } finally {
    busy.value = "";
    emit("busy", false);
  }
}

onMounted(() => void load());
</script>

<template>
  <div class="stellar2-form space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-2">
      <div class="space-y-1">
        <h2 class="text-base font-semibold text-text-primary">Stellar 2 完整配置</h2>
        <p id="stellar2-help" class="text-xs text-text-tertiary">
          theme 使用上游配置键名；blog_extensions 保存原生评论等本地功能。直接编辑完整 JSON，保存后前台即时生效。
        </p>
        <p class="text-xs text-text-quaternary">敏感字段显示为 ****；保留占位即可保持原值，显式空值可清除。</p>
      </div>
      <span v-if="dirty" class="text-xs text-state-warning whitespace-nowrap">（有未保存修改）</span>
    </div>

    <div v-if="loading" class="text-xs text-text-tertiary" role="status">正在读取 Stellar 2 配置...</div>
    <div v-else-if="migration" class="space-y-2 text-sm text-state-warning" role="status">
      <p>当前配置尚未迁移。可以编辑草稿并校验；完成迁移预览与确认后才能保存。</p>
      <p class="text-xs break-words">位置：{{ migration.source }}.{{ migration.path }}</p>
      <p class="text-xs break-words">现有配置段：{{ migration.sections.join("、") || "空" }}</p>
    </div>
    <div v-else-if="loadError" class="text-sm text-state-error break-words" role="alert">{{ loadError }}</div>

    <label for="stellar2-document" class="block text-sm text-text-secondary">
      {{ migration ? "配置校验草稿" : "配置文档" }}
    </label>
    <textarea
      id="stellar2-document"
      v-model="draft"
      spellcheck="false"
      :disabled="loading"
      aria-describedby="stellar2-help"
      class="apple-textarea code-style w-full font-mono text-xs"
      style="min-height: 420px"
    ></textarea>

    <div v-if="showValidation" class="space-y-2" role="status" aria-live="polite">
      <p class="text-sm break-words" :class="valid ? 'text-state-success' : 'text-state-error'">
        {{ validationMessage }}
      </p>
      <ul v-if="issues.length" class="space-y-1 text-xs text-text-secondary">
        <li v-for="(issue, index) in issues" :key="index" class="break-words">
          <code>{{ issue.source }}.{{ issue.path }}</code
          >：{{ issue.code }}
          <span v-if="issue.expected">（需要 {{ issue.expected }}）</span>
        </li>
      </ul>
    </div>

    <div class="flex flex-wrap gap-2">
      <VButton type="primary" :disabled="disabled || Boolean(loadError)" @click="submit('save')">
        {{ busy === "save" ? "保存中..." : "保存 Stellar 2 配置" }}
      </VButton>
      <VButton type="secondary" :disabled="disabled" @click="submit('validate')">
        {{ busy === "validate" ? "校验中..." : "校验配置" }}
      </VButton>
      <VButton type="secondary" :disabled="disabled" @click="load">重新读取</VButton>
    </div>
  </div>
</template>

<style scoped>
/* 与现有 BasicSettings 的 section 对齐，外层 VCard body 本身没有 padding。 */
.stellar2-form {
  padding: 24px 28px 28px;
}
@media (max-width: 767px) {
  .stellar2-form {
    padding-left: 16px;
    padding-right: 16px;
  }
}
</style>
