/**
 * 仪表盘组件的实例配置工具（纯函数，不引入任何 .vue）。
 *
 * 单独成文件的原因：`widgets/registry.ts` 会 import 组件 `.vue`，
 * 而 node:test 无法加载 `.vue`，所以配置相关的可测逻辑必须与组件导入分离。
 * 这里只依赖类型（`import type` 在编译期被擦除）。
 */
import type { WidgetConfigField, WidgetDefinition } from "./registry";

export interface WidgetConfigOption {
  value: string;
  label: string;
}

/** 把一个 configField 的候选值展开成 {value,label}，兼容纯字符串写法。 */
export function configFieldOptions(field: WidgetConfigField): WidgetConfigOption[] {
  return (field.options ?? []).map((option) =>
    typeof option === "string" ? { value: option, label: option } : option,
  );
}

/**
 * 组件实例配置 = 默认值叠加用户保存值。
 *
 * 只接受 `configFields` 声明过的键，并按字段类型收敛值；历史脏数据里的
 * 未知键不会进入渲染，避免旧布局影响组件行为。
 */
export function resolveWidgetConfig(
  definition: WidgetDefinition | undefined,
  saved: Record<string, unknown> | undefined,
): Record<string, unknown> {
  const base = definition?.defaultConfig ? { ...definition.defaultConfig } : {};
  if (!saved) return base;
  for (const field of definition?.configFields ?? []) {
    if (!(field.key in saved)) continue;
    const value = saved[field.key];
    if (field.type === "boolean") base[field.key] = value === true;
    else if (field.type === "number") {
      const n = typeof value === "number" ? value : Number(value);
      if (Number.isFinite(n)) base[field.key] = n;
    } else if (typeof value === "string") base[field.key] = value;
    else if (Array.isArray(value)) base[field.key] = [...value];
  }
  return base;
}

/** 取实例配置里的整数项（带范围钳制）；非法值回落默认。 */
export function configInt(
  config: Record<string, unknown> | undefined,
  key: string,
  fallback: number,
  min = 0,
  max = Number.MAX_SAFE_INTEGER,
): number {
  const raw = Number(config?.[key]);
  if (!Number.isFinite(raw)) return fallback;
  return Math.min(Math.max(Math.round(raw), min), max);
}
