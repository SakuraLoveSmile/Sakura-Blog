import { defineStore } from "pinia";
import { ref } from "vue";
import { moduleApi } from "@/api";

// 模块贡献的 Console 菜单项（对齐后端 module.Nav）
export interface ModuleNav {
  path: string;
  label: string;
  icon: string;
  group: string;
}

// 模块 manifest 条目（对齐后端 module.ManifestItem）
export interface ModuleItem {
  id: string;
  title: string;
  description: string;
  category: string;
  nav: ModuleNav[];
  enabled: boolean;
}

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
      loaded.value = true;
    } catch {
      // 拉取失败不阻塞控制台，动态菜单降级为空
      items.value = [];
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
  }

  return { items, loaded, fetch, enabledNav, isEnabled, toggle };
});
