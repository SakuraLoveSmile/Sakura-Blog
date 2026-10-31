export type SettingColor = { hex: string; alpha: number };

const clamp = (value: number, max = 1) => Math.min(max, Math.max(0, value));
const numeric = /^[-+]?(?:\d+\.?\d*|\.\d+)(?:e[-+]?\d+)?$/i;

function number(value: string): number | undefined {
  if (!numeric.test(value)) return undefined;
  const result = Number(value);
  return Number.isFinite(result) ? result : undefined;
}

function fraction(value: string): number | undefined {
  const percent = value.endsWith("%");
  const parsed = number(percent ? value.slice(0, -1) : value);
  return parsed === undefined ? undefined : clamp(parsed / (percent ? 100 : 1));
}

function channel(value: string): number | undefined {
  const percent = value.endsWith("%");
  const parsed = number(percent ? value.slice(0, -1) : value);
  return parsed === undefined ? undefined : Math.round(clamp(parsed / (percent ? 100 : 255)) * 255);
}

function hexFromChannels(channels: number[]): string {
  return `#${channels.map((value) => Math.round(value).toString(16).padStart(2, "0")).join("")}`;
}

function hue(value: string): number | undefined {
  const match = value.match(/^(.*?)(deg|grad|rad|turn)?$/);
  if (!match) return undefined;
  const parsed = number(match[1]);
  if (parsed === undefined) return undefined;
  const degrees = parsed * ({ deg: 1, grad: 0.9, rad: 180 / Math.PI, turn: 360 }[match[2] || "deg"] ?? 1);
  if (!Number.isFinite(degrees)) return undefined;
  return ((degrees % 360) + 360) % 360;
}

// 只转换取色器可表达的颜色。其他 CSS 值留给原文输入，不猜测或改写。
export function parseSettingColor(value: string): SettingColor | undefined {
  const text = value.trim().toLowerCase();
  if (text === "transparent") return { hex: "#000000", alpha: 0 };
  if (/^#[\da-f]{3,4}$/.test(text)) {
    return parseSettingColor(
      `#${text
        .slice(1)
        .split("")
        .map((digit) => digit + digit)
        .join("")}`,
    );
  }
  if (/^#[\da-f]{6}(?:[\da-f]{2})?$/.test(text)) {
    return { hex: text.slice(0, 7), alpha: text.length === 9 ? parseInt(text.slice(7), 16) / 255 : 1 };
  }
  const functional = text.match(/^(rgba?|hsla?)\((.*)\)$/);
  if (!functional) return undefined;
  const body = functional[2].trim();
  const comma = body.includes(",");
  if (comma && body.includes("/")) return undefined;
  const parts = comma ? body.split(",").map((part) => part.trim()) : body.split("/").map((part) => part.trim());
  let channels: string[];
  let alphaText: string | undefined;
  if (comma) {
    if (parts.length !== 3 && parts.length !== 4) return undefined;
    channels = parts.slice(0, 3);
    alphaText = parts[3];
  } else {
    if (parts.length > 2) return undefined;
    channels = parts[0].split(/\s+/);
    alphaText = parts[1];
    if (channels.length !== 3) return undefined;
  }
  const alpha = alphaText === undefined ? 1 : fraction(alphaText);
  if (alpha === undefined) return undefined;
  if (functional[1].startsWith("rgb")) {
    // 逗号语法不允许数字与百分比通道混用。
    if (comma && channels.some((part) => part.endsWith("%")) && !channels.every((part) => part.endsWith("%")))
      return undefined;
    const rgb = channels.map(channel);
    return rgb.some((part) => part === undefined) ? undefined : { hex: hexFromChannels(rgb as number[]), alpha };
  }
  if (!channels[1].endsWith("%") || !channels[2].endsWith("%")) return undefined;
  const h = hue(channels[0]);
  const s = fraction(channels[1]);
  const l = fraction(channels[2]);
  if (h === undefined || s === undefined || l === undefined) return undefined;
  const amplitude = s * Math.min(l, 1 - l);
  const rgb = [0, 8, 4].map((offset) => {
    const k = (offset + h / 30) % 12;
    return 255 * (l - amplitude * Math.max(-1, Math.min(k - 3, 9 - k, 1)));
  });
  return { hex: hexFromChannels(rgb), alpha };
}

function formatSettingColor(color: SettingColor): string {
  if (color.alpha >= 1) return color.hex;
  const rgb = [1, 3, 5].map((offset) => parseInt(color.hex.slice(offset, offset + 2), 16));
  return `rgba(${rgb.join(", ")}, ${Number(color.alpha.toFixed(4))})`;
}

export function replaceSettingColor(value: string, hex: string): string {
  const next = parseSettingColor(hex);
  if (!next) return value;
  return formatSettingColor({ hex: next.hex, alpha: parseSettingColor(value)?.alpha ?? 1 });
}

export function changeSettingColorAlpha(value: string, percent: number): string {
  const color = parseSettingColor(value);
  if (!color || !Number.isFinite(percent)) return value;
  return formatSettingColor({ ...color, alpha: clamp(percent / 100) });
}
