import { expect, test, type Page } from "@playwright/test";

// 文章编辑页布局改造专项：页头状态、工具栏右侧入口、断点、焦点与移动发布按钮可读性。
// 设置业务行为仍由既有专项用例覆盖，这里不重复弱 smoke。

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/);
}

async function createPost(page: Page, title = "文章编辑页 UI 夹具") {
  const slug = `editor-ui-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const result = await page.request.post("/api/v1/posts", {
    data: {
      slug,
      title,
      content_md: "## 正文编辑区\n\n这是用于布局与焦点验收的固定正文。\n\n## 下一段内容\n\n保持内容密度稳定。",
      status: "draft",
      stellar_version: 2,
      stellar_config: "{}",
      blog_extensions: "{}",
    },
  });
  expect(result.ok()).toBeTruthy();
  await page.goto(`/console/posts/edit/${slug}`);
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue(title);
  await expect(page.locator('[contenteditable="true"]').first()).toBeVisible();
  return slug;
}

test.beforeEach(async ({ page }) => login(page));

test("保存状态并入页头且写作面不再有三层卡片", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await createPost(page);

  await expect(page.locator(".editor-save-status")).toHaveCount(1);
  await expect(page.locator(".editor-status-bar")).toHaveCount(0);
  await expect(page.locator(".editor-header .editor-save-status")).toBeVisible();
  await expect(page.locator(".editor-header [role='status'][aria-live='polite']")).toHaveCount(1);

  const layout = await page.evaluate(() => {
    const surface = document.querySelector(".post-writing-surface");
    const title = document.querySelector(".post-title-block");
    const toolbarShell = document.querySelector(".post-toolbar-shell");
    const body = document.querySelector(".post-editor-body");
    const instance = document.querySelector(".post-editor-instance");
    const toolbarEnd = document.querySelector(".editor-toolbar-end");
    const scroller = document.querySelector(".editor-toolbar-scroll");
    const styles = (el: Element | null) => {
      if (!el) return null;
      const s = getComputedStyle(el);
      return {
        background: s.backgroundColor,
        borderTopWidth: s.borderTopWidth,
        borderBottomWidth: s.borderBottomWidth,
        borderLeftWidth: s.borderLeftWidth,
        borderRadius: s.borderRadius,
        overflow: s.overflow,
      };
    };
    return {
      surface: styles(surface),
      title: styles(title),
      toolbarShell: styles(toolbarShell),
      body: styles(body),
      instance: styles(instance),
      endInsideScroller: toolbarEnd ? !!toolbarEnd.closest(".editor-toolbar-scroll") : null,
      scrollOverflowX: scroller ? getComputedStyle(scroller).overflowX : null,
      kindInEnd: !!toolbarEnd?.querySelector(".editor-kind-picker"),
      sidebarToggleInEnd: !!toolbarEnd?.querySelector(".editor-sidebar-toggle"),
    };
  });

  expect(layout.surface?.overflow).not.toBe("hidden");
  expect(layout.title?.background).toBe("rgba(0, 0, 0, 0)");
  expect(layout.title?.borderTopWidth).toBe("0px");
  expect(layout.title?.borderLeftWidth).toBe("0px");
  expect(layout.title?.borderRadius).toBe("0px");
  expect(layout.body?.background).toBe("rgba(0, 0, 0, 0)");
  expect(layout.body?.borderTopWidth).toBe("0px");
  expect(layout.body?.borderLeftWidth).toBe("0px");
  expect(layout.body?.borderRadius).toBe("0px");
  expect(layout.instance?.background).toBe("rgba(0, 0, 0, 0)");
  expect(layout.instance?.borderRadius).toBe("0px");
  expect(layout.toolbarShell?.borderTopWidth).toBe("1px");
  expect(layout.toolbarShell?.borderBottomWidth).toBe("1px");
  expect(layout.endInsideScroller).toBe(false);
  expect(layout.scrollOverflowX).toBe("auto");
  expect(layout.kindInEnd).toBe(true);
  expect(layout.sidebarToggleInEnd).toBe(true);
});

test("类型菜单位于工具栏右侧且键盘、Escape 与外部点击行为不变", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await createPost(page);

  const trigger = page.getByRole("button", { name: "切换编辑器" });
  await expect(trigger).toBeVisible();
  await trigger.focus();
  await page.keyboard.press("ArrowDown");
  const menu = page.getByRole("listbox", { name: "编辑器类型" });
  await expect(menu).toBeVisible();

  const box = await menu.boundingBox();
  const viewport = page.viewportSize()!;
  expect(box).not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(viewport.width);
  expect(box!.y + box!.height).toBeLessThanOrEqual(viewport.height);
  expect(await menu.evaluate((el) => !!el.closest(".editor-toolbar-scroll"))).toBe(false);

  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(page.locator("textarea").first()).toBeVisible();
  await expect(trigger).toBeFocused();

  await trigger.click();
  await expect(menu).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
  await expect(trigger).toBeFocused();

  await trigger.click();
  await expect(menu).toBeVisible();
  await page.getByLabel("文章标题", { exact: true }).click();
  await expect(menu).toHaveCount(0);
});

test("Rich 立即切 Markdown 保留最新正文，三态、全屏与侧栏持久化正常", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await createPost(page);

  const trigger = page.getByRole("button", { name: "切换编辑器" });
  await trigger.click();
  await expect(page.getByRole("listbox", { name: "编辑器类型" })).toBeVisible();
  await page.evaluate(() => {
    const editor = (
      window as Window & {
        __consoleTiptapEditor?: { commands: { setContent: (content: string, emitUpdate: boolean) => void } };
      }
    ).__consoleTiptapEditor;
    if (!editor) throw new Error("TipTap hook unavailable");
    editor.commands.setContent("立即切换保留的最新正文", true);
    document.querySelector<HTMLButtonElement>("#post-editor-kind-markdown")?.click();
  });
  await expect(page.locator("textarea").first()).toHaveValue(/立即切换保留的最新正文/);

  await page.getByTitle("仅预览").click();
  await expect(page.locator(".editor-header select")).toHaveValue("preview");
  await page.getByTitle("仅编辑").click();
  await expect(page.locator(".editor-header select")).toHaveValue("edit");

  await page.getByTitle("全屏编辑").click();
  await expect
    .poll(() =>
      page.evaluate(() => !!document.querySelector(".editor-header")?.parentElement?.className.includes("fixed")),
    )
    .toBe(true);
  await page.getByTitle("退出全屏").click();
  await expect
    .poll(() =>
      page.evaluate(() => !!document.querySelector(".editor-header")?.parentElement?.className.includes("fixed")),
    )
    .toBe(false);

  await expect(page.locator("#post-settings-panel")).toBeVisible();
  await page.locator(".editor-sidebar-toggle").click();
  await expect(page.locator("#post-settings-panel")).toBeHidden();
  await page.reload();
  await expect(page.getByLabel("文章标题", { exact: true })).toHaveValue("文章编辑页 UI 夹具");
  await expect(page.locator("#post-settings-panel")).toBeHidden();
  await page.locator(".editor-sidebar-toggle").click();
  await expect(page.locator("#post-settings-panel")).toBeVisible();
});

test("断点、无横向溢出与移动设置抽屉焦点行为", async ({ page }) => {
  await createPost(page);

  for (const width of [390, 639, 640, 768, 1023, 1024, 1280, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.waitForTimeout(120);
    const state = await page.evaluate(() => {
      const visible = (selector: string) => {
        const el = document.querySelector<HTMLElement>(selector);
        return !!el && getComputedStyle(el).display !== "none" && el.getBoundingClientRect().width > 0;
      };
      return {
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
        mobileSettings: visible(".editor-mobile-settings"),
        desktopToggle: visible(".editor-sidebar-toggle"),
        desktopPanel: visible("#post-settings-panel"),
      };
    });
    expect(state.scrollWidth, `width ${width} 不应横向溢出`).toBeLessThanOrEqual(state.clientWidth);
    expect(state.mobileSettings, `width ${width} 移动设置入口`).toBe(width < 640);
    expect(state.desktopToggle, `width ${width} 桌面侧栏开关`).toBe(width >= 640);
    expect(state.desktopPanel, `width ${width} 桌面设置面板`).toBe(width >= 640);
  }

  await page.setViewportSize({ width: 768, height: 900 });
  await page.waitForTimeout(120);
  const tablet = await page.evaluate(() => {
    const writing = document.querySelector(".post-writing-surface")?.getBoundingClientRect();
    const panel = document.querySelector("#post-settings-panel")?.getBoundingClientRect();
    return writing && panel ? { writingBottom: writing.bottom, panelTop: panel.top } : null;
  });
  expect(tablet).not.toBeNull();
  expect(tablet!.panelTop).toBeGreaterThanOrEqual(tablet!.writingBottom - 1);

  await page.setViewportSize({ width: 1024, height: 900 });
  await page.waitForTimeout(120);
  const desktop = await page.evaluate(() => {
    const writing = document.querySelector(".post-writing-surface")?.getBoundingClientRect();
    const panel = document.querySelector("#post-settings-panel")?.getBoundingClientRect();
    return writing && panel ? { writingRight: writing.right, panelLeft: panel.left } : null;
  });
  expect(desktop).not.toBeNull();
  expect(desktop!.panelLeft).toBeGreaterThanOrEqual(desktop!.writingRight - 1);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(120);
  const trigger = page.getByRole("button", { name: "文章设置", exact: true });
  await trigger.click();
  const drawer = page.getByRole("dialog", { name: "文章设置" });
  await expect(drawer).toBeVisible();
  await expect(page.getByRole("button", { name: "关闭文章设置" })).toBeFocused();

  for (let i = 0; i < 24; i++) {
    await page.keyboard.press("Tab");
    expect(await drawer.evaluate((el) => el.contains(document.activeElement))).toBe(true);
  }
  await page.keyboard.press("Shift+Tab");
  expect(await drawer.evaluate((el) => el.contains(document.activeElement))).toBe(true);

  await page.keyboard.press("Escape");
  await expect(drawer).toHaveCount(0);
  await expect(trigger).toBeFocused();

  await trigger.click();
  await expect(drawer).toBeVisible();
  await page.mouse.click(10, 100);
  await expect(drawer).toHaveCount(0);
  await expect(trigger).toBeFocused();

  await trigger.click();
  await expect(drawer).toBeVisible();
  await page.setViewportSize({ width: 800, height: 900 });
  await page.waitForTimeout(120);
  await expect(drawer).toHaveCount(0);
  await expect(page.locator(".bg-background\\/40")).toHaveCount(0);
});

test("390px 移动发布按钮近白底深字、13px/500、至少 44×44 且对比度达标", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createPost(page);

  const publish = page.locator(".editor-header .btn-primary");
  await expect(publish).toBeVisible();
  await expect(publish).toHaveText(/发布/);

  const result = await page.evaluate(() => {
    const button = document.querySelector<HTMLElement>(".editor-header .btn-primary");
    if (!button) throw new Error("publish button missing");
    const probe = document.createElement("div");
    document.body.appendChild(probe);
    probe.style.color = "var(--color-primary)";
    const expectedBackground = getComputedStyle(probe).color;
    probe.style.color = "var(--color-primary-foreground)";
    const expectedColor = getComputedStyle(probe).color;
    probe.remove();

    const style = getComputedStyle(button);
    const rect = button.getBoundingClientRect();
    const header = document.querySelector(".editor-header")!;
    return {
      expectedBackground,
      expectedColor,
      background: style.backgroundColor,
      color: style.color,
      fontSize: style.fontSize,
      fontWeight: style.fontWeight,
      rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height, right: rect.right, bottom: rect.bottom },
      viewport: { width: innerWidth, height: innerHeight },
      headerOverflow: header.scrollWidth - header.clientWidth,
      pageOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
      text: button.textContent?.trim(),
    };
  });

  expect(result.background).toBe(result.expectedBackground);
  expect(result.color).toBe(result.expectedColor);
  expect(result.fontSize).toBe("13px");
  expect(result.fontWeight).toBe("500");
  expect(result.rect.width).toBeGreaterThanOrEqual(44);
  expect(result.rect.height).toBeGreaterThanOrEqual(44);
  expect(result.rect.x).toBeGreaterThanOrEqual(0);
  expect(result.rect.y).toBeGreaterThanOrEqual(0);
  expect(result.rect.right).toBeLessThanOrEqual(result.viewport.width);
  expect(result.rect.bottom).toBeLessThanOrEqual(result.viewport.height);
  expect(result.headerOverflow).toBeLessThanOrEqual(0);
  expect(result.pageOverflow).toBeLessThanOrEqual(0);

  const rgb = (value: string) =>
    value
      .match(/\d+(?:\.\d+)?/g)!
      .map(Number)
      .slice(0, 3);
  const luminance = (value: string) => {
    const [r, g, b] = rgb(value).map((channel) => {
      const c = channel / 255;
      return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const light = luminance(result.background);
  const dark = luminance(result.color);
  const contrast = (Math.max(light, dark) + 0.05) / (Math.min(light, dark) + 0.05);
  expect(contrast).toBeGreaterThanOrEqual(4.5);
  test.info().annotations.push({ type: "contrast", description: `${result.text} ${contrast.toFixed(2)}:1` });
});
