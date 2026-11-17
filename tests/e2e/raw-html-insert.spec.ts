import { expect, test, type Page } from "@playwright/test";

// BLG-20261117-02：编辑器直插外部 HTML 的独立验收。
// 只断言可观察行为：逐字 round-trip、插入闭环、编辑回写、删除、粘贴保真、
// 干净语义 HTML 不退化、窄视口弹窗不溢出。数据来自 scripts/e2e/run-server.sh 的隔离根目录。

const FOREIGN_BLOCK = '<div class="ext-card" style="color:red">\n  <span data-v="1">你好 👋</span>\n</div>';

type EditorHook = {
  commands: { setContent: (value: string, emitUpdate?: boolean) => void };
  storage: { markdown: { getMarkdown: () => string } };
  state: { doc: { toJSON: () => any } };
};

declare global {
  interface Window {
    __consoleTiptapEditor?: EditorHook;
  }
}

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

async function openEditor(page: Page) {
  await page.goto("/console/posts/new");
  await page.locator('[contenteditable="true"]').first().waitFor({ state: "visible", timeout: 15_000 });
  await page.waitForFunction(() => Boolean(window.__consoleTiptapEditor), undefined, { timeout: 15_000 });
}

function setContent(page: Page, markdown: string) {
  return page.evaluate((value) => {
    const editor = window.__consoleTiptapEditor;
    if (!editor) throw new Error("TipTap editor hook unavailable");
    editor.commands.setContent(value, false);
    return editor.storage.markdown.getMarkdown();
  }, markdown);
}

function rawHtmlNodes(page: Page) {
  return page.evaluate(() => {
    const editor = window.__consoleTiptapEditor;
    if (!editor) throw new Error("TipTap editor hook unavailable");
    const found: { type: string; raw: string }[] = [];
    const walk = (node: any) => {
      if (node.type === "rawHtml" || node.type === "rawHtmlInline") {
        found.push({ type: node.type, raw: node.attrs?.raw ?? "" });
      }
      for (const child of node.content ?? []) walk(child);
    };
    walk(editor.state.doc.toJSON());
    return found;
  });
}

test("外部 HTML 块与行内片段逐字 round-trip，干净语义 HTML 不退化", async ({ page }) => {
  await login(page);
  await openEditor(page);

  const cases = [
    FOREIGN_BLOCK,
    '<section id="a">\n\n  <p style="margin:0">中间有空行</p>\n\n</section>',
    '<!DOCTYPE html>\n<html lang="zh">\n<body><p>x</p></body>\n</html>',
    '<script>\nwindow.__rawHtmlExecuted = true;\n</script>',
    '<iframe src="https://example.com" width="320" height="180"></iframe>',
    '前文 <span style="color:red">红字</span> 后文',
    '<video controls width="640"><source src="a.mp4" type="video/mp4"></video>',
  ];
  for (const source of cases) {
    const roundTrip = await setContent(page, source);
    expect(roundTrip.replace(/\r\n/g, "\n").trimEnd(), source.slice(0, 40)).toBe(source.trimEnd());
  }

  // 干净语义 HTML 仍走原生可编辑路径，不能被原子化。
  await setContent(page, "<table>\n<tr><td>a</td></tr>\n</table>");
  expect(await rawHtmlNodes(page)).toHaveLength(0);
  await expect(page.locator('[contenteditable="true"] table').first()).toBeVisible();

  // mark/sub/sup 由真正的 Mark 承担，渲染出效果且原样回写。
  const inline = await setContent(page, "H<sub>2</sub>O 与 <mark>高亮</mark>");
  expect(inline).toBe("H<sub>2</sub>O 与 <mark>高亮</mark>");
  expect(await rawHtmlNodes(page)).toHaveLength(0);
});

test("工具栏插入闭环：弹窗实时预览 → 插入 → 节点渲染 → 源码逐字落盘", async ({ page }) => {
  await login(page);
  await openEditor(page);

  await page.locator('button[title="插入外部 HTML"]').click();
  const source = page.locator("#raw-html-source");
  await expect(source).toBeVisible();
  await source.fill(FOREIGN_BLOCK);

  const modalFrame = page.locator('iframe[title="外部 HTML 预览"]');
  await expect(modalFrame).toBeVisible({ timeout: 20_000 });
  await expect.poll(async () => (await modalFrame.getAttribute("srcdoc"))?.length ?? 0, { timeout: 20_000 }).toBeGreaterThan(0);

  await page.getByRole("button", { name: "插入", exact: true }).click();
  await expect(source).toBeHidden();

  const node = page.locator(".raw-html-node").first();
  await expect(node).toBeVisible();
  await expect(node.locator("iframe")).toBeVisible({ timeout: 20_000 });

  expect(await rawHtmlNodes(page)).toEqual([{ type: "rawHtml", raw: FOREIGN_BLOCK }]);
  const markdown = await page.evaluate(() => window.__consoleTiptapEditor!.storage.markdown.getMarkdown());
  expect(markdown).toContain(FOREIGN_BLOCK);
});

test("起始片段一键填入并真实渲染，可继续改源码后插入", async ({ page }) => {
  await login(page);
  await openEditor(page);

  await page.locator('button[title="插入外部 HTML"]').click();
  const source = page.locator("#raw-html-source");
  await expect(source).toHaveValue("");
  await page.getByRole("button", { name: "渐变卡片", exact: true }).click();
  await expect(source).toHaveValue(/linear-gradient/);
  const modalFrame = page.locator('iframe[title="外部 HTML 预览"]');
  await expect(modalFrame).toBeVisible({ timeout: 20_000 });
  await expect.poll(async () => (await modalFrame.getAttribute("srcdoc")) ?? "", { timeout: 20_000 }).toContain("linear-gradient");

  await page.getByRole("button", { name: "样式表格", exact: true }).click();
  await expect(source).toHaveValue(/border-collapse/);
  await page.getByRole("button", { name: "插入", exact: true }).click();
  await expect(page.locator(".raw-html-node").first()).toBeVisible();
  const nodes = await rawHtmlNodes(page);
  expect(nodes[0].raw).toContain("border-collapse");
});

test("选中节点可编辑回写与删除", async ({ page }) => {
  await login(page);
  await openEditor(page);
  await setContent(page, FOREIGN_BLOCK);

  const node = page.locator(".raw-html-node").first();
  await expect(node).toBeVisible();
  await node.click();
  await page.locator('button[aria-label="编辑 HTML 源码"]').click();

  const editor = page.locator('textarea[aria-label="HTML 源码"]');
  await expect(editor).toBeVisible();
  const next = '<div class="changed">改过了</div>';
  await editor.fill(next);
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(editor).toBeHidden();

  expect(await rawHtmlNodes(page)).toEqual([{ type: "rawHtml", raw: next }]);

  await node.click();
  await page.locator('button[aria-label="删除 HTML 片段"]').click();
  await expect(page.locator(".raw-html-node")).toHaveCount(0);
  expect(await rawHtmlNodes(page)).toHaveLength(0);
});

test("外部网页粘贴保真为 atom，干净语义块仍可编辑", async ({ page }) => {
  await login(page);
  await openEditor(page);
  await setContent(page, "");

  const pasted = '<div class="pasted" style="color:blue"><span data-x="1">粘贴内容</span></div>';
  await page.evaluate((html) => {
    const transfer = new DataTransfer();
    transfer.setData("text/html", html);
    transfer.setData("text/plain", "粘贴内容");
    const target = document.querySelector('[contenteditable="true"]');
    if (!target) throw new Error("editable surface missing");
    target.dispatchEvent(new ClipboardEvent("paste", { clipboardData: transfer, bubbles: true, cancelable: true }));
  }, pasted);

  await expect.poll(async () => (await rawHtmlNodes(page)).length, { timeout: 10_000 }).toBeGreaterThan(0);
  const nodes = await rawHtmlNodes(page);
  expect(nodes[0].type).toBe("rawHtml");
  expect(nodes[0].raw).toContain('class="pasted"');
  expect(nodes[0].raw).toContain('data-x="1"');
  expect(nodes[0].raw).toContain("粘贴内容");

  // 干净语义块粘贴后仍是原生可编辑节点，不变成不透明卡片。
  await setContent(page, "");
  await page.evaluate(() => {
    const transfer = new DataTransfer();
    transfer.setData("text/html", "<p>普通 <strong>粗体</strong> 段落</p>");
    transfer.setData("text/plain", "普通 粗体 段落");
    const target = document.querySelector('[contenteditable="true"]');
    target?.dispatchEvent(new ClipboardEvent("paste", { clipboardData: transfer, bubbles: true, cancelable: true }));
  });
  await expect(page.locator('[contenteditable="true"] strong').first()).toBeVisible({ timeout: 10_000 });
  expect(await rawHtmlNodes(page)).toHaveLength(0);
});

test("390px 视口插入弹窗无横向溢出且控件可达", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  await openEditor(page);

  await page.locator('button[title="插入外部 HTML"]').click();
  await expect(page.locator("#raw-html-source")).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(1);
  await expect(page.getByRole("button", { name: "插入", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "取消", exact: true })).toBeVisible();
});

test("弹窗与节点预览高度随内容自适应，不停在初始高度", async ({ page }) => {
  await login(page);
  await openEditor(page);

  const tall = '<div class="tall-embed" style="height:600px;background:linear-gradient(180deg,#e8e8ec,#5e5ce6)">高片段</div>';
  await page.locator('button[title="插入外部 HTML"]').click();
  await page.locator("#raw-html-source").fill(tall);
  const modalFrame = page.locator('iframe[title="外部 HTML 预览"]');
  await expect(modalFrame).toBeVisible({ timeout: 20_000 });
  const frameHeight = () =>
    modalFrame.evaluate((el) => Number((el.getAttribute("style") || "").match(/height:\s*(\d+)px/)?.[1] ?? 0));
  await expect.poll(frameHeight, { timeout: 20_000 }).toBeGreaterThan(400);

  await page.getByRole("button", { name: "插入", exact: true }).click();
  const nodeFrame = page.locator(".raw-html-node iframe").first();
  await expect(nodeFrame).toBeVisible({ timeout: 20_000 });
  await expect
    .poll(() => nodeFrame.evaluate((el) => Number((el.getAttribute("style") || "").match(/height:\s*(\d+)px/)?.[1] ?? 0)), { timeout: 20_000 })
    .toBeGreaterThan(400);
});

test("富文本 ↔ Markdown 双模式切换，外部 HTML 逐字不变", async ({ page }) => {
  await login(page);
  await openEditor(page);
  await page.getByPlaceholder("文章标题").fill("外部 HTML 双模式保真");
  // 必须走真实插入流程：编辑器钩子 setContent 不回写宿主表单，保存会丢内容。
  await page.locator('button[title="插入外部 HTML"]').click();
  await page.locator("#raw-html-source").fill(FOREIGN_BLOCK);
  await page.getByRole("button", { name: "插入", exact: true }).click();
  await expect(page.locator(".raw-html-node").first()).toBeVisible();

  await page.getByRole("button", { name: "保存草稿", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/posts\/edit\/[^/]+$/, { timeout: 15_000 });
  const slug = page.url().split("/").pop()!;

  const saved = await (await page.request.get(`/api/v1/posts/${slug}`)).json();
  expect(saved.content_md).toContain(FOREIGN_BLOCK);

  // 切到 Markdown 模式：textarea 里必须是逐字原文（表单内容异步加载，需轮询等待）。
  await page.addInitScript(() => localStorage.setItem("post-editor:kind", "markdown"));
  await page.reload();
  const markdownArea = page.locator('textarea[placeholder^="Markdown 内容"]');
  await expect(markdownArea).toBeVisible({ timeout: 15_000 });
  await expect.poll(async () => markdownArea.inputValue(), { timeout: 15_000 }).toContain(FOREIGN_BLOCK);

  // 切回富文本：仍是同一个 atom，raw 逐字不变。
  await page.addInitScript(() => localStorage.setItem("post-editor:kind", "rich"));
  await page.reload();
  await page.locator('[contenteditable="true"]').first().waitFor({ state: "visible", timeout: 15_000 });
  await page.waitForFunction(() => Boolean(window.__consoleTiptapEditor), undefined, { timeout: 15_000 });
  expect(await rawHtmlNodes(page)).toEqual([{ type: "rawHtml", raw: FOREIGN_BLOCK }]);
});

test("发布后外部 HTML 同源直出，内嵌脚本真实执行", async ({ page }) => {
  await login(page);
  await openEditor(page);
  const title = `外部 HTML 发布验收 ${Date.now()}`;
  await page.getByPlaceholder("文章标题").fill(title);

  const payload =
    '<div class="ext-embed" style="border:1px solid red">外部 HTML 验收标记</div>\n' +
    "<script>window.__rawHtmlExecuted = true;</script>";
  await page.locator('button[title="插入外部 HTML"]').click();
  await page.locator("#raw-html-source").fill(payload);
  await page.getByRole("button", { name: "插入", exact: true }).click();
  await expect(page.locator(".raw-html-node").first()).toBeVisible();

  await page.getByRole("button", { name: "保存草稿", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/posts\/edit\/[^/]+$/, { timeout: 15_000 });
  const slug = page.url().split("/").pop()!;
  await page.getByRole("button", { name: "发布", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/posts$/, { timeout: 15_000 });

  const saved = await (await page.request.get(`/api/v1/posts/${slug}`)).json();
  expect(saved.status).toBe("published");
  expect(saved.content_md).toContain("ext-embed");
  expect(saved.content_md).toContain("__rawHtmlExecuted");

  await page.goto(`/post/${slug}`);
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText("外部 HTML 验收标记", { exact: true })).toBeVisible();
  await expect(page.locator(".ext-embed")).toHaveAttribute("style", /border/);
  expect(await page.evaluate(() => (window as any).__rawHtmlExecuted)).toBe(true);
});