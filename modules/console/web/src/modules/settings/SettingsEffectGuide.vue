<script setup lang="ts">
import { computed } from "vue";
import { settingsCategoryGuides } from "./settings-category-guide";

const props = withDefaults(
  defineProps<{
    tab: string;
    previewAvailable?: boolean;
    dirty?: boolean;
  }>(),
  { previewAvailable: false, dirty: false },
);
const emit = defineEmits<{ preview: [] }>();
const guide = computed(() => settingsCategoryGuides[props.tab]);
</script>

<template>
  <section v-if="guide" class="settings-effect-guide" :aria-label="`${guide.title}效果说明`">
    <div class="guide-line">
      <p class="guide-result">
        <span class="sr-only">{{ guide.title }}：</span>
        {{ guide.result }}
      </p>
      <span v-if="dirty" class="guide-dirty"><span aria-hidden="true"></span>未保存修改</span>
      <button v-if="previewAvailable" class="guide-preview" type="button" @click="emit('preview')">
        查看草稿效果
      </button>
    </div>

    <details class="guide-explanation">
      <summary>影响位置与检查方法</summary>
      <div class="guide-details">
        <div class="guide-detail">
          <h3>影响位置</h3>
          <p>{{ guide.impact }}</p>
        </div>
        <div class="guide-detail">
          <h3>如何确认</h3>
          <p>{{ guide.verify }}</p>
        </div>
        <p class="guide-save">{{ guide.save }}</p>
      </div>
    </details>
  </section>
</template>

<style scoped>
/* 弱化为一条安静说明 + 可展开细节；信息不删除。 */
.settings-effect-guide {
  display: grid;
  gap: 4px;
  padding: 12px 0 8px;
  border-bottom: 1px solid var(--color-border);
}

.guide-line {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px 16px;
  min-width: 0;
}

.guide-result {
  margin: 0;
  flex: 1 1 420px;
  min-width: 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.guide-dirty {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--color-state-warning);
  font-size: 12px;
  white-space: nowrap;
}

.guide-dirty span {
  width: 5px;
  height: 5px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: currentColor;
}

.guide-preview {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--color-text-secondary);
  font: inherit;
  font-size: 12px;
  text-decoration: underline;
  text-underline-offset: 3px;
  cursor: pointer;
}
.guide-preview:hover {
  color: var(--color-foreground);
}
.guide-preview:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.guide-explanation summary {
  min-height: 32px;
  align-content: center;
  color: var(--color-text-tertiary);
  font-size: 12px;
  cursor: pointer;
}
.guide-explanation summary:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.guide-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 24px;
  padding: 4px 0 8px;
  max-width: 900px;
}

.guide-detail {
  min-width: 0;
}

.guide-detail h3 {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.5;
}

.guide-detail p {
  margin: 2px 0 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.guide-save {
  grid-column: 1 / -1;
  margin: 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 767px) {
  .settings-effect-guide {
    padding: 12px 0 8px;
  }
  .guide-line {
    flex-direction: column;
    gap: 6px;
  }
  .guide-result {
    flex-basis: auto;
  }
  .guide-preview {
    min-height: 44px;
  }
  .guide-explanation summary {
    min-height: 44px;
  }
  .guide-details {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }
}
</style>
