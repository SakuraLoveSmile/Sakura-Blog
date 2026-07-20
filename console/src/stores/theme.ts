import { defineStore } from "pinia";
import { ref } from "vue";

export type ThemeMode = "light" | "dark" | "system";

const STORAGE_KEY = "console-theme";

function getStoredMode(): ThemeMode {
  const v = localStorage.getItem(STORAGE_KEY);
  return v === "light" || v === "dark" || v === "system" ? v : "system";
}

function systemPrefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

/**
 * 主题状态管理：亮色 / 暗色 / 跟随系统三段式。
 * - 用户选择持久化到 localStorage；
 * - 实际生效主题通过给 <html> 增删 .dark 类来驱动 Tailwind dark: 变体；
 * - 处于「跟随系统」时实时响应系统偏好变化。
 */
export const useThemeStore = defineStore("theme", () => {
  const mode = ref<ThemeMode>(getStoredMode());

  /** 根据当前 mode 计算并应用 .dark 类。 */
  function apply() {
    const dark =
      mode.value === "dark" ||
      (mode.value === "system" && systemPrefersDark());
    document.documentElement.classList.toggle("dark", dark);
    // 同步 data-theme：站点 highlight.css（Chroma 令牌色）的暗色选择器是
    // :root[data-theme="dark"]，编辑器预览复用该样式需要此桥接。
    document.documentElement.dataset.theme = dark ? "dark" : "light";
  }

  /** 切换主题模式并持久化。 */
  function setMode(next: ThemeMode) {
    mode.value = next;
    localStorage.setItem(STORAGE_KEY, next);
    apply();
  }

  // 系统偏好变化时，若处于 system 模式则实时跟随。
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      if (mode.value === "system") apply();
    });

  return { mode, setMode, apply };
});
