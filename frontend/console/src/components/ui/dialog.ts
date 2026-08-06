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

// 图标语义色（Apple Dark 设计令牌）
const iconColors: Record<DialogType, string> = {
  info: "var(--color-primary)",
  warning: "var(--color-state-warning)",
  error: "var(--color-state-error)",
};

// 确认按钮底色：danger 红 / secondary 深灰 / 默认主色蓝
function confirmBackground(type?: "primary" | "danger" | "secondary"): string {
  if (type === "danger") return "var(--color-state-error)";
  if (type === "secondary") return "var(--color-secondary)";
  return "var(--color-primary)";
}

function confirmTextColor(type?: "primary" | "danger" | "secondary"): string {
  if (type === "secondary") return "var(--color-foreground)";
  return "#fff";
}

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
            // 遮罩：深色半透明 + 毛玻璃
            h("div", {
              class: "absolute inset-0",
              style: { background: "rgba(0,0,0,0.6)", backdropFilter: "blur(4px)" },
              onClick: handleCancel,
            }),
            h(
              "div",
              {
                class: "relative w-full max-w-sm overflow-hidden p-5",
                style: {
                  background: "var(--color-card)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-lg)",
                  boxShadow: "var(--shadow-lg)",
                },
              },
              [
                h("div", { class: "flex items-start gap-3" }, [
                  h(
                    "div",
                    {
                      class: "flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full",
                      style: {
                        background: `color-mix(in srgb, ${iconColors[type]} 14%, transparent)`,
                      },
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
                      {
                        class: "text-sm font-semibold",
                        style: { color: "var(--color-foreground)" },
                      },
                      options.title
                    ),
                    options.description
                      ? h(
                          "p",
                          {
                            class: "mt-1.5 text-xs leading-relaxed",
                            style: { color: "var(--color-text-tertiary)" },
                          },
                          options.description
                        )
                      : null,
                  ]),
                ]),
                h("div", { class: "mt-5 flex justify-end gap-2" }, [
                  // 取消按钮：描边胶囊
                  h(
                    "button",
                    {
                      class: "h-8 px-4 text-xs transition-all hover:bg-secondary",
                      style: {
                        color: "var(--color-foreground)",
                        background: "transparent",
                        border: "1px solid var(--color-border)",
                        borderRadius: "var(--radius-full)",
                      },
                      onClick: handleCancel,
                    },
                    options.cancelText || "取消"
                  ),
                  // 确认按钮：蓝底胶囊（danger 红 / secondary 深灰）
                  h(
                    "button",
                    {
                      class: "h-8 px-4 text-xs transition-all hover:opacity-90",
                      style: {
                        color: confirmTextColor(options.confirmType),
                        background: confirmBackground(options.confirmType),
                        border: "none",
                        borderRadius: "var(--radius-full)",
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
