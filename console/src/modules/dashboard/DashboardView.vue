<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useRouter } from "vue-router";
import { dashboardApi, type DashboardLayoutItem } from "@/api";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, VSpace, Toast } from "@/components/ui";
import { getWidget, sizeClass, defaultLayout } from "./widgets/registry";
import { useDashboardStats } from "./widgets/useDashboardStats";
import { useDashboardEditorStore } from "@/stores/dashboard-editor";

const router = useRouter();

// 待审核提醒横幅（固定展示，不纳入部件体系）
const { stats } = useDashboardStats();

// 编辑态与草稿在 store：跳转小组件中心页再返回时不丢失
const editor = useDashboardEditorStore();

// --- 布局 ---
const layout = ref<DashboardLayoutItem[]>([]);
const loaded = ref(false);

// 渲染源：编辑态渲染草稿，浏览态渲染已保存布局
const items = computed(() => (editor.editing ? editor.draft : layout.value));

onMounted(async () => {
  try {
    const { data } = await dashboardApi.getLayout();
    // 过滤掉注册表中不存在的部件类型（前后端版本不一致时兜底）
    layout.value = (data.items || []).filter((it: DashboardLayoutItem) => getWidget(it.type));
    if (!layout.value.length && !editor.editing) layout.value = [...defaultLayout];
  } catch {
    layout.value = [...defaultLayout];
  }
  loaded.value = true;
});

// --- 编辑态 ---
const saving = ref(false);

function startEditing() {
  editor.start(layout.value);
}

function cancelEditing() {
  layout.value = editor.cancel();
}

async function saveLayout() {
  saving.value = true;
  try {
    await dashboardApi.saveLayout(editor.draft);
    layout.value = editor.finish();
    Toast.success("仪表盘布局已保存");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || "保存失败");
  } finally {
    saving.value = false;
  }
}

function removeWidget(index: number) {
  editor.remove(index);
}

// 「添加部件」：跳转独立的小组件中心页
function goWidgetCenter() {
  router.push("/dashboard/widgets");
}

// --- 拖拽排序（复用 MenubarEditor 的原生 dnd 模式） ---
const dragIndex = ref<number | null>(null);

function onDragStart(i: number) {
  dragIndex.value = i;
}
function onDragOver(e: DragEvent, i: number) {
  e.preventDefault();
  if (dragIndex.value === null || dragIndex.value === i) return;
  const arr = editor.draft;
  const [moved] = arr.splice(dragIndex.value, 1);
  arr.splice(i, 0, moved);
  dragIndex.value = i;
}
function onDragEnd() {
  dragIndex.value = null;
}
</script>

<template>
  <div>
    <!-- VPageHeader -->
    <VPageHeader title="仪表盘">
      <template #icon>
        <Icon icon="ri:dashboard-3-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <VSpace v-if="editor.editing">
          <VButton @click="goWidgetCenter">
            <template #icon>
              <Icon icon="ri:add-circle-line" class="h-full w-full" />
            </template>
            添加部件
          </VButton>
          <VButton @click="cancelEditing">取消</VButton>
          <VButton type="secondary" :loading="saving" @click="saveLayout">
            <template #icon>
              <Icon icon="ri:save-line" class="h-full w-full" />
            </template>
            保存
          </VButton>
        </VSpace>
        <VButton v-else type="secondary" @click="startEditing">
          <template #icon>
            <Icon icon="ri:settings-4-line" class="h-full w-full" />
          </template>
          编辑
        </VButton>
      </template>
    </VPageHeader>

    <div class="m-0 space-y-4 p-4 md:m-4 md:p-0">
      <!-- 编辑态提示 -->
      <div
        v-if="editor.editing"
        class="flex items-center gap-2 rounded border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-400"
        style="border-radius: 4px"
      >
        <Icon icon="ri:drag-move-2-line" class="text-base" />
        编辑模式：拖拽部件调整顺序，点击右上角按钮移除部件，完成后记得保存
      </div>

      <!-- 待审核提醒 -->
      <router-link
        v-if="!editor.editing && stats.comment_pending > 0"
        to="/comments"
        class="flex items-center gap-2 rounded border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400"
        style="border-radius: 4px"
      >
        <Icon icon="ri:error-warning-line" class="text-base" />
        有 {{ stats.comment_pending }} 条评论待审核
        <Icon icon="ri:arrow-right-line" class="ml-auto" />
      </router-link>

      <!-- 部件网格 -->
      <div v-if="loaded || editor.editing" class="grid grid-cols-12 gap-4">
        <template v-for="(item, index) in items" :key="item.type">
          <div
            v-if="getWidget(item.type)"
            :class="[
              sizeClass[getWidget(item.type)!.size],
              editor.editing && 'relative rounded outline-dashed outline-2 outline-gray-300 dark:outline-gray-600',
              editor.editing && dragIndex === index && 'opacity-60 outline-primary',
            ]"
            :draggable="editor.editing"
            @dragstart="editor.editing && onDragStart(index)"
            @dragover="editor.editing && onDragOver($event, index)"
            @dragend="editor.editing && onDragEnd()"
          >
            <component
              :is="getWidget(item.type)!.component"
              v-bind="getWidget(item.type)!.props"
            />
            <!-- 编辑态遮罩：拖拽手柄 + 移除按钮（悬浮在卡片角外侧，避免遮挡内容） -->
            <div v-if="editor.editing" class="absolute inset-0 z-10 cursor-move rounded bg-transparent">
              <span
                class="absolute -left-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-400 shadow dark:border-gray-600 dark:bg-gray-800"
              >
                <Icon icon="ri:draggable" class="text-sm" />
              </span>
              <button
                class="absolute -right-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-400 shadow transition-colors hover:border-red-200 hover:bg-red-50 hover:text-red-500 dark:border-gray-600 dark:bg-gray-800 dark:hover:bg-red-500/20"
                title="移除部件"
                @click.stop="removeWidget(index)"
              >
                <Icon icon="ri:close-line" class="text-sm" />
              </button>
            </div>
          </div>
        </template>
      </div>

      <!-- 空布局占位 -->
      <div
        v-if="(loaded || editor.editing) && !items.length"
        class="flex flex-col items-center gap-3 rounded border border-dashed border-gray-300 py-16 text-gray-400 dark:border-gray-600"
        style="border-radius: 4px"
      >
        <Icon icon="ri:layout-grid-line" class="text-3xl" />
        <p class="text-sm">仪表盘暂无部件</p>
        <VButton v-if="editor.editing" size="sm" @click="goWidgetCenter">添加部件</VButton>
      </div>
    </div>
  </div>
</template>
