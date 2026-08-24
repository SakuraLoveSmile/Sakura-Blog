/** Schema-driven form field definitions shared by Console features. */
export type SchemaFieldType = "text" | "password" | "select" | "number" | "switch" | "color" | "textarea";

export interface SchemaField {
  key: string;
  type: SchemaFieldType;
  label?: string;
  label_key?: string;
  default?: unknown;
  options?: string[];
  placeholder?: string;
  hint?: string;
  hint_key?: string;
  rows?: number;
}

export type SchemaValues = Record<string, unknown>;

function emptyValueFor(type: SchemaFieldType): unknown {
  switch (type) {
    case "number":
      return 0;
    case "switch":
      return false;
    default:
      return "";
  }
}

/** Fill missing fields without discarding values from an extensible schema. */
export function withSchemaDefaults(fields: SchemaField[], values: SchemaValues = {}): SchemaValues {
  const result: SchemaValues = { ...values };
  for (const field of fields) {
    if (result[field.key] === undefined) {
      result[field.key] = field.default ?? emptyValueFor(field.type);
    }
  }
  return result;
}

/** Native color inputs only accept six-digit hexadecimal values. */
export function colorInputValue(value: unknown): string {
  const raw = typeof value === "string" ? value.trim() : "";
  if (/^#[0-9a-f]{6}$/i.test(raw)) return raw;
  if (/^#[0-9a-f]{3}$/i.test(raw)) {
    return `#${raw
      .slice(1)
      .split("")
      .map((part) => part + part)
      .join("")}`;
  }
  return "#000000";
}
