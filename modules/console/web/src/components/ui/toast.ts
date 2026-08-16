import { createApp, h, ref } from "vue";

// Halo Toast 复刻（重设计）：命令式提示，类型图标 + 语义色 + 可选副标题，暗色自适应。
type ToastType = "success" | "error" | "info" | "warning";

// 类型语义色（Apple Dark 设计令牌）
const typeColorVar: Record<ToastType, string> = {
  success: "var(--color-state-success)",
  error: "var(--color-state-error)",
  info: "var(--color-primary)",
  warning: "var(--color-state-warning)",
};

// 类型图标（内联 SVG path，24x24 viewBox，currentColor 填充）
const typeIconPath: Record<ToastType, string> = {
  success:
    "M12 22C6.477 22 2 17.523 2 12S6.477 2 12 2s10 4.477 10 10-4.477 10-10 10m-.997-6l7.07-7.071-1.414-1.414-5.656 5.657-2.829-2.829-1.414 1.414z",
  error:
    "M12 22C6.477 22 2 17.523 2 12S6.477 2 12 2s10 4.477 10 10-4.477 10-10 10m0-11.414L9.172 7.757 7.757 9.172 10.586 12l-2.829 2.828 1.415 1.415L12 13.414l2.828 2.829 1.415-1.415L13.414 12l2.829-2.828-1.415-1.415z",
  info: "M12 22C6.477 22 2 17.523 2 12S6.477 2 12 2s10 4.477 10 10-4.477 10-10 10m-1-11v6h2v-6zm0-4v2h2V7z",
  warning:
    "M12.866 3l9.526 16.5a1 1 0 0 1-.866 1.5H2.474a1 1 0 0 1-.866-1.5L11.134 3a1 1 0 0 1 1.732 0M11 16v2h2v-2zm0-7v5h2V9z",
};

// 优先级更高的 opts 参数（保留旧的 (message, duration) 数字签名兼容）
interface ToastOptions {
  description?: string;
  duration?: number;
}

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

function showToast(type: ToastType, message: string, description = "", duration = 2500) {
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
            class: "flex items-start gap-2.5 px-4 py-3 text-sm",
            style: {
              minWidth: "220px",
              maxWidth: "420px",
              borderRadius: "8px",
              background: "var(--color-card)",
              color: "var(--color-foreground)",
              border: "1px solid var(--color-border)",
              boxShadow: "var(--shadow-lg)",
              opacity: visible.value ? 1 : 0,
              transform: visible.value ? "translateY(0)" : "translateY(-8px)",
              transition: "all 0.3s ease",
            },
          },
          [
            // 类型图标（语义色）
            h(
              "svg",
              {
                width: "18",
                height: "18",
                viewBox: "0 0 24 24",
                class: "mt-px flex-shrink-0",
                style: { color: typeColorVar[type], fill: "currentColor" },
              },
              [h("path", { d: typeIconPath[type], fill: "currentColor" })]
            ),
            // 文本区：标题 + 可选副标题
            h("div", { class: "min-w-0 flex-1" }, [
              h("div", { class: "font-medium leading-snug" }, message),
              description
                ? h(
                    "div",
                    {
                      class: "mt-0.5 text-xs leading-snug",
                      style: { color: "var(--color-text-tertiary)" },
                    },
                    description
                  )
                : null,
            ]),
          ]
        );
    },
  });

  app.mount(el);
}

// 归一化第二参数：数字视为旧签名的 duration，对象为 opts。
function normalize(arg?: number | ToastOptions): { description: string; duration: number } {
  if (typeof arg === "number") return { description: "", duration: arg };
  return { description: arg?.description ?? "", duration: arg?.duration ?? 2500 };
}

export const Toast = {
  success: (message: string, opts?: number | ToastOptions) => {
    const { description, duration } = normalize(opts);
    showToast("success", message, description, duration);
  },
  error: (message: string, opts?: number | ToastOptions) => {
    const { description, duration } = normalize(opts);
    showToast("error", message, description, duration);
  },
  info: (message: string, opts?: number | ToastOptions) => {
    const { description, duration } = normalize(opts);
    showToast("info", message, description, duration);
  },
  warning: (message: string, opts?: number | ToastOptions) => {
    const { description, duration } = normalize(opts);
    showToast("warning", message, description, duration);
  },
};
