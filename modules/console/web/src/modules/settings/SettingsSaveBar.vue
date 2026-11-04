<script setup lang="ts">
import { computed } from "vue";
import { Icon } from "@iconify/vue";
import { VButton } from "@/components/ui";

// 设置页统一的保存栏。主题页共用同一份草稿，所以显示全部页面的修改数；站点信息与访客反馈独立保存，只说明本页。
const props = withDefaults(
  defineProps<{
    /** theme：整份主题草稿统一保存；host：站点信息、访客反馈各自保存。 */
    scope: "theme" | "host";
    /** 是否有可保存的修改；没有时保存按钮禁用。 */
    dirty: boolean;
    /** 本页自己是否有修改，决定“放弃本页修改”是否可用。 */
    pageDirty: boolean;
    /** 未保存修改处数（主题页为全部页面之和，独立保存页只算本页）。 */
    total?: number;
    /** 主题页的分页计数。 */
    breakdown?: { label: string; count: number }[];
    /** 独立保存页：主题草稿在其他页另有多少处未保存。 */
    otherThemeCount?: number;
    /** 本栏对应的保存请求进行中。 */
    saving?: boolean;
    /** 其他保存请求进行中，暂不能提交或放弃。 */
    busy?: boolean;
    /** 阻止保存的原因（输入有误、有未加入的映射项等），为空表示可以保存。 */
    blocked?: string;
  }>(),
  { total: 0, breakdown: () => [], otherThemeCount: 0, saving: false, busy: false, blocked: "" },
);
const emit = defineEmits<{ discard: [] }>();

const title = computed(() => {
  if (!props.dirty) return "没有未保存修改";
  if (props.total > 0) return `${props.total} 处未保存修改`;
  return "有未保存的配置改动";
});
const detail = computed(() => {
  if (props.scope === "host")
    return props.otherThemeCount > 0 ? `仅保存本页；主题草稿另有 ${props.otherThemeCount} 处未保存` : "仅保存本页";
  const parts = props.breakdown.filter((item) => item.count > 0).map((item) => `${item.label} ${item.count}`);
  return [...parts, "草稿在各页共享"].join(" · ");
});
</script>

<template>
  <div class="settings-save-bar" role="group" aria-label="保存操作">
    <div class="settings-save-info">
      <span v-if="dirty" class="settings-save-dot" aria-hidden="true"></span>
      <strong class="settings-save-title">{{ title }}</strong>
      <span v-if="blocked" class="settings-save-detail settings-error" role="alert">{{ blocked }}</span>
      <span v-else class="settings-save-detail">{{ detail }}</span>
    </div>
    <div class="settings-save-actions">
      <VButton type="ghost" size="md" :disabled="!pageDirty || busy" @click="emit('discard')"
        >放弃本页修改</VButton
      >
      <VButton
        type="primary"
        size="md"
        native-type="submit"
        :disabled="!dirty || busy || Boolean(blocked)"
        :loading="saving"
        ><template #icon><Icon icon="ri:save-line" /></template
        >{{ scope === "theme" ? "保存全部主题设置" : "保存" }}</VButton
      >
    </div>
  </div>
</template>

<style scoped>
.settings-save-bar {
  position: sticky;
  /* 预览面板停靠在底部时，保存栏停在它上方而不是被盖住。 */
  bottom: var(--settings-dock-offset, 0px);
  z-index: 10;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  margin-top: 32px;
  /* 横向留白由 .settings-container 统一提供，保存栏只补纵向安全区。 */
  padding: 14px 0 calc(14px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: var(--color-background);
}
.settings-save-info {
  display: flex;
  flex: 1 1 280px;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 10px;
  min-width: 0;
}
.settings-save-dot {
  align-self: center;
  width: 6px;
  height: 6px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--color-state-warning);
}
.settings-save-title {
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 600;
}
.settings-save-detail {
  min-width: 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.settings-save-detail.settings-error {
  color: var(--color-state-error);
}
.settings-save-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
}
@media (max-width: 767px) {
  .settings-save-bar {
    margin-top: 24px;
  }
  .settings-save-info {
    flex-basis: 100%;
  }
  .settings-save-actions {
    flex: 1 1 100%;
    justify-content: flex-end;
  }
}
</style>
