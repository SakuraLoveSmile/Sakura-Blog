import { test } from "node:test";
import assert from "node:assert/strict";
import {
  findTextMatches,
  replaceAllSearchMatches,
  replaceSearchMatch,
} from "../web/src/components/editor/search-replace.ts";

function createDocument(...texts: string[]) {
  return {
    descendants(callback: (node: { isText: boolean; text: string }, pos: number) => boolean) {
      texts.reduce((pos, text) => {
        callback({ isText: true, text }, pos);
        return pos + text.length + 2;
      }, 1);
    },
  };
}

function createHarness(...texts: string[]) {
  const mark = { type: { name: "bold" } };
  const calls: Array<{
    type: "replace" | "delete";
    from: number;
    to: number;
    node?: { text: string; marks: unknown[] };
  }> = [];
  const doc = {
    ...createDocument(...texts),
    resolve: () => ({ marks: () => [mark] }),
  };
  const transaction = {
    replaceWith(from: number, to: number, node: { text: string; marks: unknown[] }) {
      calls.push({ type: "replace", from, to, node });
      return transaction;
    },
    delete(from: number, to: number) {
      calls.push({ type: "delete", from, to });
      return transaction;
    },
  };
  const state = {
    doc,
    schema: {
      text(text: string, marks: unknown[]) {
        return { text, marks };
      },
    },
    tr: transaction,
  };
  const editor = {
    state,
    view: {
      dispatch() {},
    },
  };
  return { editor: editor as never, state, calls };
}

test("search replace · collects all matches and handles empty queries", () => {
  const { state } = createHarness("Alpha alpha ALPHA");

  assert.equal(findTextMatches(state.doc, "alpha").length, 3);
  assert.equal(findTextMatches(state.doc, "alpha", true).length, 1);
  assert.deepEqual(findTextMatches(state.doc, ""), []);
});

test("search replace · replaces one match without changing other text", () => {
  const harness = createHarness("one two one");
  const matches = findTextMatches(harness.state.doc, "one");

  replaceSearchMatch(harness.editor, matches[0], "first");

  assert.deepEqual(harness.calls, [
    { type: "replace", from: 1, to: 4, node: { text: "first", marks: [{ type: { name: "bold" } }] } },
  ]);
});

test("search replace · replaces all from right to left and preserves marks", () => {
  const harness = createHarness("one two one");
  const matches = findTextMatches(harness.state.doc, "one");

  replaceAllSearchMatches(harness.editor, matches, "replacement");

  assert.equal(harness.calls.length, 2);
  assert.deepEqual(harness.calls.map((call) => [call.type, call.from, call.to]), [
    ["replace", 9, 12],
    ["replace", 1, 4],
  ]);
  assert.deepEqual(harness.calls.map((call) => call.node?.marks), [
    [{ type: { name: "bold" } }],
    [{ type: { name: "bold" } }],
  ]);
});

test("search replace · an empty replacement deletes each match", () => {
  const harness = createHarness("keep remove remove");
  const matches = findTextMatches(harness.state.doc, "remove");

  replaceAllSearchMatches(harness.editor, matches, "");

  assert.deepEqual(harness.calls.map((call) => [call.type, call.from, call.to]), [
    ["delete", 13, 19],
    ["delete", 6, 12],
  ]);
});
