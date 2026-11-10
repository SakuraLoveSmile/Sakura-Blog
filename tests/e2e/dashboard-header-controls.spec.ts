// 仪表盘头部控件（BLG-20261110-07）：在真实 SPA 上量测尺寸、状态、提示与手机排布。
// 验收项编号（D / M / X）对应 docs/baselines/dashboard-header-controls/README.md。
// 本用例只断言、不写任何仓库文件；取证截图由一次性脚本生成，避免常规 e2e 运行弄脏已跟踪的基线。
import { expect, test, type Browser, type Page } from "@playwright/test";
import {
  loginToDashboard,
  measureHeader,
  type HeaderControlBox,
  type HeaderMeasurement,
} from "./support/dashboard-header";

const MODES = ["自适应", "电脑", "平板", "手机"] as const;
const REFRESH = "刷新仪表盘数据";
const TIP_KEY: Record<(typeof MODES)[number], string> = { 自适应: "auto", 电脑: "lg", 平板: "sm", 手机: "xxs" };
const TIP_HINT: Record<(typeof MODES)[number], string> = {
  自适应: "跟随窗口宽度",
  电脑: "预览宽度 1440px",
  平板: "预览宽度 800px",
  手机: "预览宽度 390px",
};
const PREVIEW_WIDTH = { 电脑: "1440px", 平板: "800px", 手机: "390px" } as const;
const TRANSPARENT = "rgba(0, 0, 0, 0)";

const DESKTOP = { viewport: { width: 1440, height: 900 } };
const PHONE = { viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true };

async function openDashboard(browser: Browser, baseURL: string | undefined, options: object) {
  const context = await browser.newContext({ baseURL, ...options });
  const page = await context.newPage();
  await loginToDashboard(page);
  return { context, page };
}

function control(m: HeaderMeasurement, name: string): HeaderControlBox {
  const found = m.controls.find((item) => item.name === name);
  if (!found) throw new Error(`头部没有按钮「${name}」，实际有：${m.controls.map((item) => item.name).join("、")}`);
  return found;
}

/** 把设计令牌解析成浏览器实际使用的 rgb 字符串，避免在用例里写死色值。 */
async function tokenColor(page: Page, token: string): Promise<string> {
  return page.evaluate((name) => {
    const probe = document.createElement("div");
    probe.style.color = `var(${name})`;
    document.body.appendChild(probe);
    const color = getComputedStyle(probe).color;
    probe.remove();
    return color;
  }, token);
}

function modeButton(page: Page, name: string) {
  return page.getByRole("button", { name, exact: true });
}

function modeTip(page: Page, name: (typeof MODES)[number]) {
  return page.locator(`.dashboard-tip:has(#dashboard-tip-${TIP_KEY[name]})`);
}

/** 伪元素 ::after 的计算样式（选中短线）。 */
async function underline(page: Page, name: string) {
  return modeButton(page, name).evaluate((el) => {
    const style = getComputedStyle(el, "::after");
    return {
      content: style.content,
      width: style.width,
      height: style.height,
      bottom: style.bottom,
      background: style.backgroundColor,
      boxShadow: style.boxShadow,
    };
  });
}

/** 当前焦点元素的可读名称。 */
async function focusedName(page: Page): Promise<string> {
  return page.evaluate(() => {
    const el = document.activeElement as HTMLElement | null;
    return el?.getAttribute("aria-label") || (el?.textContent || "").trim();
  });
}

test("D1–D11 桌面 1440：工具组是 36px 幽灵控件，选中为短线，悬停与键盘聚焦出现带预览宽度的提示", async ({
  browser,
  baseURL,
}) => {
  const { context, page } = await openDashboard(browser, baseURL, DESKTOP);
  try {
    const m = await measureHeader(page);
    expect(m.dataBp).toBe("lg");

    // D1 写作组不变（描边 / 主按钮，13px，36px 高）
    for (const name of ["继续写草稿", "撰写文章"]) {
      expect(control(m, name).height, `${name} 高度`).toBe(36);
      expect(control(m, name).fontSize, `${name} 字号`).toBe("13px");
    }

    // D2 / D3 工具组全部 36px，默认态无底色无描边，且没有原生 title
    for (const name of [...MODES, REFRESH]) {
      const c = control(m, name);
      expect([c.width, c.height], `${name} 尺寸`).toEqual([36, 36]);
      expect(c.background, `${name} 底色`).toBe(TRANSPARENT);
      expect(c.border, `${name} 边框`).toBe(`1px solid ${TRANSPARENT}`);
      expect(c.title, `${name} 的原生 title`).toBeNull();
    }
    const settings = control(m, "设置布局");
    expect(settings.height).toBe(36);
    expect(settings.fontSize).toBe("13px");
    expect(settings.background).toBe(TRANSPARENT);
    expect(settings.border).toBe(`1px solid ${TRANSPARENT}`);

    // D4 间距：切换组内 2px，切换组 / 刷新 / 设置布局之间 8px；同一行
    const [auto, desktop, tablet, phone] = MODES.map((name) => control(m, name));
    expect(desktop.x - (auto.x + auto.width)).toBe(2);
    expect(tablet.x - (desktop.x + desktop.width)).toBe(2);
    expect(phone.x - (tablet.x + tablet.width)).toBe(2);
    const refresh = control(m, REFRESH);
    expect(refresh.x - (phone.x + phone.width)).toBe(8);
    expect(settings.x - (refresh.x + refresh.width)).toBe(8);
    for (const c of [desktop, tablet, phone, refresh, settings]) expect(c.y + c.height / 2).toBeCloseTo(auto.y + auto.height / 2, 0);

    // D5 选中态：前景色 + 16×1 短线，其余没有短线；不使用 inset 阴影
    const foreground = await tokenColor(page, "--color-foreground");
    const muted = await tokenColor(page, "--color-muted-foreground");
    expect(control(m, "自适应").color).toBe(foreground);
    expect(control(m, "电脑").color).toBe(muted);
    expect(await underline(page, "自适应")).toEqual({
      content: '""',
      width: "16px",
      height: "1px",
      bottom: "5px",
      background: foreground,
      boxShadow: "none",
    });
    expect((await underline(page, "电脑")).content).toBe("none");
    await expect(modeButton(page, "自适应")).toHaveAttribute("aria-pressed", "true");
    await expect(modeButton(page, "电脑")).toHaveAttribute("aria-pressed", "false");

    // 提示默认隐藏
    for (const name of MODES) await expect(modeTip(page, name)).toBeHidden();

    // D6 悬停：底色换成 accent，图标 foreground
    const accent = await tokenColor(page, "--color-accent");
    await modeButton(page, "平板").hover();
    await expect
      .poll(() => modeButton(page, "平板").evaluate((el) => getComputedStyle(el).backgroundColor))
      .toBe(accent);
    expect(await modeButton(page, "平板").evaluate((el) => getComputedStyle(el).color)).toBe(foreground);

    // D7 悬停提示：模式名 + 预览宽度；位置在按钮下方 6px，水平居中（第一枚在桌面也居中）
    const strongBorder = await tokenColor(page, "--color-border-strong");
    const popover = await tokenColor(page, "--color-popover");
    for (const name of MODES) {
      await modeButton(page, name).hover();
      const tip = modeTip(page, name);
      await expect(tip).toBeVisible();
      await expect.poll(() => tip.evaluate((el) => getComputedStyle(el).opacity)).toBe("1");
      await expect(tip).toContainText(name);
      await expect(tip).toContainText(TIP_HINT[name]);
      const buttonBox = (await modeButton(page, name).boundingBox())!;
      const tipBox = (await tip.boundingBox())!;
      expect(tipBox.y - (buttonBox.y + buttonBox.height), `${name} 提示的垂直间距`).toBeCloseTo(6, 0);
      expect(tipBox.x + tipBox.width / 2, `${name} 提示水平居中`).toBeCloseTo(buttonBox.x + buttonBox.width / 2, 0);
      // 其余提示仍然隐藏
      for (const other of MODES.filter((item) => item !== name)) await expect(modeTip(page, other)).toBeHidden();
    }
    const tipStyle = await modeTip(page, "手机").evaluate((el) => {
      const style = getComputedStyle(el);
      return {
        fontSize: style.fontSize,
        hintSize: getComputedStyle(el.querySelector("small") as Element).fontSize,
        background: style.backgroundColor,
        border: `${style.borderTopWidth} ${style.borderTopStyle} ${style.borderTopColor}`,
        radius: style.borderTopLeftRadius,
        shadow: style.boxShadow !== "none",
      };
    });
    expect(tipStyle).toEqual({
      fontSize: "12px",
      hintSize: "11px",
      background: popover,
      border: `1px solid ${strongBorder}`,
      radius: "6px",
      shadow: true,
    });

    // D8 刷新按钮的提示
    await modeButton(page, REFRESH).hover();
    const refreshTip = page.locator(".dashboard-tool:has(.dashboard-refresh-btn) .dashboard-tip");
    await expect(refreshTip).toBeVisible();
    await expect(refreshTip).toHaveText(REFRESH);

    // D7（键盘）/ D9 聚焦环与 Tab 顺序：写作 → 视图切换 → 刷新 → 设置布局
    await page.mouse.move(2, 2);
    await page.getByRole("button", { name: "继续写草稿" }).focus();
    const order: string[] = [];
    for (let step = 0; step < 7; step += 1) {
      await page.keyboard.press("Tab");
      order.push(await focusedName(page));
      if (step === 1) {
        // 聚焦第一枚视图按钮：提示显示，聚焦环 2px / offset 2px / ring 色
        const ring = await tokenColor(page, "--color-ring");
        await expect(modeTip(page, "自适应")).toBeVisible();
        const outline = await modeButton(page, "自适应").evaluate((el) => {
          const style = getComputedStyle(el);
          return `${style.outlineWidth} ${style.outlineStyle} ${style.outlineColor} ${style.outlineOffset}`;
        });
        expect(outline).toBe(`2px solid ${ring} 2px`);
      }
    }
    expect(order).toEqual(["撰写文章", "自适应", "电脑", "平板", "手机", REFRESH, "设置布局"]);
    // 焦点离开视图切换后提示收起；反向 Tab 回到「手机」时提示重新出现
    for (const name of MODES) await expect(modeTip(page, name)).toBeHidden();
    await page.keyboard.press("Shift+Tab");
    await page.keyboard.press("Shift+Tab");
    expect(await focusedName(page)).toBe("手机");
    await expect(modeTip(page, "手机")).toBeVisible();

    // D10 刷新中：转圈图标 + aria-busy，没有额外底色，随后恢复
    await modeButton(page, REFRESH).click();
    await page.mouse.move(2, 2);
    const refreshButton = modeButton(page, REFRESH);
    await expect(refreshButton).toHaveAttribute("aria-busy", "true");
    await expect(refreshButton.locator("svg.animate-spin")).toHaveCount(1);
    expect(await refreshButton.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe(TRANSPARENT);
    await expect(refreshButton).toHaveAttribute("aria-busy", "false", { timeout: 4000 });

    // D11 功能不变：预览宽度、选中状态与设置布局入口
    for (const name of ["电脑", "平板", "手机"] as const) {
      await modeButton(page, name).click();
      await expect(modeButton(page, name)).toHaveAttribute("aria-pressed", "true");
      await expect(page.locator(".dashboard-canvas")).toHaveCSS("width", PREVIEW_WIDTH[name]);
      await expect(modeButton(page, "自适应")).toHaveAttribute("aria-pressed", "false");
    }
    await modeButton(page, "自适应").click();
    await expect(page.locator(".dashboard-canvas")).toHaveAttribute("style", /width: 100%/);
    await page.getByRole("button", { name: "设置布局" }).click();
    await expect(page).toHaveURL(/\/console\/dashboard\/designer/);

    // G3 设计页的保存按钮仍是 .apple-edit-btn 原样（14px）
    const save = page.locator("button.apple-edit-btn").first();
    await expect(save).toBeVisible();
    await expect(save).toHaveCSS("font-size", "14px");
  } finally {
    await context.close();
  }
});

test("M1–M8 手机 390：写作按钮平分整行，工具一行排完，窄屏不横向滚动，触屏不残留提示", async ({
  browser,
  baseURL,
}) => {
  const { context, page } = await openDashboard(browser, baseURL, PHONE);
  try {
    const m = await measureHeader(page);
    expect(m.dataBp).toBe("xxs");

    // M1 写作按钮平分整行
    const draft = control(m, "继续写草稿");
    const write = control(m, "撰写文章");
    expect(draft.height).toBe(44);
    expect(write.height).toBe(44);
    expect(Math.abs(draft.width - write.width)).toBeLessThanOrEqual(1);
    expect(draft.width + write.width + 8).toBeCloseTo(m.actions.width, 0);

    // M2 / M3 工具一行排完，每枚 ≥44，间距 2 / 4px，总宽不超过 375 宽手机的可用宽度 351
    const tools = [...MODES, REFRESH, "设置布局"].map((name) => control(m, name));
    for (const c of tools) {
      expect(c.width, `${c.name} 宽`).toBeGreaterThanOrEqual(44);
      expect(c.height, `${c.name} 高`).toBeGreaterThanOrEqual(44);
      expect(Math.abs(c.y - tools[0].y), `${c.name} 与第一枚同一行`).toBeLessThanOrEqual(1);
    }
    expect(tools[1].x - (tools[0].x + tools[0].width)).toBe(2);
    expect(tools[4].x - (tools[3].x + tools[3].width)).toBe(4);
    expect(tools[5].x - (tools[4].x + tools[4].width)).toBe(4);
    const last = tools[tools.length - 1];
    expect(last.x + last.width - tools[0].x).toBeLessThanOrEqual(351);
    expect(last.x + last.width).toBeLessThanOrEqual(m.clientWidth);
    // 设置布局退到幽灵层级：透明底、无边框
    expect(last.background).toBe(TRANSPARENT);
    expect(last.border).toBe(`1px solid ${TRANSPARENT}`);

    // M4 控件区 3 行变 2 行
    expect(m.actions.height).toBeLessThanOrEqual(98);
    // M5 选中短线在 44px 按钮里保持在图标下 3px
    expect((await underline(page, "自适应")).bottom).toBe("9px");

    // M7 键盘聚焦第一枚视图按钮：提示不溢出视口左缘
    await page.getByRole("button", { name: "撰写文章" }).focus();
    await page.keyboard.press("Tab");
    const tip = modeTip(page, "自适应");
    await expect(tip).toBeVisible();
    const tipBox = (await tip.boundingBox())!;
    expect(tipBox.x).toBeGreaterThanOrEqual(0);
    expect(tipBox.x + tipBox.width).toBeLessThanOrEqual(m.clientWidth);
    await page.keyboard.press("Tab");
    await page.keyboard.press("Tab");

    // M8 触屏点按切换后不残留提示
    await modeButton(page, "手机").tap();
    await expect(modeButton(page, "手机")).toHaveAttribute("aria-pressed", "true");
    await expect(modeTip(page, "手机")).toBeHidden();
    await modeButton(page, "自适应").tap();

    // M6 窄屏：头部按钮不出视口；390 / 375 / 360 页面不横向滚动；320 时设置布局换行而不是溢出
    for (const width of [375, 360, 320]) {
      await page.setViewportSize({ width, height: 844 });
      await expect.poll(async () => (await measureHeader(page)).clientWidth).toBe(width);
      const narrow = await measureHeader(page);
      for (const c of narrow.controls) {
        expect(c.x, `${width}px 下 ${c.name} 左缘`).toBeGreaterThanOrEqual(0);
        expect(c.x + c.width, `${width}px 下 ${c.name} 右缘`).toBeLessThanOrEqual(width);
      }
      if (width >= 360) {
        expect(narrow.scrollWidth, `${width}px 页面横向滚动`).toBeLessThanOrEqual(narrow.clientWidth);
        const row = [...MODES, REFRESH, "设置布局"].map((name) => control(narrow, name));
        for (const c of row) expect(Math.abs(c.y - row[0].y), `${width}px 下 ${c.name} 同一行`).toBeLessThanOrEqual(1);
      } else {
        const wrapped = control(narrow, "设置布局");
        expect(wrapped.y, "320px 下设置布局换到下一行").toBeGreaterThanOrEqual(control(narrow, "手机").y + 44);
      }
    }
  } finally {
    await context.close();
  }
});

test("X1 真机视口 390 + 「电脑」预览（data-bp=lg）：头部每个按钮宽高都不小于 44px", async ({ browser, baseURL }) => {
  const { context, page } = await openDashboard(browser, baseURL, PHONE);
  try {
    await modeButton(page, "电脑").tap();
    await expect(page.locator(".dashboard-apple")).toHaveAttribute("data-bp", "lg");
    // 画布宽度有 0.3s 过渡，等它落定后再量
    await expect(page.locator(".dashboard-canvas")).toHaveCSS("width", PREVIEW_WIDTH.电脑);
    const m = await measureHeader(page);
    expect(m.canvasWidth).toBe(1440);
    for (const c of m.controls) {
      expect(c.width, `${c.name} 宽`).toBeGreaterThanOrEqual(44);
      expect(c.height, `${c.name} 高`).toBeGreaterThanOrEqual(44);
    }
  } finally {
    await context.close();
  }
});

test("X2 桌面 1440 + 「手机」预览（data-bp=xxs）：写作按钮与工具同为 44px，布局与手机一致", async ({
  browser,
  baseURL,
}) => {
  const { context, page } = await openDashboard(browser, baseURL, DESKTOP);
  try {
    await modeButton(page, "手机").click();
    await expect(page.locator(".dashboard-apple")).toHaveAttribute("data-bp", "xxs");
    // 画布宽度有 0.3s 过渡，等它落定后再量
    await expect(page.locator(".dashboard-canvas")).toHaveCSS("width", PREVIEW_WIDTH.手机);
    const m = await measureHeader(page);
    expect(m.canvasWidth).toBe(390);
    for (const c of m.controls) {
      expect(c.width, `${c.name} 宽`).toBeGreaterThanOrEqual(44);
      expect(c.height, `${c.name} 高`).toBeGreaterThanOrEqual(44);
    }
    const draft = control(m, "继续写草稿");
    const write = control(m, "撰写文章");
    expect(Math.abs(draft.width - write.width)).toBeLessThanOrEqual(1);
    expect(draft.width + write.width + 8).toBeCloseTo(m.actions.width, 0);
    const tools = [...MODES, REFRESH, "设置布局"].map((name) => control(m, name));
    for (const c of tools) expect(Math.abs(c.y - tools[0].y), `${c.name} 同一行`).toBeLessThanOrEqual(1);
    expect(m.actions.height).toBeLessThanOrEqual(98);
  } finally {
    await context.close();
  }
});
