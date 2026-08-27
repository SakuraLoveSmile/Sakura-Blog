import { Node } from "@tiptap/core";
import { VueNodeViewRenderer } from "@tiptap/vue-3";
import { runtimePlugins } from "@/modules/plugins/usePluginCatalog";
import StellarTagView from "./StellarTagView.vue";

// Stellar 标签插件的富文本可视化节点（Spec：Stellar 标签富文本可视化）。
// 原理：markdown-it core rule 把独占整行/整块的 {% ... %} 预替换为
// <div data-stellar-tag="BASE64(raw)"></div> 占位（html:true 下按 html_block 透传），
// TipTap parseHTML 将其解析为 atom 节点；NodeView 用 iframe 渲染前台真实效果；
// 序列化时原样写回 raw，round-trip 无损。

// 成对块级标签（{% tag %}…{% endtag %}），与 internal/post/shortcode.go 对齐
// 顺序敏感：banner body 可嵌 navbar，须先处理 banner 再处理 navbar，
// 否则 navbar 先被替换为占位 div 会破坏 banner 的 raw 内容
const PAIRED_TAGS = [
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
// navbar/frame 同时有块级和单行两种形式，块级在 PAIRED_TAGS 先处理，
// 未匹配块级的单行用法由此处 singleLineRe 兜底
const SINGLE_TAGS = [
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
  return /\{%\s*([a-zA-Z]+)/.exec(raw)?.[1] || "tag";
}

// 标签名 → 中文标题映射（来自插件库元数据），供气泡类型标签显示
/** 类型标签中文名（如 note → 「提示框 Note」）；未命中回退标签名 */
export function typeLabelOf(raw: string): string {
  const name = tagNameOf(raw);
  return runtimePlugins.value.find((plugin) => plugin.name === name)?.title || name;
}

const placeholder = (raw: string) => `<div data-stellar-tag="${encodeRaw(raw)}"></div>`;

// 成对块级：{% tag ... %} body {% endtag %}（非贪婪，跨行含空行；不支持同名嵌套，与后端一致）
const pairedRes = PAIRED_TAGS.map((tag) => new RegExp(`\\{%\\s*${tag}\\b[\\s\\S]*?\\{%\\s*end${tag}\\s*%\\}`, "g"));

// 单行：整行只有一个标签（行首尾锚定）；夹在句子中间的内联用法保留为纯文本，避免破坏段落流
const singleLineRe = new RegExp(`^[ \\t]*(\\{%\\s*(?:${SINGLE_TAGS.join("|")})\\b[^\\n]*?%\\})[ \\t]*$`, "gm");

/** 把 Markdown 源码中的 Stellar 标签替换为占位 div（markdown-it core rule 调用） */
export function wrapShortcodes(src: string): string {
  let out = src;
  for (const re of pairedRes) {
    out = out.replace(re, (m) => placeholder(m));
  }
  out = out.replace(singleLineRe, (_m, tag: string) => placeholder(tag));
  return out;
}

export const StellarTag = Node.create({
  name: "stellarTag",
  group: "block",
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      raw: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "div[data-stellar-tag]",
        getAttrs: (el) => ({
          raw: decodeRaw((el as HTMLElement).getAttribute("data-stellar-tag") || ""),
        }),
      },
    ];
  },

  renderHTML({ node }) {
    return ["div", { "data-stellar-tag": encodeRaw(node.attrs.raw) }];
  },

  addNodeView() {
    return VueNodeViewRenderer(StellarTagView);
  },

  // tiptap-markdown 集成：serialize 原样回写 raw；parse.setup 注册 core rule
  // 覆盖初始内容 / setContent / insertContent / 粘贴全部解析路径
  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          state.write(node.attrs.raw);
          state.closeBlock(node);
        },
        parse: {
          setup(md: any) {
            md.core.ruler.before("normalize", "stellar_tag", (state: any) => {
              state.src = wrapShortcodes(state.src);
            });
          },
        },
      },
    };
  },
});
