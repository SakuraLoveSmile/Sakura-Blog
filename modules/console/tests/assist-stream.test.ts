// 写作助手 SSE 客户端：请求形态、事件分发、流开始前的失败、流中断与取消。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { afterEach, test } from "node:test";
import assert from "node:assert/strict";
import { aiApi, AssistRequestError } from "../../../packages/api-client/src/api/editor.ts";
import { configureApiTransport } from "../../../packages/api-client/src/transport.ts";
import type { AIAssistEvent } from "../../../packages/api-client/src/generated";

const realFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = realFetch;
  configureApiTransport({ onUnauthorized: undefined, onError: undefined });
});

const encoder = new TextEncoder();

function sse(...events: object[]): string {
  return events.map((event) => `data: ${JSON.stringify(event)}\n\n`).join("");
}

function streamResponse(chunks: string[]): Response {
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
      controller.close();
    },
  });
  return new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream; charset=utf-8" } });
}

async function collect(
  respond: (url: string, init: RequestInit) => Response | Promise<Response>,
  options: { signal?: AbortSignal } = {},
): Promise<{ events: AIAssistEvent[]; requests: Array<{ url: string; init: RequestInit }> }> {
  const requests: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init: RequestInit = {}) => {
    requests.push({ url: String(input), init });
    return respond(String(input), init);
  }) as typeof fetch;
  const events: AIAssistEvent[] = [];
  await aiApi.assistStream({ action: "polish", selection: "原文" }, { ...options, onEvent: (event) => events.push(event) });
  return { events, requests };
}

test("assistStream: posts the JSON body with cookies and delivers events in order", async () => {
  const { events, requests } = await collect(() =>
    streamResponse([sse({ type: "delta", text: "润色" }, { type: "delta", text: "后" }, { type: "done", finish_reason: "stop" })]),
  );
  assert.deepEqual(events, [
    { type: "delta", text: "润色" },
    { type: "delta", text: "后" },
    { type: "done", finish_reason: "stop" },
  ]);
  assert.equal(requests.length, 1);
  const { url, init } = requests[0];
  assert.equal(url, "/api/v1/ai/assist");
  assert.equal(init.method, "POST");
  assert.equal(init.credentials, "same-origin");
  const headers = init.headers as Record<string, string>;
  assert.equal(headers["Content-Type"], "application/json");
  assert.equal(headers.Accept, "text/event-stream");
  assert.deepEqual(JSON.parse(String(init.body)), { action: "polish", selection: "原文" });
});

test("assistStream: events survive being cut at any byte-level boundary of the decoded text", async () => {
  const full = sse({ type: "delta", text: "你好，世界" }, { type: "done" });
  const bytes = encoder.encode(full);
  // 在多字节字符中间切开：TextDecoder 的 stream 模式必须把它们接上。
  const cut = full.indexOf("好") + 1; // 字符串下标；换算成字节下标后落在汉字中间
  const byteCut = encoder.encode(full.slice(0, cut)).length - 1;
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(bytes.slice(0, byteCut));
      controller.enqueue(bytes.slice(byteCut));
      controller.close();
    },
  });
  const { events } = await collect(() => new Response(body, { status: 200 }));
  assert.deepEqual(events, [{ type: "delta", text: "你好，世界" }, { type: "done" }]);
});

test("assistStream: ignores keep-alive comments and unparsable data lines", async () => {
  const { events } = await collect(() =>
    streamResponse([": ping\n\n", "data: not json\n\n", sse({ type: "delta", text: "a" }), sse({ type: "done" })]),
  );
  assert.deepEqual(events, [{ type: "delta", text: "a" }, { type: "done" }]);
});

test("assistStream: an upstream error event is delivered as the end of the stream", async () => {
  const { events } = await collect(() =>
    streamResponse([sse({ type: "delta", text: "半截" }, { type: "error", message: "AI 调用失败: 上游超时" })]),
  );
  assert.deepEqual(events, [
    { type: "delta", text: "半截" },
    { type: "error", message: "AI 调用失败: 上游超时" },
  ]);
});

test("assistStream: a stream that closes without done or error is reported as interrupted", async () => {
  const { events } = await collect(() => streamResponse([sse({ type: "delta", text: "只有一半" })]));
  assert.equal(events.length, 2);
  assert.deepEqual(events[0], { type: "delta", text: "只有一半" });
  assert.equal(events[1].type, "error");
  assert.match(events[1].message ?? "", /中断/);
});

test("assistStream: a read failure after the stream started is reported, not thrown", async () => {
  // controller.error 会丢弃队列里尚未被读走的块，所以第一次 pull 给数据，第二次 pull 才报错。
  let pulls = 0;
  const body = new ReadableStream<Uint8Array>({
    pull(controller) {
      pulls++;
      if (pulls === 1) controller.enqueue(encoder.encode(sse({ type: "delta", text: "a" })));
      else controller.error(new TypeError("network error"));
    },
  });
  const { events } = await collect(() => new Response(body, { status: 200 }));
  assert.deepEqual(events[0], { type: "delta", text: "a" });
  assert.equal(events[events.length - 1].type, "error");
  assert.equal(events.filter((e) => e.type === "error").length, 1);
});

test("assistStream: failures before the stream starts throw AssistRequestError with a displayable message", async () => {
  const cases: Array<[Response, number, RegExp]> = [
    [Response.json({ error: "请先选中要处理的文字" }, { status: 400 }), 400, /请先选中/],
    [Response.json({ detail: "AI 未配置，请到 设置 → AI 填写" }, { status: 400 }), 400, /AI 未配置/],
    [Response.json({ title: "写作助手正忙，请稍后再试" }, { status: 429 }), 429, /正忙/],
    [new Response("", { status: 413 }), 413, /太长/],
    [new Response("", { status: 403 }), 403, /没有权限/],
    [new Response("<html>bad gateway</html>", { status: 502 }), 502, /HTTP 502/],
  ];
  for (const [response, status, message] of cases) {
    await assert.rejects(
      collect(() => response),
      (error: unknown) =>
        error instanceof AssistRequestError && error.status === status && message.test(error.message),
      `status ${status}`,
    );
  }
});

test("assistStream: 401 also runs the host's unauthorized policy", async () => {
  const seen: unknown[] = [];
  configureApiTransport({ onUnauthorized: (error) => seen.push(error) });
  await assert.rejects(
    collect(() => new Response("", { status: 401 })),
    (error: unknown) => error instanceof AssistRequestError && error.status === 401 && /过期/.test(error.message),
  );
  assert.equal(seen.length, 1);
  assert.ok(seen[0] instanceof AssistRequestError);
});

test("assistStream: a network failure is a status-0 AssistRequestError", async () => {
  await assert.rejects(
    collect(() => {
      throw new TypeError("Failed to fetch");
    }),
    (error: unknown) => error instanceof AssistRequestError && error.status === 0 && /网络/.test(error.message),
  );
});

test("assistStream: aborting rethrows the abort and never reports an interruption event", async () => {
  const controller = new AbortController();
  const events: AIAssistEvent[] = [];
  globalThis.fetch = (async (_input: RequestInfo | URL, init: RequestInit = {}) => {
    const signal = init.signal as AbortSignal;
    const body = new ReadableStream<Uint8Array>({
      start(stream) {
        stream.enqueue(encoder.encode(sse({ type: "delta", text: "先到的" })));
        signal.addEventListener("abort", () => stream.error(new DOMException("aborted", "AbortError")));
      },
    });
    return new Response(body, { status: 200 });
  }) as typeof fetch;

  const pending = aiApi.assistStream(
    { action: "polish", selection: "x" },
    {
      signal: controller.signal,
      onEvent: (event) => {
        events.push(event);
        if (event.type === "delta") controller.abort();
      },
    },
  );
  await assert.rejects(pending, (error: unknown) => error instanceof DOMException && error.name === "AbortError");
  assert.deepEqual(events, [{ type: "delta", text: "先到的" }]);
});

test("assistStream: aborting before the request leaves rejects with the abort error", async () => {
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(
    collect(
      (_url, init) => {
        throw new DOMException("aborted", (init.signal as AbortSignal).aborted ? "AbortError" : "Error");
      },
      { signal: controller.signal },
    ),
    (error: unknown) => error instanceof DOMException && error.name === "AbortError",
  );
});
