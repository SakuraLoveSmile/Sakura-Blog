import { createApp, h, ref } from "vue";

// Halo Toast 复刻（Apple Dark Mode 浮动胶囊 HUD 提示）
type ToastType = "success" | "error" | "info" | "warning";

// 类型语义色（Apple Dark 设计令牌）
const typeColorVar: Record<ToastType, string> = {
  success: "#30d158",
  error: "#ff453a",
  info: "#2e8dff",
  warning: "#ff9f0a",
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

interface ToastOptions {
  description?: string;
  duration?: number;
}

let toastContainer: HTMLElement | null = null;

function ensureContainer(): HTMLElement {
  if (!toastContainer || !document.body.contains(toastContainer)) {
    toastContainer = document.createElement("div");
    toastContainer.id = "blog-toast-container";
    toastContainer.style.cssText =
      "position:fixed;top:20px;left:50%;transform:translateX(-50%);z-index:9999999;display:flex;flex-direction:column;align-items:center;gap:8px;pointer-events:none;width:max-content;max-width:92vw;";
    document.body.appendChild(toastContainer);
  }
  return toastContainer;
}

function showToast(type: ToastType, message: string, description = "", duration = 2500) {
  const container = ensureContainer();
  const el = document.createElement("div");
  el.style.pointerEvents = "auto";
  container.appendChild(el);

  const visible = ref(false);

  const app = createApp({
    setup() {
      setTimeout(() => (visible.value = true), 10);

      const timer = setTimeout(() => {
        dismiss();
      }, duration);

      function dismiss() {
        clearTimeout(timer);
        visible.value = false;
        setTimeout(() => {
          app.unmount();
          el.remove();
        }, 250);
      }

      return () =>
        h(
          "div",
          {
            class: "apple-toast-item",
            onClick: dismiss,
            style: {
              display: "flex",
              alignItems: description ? "flex-start" : "center",
              gap: "10px",
              padding: description ? "10px 16px" : "9px 18px",
              minWidth: "180px",
              maxWidth: "460px",
              borderRadius: description ? "14px" : "99px",
              background: "rgba(28, 28, 30, 0.94)",
              color: "#f5f5f7",
              border: "1px solid rgba(255, 255, 255, 0.14)",
              boxShadow:
                "0 12px 32px rgba(0, 0, 0, 0.55), 0 2px 8px rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.12)",
              backdropFilter: "blur(20px)",
              WebkitBackdropFilter: "blur(20px)",
              opacity: visible.value ? 1 : 0,
              transform: visible.value ? "translateY(0) scale(1)" : "translateY(-10px) scale(0.96)",
              transition: "all 220ms cubic-bezier(0.32, 0.72, 0, 1)",
              cursor: "pointer",
              userSelect: "none",
              fontSize: "13.5px",
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
                style: {
                  color: typeColorVar[type],
                  fill: "currentColor",
                  flexShrink: "0",
                  marginTop: description ? "2px" : "0",
                },
              },
              [h("path", { d: typeIconPath[type], fill: "currentColor" })],
            ),
            // 文本区：标题 + 可选副标题
            h(
              "div",
              {
                style: {
                  minWidth: "0",
                  flex: "1",
                  display: "flex",
                  flexDirection: "column",
                  gap: "2px",
                },
              },
              [
                h(
                  "div",
                  {
                    style: {
                      fontWeight: "550",
                      lineHeight: "1.35",
                      color: "#ffffff",
                      fontSize: "13.5px",
                    },
                  },
                  message,
                ),
                description
                  ? h(
                      "div",
                      {
                        style: {
                          fontSize: "12px",
                          lineHeight: "1.4",
                          color: "#8e8e93",
                        },
                      },
                      description,
                    )
                  : null,
              ],
            ),
          ],
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
