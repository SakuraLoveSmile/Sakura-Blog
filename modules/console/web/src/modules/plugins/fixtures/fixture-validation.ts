import type { FixtureItem } from "./fixture-types";

export interface FixtureValidationResult {
  valid: boolean;
  errors: string[];
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function invalid(...errors: string[]): FixtureValidationResult {
  return { valid: false, errors };
}

function valid(): FixtureValidationResult {
  return { valid: true, errors: [] };
}

function parseJson(raw: string): FixtureValidationResult | { data: unknown } {
  try {
    return { data: JSON.parse(raw) };
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : "无法解析 JSON";
    return invalid(`JSON syntax error: ${message}`);
  }
}

function validateContentItems(fixtureId: string, data: Record<string, unknown>): FixtureValidationResult {
  if (!Array.isArray(data.content)) {
    return invalid(`${fixtureId}.content 应为 array`);
  }

  for (const [index, item] of data.content.entries()) {
    if (!isObject(item)) {
      return invalid(`${fixtureId}.content[${index}] 应为 object`);
    }
    if (typeof item.title !== "string") {
      return invalid(`${fixtureId}.content[${index}].title 应为 string`);
    }
    if (typeof item.url !== "string") {
      return invalid(`${fixtureId}.content[${index}].url 应为 string`);
    }
  }

  return valid();
}

export function validateFixtureData(fixture: FixtureItem, raw: string): FixtureValidationResult {
  if (fixture.format === "markdown") return valid();

  const parsed = parseJson(raw);
  if (!("data" in parsed)) return parsed;
  if (!isObject(parsed.data)) {
    return invalid(`${fixture.id} 数据必须是 object`);
  }

  switch (fixture.id) {
    case "friends":
    case "sites":
      return validateContentItems(fixture.id, parsed.data);
    case "rating":
      if (typeof parsed.data.avg !== "number" || !Number.isFinite(parsed.data.avg)) {
        return invalid("rating.avg 应为 number");
      }
      if (typeof parsed.data.count !== "number" || !Number.isFinite(parsed.data.count) || parsed.data.count < 0) {
        return invalid("rating.count 应为 number 且不小于 0");
      }
      return valid();
    case "vote":
      if (!isObject(parsed.data.votes)) {
        return invalid("vote.votes 应为 object");
      }
      if (
        typeof parsed.data.votes.up !== "number" ||
        !Number.isFinite(parsed.data.votes.up) ||
        parsed.data.votes.up < 0
      ) {
        return invalid("vote.votes.up 应为 number 且不小于 0");
      }
      if (
        typeof parsed.data.votes.down !== "number" ||
        !Number.isFinite(parsed.data.votes.down) ||
        parsed.data.votes.down < 0
      ) {
        return invalid("vote.votes.down 应为 number 且不小于 0");
      }
      return valid();
    default:
      return valid();
  }
}
