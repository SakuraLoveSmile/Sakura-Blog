<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { pluginApi, type RuntimePluginManifest } from "@/api";
import { VCard, VEmpty, VLoading, VPageHeader, VTag } from "@/components/ui";

const items = ref<RuntimePluginManifest[]>([]);
const loading = ref(true);
const error = ref("");

async function loadPlugins() {
  loading.value = true;
  error.value = "";
  try {
    const response = await pluginApi.runtime();
    items.value = response.data.items;
  } catch (cause: unknown) {
    error.value = cause instanceof Error ? cause.message : "脚本插件清单加载失败";
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void loadPlugins();
});
</script>

<template>
  <div>
    <VPageHeader title="脚本插件" subtitle="服务端已加载的脚本插件及其权限声明" />
    <div class="mx-auto max-w-[1100px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="loading" />
      <VEmpty v-else-if="error" title="无法加载脚本插件" :message="error">
        <template #actions>
          <button type="button" class="retry-button" @click="loadPlugins">
            <Icon icon="ri:refresh-line" />
            重试
          </button>
        </template>
      </VEmpty>
      <VEmpty
        v-else-if="items.length === 0"
        title="暂无脚本插件"
        message="服务端 plugins 目录中尚未加载可用的脚本插件。"
      />
      <div v-else class="grid gap-4 md:grid-cols-2">
        <VCard v-for="plugin in items" :key="plugin.id" :title="plugin.name || plugin.id">
          <template #header>
            <div class="flex min-w-0 items-center justify-between gap-3">
              <div class="min-w-0">
                <h2 class="truncate text-sm font-semibold text-foreground">{{ plugin.name || plugin.id }}</h2>
                <p class="mt-1 truncate font-mono text-xs text-text-tertiary">{{ plugin.id }}</p>
              </div>
              <VTag theme="secondary" rounded>{{ plugin.version || "未知版本" }}</VTag>
            </div>
          </template>

          <dl class="space-y-3 text-xs">
            <div class="flex items-start justify-between gap-4">
              <dt class="text-text-tertiary">类型</dt>
              <dd class="font-mono text-text-secondary">{{ plugin.kind || "script" }}</dd>
            </div>
            <div>
              <dt class="mb-2 text-text-tertiary">Hooks</dt>
              <dd v-if="plugin.hooks.length" class="flex flex-wrap gap-2">
                <VTag v-for="hook in plugin.hooks" :key="hook" theme="primary" rounded>{{ hook }}</VTag>
              </dd>
              <span v-else class="text-text-quaternary">未声明</span>
            </div>
            <div>
              <dt class="mb-2 text-text-tertiary">权限</dt>
              <dd v-if="plugin.permissions.length" class="flex flex-wrap gap-2">
                <VTag v-for="permission in plugin.permissions" :key="permission" rounded>{{ permission }}</VTag>
              </dd>
              <span v-else class="text-text-quaternary">未声明</span>
            </div>
            <div v-if="plugin.network_allowlist?.length">
              <dt class="mb-2 text-text-tertiary">网络白名单</dt>
              <dd class="space-y-1 font-mono text-text-secondary">
                <div v-for="origin in plugin.network_allowlist" :key="origin">{{ origin }}</div>
              </dd>
            </div>
          </dl>
        </VCard>
      </div>
    </div>
  </div>
</template>

<style scoped>
.retry-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 34px;
  padding: 0 14px;
  border-radius: var(--radius-full);
  color: var(--color-foreground);
  background: var(--color-primary);
  font-size: 13px;
  font-weight: 600;
}

.retry-button:hover {
  opacity: 0.9;
}
</style>
