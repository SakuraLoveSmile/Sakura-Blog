// BLG-20261107-01 验收脚本：对两个隔离沙箱服务截图，并记录可复核的事实（verification.json）。
//   A = SANDBOX_A（默认 http://127.0.0.1:8197）  已初始化 Stellar 2.0 的站点，里面有笔记本 acceptance 与已发布笔记 responsive-check
//   B = SANDBOX_B（默认 http://127.0.0.1:8198）  旧版布局：旧 notebooks/notes 表 + 没有 Stellar 2.0 快照（尚未迁移）
// 沙箱的搭建步骤见同目录 README.md；本脚本只读沙箱，不写入任何数据。
//
// 用法（在仓库根目录，Console 已用 make build 构建，两个沙箱已在运行）：
//   E2E_ADMIN_PASSWORD=<scripts/e2e/prepare.go 里夹具管理员的密码> \
//     node docs/baselines/notebook-visibility-console/capture-notebook.mjs <输出目录>
// 夹具账号是 admin，密码以 scripts/e2e/prepare.go 为准；这里不重复写出，避免两处不一致。
import { chromium } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const out = process.argv[2];
if (!out) throw new Error("usage: node capture-notebook.mjs <output-dir>");
const password = process.env.E2E_ADMIN_PASSWORD;
if (!password) throw new Error("set E2E_ADMIN_PASSWORD (the fixture admin password is defined in scripts/e2e/prepare.go)");
fs.mkdirSync(out, { recursive: true });
const A = process.env.SANDBOX_A ?? "http://127.0.0.1:8197";
const B = process.env.SANDBOX_B ?? "http://127.0.0.1:8198";
const facts = { generated_at: new Date().toISOString(), checks: [], touch_targets: {}, links: [] };
let failed = 0;
function check(name, ok, detail) {
  facts.checks.push({ name, ok: !!ok, detail: detail ?? null });
  if (!ok) failed++;
  console.log(ok ? "PASS" : "FAIL", name, detail ?? "");
}

const viewports = {
  desktop: { width: 1280, height: 860, scheme: "light", suffix: "1280" },
  mobile: { width: 390, height: 844, scheme: "dark", suffix: "390" },
};

async function login(page, base) {
  await page.goto(`${base}/console/login`);
  await page.getByPlaceholder("请输入用户名").fill("admin");
  await page.getByPlaceholder("请输入密码").fill(password);
  await page.getByPlaceholder("请输入密码").press("Enter");
  await page.waitForURL(/\/console\/dashboard/);
}

// 只统计 .notebooks-body 内可见的交互控件；共享组件（富文本工具栏、图标选择器）单独列出，不隐藏。
async function touchTargets(page) {
  return page.evaluate(() => {
    const own = [];
    const shared = [];
    const root = document.querySelector(".notebooks-body");
    if (!root) return { own, shared, overflowX: document.documentElement.scrollWidth > innerWidth };
    for (const el of root.querySelectorAll("a, button, input, select, textarea, summary")) {
      if (el.type === "hidden" || el.type === "file") continue;
      const rect = el.getBoundingClientRect();
      if (rect.width < 2 || rect.height < 2) continue;
      const style = getComputedStyle(el);
      if (style.visibility === "hidden" || style.display === "none") continue;
      const row = {
        tag: el.tagName.toLowerCase(),
        text: (el.textContent || el.getAttribute("aria-label") || el.getAttribute("placeholder") || "").trim().slice(0, 24),
        height: Math.round(rect.height * 10) / 10,
      };
      // 共享的富文本编辑器、图标选择器和附件选择器沿用各自既有尺寸，单独列出但不计入本任务的 44px 判定。
      const isShared = !!el.closest(".notebooks-editor-shell, .notebooks-shared");
      (isShared ? shared : own).push(row);
    }
    return { own, shared, overflowX: document.documentElement.scrollWidth > innerWidth };
  });
}

async function shot(page, name) {
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(out, name) });
}

const browser = await chromium.launch();
try {
  for (const [key, vp] of Object.entries(viewports)) {
    // ---------- A：已迁移站点 ----------
    const ctx = await browser.newContext({
      viewport: { width: vp.width, height: vp.height },
      colorScheme: vp.scheme,
      isMobile: vp.width < 768,
      hasTouch: vp.width < 768,
      deviceScaleFactor: 2,
    });
    const page = await ctx.newPage();
    await login(page, A);

    await page.goto(`${A}/console/notebooks`);
    await page.getByText("1 篇笔记 · 1 篇已发布").waitFor();
    await shot(page, `console-notebooks-list-${vp.suffix}.png`);
    const listLinks = await page.locator("a.notebooks-ext-link").evaluateAll((els) => els.map((a) => a.getAttribute("href")));
    facts.touch_targets[`notebooks-list-${key}`] = await touchTargets(page);

    await page.goto(`${A}/console/notebooks/acceptance/notes`);
    await page.getByText("responsive-check").waitFor();
    await shot(page, `console-notes-list-${vp.suffix}.png`);
    const noteLinks = await page.locator("a.notebooks-ext-link").evaluateAll((els) => els.map((a) => a.getAttribute("href")));
    facts.touch_targets[`notes-list-${key}`] = await touchTargets(page);

    await page.goto(`${A}/console/notebooks/acceptance/edit`);
    await page.getByText("基本信息").waitFor();
    await shot(page, `console-notebook-editor-${vp.suffix}.png`);
    facts.touch_targets[`notebook-editor-${key}`] = await touchTargets(page);

    await page.goto(`${A}/console/notebooks/acceptance/notes/1/edit`);
    await page.getByText("基本信息").waitFor();
    await shot(page, `console-note-editor-${vp.suffix}.png`);
    facts.touch_targets[`note-editor-${key}`] = await touchTargets(page);

    // 每个“在博客中查看”链接都能打开（HTTP 200），并且是站内路径。
    for (const href of new Set([...listLinks, ...noteLinks])) {
      const response = await page.request.get(`${A}${href}`);
      facts.links.push({ href, status: response.status(), viewport: key });
      check(`public link ${href} (${key}) opens`, href.startsWith("/") && response.status() === 200, `HTTP ${response.status()}`);
    }

    // 前台实际显示：笔记本索引、笔记本首页、笔记页。
    await page.goto(`${A}/notebooks`);
    await shot(page, `public-notebooks-index-${vp.suffix}.png`);
    await page.goto(`${A}/notebooks/acceptance`);
    await shot(page, `public-notebook-${vp.suffix}.png`);
    await page.goto(`${A}/notebooks/acceptance/responsive-check`);
    check(`public note page shows title (${key})`, await page.getByRole("heading", { name: "检查项" }).first().isVisible());
    await shot(page, `public-note-${vp.suffix}.png`);
    await ctx.close();

    // ---------- B：旧版布局 / 尚未迁移 ----------
    const legacy = await browser.newContext({
      viewport: { width: vp.width, height: vp.height },
      colorScheme: vp.scheme,
      isMobile: vp.width < 768,
      hasTouch: vp.width < 768,
      deviceScaleFactor: 2,
    });
    const lp = await legacy.newPage();
    const response = await lp.goto(`${B}/notebooks`);
    check(`legacy public page is a readable 409 page (${key})`, response?.status() === 409 && (await lp.title()) === "站点内容正在升级", `HTTP ${response?.status()}`);
    await shot(lp, `legacy-public-409-${vp.suffix}.png`);
    await login(lp, B);
    await lp.goto(`${B}/console/notebooks`);
    await lp.getByText("需要先迁移到 Stellar 2.0").waitFor();
    check(`legacy console shows the migration notice (${key})`, await lp.locator(".notebooks-notice[role=alert]").isVisible());
    check(`legacy console hides the raw backend text until expanded (${key})`, !(await lp.locator(".notebooks-notice-detail pre").isVisible()));
    await shot(lp, `legacy-console-notebooks-${vp.suffix}.png`);
    facts.touch_targets[`legacy-notice-${key}`] = await touchTargets(lp);
    await legacy.close();
  }

  // 非浏览器客户端仍然拿到 JSON（脚本和 API 不受 HTML 说明页影响）。
  const json = await fetch(`${B}/notebooks`);
  const body = await json.json();
  check("legacy public page keeps JSON for API clients", json.status === 409 && body.code === "stellar_migration_required", `HTTP ${json.status}`);

  const mobileSmall = Object.entries(facts.touch_targets)
    .filter(([name]) => name.endsWith("mobile"))
    .flatMap(([name, value]) => value.own.filter((row) => row.height < 44).map((row) => ({ page: name, ...row })));
  check("mobile own controls are at least 44px tall", mobileSmall.length === 0, JSON.stringify(mobileSmall));
  const overflow = Object.entries(facts.touch_targets).filter(([, value]) => value.overflowX).map(([name]) => name);
  check("no horizontal overflow on any checked screen", overflow.length === 0, overflow.join(","));
} finally {
  await browser.close();
  fs.writeFileSync(path.join(out, "verification.json"), JSON.stringify(facts, null, 2) + "\n");
}
if (failed) {
  console.error(`${failed} check(s) failed`);
  process.exit(1);
}
