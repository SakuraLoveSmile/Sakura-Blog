import Sortable from "sortablejs";
import { onBeforeUnmount, onMounted, watch, type Ref } from "vue";
import type { DashboardSectionKey, MoveRef } from "./widgets/layout-move";

interface UseSortableSectionOptions {
  /** 该容器对应的区域标识 */
  section: DashboardSectionKey;
  /** 编辑态开关；仅编辑态可拖拽 */
  editing: Ref<boolean>;
  /** 数据层回调：由父组件依据 from/to 调用 moveWidget 并持久化 */
  onMove: (from: MoveRef, to: MoveRef) => void;
}

/**
 * 将 SortableJS 挂载到给定容器，并与 Vue 数据层解耦：
 * onEnd 中先把 SortableJS 对真实 DOM 的物理移动还原到拖拽前状态，
 * 再回调数据层（moveWidget）更新响应式布局，交给 Vue 按 :key 重渲染，
 * 避免 SortableJS 与 Vue 对 DOM 的双写冲突。
 *
 * 事件字段说明：@types/sortablejs 声明 from/to/item/oldIndex/newIndex，
 * 这些正是 SortableJS 运行期可靠写入的字段（rootEl/toEl 在运行期为 undefined），
 * 因此统一以 from/to/item 读取。
 */
export function useSortableSection(
  el: Ref<HTMLElement | null>,
  options: UseSortableSectionOptions,
) {
  const { section, editing, onMove } = options;
  let instance: Sortable | null = null;

  /**
   * 把被 SortableJS 移动的 DOM 节点还原回拖拽前位置。
   * 以运行期可靠的 from(来源容器)/item(被拖节点)/oldIndex(来源原始下标) 还原，
   * 让 Vue 后续按 :key 重渲染时无需再 reconcile 本次物理移动。
   */
  function restoreDom(evt: Sortable.SortableEvent) {
    const from = evt.from as HTMLElement | null;
    const item = evt.item as HTMLElement | null;
    const oldIndex = evt.oldIndex as number | undefined;
    if (!from || !item) return;
    const ref = from.querySelectorAll(".widget-cell")[oldIndex ?? 0] ?? null;
    from.insertBefore(item, ref);
  }

  function mount() {
    if (!el.value || instance) return;
    el.value.dataset.section = section;
    instance = Sortable.create(el.value, {
      group: "dashboard-widgets",
      handle: ".apple-drag-handle",
      draggable: ".widget-cell",
      animation: 250,
      forceFallback: true,
      fallbackOnBody: true,
      fallbackTolerance: 4,
      ghostClass: "widget-ghost",
      chosenClass: "widget-chosen",
      dragClass: "widget-drag",
      disabled: !editing.value,
      onEnd(evt: Sortable.SortableEvent) {
        // 还原 DOM，交回数据层处理
        restoreDom(evt);
        const fromEl = evt.from as HTMLElement | null;
        const toEl = evt.to as HTMLElement | null;
        const fromKey = (fromEl?.dataset.section || "") as DashboardSectionKey;
        const toKey = (toEl?.dataset.section || "") as DashboardSectionKey;
        if (!fromKey || !toKey) return;
        onMove(
          { key: fromKey, index: evt.oldIndex ?? 0 },
          { key: toKey, index: evt.newIndex ?? 0 },
        );
      },
    });
  }

  function unmount() {
    instance?.destroy();
    instance = null;
  }

  onMounted(mount);
  watch(el, (val) => {
    if (val && !instance) mount();
    else if (!val && instance) unmount();
  });
  watch(editing, (val) => {
    instance?.option("disabled", !val);
  });
  onBeforeUnmount(unmount);

  return { mount, unmount };
}
