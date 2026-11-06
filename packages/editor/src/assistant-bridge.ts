// 写作助手与编辑器之间的桥接契约。
// Markdown（<textarea>）与富文本（TipTap）各自实现，助手面板只依赖这里的类型，
// 不关心选区在编辑器内部如何表示。文本一律是 Markdown 原文。

export type AssistantEditorMode = "markdown" | "rich";

/**
 * 取快照的意图：
 * - selection：处理选中的文字；没有选区时表示在光标处生成。
 * - continue：以选区（或光标）所在块的末尾为续写起点。
 */
export type AssistantIntent = "selection" | "continue";

/**
 * 结果写回的位置：
 * - replace：替换选中的文字；
 * - at-caret：插入到光标处；
 * - after-block：作为新的块，插到所在块之后。
 */
export type AssistantPlacement = "replace" | "at-caret" | "after-block";

/** 选区概况，用于面板实时展示；不含前后文。 */
export interface AssistantSelectionInfo {
  /** 只有光标，没有选中文字。 */
  empty: boolean;
  /** 选中的 Markdown 原文。富文本跨块选区已经扩到完整的块。 */
  text: string;
  /** 富文本跨块选区是否被扩到了完整的块。 */
  expanded: boolean;
}

/** 一次请求用到的编辑上下文。快照之后编辑器可以继续被修改，应用前由桥接重新核对。 */
export interface AssistantSnapshot {
  /** 递增编号，用来确认应用的是最近一次抓取的快照。 */
  readonly id: number;
  readonly mode: AssistantEditorMode;
  readonly intent: AssistantIntent;
  readonly placement: AssistantPlacement;
  /** 待处理的文字；at-caret 与 after-block 为空串。 */
  readonly text: string;
  /** 处理位置之前的全部 Markdown，调用方按上限截取。 */
  readonly before: string;
  /** 处理位置之后的全部 Markdown，调用方按上限截取。 */
  readonly after: string;
  readonly expanded: boolean;
}

export type AssistantApplyFailure = "stale" | "unavailable" | "failed";

export type AssistantApplyResult = { ok: true; undoable: boolean } | { ok: false; reason: AssistantApplyFailure };

export interface AssistantBridge {
  readonly mode: AssistantEditorMode;
  /** 当前选区概况；编辑面不可用（例如 Markdown 仅预览）时返回 null。 */
  peek(): AssistantSelectionInfo | null;
  /** 为一次请求抓取快照；编辑面不可用时返回 null。 */
  capture(intent: AssistantIntent): AssistantSnapshot | null;
  /** 快照对应的位置与原文是否还能对上。 */
  canApply(snapshot: AssistantSnapshot): boolean;
  /** 把结果写回编辑器，一次操作、可撤销；不保存文章。 */
  apply(snapshot: AssistantSnapshot, text: string): AssistantApplyResult;
  /** 最近一次 apply 之后文档是否没有再被改动，即撤销不会连带别的修改。 */
  canUndo(): boolean;
  /** 撤销最近一次 apply；返回是否成功还原。 */
  undo(): boolean;
  /**
   * 用户自己用编辑器的撤销（Ctrl/⌘+Z）退回了最近一次 apply：文档已回到应用前的样子。
   * 返回 true 时桥接已还原快照的位置，同一份快照可以再次应用；每次 apply 之后至多返回一次 true。
   */
  reverted?(): boolean;
  /** 订阅选区与内容变化；返回取消订阅函数。 */
  subscribe(listener: () => void): () => void;
  /** 把焦点还给编辑面。 */
  focus(): void;
}
