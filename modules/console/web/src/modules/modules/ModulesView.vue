<script setup lang="ts">
import { onMounted, computed, reactive, ref } from "vue";
import { moduleApi } from "@/api";
import { SchemaForm, type SchemaField, type SchemaValues } from "@/components/forms";
import { VButton, VPageHeader, VSwitch, VTag, VEmpty, VLoading, Toast } from "@/components/ui";
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
  <div class="modules-page">
    <VPageHeader title="功能模块" />

    <div class="modules-container">
      <p class="modules-note">功能模块以内置形式随程序编译，可在此启用/停用。停用后其菜单入口隐藏、后台接口关闭。</p>

      <VLoading v-if="!store.loaded && !items.length" />

      <VEmpty v-else-if="!items.length" title="暂无功能模块" message="尚未注册任何功能模块" />

      <ul v-else class="module-records">
        <li v-for="m in items" :key="m.id" class="module-record">
          <div class="module-main">
            <div class="module-title-line">
              <span class="module-title">{{ m.title }}</span>
              <VTag>{{ m.category || "模块" }}</VTag>
              <VTag v-if="m.locked" theme="secondary">核心</VTag>
            </div>
            <p class="module-description">{{ m.description }}</p>
          </div>
          <div class="module-actions">
            <VButton v-if="hasConfig(m)" size="sm" @click="toggleConfig(m)">
              {{ expandedID === m.id ? "收起配置" : "配置" }}
            </VButton>
            <VSwitch
              :model-value="m.enabled"
              :disabled="m.locked"
              :aria-label="`${m.title || m.id} 开关`"
              @change="(v: boolean) => onToggle(m, v)"
            />
          </div>

          <div v-if="expandedID === m.id" class="module-config">
            <VLoading v-if="configLoading[m.id]" />
            <template v-else>
              <SchemaForm
                :fields="configFields(m)"
                :model-value="configs[m.id] || {}"
                @update:model-value="(value) => updateConfig(m.id, value)"
              />
              <div class="module-config-actions">
                <VButton type="primary" size="sm" :disabled="configSaving[m.id]" @click="saveConfig(m)">
                  {{ configSaving[m.id] ? "保存中…" : "保存配置" }}
                </VButton>
              </div>
            </template>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.modules-page {
  min-height: 100%;
  background: var(--color-background);
}
.modules-container {
  width: 100%;
  max-width: 1080px;
  margin: 0;
  padding: 8px 32px 40px;
  box-sizing: border-box;
}
.modules-note {
  margin: 0;
  padding: 12px 0;
  border-bottom: 1px solid var(--color-border);
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}
.module-records {
  margin: 0;
  padding: 0;
  list-style: none;
}
.module-record {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 16px;
  padding: 16px 0;
  border-bottom: 1px solid var(--color-border);
}
.module-main {
  flex: 1 1 320px;
  min-width: 0;
}
.module-title-line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.module-title {
  color: var(--color-foreground);
  font-size: 14px;
  font-weight: 500;
}
.module-description {
  margin: 4px 0 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}
.module-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.module-config {
  flex-basis: 100%;
  padding-top: 16px;
  border-top: 1px solid var(--color-border);
}
.module-config-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
@media (max-width: 767px) {
  .modules-container {
    padding: 0 16px 32px;
  }
  .module-actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
