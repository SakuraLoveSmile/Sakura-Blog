import { ref, onBeforeUnmount, getCurrentInstance } from "vue";

// 本地草稿保护 composable：防抖落 localStorage + 恢复/丢弃条幅状态。
// RichTextEditor 与 MarkdownEditor 共用（同一 storageKey 即同一份草稿，双模式切换不冲突）。
// 草稿检测由调用方通过 checkDraft() 主动触发（而非内部 onMounted），
// 以便富文本编辑器在 onCreate 固化 round-trip 基线后再检测，避免幻影草稿误弹条幅。
export function useLocalDraft(options: {
  storageKey?: string;
  getValue: () => string;
  setValue: (v: string) => void;
  /** 编辑器规范化基线：草稿等于基线视为无意义（如富文本 round-trip 重排产生的幻影草稿），不弹条幅 */
  baseline?: () => string;
}) {
  const draftBanner = ref(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  const draftKey = () => (options.storageKey ? `draft:${options.storageKey}` : "");

  /** 防抖 2s 落 localStorage */
  function scheduleDraft() {
    const key = draftKey();
    if (!key) return;
    clearTimeout(timer);
    timer = setTimeout(() => {
      localStorage.setItem(key, options.getValue());
    }, 2000);
  }

  function clearDraft() {
    clearTimeout(timer);
    const key = draftKey();
    if (key) localStorage.removeItem(key);
  }

  function restoreDraft() {
    const key = draftKey();
    if (!key) return;
    const draft = localStorage.getItem(key);
    if (draft !== null) options.setValue(draft);
    draftBanner.value = false;
  }

  function discardDraft() {
    clearDraft();
    draftBanner.value = false;
  }

  /** 检查 localStorage 是否有未保存草稿，有则弹条幅。
   * 由调用方在 baseline 就绪后（如富文本 onCreate）调用，
   * 确保 round-trip 基线参与比较，过滤幻影草稿。
   * 可安全多次调用：每次先重置条幅再重新判定，后一次调用覆盖前一次结果。 */
  function checkDraft() {
    draftBanner.value = false;
    const key = draftKey();
    if (!key) return;
    const draft = localStorage.getItem(key);
    if (draft === null || draft === "") return;
    const base = options.baseline?.();
    // 草稿等于当前值或等于编辑器规范化基线 -> 无意义，清掉并不弹条幅
    if (draft === options.getValue() || (base !== undefined && draft === base)) {
      localStorage.removeItem(key);
      return;
    }
    draftBanner.value = true;
  }

  // Spec 62：非组件环境（node:test 纯函数测试）跳过生命周期注册——
  // composable 保持纯函数语义，可脱离 Vue 实例直接验证防抖/草稿判定逻辑
  if (getCurrentInstance()) {
    onBeforeUnmount(() => clearTimeout(timer));
  }

  return { draftBanner, scheduleDraft, clearDraft, restoreDraft, discardDraft, checkDraft };
}
