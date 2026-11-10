// 仪表盘尺寸档位统一足迹：同一断点同一档位必须同尺寸，且不裁切、不空荡。
// 通过布局 API 直接把 16 个组件摆成同一档位，再在真实 SPA 上量测 DOM。
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { WIDGET_PRESET_FOOTPRINTS } from "../../modules/console/web/src/modules/dashboard/widgets/layout";

const WIDGET_IDS = [
  "stat-posts",
  "stat-published",
  "stat-views",
  "stat-comments",
  "stat-attachments",
  "stat-users",
  "trend-chart",
  "hourly-chart",
  "category-donut",
  "recent-posts",
  "recent-comments",
  "quick-actions",
  "top-posts",
  "system-health",
  "stellar-version",
  "system-version",
];

const STAT_IDS = new Set(WIDGET_IDS.filter((id) => id.startsWith("stat-")));

const CASES = [
  { bp: "lg", width: 1440, height: 900 },
  { bp: "sm", width: 900, height: 1000 },
  { bp: "xxs", width: 390, height: 844 },
] as const;

const PRESETS = ["small", "medium", "large"] as const;
const EVIDENCE_DIR = join(process.cwd(), "docs/baselines/dashboard-unified-size");

async function seedContent(page: Page) {
  const slugs: string[] = [];
  for (let index = 1; index <= 6; index += 1) {
    const response = await page.request.post("/api/v1/posts", {
      data: {
        slug: `dashboard-size-${index}`,
        title: `尺寸档位验收文章 ${index}`,
        content_md: `用于仪表盘填充度验收的第 ${index} 篇文章，内容仅作占位。`,
        status: "published",
        category: index % 2 === 0 ? "技术" : "随笔",
        stellar_version: 2,
        stellar_config: "{}",
        blog_extensions: "{}",
      },
    });
    expect(response.ok(), `创建夹具文章失败：${response.status()} ${await response.text()}`).toBe(true);
    slugs.push(`dashboard-size-${index}`);
  }
  // 访问公开文章页产生真实 PV/UV，让趋势、时段与热门组件按有数据的状态量测；
  // PV 走 3 秒批量落库，这里等缓冲 flush 后再进仪表盘。
  for (const slug of slugs) {
    for (let hit = 0; hit < 3; hit += 1) {
      await page.request.get(`/post/${slug}`);
    }
  }
  await page.waitForTimeout(4000);
}

async function login(page: Page) {
  await page.goto("/console/login");
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill("e2e-password");
  await page.getByRole("button", { name: "登 录", exact: true }).click();
  await expect(page).toHaveURL(/\/console\/dashboard/, { timeout: 30_000 });
}

/** 把一组组件按同一档位纵向排开，避免重叠影响量测。 */
function buildWidgets(bp: "lg" | "sm" | "xxs", preset: (typeof PRESETS)[number], ids: string[]) {
  const size = WIDGET_PRESET_FOOTPRINTS[bp][preset];
  let y = 0;
  return ids.map((id, index) => {
    const widget = { i: `${id}-${preset}-${index + 1}`, id, x: 0, y, w: size.w, h: size.h };
    y += size.h;
    return widget;
  });
}

async function saveLayout(page: Page, layouts: Record<string, unknown>) {
  const response = await page.request.put("/api/v1/dashboard/layout", {
    data: { version: 10, layouts },
  });
  expect(response.ok(), `保存布局失败：${response.status()} ${await response.text()}`).toBe(true);
}

interface Measurement {
  bp: string;
  preset: string;
  widgetId: string;
  gsW: number;
  gsH: number;
  width: number;
  height: number;
  cardWidth: number;
  cardHeight: number;
  scrollHeight: number;
  clientHeight: number;
  scrollWidth: number;
  clientWidth: number;
  fill: number;
  empty: boolean;
  topGap: number;
  bottomGap: number;
}

async function measure(page: Page, bp: string, preset: string): Promise<Measurement[]> {
  return page.evaluate(
    ({ bp, preset }) => {
      return [...document.querySelectorAll<HTMLElement>(".grid-stack-item")].map((item) => {
        const card = item.querySelector<HTMLElement>(".apple-card") ?? item.querySelector<HTMLElement>(".widget-cell")!;
        const rect = card.getBoundingClientRect();
        // display:none 的子元素 rect 是 0×0 且落在 (0,0)，会把内容包围盒拉成整页。
        const content = [...card.children]
          .filter((child) => !child.classList.contains("sr-only"))
          .map((child) => child.getBoundingClientRect())
          .filter((rect) => rect.width > 0 && rect.height > 0);
        const tops = content.map((rect) => rect.top);
        const bottoms = content.map((rect) => rect.bottom);
        const contentTop = tops.length ? Math.min(...tops) : rect.top;
        const contentBottom = bottoms.length ? Math.max(...bottoms) : rect.bottom;
        const visibleHeight = contentBottom - contentTop;
        return {
          bp,
          preset,
          widgetId: item.getAttribute("data-widget-id") ?? "",
          gsW: Number(item.getAttribute("gs-w")),
          gsH: Number(item.getAttribute("gs-h")),
          width: Math.round(rect.width * 10) / 10,
          height: Math.round(rect.height * 10) / 10,
          cardWidth: card.clientWidth,
          cardHeight: card.clientHeight,
          scrollHeight: card.scrollHeight,
          clientHeight: card.clientHeight,
          scrollWidth: card.scrollWidth,
          clientWidth: card.clientWidth,
          fill: Math.round((visibleHeight / rect.height) * 100) / 100,
          empty: Boolean(
            card.querySelector(
              ".trend-empty, .trend-chart-state, .hourly-empty-state, .top-posts-empty, .recent-placeholder, .donut-placeholder, .health-empty",
            ),
          ) || /暂无访问数据|暂无文章|暂无评论|暂无分类数据/.test(card.textContent ?? ""),
          topGap: Math.round((contentTop - rect.top) * 10) / 10,
          bottomGap: Math.round((rect.bottom - contentBottom) * 10) / 10,
        };
      });
    },
    { bp, preset },
  );
}

test("仪表盘三档足迹在三断点下一致、不裁切、不空荡", async ({ page }) => {
  test.setTimeout(240_000);
  const errors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("pageerror", (error) => errors.push(error.message));
  const failedResponses: string[] = [];
  page.on("response", (response) => {
    if (response.status() >= 400) failedResponses.push(`${response.status()} ${response.url()}`);
  });

  await login(page);
  await seedContent(page);

  const all: Measurement[] = [];
  const shots: Array<{ bp: string; preset: string }> = [];

  for (const { bp, width, height } of CASES) {
    await page.setViewportSize({ width, height });
    for (const preset of PRESETS) {
      // 整行「大」档 16 张会超过客户端 512 行上限，分两批摆。
      const batches = [WIDGET_IDS.slice(0, 8), WIDGET_IDS.slice(8)];
      for (const [batchIndex, batch] of batches.entries()) {
        const widgets = buildWidgets(bp, preset, batch);
        await saveLayout(page, {
          lg: bp === "lg" ? widgets : [],
          md: bp === "lg" ? widgets : [],
          sm: bp === "sm" ? widgets : [],
          xs: bp === "xxs" ? widgets : [],
          xxs: bp === "xxs" ? widgets : [],
        });
        await page.goto("/console/dashboard");
        await expect(page.locator(".grid-stack-item")).toHaveCount(batch.length, { timeout: 20_000 });
        // 骨架屏是临时态，等它退场后再量真实内容，避免把加载中的行数当成裁切。
        await page
          .waitForFunction(() => document.querySelectorAll(".dashboard-skeleton-list").length === 0, null, { timeout: 10_000 })
          .catch(() => undefined);
        await page.waitForTimeout(300);
        all.push(...(await measure(page, bp, preset)));
        if (preset === "large" && batchIndex === 0) {
          await page.screenshot({ path: join(EVIDENCE_DIR, `after-${bp}-large.png`), fullPage: true });
        }
      }
      if (preset === "medium") {
        await page.screenshot({ path: join(EVIDENCE_DIR, `after-${bp}-medium.png`), fullPage: true });
        shots.push({ bp, preset });
      }
    }
  }

  mkdirSync(EVIDENCE_DIR, { recursive: true });
  writeFileSync(join(EVIDENCE_DIR, "measurements.json"), JSON.stringify({ generatedAt: new Date().toISOString(), measurements: all }, null, 2));

  // 页面横向溢出
  const overflow = await page.evaluate(() => document.scrollingElement!.scrollWidth - window.innerWidth);
  expect(overflow, "页面出现横向溢出").toBeLessThanOrEqual(0);

  for (const item of all) {
    const footprint = WIDGET_PRESET_FOOTPRINTS[item.bp as "lg" | "sm" | "xxs"][item.preset as (typeof PRESETS)[number]];
    // 1) 图格与足迹表一致
    expect(item.gsW, `${item.widgetId}/${item.bp}/${item.preset} 列宽`).toBe(footprint.w);
    expect(item.gsH, `${item.widgetId}/${item.bp}/${item.preset} 行高`).toBe(footprint.h);
    // 2) 不裁切
    expect(item.scrollHeight, `${item.widgetId}/${item.bp}/${item.preset} 纵向裁切`).toBeLessThanOrEqual(item.clientHeight + 1);
    expect(item.scrollWidth, `${item.widgetId}/${item.bp}/${item.preset} 横向裁切`).toBeLessThanOrEqual(item.clientWidth + 1);
  }

  // 3) 同断点同档位：任意两个组件渲染尺寸一致
  for (const { bp } of CASES) {
    for (const preset of PRESETS) {
      const group = all.filter((item) => item.bp === bp && item.preset === preset);
      const widths = new Set(group.map((item) => item.width));
      const heights = new Set(group.map((item) => item.height));
      expect(widths.size, `${bp}/${preset} 同档宽度不一致：${[...widths].join("/")}`).toBe(1);
      expect(heights.size, `${bp}/${preset} 同档高度不一致：${[...heights].join("/")}`).toBe(1);
    }
  }

  // 4) 填充度：中/大档不空荡；统计卡在盒子偏大时按上下留白对称验收
  for (const item of all) {
    if (item.preset === "small" || item.empty) continue;
    if (STAT_IDS.has(item.widgetId)) {
      expect(
        Math.abs(item.topGap - item.bottomGap),
        `${item.widgetId}/${item.bp}/${item.preset} 上下留白不对称（${item.topGap}/${item.bottomGap}）`,
      ).toBeLessThanOrEqual(12);
      continue;
    }
    expect(item.fill, `${item.widgetId}/${item.bp}/${item.preset} 填充度过低`).toBeGreaterThanOrEqual(0.6);
  }

  // E2E 夹具没有启用 perf 模块，系统状态组件对该端点的 403 是既有环境条件，
  // 组件已按 "--" 降级；资源加载失败按 URL 断言，其余 JS 异常仍按 console/pageerror 拦截。
  const ignored = /\/api\/v1\/perf\/overview$/;
  const realErrors = errors.filter((message) => !message.startsWith("Failed to load resource"));
  const realFailures = [...new Set(failedResponses)].filter((entry) => !ignored.test(entry.split(" ").slice(1).join(" ")));
  expect(realErrors, `控制台错误：\n${realErrors.join("\n")}`).toEqual([]);
  expect(realFailures, `失败请求：\n${realFailures.join("\n")}`).toEqual([]);
  expect(shots.length).toBe(CASES.length);
});
