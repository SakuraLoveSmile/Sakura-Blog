import { ref, watch } from "vue";

export type HeadingStyleMode = "panel" | "card" | "tree";

const STORAGE_KEY = "blog_settings_heading_style";
const saved = typeof localStorage !== "undefined" ? (localStorage.getItem(STORAGE_KEY) as HeadingStyleMode) : null;
const initialMode: HeadingStyleMode = saved === "card" || saved === "tree" ? saved : "panel";

export const headingStyleMode = ref<HeadingStyleMode>(initialMode);

if (typeof localStorage !== "undefined") {
  watch(headingStyleMode, (mode) => {
    try {
      localStorage.setItem(STORAGE_KEY, mode);
    } catch {}
  });
}
