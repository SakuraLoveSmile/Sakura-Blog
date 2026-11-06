// 助手结果与原文的逐词行内差异。纯函数，不依赖 DOM。
// 先分词（中文按词、英文按单词），再用 Myers 算法求最短编辑脚本；
// 输入过大或差异过多时返回 null，由界面退回只看结果。

export type DiffOp = "equal" | "insert" | "delete";

export interface DiffSegment {
  type: DiffOp;
  text: string;
}

export interface InlineDiff {
  segments: DiffSegment[];
  /** 保持不变的字符占较长一方的比例，0 到 1。 */
  similarity: number;
  /** 新增与删除的字符数。 */
  added: number;
  removed: number;
}

/** 去掉公共前后缀后，两边词数之和的上限。 */
export const DIFF_MAX_TOKENS = 20000;
/** Myers 搜索的编辑距离上限，超过就放弃对比。 */
export const DIFF_MAX_EDIT = 1500;

interface WordSegmenter {
  segment(input: string): Iterable<{ segment: string }>;
}
type WordSegmenterCtor = new (locale?: string, options?: { granularity: "word" }) => WordSegmenter;

// 汉字逐字、英文数字成词、空白成段，其余字符（含成对代理项）各自成词。
const FALLBACK_TOKEN = /[㐀-䶿一-鿿]|[A-Za-z0-9_À-ɏ]+|\s+|[\ud800-\udbff][\udc00-\udfff]|[^]/g;

/** 不依赖 Intl.Segmenter 的分词，旧浏览器使用。 */
export function tokenizeFallback(text: string): string[] {
  return text.match(FALLBACK_TOKEN) ?? [];
}

let segmenter: WordSegmenter | null | undefined;

function getSegmenter(): WordSegmenter | null {
  if (segmenter === undefined) {
    try {
      const Ctor = (Intl as unknown as { Segmenter?: WordSegmenterCtor }).Segmenter;
      segmenter = Ctor ? new Ctor("zh", { granularity: "word" }) : null;
    } catch {
      segmenter = null;
    }
  }
  return segmenter;
}

/** 把文本切成词；拼接所有词必然还原原文。 */
export function tokenize(text: string): string[] {
  const seg = getSegmenter();
  if (!seg) return tokenizeFallback(text);
  const out: string[] = [];
  for (const part of seg.segment(text)) out.push(part.segment);
  return out;
}

interface Edit {
  type: DiffOp;
  token: string;
}

/** Myers O(ND) 最短编辑脚本；编辑距离超过 maxEdit 时返回 null。 */
function myers(a: string[], b: string[], maxEdit: number): Edit[] | null {
  const n = a.length;
  const m = b.length;
  const max = Math.min(n + m, maxEdit);
  const offset = max + 1;
  const v = new Int32Array(2 * max + 3);
  const trace: Int32Array[] = [];
  let found = -1;

  for (let d = 0; d <= max && found < 0; d++) {
    trace.push(v.slice());
    for (let k = -d; k <= d; k += 2) {
      let x =
        k === -d || (k !== d && v[k - 1 + offset] < v[k + 1 + offset]) ? v[k + 1 + offset] : v[k - 1 + offset] + 1;
      let y = x - k;
      while (x < n && y < m && a[x] === b[y]) {
        x++;
        y++;
      }
      v[k + offset] = x;
      if (x >= n && y >= m) {
        found = d;
        break;
      }
    }
  }
  if (found < 0) return null;

  const edits: Edit[] = [];
  let x = n;
  let y = m;
  for (let d = found; d >= 0; d--) {
    const prev = trace[d];
    const k = x - y;
    const prevK = k === -d || (k !== d && prev[k - 1 + offset] < prev[k + 1 + offset]) ? k + 1 : k - 1;
    const prevX = prev[prevK + offset];
    const prevY = prevX - prevK;
    while (x > prevX && y > prevY) {
      x--;
      y--;
      edits.push({ type: "equal", token: a[x] });
    }
    if (d > 0) {
      if (x === prevX) {
        y--;
        edits.push({ type: "insert", token: b[y] });
      } else {
        x--;
        edits.push({ type: "delete", token: a[x] });
      }
    }
  }
  return edits.reverse();
}

function mergeEdits(edits: Edit[]): DiffSegment[] {
  const segments: DiffSegment[] = [];
  for (const edit of edits) {
    const last = segments[segments.length - 1];
    if (last && last.type === edit.type) last.text += edit.token;
    else segments.push({ type: edit.type, text: edit.token });
  }
  return segments;
}

/** 夹在两处修改之间的短小不变片段（一两个字、空白、标点）并入修改，避免满屏零碎高亮。 */
function isTrivial(text: string): boolean {
  return text.length <= 2 || /^[\s\p{P}\p{S}]+$/u.test(text);
}

function tidy(segments: DiffSegment[]): DiffSegment[] {
  const out: DiffSegment[] = [];
  let deleted = "";
  let inserted = "";
  let open = false;
  const flush = () => {
    if (deleted) out.push({ type: "delete", text: deleted });
    if (inserted) out.push({ type: "insert", text: inserted });
    deleted = "";
    inserted = "";
    open = false;
  };
  for (let i = 0; i < segments.length; i++) {
    const seg = segments[i];
    if (seg.type === "delete") {
      deleted += seg.text;
      open = true;
    } else if (seg.type === "insert") {
      inserted += seg.text;
      open = true;
    } else {
      const next = segments[i + 1];
      if (open && next && next.type !== "equal" && isTrivial(seg.text)) {
        deleted += seg.text;
        inserted += seg.text;
      } else {
        flush();
        out.push(seg);
      }
    }
  }
  flush();
  return out;
}

/** 逐词对比；输入过大或改动过多时返回 null。 */
export function buildInlineDiff(before: string, after: string): InlineDiff | null {
  if (before === after) {
    return { segments: before ? [{ type: "equal", text: before }] : [], similarity: 1, added: 0, removed: 0 };
  }
  const a = tokenize(before);
  const b = tokenize(after);

  let start = 0;
  while (start < a.length && start < b.length && a[start] === b[start]) start++;
  let endA = a.length;
  let endB = b.length;
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA--;
    endB--;
  }
  const middleA = a.slice(start, endA);
  const middleB = b.slice(start, endB);
  if (middleA.length + middleB.length > DIFF_MAX_TOKENS) return null;
  const middle = myers(middleA, middleB, DIFF_MAX_EDIT);
  if (!middle) return null;

  const edits: Edit[] = [
    ...a.slice(0, start).map((token): Edit => ({ type: "equal", token })),
    ...middle,
    ...a.slice(endA).map((token): Edit => ({ type: "equal", token })),
  ];
  const raw = mergeEdits(edits);
  let equal = 0;
  let added = 0;
  let removed = 0;
  for (const seg of raw) {
    if (seg.type === "equal") equal += seg.text.length;
    else if (seg.type === "insert") added += seg.text.length;
    else removed += seg.text.length;
  }
  const longest = Math.max(before.length, after.length, 1);
  return { segments: tidy(raw), similarity: Math.min(1, equal / longest), added, removed };
}
