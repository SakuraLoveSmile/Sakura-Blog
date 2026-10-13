// 只重写被编辑的 JSON 节点，未触碰的数字、参数袋和显式空值保留原文。
interface JSONNode {
  start: number;
  end: number;
  value: unknown;
  entries?: { key: string; start: number; node: JSONNode }[];
  items?: JSONNode[];
}
function document(raw: string): JSONNode {
  JSON.parse(raw); // 先校验完整文档，再记录 token 位置；不使用解析结果重编码原文。
  let i = 0;
  const whitespace = () => {
    while (/\s/.test(raw[i] ?? "") && i < raw.length) i++;
  };
  function stringEnd() {
    i++;
    while (i < raw.length) {
      if (raw[i++] === '"') break;
      if (raw[i - 1] === "\\") i++;
    }
  }
  function node(): JSONNode {
    whitespace();
    const start = i;
    let entries: JSONNode["entries"], items: JSONNode["items"];
    if (raw[i] === "{") {
      entries = [];
      i++;
      whitespace();
      while (raw[i] !== "}") {
        const keyStart = i;
        stringEnd();
        const key = JSON.parse(raw.slice(keyStart, i)) as string;
        whitespace();
        i++;
        entries.push({ key, start: keyStart, node: node() });
        whitespace();
        if (raw[i] !== ",") break;
        i++;
        whitespace();
      }
      i++;
    } else if (raw[i] === "[") {
      items = [];
      i++;
      whitespace();
      while (raw[i] !== "]") {
        items.push(node());
        whitespace();
        if (raw[i] !== ",") break;
        i++;
      }
      i++;
    } else if (raw[i] === '"') stringEnd();
    else {
      while (i < raw.length && !/[\s,}\]]/.test(raw[i])) i++;
    }
    return { start, end: i, value: JSON.parse(raw.slice(start, i)), entries, items };
  }
  const root = node();
  if (!root.entries) throw new Error("配置必须是 JSON 对象");
  return root;
}
export function readJSONField(raw: string, path: string): unknown {
  let current: JSONNode | undefined = document(raw);
  for (const key of path.split("."))
    current =
      current?.entries
        ?.slice()
        .reverse()
        .find((entry) => entry.key === key)?.node ?? current?.items?.[Number(key)];
  return current?.value;
}
export function readJSONFieldRaw(raw: string, path: string): string | undefined {
  let current: JSONNode | undefined = document(raw);
  for (const key of path.split("."))
    current =
      current?.entries
        ?.slice()
        .reverse()
        .find((entry) => entry.key === key)?.node ?? current?.items?.[Number(key)];
  return current ? raw.slice(current.start, current.end) : undefined;
}
function replace(raw: string, start: number, end: number, value: string): string {
  return raw.slice(0, start) + value + raw.slice(end);
}
// 新值来自控件；相同子值保持旧 token（包括 JS Number 无法精确表达的参数）。
function editedNode(raw: string, old: JSONNode, value: unknown): string {
  if (JSON.stringify(old.value) === JSON.stringify(value)) return raw.slice(old.start, old.end);
  if (old.entries && value !== null && typeof value === "object" && !Array.isArray(value)) {
    const object = value as Record<string, unknown>;
    const keys = new Set(old.entries.map((entry) => entry.key));
    const entries = old.entries
      .filter((entry) => Object.hasOwn(object, entry.key))
      .map((entry) => raw.slice(entry.start, entry.node.start) + editedNode(raw, entry.node, object[entry.key]));
    for (const key of Object.keys(object))
      if (!keys.has(key)) entries.push(JSON.stringify(key) + ":" + JSON.stringify(object[key]));
    return "{" + entries.join(",") + "}";
  }
  if (old.items && Array.isArray(value))
    return (
      "[" +
      value
        .map((item, index) =>
          (old.items!.find((node) => JSON.stringify(node.value) === JSON.stringify(item)) ?? old.items![index])
            ? editedNode(
                raw,
                old.items!.find((node) => JSON.stringify(node.value) === JSON.stringify(item)) ?? old.items![index],
                item,
              )
            : JSON.stringify(item),
        )
        .join(",") +
      "]"
    );
  return JSON.stringify(value);
}
export function patchJSONField(raw: string, path: string, value: unknown, rawValue?: string): string {
  if (rawValue !== undefined) JSON.parse(rawValue);
  const root = document(raw);
  const keys = path.split(".");
  function patch(current: JSONNode, depth: number): string {
    const key = keys[depth];
    if (current.items) {
      const index = Number(key),
        child = current.items[index];
      if (!Number.isInteger(index) || index < 0) throw new Error("无效数组索引");
      if (child) {
        const next = depth === keys.length - 1 ? (rawValue ?? editedNode(raw, child, value)) : patch(child, depth + 1);
        return replace(
          raw.slice(current.start, current.end),
          child.start - current.start,
          child.end - current.start,
          next,
        );
      }
      let next = rawValue ?? JSON.stringify(value);
      for (let j = keys.length - 1; j > depth; j--) next = "{" + JSON.stringify(keys[j]) + ":" + next + "}";
      const items = current.items.map((item) => raw.slice(item.start, item.end));
      while (items.length < index) items.push("null");
      items[index] = next;
      return "[" + items.join(",") + "]";
    }
    const entry = current.entries
      ?.slice()
      .reverse()
      .find((item) => item.key === key);
    if (entry && depth === keys.length - 1 && value === undefined && rawValue === undefined) {
      const entries = current
        .entries!.filter((item) => item !== entry)
        .map((item) => raw.slice(item.start, item.node.end));
      return "{" + entries.join(",") + "}";
    }
    if (entry) {
      const next =
        depth === keys.length - 1 ? (rawValue ?? editedNode(raw, entry.node, value)) : patch(entry.node, depth + 1);
      return replace(
        raw.slice(current.start, current.end),
        entry.node.start - current.start,
        entry.node.end - current.start,
        next,
      );
    }
    if (value === undefined && rawValue === undefined) return raw.slice(current.start, current.end);
    let next = rawValue ?? JSON.stringify(value);
    for (let j = keys.length - 1; j > depth; j--) next = "{" + JSON.stringify(keys[j]) + ":" + next + "}";
    if (!current.entries) return "{" + JSON.stringify(key) + ":" + next + "}";
    const end = current.end - 1;
    return raw.slice(current.start, end) + (current.entries.length ? "," : "") + JSON.stringify(key) + ":" + next + "}";
  }
  return raw.slice(0, root.start) + patch(root, 0) + raw.slice(root.end);
}
