import { onBeforeUnmount, onMounted, type ComputedRef } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate } from "vue-router";
import { Dialog } from "@/components/ui";

export function confirmNotebookAction(title: string, description: string): Promise<boolean> {
  return new Promise((resolve) =>
    Dialog.warning({
      title,
      description,
      confirmText: "确认",
      onConfirm: () => {
        resolve(true);
      },
      onCancel: () => resolve(false),
    }),
  );
}
export function useNotebookDraft(dirty: ComputedRef<boolean>, busy: ComputedRef<boolean>, flush?: () => Promise<void>) {
  let confirmation: Promise<boolean> | undefined;
  async function leave() {
    if (busy.value) return false;
    await flush?.();
    if (!dirty.value) return true;
    confirmation ??= confirmNotebookAction("放弃未保存的修改？", "离开后，未保存的配置与正文修改将丢失。");
    try {
      return await confirmation;
    } finally {
      confirmation = undefined;
    }
  }
  onBeforeRouteLeave(leave);
  onBeforeRouteUpdate(leave);
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (dirty.value || busy.value) {
      event.preventDefault();
      event.returnValue = "";
    }
  };
  onMounted(() => window.addEventListener("beforeunload", beforeUnload));
  onBeforeUnmount(() => window.removeEventListener("beforeunload", beforeUnload));
}
