import { expect, test, type Page } from "@playwright/test";

// 真实隔离服务建立夹具；门控用例只模拟保存响应，用于验收组件并发行为。
async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
}
async function createPost(page: Page, status = "draft") {
  const slug = `writing-state-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const result = await page.request.post("/api/v1/posts", {
    data: { slug, title: "写作状态夹具", content_md: "原始正文", status },
  });
  expect(result.ok()).toBeTruthy();
  await page.goto(`/console/posts/edit/${slug}`);
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue("写作状态夹具");
  await expect(page.locator('[contenteditable="true"]').first()).toBeVisible();
  return slug;
}
const status = (page: Page) => page.locator(".editor-status-bar [role='status']");

// 同一浏览器事件中输入并操作，明确覆盖富文本 200ms 防抖窗口。
async function inputAndClick(page: Page, content: string, selector: string) {
  await page.evaluate(
    ({ content, selector }) => {
      const editor = (
        window as Window & {
          __consoleTiptapEditor?: { commands: { setContent: (content: string, emitUpdate: boolean) => void } };
        }
      ).__consoleTiptapEditor;
      if (!editor) throw new Error("TipTap hook unavailable");
      editor.commands.setContent(content, true);
      const button = document.querySelector<HTMLButtonElement>(selector);
      if (!button) throw new Error(`Missing action ${selector}`);
      button.click();
    },
    { content, selector },
  );
}

test.beforeEach(async ({ page }) => login(page));

test("立即输入后保存、预览与切换编辑器包含最新正文；元数据触发离开保护", async ({ page }) => {
  const slug = await createPost(page);
  await expect(status(page)).toHaveText("所有修改已保存");
  const saved = page.waitForResponse(
    (r) => r.url().endsWith(`/api/v1/posts/${slug}`) && r.request().method() === "PUT",
  );
  await inputAndClick(page, "立即保存的最新正文", '[aria-label="保存草稿"]');
  expect((await saved).ok()).toBeTruthy();
  await expect(status(page)).toContainText("已保存");
  const persisted = await page.request.get(`/api/v1/posts/${slug}`);
  expect((await persisted.json()).content_md).toContain("立即保存的最新正文");

  const preview = page.waitForRequest((r) => r.url().endsWith("/api/v1/preview/post"));
  await inputAndClick(page, "立即预览的最新正文", '[aria-label="预览文章"]');
  expect((await preview).postDataJSON().content_md).toContain("立即预览的最新正文");

  await page.getByRole("button", { name: "切换编辑器" }).click();
  await inputAndClick(page, "立即切换保留的最新正文", "#post-editor-kind-markdown");
  await expect(page.locator("textarea").first()).toHaveValue("立即切换保留的最新正文");
  await page.getByLabel("文章标题", { exact: true }).fill("元数据也尚未保存");
  await expect(status(page)).toHaveText("有未保存的修改");
  await page.getByRole("button", { name: "返回文章列表" }).click();
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toContainText("有未保存的修改");
  await dialog.getByRole("button", { name: "继续编辑" }).click();
  await expect(page).toHaveURL(new RegExp(`/posts/edit/${slug}$`));
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue("元数据也尚未保存");
  await page.getByRole("button", { name: "返回文章列表" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "离开页面" }).click();
  await expect(page).toHaveURL(/\/console\/posts$/);
});

test("保存期间继续编辑不会误标已保存或清除新备份，保存失败保留修改", async ({ page }) => {
  const slug = await createPost(page);
  const bodies: Array<Record<string, unknown>> = [];
  let release: ((status: number) => void) | undefined;
  await page.route(`**/api/v1/posts/${slug}`, async (route) => {
    if (route.request().method() !== "PUT") return route.fallback();
    bodies.push(route.request().postDataJSON());
    const code = await new Promise<number>((resolve) => {
      release = resolve;
    });
    await route.fulfill({ status: code, json: code === 200 ? { ok: true } : { error: "门控保存失败" } });
  });
  await page.getByLabel("文章标题", { exact: true }).fill("本次提交标题");
  await inputAndClick(page, "本次提交正文", '[aria-label="保存草稿"]');
  await expect.poll(() => bodies.length).toBe(1);
  expect(bodies[0].content_md).toContain("本次提交正文");
  await expect(page.getByRole("button", { name: "保存草稿", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "发布", exact: true })).toBeDisabled();
  await page.getByLabel("文章标题", { exact: true }).fill("请求期间继续写的标题");
  await page.locator('[contenteditable="true"]').first().fill("请求期间继续写的正文");
  await page.keyboard.press("ControlOrMeta+s");
  expect(bodies.length).toBe(1);
  release!(200);
  await expect(status(page)).toHaveText("有未保存的修改");
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue("请求期间继续写的标题");
  await expect
    .poll(() => page.evaluate((slug) => localStorage.getItem(`draft:post:${slug}`), slug))
    .toContain("请求期间继续写的正文");
  await page.getByRole("button", { name: "保存草稿", exact: true }).click();
  await expect.poll(() => bodies.length).toBe(2);
  expect(bodies[1].title).toBe("请求期间继续写的标题");
  release!(500);
  await expect(status(page)).toHaveText("保存失败，修改尚未保存");
  await expect(page.locator('[contenteditable="true"]').first()).toContainText("请求期间继续写的正文");
  await expect
    .poll(() => page.evaluate((slug) => localStorage.getItem(`draft:post:${slug}`), slug))
    .toContain("请求期间继续写的正文");
});

test("已发布文章保存仍只写草稿头，不改变线上内容", async ({ page }) => {
  const slug = await createPost(page, "published");
  const saved = page.waitForResponse(
    (r) => r.url().endsWith(`/api/v1/posts/${slug}/draft`) && r.request().method() === "PUT",
  );
  await inputAndClick(page, "尚未发布的新正文", '[aria-label="保存草稿"]');
  expect((await saved).ok()).toBeTruthy();
  await expect(status(page)).toContainText("尚未发布");
  const result = await page.request.get(`/api/v1/posts/${slug}`);
  const article = await result.json();
  expect(article.content_md).toBe("原始正文");
  expect(article.draft.content_md).toContain("尚未发布的新正文");
});

test("手机初始专注正文，设置默认关闭且可关闭/Escape，编辑器可切换", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/console/posts/new");
  await expect(page.getByRole("dialog", { name: "文章设置" })).toHaveCount(0);
  await expect(page.getByLabel("文章标题", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "切换编辑器" })).toBeVisible();
  await page.getByRole("button", { name: "文章设置", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "文章设置" })).toBeVisible();
  await expect(
    page.getByRole("dialog", { name: "文章设置" }).getByRole("button", { name: "设置", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "关闭文章设置" }).click();
  await expect(page.getByRole("dialog", { name: "文章设置" })).toHaveCount(0);
  await page.getByRole("button", { name: "文章设置", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "文章设置" })).toHaveCount(0);
  await page.getByRole("button", { name: "切换编辑器" }).click();
  await page.getByRole("option", { name: "Markdown 编辑器" }).click();
  await expect(page.locator("textarea").first()).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test("创建期间继续输入会保留编辑实例，后续保存更新同一篇文章", async ({ page }) => {
  await page.goto("/console/posts/new");
  await expect(page.locator('[contenteditable="true"]').first()).toBeVisible();
  let release: (() => void) | undefined;
  let createdSlug = "";
  let creates = 0;
  await page.route("**/api/v1/posts", async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    creates++;
    const response = await route.fetch();
    expect(response.ok()).toBeTruthy();
    createdSlug = (await response.json()).slug;
    await new Promise<void>((resolve) => {
      release = resolve;
    });
    await route.fulfill({ response });
  });
  await page.getByLabel("文章标题", { exact: true }).fill("首次创建标题");
  await inputAndClick(page, "首次创建正文", '[aria-label="保存草稿"]');
  await expect.poll(() => !!release).toBe(true);
  await page.getByLabel("文章标题", { exact: true }).fill("创建期间的新标题");
  await page.locator('[contenteditable="true"]').first().fill("创建期间的新正文");
  release!();
  await expect(status(page)).toHaveText("有未保存的修改");
  await expect(page).toHaveURL(/\/console\/posts\/new$/);
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue("创建期间的新标题");
  await expect
    .poll(() => page.evaluate((slug) => localStorage.getItem(`draft:post:${slug}`), createdSlug))
    .toContain("创建期间的新正文");
  expect(await page.evaluate(() => localStorage.getItem("draft:post:new"))).toBeNull();
  await page.getByRole("button", { name: "返回文章列表" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "离开页面" }).click();
  await expect(page).toHaveURL(/\/console\/posts$/);
  await page.goto("/console/posts/new");
  await expect(page.getByText("检测到本地未保存草稿，是否恢复？")).toHaveCount(0);
  await expect(page.locator('[contenteditable="true"]').first()).toHaveText("");
  await page.goto(`/console/posts/edit/${encodeURIComponent(createdSlug)}`);
  await expect(page.getByText("检测到本地未保存草稿，是否恢复？")).toBeVisible();
  await page.getByRole("button", { name: "恢复", exact: true }).click();
  await expect(page.locator('[contenteditable="true"]').first()).toContainText("创建期间的新正文");
  await page.getByLabel("文章标题", { exact: true }).fill("创建期间的新标题");
  await page.getByRole("button", { name: "保存草稿", exact: true }).click();
  await expect(status(page)).toContainText("已保存");
  await expect(page).toHaveURL(new RegExp(`/posts/edit/${encodeURIComponent(createdSlug)}$`));
  expect(creates).toBe(1);
  const article = await (await page.request.get(`/api/v1/posts/${createdSlug}`)).json();
  expect(article.title).toBe("创建期间的新标题");
  expect(article.content_md).toContain("创建期间的新正文");
  expect(await page.evaluate((slug) => localStorage.getItem(`draft:post:${slug}`), createdSlug)).toBeNull();
});

test("发布期间继续输入不离开编辑页，刷新保护先同步最新正文备份", async ({ page }) => {
  const slug = await createPost(page);
  let release: (() => void) | undefined;
  await page.route(`**/api/v1/posts/${slug}/publish`, async (route) => {
    const response = await route.fetch();
    expect(response.ok()).toBeTruthy();
    await new Promise<void>((resolve) => {
      release = resolve;
    });
    await route.fulfill({ response });
  });
  await inputAndClick(page, "本次发布正文", ".editor-header .btn-primary");
  await expect.poll(() => !!release).toBe(true);
  await page.locator('[contenteditable="true"]').first().fill("发布期间继续输入的正文");
  release!();
  await expect(status(page)).toHaveText("有未保存的修改");
  await expect(page).toHaveURL(new RegExp(`/posts/edit/${slug}$`));
  const article = await (await page.request.get(`/api/v1/posts/${slug}`)).json();
  expect(article.status).toBe("published");
  expect(article.content_md).toContain("本次发布正文");
  const unloadResult = await page.evaluate((slug) => {
    const editor = (
      window as Window & {
        __consoleTiptapEditor?: { commands: { setContent: (text: string, update: boolean) => void } };
      }
    ).__consoleTiptapEditor;
    editor!.commands.setContent("刷新前最后一次输入", true);
    const event = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(event);
    return { prevented: event.defaultPrevented, draft: localStorage.getItem(`draft:post:${slug}`) };
  }, slug);
  expect(unloadResult.prevented).toBe(true);
  expect(unloadResult.draft).toContain("刷新前最后一次输入");
});

test("同组件切换文章时迟到的加载响应不能覆盖当前文章", async ({ page }) => {
  const first = await createPost(page);
  const second = `late-load-${Date.now()}`;
  expect(
    (
      await page.request.post("/api/v1/posts", {
        data: { slug: second, title: "迟到文章标题", content_md: "迟到文章正文", status: "draft" },
      })
    ).ok(),
  ).toBeTruthy();
  let release: (() => void) | undefined;
  await page.route(`**/api/v1/posts/${second}`, async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const response = await route.fetch();
    await new Promise<void>((resolve) => {
      release = resolve;
    });
    await route.fulfill({ response });
  });
  const navigate = async (slug: string) =>
    page.evaluate((slug) => {
      const app = document.querySelector("#app") as HTMLElement & {
        __vue_app__: { config: { globalProperties: { $router: { push: (path: string) => Promise<unknown> } } } };
      };
      return app.__vue_app__.config.globalProperties.$router.push(`/posts/edit/${slug}`);
    }, slug);
  await navigate(second);
  await expect.poll(() => !!release).toBe(true);
  await navigate(first);
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue("写作状态夹具");
  const lateResponse = page.waitForResponse((response) => response.url().endsWith(`/api/v1/posts/${second}`));
  release!();
  await lateResponse;
  await page.unroute(`**/api/v1/posts/${second}`);
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue("写作状态夹具");
  await expect(page.locator('[contenteditable="true"]').first()).toContainText("原始正文");
  await page.getByLabel("文章标题", { exact: true }).fill("当前文章新的标题");
  const saved = page.waitForResponse(
    (response) => response.url().endsWith(`/api/v1/posts/${first}`) && response.request().method() === "PUT",
  );
  await page.getByRole("button", { name: "保存草稿", exact: true }).click();
  expect((await saved).request().postDataJSON().title).toBe("当前文章新的标题");
  const untouched = await (await page.request.get(`/api/v1/posts/${second}`)).json();
  expect(untouched.title).toBe("迟到文章标题");
});
