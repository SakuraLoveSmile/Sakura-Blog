<script setup lang="ts">
import { onMounted } from"vue";
import { useRouter } from"vue-router";
import { Icon } from"@iconify/vue";
import { VPageHeader, VButton, VCard, VTag } from"@/components/ui";
import { widgetRegistry, sizeLabel } from"./widgets/registry";
import { loadDashboardLayout } from"./widgets/layout";
import { useDashboardEditorStore } from"@/stores/dashboard-editor";

const router = useRouter();
const editor = useDashboardEditorStore();

// 直接访问本页（未处于编辑态）时：拉取当前布局并进入编辑态，
// 保证添加行为可用且返回仪表盘后处于编辑态。
onMounted(async () => {
 if (editor.editing) return;
 editor.start(await loadDashboardLayout());
});

function backToDashboard() {
 router.push("/dashboard");
}
</script>

<template>
 <div>
 <VPageHeader title="小组件">
 <template #icon>
 <Icon icon="ri:apps-2-line" class="mr-2 self-center text-xl" />
 </template>
 <template #actions>
 <VButton @click="backToDashboard">
 <template #icon>
 <Icon icon="ri:arrow-left-line" class="h-full w-full" />
 </template>
 返回仪表盘
 </VButton>
 </template>
 </VPageHeader>

 <div class="m-0 space-y-4 p-4 md:m-4 md:p-0">
 <!-- 说明条 -->
 <div
 class="flex items-center gap-2 rounded-base border border-blue-500/30 bg-blue-500/10 px-4 py-3 text-sm text-blue-400"
 >
 <Icon icon="ri:information-line" class="text-base" />
 点击「添加」把部件加入仪表盘布局，可连续添加多个，完成后返回仪表盘保存
 </div>

 <!-- 部件卡片网格 -->
 <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
 <VCard v-for="w in widgetRegistry" :key="w.type" :body-class="['p-0!']">
 <!-- 实时预览区：渲染真实部件组件，禁用交互 -->
 <div class="h-44 overflow-hidden border-b border-border bg-background/60 p-3">
 <div
 class="pointer-events-none select-none"
 style="transform: scale(0.75); transform-origin: top left; width: 133.34%"
 aria-hidden="true"
 >
 <component :is="w.component" v-bind="w.props" />
 </div>
 </div>
 <!-- 信息行 -->
 <div class="flex items-center justify-between gap-2 px-4 py-3">
 <div class="flex min-w-0 items-center gap-2">
 <Icon :icon="w.icon" class="flex-shrink-0 text-lg text-text-tertiary" />
 <span class="truncate text-sm font-medium text-foreground">{{ w.label }}</span>
 <VTag>尺寸：{{ sizeLabel[w.size] }}</VTag>
 </div>
 <VButton v-if="!editor.isAdded(w.type)" size="sm" type="secondary" @click="editor.add(w.type)">
 <template #icon>
 <Icon icon="ri:add-line" class="h-full w-full" />
 </template>
 添加
 </VButton>
 <VButton v-else size="sm" disabled>已添加</VButton>
 </div>
 </VCard>
 </div>
 </div>
 </div>
</template>
