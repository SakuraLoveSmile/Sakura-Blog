<script setup lang="ts">
// VSkeleton（Apple Dark 流光骨架屏）
withDefaults(
  defineProps<{
    type?: "row" | "card" | "text" | "avatar" | "list";
    count?: number;
    rows?: number;
  }>(),
  {
    type: "row",
    count: 5,
    rows: 4,
  },
);
</script>

<template>
  <div class="skeleton-container w-full space-y-3 p-4">
    <!-- 行列表骨架（Table/Entity 行） -->
    <template v-if="type === 'row' || type === 'list'">
      <div
        v-for="i in count"
        :key="i"
        class="flex items-center justify-between gap-4 py-3 border-b border-border/50 last:border-0"
      >
        <div class="flex items-center gap-3 min-w-0 flex-1">
          <div class="h-4 w-4 rounded skeleton shrink-0" />
          <div class="h-10 w-14 rounded-md skeleton shrink-0 hidden sm:block" />
          <div class="space-y-2 flex-1 min-w-0">
            <div class="h-4 w-1/3 rounded skeleton" />
            <div class="h-3 w-1/2 rounded skeleton opacity-70" />
          </div>
        </div>
        <div class="flex items-center gap-4 shrink-0">
          <div class="h-5 w-16 rounded-full skeleton hidden sm:block" />
          <div class="h-4 w-12 rounded skeleton" />
        </div>
      </div>
    </template>

    <!-- 卡片网格骨架（Dashboard / Attachments） -->
    <template v-else-if="type === 'card'">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div v-for="i in count" :key="i" class="rounded-xl border border-border bg-card p-4 space-y-3">
          <div class="h-32 w-full rounded-lg skeleton" />
          <div class="h-4 w-3/4 rounded skeleton" />
          <div class="h-3 w-1/2 rounded skeleton opacity-70" />
        </div>
      </div>
    </template>

    <!-- 纯文本行骨架 -->
    <template v-else-if="type === 'text'">
      <div v-for="i in rows || count" :key="i" class="space-y-2">
        <div
          class="h-3.5 rounded skeleton"
          :style="{ width: i === (rows || count) ? '60%' : `${85 + ((i * 17) % 15)}%` }"
        />
      </div>
    </template>

    <!-- 头像骨架 -->
    <template v-else-if="type === 'avatar'">
      <div class="h-10 w-10 rounded-full skeleton shrink-0" />
    </template>
  </div>
</template>

<style scoped>
.skeleton {
  background: var(--color-card);
  animation: skeleton-pulse 1.8s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
</style>
