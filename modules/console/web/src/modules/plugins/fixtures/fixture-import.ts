import type { FixtureItem, FixtureState } from "./fixture-types";
import { validateFixtureData } from "./fixture-validation";

export interface PersistedFixtureState {
  customData: Record<string, string>;
  states: Record<string, FixtureState>;
}

export type FixtureImportResult = { success: true; state: PersistedFixtureState } | { success: false; error: string };

const VALID_STATES = new Set<FixtureState>(["normal", "empty", "error"]);

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function findFixture(fixtures: FixtureItem[], id: string): FixtureItem | undefined {
  return fixtures.find((fixture) => fixture.id === id);
}

export function parseFixtureImport(raw: string, fixtures: FixtureItem[]): FixtureImportResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : "无法解析 JSON";
    return { success: false, error: `JSON syntax error: ${message}` };
  }

  if (!isObject(parsed)) {
    return { success: false, error: "导入文件格式错误，必须为 JSON 对象。" };
  }
  if (parsed.version !== 1) {
    return { success: false, error: "不支持的 Fixture 配置版本，必须为 version 1。" };
  }
  if (!isObject(parsed.customData)) {
    return { success: false, error: "customData 必须为 JSON 对象。" };
  }
  if (!isObject(parsed.states)) {
    return { success: false, error: "states 必须为 JSON 对象。" };
  }

  const customData: Record<string, string> = {};
  for (const [id, value] of Object.entries(parsed.customData)) {
    const fixture = findFixture(fixtures, id);
    if (!fixture) {
      return { success: false, error: `customData 包含未知 Fixture ID：${id}` };
    }
    if (typeof value !== "string") {
      return { success: false, error: `customData.${id} 必须为 string` };
    }
    const validation = validateFixtureData(fixture, value);
    if (!validation.valid) {
      return {
        success: false,
        error: `customData.${id} 校验失败：${validation.errors.join("；")}`,
      };
    }
    customData[id] = value;
  }

  const states: Record<string, FixtureState> = {};
  for (const [id, value] of Object.entries(parsed.states)) {
    if (!findFixture(fixtures, id)) {
      return { success: false, error: `states 包含未知 Fixture ID：${id}` };
    }
    if (typeof value !== "string" || !VALID_STATES.has(value as FixtureState)) {
      return { success: false, error: `states.${id} 必须为 normal、empty 或 error` };
    }
    states[id] = value as FixtureState;
  }

  return { success: true, state: { customData, states } };
}
