<script setup lang="ts">
// Halo VEntityContainer 复刻（packages/components/src/components/entity/EntityContainer.vue）
// 表格容器，渲染 table > tbody，由子级 VEntity 提供 tr 行
</script>

<template>
  <div class="w-full overflow-x-auto">
    <table class="w-full border-spacing-0">
      <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
        <slot></slot>
      </tbody>
    </table>
  </div>
</template>
