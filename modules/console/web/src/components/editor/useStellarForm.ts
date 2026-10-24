import { watch, ref } from "vue";
import { patchJSONField, readJSONField, readJSONFieldRaw, fingerprint } from "./stellar-form-json";

export interface StellarFormField {
  form: string;
  path: string;
  blog?: boolean;
  fallback?: unknown;
  read?: (value: any) => unknown;
  write?: (value: any, current: any) => unknown;
  raw?: boolean;
  host?: boolean;
}
function get(object: any, path: string): any {
  return path.split(".").reduce((value, key) => value?.[key], object);
}
function set(object: any, path: string, value: unknown) {
  const keys = path.split(".");
  for (const key of keys.slice(0, -1)) object = object[key];
  object[keys[keys.length - 1]] = value;
}
function snapshot(value: any): any {
  if (!value || typeof value !== "object") return value;
  const result: any = Array.isArray(value) ? [] : {};
  for (const key of Reflect.ownKeys(value)) if (key !== "length") result[key] = snapshot(value[key]);
  return result;
}
// 原文/新记录优先回填；之后每次基本控件修改只 patch 对应字段。
// flush:sync 保证点击保存前已完成同步，guard 避免回填触发反向写入。
export function useStellarForm(
  getForm: () => any,
  fields: StellarFormField[],
  configKey = "stellar_config",
  blogKey = "blog_extensions",
) {
  const error = ref("");
  let syncing = false,
    previous: any,
    previousConfig = "",
    previousBlog = "";
  let baseline: string[] = [];
  let baselineValues: any[] = [];
  const rawDraftErrors = new Map<string, string>();
  watch(
    () => {
      const form = getForm();
      return [form, form[configKey], form[blogKey], ...fields.map((field) => fingerprint(get(form, field.form)))];
    },
    () => {
      if (syncing) return;
      syncing = true;
      const form = getForm();
      try {
        error.value = "";
        if (form.stellar_version !== undefined && form.stellar_version !== 2) return;
        const changedRecord = previous !== form;
        const configChanged = changedRecord || previousConfig !== form[configKey];
        const blogChanged = changedRecord || previousBlog !== form[blogKey];
        // 任一原文不合法时不以控件值覆盖；服务端保存校验返回具体诊断。
        readJSONField(form[configKey], "");
        readJSONField(form[blogKey], "");
        fields.forEach((field, index) => {
          const rawKey = field.blog ? blogKey : configKey;
          if (field.blog ? blogChanged : configChanged) {
            rawDraftErrors.delete(field.form);
            const value = field.raw
              ? readJSONFieldRaw(form[rawKey], field.path)
              : readJSONField(form[rawKey], field.path);
            if (value === undefined && field.host) return;
            set(form, field.form, field.read ? field.read(value) : (value ?? field.fallback ?? ""));
          } else if (baseline[index] !== fingerprint(get(form, field.form))) {
            const value = get(form, field.form);
            try {
              form[rawKey] = patchJSONField(
                form[rawKey],
                field.path,
                field.write ? field.write(value, readJSONField(form[rawKey], field.path)) : value,
                field.raw ? value || "{}" : undefined,
              );
              rawDraftErrors.delete(field.form);
            } catch (problem) {
              if (!field.raw) throw problem;
              rawDraftErrors.set(
                field.form,
                "参数 JSON 尚未完成：" + (problem instanceof Error ? problem.message : "无效原文"),
              );
            }
          }
        });
        // 多个旧控件消费同一个上游字段时同步回显（例如 Hero 与集合 headline）。
        fields.forEach((field) => {
          if (rawDraftErrors.has(field.form)) return;
          const rawKey = field.blog ? blogKey : configKey;
          const value = field.raw
            ? readJSONFieldRaw(form[rawKey], field.path)
            : readJSONField(form[rawKey], field.path);
          if (value === undefined && field.host) return;
          set(form, field.form, field.read ? field.read(value) : (value ?? field.fallback ?? ""));
        });
        error.value = [...rawDraftErrors.values()].join("\n");
      } catch (problem) {
        error.value = "请先修正配置 JSON，再修改基本字段：" + (problem instanceof Error ? problem.message : "原文无效");
        if (previous === form)
          fields.forEach((field, index) => {
            if (baseline[index] !== undefined && !field.raw) set(form, field.form, snapshot(baselineValues[index]));
          });
      } finally {
        previous = form;
        previousConfig = form[configKey];
        previousBlog = form[blogKey];
        baseline = fields.map((field) => fingerprint(get(form, field.form)));
        baselineValues = fields.map((field) => snapshot(get(form, field.form)));
        syncing = false;
      }
    },
    { immediate: true, flush: "sync" },
  );
  return error;
}
