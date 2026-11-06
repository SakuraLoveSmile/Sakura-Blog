import { expect, test, type Page } from "@playwright/test";

// 文章编辑器「写作助手」浏览器验收（BLG-20261106-01），用例编号对应 docs/plans/editor-writing-assistant.md 的验收清单。
// 真实模型服务不在验收范围：页面里的 fetch 被替换成受控的 SSE 流，且只拦截 /api/v1/ai/assist，其余请求照常走真实隔离服务。
// 设置 ASSISTANT_SHOTS_DIR 才会写验收截图（默认不写，避免普通运行改动受跟踪的 docs/baselines）。

type AssistEvent = {
  type: "delta" | "done" | "error";
  text?: string;
  message?: string;
  finish_reason?: string;
  truncated?: boolean;
};
type Reply = {
  /** 非 200 时按流开始前的 JSON 错误返回。 */
  status?: number;
  body?: unknown;
  networkError?: boolean;
  /** 自动回放的事件；不设置则是手动模式，由 push()/end() 推进。 */
  events?: AssistEvent[];
  /** 回放时每个事件之间的间隔。 */
  gapMs?: number;
};
type AssistWindow = Window & {
  __assist: {
    requests: Array<Record<string, unknown>>;
    aborted: number;
    cancelled: number;
    reply: Reply | null;
    push(event: AssistEvent): void;
    end(): void;
  };
  __consoleTiptapEditor?: any;
};

/** 在页面里安装受控的 SSE 流（addInitScript 在每次导航前执行）。 */
function installAssistMock() {
  const realFetch = window.fetch.bind(window);
  const state: AssistWindow["__assist"] = {
    requests: [],
    aborted: 0,
    cancelled: 0,
    reply: null,
    push: () => {},
    end: () => {},
  };
  (window as unknown as AssistWindow).__assist = state;
  window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    if (!url.includes("/api/v1/ai/assist")) return realFetch(input, init);
    state.requests.push(JSON.parse(String(init?.body ?? "{}")));
    const reply = state.reply;
    if (reply?.networkError) return Promise.reject(new TypeError("Failed to fetch"));
    if (reply?.status && reply.status !== 200) {
      return Promise.resolve(
        new Response(JSON.stringify(reply.body ?? {}), {
          status: reply.status,
          headers: { "Content-Type": "application/json" },
        }),
      );
    }
    const encoder = new TextEncoder();
    let controller!: ReadableStreamDefaultController<Uint8Array>;
    const stream = new ReadableStream<Uint8Array>({
      start(c) {
        controller = c;
      },
      cancel() {
        state.cancelled++;
      },
    });
    const send = (event: AssistEvent) => {
      try {
        controller.enqueue(encoder.encode("data: " + JSON.stringify(event) + "\n\n"));
      } catch {
        // 流已经结束或出错
      }
    };
    state.push = send;
    state.end = () => {
      try {
        controller.close();
      } catch {
        // 同上
      }
    };
    init?.signal?.addEventListener("abort", () => {
      state.aborted++;
      try {
        controller.error(new DOMException("Aborted", "AbortError"));
      } catch {
        // 同上
      }
    });
    if (reply?.events) {
      const events = reply.events;
      const gap = reply.gapMs ?? 0;
      void (async () => {
        for (const event of events) {
          send(event);
          if (gap) await new Promise((resolve) => setTimeout(resolve, gap));
        }
        state.end();
      })();
    }
    return Promise.resolve(new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } }));
  };
}

const FIXTURE_TITLE = "写作助手验收夹具";
const P1 = "写博客是整理思路的一种方式。每一篇文章都是一次对自己的追问。";
const P2_A = "第二节的第一句话。";
const P2 = `${P2_A}第二节的第二句话，这里有**加粗内容**和[一个链接](https://example.com/a)。`;
const FIXTURE_MD = `## 第一节\n\n${P1}\n\n## 第二节\n\n${P2}`;

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
}

async function createPost(page: Page, opts: { kind?: "rich" | "markdown"; content?: string; title?: string } = {}) {
  const { kind = "rich", content = FIXTURE_MD, title = FIXTURE_TITLE } = opts;
  const slug = `assistant-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const created = await page.request.post("/api/v1/posts", {
    data: {
      slug,
      title,
      content_md: content,
      status: "draft",
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(created.ok(), `创建文章失败：${created.status()} ${await created.text()}`).toBeTruthy();
  await page.evaluate((k) => localStorage.setItem("post-editor:kind", k), kind);
  await page.goto(`/console/posts/edit/${slug}`);
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue(title);
  if (kind === "rich") {
    await expect(page.locator(".ProseMirror").first()).toContainText(content.replace(/[#*\[\]`>]/g, "").slice(0, 4));
    await page.waitForFunction(() => !!(window as AssistWindow).__consoleTiptapEditor);
  } else {
    await expect(mdArea(page)).toHaveValue(content);
  }
  return slug;
}

const mdArea = (page: Page) => page.getByPlaceholder(/^Markdown 内容/);
const panel = (page: Page) => page.locator("#writing-assistant-panel");
const toggle = (page: Page) => page.getByTestId("assistant-toggle");
const action = (page: Page, id: string) => panel(page).locator(`[data-action="${id}"]`);
const resultBody = (page: Page) => panel(page).getByTestId("wa-body");
const saveStatus = (page: Page) => page.locator(".editor-save-status");

async function openPanel(page: Page) {
  await toggle(page).click();
  await expect(panel(page)).toBeVisible();
}

const setReply = (page: Page, reply: Reply | null) =>
  page.evaluate((r) => {
    (window as AssistWindow).__assist.reply = r;
  }, reply);
const assistRequests = (page: Page) => page.evaluate(() => (window as AssistWindow).__assist.requests);
const assistCounters = (page: Page) =>
  page.evaluate(() => {
    const s = (window as AssistWindow).__assist;
    return { requests: s.requests.length, aborted: s.aborted, cancelled: s.cancelled };
  });
const pushEvent = (page: Page, event: AssistEvent) =>
  page.evaluate((e) => (window as AssistWindow).__assist.push(e), event);
const endStream = (page: Page) => page.evaluate(() => (window as AssistWindow).__assist.end());

/** 一次性回放：若干 delta + done。 */
const finishWith = (text: string, done: Partial<AssistEvent> = {}): Reply => ({
  events: [
    { type: "delta", text },
    { type: "done", ...done },
  ],
});

async function richSelect(page: Page, start: string, end = start) {
  await page.evaluate(
    ({ start, end }) => {
      const editor = (window as AssistWindow).__consoleTiptapEditor;
      let from = -1;
      let to = -1;
      editor.state.doc.descendants((node: any, pos: number) => {
        if (!node.isText || !node.text) return;
        if (from < 0) {
          const i = node.text.indexOf(start);
          if (i >= 0) from = pos + i;
        }
        const j = node.text.indexOf(end);
        if (from >= 0 && j >= 0 && pos + j >= from) to = pos + j + end.length;
      });
      if (from < 0 || to < 0) throw new Error(`text not found: ${start} … ${end}`);
      editor.chain().focus().setTextSelection({ from, to }).run();
    },
    { start, end },
  );
}

/** 把光标放在某段文字的末尾（无选区）。 */
async function richCaretAfter(page: Page, text: string) {
  await page.evaluate((text) => {
    const editor = (window as AssistWindow).__consoleTiptapEditor;
    let at = -1;
    editor.state.doc.descendants((node: any, pos: number) => {
      if (at >= 0 || !node.isText || !node.text) return;
      const i = node.text.indexOf(text);
      if (i >= 0) at = pos + i + text.length;
    });
    if (at < 0) throw new Error(`text not found: ${text}`);
    editor.chain().focus().setTextSelection(at).run();
  }, text);
  // TipTap 的 focus() 在下一帧才真正聚焦；等聚焦后再输入，否则第一个字会丢。
  await expect(page.locator(".ProseMirror").first()).toBeFocused();
}

const richMarkdown = (page: Page) =>
  page.evaluate(() => (window as AssistWindow).__consoleTiptapEditor.storage.markdown.getMarkdown() as string);

async function mdSelect(page: Page, start: string, end = start) {
  await mdArea(page).evaluate(
    (ta: HTMLTextAreaElement, { start, end }) => {
      const a = ta.value.indexOf(start);
      const b = ta.value.indexOf(end, a);
      if (a < 0 || b < 0) throw new Error(`text not found: ${start} … ${end}`);
      ta.focus();
      ta.setSelectionRange(a, b + end.length);
      ta.dispatchEvent(new Event("select", { bubbles: true }));
    },
    { start, end },
  );
}

async function mdCaretAfter(page: Page, text: string) {
  await mdArea(page).evaluate((ta: HTMLTextAreaElement, text) => {
    const a = ta.value.indexOf(text);
    if (a < 0) throw new Error(`text not found: ${text}`);
    ta.focus();
    ta.setSelectionRange(a + text.length, a + text.length);
    ta.dispatchEvent(new Event("select", { bubbles: true }));
  }, text);
}

/** 等有限的 CSS 过渡/动画结束再量位置（过渡中的 transform 会改变包围盒）；光标闪烁这类无限动画不等。 */
const settle = (page: Page) =>
  page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ).then(() => undefined),
  );

const hasHorizontalScroll = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);

async function shot(page: Page, name: string) {
  const dir = process.env.ASSISTANT_SHOTS_DIR;
  if (dir) await page.screenshot({ path: `${dir}/${name}.png` });
}

const consoleErrors: string[] = [];

test.beforeEach(async ({ page }) => {
  await page.addInitScript(installAssistMock);
  await login(page);
  consoleErrors.length = 0;
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });
  page.on("pageerror", (error) => consoleErrors.push(String(error)));
});

test.afterEach(() => {
  expect(consoleErrors, "浏览器控制台不应有错误").toEqual([]);
});

// ---------------------------------------------------------------- A. 入口与布局

test("A1 桌面：入口按钮带快捷键提示，面板并入右栏，标题与正文位置不跳动，关闭后恢复大纲/设置", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  await expect(toggle(page)).toBeVisible();
  await expect(toggle(page)).toHaveAttribute("title", /写作助手（(⌘J|Ctrl\+J)）/);
  await shot(page, "after-editor-1280");

  const titleBox = await page.locator("#post-title").boundingBox();
  const editorBox = await page.locator(".ProseMirror").first().boundingBox();
  await expect(page.getByRole("button", { name: "大纲", exact: true })).toBeVisible();

  await openPanel(page);
  await expect(page.locator("#post-settings-panel")).toContainText("写作助手");
  await expect(page.getByRole("button", { name: "大纲", exact: true })).toBeHidden();
  expect(await page.locator("#post-title").boundingBox()).toEqual(titleBox);
  expect(await page.locator(".ProseMirror").first().boundingBox()).toEqual(editorBox);
  expect(await hasHorizontalScroll(page)).toBe(false);

  await toggle(page).click();
  await expect(panel(page)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "大纲", exact: true })).toBeVisible();
});

test("A2 快捷键：标题、正文、面板里都能开关，打开后焦点进面板、关闭后回编辑器，Esc 关闭，生成中 Esc 先停止", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  const activeId = () => page.evaluate(() => document.activeElement?.id ?? "");
  const inEditor = () => page.evaluate(() => !!document.activeElement?.closest(".ProseMirror"));

  // 事件的默认行为被阻止（浏览器自己的 Ctrl/⌘+J 不再触发）。
  const prevented = await page.evaluate(() => {
    const mac = /macintosh|mac os x/i.test(navigator.userAgent);
    const event = new KeyboardEvent("keydown", {
      key: "j",
      ctrlKey: !mac,
      metaKey: mac,
      bubbles: true,
      cancelable: true,
    });
    document.body.dispatchEvent(event);
    return event.defaultPrevented;
  });
  expect(prevented).toBe(true);
  await expect(panel(page)).toBeVisible(); // 上面的合成按键已经打开了面板
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toHaveCount(0);

  // 标题输入框里打开：焦点进入面板。
  await page.locator("#post-title").focus();
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toBeVisible();
  await expect.poll(activeId).toBe("writing-assistant-panel");
  // 面板里再按：关闭，焦点回到编辑器。
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toHaveCount(0);
  await expect.poll(inEditor).toBe(true);

  // 正文里打开、正文里关闭。
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toBeVisible();
  await page.locator(".ProseMirror").first().click();
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toHaveCount(0);

  // Esc 关闭，焦点回到编辑器。
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(panel(page)).toHaveCount(0);
  await expect.poll(inEditor).toBe(true);

  // 生成中 Esc 先停止（面板仍在，保留已生成的部分），再按一次才关闭。
  await richSelect(page, P1);
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toBeVisible();
  await action(page, "polish").click();
  await expect(action(page, "stop")).toBeVisible();
  await pushEvent(page, { type: "delta", text: "已经生成的一部分" });
  await expect(resultBody(page)).toContainText("已经生成的一部分");
  await page.keyboard.press("Escape");
  await expect(panel(page)).toBeVisible();
  await expect(action(page, "apply")).toBeVisible();
  await expect(resultBody(page)).toContainText("已经生成的一部分");
  expect((await assistCounters(page)).aborted).toBe(1);
  await page.keyboard.press("Escape");
  await expect(panel(page)).toHaveCount(0);
});

test("A3 手机：工具栏有入口，非模态底部面板在标签栏之上、不盖顶栏，编辑区仍可滚动，点击区 ≥44px", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const long = Array.from({ length: 40 }, (_, i) => `第 ${i + 1} 段正文，用来撑出页面高度。${"内容".repeat(30)}`).join(
    "\n\n",
  );
  await createPost(page, { content: long });
  await expect(toggle(page)).toBeVisible();
  const toggleBox = (await toggle(page).boundingBox())!;
  expect(toggleBox.width).toBeGreaterThanOrEqual(44);
  expect(toggleBox.height).toBeGreaterThanOrEqual(44);
  await shot(page, "after-editor-390");

  await openPanel(page);
  await settle(page);
  await shot(page, "after-panel-390");
  const box = (await panel(page).boundingBox())!;
  const nav = (await page.getByRole("navigation", { name: "移动端底部导航" }).boundingBox())!;
  const header = (await page.locator(".editor-header").boundingBox())!;
  // 面板紧贴底部标签栏上沿（不盖住标签栏，也不留缝）。
  expect(Math.abs(box.y + box.height - nav.y)).toBeLessThanOrEqual(1);
  expect(box.y).toBeGreaterThanOrEqual(header.y + header.height);
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(390);
  expect(await hasHorizontalScroll(page)).toBe(false);

  // 面板内每个可见的可点击控件都不小于 44×44。
  const small = await panel(page).evaluate((root) => {
    const bad: string[] = [];
    for (const el of root.querySelectorAll<HTMLElement>("button, select, textarea, [tabindex='0']")) {
      const r = el.getBoundingClientRect();
      if (r.width === 0 || r.height === 0) continue;
      if (r.width < 44 || r.height < 44)
        bad.push(`${el.tagName}.${el.className} ${Math.round(r.width)}x${Math.round(r.height)}`);
    }
    return bad;
  });
  expect(small).toEqual([]);

  // 面板打开时，编辑区仍可滚动（滚动发生在 Console 的内容区里）。
  const scroller = page.locator("main.content");
  const before = await scroller.evaluate((el) => el.scrollTop);
  await page.mouse.move(195, 160);
  await page.mouse.wheel(0, 400);
  await expect.poll(() => scroller.evaluate((el) => el.scrollTop)).toBeGreaterThan(before);
  // 底部标签栏没有被盖住。
  await expect(page.getByRole("navigation", { name: "移动端底部导航" })).toBeVisible();
});

test("A4 800 宽：同样是底部面板，右栏内容不被替换也不被挤出；1280 下关闭后保留之前所在的标签", async ({ page }) => {
  await page.setViewportSize({ width: 800, height: 900 });
  await createPost(page);
  await openPanel(page);
  await settle(page);
  await shot(page, "after-panel-800");
  const box = (await panel(page).boundingBox())!;
  expect(box.x + box.width).toBeLessThanOrEqual(800);
  expect(box.y + box.height).toBeLessThanOrEqual(900);
  expect(box.width).toBeLessThanOrEqual(420);
  expect(await hasHorizontalScroll(page)).toBe(false);
  await expect(page.getByRole("button", { name: "大纲", exact: true })).toBeVisible();
  await toggle(page).click();
  await expect(panel(page)).toHaveCount(0);

  await page.setViewportSize({ width: 1280, height: 900 });
  const tab = page.getByRole("button", { name: "设置", exact: true });
  await tab.click();
  await expect(tab).toHaveAttribute("aria-pressed", "true");
  await openPanel(page);
  await expect(tab).toBeHidden();
  await toggle(page).click();
  await expect(tab).toBeVisible();
  await expect(tab).toHaveAttribute("aria-pressed", "true");
});

test("A5 切换编辑器或离开文章时，进行中的请求被取消，面板状态重置", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  await richSelect(page, P1);
  await openPanel(page);
  await action(page, "polish").click();
  await pushEvent(page, { type: "delta", text: "生成到一半" });
  await expect(resultBody(page)).toContainText("生成到一半");

  // 切到 Markdown 编辑器：请求被取消，结果清空。
  await page.getByRole("button", { name: "切换编辑器" }).click();
  await page.getByRole("option", { name: "Markdown 编辑器" }).click();
  await expect(mdArea(page)).toBeVisible();
  await expect.poll(async () => (await assistCounters(page)).aborted).toBe(1);
  await expect(panel(page).getByTestId("wa-result")).toHaveCount(0);
  await expect(panel(page).getByTestId("wa-selection")).toContainText("没有选中文字");

  // 离开文章：再发起一次，离开后请求同样被取消。
  await mdSelect(page, P1);
  await action(page, "polish").click();
  await expect.poll(async () => (await assistCounters(page)).requests).toBe(2);
  await pushEvent(page, { type: "delta", text: "又生成了一半" });
  await page.getByRole("button", { name: "返回文章列表" }).click();
  await expect(page).toHaveURL(/\/console\/posts$/);
  expect((await assistCounters(page)).aborted).toBe(2);
});

test("A6 工具栏：窄时入口只留图标，把宽度让给格式工具；收起右栏变宽后带上文字", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  const label = toggle(page).locator(".editor-assistant-label");

  // 右栏打开时工具栏约 650px：只有图标，名称与快捷键提示仍在。
  await expect(label).toBeHidden();
  await expect(toggle(page)).toHaveAccessibleName("写作助手");
  const narrow = (await toggle(page).boundingBox())!.width;

  // 收起右栏后工具栏超过 840px：带上文字，入口变宽，页面仍无横向滚动。
  await page.getByTitle("收起右侧面板").click();
  await expect(label).toBeVisible();
  await expect(label).toHaveText("写作助手");
  expect((await toggle(page).boundingBox())!.width).toBeGreaterThan(narrow + 30);
  expect(await hasHorizontalScroll(page)).toBe(false);
  await expect(toggle(page)).toHaveAttribute("title", /写作助手（(⌘J|Ctrl\+J)）/);
  await shot(page, "after-editor-wide-1280");
});

// ---------------------------------------------------------------- B. 选区与动作

test("B1 Markdown：显示选区摘录与字数；没有选区时只有续写与自定义指令可用；仅预览模式下整体不可用", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  const selection = panel(page).getByTestId("wa-selection");
  const instruction = panel(page).locator("#writing-assistant-instruction");

  await expect(selection).toContainText("没有选中文字");
  for (const id of ["polish", "rewrite", "expand", "shorten", "translate"]) {
    await expect(action(page, id)).toBeDisabled();
    await expect(action(page, id)).toHaveAttribute("title", "先选中要处理的文字");
  }
  await expect(action(page, "continue")).toBeEnabled();
  await expect(action(page, "custom")).toBeDisabled(); // 指令为空
  await instruction.fill("在这里补一句总结");
  await expect(action(page, "custom")).toBeEnabled(); // 自定义指令没有选区也能用（在光标处写入）
  await instruction.fill("");

  await mdSelect(page, P1);
  await expect(selection).toContainText(`已选 ${[...P1].length} 字`);
  await expect(selection).toContainText(P1.slice(0, 12));
  for (const id of ["polish", "rewrite", "expand", "shorten", "continue", "translate"]) {
    await expect(action(page, id)).toBeEnabled();
  }

  // 仅预览：编辑面不可用，选区概况与按钮状态同步变化。
  await page.getByTitle("切换视图模式").selectOption("preview");
  await expect(selection).toContainText("编辑区当前不可用");
  for (const id of ["polish", "continue", "translate"]) await expect(action(page, id)).toBeDisabled();
  // 回到可编辑的模式：文本框一直保留着原来的选区，概况随之恢复；光标落下后变回「没有选中文字」。
  await page.getByTitle("切换视图模式").selectOption("split");
  await expect(selection).toContainText(`已选 ${[...P1].length} 字`);
  await mdArea(page).press("ArrowRight");
  await expect(selection).toContainText("没有选中文字");
  await expect(action(page, "polish")).toBeDisabled();
});

test("B2 富文本：选区含加粗与链接时，发给后端的 selection 是对应的 Markdown", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  await openPanel(page);
  await richSelect(page, "这里有", "一个链接");
  const expected = "这里有**加粗内容**和[一个链接](https://example.com/a)";
  await expect(panel(page).getByTestId("wa-selection")).toContainText(expected);

  await setReply(page, finishWith("润色后的内容"));
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();
  const [request] = await assistRequests(page);
  expect(request.action).toBe("polish");
  expect(request.selection).toBe(expected);
  expect(request.before).toContain(`${P2_A}第二节的第二句话，`);
  expect(request.after).toBe("。");
});

test("B3 限制：选区超过 8000 字时需要选区的动作被禁用；自定义指令为空或超过 500 字时不可发送", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const big = "字".repeat(8001);
  await createPost(page, { kind: "markdown", content: `开头\n\n${big}\n\n结尾` });
  await openPanel(page);
  const selection = panel(page).getByTestId("wa-selection");

  await mdSelect(page, big);
  await expect(selection).toContainText("已选 8001 字");
  await expect(selection).toContainText("超过 8000 字上限");
  for (const id of ["polish", "rewrite", "expand", "shorten", "translate"]) {
    await expect(action(page, id)).toBeDisabled();
    await expect(action(page, id)).toHaveAttribute("title", /8000/);
  }
  await expect(action(page, "continue")).toBeEnabled(); // 续写不看选区，长度不受限
  await mdSelect(page, "字".repeat(8000));
  await expect(selection).toContainText("已选 8000 字");
  await expect(action(page, "polish")).toBeEnabled();

  const instruction = panel(page).locator("#writing-assistant-instruction");
  await instruction.fill("字".repeat(501));
  await expect(action(page, "custom")).toBeDisabled();
  await expect(action(page, "custom")).toHaveAttribute("title", /不能超过 500/);
  await instruction.fill("字".repeat(500));
  await expect(action(page, "custom")).toBeEnabled();
  await instruction.fill("   ");
  await expect(action(page, "custom")).toBeDisabled();
});

test("B4 七个动作的 action 正确，翻译带目标语言，请求带标题、大纲与前后文", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await setReply(page, finishWith("结果。"));
  await mdSelect(page, P1);

  for (const id of ["polish", "rewrite", "expand", "shorten"]) {
    await action(page, id).click();
    await expect(action(page, "apply")).toBeVisible();
    await action(page, "discard").click();
  }
  await panel(page).locator("#writing-assistant-lang").selectOption("ja");
  await action(page, "translate").click();
  await expect(action(page, "apply")).toBeVisible();
  await action(page, "discard").click();
  await action(page, "continue").click();
  await expect(action(page, "apply")).toBeVisible();
  await action(page, "discard").click();
  await panel(page).locator("#writing-assistant-instruction").fill("改成更口语的语气");
  await action(page, "custom").click();
  await expect(action(page, "apply")).toBeVisible();

  const requests = await assistRequests(page);
  expect(requests.map((r) => r.action)).toEqual([
    "polish",
    "rewrite",
    "expand",
    "shorten",
    "translate",
    "continue",
    "custom",
  ]);
  const [polish, , , , translate, cont, custom] = requests;
  expect(polish.selection).toBe(P1);
  expect(polish.title).toBe(FIXTURE_TITLE);
  expect(polish.outline).toEqual(["第一节", "第二节"]);
  expect(polish.before).toBe("## 第一节\n\n");
  expect(polish.after).toBe(`\n\n## 第二节\n\n${P2}`);
  expect(polish).not.toHaveProperty("instruction");
  expect(polish).not.toHaveProperty("target_lang");
  expect(translate.target_lang).toBe("ja");
  expect(cont).not.toHaveProperty("selection"); // 续写不发送选区
  expect(cont.before).toBe(`## 第一节\n\n${P1}`);
  expect(custom.instruction).toBe("改成更口语的语气");
  expect(custom.selection).toBe(P1);
});

test("B4 长文档：前后文与大纲按上限截取，请求体仍小于 256KB", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const paragraphs = Array.from({ length: 400 }, (_, i) =>
    i % 4 === 0 ? `## 小节 ${i / 4 + 1}` : `第 ${i} 段：${"字".repeat(300)}`,
  );
  const content = paragraphs.join("\n\n"); // 约 90KB，100 个标题
  await createPost(page, { kind: "markdown", content });
  await openPanel(page);
  await setReply(page, finishWith("结果。"));
  await mdSelect(page, `第 201 段：${"字".repeat(300)}`);
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();

  const [request] = (await assistRequests(page)) as Array<{
    selection: string;
    before: string;
    after: string;
    outline: string[];
  }>;
  expect([...request.before].length).toBeLessThanOrEqual(1500);
  expect([...request.after].length).toBeLessThanOrEqual(600);
  expect(request.outline).toHaveLength(60);
  const bytes = new TextEncoder().encode(JSON.stringify(request)).length;
  expect(bytes).toBeLessThan(256 * 1024);
  expect(bytes).toBeLessThan(20 * 1024);
});

test("B5 富文本跨块选区：扩展到完整的块，面板提示，selection 是这些块的 Markdown", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  await openPanel(page);
  await richSelect(page, "整理思路", "第二节的第一句话");
  const selection = panel(page).getByTestId("wa-selection");
  await expect(selection).toContainText("选区跨越了多个块");
  await expect(selection).toContainText(P1.slice(0, 10));

  await setReply(page, finishWith("结果"));
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();
  const [request] = await assistRequests(page);
  expect(request.selection).toBe(`${P1}\n\n## 第二节\n\n${P2}`);
  expect(request.before).toBe("## 第一节"); // 前文是前面的块序列化后的 Markdown
  expect(request.after ?? "").toBe(""); // 后文为空时请求里不带 after
});

// ---------------------------------------------------------------- C. 流式与结果

test("C1 流式：文字逐步出现，生成中只能停止；停止后保留已生成的内容，可应用、重试或放弃", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await mdSelect(page, P1);
  await action(page, "polish").click();

  await expect(action(page, "stop")).toBeVisible();
  for (const id of ["polish", "rewrite", "expand", "shorten", "continue", "translate", "custom"]) {
    await expect(action(page, id)).toBeDisabled();
  }
  await pushEvent(page, { type: "delta", text: "第一段" });
  await expect(resultBody(page)).toContainText("第一段");
  await expect(resultBody(page)).not.toContainText("第二段");
  await pushEvent(page, { type: "delta", text: "第二段" });
  await expect(resultBody(page)).toContainText("第一段第二段");

  await action(page, "stop").click();
  await expect.poll(async () => (await assistCounters(page)).aborted).toBe(1);
  await expect(resultBody(page)).toContainText("第一段第二段");
  await expect(action(page, "apply")).toBeEnabled();
  await expect(action(page, "retry")).toBeVisible();
  await expect(action(page, "discard")).toBeVisible();
  await expect(action(page, "stop")).toHaveCount(0);

  // 重试：基于同一个选区重新生成。
  await action(page, "retry").click();
  await expect.poll(async () => (await assistCounters(page)).requests).toBe(2);
  await pushEvent(page, { type: "delta", text: "重试的结果" });
  await pushEvent(page, { type: "done" });
  await endStream(page);
  await expect(action(page, "apply")).toBeVisible();
  await expect(resultBody(page)).toContainText("重试的结果");
  expect((await assistRequests(page)).map((r) => r.selection)).toEqual([P1, P1]);

  // 放弃：回到可选动作。
  await action(page, "discard").click();
  await expect(panel(page).getByTestId("wa-result")).toHaveCount(0);
  await expect(action(page, "polish")).toBeEnabled();
});

test("C2 完成后可在对比与结果间切换，对比用删除线与高亮标出修改，翻译与续写直接看结果，390 下不横向溢出", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await mdSelect(page, P1);
  const NEW_P1 = "写博客是整理思路的好方法。每一篇文章，都是一次对自己的追问。";
  await setReply(page, finishWith(NEW_P1));
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();

  const diffButton = panel(page).getByRole("button", { name: "对比", exact: true });
  const resultButton = panel(page).getByRole("button", { name: "结果", exact: true });
  await expect(diffButton).toHaveAttribute("aria-pressed", "true"); // 修改不大，默认看对比
  await expect(resultBody(page).locator("ins").first()).toBeVisible();
  await expect(resultBody(page).locator("del").first()).toBeVisible();
  await expect(resultBody(page).locator("del").first()).toHaveCSS("text-decoration-line", "line-through");
  await expect(resultBody(page).locator("ins").first()).toHaveCSS("text-decoration-line", "underline");
  await expect(panel(page).getByTestId("wa-result")).toContainText(/新增 \d+ 字，删除 \d+ 字/);
  await shot(page, "after-diff-1280");

  await resultButton.click();
  await expect(resultButton).toHaveAttribute("aria-pressed", "true");
  await expect(resultBody(page).locator("ins, del")).toHaveCount(0);
  await expect(resultBody(page)).toHaveText(NEW_P1);
  await diffButton.click();
  await expect(resultBody(page).locator("ins").first()).toBeVisible();

  // 手机宽度下对比视图不横向溢出。
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(panel(page)).toHaveClass(/wa--sheet/);
  await settle(page);
  await expect(resultBody(page).locator("ins").first()).toBeVisible();
  expect(await hasHorizontalScroll(page)).toBe(false);
  expect(await resultBody(page).evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true);
  await page.setViewportSize({ width: 1280, height: 900 });
  await settle(page);

  // 翻译与续写没有可比的原文：直接看结果，没有对比开关。
  await action(page, "discard").click();
  await setReply(page, finishWith("Writing a blog is a way to organise your thoughts."));
  await action(page, "translate").click();
  await expect(action(page, "apply")).toBeVisible();
  await expect(panel(page).getByRole("group", { name: "结果视图" })).toHaveCount(0);
  await expect(resultBody(page).locator("ins, del")).toHaveCount(0);
  await expect(resultBody(page)).toHaveText("Writing a blog is a way to organise your thoughts.");
  await action(page, "discard").click();
  await setReply(page, finishWith("接着写下去的一段。"));
  await action(page, "continue").click();
  await expect(action(page, "apply")).toBeVisible();
  await expect(panel(page).getByRole("group", { name: "结果视图" })).toHaveCount(0);
  await expect(resultBody(page)).toHaveText("接着写下去的一段。");
});

test("C5 手机：生成后结果自动滚进底部面板的可见区域，应用按钮不用再滚动就能点到", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await settle(page);
  await mdSelect(page, P1);
  await setReply(page, finishWith("写博客是整理思路的好方法。每一篇文章，都是一次对自己的追问。"));
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();

  // 面板高度有限，结果排在选区与动作的下面：生成后要自动滚进面板的可见区域，对比与「应用」都能直接看到、点到。
  await expect(resultBody(page).locator("ins").first()).toBeInViewport();
  await expect(resultBody(page).locator("del").first()).toBeInViewport();
  await expect(action(page, "apply")).toBeInViewport({ ratio: 1 });
  expect(await hasHorizontalScroll(page)).toBe(false);
  await shot(page, "after-diff-390");
});

test("C3 未配置、并发已满、上游失败、网络中断都在面板内提示并可重试，不弹对话框", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await mdSelect(page, P1);
  const alert = panel(page).getByRole("alert");

  const cases: Array<[string, Reply, string]> = [
    [
      "未配置",
      { status: 400, body: { error: "AI 未配置，请到 设置 → AI 的「对话模型」中填写接口地址与模型" } },
      "设置 → AI",
    ],
    ["并发已满", { status: 429, body: { error: "写作助手正忙，请稍后再试" } }, "写作助手正忙"],
    ["上游失败", { events: [{ type: "error", message: "上游服务暂时不可用" }] }, "上游服务暂时不可用"],
    ["网络中断", { networkError: true }, "网络连接失败"],
  ];
  for (const [name, reply, message] of cases) {
    await setReply(page, reply);
    await action(page, "polish").click();
    await expect(alert, name).toContainText(message);
    await expect(action(page, "retry"), name).toBeVisible();
    await expect(action(page, "apply"), name).toHaveCount(0);
    await expect(page.getByRole("dialog"), name).toHaveCount(0);
    await expect(page.getByRole("alertdialog"), name).toHaveCount(0);
    if (name === "未配置") await shot(page, "after-error-1280");
    await action(page, "discard").click();
    await expect(action(page, "polish"), name).toBeEnabled();
  }

  // 故障排除后点「重试」就能继续。
  await setReply(page, { networkError: true });
  await action(page, "polish").click();
  await expect(alert).toContainText("网络连接失败");
  await setReply(page, finishWith("恢复后的结果。"));
  await action(page, "retry").click();
  await expect(resultBody(page)).toContainText("恢复后的结果。");
  await expect(action(page, "apply")).toBeVisible();
});

test("C4 输出被截断或因长度结束时提示结果可能不完整", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await mdSelect(page, P1);
  const result = panel(page).getByTestId("wa-result");

  await setReply(page, finishWith("被截断的结果", { truncated: true }));
  await action(page, "polish").click();
  await expect(result).toContainText("结果可能不完整");
  await expect(result).toContainText("已被截断");
  await expect(action(page, "apply")).toBeEnabled(); // 只提示，不阻止
  await action(page, "discard").click();

  await setReply(page, finishWith("到长度上限的结果", { finish_reason: "length" }));
  await action(page, "polish").click();
  await expect(result).toContainText("长度限制");
  await expect(result).toContainText("结果可能不完整");
});

// ---------------------------------------------------------------- D. 应用与撤销

const NEW_P1 = "写博客是整理思路的好方法。每一篇文章，都是一次对自己的追问。";

/** 记录对文章的写请求（保存、发布等）；助手的应用不应触发任何一个。 */
function trackWrites(page: Page) {
  const writes: string[] = [];
  page.on("request", (request) => {
    if (/\/api\/v1\/posts(\/|$)/.test(request.url()) && request.method() !== "GET") {
      writes.push(`${request.method()} ${request.url()}`);
    }
  });
  return writes;
}

test("D1-D3 Markdown：应用只改编辑器内容并进入未保存状态，Ctrl/⌘+Z 一步撤销，撤销按钮还原且再编辑后消失", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  const writes = trackWrites(page);
  await expect(saveStatus(page)).toHaveText("所有修改已保存");
  await openPanel(page);
  await mdSelect(page, P1);
  await setReply(page, finishWith(NEW_P1));
  await action(page, "polish").click();
  await action(page, "apply").click();

  // D1：编辑器内容更新、进入未保存状态，没有任何保存请求。
  const applied = FIXTURE_MD.replace(P1, NEW_P1);
  await expect(mdArea(page)).toHaveValue(applied);
  await expect(saveStatus(page)).toHaveText("有未保存的修改");
  await expect(panel(page).getByTestId("wa-result")).toContainText("已写入正文");
  expect(writes).toEqual([]);

  // D2：焦点已回到编辑器，Ctrl/⌘+Z 一步还原；结果又可以再次应用。
  await page.keyboard.press("ControlOrMeta+z");
  await expect(mdArea(page)).toHaveValue(FIXTURE_MD);
  await expect(action(page, "apply")).toBeVisible();
  await expect(action(page, "undo")).toHaveCount(0);

  // D3：再应用，点「撤销」按钮同样还原。
  await action(page, "apply").click();
  await expect(mdArea(page)).toHaveValue(applied);
  await expect(action(page, "undo")).toBeVisible();
  await action(page, "undo").click();
  await expect(mdArea(page)).toHaveValue(FIXTURE_MD);
  await expect(action(page, "apply")).toBeVisible();

  // 应用后又改了正文：「撤销」按钮消失，并说明原因。
  await action(page, "apply").click();
  await expect(action(page, "undo")).toBeVisible();
  await page.keyboard.press("ArrowRight"); // 应用后插入的文字处于选中状态，先收起选区
  await page.keyboard.type("补");
  await expect(action(page, "undo")).toHaveCount(0);
  await expect(panel(page).getByTestId("wa-result")).toContainText("无法一键撤销");
  expect(writes).toEqual([]);
});

test("D1-D3 富文本：应用是一次事务，Ctrl/⌘+Z 一步撤销，撤销按钮还原且再编辑后消失", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  const writes = trackWrites(page);
  const original = await richMarkdown(page);
  await openPanel(page);
  await richSelect(page, P1);
  await setReply(page, finishWith(NEW_P1));
  await action(page, "polish").click();
  await action(page, "apply").click();

  await expect.poll(() => richMarkdown(page)).toBe(original.replace(P1, NEW_P1));
  await expect(saveStatus(page)).toHaveText("有未保存的修改");
  expect(writes).toEqual([]);

  // 之前的输入不会被连带撤销：先在别处输入，再应用，再撤销，只回到应用之前。
  await page.keyboard.press("ControlOrMeta+z");
  await expect.poll(() => richMarkdown(page)).toBe(original);
  await expect(action(page, "apply")).toBeVisible();

  await action(page, "apply").click();
  await expect.poll(() => richMarkdown(page)).toBe(original.replace(P1, NEW_P1));
  await action(page, "undo").click();
  await expect.poll(() => richMarkdown(page)).toBe(original);
  await expect(action(page, "apply")).toBeVisible();

  await action(page, "apply").click();
  await expect(action(page, "undo")).toBeVisible();
  await page.keyboard.type("补");
  await expect(action(page, "undo")).toHaveCount(0);
  expect(writes).toEqual([]);
});

test("D1 富文本：先输入再应用再撤销，只回到应用之前，之前的输入保留", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  await richCaretAfter(page, P2_A);
  await page.keyboard.type("先写的一句。");
  const typed = await richMarkdown(page);
  expect(typed).toContain("先写的一句。");

  await openPanel(page);
  await richSelect(page, P1);
  await setReply(page, finishWith(NEW_P1));
  await action(page, "polish").click();
  await action(page, "apply").click();
  await expect.poll(() => richMarkdown(page)).toBe(typed.replace(P1, NEW_P1));
  await page.keyboard.press("ControlOrMeta+z");
  await expect.poll(() => richMarkdown(page)).toBe(typed);
});

test("D4 生成期间原文被改动：应用禁用并提示，重试基于当前选区重新生成", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await mdSelect(page, P1);
  await action(page, "polish").click();
  await pushEvent(page, { type: "delta", text: "生成中的结果" });

  // 生成期间改动被选中的原文。
  await mdArea(page).evaluate((ta: HTMLTextAreaElement) => {
    const at = ta.value.indexOf("整理思路");
    ta.focus();
    ta.setSelectionRange(at, at);
  });
  await page.keyboard.type("【改】");
  await pushEvent(page, { type: "done" });
  await endStream(page);

  await expect(panel(page).getByTestId("wa-result")).toContainText("原文在生成期间被改动");
  await expect(action(page, "apply")).toBeDisabled();

  // 重新选中改动后的文字，点「重试」：新的请求带的是当前选区。
  const changed = P1.replace("整理思路", "【改】整理思路");
  await mdSelect(page, changed);
  await setReply(page, finishWith(NEW_P1));
  await action(page, "retry").click();
  await expect(action(page, "apply")).toBeEnabled();
  expect((await assistRequests(page)).map((r) => r.selection)).toEqual([P1, changed]);
  await action(page, "apply").click();
  await expect(mdArea(page)).toHaveValue(FIXTURE_MD.replace(P1, NEW_P1));
});

test("D5 Markdown 续写：在所在块之后插入新段落；光标在围栏代码块内时插到围栏之后", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const md = "## 代码\n\n前面的段落。\n\n```js\nconst a = 1;\n\nconst b = 2;\n```\n\n结尾段落。";
  await createPost(page, { kind: "markdown", content: md });
  await openPanel(page);
  await setReply(page, finishWith("续写出的新段落。"));

  await mdCaretAfter(page, "前面的");
  await action(page, "continue").click();
  await action(page, "apply").click();
  const first = md.replace("前面的段落。", "前面的段落。\n\n续写出的新段落。");
  await expect(mdArea(page)).toHaveValue(first);
  await action(page, "discard").click();

  await mdCaretAfter(page, "const a = 1;");
  await action(page, "continue").click();
  await action(page, "apply").click();
  await expect(mdArea(page)).toHaveValue(first.replace("```\n\n结尾段落。", "```\n\n续写出的新段落。\n\n结尾段落。"));
  const requests = await assistRequests(page);
  expect(requests[1].before).toMatch(/```js\nconst a = 1;\n\nconst b = 2;\n```$/);
});

test("D5 富文本续写：在所在段落之后插入新段落，其余内容不变", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page);
  const original = await richMarkdown(page);
  await openPanel(page);
  await richCaretAfter(page, "整理思路");
  await setReply(page, finishWith("续写出的新段落。"));
  await action(page, "continue").click();
  await action(page, "apply").click();
  await expect.poll(() => richMarkdown(page)).toBe(original.replace(P1, `${P1}\n\n续写出的新段落。`));
  await page.keyboard.press("ControlOrMeta+z");
  await expect.poll(() => richMarkdown(page)).toBe(original);
});

test("D6 选区含标签而结果中的标签与原文不一致时给出提示，但不阻止应用", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const line = "这里有一个标签 {% button 按钮 https://example.com %} 在句中。";
  await createPost(page, { kind: "markdown", content: `前文。\n\n${line}\n\n后文。` });
  await openPanel(page);
  await mdSelect(page, line);

  await setReply(page, finishWith("这里有一个标签 {% button 改了的按钮 https://example.org %} 在句中。"));
  await action(page, "polish").click();
  await expect(panel(page).getByTestId("wa-result")).toContainText("标签");
  await expect(panel(page).getByTestId("wa-result")).toContainText("与原文不一致");
  await expect(action(page, "apply")).toBeEnabled();
  await action(page, "discard").click();

  // 标签原样保留时没有提示。
  await setReply(page, finishWith("这里有一个标签 {% button 按钮 https://example.com %} 出现在句中。"));
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();
  await expect(panel(page).getByTestId("wa-result")).not.toContainText("与原文不一致");
});

test("D7 复制把结果写入剪贴板；放弃清空结果并回到可选动作", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await createPost(page, { kind: "markdown" });
  await openPanel(page);
  await mdSelect(page, P1);
  await setReply(page, finishWith(`\n\n${NEW_P1}\n`));
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();

  await action(page, "copy").click();
  await expect(action(page, "copy")).toContainText("已复制");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(NEW_P1);
  await expect(mdArea(page)).toHaveValue(FIXTURE_MD); // 复制不改正文

  await action(page, "discard").click();
  await expect(panel(page).getByTestId("wa-result")).toHaveCount(0);
  await expect(action(page, "polish")).toBeEnabled();
  await expect(mdArea(page)).toHaveValue(FIXTURE_MD);
});

// ---------------------------------------------------------------- E. 权限与边界

test("E2 助手只请求 /api/v1/ai/assist，现有「AI 属性生成」弹窗不受影响", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await createPost(page, { kind: "markdown" });
  const aiNetwork: string[] = [];
  page.on("request", (request) => {
    if (/\/api\/v1\/ai\//.test(request.url())) aiNetwork.push(request.url());
  });
  await openPanel(page);
  await mdSelect(page, P1);
  await setReply(page, finishWith("结果。"));
  await action(page, "polish").click();
  await expect(action(page, "apply")).toBeVisible();
  // 助手的请求被页面内的受控流接管：没有真实的 AI 网络请求，更不会碰 /ai/generate。
  expect(aiNetwork).toEqual([]);
  expect((await assistRequests(page)).length).toBe(1);

  await page.getByTitle("AI 属性生成").click();
  await expect(page.getByRole("dialog", { name: "AI 属性生成" })).toBeVisible();
  // 弹窗打开时 Ctrl/⌘+J 让位给弹窗，不会在它后面悄悄开关助手。
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toBeVisible();
  await page.getByRole("button", { name: "关闭对话框" }).click();
  await expect(page.getByRole("dialog", { name: "AI 属性生成" })).toHaveCount(0);
});

// ---------------------------------------------------------------- F. 质量

test("F3 键盘可达：面板内控件都能用 Tab 到达且焦点可见；结果区是 aria-live；减弱动效时光标不闪烁", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await createPost(page, { kind: "markdown" });
  await mdSelect(page, P1);
  await page.keyboard.press("ControlOrMeta+j");
  await expect(panel(page)).toBeVisible();
  await panel(page).locator("#writing-assistant-instruction").fill("改成口语");
  await panel(page).evaluate((el) => el.focus());

  const tabbable = await panel(page).evaluate(
    (root) =>
      Array.from(root.querySelectorAll<HTMLElement>("button, select, textarea, [tabindex='0']")).filter(
        (el) => !(el as HTMLButtonElement).disabled && el.getClientRects().length > 0,
      ).length,
  );
  expect(tabbable).toBeGreaterThanOrEqual(10);
  const visited = new Set<string>();
  const noRing: string[] = [];
  for (let i = 0; i < tabbable; i++) {
    await page.keyboard.press("Tab");
    const info = await page.evaluate(() => {
      const root = document.getElementById("writing-assistant-panel");
      const el = document.activeElement as HTMLElement | null;
      if (!el || !root?.contains(el)) return null;
      const style = getComputedStyle(el);
      return {
        name: el.dataset.action || el.id || el.getAttribute("aria-label") || el.tagName,
        ring: style.outlineStyle !== "none" && parseFloat(style.outlineWidth) > 0,
      };
    });
    expect(info, `第 ${i + 1} 次 Tab 后焦点应仍在面板内`).not.toBeNull();
    visited.add(info!.name);
    if (!info!.ring) noRing.push(info!.name);
  }
  expect(visited.size).toBe(tabbable); // 每个控件恰好到达一次
  expect(noRing, "键盘聚焦的控件都要有可见的焦点环").toEqual([]);

  // 生成中：结果区忙碌、状态是 aria-live、光标在减弱动效下不闪烁。
  await action(page, "polish").click();
  await expect(resultBody(page)).toHaveAttribute("aria-busy", "true");
  await expect(panel(page).locator('[role="status"][aria-live="polite"]')).toHaveCount(1);
  await pushEvent(page, { type: "delta", text: "生成中" });
  await expect(resultBody(page)).toContainText("生成中");
  expect(await resultBody(page).evaluate((el) => getComputedStyle(el, "::after").animationName)).toBe("none");
  await pushEvent(page, { type: "done" });
  await endStream(page);
  await expect(resultBody(page)).toHaveAttribute("aria-busy", "false");
  await expect(panel(page).locator('[role="status"][aria-live="polite"]')).toContainText("已生成");
});
