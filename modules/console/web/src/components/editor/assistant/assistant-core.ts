// 写作助手的纯逻辑：动作定义、请求构建与截断、标题大纲、围栏感知的块边界、
// 标签（{% … %}）校验、快捷键判断。不依赖 DOM 与 Vue，便于单元测试。
import type { AIAssistRequest } from "@blog/api-client";
import type { AssistantPlacement } from "@blog/editor/assistant-bridge";

export type AssistAction = "polish" | "rewrite" | "expand" | "shorten" | "continue" | "translate" | "custom";

export interface AssistActionSpec {
  id: AssistAction;
  label: string;
  /** 必须选中文字才能执行。 */
  needsSelection: boolean;
  hint: string;
}

export const ASSIST_ACTIONS: readonly AssistActionSpec[] = [
  { id: "polish", label: "润色", needsSelection: true, hint: "修正错字、语病和标点，不改变原意" },
  { id: "rewrite", label: "改写", needsSelection: true, hint: "保持原意，换一种更清晰的说法" },
  { id: "expand", label: "扩写", needsSelection: true, hint: "补充细节和例子，篇幅约为原来的 1.5 到 2 倍" },
  { id: "shorten", label: "缩写", needsSelection: true, hint: "保留关键信息，压缩到原来的一半左右" },
  { id: "continue", label: "续写", needsSelection: false, hint: "接着当前段落往下写一段" },
  { id: "translate", label: "翻译", needsSelection: true, hint: "翻译成所选语言，保留格式和标签" },
];

export const ASSIST_LANGUAGES: readonly { code: string; label: string }[] = [
  { code: "en", label: "英文" },
  { code: "zh", label: "简体中文" },
  { code: "zh-tw", label: "繁体中文" },
  { code: "ja", label: "日文" },
  { code: "ko", label: "韩文" },
  { code: "fr", label: "法文" },
  { code: "de", label: "德文" },
  { code: "es", label: "西班牙文" },
  { code: "ru", label: "俄文" },
];

/** 与后端 modules/console/api/ai_assist.go 的上限保持一致（单位：字符，按 Unicode 码点计）。 */
export const ASSIST_LIMITS = {
  selection: 8000,
  before: 1500,
  after: 600,
  outlineItems: 60,
  outlineItemChars: 120,
  instruction: 500,
  title: 200,
} as const;

/** 助手结果在界面上的两种视图。 */
export type AssistView = "diff" | "result";

// ---------- 字符计数与截取（按码点，不把代理对劈开） ----------

export function countChars(text: string): number {
  return Array.from(text).length;
}

export function firstChars(text: string, n: number): string {
  if (n <= 0) return "";
  if (text.length <= n) return text;
  const head = Array.from(text.slice(0, n * 2));
  // 切口落在代理对中间时，末尾会剩下孤立的高代理项。
  const last = head[head.length - 1];
  if (last !== undefined && /^[\ud800-\udbff]$/.test(last)) head.pop();
  return head.slice(0, n).join("");
}

export function lastChars(text: string, n: number): string {
  if (n <= 0) return "";
  if (text.length <= n) return text;
  const tail = Array.from(text.slice(-(n * 2)));
  if (tail.length > 0 && /^[\udc00-\udfff]$/.test(tail[0])) tail.shift();
  return tail.slice(-n).join("");
}

/** 面板里展示选区摘录：折叠空白，过长时加省略号。 */
export function excerpt(text: string, max = 140): string {
  const flat = text.replace(/\s+/g, " ").trim();
  return countChars(flat) <= max ? flat : firstChars(flat, max) + "…";
}

// ---------- 标题大纲 ----------

const FENCE_LINE = /^ {0,3}(`{3,}|~{3,})(.*)$/;

/** 取 Markdown 正文里的标题文字（跳过围栏代码块），供后端了解文章结构。 */
export function extractOutline(markdown: string): string[] {
  const out: string[] = [];
  let fence: { ch: string; len: number } | null = null;
  for (const raw of markdown.split("\n")) {
    const line = raw.replace(/\r$/, "");
    const f = FENCE_LINE.exec(line);
    if (fence) {
      if (f && f[1][0] === fence.ch && f[1].length >= fence.len && f[2].trim() === "") fence = null;
      continue;
    }
    if (f) {
      fence = { ch: f[1][0], len: f[1].length };
      continue;
    }
    const heading = /^ {0,3}#{1,6}[ \t]+(.+?)(?:[ \t]+#+)?[ \t]*$/.exec(line);
    if (heading) out.push(heading[1].trim());
  }
  return out;
}

// ---------- 请求构建 ----------

export interface AssistRequestInput {
  action: AssistAction;
  /** 快照里的文字与前后文；前后文可以是整篇内容，这里按上限截取。 */
  context: { text: string; before: string; after: string };
  title: string;
  outline: readonly string[];
  /** 自定义指令的内容。 */
  instruction?: string;
  /** 翻译目标语言代码。 */
  targetLang?: string;
}

export function buildAssistRequest(input: AssistRequestInput): AIAssistRequest {
  const { action, context } = input;
  const request: AIAssistRequest = { action };

  // 续写只看光标之前的内容，选区不参与（后端同样忽略）。
  if (action !== "continue" && context.text.trim() !== "") request.selection = context.text;
  const before = lastChars(context.before, ASSIST_LIMITS.before);
  if (before) request.before = before;
  const after = firstChars(context.after, ASSIST_LIMITS.after);
  if (after) request.after = after;

  const title = firstChars(input.title.trim(), ASSIST_LIMITS.title);
  if (title) request.title = title;
  const outline = input.outline
    .map((item) => firstChars(item.trim(), ASSIST_LIMITS.outlineItemChars))
    .filter((item) => item !== "")
    .slice(0, ASSIST_LIMITS.outlineItems);
  if (outline.length > 0) request.outline = outline;

  const instruction = (input.instruction ?? "").trim();
  if (action === "custom" && instruction) request.instruction = instruction;
  if (action === "translate" && input.targetLang) request.target_lang = input.targetLang;
  return request;
}

export interface AssistValidation {
  action: AssistAction;
  /** 选中文字的字符数；没有选区为 0。 */
  selectionChars: number;
  /** 当前能不能取到编辑面（例如 Markdown 仅预览时不能）。 */
  editable: boolean;
  instruction: string;
}

/** 返回不能执行的原因；可以执行时返回空串。 */
export function assistBlockedReason(input: AssistValidation): string {
  if (!input.editable) return "请先切换到编辑模式";
  const spec = ASSIST_ACTIONS.find((item) => item.id === input.action);
  if (input.action === "custom") {
    const chars = countChars(input.instruction.trim());
    if (chars === 0) return "请输入要让 AI 做什么";
    if (chars > ASSIST_LIMITS.instruction) return `指令不能超过 ${ASSIST_LIMITS.instruction} 字`;
  }
  if (spec?.needsSelection && input.selectionChars === 0) return "先选中要处理的文字";
  // 续写不用选区，再长也不受限。
  if (input.action !== "continue" && input.selectionChars > ASSIST_LIMITS.selection) {
    return `选区超过 ${ASSIST_LIMITS.selection} 字，请缩小范围`;
  }
  return "";
}

// ---------- 结果处理 ----------

/** 统一换行并去掉首尾空行，写回编辑器前使用。 */
export function normalizeResult(text: string): string {
  return text.replace(/\r\n?/g, "\n").replace(/^\n+/, "").replace(/\s+$/, "");
}

/** 能逐词对比原文的结果才提供「对比」；翻译与插入类没有可比对象。 */
export function diffApplicable(action: AssistAction, placement: AssistantPlacement): boolean {
  return placement === "replace" && action !== "translate";
}

/** 完成后默认看哪个视图：改动不大时对比一目了然，改动很大时直接看结果。 */
export function preferredView(similarity: number | null): AssistView {
  return similarity !== null && similarity >= 0.35 ? "diff" : "result";
}

// ---------- 标签（{% … %}）校验 ----------

const SHORTCODE_TAG = /\{%[\s\S]*?%\}/g;

export function shortcodeTags(text: string): string[] {
  return (text.match(SHORTCODE_TAG) ?? []).map((tag) => tag.replace(/\s+/g, " ").trim());
}

/** 结果里的标签与原文不一致（增减或被改写）。 */
export function shortcodesChanged(original: string, result: string): boolean {
  const a = shortcodeTags(original).sort();
  const b = shortcodeTags(result).sort();
  return a.length !== b.length || a.some((tag, i) => tag !== b[i]);
}

// ---------- Markdown 块边界与定位（<textarea> 编辑器用） ----------

interface Line {
  start: number;
  end: number;
  text: string;
}

function splitLines(markdown: string): Line[] {
  const lines: Line[] = [];
  let start = 0;
  for (;;) {
    const nl = markdown.indexOf("\n", start);
    const end = nl < 0 ? markdown.length : nl;
    lines.push({ start, end, text: markdown.slice(start, end).replace(/\r$/, "") });
    if (nl < 0) break;
    start = nl + 1;
  }
  return lines;
}

/** 处理完一行之后，是否还在围栏代码块内。 */
function fenceAfter(line: string, fence: { ch: string; len: number } | null): { ch: string; len: number } | null {
  const f = FENCE_LINE.exec(line);
  if (fence) {
    return f && f[1][0] === fence.ch && f[1].length >= fence.len && f[2].trim() === "" ? null : fence;
  }
  return f ? { ch: f[1][0], len: f[1].length } : null;
}

/**
 * pos 所在块（或紧邻其前的块）的末尾偏移。块以空行分隔，围栏代码块里的空行不算；
 * 光标落在空行上时，取它上方最近的块。文档里没有任何内容时返回 0。
 */
export function blockEnd(markdown: string, pos: number): number {
  const lines = splitLines(markdown);
  const at = Math.max(0, Math.min(pos, markdown.length));
  let current = lines.findIndex((line) => at >= line.start && at <= line.end);
  if (current < 0) current = lines.length - 1;

  // 逐行推进围栏状态，记录每行开始时所处的围栏（null 表示不在围栏内）。
  const starts: Array<{ ch: string; len: number } | null> = [];
  let fence: { ch: string; len: number } | null = null;
  for (const line of lines) {
    starts.push(fence);
    fence = fenceAfter(line.text, fence);
  }

  let anchor = current;
  while (anchor >= 0 && lines[anchor].text.trim() === "" && starts[anchor] === null) anchor--;
  if (anchor < 0) return 0;

  let state = starts[anchor];
  for (let i = anchor; i < lines.length; i++) {
    state = fenceAfter(lines[i].text, state);
    const next = lines[i + 1];
    if (state === null && (next === undefined || next.text.trim() === "")) return lines[i].end;
  }
  return markdown.length;
}

/**
 * 编辑后在新文本里重新定位原来的片段：位置没动就原样返回；
 * 否则只在片段在全文中唯一出现时才认定位置（出现多处就无法确定是哪一处）。
 */
export function relocate(value: string, text: string, hint: number): number {
  if (text === "") return -1;
  if (value.startsWith(text, hint)) return hint;
  const first = value.indexOf(text);
  if (first < 0) return -1;
  return value.indexOf(text, first + 1) < 0 ? first : -1;
}

/** 同上，但定位的是「某段前文之后的位置」：返回该前文结束处的偏移。 */
export function relocateAfter(value: string, tail: string, hint: number): number {
  if (tail === "") return hint === 0 ? 0 : -1;
  if (hint >= tail.length && value.slice(hint - tail.length, hint) === tail) return hint;
  const first = value.indexOf(tail);
  if (first < 0) return -1;
  return value.indexOf(tail, first + 1) < 0 ? first + tail.length : -1;
}

// ---------- 快捷键 ----------

export interface HotkeyEvent {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  repeat?: boolean;
  isComposing?: boolean;
}

/** Ctrl/⌘+J：与 DefaultLayout 的 Mod+K 同样的修饰键判断；输入法组字中与长按重复不触发。 */
export function isAssistantHotkey(event: HotkeyEvent, isMac: boolean): boolean {
  if (event.isComposing || event.repeat) return false;
  if (event.shiftKey || event.altKey) return false;
  if (event.key.toLowerCase() !== "j") return false;
  return (event.ctrlKey && !isMac) || event.metaKey;
}
