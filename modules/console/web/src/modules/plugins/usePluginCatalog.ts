import { ref } from "vue";
import { pluginApi } from "@/api";
import { plugins, type PluginMeta } from "./plugins-data";
import { mergePluginCatalog, type PluginCatalogItem } from "./plugin-catalog";

export const runtimePlugins = ref<PluginMeta[]>(plugins);

let loadPromise: Promise<PluginMeta[]> | null = null;

/** Load the parser-owned catalog once; static metadata remains the offline fallback. */
export function loadPluginCatalog(): Promise<PluginMeta[]> {
  if (loadPromise) return loadPromise;

  loadPromise = pluginApi
    .catalog()
    .then(({ data }) => {
      if (data.items.length > 0) {
        runtimePlugins.value = mergePluginCatalog(plugins, data.items as PluginCatalogItem[]);
      }
      return runtimePlugins.value;
    })
    .catch(() => runtimePlugins.value);

  return loadPromise;
}
