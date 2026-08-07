<script setup lang="ts">
import { ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VCard } from "@/components/ui";

const props = withDefaults(
  defineProps<{
    title: string;
    subtitle?: string;
    defaultOpen?: boolean;
  }>(),
  {
    subtitle: "",
    defaultOpen: false,
  }
);

const open = ref(props.defaultOpen);

// 外部（PageEditor 的 configSectionOpen 变了）能强制展开
watch(
  () => props.defaultOpen,
  (val) => {
    if (val) open.value = true;
  },
  { immediate: false }
);
</script>

<template>
  <VCard>
    <div>
      <div
        class="flex cursor-pointer items-center justify-between gap-4 -m-3 p-3 hover:bg-secondary dark:hover:bg-accent/20 rounded-inherit"
        @click="open = !open"
      >
        <div class="min-w-0">
          <div class="text-base font-bold text-foreground dark:text-foreground">{{ title }}</div>
          <div
            v-if="subtitle"
            class="mt-0.5 text-xs font-normal text-text-quaternary dark:text-muted-foreground"
          >
            {{ subtitle }}
          </div>
        </div>
        <Icon
          :icon="open ? 'ri:arrow-up-s-line' : 'ri:arrow-down-s-line'"
          class="h-5 w-5 shrink-0 text-muted-foreground"
        />
      </div>
      <div
        v-if="open"
        class="mt-4 pt-4 border-t border-border dark:border-border-strong"
      >
        <slot />
      </div>
    </div>
  </VCard>
</template>
