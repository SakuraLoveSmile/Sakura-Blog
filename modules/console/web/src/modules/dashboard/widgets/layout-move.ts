import type { SectionLayout } from "./layout";
import type { LayoutWidget } from "./registry";

/** 仪表盘五区域的统一区域标识（含容量为 1 的 chartWide） */
export type DashboardSectionKey =
  | "stats"
  | "chartWide"
  | "chartNarrow"
  | "recents"
  | "others";

/** 列表型区域（数组容器，可容纳多个组件） */
export type ListSectionKey = "stats" | "chartNarrow" | "recents" | "others";

export interface MoveRef {
  key: DashboardSectionKey;
  index: number;
}

/**
 * 在同/跨区域间移动组件。
 *
 * 纯函数：不触碰 DOM / SortableJS，返回新的 SectionLayout（浅克隆各区域数组），
 * 调用方负责持久化与 Vue 重渲染。
 *
 * 语义要点：
 * - 列表区域（stats/chartNarrow/recents/others）按 index 在数组内插拔。
 * - 跨同列表重排：先按 from.index 拔出，再按 to.index 插入（to.index 为最终落位的数组下标）。
 * - chartWide 容量为 1：拖入已占用的 chartWide 时，占用者被交换回来源区域原位置
 *   （from.key 的 from.index），被拖组件占据 chartWide。
 * - 从 chartWide 拖出时，chartWide 置空，被拖组件落入目标列表 to.index。
 */
export function moveWidget(
  section: SectionLayout,
  from: MoveRef,
  to: MoveRef,
): SectionLayout {
  const result: SectionLayout = {
    stats: [...section.stats],
    chartWide: section.chartWide ? { ...section.chartWide } : null,
    chartNarrow: [...section.chartNarrow],
    recents: [...section.recents],
    others: [...section.others],
  };

  // 1. 取出被拖组件
  let moved: LayoutWidget | null = null;
  if (from.key === "chartWide") {
    moved = result.chartWide;
    result.chartWide = null;
  } else {
    const list = result[from.key];
    if (from.index < 0 || from.index >= list.length) return section;
    moved = list[from.index];
    list.splice(from.index, 1);
  }
  if (!moved) return section;

  // 2. 落入 chartWide（容量为 1）
  if (to.key === "chartWide") {
    const occupant = result.chartWide;
    result.chartWide = moved;
    if (occupant) {
      // 占用者交换回来源区域原位置
      if (from.key === "chartWide") {
        // 同槽位互拖：无实际变化，保持当前占用者
        result.chartWide = occupant;
      } else {
        result[from.key].splice(from.index, 0, occupant);
      }
    }
    return result;
  }

  // 3. 落入列表区域（to.index 为最终落位数组下标）
  const toList = result[to.key];
  const insertAt = Math.max(0, Math.min(to.index, toList.length));
  toList.splice(insertAt, 0, moved);
  return result;
}
