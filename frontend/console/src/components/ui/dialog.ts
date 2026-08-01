import { createApp, h, ref } from "vue";

// Halo Dialog 复刻：命令式确认弹窗（Dialog.warning / Dialog.info / Dialog.error）
export interface DialogOptions {
  title: string;
  description?: string;
  confirmText?: string;
  cancelText?: string;
  confirmType?: "primary" | "danger" | "secondary";
  onConfirm?: () => void | Promise<void>;
  onCancel?: () => void;
}

type DialogType = "info" | "warning" | "error";

const iconColors: Record<DialogType, string> = {
  info: "#4ccba0",
  warning: "#f59e0b",
  error: "#d71d1d",
};

function showDialog(type: DialogType, options: DialogOptions) {
  const container = document.createElement("div");
  document.body.appendChild(container);

  const visible = ref(true);
  const loading = ref(false);

  const app = createApp({
    setup() {
      async function handleConfirm() {
        if (options.onConfirm) {
          loading.value = true;
          try {
            await options.onConfirm();
          } finally {
            loading.value = false;
          }
        }
        close();
      }

      function handleCancel() {
        options.onCancel?.();
        close();
      }

      function close() {
        visible.value = false;
        setTimeout(() => {
          app.unmount();
          container.remove();
        }, 200);
      }

      return () =>
        h(
          "div",
          {
            class: "fixed inset-0 z-[9999] flex items-center justify-center p-4",
            style: { opacity: visible.value ? 1 : 0, transition: "opacity 0.2s ease" },
          },
          [
            h("div", {
              class: "absolute inset-0 bg-gray-500/75",
              onClick: handleCancel,
            }),
            h(
              "div",
              {
                class:
                  "relative w-full max-w-sm overflow-hidden rounded bg-white p-5 shadow-xl dark:bg-gray-800",
                style: { borderRadius: "4px" },
              },
              [
                h("div", { class: "flex items-start gap-3" }, [
                  h(
                    "div",
                    {
                      class: "flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full",
                      style: { background: `${iconColors[type]}1a` },
                    },
                    [
                      h(
                        "svg",
                        {
                          class: "h-5 w-5",
                          style: { color: iconColors[type] },
                          fill: "none",
                          stroke: "currentColor",
                          "stroke-width": "2",
                          viewBox: "0 0 24 24",
                        },
                        [
                          h("path", {
                            d: "M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                          }),
                        ]
                      ),
                    ]
                  ),
                  h("div", { class: "min-w-0 flex-1" }, [
                    h(
                      "h3",
                      { class: "text-sm font-semibold text-gray-900 dark:text-gray-100" },
                      options.title
                    ),
                    options.description
                      ? h(
                          "p",
                          { class: "mt-1.5 text-xs leading-relaxed text-gray-500 dark:text-gray-400" },
                          options.description
                        )
                      : null,
                  ]),
                ]),
                h("div", { class: "mt-5 flex justify-end gap-2" }, [
                  h(
                    "button",
                    {
                      class:
                        "h-8 rounded px-3 text-xs text-gray-600 transition-all hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700",
                      style: { border: "1px solid #d9d9d9", borderRadius: "4px" },
                      onClick: handleCancel,
                    },
                    options.cancelText || "取消"
                  ),
                  h(
                    "button",
                    {
                      class: "h-8 rounded px-3 text-xs text-white transition-all hover:opacity-90",
                      style: {
                        background:
                          options.confirmType === "danger"
                            ? "#d71d1d"
                            : options.confirmType === "secondary"
                              ? "#0e1731"
                              : "#4ccba0",
                        borderRadius: "4px",
                      },
                      disabled: loading.value,
                      onClick: handleConfirm,
                    },
                    loading.value ? "处理中..." : options.confirmText || "确定"
                  ),
                ]),
              ]
            ),
          ]
        );
    },
  });

  app.mount(container);
}

export const Dialog = {
  info: (options: DialogOptions) => showDialog("info", options),
  warning: (options: DialogOptions) => showDialog("warning", options),
  error: (options: DialogOptions) => showDialog("error", options),
};
