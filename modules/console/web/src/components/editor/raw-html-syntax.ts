import { encodeRaw } from "./stellar-tag-syntax";

// 外部 HTML 的富文本保真语法层（与 stellar-tag-syntax 同构，纯字符串、无 DOM，可在 node:test 跑）。
// 原理：markdown-it core rule 在 block 解析后，把 TipTap schema 无法表示的 html_block
// 替换为 <div data-raw-html="BASE64(raw)"></div> 占位（html:true 下按 html_block 透传），
// TipTap parseHTML 解析为 atom 节点；序列化原样回写 raw，round-trip 无损。
// 边界取自 CommonMark 的 html_block，与后端 goldmark 的发布语义一致。

export const RAW_HTML_ATTR = "data-raw-html";
export const RAW_HTML_INLINE_ATTR = "data-raw-html-inline";

/** TipTap schema 原生可表示的标签（StarterKit + Link/Image/Table/TaskList/CodeBlockWithLang + 自建 mark/sub/sup）。 */
const NATIVE_TAGS = new Set([
  "p", "h1", "h2", "h3", "h4", "h5", "h6",
  "ul", "ol", "li", "blockquote", "pre", "code", "hr", "br",
  "em", "i", "strong", "b", "del", "s", "strike", "a", "img",
  "mark", "sub", "sup",
  "table", "thead", "tbody", "tfoot", "tr", "td", "th",
  "input",
]);

/** 各标签上 TipTap 实际会读回的属性；未列出的标签不允许任何属性。 */
const TAG_ATTRS: Record<string, ReadonlySet<string>> = {
  a: new Set(["href", "title", "target", "rel"]),
  img: new Set(["src", "alt", "title"]),
  td: new Set(["colspan", "rowspan", "colwidth"]),
  th: new Set(["colspan", "rowspan", "colwidth"]),
  ol: new Set(["start"]),
  code: new Set(["class"]),
  pre: new Set(["class"]),
  input: new Set(["type", "checked", "disabled", "class"]),
};

/** HTML 原始文本元素：内容不是标记，扫描时整段跳过到匹配闭合标签。 */
const RAW_TEXT_ELEMENTS = new Set([
  "script", "style", "textarea", "title", "xmp", "iframe", "noembed", "noframes", "plaintext",
]);

/** void 元素没有闭合标签，不参与配平。 */
const VOID_ELEMENTS = new Set([
  "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr",
]);

export interface HtmlAttribute {
  name: string;
  value: string;
}

export interface HtmlTag {
  name: string;
  attrs: HtmlAttribute[];
  closing: boolean;
  selfClosing: boolean;
  start: number;
  end: number;
}

const WHITESPACE = /\s/;

/** 纯字符串扫描标签与属性；跳过注释、doctype、处理指令与原始文本元素内容。 */
export function scanHtmlTags(raw: string): HtmlTag[] {
  const tags: HtmlTag[] = [];
  let cursor = 0;
  while (cursor < raw.length) {
    const open = raw.indexOf("<", cursor);
    if (open < 0) break;
    if (raw.startsWith("<!--", open)) {
      const close = raw.indexOf("-->", open + 4);
      cursor = close < 0 ? raw.length : close + 3;
      continue;
    }
    if (raw.startsWith("<!", open) || raw.startsWith("<?", open)) {
      const close = raw.indexOf(">", open + 2);
      cursor = close < 0 ? raw.length : close + 1;
      continue;
    }
    const head = /^<(\/?)([a-zA-Z][a-zA-Z0-9:-]*)/.exec(raw.slice(open));
    if (!head) {
      cursor = open + 1;
      continue;
    }
    const closing = head[1] === "/";
    const name = head[2].toLowerCase();
    let position = open + head[0].length;
    const attrs: HtmlAttribute[] = [];
    let selfClosing = false;
    while (position < raw.length) {
      while (position < raw.length && WHITESPACE.test(raw[position])) position++;
      if (position >= raw.length) break;
      if (raw[position] === ">") {
        position++;
        break;
      }
      if (raw[position] === "/" && raw[position + 1] === ">") {
        selfClosing = true;
        position += 2;
        break;
      }
      const attrName = /^[^\s"'>/=]+/.exec(raw.slice(position));
      if (!attrName) {
        position++;
        continue;
      }
      position += attrName[0].length;
      while (position < raw.length && WHITESPACE.test(raw[position])) position++;
      let value = "";
      if (raw[position] === "=") {
        position++;
        while (position < raw.length && WHITESPACE.test(raw[position])) position++;
        const quote = raw[position];
        if (quote === '"' || quote === "'") {
          const end = raw.indexOf(quote, position + 1);
          if (end < 0) {
            value = raw.slice(position + 1);
            position = raw.length;
          } else {
            value = raw.slice(position + 1, end);
            position = end + 1;
          }
        } else {
          const unquoted = /^[^\s>]*/.exec(raw.slice(position));
          value = unquoted ? unquoted[0] : "";
          position += value.length;
        }
      }
      attrs.push({ name: attrName[0].toLowerCase(), value });
    }
    tags.push({ name, attrs, closing, selfClosing, start: open, end: position });
    cursor = position;
    if (!closing && !selfClosing && RAW_TEXT_ELEMENTS.has(name)) {
      const rest = raw.slice(position);
      const close = new RegExp(`</${name}\\s*>`, "i").exec(rest);
      if (!close) {
        cursor = raw.length;
        continue;
      }
      const closeStart = position + close.index;
      const closeEnd = closeStart + close[0].length;
      tags.push({ name, attrs: [], closing: true, selfClosing: false, start: closeStart, end: closeEnd });
      cursor = closeEnd;
    }
  }
  return tags;
}

function attrPreserved(tag: HtmlTag, attr: HtmlAttribute): boolean {
  const allowed = TAG_ATTRS[tag.name];
  if (!allowed || !allowed.has(attr.name)) return false;
  // CodeBlockWithLang 只读回 language-* class；其他 class 会丢，必须走原子节点。
  if ((tag.name === "code" || tag.name === "pre") && attr.name === "class") {
    return attr.value.split(/\s+/).filter(Boolean).every((word) => /^language-[\w+-]+$/.test(word));
  }
  // TaskItem 只解析 checkbox；其他 input 类型不在 schema 内。
  if (tag.name === "input" && attr.name === "type") return attr.value.toLowerCase() === "checkbox";
  // TaskItem 自己渲染出的 class 必须放行，否则任务列表往返会退化成原始 HTML。
  if (tag.name === "input" && attr.name === "class") {
    return attr.value.split(/\s+/).filter(Boolean).every((word) => word === "task-list-item-checkbox");
  }
  return true;
}

/**
 * 判断一段 HTML 是否超出 TipTap schema 的表示能力。
 * true → 必须原子化保真；false → 沿用既有原生可编辑路径。
 * 无标签（纯文本、仅注释）返回 false：注释 marker 归 Stellar 机制处理，不在此接管。
 */
export function isForeignHtml(raw: string): boolean {
  const tags = scanHtmlTags(raw);
  if (tags.length === 0) return false;
  for (const tag of tags) {
    if (!NATIVE_TAGS.has(tag.name)) return true;
    for (const attr of tag.attrs) {
      if (!attrPreserved(tag, attr)) return true;
    }
  }
  return false;
}

export function rawHtmlPlaceholder(raw: string): string {
  return `<div ${RAW_HTML_ATTR}="${encodeRaw(raw)}"></div>`;
}

export function rawHtmlInlinePlaceholder(raw: string): string {
  return `<span ${RAW_HTML_INLINE_ATTR}="${encodeRaw(raw)}"></span>`;
}

/** 占位与 Stellar 标签占位都不能被再次接管：保证幂等，混合块整体回退源码。 */
function alreadyPlaceholder(raw: string): boolean {
  return raw.includes(RAW_HTML_ATTR) || raw.includes(RAW_HTML_INLINE_ATTR) || raw.includes("data-stellar-tag");
}

function lineStarts(src: string): number[] {
  const starts = [0];
  for (let i = 0; i < src.length; i++) {
    if (src[i] === "\n") starts.push(i + 1);
  }
  return starts;
}

/** 未闭合标签深度：用于把被空行切断的同一个 HTML 元素重新并回一个节点。 */
function tagBalance(raw: string): number {
  let depth = 0;
  for (const tag of scanHtmlTags(raw)) {
    if (tag.closing) depth--;
    else if (!tag.selfClosing && !VOID_ELEMENTS.has(tag.name)) depth++;
  }
  return depth;
}

interface MarkdownItLike {
  core: { ruler: { after: (after: string, name: string, rule: (state: any) => void) => void } };
}

// tiptap-markdown 会对同一个 markdown-it 实例多次调用 parse.setup；重复注册会让第二遍
// 按 token.map 重新切片，把已并入占位的块再原子化一次，导致内容重复。按实例去重。
const blockRuleHosts = new WeakSet<object>();
const inlineRuleHosts = new WeakSet<object>();

/**
 * 注册 token 层 core rule。挂在 block 之后，因此晚于 stellar_tag（before normalize）执行，
 * 两者顺序天然兼容；raw 按 token.map 从 state.src 切片，保住缩进、空行、大小写与属性顺序。
 *
 * CommonMark 会在空行处切断 HTML 块，也会把 doctype 单独成块。因此先把相邻 html_block
 * 归组：行号连续的始终合并（doctype + html）；仅隔空行的只在标签未配平时合并
 * （被切断的同一个元素），已配平的块各自独立，干净语义 HTML 不被邻近外部块牵连。
 */
export function registerRawHtmlRule(md: MarkdownItLike): void {
  if (blockRuleHosts.has(md)) return;
  blockRuleHosts.add(md);
  md.core.ruler.after("block", "raw_html_block", (state: any) => {
    const starts = lineStarts(state.src);
    const lineEnd = (line: number) => (line < starts.length ? starts[line] : state.src.length);
    const sliceLines = (first: number, last: number) => state.src.slice(starts[first], lineEnd(last));

    let group: { first: number; last: number; balance: number; tokens: any[] } | null = null;

    const flush = () => {
      if (!group) return;
      const { first, last, tokens } = group;
      group = null;
      // 去掉块末的行终止符：closeBlock 会重新补分隔，保留会让空行逐轮累积。
      const raw = sliceLines(first, last).replace(/\n$/, "");
      if (!raw || alreadyPlaceholder(raw) || !isForeignHtml(raw)) return;
      tokens[0].content = rawHtmlPlaceholder(raw);
      tokens[0].map = [first, last];
      // html_block 渲染规则直接返回 content 且不看 hidden，只能清空内容来抑制被并入的块。
      for (const token of tokens.slice(1)) token.content = "";
    };

    for (const token of state.tokens) {
      // content 已是占位或已被并入其他占位时不再接管：即使规则被重复注册也保持幂等。
      if (token.type !== "html_block" || !token.map || !token.content || alreadyPlaceholder(token.content)) {
        flush();
        continue;
      }
      const [first, last] = token.map as [number, number];
      if (first >= starts.length) {
        flush();
        continue;
      }
      if (group) {
        const contiguous = first === group.last;
        const blankGap = first > group.last && /^[ \t\n]*$/.test(sliceLines(group.last, first));
        if (contiguous || (blankGap && group.balance > 0)) {
          group.last = last;
          group.balance += tagBalance(sliceLines(first, last));
          group.tokens.push(token);
          continue;
        }
        flush();
      }
      group = { first, last, balance: tagBalance(sliceLines(first, last)), tokens: [token] };
    }
    flush();
  });
}

/**
 * 行内 HTML 保真。只接管「纯 HTML 连续段」：从 foreign 的 html_inline 起，按标签配平
 * 吸收后续 html_inline / text，整段换成 inline 占位。段内一旦出现 markdown 格式 token
 * （strong_open 等）就放弃——它们的 content 为空，重建会丢掉原文的 ** 等标记。
 */
export function registerRawHtmlInlineRule(md: MarkdownItLike): void {
  if (inlineRuleHosts.has(md)) return;
  inlineRuleHosts.add(md);
  md.core.ruler.after("inline", "raw_html_inline", (state: any) => {
    for (const token of state.tokens) {
      if (token.type !== "inline" || !token.children) continue;
      const children = token.children as any[];
      const out: any[] = [];
      let index = 0;
      // 放弃接管的段仍可能横跨后续 token：其余配平部分必须一并原样保留，
      // 否则孤立的闭合标签会被单独芯片化，留下不配平的输出。
      let skipBalance = 0;
      while (index < children.length) {
        const child = children[index];
        if (child.type !== "html_inline") {
          out.push(child);
          index++;
          continue;
        }
        if (skipBalance > 0) {
          skipBalance = Math.max(0, skipBalance + tagBalance(child.content));
          out.push(child);
          index++;
          continue;
        }
        if (alreadyPlaceholder(child.content) || !isForeignHtml(child.content)) {
          out.push(child);
          index++;
          continue;
        }
        let balance = tagBalance(child.content);
        let raw = child.content;
        let next = index + 1;
        let reconstructable = true;
        while (next < children.length && balance > 0) {
          const following = children[next];
          if (following.type !== "html_inline" && following.type !== "text") {
            reconstructable = false;
            break;
          }
          raw += following.content;
          if (following.type === "html_inline") balance += tagBalance(following.content);
          next++;
        }
        if (!reconstructable || alreadyPlaceholder(raw)) {
          for (let kept = index; kept < next; kept++) out.push(children[kept]);
          skipBalance = Math.max(0, balance);
          index = next;
          continue;
        }
        out.push({ ...child, content: rawHtmlInlinePlaceholder(raw) });
        index = next;
      }
      token.children = out;
    }
  });
}