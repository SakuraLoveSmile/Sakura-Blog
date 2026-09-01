import { ref } from "vue";
import { pluginApi } from "@/api";
import type { PluginMeta } from "./plugin-types";
import { mergePluginCatalog, type PluginCatalogItem } from "./plugin-catalog";

export const runtimePlugins = ref<PluginMeta[]>([]);
export type CatalogStatus = "idle" | "loading" | "success" | "error";
export const catalogStatus = ref<CatalogStatus>("idle");
export const catalogError = ref("");

let loadPromise: Promise<PluginMeta[]> | null = null;

/** Load the parser-owned catalog once; the API is the runtime source of truth. */
export function loadPluginCatalog(): Promise<PluginMeta[]> {
  if (loadPromise) return loadPromise;

  catalogStatus.value = "loading";
  catalogError.value = "";
  const request = pluginApi
    .catalog()
    .then(({ data }) => {
      runtimePlugins.value = mergePluginCatalog([], data.items as PluginCatalogItem[]);
      catalogStatus.value = "success";
      return runtimePlugins.value;
    })
    .catch((cause: unknown) => {
      catalogStatus.value = "error";
      catalogError.value = cause instanceof Error ? cause.message : "标签插件目录加载失败";
      loadPromise = null;
      return runtimePlugins.value;
    });

  loadPromise = request;
  return loadPromise;
}

/** 清理失败态并重新请求目录，供独立路由的错误态使用。 */
export function retryLoadCatalog(): void {
  loadPromise = null;
  void loadPluginCatalog();
}
