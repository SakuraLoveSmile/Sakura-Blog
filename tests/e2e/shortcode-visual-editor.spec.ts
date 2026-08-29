import { expect, test, type Page } from "@playwright/test";

// 全 catalog 可视化属性编辑的关键流程（计划 §11.5）：
// 只挑有代表性的 6 类标签跑浏览器 E2E，其余 52 个由
// internal/post/shortcode_edit_test.go 的 58 条 fixture + 渲染 round-trip 机械覆盖。
//   1. note   普通 inline（named + positional + content）
//   2. tabs   marker 块级（正文结构保留，不做子编辑器）
//   3. navbar hybrid：块级 / 单行都要保持 variant
//   4. frame  hybrid + 两套字段（块级 device/width/content，单行 device/img/video/focus/alt）
//   5. emoji  特殊 inline（source/name/url 两种模式）
//   6. links  零参数（不再误判 unsupported）

type EditorHook = {
  commands: {
    setContent: (value: string, emitUpdate?: boolean) => void;
    setNodeSelection: (position: number) => void;
  };
  state: { doc: { descendants: (fn: (node: { type: { name: string } }, pos: number) => void) => void } };
  storage: { markdown: { getMarkdown: () => string } };
};

async function setMarkdown(page: Page, markdown: string) {
  await page.evaluate((md) => {
    const editor = (window as Window & { __consoleTiptapEditor?: EditorHook }).__consoleTiptapEditor;
    if (!editor) throw new Error("development TipTap editor hook is unavailable");
    editor.commands.setContent(md, false);
  }, markdown);
  await expect(page.locator(".stellar-tag-node").first()).toBeVisible({ timeout: 15_000 });
}

const getMarkdown = (page: Page) =>
  page.evaluate(() => {
    const editor = (window as Window & { __consoleTiptapEditor?: EditorHook }).__consoleTiptapEditor;
    if (!editor) throw new Error("development TipTap editor hook is unavailable");
    return editor.storage.markdown.getMarkdown();
  });

/** 选中唯一的 Stellar 标签节点：先走真实点击，失败再用编辑器命令兜底。 */
async function selectTagNode(page: Page) {
  await page.locator(".stellar-tag-node").first().click({ force: true });
  const bubble = page.locator(".stellar-bubble");
  if (await bubble.isVisible().catch(() => false)) return;
  await page.evaluate(() => {
    const editor = (window as Window & { __consoleTiptapEditor?: EditorHook }).__consoleTiptapEditor;
    if (!editor) throw new Error("development TipTap editor hook is unavailable");
    let found = -1;
    editor.state.doc.descendants((node, pos) => {
      if (found < 0 && node.type.name === "stellarTag") found = pos;
    });
    if (found < 0) throw new Error("stellarTag node not found");
    editor.commands.setNodeSelection(found);
  });
}

async function openPropertyPanel(page: Page) {
  await selectTagNode(page);
  const bubble = page.locator(".stellar-bubble");
  await expect(bubble).toBeVisible({ timeout: 10_000 });
  await bubble.getByRole("button", { name: "编辑属性" }).click();
  const dialog = page.getByRole("dialog", { name: "编辑组件属性" });
  await expect(dialog).toBeVisible({ timeout: 10_000 });
  // 等解析完成后表单出现（loading 态下不渲染字段）
  await expect(dialog.locator("text=正在读取属性")).toHaveCount(0, { timeout: 15_000 });
  return dialog;
}

/** 属性面板改值后的 raw 回写有 200ms 防抖，用轮询断言而非固定 sleep。 */
async function expectMarkdownContains(page: Page, needle: string) {
  await expect
    .poll(async () => (await getMarkdown(page)).replace(/\r\n/g, "\n"), { timeout: 15_000 })
    .toContain(needle);
}

async function expectPreviewContains(page: Page, needle: string) {
  await expect
    .poll(async () => (await page.locator(".stellar-tag-node iframe").first().getAttribute("srcdoc")) ?? "", {
      timeout: 20_000,
    })
    .toContain(needle);
}

test.beforeEach(async ({ page }) => {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
  await page.goto("/console/posts/new");
  await page.locator('[contenteditable="true"]').first().waitFor({ state: "visible", timeout: 15_000 });
});

test("note（普通 inline）：初始值回填 → 改标题 → raw 与预览同步更新", async ({ page }) => {
  await setMarkdown(page, "{% note color:green 提示 这是一条提示 %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-color")).toHaveValue("green");
  await expect(dialog.locator("#schema-field-title")).toHaveValue("提示");
  await expect(dialog.locator("#schema-field-content")).toHaveValue("这是一条提示");

  await dialog.locator("#schema-field-title").fill("新标题");
  // builder 规则：positional 先于 named，所以 color:green 被移到标题之后。
  // 二者渲染等价（Go 侧 TestShortcodeEditRoundTripKeepsRender 已断言），此处锁定实际输出防回归。
  await expectMarkdownContains(page, "{% note 新标题 color:green 这是一条提示 %}");
  await expectPreviewContains(page, "新标题");
});

test("tabs（marker 块级）：正文结构原样保留，只改外层属性", async ({ page }) => {
  await setMarkdown(
    page,
    "{% tabs active:1 %}\n<!-- tab 标签一 -->\n第一页内容。\n<!-- tab 标签二 -->\n第二页内容。\n{% endtabs %}",
  );
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-active")).toHaveValue("1");
  const body = await dialog.locator("#schema-field-content").inputValue();
  expect(body).toContain("<!-- tab 标签一 -->");
  expect(body).toContain("<!-- tab 标签二 -->");

  await dialog.locator("#schema-field-active").fill("2");
  await expectMarkdownContains(page, "active:2");
  // marker 正文不能被属性面板改写掉
  await expectMarkdownContains(page, "<!-- tab 标签一 -->");
  await expectMarkdownContains(page, "<!-- tab 标签二 -->");
  await expectPreviewContains(page, "第二页内容。");
});

test("navbar（hybrid 块级）：改 active 后仍保持块级包装", async ({ page }) => {
  await setMarkdown(page, "{% navbar active:首页 %}\n[首页](/)\n[关于](/about)\n{% endnavbar %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-active")).toHaveValue("首页");
  await expect(dialog.locator("#schema-field-content")).toHaveValue("[首页](/)\n[关于](/about)");

  await dialog.locator("#schema-field-active").fill("关于");
  await expectMarkdownContains(page, "{% navbar active:关于 %}");
  await expectMarkdownContains(page, "{% endnavbar %}");
  await expectPreviewContains(page, "关于");
});

test("navbar（hybrid 单行）：改 active 后仍保持单行，不被改写成块级", async ({ page }) => {
  await setMarkdown(page, "{% navbar active:/ [首页](/) %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-active")).toHaveValue("/");

  await dialog.locator("#schema-field-active").fill("/about");
  await expectMarkdownContains(page, "{% navbar active:/about [首页](/) %}");
  const markdown = await getMarkdown(page);
  expect(markdown).not.toContain("{% endnavbar %}");
});

test("frame（hybrid 块级）：device/width/content 三字段", async ({ page }) => {
  await setMarkdown(
    page,
    "{% frame device:iphone width:320px %}\n![截图](/assets/img/preview/frame-shot.jpg)\n{% endframe %}",
  );
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-device")).toHaveValue("iphone");
  await expect(dialog.locator("#schema-field-width")).toHaveValue("320px");

  await dialog.locator("#schema-field-width").fill("480px");
  await expectMarkdownContains(page, "width:480px");
  await expectMarkdownContains(page, "{% endframe %}");
});

test("frame（hybrid 单行）：切到 inline 专属五字段且不带 content", async ({ page }) => {
  await setMarkdown(page, "{% frame iphone 手机截图 img:/assets/img/preview/frame-shot.jpg %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-device")).toHaveValue("iphone");
  await expect(dialog.locator("#schema-field-alt")).toHaveValue("手机截图");
  await expect(dialog.locator("#schema-field-img")).toHaveValue("/assets/img/preview/frame-shot.jpg");
  await expect(dialog.locator("#schema-field-focus")).toHaveValue("");
  await expect(dialog.locator("#schema-field-content")).toHaveCount(0);

  await dialog.locator("#schema-field-focus").fill("top");
  await expectMarkdownContains(page, "focus:top");
  const markdown = await getMarkdown(page);
  expect(markdown).not.toContain("{% endframe %}");
});

test("emoji（特殊 inline）：source/name 模式字段回填与改值", async ({ page }) => {
  await setMarkdown(page, "{% emoji twemoji 1f44d height:20px %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-source")).toHaveValue("twemoji");
  await expect(dialog.locator("#schema-field-name")).toHaveValue("1f44d");
  await expect(dialog.locator("#schema-field-height")).toHaveValue("20px");
  await expect(dialog.locator("#schema-field-url")).toHaveValue("");

  await dialog.locator("#schema-field-height").fill("32px");
  await expectMarkdownContains(page, "{% emoji twemoji 1f44d height:32px %}");
});

test("emoji URL 模式：修改其他字段后保留外链与 alt", async ({ page }) => {
  await setMarkdown(page, "{% emoji url:https://example.com/e.png name:原表情 height:20px %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-url")).toHaveValue("https://example.com/e.png");
  await expect(dialog.locator("#schema-field-name")).toHaveValue("原表情");
  await dialog.locator("#schema-field-height").fill("32px");
  await expectMarkdownContains(page, "{% emoji url:https://example.com/e.png name:原表情 height:32px %}");
  const markdown = await getMarkdown(page);
  expect(markdown).not.toContain("emoji twemoji");
});

test("结构化编辑校验：清空必填字段时禁用应用且不提交无效 raw", async ({ page }) => {
  await setMarkdown(page, "{% button 访问主页 https://example.com %}");
  const dialog = await openPropertyPanel(page);
  const original = await getMarkdown(page);

  await dialog.locator("#schema-field-url").fill("");
  await expect(dialog.getByRole("button", { name: "应用" })).toBeDisabled();
  await expect(dialog.getByText(/必填参数/)).toBeVisible();
  await dialog.getByRole("button", { name: "取消" }).click();
  expect(await getMarkdown(page)).toBe(original);
});

test("结构化编辑回退：含单双引号的参数不提交并提示源码编辑", async ({ page }) => {
  await setMarkdown(page, "{% button 访问主页 https://example.com %}");
  const dialog = await openPropertyPanel(page);
  const original = await getMarkdown(page);

  await dialog.locator("#schema-field-text").fill("含 \"双引号\" 与 '单引号'");
  await expect(dialog.getByRole("button", { name: "应用" })).toBeDisabled();
  await expect(dialog.getByText(/单双引号/)).toBeVisible();
  await dialog.getByRole("button", { name: "应用" }).click({ force: true });
  expect(await getMarkdown(page)).toBe(original);
});

test("links（零参数）：面板提示无属性但保留源码编辑", async ({ page }) => {
  await setMarkdown(page, "{% links %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.getByText("该组件没有可编辑属性")).toBeVisible();
  // 源码编辑通道仍在
  await expect(dialog.getByRole("button", { name: "源码编辑" })).toBeVisible();
  await expectMarkdownContains(page, "{% links %}");
});

test("编辑已有组件不会注入 catalog default（folding 未写的 open 不落盘）", async ({ page }) => {
  await setMarkdown(page, "{% folding color:cyan 标题 %}\n折叠正文\n{% endfolding %}");
  const dialog = await openPropertyPanel(page);

  // open 未在 raw 中出现，开关必须是未勾选状态（catalog default 为 true）
  await expect(dialog.locator("#schema-field-open")).not.toBeChecked();
  await expect(dialog.locator("#schema-field-color")).toHaveValue("cyan");

  // 只改标题，flag 仍不应被写进 shortcode（同上：positional 先于 named）
  await dialog.locator("#schema-field-title").fill("新折叠标题");
  await expectMarkdownContains(page, "{% folding 新折叠标题 color:cyan %}");
  const markdown = await getMarkdown(page);
  expect(markdown).not.toContain("open");
  await expectPreviewContains(page, "新折叠标题");
});

test("flag 统一输出 key:true（image download 勾选后回写）", async ({ page }) => {
  await setMarkdown(page, "{% image /assets/img/preview/image-demo.jpg 示例图 %}");
  const dialog = await openPropertyPanel(page);

  await expect(dialog.locator("#schema-field-download")).not.toBeChecked();
  await dialog.locator("#schema-field-download").check();
  await expectMarkdownContains(page, "download:true");
});
