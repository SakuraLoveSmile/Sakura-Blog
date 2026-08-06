import { defineStore } from"pinia";
import { ref } from"vue";
import type { DashboardLayoutItem } from"@/api";

/**
 * 仪表盘编辑态状态：编辑标记 + 布局草稿 + 进入编辑时的快照。
 * 提升到 store 是为了「添加部件」跳转独立小组件中心页（/dashboard/widgets）
 * 后返回仪表盘时编辑草稿不丢失。仅存内存，刷新页面丢弃未保存草稿属预期行为。
 */
export const useDashboardEditorStore = defineStore("dashboard-editor", () => {
 const editing = ref(false);
 const draft = ref<DashboardLayoutItem[]>([]);
 let snapshot: DashboardLayoutItem[] = [];

 /** 进入编辑：以当前布局为草稿并存快照。 */
 function start(layout: DashboardLayoutItem[]) {
 snapshot = layout.map((it) => ({ ...it }));
 draft.value = layout.map((it) => ({ ...it }));
 editing.value = true;
 }

 /** 取消编辑：丢弃草稿，返回进入编辑时的快照。 */
 function cancel(): DashboardLayoutItem[] {
 draft.value = [];
 editing.value = false;
 return snapshot.map((it) => ({ ...it }));
 }

 /** 保存成功后退出编辑，返回最终布局。 */
 function finish(): DashboardLayoutItem[] {
 const result = draft.value.map((it) => ({ ...it }));
 draft.value = [];
 editing.value = false;
 return result;
 }

 /** 部件是否已在草稿中（不允许重复添加）。 */
 function isAdded(type: string) {
 return draft.value.some((it) => it.type === type);
 }

 /** 追加部件到草稿末尾。 */
 function add(type: string) {
 if (isAdded(type)) return;
 draft.value.push({ type });
 }

 /** 从草稿移除部件。 */
 function remove(index: number) {
 draft.value.splice(index, 1);
 }

 return { editing, draft, start, cancel, finish, isAdded, add, remove };
});
