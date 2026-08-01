import { createApp, h, ref } from "vue";

// Halo Toast 复刻：命令式提示（Toast.success / Toast.error / Toast.info / Toast.warning）
type ToastType = "success" | "error" | "info" | "warning";

const colors: Record<ToastType, string> = {
  success: "#4ccba0",
  error: "#d71d1d",
  info: "#0e1731",
  warning: "#f59e0b",
};

let toastContainer: HTMLElement | null = null;

function ensureContainer(): HTMLElement {
  if (!toastContainer || !document.body.contains(toastContainer)) {
    toastContainer = document.createElement("div");
    toastContainer.className =
      "fixed top-4 left-1/2 z-[99999] flex -translate-x-1/2 flex-col items-center gap-2";
    document.body.appendChild(toastContainer);
  }
  return toastContainer;
}

function showToast(type: ToastType, message: string, duration = 2500) {
  const container = ensureContainer();
  const el = document.createElement("div");
  container.appendChild(el);

  const visible = ref(false);

  const app = createApp({
    setup() {
      setTimeout(() => (visible.value = true), 10);
      setTimeout(() => {
        visible.value = false;
        setTimeout(() => {
          app.unmount();
          el.remove();
        }, 300);
      }, duration);

      return () =>
        h(
          "div",
          {
            class:
              "flex items-center gap-2 rounded bg-white px-4 py-2.5 text-sm text-gray-800 shadow-lg dark:bg-gray-800 dark:text-gray-100",
            style: {
              borderRadius: "4px",
              border: "1px solid #eaecf0",
              opacity: visible.value ? 1 : 0,
              transform: visible.value ? "translateY(0)" : "translateY(-8px)",
              transition: "all 0.3s ease",
            },
          },
          [
            h("span", {
              class: "inline-block h-2 w-2 flex-shrink-0 rounded-full",
              style: { background: colors[type] },
            }),
            message,
          ]
        );
    },
  });

  app.mount(el);
}

export const Toast = {
  success: (message: string, duration?: number) => showToast("success", message, duration),
  error: (message: string, duration?: number) => showToast("error", message, duration),
  info: (message: string, duration?: number) => showToast("info", message, duration),
  warning: (message: string, duration?: number) => showToast("warning", message, duration),
};
