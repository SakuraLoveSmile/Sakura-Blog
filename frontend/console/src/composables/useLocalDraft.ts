import { ref, onMounted, onBeforeUnmount } from"vue";

// 本地草稿保护 composable：防抖落 localStorage + 恢复/丢弃条幅状态。
// RichTextEditor 与 MarkdownEditor 共用（同一 storageKey 即同一份草稿，双模式切换不冲突）。
export function useLocalDraft(options: {
 storageKey?: string;
 getValue: () => string;
 setValue: (v: string) => void;
 /** 编辑器规范化基线：草稿等于基线视为无意义（如富文本 round-trip 重排产生的幻影草稿），不弹条幅 */
 baseline?: () => string;
}) {
 const draftBanner = ref(false);
 let timer: ReturnType<typeof setTimeout> | undefined;

 const draftKey = () => (options.storageKey ? `draft:${options.storageKey}` :"");

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

 onMounted(() => {
 const key = draftKey();
 if (!key) return;
 const draft = localStorage.getItem(key);
 if (draft === null || draft ==="") return;
 const base = options.baseline?.();
 // 草稿等于当前值或等于编辑器规范化基线 → 无意义，清掉并不弹条幅（修复"一直显示"）
 if (draft === options.getValue() || (base !== undefined && draft === base)) {
 localStorage.removeItem(key);
 return;
 }
 draftBanner.value = true;
 });

 onBeforeUnmount(() => clearTimeout(timer));

 return { draftBanner, scheduleDraft, clearDraft, restoreDraft, discardDraft };
}
