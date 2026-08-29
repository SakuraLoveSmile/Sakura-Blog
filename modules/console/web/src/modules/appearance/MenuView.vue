<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VCard, VPageHeader } from "@/components/ui";
import MenubarEditor from "@/modules/settings/MenubarEditor.vue";
import { ensureSettingsDefaults } from "@/modules/settings/settings-state";

const data = ref<Record<string, any>>({});
const loading = ref(true);
const loadError = ref("");
const saving = ref(false);

async function load() {
  loading.value = true;
  try {
    const { data: response } = await settingsApi.all();
    data.value = ensureSettingsDefaults({ ...response });
  } catch {
    loadError.value = "加载菜单设置失败";
  } finally {
    loading.value = false;
  }
}
async function save() {
  saving.value = true;
  try {
    await settingsApi.updateSection("menubar", data.value.menubar);
    Toast.success("菜单已保存，前台已生效");
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "菜单保存失败"));
  } finally {
    saving.value = false;
  }
}
onMounted(() => void load());
</script>

<template>
  <div class="menu-page">
    <VPageHeader title="菜单">
      <template #icon><Icon icon="ri:menu-line" class="text-primary" /></template>
      <template #subtitle>管理主题顶部菜单的标题、链接、图标、主题色与排序</template>
    </VPageHeader>
    <main class="menu-container">
      <div v-if="loading" class="menu-state"><Icon icon="ri:loader-4-line" class="menu-spin" width="22" height="22" />加载中...</div>
      <div v-else-if="loadError" class="menu-state menu-error">{{ loadError }}<VButton type="secondary" size="sm" @click="load">重试</VButton></div>
      <VCard v-else title="顶部菜单"><MenubarEditor :items="data.menubar.items" :columns="data.menubar.columns" :saving="saving ? 'menubar' : ''" @update:columns="data.menubar.columns = $event" @save="save" /></VCard>
    </main>
  </div>
</template>

<style scoped>
.menu-page { min-height: 100%; background: var(--color-background); }
.menu-container { width: 100%; max-width: 1100px; margin: 0 auto; padding: 24px; box-sizing: border-box; }
.menu-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--color-text-tertiary); font-size: 13px; }
.menu-error { color: var(--color-state-error); }
.menu-spin { animation: menu-spin 1s linear infinite; }
@keyframes menu-spin { to { transform: rotate(360deg); } }
@media (max-width: 640px) { .menu-container { padding: 12px; } }
</style>
