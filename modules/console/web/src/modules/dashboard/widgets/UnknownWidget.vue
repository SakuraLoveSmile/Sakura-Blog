<script setup lang="ts">
defineProps<{ widgetId: string }>();
</script>

<template>
  <div class="apple-card h-full flex items-center justify-center text-center text-muted-foreground">
    <div>
      <div class="text-sm font-medium">组件暂不可用</div>
      <div class="mt-1 text-xs opacity-70">{{ widgetId }}</div>
    </div>
  </div>
</template>
