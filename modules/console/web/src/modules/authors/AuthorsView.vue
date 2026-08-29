<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VCard, VPageHeader } from "@/components/ui";
import AuthorsTab from "@/modules/settings/AuthorsTab.vue";
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
  } catch { loadError.value = "加载作者设置失败"; }
  finally { loading.value = false; }
}
async function save() {
  saving.value = true;
  try { await settingsApi.updateSection("authors", data.value.authors); Toast.success("作者设置已保存"); }
  catch (error) { Toast.error(getApiErrorMessage(error, "作者保存失败")); }
  finally { saving.value = false; }
}
onMounted(() => void load());
</script>

<template>
  <div class="authors-page"><VPageHeader title="作者"><template #icon><Icon icon="ri:user-star-line" class="text-primary" /></template><template #subtitle>管理文章 front matter 可引用的作者资料</template></VPageHeader><main class="authors-container"><div v-if="loading" class="authors-state">加载中...</div><div v-else-if="loadError" class="authors-state authors-error">{{ loadError }}<VButton type="secondary" size="sm" @click="load">重试</VButton></div><VCard v-else title="作者管理"><AuthorsTab :data="data.authors" :saving="saving ? 'authors' : ''" @save="save" /></VCard></main></div>
</template>

<style scoped>
.authors-page { min-height: 100%; background: var(--color-background); }
.authors-container { width: 100%; max-width: 1100px; margin: 0 auto; padding: 24px; box-sizing: border-box; }
.authors-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--color-text-tertiary); font-size: 13px; }
.authors-error { color: var(--color-state-error); }
@media (max-width: 640px) { .authors-container { padding: 12px; } }
</style>
