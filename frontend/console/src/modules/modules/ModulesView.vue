<script setup lang="ts">
import { onMounted, computed } from "vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VSwitch, VTag, VEmpty, VLoading, Toast } from "@/components/ui";
import { useModulesStore, type ModuleItem } from "@/stores/modules";

const store = useModulesStore();

onMounted(() => {
  // 每次进入管理页强制刷新，保证与后端状态一致
  store.fetch();
});

const items = computed<ModuleItem[]>(() => store.items);

async function onToggle(m: ModuleItem, next: boolean) {
  try {
    await store.toggle(m.id, next);
    Toast.success(next ? "已启用" : "已停用", { description: m.title });
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "操作失败");
  }
}
</script>

<template>
  <div>
    <VPageHeader title="功能模块">
      <template #icon><Icon icon="ri:apps-2-line" class="text-primary" /></template>
    </VPageHeader>

    <div class="m-0 p-4 md:m-4 md:p-0">
      <VCard>
        <!-- 说明条 -->
        <div
          class="mb-4 flex items-center gap-2 rounded-base border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-400"
        >
          <Icon icon="ri:information-line" class="text-base" />
          功能模块以内置形式随程序编译，可在此启用/停用。停用后其菜单入口隐藏、后台接口关闭。
        </div>

        <VLoading v-if="!store.loaded && !items.length" />

        <VEmpty v-else-if="!items.length" title="暂无功能模块" message="尚未注册任何功能模块" />

        <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div
            v-for="m in items"
            :key="m.id"
            class="flex items-start gap-3 rounded-base border border-gray-200 bg-white p-4 transition-shadow hover:shadow-sm dark:border-gray-700 dark:bg-gray-800"
          >
            <span
              class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-base bg-primary/10 text-primary"
            >
              <Icon :icon="m.nav?.[0]?.icon || 'ri:puzzle-line'" class="text-xl" />
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">{{ m.title }}</span>
                <VTag>{{ m.category || "模块" }}</VTag>
              </div>
              <p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">{{ m.description }}</p>
            </div>
            <VSwitch :model-value="m.enabled" class="mt-1" @change="(v: boolean) => onToggle(m, v)" />
          </div>
        </div>
      </VCard>
    </div>
  </div>
</template>
