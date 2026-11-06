// 写作助手 SSE 解析器：按规范逐行解析，任意位置切块都得到同样的事件。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { createSSEParser, type SSEMessage } from "../../../packages/api-client/src/sse.ts";

function parseAll(chunks: string[]): SSEMessage[] {
  const out: SSEMessage[] = [];
  const parser = createSSEParser((message) => out.push(message));
  for (const chunk of chunks) parser.push(chunk);
  parser.end();
  return out;
}

const stream =
  ': ping\n\ndata: {"type":"delta","text":"你好"}\n\ndata: {"type":"delta","text":"，世界"}\n\ndata: {"type":"done"}\n\n';
const expected = [
  { event: "message", data: '{"type":"delta","text":"你好"}' },
  { event: "message", data: '{"type":"delta","text":"，世界"}' },
  { event: "message", data: '{"type":"done"}' },
];

test("sse: parses events and ignores comment keep-alives", () => {
  assert.deepEqual(parseAll([stream]), expected);
});

test("sse: the same events arrive however the stream is cut", () => {
  for (let cut = 0; cut <= stream.length; cut++) {
    assert.deepEqual(parseAll([stream.slice(0, cut), stream.slice(cut)]), expected, `cut at ${cut}`);
  }
  // 逐字符喂入是最坏情况。
  assert.deepEqual(parseAll(Array.from(stream)), expected);
});

test("sse: handles \\r\\n and bare \\r line endings, including a cut between them", () => {
  const crlf = 'data: {"a":1}\r\n\r\ndata: {"b":2}\r\n\r\n';
  const want = [
    { event: "message", data: '{"a":1}' },
    { event: "message", data: '{"b":2}' },
  ];
  assert.deepEqual(parseAll([crlf]), want);
  for (let cut = 0; cut <= crlf.length; cut++) {
    assert.deepEqual(parseAll([crlf.slice(0, cut), crlf.slice(cut)]), want, `crlf cut at ${cut}`);
  }
  assert.deepEqual(parseAll(['data: x\r\rdata: y\r\r']), [
    { event: "message", data: "x" },
    { event: "message", data: "y" },
  ]);
});

test("sse: joins multi-line data, strips one leading space and honours event names", () => {
  assert.deepEqual(parseAll(["event: custom\ndata: one\ndata:two\ndata:  three\n\n"]), [
    { event: "custom", data: "one\ntwo\n three" },
  ]);
  assert.deepEqual(parseAll(["data\n\n"]), [{ event: "message", data: "" }]);
});

test("sse: skips a leading BOM, unknown fields and events without data", () => {
  assert.deepEqual(parseAll(["﻿data: ok\n\n"]), [{ event: "message", data: "ok" }]);
  assert.deepEqual(parseAll(["id: 7\nretry: 100\n\n"]), []);
  assert.deepEqual(parseAll(["event: only-name\n\n"]), []);
});

test("sse: an event cut off before its blank line is not dispatched", () => {
  assert.deepEqual(parseAll(['data: {"type":"delta"']), []);
  assert.deepEqual(parseAll(["data: partial\n"]), []);
});
