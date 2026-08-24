import { createI18n } from "vue-i18n";
import zhCN from "./locales/zh-CN";

export const supportedLocales = ["zh-CN", "en-US"] as const;
export type SupportedLocale = (typeof supportedLocales)[number];

export const i18n = createI18n({
  legacy: false,
  locale: "zh-CN" as SupportedLocale,
  fallbackLocale: "zh-CN",
  messages: { "zh-CN": zhCN } as Record<SupportedLocale, typeof zhCN>,
});

const loaded = new Set<SupportedLocale>(["zh-CN"]);

export async function loadLocale(locale: SupportedLocale): Promise<void> {
  if (!loaded.has(locale)) {
    const messages = await import("./locales/en-US");
    i18n.global.setLocaleMessage(locale, messages.default);
    loaded.add(locale);
  }
  i18n.global.locale.value = locale;
}
