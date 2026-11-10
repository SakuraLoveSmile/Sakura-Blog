// 仪表盘头部控件的登录与量测辅助：BLG-20261110-07 的 e2e 用例与改前/改后取证脚本共用。
// 量测只依赖 `.dashboard-heading` / `.apple-header-actions` 这两个稳定外壳，不依赖具体按钮类名，
// 因此同一份代码可以在改动前后的构建上取同口径的数据。
import type { Page } from "@playwright/test";

export interface HeaderControlBox {
  name: string;
  x: number;
  y: number;
  width: number;
  height: number;
  fontSize: string;
  color: string;
  background: string;
  border: string;
  title: string | null;
}

export interface HeaderMeasurement {
  viewport: { width: number; height: number };
  dataBp: string | null;
  canvasWidth: number;
  heading: { height: number };
  actions: { x: number; y: number; width: number; height: number };
  scrollWidth: number;
  clientWidth: number;
  controls: HeaderControlBox[];
}

/** 用 e2e 夹具账号登录并等头部渲染出来。 */
export async function loginToDashboard(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await page.waitForURL(/\/console\/dashboard/, { timeout: 30_000 });
  await page.locator(".dashboard-heading .apple-header-actions").waitFor({ state: "visible" });
}

/** 量取头部所有按钮的盒子与计算样式。 */
export async function measureHeader(page: Page): Promise<HeaderMeasurement> {
  // 回调里不声明具名函数：tsx 的 keepNames 会注入浏览器里不存在的 __name 辅助函数。
  return page.evaluate(() => {
    const heading = document.querySelector(".dashboard-heading") as HTMLElement;
    const actions = heading.querySelector(".apple-header-actions") as HTMLElement;
    const canvas = document.querySelector(".dashboard-canvas") as HTMLElement;
    const controls = [...actions.querySelectorAll("button")].map((el) => {
      const rect = el.getBoundingClientRect();
      const style = getComputedStyle(el);
      return {
        name: el.getAttribute("aria-label") || (el.textContent || "").trim(),
        x: Math.round(rect.x * 100) / 100,
        y: Math.round(rect.y * 100) / 100,
        width: Math.round(rect.width * 100) / 100,
        height: Math.round(rect.height * 100) / 100,
        fontSize: style.fontSize,
        color: style.color,
        background: style.backgroundColor,
        border: `${style.borderTopWidth} ${style.borderTopStyle} ${style.borderTopColor}`,
        title: el.getAttribute("title"),
      };
    });
    const actionsRect = actions.getBoundingClientRect();
    return {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      dataBp: document.querySelector(".dashboard-apple")?.getAttribute("data-bp") ?? null,
      canvasWidth: Math.round(canvas.getBoundingClientRect().width * 100) / 100,
      heading: { height: Math.round(heading.getBoundingClientRect().height * 100) / 100 },
      actions: {
        x: Math.round(actionsRect.x * 100) / 100,
        y: Math.round(actionsRect.y * 100) / 100,
        width: Math.round(actionsRect.width * 100) / 100,
        height: Math.round(actionsRect.height * 100) / 100,
      },
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
      controls,
    };
  });
}
