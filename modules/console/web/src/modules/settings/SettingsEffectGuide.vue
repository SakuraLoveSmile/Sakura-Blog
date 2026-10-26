<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { VButton } from "@/components/ui";
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
const expanded = ref(false);
let media: MediaQueryList | undefined;
function followWidth() {
  expanded.value = media?.matches ?? false;
}
onMounted(() => {
  media = matchMedia("(min-width: 768px)");
  followWidth();
  media.addEventListener("change", followWidth);
});
onBeforeUnmount(() => media?.removeEventListener("change", followWidth));
</script>

<template>
  <section v-if="guide" class="settings-effect-guide" :aria-label="`${guide.title}效果说明`">
    <div class="guide-heading">
      <div class="guide-heading-copy">
        <p class="guide-eyebrow">调整后会发生什么</p>
        <div class="guide-title-row">
          <h2>{{ guide.title }}</h2>
          <span v-if="dirty" class="guide-dirty"><span aria-hidden="true"></span>未保存修改</span>
        </div>
        <p class="guide-result">{{ guide.result }}</p>
      </div>
      <VButton v-if="previewAvailable" class="guide-preview" type="secondary" size="sm" @click="emit('preview')">
        <template #icon><Icon icon="ri:eye-line" /></template>
        查看草稿效果
      </VButton>
    </div>

    <details class="guide-explanation" :open="expanded" @toggle="expanded = ($event.target as HTMLDetailsElement).open">
      <summary>影响位置与检查方法</summary>
      <div class="guide-details">
        <div class="guide-detail">
          <span class="guide-detail-icon" aria-hidden="true"><Icon icon="ri:layout-line" /></span>
          <div>
            <h3>影响位置</h3>
            <p>{{ guide.impact }}</p>
          </div>
        </div>
        <div class="guide-detail">
          <span class="guide-detail-icon" aria-hidden="true"><Icon icon="ri:checkbox-circle-line" /></span>
          <div>
            <h3>如何确认</h3>
            <p>{{ guide.verify }}</p>
          </div>
        </div>
      </div>
    </details>

    <p class="guide-save">
      <Icon icon="ri:save-3-line" aria-hidden="true" />
      {{ guide.save }}
    </p>
  </section>
</template>

<style scoped>
.settings-effect-guide {
  display: grid;
  gap: 12px;
  padding: 20px 28px 16px;
  border-bottom: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 24%, var(--color-card));
}

.guide-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  min-width: 0;
}

.guide-heading-copy {
  min-width: 0;
  max-width: 760px;
}

.guide-eyebrow {
  margin: 0 0 6px;
  color: var(--color-text-quaternary);
  font-size: 11px;
  font-weight: 600;
}

.guide-title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
}

.guide-title-row h2 {
  margin: 0;
  color: var(--color-foreground);
  font-size: 17px;
  font-weight: 650;
  line-height: 1.35;
}

.guide-result {
  margin: 6px 0 0;
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.guide-dirty {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--color-state-warning);
  font-size: 11px;
  line-height: 1.4;
}

.guide-dirty span {
  width: 6px;
  height: 6px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: currentColor;
}

.guide-preview {
  min-height: 40px;
  flex: 0 0 auto;
}

.guide-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
  max-width: 900px;
}
.guide-explanation summary {
  min-height: 44px;
  align-content: center;
  color: var(--color-text-secondary);
  font-size: 12px;
  cursor: pointer;
}
.guide-explanation summary:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
@media (min-width: 768px) {
  .guide-explanation summary {
    display: none;
  }
}

.guide-detail {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr);
  align-items: start;
  gap: 9px;
  min-width: 0;
}

.guide-detail-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 20px;
  color: var(--color-text-tertiary);
  font-size: 14px;
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
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.guide-save {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  margin: 0;
  color: var(--color-text-quaternary);
  font-size: 11px;
  line-height: 1.5;
}

.guide-save :deep(svg) {
  flex: 0 0 auto;
  margin-top: 1px;
}

.guide-preview:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

@media (max-width: 767px) {
  .settings-effect-guide {
    padding: 18px 16px 16px;
    gap: 14px;
  }

  .guide-heading {
    flex-direction: column;
    gap: 12px;
  }

  .guide-preview {
    min-height: 44px;
  }

  .guide-details {
    grid-template-columns: minmax(0, 1fr);
    gap: 14px;
  }
}
</style>
