import { defineStore } from "pinia";
import { ref } from "vue";
import { moduleApi, type ModuleManifestItem, type ModuleNav } from "@/api";
import { syncPluginWidgets } from "@/plugin-registry";

export type ModuleItem = ModuleManifestItem;
export type { ModuleNav };

/**
 * 功能模块状态：登录后拉取 manifest，供动态菜单与模块管理页使用。
 * enabledNav 提供已启用模块贡献的菜单项（DefaultLayout 合并进静态导航）。
 */
export const useModulesStore = defineStore("modules", () => {
  const items = ref<ModuleItem[]>([]);
  const loaded = ref(false);

  async function fetch() {
    try {
      const { data } = await moduleApi.list();
      items.value = data.items || [];
      await syncPluginWidgets(new Set(items.value.filter((item) => item.enabled).map((item) => item.id)));
      loaded.value = true;
    } catch {
      // 拉取失败不阻塞控制台，动态菜单降级为空
      items.value = [];
      await syncPluginWidgets(new Set());
      loaded.value = true;
    }
  }

  /** 已启用模块贡献的全部菜单项（扁平） */
  function enabledNav(): ModuleNav[] {
    return items.value.filter((m) => m.enabled).flatMap((m) => m.nav || []);
  }

  /** 某模块是否启用（页面级 guard 用） */
  function isEnabled(id: string): boolean {
    return items.value.some((m) => m.id === id && m.enabled);
  }

  async function toggle(id: string, enabled: boolean) {
    await moduleApi.toggle(id, enabled);
    const it = items.value.find((m) => m.id === id);
    if (it) it.enabled = enabled;
    await syncPluginWidgets(new Set(items.value.filter((item) => item.enabled).map((item) => item.id)));
  }

  return { items, loaded, fetch, enabledNav, isEnabled, toggle };
});
