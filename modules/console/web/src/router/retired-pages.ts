// 已退役的旧主题分段页面：Stellar 2 迁移后，这些页面上的配置已并入「系统设置」的对应页。
// 旧地址保留为入口兼容层，统一重定向到设置页，避免旧书签、旧通知里的链接变成死胡同；
// 页面组件本身已删除，设置只走 /settings 的统一草稿与保存协议。
export type RetiredConsolePage = "appearance" | "menus" | "authors";

interface RetiredPageTarget {
  /** 设置页注册表（settings-pages.ts）中的页面键。 */
  tab: string;
  /** 旧页面自己处理过的特殊 ?tab= 映射；未列出的值一律落到该页默认目标。 */
  legacyTabs?: Record<string, string>;
}

const retiredPages: Record<RetiredConsolePage, RetiredPageTarget> = {
  appearance: { tab: "appearance", legacyTabs: { feedback: "feedback" } },
  menus: { tab: "navigation" },
  authors: { tab: "community" },
};

/** 旧页面地址 → 系统设置页；保留旧页面对 ?tab=feedback 的既有改道。 */
export function retiredConsoleRedirect(page: RetiredConsolePage, tab: unknown) {
  const target = retiredPages[page];
  const legacy = typeof tab === "string" ? target.legacyTabs?.[tab] : undefined;
  return { path: "/settings", query: { tab: legacy ?? target.tab } };
}
