<script setup lang="ts">
import { onMounted, computed, reactive, ref } from "vue";
import { Icon } from "@iconify/vue";
import { moduleApi } from "@/api";
import { SchemaForm, type SchemaField, type SchemaValues } from "@/components/forms";
import { VPageHeader, VCard, VSwitch, VTag, VEmpty, VLoading, Toast } from "@/components/ui";
import { useModulesStore, type ModuleItem } from "@/stores/modules";

const store = useModulesStore();

onMounted(() => {
  // 每次进入管理页强制刷新，保证与后端状态一致
  store.fetch();
});

const items = computed<ModuleItem[]>(() => store.items);
const expandedID = ref("");
const configs = ref<Record<string, SchemaValues>>({});
const configLoading = reactive<Record<string, boolean>>({});
const configSaving = reactive<Record<string, boolean>>({});

async function onToggle(m: ModuleItem, next: boolean) {
  try {
    await store.toggle(m.id, next);
    Toast.success(next ? "已启用" : "已停用", { description: m.title });
  } catch {}
}

function configFields(m: ModuleItem): SchemaField[] {
  return (m.config_schema || []) as SchemaField[];
}

function hasConfig(m: ModuleItem): boolean {
  return Boolean(m.config_section && m.config_schema?.length);
}

async function toggleConfig(m: ModuleItem) {
  if (!hasConfig(m) || !m.config_section) return;
  if (expandedID.value === m.id) {
    expandedID.value = "";
    return;
  }
  expandedID.value = m.id;
  if (configs.value[m.id]) return;
  configLoading[m.id] = true;
  try {
    const { data } = await moduleApi.getConfig(m.config_section);
    configs.value = { ...configs.value, [m.id]: data };
  } catch {
    expandedID.value = "";
  } finally {
    configLoading[m.id] = false;
  }
}

async function saveConfig(m: ModuleItem) {
  if (!m.config_section) return;
  configSaving[m.id] = true;
  try {
    await moduleApi.putConfig(m.config_section, configs.value[m.id] || {});
    Toast.success("配置已保存", { description: m.title });
  } catch {
    // transport interceptor presents the normalized error toast
  } finally {
    configSaving[m.id] = false;
  }
}

function updateConfig(id: string, value: SchemaValues) {
  configs.value = { ...configs.value, [id]: value };
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
          class="mb-4 flex items-center gap-2 rounded-base border border-primary bg-primary/10 px-4 py-3 text-sm text-primary border-primary/30 bg-primary/10 text-primary"
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
            class="rounded-base border border-border bg-card p-4 transition-shadow hover:shadow-sm"
          >
            <div class="flex items-start gap-3">
              <span
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-base bg-primary/10 text-primary"
              >
                <Icon :icon="m.nav?.[0]?.icon || 'ri:puzzle-line'" class="text-xl" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="truncate text-sm font-medium text-foreground">{{ m.title }}</span>
                  <VTag>{{ m.category || "模块" }}</VTag>
                  <VTag v-if="m.locked" theme="secondary">核心</VTag>
                </div>
                <p class="mt-1 line-clamp-2 text-xs text-text-tertiary">{{ m.description }}</p>
              </div>
              <div class="flex shrink-0 items-center gap-3">
                <button
                  v-if="hasConfig(m)"
                  type="button"
                  class="rounded-full border border-border px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
                  @click="toggleConfig(m)"
                >
                  {{ expandedID === m.id ? "收起配置" : "配置" }}
                </button>
                <VSwitch
                  :model-value="m.enabled"
                  :disabled="m.locked"
                  class="mt-1"
                  @change="(v: boolean) => onToggle(m, v)"
                />
              </div>
            </div>

            <div v-if="expandedID === m.id" class="mt-4 border-t border-border pt-4">
              <VLoading v-if="configLoading[m.id]" />
              <template v-else>
                <SchemaForm
                  :fields="configFields(m)"
                  :model-value="configs[m.id] || {}"
                  @update:model-value="(value) => updateConfig(m.id, value)"
                />
                <div class="mt-4 flex justify-end">
                  <button
                    type="button"
                    class="rounded-full bg-primary px-4 py-2 text-xs font-medium text-primary-foreground transition-opacity hover:opacity-90 disabled:cursor-wait disabled:opacity-50"
                    :disabled="configSaving[m.id]"
                    @click="saveConfig(m)"
                  >
                    {{ configSaving[m.id] ? "保存中…" : "保存配置" }}
                  </button>
                </div>
              </template>
            </div>
          </div>
        </div>
      </VCard>
    </div>
  </div>
</template>
