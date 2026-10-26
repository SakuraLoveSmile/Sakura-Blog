// 成对块级标签（{% tag %}…{% endtag %}），与 internal/post/shortcode.go 对齐
// 只决定语法类型；嵌套按原文边界扫描，顺序不影响外层 raw。
const PAIRED_TAGS = [
  "dropdown",
  "blog_about",
  "blog_tip",
  "banner",
  "navbar",
  "folding",
  "tabs",
  "timeline",
  "gallery",
  "mermaid",
  "okr",
  "folders",
  "grid",
  "box",
  "table",
  "tip",
  "poetry",
  "swiper",
  "chat",
  "frame",
  "about",
  "blockquote",
  "reel",
  "paper",
];

// 单行标签（{% tag ... %}），与 shortcode.go 对齐
// navbar/frame/tip 同时保留既有块级和单行形式。
const SINGLE_TAGS = [
  "gist",
  "tip",
  "blog_links",
  "blog_moments",
  "blog_ghrepo",
  "blog_ghuser",
  "note",
  "button",
  "checkbox",
  "radio",
  "quot",
  "link",
  "image",
  "mark",
  "emoji",
  "icon",
  "hashtag",
  "ghrepo",
  "ghuser",
  "video",
  "audio",
  "copy",
  "rating",
  "mbti",
  "ghcard",
  "vote",
  "friends",
  "links",
  "moments",
  "sites",
  "u",
  "emp",
  "wavy",
  "del",
  "kbd",
  "psw",
  "blur",
  "sup",
  "sub",
  "md",
  "toc",
  "albums",
  "posters",
  "navbar",
  "frame",
];

// base64 编解码（支持 Unicode），规避 HTML 属性中的引号/换行/空行问题
export function encodeRaw(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let bin = "";
  bytes.forEach((b) => (bin += String.fromCharCode(b)));
  return btoa(bin);
}

export function decodeRaw(b64: string): string {
  try {
    const bin = atob(b64);
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  } catch {
    return "";
  }
}

/** 从 raw 提取标签名（徽标显示用） */
export function tagNameOf(raw: string): string {
  return /\{%\s*([a-zA-Z][a-zA-Z0-9_-]*)/.exec(raw)?.[1] || "tag";
}

const placeholder = (raw: string) => `<div data-stellar-tag="${encodeRaw(raw)}"></div>`;

type SourceRange = { start: number; end: number };
type TagToken = SourceRange & { name: string; closing: boolean; incomplete?: boolean };

const blockTags = new Set(PAIRED_TAGS.filter((name) => !SINGLE_TAGS.includes(name)));
const singleTags = new Set(SINGLE_TAGS);
const knownTags = new Set([...PAIRED_TAGS, ...SINGLE_TAGS]);

function mergeRanges(ranges: SourceRange[]): SourceRange[] {
  const sorted = ranges.sort((a, b) => a.start - b.start || a.end - b.end);
  const merged: SourceRange[] = [];
  for (const range of sorted) {
    const previous = merged[merged.length - 1];
    if (previous && range.start <= previous.end) previous.end = Math.max(previous.end, range.end);
    else merged.push({ ...range });
  }
  return merged;
}

// 移除代码围栏的引用/列表容器前缀；节点原文仍从未修改的 src 截取。
function fenceLine(line: string): string {
  let rest = line;
  for (;;) {
    const prefix = /^(?: {0,3}>[ \t]?| {0,3}(?:[-+*]|\d{1,9}[.)])[ \t]+)/.exec(rest);
    if (!prefix) return rest;
    rest = rest.slice(prefix[0].length);
  }
}

function protectedRanges(src: string): SourceRange[] {
  const blocks: SourceRange[] = [];
  let fence: { character: string; length: number } | undefined;
  let offset = 0;
  for (const line of src.split(/(?<=\n)/)) {
    const text = line.replace(/[\r\n]+$/, "");
    const candidate = fenceLine(text);
    if (fence) {
      blocks.push({ start: offset, end: offset + line.length });
      const close = /^ {0,3}(`+|~+)[ \t]*$/.exec(candidate);
      if (close && close[1][0] === fence.character && close[1].length >= fence.length) fence = undefined;
    } else {
      const open = /^ {0,3}(`{3,}|~{3,})(.*)$/.exec(candidate);
      // 反引号围栏的信息字符串不能再含反引号。
      if (open && (open[1][0] !== "`" || !open[2].includes("`"))) {
        fence = { character: open[1][0], length: open[1].length };
        blocks.push({ start: offset, end: offset + line.length });
      } else if (/^(?: {4}|\t)/.test(text)) {
        blocks.push({ start: offset, end: offset + line.length });
      }
    }
    offset += line.length;
  }
  // HTML 代码区和注释中的示例也不能变成富文本原子节点。
  for (const match of src.matchAll(/<!--[\s\S]*?(?:-->|$)|<(pre|code|script|style)\b[^>]*>[\s\S]*?(?:<\/\1\s*>|$)/gi)) {
    blocks.push({ start: match.index, end: match.index + match[0].length });
  }
  const ranges = mergeRanges(blocks);
  const inline: SourceRange[] = [];
  let rangeIndex = 0;
  for (let i = 0; i < src.length;) {
    while (ranges[rangeIndex] && ranges[rangeIndex].end <= i) rangeIndex++;
    const block = ranges[rangeIndex];
    if (block && block.start <= i) {
      i = block.end;
      continue;
    }
    let backslashes = 0;
    if (src[i] === "`") {
      for (let previous = i - 1; previous >= 0 && src[previous] === "\\"; previous--) backslashes++;
    }
    if (src[i] !== "`" || backslashes % 2 === 1) {
      i++;
      continue;
    }
    const start = i;
    while (src[i] === "`") i++;
    const length = i - start;
    let end = i;
    let closed = false;
    while (end < src.length && (!block || end < block.start)) {
      // 代码跨度不能跨过段落边界。
      if (src[end] === "\n" || src[end] === "\r") {
        let next = end + (src[end] === "\r" && src[end + 1] === "\n" ? 2 : 1);
        while (src[next] === " " || src[next] === "\t") next++;
        if (src[next] === "\n" || src[next] === "\r") break;
      }
      if (src[end] !== "`") {
        end++;
        continue;
      }
      const run = end;
      while (src[end] === "`") end++;
      if (end - run === length) {
        inline.push({ start, end });
        i = end;
        closed = true;
        break;
      }
    }
    if (!closed) i = start + length;
  }
  return mergeRanges([...ranges, ...inline]);
}

function readTag(src: string, start: number): TagToken | undefined {
  const nameMatch = /^\{%\s*([a-zA-Z][a-zA-Z0-9_-]*)\b/.exec(src.slice(start));
  if (!nameMatch) return;
  let quote = "";
  for (let end = start + nameMatch[0].length; end < src.length; end++) {
    const character = src[end];
    if (quote) {
      if (character === "\\") end++;
      else if (character === quote) quote = "";
    } else if (character === '"' || character === "'") quote = character;
    else if (character === "%" && src[end + 1] === "}") {
      const closing = nameMatch[1].startsWith("end");
      return { start, end: end + 2, name: closing ? nameMatch[1].slice(3) : nameMatch[1], closing };
    }
  }
  const closing = nameMatch[1].startsWith("end");
  return { start, end: src.length, name: closing ? nameMatch[1].slice(3) : nameMatch[1], closing, incomplete: true };
}

function lineRange(src: string, token: SourceRange): SourceRange {
  const start = src.lastIndexOf("\n", token.start - 1) + 1;
  const newline = src.indexOf("\n", token.end);
  let end = newline < 0 ? src.length : newline;
  if (src[end - 1] === "\r") end--;
  return { start, end };
}

function standaloneRange(src: string, token: SourceRange): SourceRange | undefined {
  const range = lineRange(src, token);
  if (!/^[ \t]*$/.test(src.slice(range.start, token.start)) || !/^[ \t]*$/.test(src.slice(token.end, range.end)))
    return;
  return range;
}

/** 正常 Markdown 中的外层标签以原文节点保存；未知和残缺结构沿用源码编辑回退。 */
export function wrapShortcodes(src: string): string {
  const protectedCode = protectedRanges(src);
  const tokens: TagToken[] = [];
  let rangeIndex = 0;
  for (let cursor = 0; cursor < src.length;) {
    const start = src.indexOf("{%", cursor);
    if (start < 0) break;
    while (protectedCode[rangeIndex] && protectedCode[rangeIndex].end <= start) rangeIndex++;
    const code = protectedCode[rangeIndex];
    if (code && code.start <= start) {
      cursor = code.end;
      continue;
    }
    const token = readTag(src, start);
    if (token) tokens.push(token);
    cursor = token?.end ?? start + 2;
  }

  // 同名结束符只归属最近的开符。未获得结束符的 hybrid 保持单行，
  // 避免后面独立的旧块级用法污染前面外层块中的合法单行标签。
  const matchedOpenings = new Set<TagToken>();
  const pendingOpenings = new Map<string, TagToken[]>();
  for (const token of tokens) {
    const pending = pendingOpenings.get(token.name) ?? [];
    if (token.closing) {
      const opening = pending.pop();
      if (opening) matchedOpenings.add(opening);
    } else pending.push(token);
    pendingOpenings.set(token.name, pending);
  }
  const stack: TagToken[] = [];
  const blocks: SourceRange[] = [];
  for (const token of tokens) {
    if (token.incomplete) {
      blocks.push({ start: token.start, end: src.length });
    } else if (token.closing) {
      const opening = stack[stack.length - 1];
      // 交错结束符不猜测修复；保留未闭合外层，随后将整段源码保存在一个节点中。
      if (!opening || opening.name !== token.name) continue;
      stack.pop();
      blocks.push({ start: opening.start, end: token.end });
    } else if (blockTags.has(token.name) || !knownTags.has(token.name) || matchedOpenings.has(token)) {
      // 未知标签无法推断单行/块级边界；未闭合时保留从开符到文末的完整源码。
      stack.push(token);
    }
  }
  for (const token of stack) blocks.push({ start: token.start, end: src.length });

  // 先确定完整外层区间，再替换；内层从未经过占位转换，raw 始终直接截取原文。
  blocks.sort((a, b) => a.start - b.start || b.end - a.end);
  const outerBlocks: typeof blocks = [];
  for (const block of blocks) {
    const outer = outerBlocks[outerBlocks.length - 1];
    if (!outer || block.start >= outer.end) outerBlocks.push(block);
  }
  const replacements: SourceRange[] = [];
  let blockIndex = 0;
  for (const token of tokens) {
    while (outerBlocks[blockIndex] && outerBlocks[blockIndex].end <= token.start) blockIndex++;
    const block = outerBlocks[blockIndex];
    if (block && block.start <= token.start) {
      if (token.start === block.start) replacements.push(lineRange(src, block));
      continue;
    }
    if (!token.closing && singleTags.has(token.name) && !src.slice(token.start, token.end).includes("\n")) {
      const range = standaloneRange(src, token);
      if (range) replacements.push(range);
    }
  }

  let result = "";
  let cursor = 0;
  for (const range of mergeRanges(replacements)) {
    result += src.slice(cursor, range.start) + placeholder(src.slice(range.start, range.end));
    cursor = range.end;
  }
  return result + src.slice(cursor);
}
