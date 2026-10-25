import { computed, ref } from "vue";
import { getApiErrorMessage, readStellarSettingsProblem, stellarSettingsApi } from "@blog/api-client";
import type {
  StellarSettingsIssue,
  StellarSettingsMigration,
  StellarSettingsValidationResponse,
} from "@blog/api-client";
import { patchJSONField } from "../../components/editor/stellar-form-json";

interface StellarSettingsAPI {
  get(): Promise<{ data: string }>;
  save(document: string): Promise<{ data: string }>;
  validate(document: string): Promise<{ data: StellarSettingsValidationResponse }>;
}
interface StellarSettingsOptions {
  api?: StellarSettingsAPI;
  confirmReload?: () => boolean;
}

// 普通表单和完整 JSON 编辑器共用同一份原文，不重编码未编辑的参数和敏感占位值。
export function useStellarSettings(options: StellarSettingsOptions = {}) {
  const api = options.api ?? stellarSettingsApi;
  const draft = ref('{\n  "version": 2,\n  "theme": {},\n  "blog_extensions": {}\n}');
  const saved = ref(draft.value);
  const loading = ref(true);
  const busy = ref<"" | "save" | "validate">("");
  const loadError = ref("");
  const migration = ref<StellarSettingsMigration>();
  const issues = ref<StellarSettingsIssue[]>([]);
  const validationMessage = ref("");
  const valid = ref(false);
  const pendingInput = ref(false);
  const loadRevision = ref(0);
  const checkedDraft = ref<string>();
  let loadingRequest = false;

  const dirty = computed(() => draft.value !== saved.value || pendingInput.value);
  const parseError = computed(() => {
    try {
      const document = JSON.parse(draft.value);
      if (!document || typeof document !== "object" || Array.isArray(document)) return "配置必须是 JSON 对象。";
      for (const key of ["theme", "blog_extensions"]) {
        const section = document[key];
        if (section !== undefined && (!section || typeof section !== "object" || Array.isArray(section)))
          return `${key} 必须是 JSON 对象，请在完整配置中修正。`;
      }
      return "";
    } catch {
      return "JSON 语法有误，请在完整配置中修正后继续使用表单。";
    }
  });
  const showValidation = computed(() => checkedDraft.value === draft.value && Boolean(validationMessage.value));

  async function load(): Promise<boolean> {
    if (loadingRequest || busy.value) return false;
    if (
      dirty.value &&
      !(options.confirmReload ?? (() => window.confirm("重新读取将替换未保存的 Stellar 2 配置，是否继续？")))()
    )
      return false;
    loadingRequest = true;
    loading.value = true;
    loadError.value = "";
    migration.value = undefined;
    checkedDraft.value = undefined;
    try {
      const { data } = await api.get();
      draft.value = data;
      saved.value = data;
      pendingInput.value = false;
      loadRevision.value++;
      return true;
    } catch (error) {
      const problem = readStellarSettingsProblem(error);
      migration.value = problem?.migration;
      loadError.value = problem?.detail || getApiErrorMessage(error, "读取 Stellar 2 配置失败");
      return false;
    } finally {
      loadingRequest = false;
      loading.value = false;
    }
  }

  async function submit(action: "save" | "validate"): Promise<boolean> {
    if (loading.value || busy.value || (action === "save" && (loadError.value || migration.value))) return false;
    if (action === "save" && pendingInput.value) {
      checkedDraft.value = draft.value;
      valid.value = false;
      issues.value = [];
      validationMessage.value = "部分设置还有未完成的输入，请修正后再保存";
      return false;
    }
    busy.value = action;
    const payload = draft.value;
    try {
      if (action === "validate") {
        const { data } = await api.validate(payload);
        checkedDraft.value = payload;
        valid.value = data.valid;
        issues.value = data.issues;
        validationMessage.value = data.valid
          ? "配置符合 Stellar 2.0.0 规则。校验不会修改已保存配置。"
          : "配置未通过校验，请根据诊断修改。";
        return data.valid;
      }
      const { data } = await api.save(payload);
      saved.value = data;
      if (draft.value === payload) draft.value = data;
      checkedDraft.value = undefined;
      return true;
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
      return false;
    } finally {
      busy.value = "";
    }
  }

  function updateField(path: string, value: unknown, rawValue?: string): boolean {
    if (loading.value || parseError.value) return false;
    try {
      draft.value = patchJSONField(draft.value, path, value, rawValue);
      return true;
    } catch (error) {
      // 原文保护拒绝有精度歧义的操作时，保留草稿并展示可操作的原因。
      checkedDraft.value = draft.value;
      valid.value = false;
      issues.value = [];
      validationMessage.value = `此项未修改：${error instanceof Error ? error.message : "无法更新配置，请在高级原文中检查。"}`;
      return false;
    }
  }

  return {
    draft,
    saved,
    loading,
    busy,
    loadError,
    migration,
    dirty,
    parseError,
    pendingInput,
    loadRevision,
    issues,
    validationMessage,
    showValidation,
    valid,
    load,
    submit,
    updateField,
  };
}

export type StellarSettingsEditor = ReturnType<typeof useStellarSettings>;
