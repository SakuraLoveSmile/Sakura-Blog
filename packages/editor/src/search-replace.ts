import { Extension, type Editor } from "@tiptap/core";
import { Plugin, PluginKey, type EditorState } from "@tiptap/pm/state";
import type { Node as ProseMirrorNode } from "@tiptap/pm/model";
import { Decoration, DecorationSet } from "@tiptap/pm/view";

export interface SearchMatch {
  from: number;
  to: number;
  text: string;
}

export interface SearchReplaceState {
  query: string;
  caseSensitive: boolean;
  activeIndex: number;
}

const EMPTY_STATE: SearchReplaceState = {
  query: "",
  caseSensitive: false,
  activeIndex: 0,
};

export const searchReplacePluginKey = new PluginKey<SearchReplaceState>("searchReplace");

/**
 * Find matches inside text nodes without flattening the document. Keeping each
 * match in its original node means replacing it can retain the node's marks.
 * Empty queries intentionally produce no matches.
 */
export function findTextMatches(
  doc: ProseMirrorNode,
  query: string,
  caseSensitive = false,
): SearchMatch[] {
  if (!query) return [];

  const needle = caseSensitive ? query : query.toLocaleLowerCase();
  const matches: SearchMatch[] = [];

  doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return true;

    const haystack = caseSensitive ? node.text : node.text.toLocaleLowerCase();
    let offset = 0;
    while (offset < haystack.length) {
      const index = haystack.indexOf(needle, offset);
      if (index < 0) break;
      matches.push({
        from: pos + index,
        to: pos + index + query.length,
        text: node.text.slice(index, index + query.length),
      });
      offset = index + needle.length;
    }
    return true;
  });

  return matches;
}

function activeMatchIndex(activeIndex: number, count: number): number {
  if (count === 0) return -1;
  return ((activeIndex % count) + count) % count;
}

function createDecorations(state: EditorState, search: SearchReplaceState): DecorationSet {
  const matches = findTextMatches(state.doc, search.query, search.caseSensitive);
  const active = activeMatchIndex(search.activeIndex, matches.length);
  return DecorationSet.create(
    state.doc,
    matches.map((match, index) =>
      Decoration.inline(match.from, match.to, {
        class: index === active ? "rte-search-match rte-search-match-current" : "rte-search-match",
      }),
    ),
  );
}

export const SearchReplaceExtension = Extension.create({
  name: "searchReplace",

  addProseMirrorPlugins() {
    return [
      new Plugin<SearchReplaceState>({
        key: searchReplacePluginKey,
        state: {
          init: () => ({ ...EMPTY_STATE }),
          apply(transaction, value) {
            const meta = transaction.getMeta(searchReplacePluginKey) as Partial<SearchReplaceState> | undefined;
            if (!meta) return value;
            return {
              query: meta.query ?? value.query,
              caseSensitive: meta.caseSensitive ?? value.caseSensitive,
              activeIndex: meta.activeIndex ?? value.activeIndex,
            };
          },
        },
        props: {
          decorations(state) {
            return createDecorations(state, searchReplacePluginKey.getState(state) ?? EMPTY_STATE);
          },
        },
      }),
    ];
  },
});

export function setSearchReplaceState(editor: Editor, search: SearchReplaceState): void {
  editor.view.dispatch(editor.state.tr.setMeta(searchReplacePluginKey, search));
}

function marksAt(state: EditorState, from: number) {
  return state.doc.resolve(from).marks();
}

/** Replace one match and preserve the marks at the start of the matched text. */
export function replaceSearchMatch(editor: Editor, match: SearchMatch, replacement: string): void {
  const { state } = editor;
  const tr = state.tr;
  if (replacement) {
    tr.replaceWith(match.from, match.to, state.schema.text(replacement, marksAt(state, match.from)));
  } else {
    tr.delete(match.from, match.to);
  }
  editor.view.dispatch(tr);
}

/**
 * Replace from right to left so changing one match's length never shifts the
 * positions of the matches still waiting to be replaced.
 */
export function replaceAllSearchMatches(
  editor: Editor,
  matches: readonly SearchMatch[],
  replacement: string,
): void {
  if (matches.length === 0) return;

  const { state } = editor;
  const tr = state.tr;
  [...matches]
    .sort((left, right) => right.from - left.from)
    .forEach((match) => {
      if (replacement) {
        tr.replaceWith(match.from, match.to, state.schema.text(replacement, marksAt(state, match.from)));
      } else {
        tr.delete(match.from, match.to);
      }
    });
  editor.view.dispatch(tr);
}
