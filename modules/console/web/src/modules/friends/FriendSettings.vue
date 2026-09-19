<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VCard } from "@/components/ui";

const data = ref<Record<string, any>>({});
const loading = ref(true);
const saving = ref(false);

async function load() {
  loading.value = true;
  try {
    const { data: response } = await settingsApi.all();
    data.value = { ...(response.friends || {}) };
    data.value.refresh_interval = data.value.refresh_interval ?? 60;
    data.value.posts_per_link = data.value.posts_per_link ?? 20;
    data.value.check_interval = data.value.check_interval ?? 360;
    data.value.snapshot_interval = data.value.snapshot_interval ?? 1440;
    data.value.snapshot_provider = data.value.snapshot_provider || "https://image.thum.io/get/width/1280/crop/720/{url}";
  } catch {
    Toast.error("加载友链设置失败");
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    await settingsApi.updateSection("friends", data.value);
    Toast.success("友链设置已保存");
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "友链设置保存失败"));
  } finally {
    saving.value = false;
  }
}

onMounted(() => void load());
</script>

<template>
  <div class="friend-settings-page">
    <VCard title="友链设置">
      <p class="friend-settings-hint">友链列表的 RSS 聚合、连通性巡检和网站截图缓存配置。</p>
      <div v-if="loading" class="friend-settings-loading"><Icon icon="ri:loader-4-line" class="friend-settings-spin" />加载中...</div>
      <template v-else>
        <div class="friend-settings-grid">
          <div class="friend-settings-field"><label>RSS 刷新间隔（分钟，≤0 用默认 60）</label><input v-model.number="data.refresh_interval" type="number" class="apple-input" /></div>
          <div class="friend-settings-field"><label>每友链文章数量（≤0 用默认 20）</label><input v-model.number="data.posts_per_link" type="number" min="1" class="apple-input" /></div>
          <div class="friend-settings-field"><label>连通性巡检间隔（分钟，≤0 关闭）</label><input v-model.number="data.check_interval" type="number" class="apple-input" /></div>
          <div class="friend-settings-field"><label>截图缓存间隔（分钟，≤0 关闭）</label><input v-model.number="data.snapshot_interval" type="number" class="apple-input" /></div>
          <div class="friend-settings-field friend-settings-wide"><label>截图服务 URL 模板（使用 {url}）</label><input v-model="data.snapshot_provider" class="apple-input" /></div>
        </div>
        <div class="friend-settings-actions"><VButton type="primary" :loading="saving" @click="save"><template #icon><Icon icon="ri:save-line" /></template>保存友链设置</VButton></div>
      </template>
    </VCard>
  </div>
</template>

<style scoped>
.friend-settings-page { width: 100%; max-width: 900px; margin: 0 auto; }
.friend-settings-hint { margin: 0 0 16px; color: var(--color-text-tertiary); font-size: 12px; }
.friend-settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.friend-settings-wide { grid-column: 1 / -1; }
.friend-settings-field label { display: block; margin-bottom: 6px; color: var(--color-text-tertiary); font-size: 12px; }
.friend-settings-actions { display: flex; justify-content: flex-end; margin-top: 24px; }
.friend-settings-loading { display: flex; align-items: center; gap: 8px; padding: 32px 0; color: var(--color-text-tertiary); font-size: 13px; }
.friend-settings-spin { animation: friend-settings-spin 1s linear infinite; }
@keyframes friend-settings-spin { to { transform: rotate(360deg); } }
@media (max-width: 640px) { .friend-settings-grid { grid-template-columns: 1fr; } .friend-settings-wide { grid-column: auto; } }
</style>
