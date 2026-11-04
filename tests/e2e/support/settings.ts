import { expect, type Locator, type Page } from "@playwright/test";

// 系统设置按“分组导航 → 设置页”组织：桌面是左侧纵向导航，手机是“分组列表 → 详情”。
// 这些辅助函数只通过可见的导航操作，草稿不会因切换页面而丢失。
export const settingsNav = (page: Page, key: string) =>
  page.locator(`.settings-nav:visible [data-settings-page="${key}"]`);

/** Console 外壳里自己滚动的内容区；sticky 页头与保存栏都相对它定位。 */
export const contentScroller = (page: Page) => page.locator("main.content");
export const scrollTopOf = (page: Page) => contentScroller(page).evaluate((el) => Math.round(el.scrollTop));

/**
 * 目标必须真的露在 sticky 页头下面：顶部不高于页头底边，且该处的命中测试落在目标自己身上。
 * Playwright 的可见性断言不看遮挡——页头盖住标题时它们仍然通过，所以锚点与换页要用它。
 */
export async function expectClearOfHeader(target: Locator, what: string) {
  await expect
    .poll(
      async () => {
        // 先数一下命中数：定位器没有命中时，轮询的超时信息里要能看出这一点，而不是只看到“超时”。
        const count = await target.count();
        if (count !== 1) return `定位器命中 ${count} 个元素`;
        return target.evaluate((el) => {
          const header = document.querySelector(".settings-view > .page-header")!.getBoundingClientRect();
          const box = el.getBoundingClientRect();
          const hit = document.elementFromPoint(box.left + Math.min(24, box.width / 2), box.top + box.height / 2);
          const covered = hit !== el && !el.contains(hit);
          return box.top >= header.bottom && !covered
            ? "clear"
            : `top=${Math.round(box.top)} headerBottom=${Math.round(header.bottom)} covered=${covered}`;
        });
      },
      { message: `${what}应露在 sticky 页头下面`, timeout: 6_000 },
    )
    .toBe("clear");
}

export async function openSettingsPage(page: Page, key: string) {
  const item = settingsNav(page, key);
  const back = page.getByRole("button", { name: "返回设置", exact: true });
  // 刚加载的页面还没渲染出导航时不能当作“列表已隐藏”：先等到导航项（桌面，或手机的分组列表）
  // 或“返回设置”（手机详情）出现，再决定是否需要先回到分组列表。
  await expect.poll(async () => (await item.isVisible()) || (await back.isVisible())).toBe(true);
  if (!(await item.isVisible())) await back.click();
  await item.click();
  await expect(page.locator(`#settings-page-${key}`)).toBeVisible();
  if ((page.viewportSize()?.width ?? 1280) >= 768) await expect(item).toHaveAttribute("aria-current", "page");
}

/** 展开“代码与原文”页里的完整配置原文。 */
export async function openSettingsRaw(page: Page) {
  await openSettingsPage(page, "advanced");
  await page.locator(".settings-raw-editor").evaluate((el) => {
    (el as HTMLDetailsElement).open = true;
  });
  await expect(page.locator("#stellar2-document")).toBeVisible();
}

export type TouchTargetReport = {
  /** 参与测量的可点击控件（已展开全部折叠分组；不含 disabled 与不可见的）。 */
  controls: number;
  /** 自身盒子宽或高不足 44px、靠透明扩大区补足点击区的控件。 */
  boxUnder44: number;
  /** 其中点击区（自身加上连续的扩大区）已达到 44px 的控件。 */
  reached44: number;
  /** 点击区不足 44px，截断处不是别的控件：扩大区没补、被裁掉，或被不可点击的元素盖住。 */
  uncovered: string[];
  /** 点击区不足 44px，是因为再往外就是别的控件（自身区域或它的扩大区）：扩大区只补到邻居为止，不抢点击。 */
  blocked: string[];
  /** blocked 里点击区最小的那个（px）；没有 blocked 时为 0。 */
  blockedMin: number;
  /** 控件（或可点击的标签）自身区域里有点被别的元素抢走，常见原因是相邻控件的扩大区盖了过来。 */
  stolen: string[];
};

/**
 * 手机点击区的命中测试，逐个控件用 elementFromPoint 量真实的点击区：
 * - 不足 44px 的方向，从控件边缘向两侧逐像素探测，数连续仍落在该控件（或它的 label）上的像素，加上自身尺寸就是点击区；
 *   跨度方向取左中右（或上中下）三条线，取最小值。不足 44px 时按截断处的占用者归类：别的控件算 blocked，其余算 uncovered；
 * - 每个控件（含带 for 的标签）自身区域内的点，必须仍落在它自己身上，被别的元素抢走算 stolen。
 * boundingBox ≥ 44 的断言看不到“透明伪元素扩大的点击区”，更看不到扩大区抢走邻居点击的问题，所以用命中测试。
 * 先展开全部 <details>：折叠分组里的控件没展开时有布局尺寸，却画不出来、点不到。只改 DOM 的 open，不改任何设置值。
 */
export async function measureTouchTargets(page: Page, rootSelector: string): Promise<TouchTargetReport> {
  return page.evaluate((selector) => {
    const root = document.querySelector(selector);
    if (!root) throw new Error(`没有找到 ${selector}`);
    for (const details of root.querySelectorAll("details")) details.open = true;
    void (root as HTMLElement).offsetHeight;

    const MIN = 44;
    const isVisible = (el: Element) => {
      const box = el.getBoundingClientRect();
      const style = getComputedStyle(el);
      return (
        box.width > 1 && box.height > 1 && style.visibility !== "hidden" && style.display !== "none" && !el.closest("[hidden]")
      );
    };
    const CONTROLS =
      "button, a[href], input:not([type=hidden]), select, textarea, [role=switch], summary, .visual-choice, .settings-seg label";
    const seen = new Set<Element>();
    const controls: HTMLElement[] = [];
    for (const candidate of root.querySelectorAll<HTMLElement>(CONTROLS)) {
      if ((candidate as HTMLButtonElement).disabled) continue;
      let el = candidate;
      // 原生单选/复选/取色输入只是叠在 label 里的透明层，可点击的容器是 label。
      if (el.matches("input[type=radio], input[type=checkbox], input[type=color]")) {
        const label = (el as HTMLInputElement).labels?.[0];
        if (label) el = label;
      }
      if (seen.has(el) || !isVisible(el)) continue;
      seen.add(el);
      controls.push(el);
    }
    const labels = [...root.querySelectorAll<HTMLElement>("label[for]")].filter((label) => !seen.has(label) && isVisible(label));
    const victims = [...controls, ...labels];
    const victimSet = new Set<Element>(victims);

    const belongsTo = (hit: Element | null, el: HTMLElement) => {
      if (!hit) return false;
      if (el.contains(hit)) return true;
      const control = (el as HTMLLabelElement).control;
      if (control?.contains(hit)) return true;
      const own = (el as HTMLInputElement).labels;
      return Boolean(own && [...own].some((label) => label.contains(hit)));
    };
    const victimOf = (hit: Element | null) => {
      for (let node = hit; node && node !== document.body; node = node.parentElement) if (victimSet.has(node)) return node as HTMLElement;
      return null;
    };
    // 容器里嵌套的控件（如折叠摘要里的“恢复默认”）有自己的点击区：落在它上面的点不算容器的。
    const ownHit = (hit: Element | null, el: HTMLElement) => {
      if (!belongsTo(hit, el)) return false;
      const holder = victimOf(hit);
      return !holder || holder === el || !el.contains(holder);
    };
    // 圆角控件（如开关胶囊）的四角在圆弧之外，命中测试会落到它后面的元素，那不算被抢。
    const insideShape = (el: Element, box: DOMRect, x: number, y: number) => {
      const style = getComputedStyle(el);
      const radius = Math.min(
        Math.max(...[style.borderTopLeftRadius, style.borderTopRightRadius, style.borderBottomRightRadius, style.borderBottomLeftRadius].map((v) => parseFloat(v) || 0)),
        box.width / 2,
        box.height / 2,
      );
      if (radius === 0) return true;
      const cx = Math.min(Math.max(x, box.left + radius), box.right - radius);
      const cy = Math.min(Math.max(y, box.top + radius), box.bottom - radius);
      return Math.hypot(x - cx, y - cy) <= radius - 0.5;
    };
    const axis = (from: number, to: number) => {
      const step = Math.max(6, (to - from) / 40);
      const count = Math.max(1, Math.ceil((to - from) / step));
      return Array.from({ length: count + 1 }, (_, i) => from + ((to - from) * i) / count);
    };
    const describe = (el: Element) => {
      const text = (el.getAttribute("aria-label") || el.textContent || "").trim().replace(/\s+/g, " ").slice(0, 20);
      const box = el.getBoundingClientRect();
      const cls = typeof el.className === "string" && el.className ? `.${el.className.split(" ")[0]}` : "";
      return `${el.tagName.toLowerCase()}${cls}「${text}」${Math.round(box.width)}×${Math.round(box.height)}`;
    };

    // 从控件边缘向外逐像素探测，数连续仍落在该控件上的像素（至多 40px）；blocker 是第一个不属于它的元素，没被挡住时为 null。
    const reach = (el: HTMLElement, box: DOMRect, vertical: boolean, cross: number, sign: -1 | 1) => {
      for (let length = 0; length < 40; length++) {
        const along = length + 0.5;
        const x = vertical ? cross : sign < 0 ? box.left - along : box.right + along;
        const y = vertical ? (sign < 0 ? box.top - along : box.bottom + along) : cross;
        if (x < 0 || y < 0 || x >= innerWidth || y >= innerHeight) return { length, blocker: null };
        const hit = document.elementFromPoint(x, y);
        if (!ownHit(hit, el)) return { length, blocker: hit ?? document.documentElement };
      }
      return { length: 40, blocker: null };
    };

    const uncovered: string[] = [];
    const blocked: string[] = [];
    const stolen: string[] = [];
    let blockedMin = 0;
    let boxUnder44 = 0;
    let reached44 = 0;
    for (const el of victims) {
      el.scrollIntoView({ block: "center", inline: "nearest", behavior: "instant" as ScrollBehavior });
      const box = el.getBoundingClientRect();
      const inContent = Boolean(el.closest(".settings-content"));

      // 自身区域：点不能被别的元素抢走。页头、保存栏与手机底部导航是固定盖层，只对内容区里的控件才算“抢”。
      let lost = 0;
      let thief = "";
      for (const x of axis(box.left + 1, box.right - 1)) {
        for (const y of axis(box.top + 1, box.bottom - 1)) {
          if (x < 0 || y < 0 || x >= innerWidth || y >= innerHeight || !insideShape(el, box, x, y)) continue;
          const hit = document.elementFromPoint(x, y);
          if (belongsTo(hit, el) || (inContent && hit && !hit.closest(".settings-content"))) continue;
          lost += 1;
          thief ||= hit ? describe(hit) : "null";
        }
      }
      if (lost) stolen.push(`${describe(el)} 自身区域有 ${lost} 个点被 ${thief} 抢走`);

      // 点击区：自身盒子不足 44px 的方向，补足的那部分也要点得中它。
      if (!controls.includes(el)) continue;
      const short = [
        { vertical: true, size: box.height },
        { vertical: false, size: box.width },
      ].filter(({ size }) => size < MIN);
      if (!short.length) continue;
      boxUnder44 += 1;
      let worst: { total: number; direction: string; before: number; after: number; blocker: Element | null } | undefined;
      for (const { vertical, size } of short) {
        const start = vertical ? box.left : box.top;
        const span = vertical ? box.width : box.height;
        // 探测线避开嵌套控件所在的那一列（行），否则量到的是嵌套控件的点击区。
        const nested = victims.filter((other) => other !== el && el.contains(other)).map((other) => other.getBoundingClientRect());
        const lines = span > 8 ? [start + 2, start + span / 2, start + span - 2] : [start + span / 2];
        const free = lines.filter((line) => !nested.some((n) => (vertical ? line >= n.left - 1 && line <= n.right + 1 : line >= n.top - 1 && line <= n.bottom + 1)));
        for (const cross of free.length ? free : lines) {
          const before = reach(el, box, vertical, cross, -1);
          const after = reach(el, box, vertical, cross, 1);
          const total = size + before.length + after.length;
          if (worst && total >= worst.total) continue;
          const limiting = [before, after].filter((side) => side.blocker).sort((a, b) => a.length - b.length)[0];
          worst = { total, direction: vertical ? "高" : "宽", before: before.length, after: after.length, blocker: limiting?.blocker ?? null };
        }
      }
      if (!worst || worst.total >= MIN - 0.5) {
        reached44 += 1;
        continue;
      }
      const other = victimOf(worst.blocker);
      const text = `${describe(el)} ${worst.direction}方向点击区只有 ${Math.floor(worst.total)}px（向前 ${worst.before}、向后 ${worst.after}）`;
      if (other && other !== el) {
        blocked.push(`${text}，再往外是 ${describe(other)}`);
        blockedMin = blockedMin ? Math.min(blockedMin, worst.total) : worst.total;
      } else uncovered.push(`${text}，截断处是 ${worst.blocker ? describe(worst.blocker) : "视口边缘"}`);
    }
    return { controls: controls.length, boxUnder44, reached44, uncovered, blocked, blockedMin: Math.floor(blockedMin), stolen };
  }, rootSelector);
}
