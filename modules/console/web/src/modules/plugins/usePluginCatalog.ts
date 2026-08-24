import { ref } from "vue";
import { pluginApi } from "@/api";
import type { PluginMeta } from "./plugin-types";
import { mergePluginCatalog, type PluginCatalogItem } from "./plugin-catalog";

export const runtimePlugins = ref<PluginMeta[]>([]);

let loadPromise: Promise<PluginMeta[]> | null = null;

/** Load the parser-owned catalog once; the API is the runtime source of truth. */
export function loadPluginCatalog(): Promise<PluginMeta[]> {
  if (loadPromise) return loadPromise;

  loadPromise = pluginApi
    .catalog()
    .then(({ data }) => {
      if (data.items.length > 0) {
        runtimePlugins.value = mergePluginCatalog([], data.items as PluginCatalogItem[]);
      }
      return runtimePlugins.value;
    })
    .catch(() => runtimePlugins.value);

  return loadPromise;
}
