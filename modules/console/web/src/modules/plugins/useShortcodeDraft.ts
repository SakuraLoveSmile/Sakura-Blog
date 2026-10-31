import { computed, onScopeDispose, ref } from "vue";
import type { ShortcodeEditResult } from "@blog/api-client";
import type { PluginMeta, PluginVariant } from "./plugin-types";
import { formForVariant, initFormValues, validateEditValues } from "./shortcode";
import { RenderGate } from "./preview/render-gate";

// Studio 与插入面板共用原文/参数状态；源码只能由后端解析，不能从旧表单猜测回填。
export function useShortcodeDraft(
  getPlugin: () => PluginMeta | null,
  parse: (raw: string) => Promise<ShortcodeEditResult>,
) {
  const raw = ref("");
  const initialValues = ref<Record<string, unknown> | undefined>();
  const present = ref<string[] | undefined>();
  const variant = ref<PluginVariant | undefined>();
  const structured = ref(false);
  const parsing = ref(false);
  const sourceReason = ref("");
  const formValid = ref(false);
  const formError = ref("");
  const gate = new RenderGate(250);

  const formPlugin = computed(() => {
    const plugin = getPlugin();
    return plugin && structured.value && !parsing.value ? formForVariant(plugin, variant.value) : null;
  });
  const canApply = computed(() => !!raw.value.trim() && !parsing.value && (!structured.value || formValid.value));

  function startInsertion() {
    gate.invalidate();
    const plugin = getPlugin();
    initialValues.value = undefined;
    present.value = undefined;
    variant.value = undefined;
    parsing.value = false;
    sourceReason.value = "";
    structured.value = !!plugin?.form?.length;
    raw.value = structured.value ? "" : plugin?.example || "";
    formError.value = plugin ? validateEditValues(plugin, initFormValues(plugin.form || [])) || "" : "";
    formValid.value = !formError.value;
  }

  function updateSource(source: string, immediate = false) {
    raw.value = source;
    sourceReason.value = "";
    const plugin = getPlugin();
    if (!plugin || !source.trim()) {
      gate.invalidate();
      structured.value = false;
      parsing.value = false;
      initialValues.value = undefined;
      present.value = undefined;
      variant.value = undefined;
      formValid.value = false;
      formError.value = "";
      return;
    }
    parsing.value = true;
    const seq = gate.schedule(async () => {
      try {
        const parsed = await parse(source);
        if (!gate.isCurrent(seq)) return;
        if (!parsed.editable || parsed.name.toLowerCase() !== plugin.name.toLowerCase()) {
          structured.value = false;
          sourceReason.value = "当前源码包含无法可靠转换的语法，保留源码编辑。";
          return;
        }
        initialValues.value = parsed.values;
        present.value = parsed.present || [];
        variant.value = parsed.variant;
        structured.value = true;
        formError.value =
          validateEditValues(
            formForVariant(plugin, parsed.variant),
            parsed.values as Record<string, string | boolean>,
          ) || "";
        formValid.value = !formError.value;
      } catch {
        if (!gate.isCurrent(seq)) return;
        structured.value = false;
        sourceReason.value = "参数解析失败，原文已保留，可继续编辑源码。";
      } finally {
        if (gate.isCurrent(seq)) parsing.value = false;
      }
    }, immediate);
  }

  function updateForm(source: string) {
    if (parsing.value || !structured.value || !formValid.value) return;
    raw.value = source;
  }

  function updateValidation(state: { valid: boolean; error?: string }) {
    if (parsing.value) return;
    formValid.value = state.valid;
    formError.value = state.error || "";
  }

  onScopeDispose(() => gate.dispose());

  return {
    raw,
    initialValues,
    present,
    variant,
    structured,
    parsing,
    sourceReason,
    formValid,
    formError,
    formPlugin,
    canApply,
    startInsertion,
    updateSource,
    updateForm,
    updateValidation,
  };
}
