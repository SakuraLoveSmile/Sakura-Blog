function B(i) {
  switch (i) {
    case "waiting_configuration":
      return "反馈已保存，等待管理员配置该软件。";
    case "waiting_source_confirmation":
      return "反馈已保存，等待管理员确认来源。";
    case "waiting_manual_archive":
      return "反馈已保存，等待管理员归档。";
    default:
      return null;
  }
}
class k extends Error {
  status;
  code;
  /** 429 daily_quota_exceeded 等错误体携带的同结构额度（若有）。 */
  quota;
  constructor(t, e, s, r) {
    super(s || e), this.name = "ApiError", this.status = t, this.code = e, this.quota = r;
  }
}
function N(i, t) {
  return `${i.replace(/\/+$/, "")}${t}`;
}
function dt(i) {
  const t = new Uint8Array(i);
  return crypto.getRandomValues(t), Array.from(t, (e) => e.toString(16).padStart(2, "0")).join("");
}
function ct() {
  if (typeof crypto.randomUUID == "function") return crypto.randomUUID();
  const i = dt(16).split("");
  i[12] = "4";
  const t = parseInt(i[16] ?? "0", 16);
  i[16] = (t & 3 | 8).toString(16);
  const e = i.join("");
  return `${e.slice(0, 8)}-${e.slice(8, 12)}-${e.slice(12, 16)}-${e.slice(16, 20)}-${e.slice(20)}`;
}
function ut(i) {
  if (typeof i != "object" || i === null) return;
  const t = i;
  if (!(typeof t.dailyLimit != "number" || typeof t.used != "number" || typeof t.remaining != "number" || typeof t.resetAt != "string"))
    return { dailyLimit: t.dailyLimit, used: t.used, remaining: t.remaining, resetAt: t.resetAt };
}
function st(i, t) {
  const e = t?.error;
  throw new k(
    i,
    e?.code ?? `http_${i}`,
    e?.message ?? `HTTP ${i}`,
    ut(t?.quota)
  );
}
const ft = 2e4;
async function T(i, t, e, s = {}) {
  const r = {};
  s.token && (r.Authorization = `Bearer ${s.token}`);
  let o;
  s.body !== void 0 && (r["Content-Type"] = "application/json", o = JSON.stringify(s.body));
  const l = typeof s.timeoutMs == "number" ? new AbortController() : null, h = l ? setTimeout(() => l.abort(), s.timeoutMs) : null;
  try {
    const a = await fetch(N(i, e), {
      method: t,
      headers: r,
      body: o,
      credentials: "omit",
      ...l ? { signal: l.signal } : {}
    });
    if (a.status === 204) return;
    let d = null;
    try {
      d = await a.json();
    } catch {
      d = null;
    }
    return a.ok || st(a.status, d), d;
  } finally {
    h !== null && clearTimeout(h);
  }
}
function bt(i, t) {
  return T(i, "POST", "/api/auth/login", { body: t });
}
function pt(i, t) {
  return T(i, "GET", "/api/auth/session", {
    token: t,
    timeoutMs: ft
  });
}
async function gt(i, t, e) {
  const s = Array.isArray(e.logs) && e.logs.length > 0;
  if (e.screenshot || s) {
    const r = new FormData(), o = {
      idempotencyKey: e.idempotencyKey,
      appId: e.appId,
      text: e.text
    };
    if (e.appName && (o.appName = e.appName), e.context && (o.context = e.context), e.capture && (o.capture = e.capture), s && e.logs && (o.logs = e.logs.map((d) => ({
      filename: d.filename,
      source: d.source,
      ...d.sha256 ? { sha256: d.sha256 } : {}
    }))), r.append("metadata", JSON.stringify(o)), e.screenshot && r.append("screenshot", e.screenshot, "screenshot.png"), s && e.logs)
      for (const d of e.logs)
        r.append("logs", d.blob, d.filename);
    const l = {};
    t && (l.Authorization = `Bearer ${t}`);
    const h = await fetch(N(i, "/api/feedback"), {
      method: "POST",
      headers: l,
      body: r,
      credentials: "omit"
    });
    if (h.status === 204) return;
    let a = null;
    try {
      a = await h.json();
    } catch {
      a = null;
    }
    return h.ok || st(h.status, a), a;
  }
  return T(i, "POST", "/api/feedback", { body: e, token: t });
}
function X(i, t, e) {
  return T(i, "GET", `/api/feedback/${encodeURIComponent(e)}`, { token: t });
}
function C(i) {
  return Array.from(i).length;
}
const S = 1e4;
function A(i, t = {}) {
  const e = i.trim();
  if (!e) return { ok: !1, reason: "请输入服务器地址" };
  let s;
  try {
    s = new URL(e);
  } catch {
    return {
      ok: !1,
      reason: "地址格式无效（示例：https://fb.example.com 或 http://127.0.0.1:8787）"
    };
  }
  const r = s.protocol.toLowerCase();
  if (r !== "http:" && r !== "https:")
    return { ok: !1, reason: "仅支持 http(s) 地址（示例：https://fb.example.com）" };
  if (!s.hostname) return { ok: !1, reason: "地址缺少主机名" };
  if (s.username || s.password)
    return { ok: !1, reason: "地址不能包含用户名或密码" };
  if (s.search) return { ok: !1, reason: "地址不能包含查询参数" };
  if (s.hash) return { ok: !1, reason: "地址不能包含 # 片段" };
  if (t.pageIsHttps && r === "http:")
    return { ok: !1, reason: "HTTPS 页面无法使用 HTTP 服务地址（浏览器会拦截混合内容）" };
  const o = s.pathname.replace(/\/+$/, "");
  return { ok: !0, base: `${r}//${s.host}${o}` };
}
function mt(i) {
  const t = new TextEncoder().encode(i);
  let e = "";
  for (const s of t) e += String.fromCharCode(s);
  return btoa(e).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}
function rt(i, t) {
  const e = A(t), r = `${e.ok ? e.base : t.trim()}\0${i}`;
  return `feedback-widget.server-override.${mt(r)}`;
}
function vt(i, t, e) {
  try {
    const s = rt(i, t);
    return e === null ? window.localStorage.removeItem(s) : window.localStorage.setItem(s, e), !0;
  } catch {
    return !1;
  }
}
function xt(i) {
  return `web:${i}`.slice(0, 100);
}
function wt(i) {
  if (i instanceof k)
    switch (i.code) {
      case "invalid_credentials":
        return "用户名或密码错误";
      case "rate_limited":
        return "尝试过于频繁，请稍后再试";
      case "origin_not_allowed":
        return "当前页面来源未被该应用允许登录，请联系管理员";
      case "unknown_app":
        return "应用未在服务端登记（请检查 app-id 配置）";
      default:
        return i.message;
    }
  return "网络异常，登录失败，请稍后重试";
}
const yt = (
  /* css */
  `
:host {
  --fb-z-index-default: 2147483000;
  --fb-accent: #0071e3;
  --fb-accent-hover: #0077ed;
  --fb-accent-active: #006edb;
  --fb-accent-disabled: #a0c7f5;
  --fb-focus-ring: rgba(0, 113, 227, 0.28);

  /* 默认浅色变量 */
  --fb-bg: #ffffff;
  --fb-bg-subtle: #f5f5f7;
  --fb-bg-subtle-hover: #e8e8ed;
  --fb-text: #1d1d1f;
  --fb-text-secondary: #86868b;
  --fb-text-tertiary: #a1a1a6;
  --fb-border: rgba(0, 0, 0, 0.08);
  --fb-border-strong: rgba(0, 0, 0, 0.16);
  --fb-shadow: 0 12px 32px rgba(0, 0, 0, 0.12), 0 2px 6px rgba(0, 0, 0, 0.04);
  --fb-tab-shadow: 0 4px 14px rgba(0, 0, 0, 0.08), 0 1px 3px rgba(0, 0, 0, 0.05);

  --fb-status-error-bg: #fff2f4;
  --fb-status-error-border: #ffccd5;
  --fb-status-error-text: #d70015;

  --fb-status-success-bg: #f0fdf4;
  --fb-status-success-border: #bbf7d0;
  --fb-status-success-text: #15803d;

  --fb-status-warn-bg: #fff9eb;
  --fb-status-warn-border: #fde68a;
  --fb-status-warn-text: #b45309;

  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display",
    "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
  font-size: 14px;
  line-height: 1.5;
  color: var(--fb-text);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/*
 * [hidden] 全局兜底（必须保留）。
 *
 * 浏览器默认样式表里的 [hidden] { display: none } 是 **UA 规则**，会被本文件的
 * 任何作者规则压掉 —— 例如 .fb-screenshot-wrap { display: flex } 会让
 * element.hidden = true 完全失效：元素照旧占据版面。
 *
 * 已发生过的真实故障（宿主未声明 capture-mode，即默认 off 的绝大多数宿主）：
 * 面板显示一个空的「当前截图 / 点击放大」区，缩略图 src 为空 → 破图占位，
 * 点击也无法放大（无截图时 openZoomModal 直接返回）。「移除截图」看起来也毫无反应。
 *
 * 因此这里统一恢复 hidden 语义，避免每加一个设置 display 的组件规则就重新踩一次。
 * 依赖它的元素不止截图层：面板、截图预览区与
 * 「截取当前页面 / 重新截图 / 移除截图」三件套（按状态互斥显示）同样靠它兜底。
 * 回归覆盖：e2e/run_browser.py 的 P5a / P5b / P6（只有真实浏览器有样式级联）。
 */
[hidden] {
  display: none !important;
}

/* 显式深色或系统深色模式 */
:host([theme="dark"]) {
  --fb-accent-disabled: #1d4677;
  --fb-bg: #1c1c1e;
  --fb-bg-subtle: #2c2c2e;
  --fb-bg-subtle-hover: #3a3a3c;
  --fb-text: #f5f5f7;
  --fb-text-secondary: #98989d;
  --fb-text-tertiary: #636366;
  --fb-border: rgba(255, 255, 255, 0.12);
  --fb-border-strong: rgba(255, 255, 255, 0.24);
  --fb-shadow: 0 16px 40px rgba(0, 0, 0, 0.45), 0 2px 6px rgba(0, 0, 0, 0.3);
  --fb-tab-shadow: 0 4px 14px rgba(0, 0, 0, 0.4);

  --fb-status-error-bg: rgba(255, 69, 58, 0.15);
  --fb-status-error-border: rgba(255, 69, 58, 0.3);
  --fb-status-error-text: #ff453a;

  --fb-status-success-bg: rgba(48, 209, 88, 0.15);
  --fb-status-success-border: rgba(48, 209, 88, 0.3);
  --fb-status-success-text: #30d158;

  --fb-status-warn-bg: rgba(255, 159, 10, 0.15);
  --fb-status-warn-border: rgba(255, 159, 10, 0.3);
  --fb-status-warn-text: #ff9f0a;
}

@media (prefers-color-scheme: dark) {
  :host(:not([theme="light"]):not([theme="dark"])) {
    --fb-accent-disabled: #1d4677;
    --fb-bg: #1c1c1e;
    --fb-bg-subtle: #2c2c2e;
    --fb-bg-subtle-hover: #3a3a3c;
    --fb-text: #f5f5f7;
    --fb-text-secondary: #98989d;
    --fb-text-tertiary: #636366;
    --fb-border: rgba(255, 255, 255, 0.12);
    --fb-border-strong: rgba(255, 255, 255, 0.24);
    --fb-shadow: 0 16px 40px rgba(0, 0, 0, 0.45), 0 2px 6px rgba(0, 0, 0, 0.3);
    --fb-tab-shadow: 0 4px 14px rgba(0, 0, 0, 0.4);

    --fb-status-error-bg: rgba(255, 69, 58, 0.15);
    --fb-status-error-border: rgba(255, 69, 58, 0.3);
    --fb-status-error-text: #ff453a;

    --fb-status-success-bg: rgba(48, 209, 88, 0.15);
    --fb-status-success-border: rgba(48, 209, 88, 0.3);
    --fb-status-success-text: #30d158;

    --fb-status-warn-bg: rgba(255, 159, 10, 0.15);
    --fb-status-warn-border: rgba(255, 159, 10, 0.3);
    --fb-status-warn-text: #ff9f0a;
  }
}

/* 隐藏宿主控制 */
:host([show-launcher="false"]) .fb-launcher {
  display: none !important;
}

/* ---- 侧边贴边入口标签 ---- */
.fb-launcher {
  position: fixed;
  z-index: var(--fb-z-index, var(--fb-z-index-default));
  bottom: clamp(16px, var(--fb-launcher-bottom, 25%), calc(100vh - 64px));
  right: 0;
  left: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
  min-width: 44px;
  padding: 8px 12px 8px 10px;
  border: 1px solid var(--fb-border);
  border-right: none;
  border-radius: 12px 0 0 12px;
  background: var(--fb-bg);
  color: var(--fb-text);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  box-shadow: var(--fb-tab-shadow);
  transition: transform 0.18s cubic-bezier(0.16, 1, 0.3, 1),
              background 0.18s ease,
              opacity 0.18s ease;
  user-select: none;
  -webkit-user-select: none;
}

.fb-launcher:hover {
  transform: translateX(-3px);
  background: var(--fb-bg-subtle);
}

.fb-launcher:active {
  transform: translateX(-1px);
}

.fb-launcher:focus-visible {
  outline: 2px solid var(--fb-accent);
  outline-offset: 1px;
}

.fb-launcher-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  fill: currentColor;
  color: var(--fb-accent);
}

:host([side="left"]) .fb-launcher {
  right: auto;
  left: 0;
  padding: 8px 10px 8px 12px;
  border-left: none;
  border-right: 1px solid var(--fb-border);
  border-radius: 0 12px 12px 0;
}

:host([side="left"]) .fb-launcher:hover {
  transform: translateX(3px);
}

:host([side="left"]) .fb-launcher:active {
  transform: translateX(1px);
}

/* ---- 灵感球 (Inspiration Orb) ---- */
.fb-orb {
  position: fixed;
  z-index: var(--fb-z-index, var(--fb-z-index-default));
  bottom: clamp(16px, var(--fb-launcher-bottom, 25%), calc(100vh - 64px));
  right: 20px;
  left: auto;
  width: 48px;
  height: 48px;
  min-width: 48px;
  min-height: 48px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(255, 255, 255, 0.82);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  color: var(--fb-text);
  cursor: grab;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12), 0 1px 3px rgba(0, 0, 0, 0.06), inset 0 0 0 0.5px rgba(255, 255, 255, 0.8);
  touch-action: none;
  user-select: none;
  -webkit-user-select: none;
  transition: transform 0.28s cubic-bezier(0.16, 1, 0.3, 1),
              box-shadow 0.2s ease,
              background 0.2s ease;
}

:host([side="left"]) .fb-orb {
  right: auto;
  left: 20px;
}

.fb-orb:hover {
  transform: scale(1.06);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.15), 0 2px 6px rgba(0, 0, 0, 0.08);
}

.fb-orb:active,
.fb-orb.is-dragging {
  cursor: grabbing;
  transform: scale(1.1);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18), 0 3px 8px rgba(0, 0, 0, 0.1);
}

.fb-orb.is-dragging {
  transition: none !important;
}

.fb-orb:focus-visible {
  outline: 2px solid var(--fb-accent);
  outline-offset: 2px;
}

.fb-orb-icon {
  width: 22px;
  height: 22px;
  color: var(--fb-accent);
  pointer-events: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

:host([theme="dark"]) .fb-orb {
  background: rgba(44, 44, 46, 0.82);
  border: 1px solid rgba(255, 255, 255, 0.16);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.45), inset 0 0 0 0.5px rgba(255, 255, 255, 0.15);
}

@media (prefers-color-scheme: dark) {
  :host(:not([theme="light"]):not([theme="dark"])) .fb-orb {
    background: rgba(44, 44, 46, 0.82);
    border: 1px solid rgba(255, 255, 255, 0.16);
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.45), inset 0 0 0 0.5px rgba(255, 255, 255, 0.15);
  }
}

/* 控制 launcher 模式显示/隐藏 */
:host(:not([launcher-mode="orb"])) .fb-orb {
  display: none !important;
}

:host([launcher-mode="orb"]) .fb-tab-launcher {
  display: none !important;
}

/* 面板打开时隐藏标签 */
.fb-launcher.is-hidden {
  display: none !important;
}

/* ---- 桌面圆角浮层面板（无遮罩、不挤压布局、不锁滚动） ---- */
.fb-panel {
  position: fixed;
  bottom: 16px;
  right: 16px;
  left: auto;
  top: auto;
  z-index: var(--fb-z-index, var(--fb-z-index-default));
  width: min(400px, calc(100vw - 32px));
  max-height: min(620px, calc(100dvh - 32px));
  height: auto;
  box-sizing: border-box;
  background: var(--fb-bg);
  border: 1px solid var(--fb-border);
  border-radius: 16px;
  box-shadow: var(--fb-shadow);
  display: flex;
  flex-direction: column;
  opacity: 0;
  transform: scale(0.96) translateY(8px);
  pointer-events: none;
  visibility: hidden;
  transition: opacity 0.2s cubic-bezier(0.16, 1, 0.3, 1),
              transform 0.2s cubic-bezier(0.16, 1, 0.3, 1),
              visibility 0.2s;
  overflow: hidden;
}

.fb-panel.is-open {
  opacity: 1;
  transform: scale(1) translateY(0);
  pointer-events: auto;
  visibility: visible;
}

:host([side="left"]) .fb-panel {
  right: auto;
  left: 16px;
}

/* ---- 移动端全屏布局（断点 768px） ---- */
@media (max-width: 767.98px) {
  .fb-panel {
    bottom: 0;
    top: 0;
    left: 0;
    right: 0;
    width: 100vw;
    max-height: 100dvh;
    height: 100dvh;
    border-radius: 0;
    border: none;
    box-shadow: none;
    transform: translateY(100%);
    padding-top: env(safe-area-inset-top, 0);
    padding-bottom: env(safe-area-inset-bottom, 0);
  }

  .fb-panel.is-open {
    transform: translateY(0);
  }
}

/* ---- 面板头部 ---- */
.fb-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--fb-border);
  flex-shrink: 0;
}

.fb-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.fb-header-icon {
  width: 18px;
  height: 18px;
  fill: currentColor;
  color: var(--fb-accent);
  flex-shrink: 0;
}

.fb-header h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--fb-text);
  letter-spacing: -0.01em;
}

.fb-header-meta {
  font-size: 12px;
  color: var(--fb-text-tertiary);
  margin-left: 4px;
}

.fb-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: var(--fb-bg-subtle);
  color: var(--fb-text-secondary);
  border-radius: 50%;
  cursor: pointer;
  padding: 0;
  transition: background 0.15s, color 0.15s;
}

.fb-close:hover {
  background: var(--fb-bg-subtle-hover);
  color: var(--fb-text);
}

.fb-close:focus-visible,
.fb-submit:focus-visible,
.fb-btn-secondary:focus-visible,
.fb-task-link:focus-visible,
.fb-retry-btn:focus-visible {
  outline: 2px solid var(--fb-accent);
  outline-offset: 2px;
}

/* ---- 面板主体 ---- */
.fb-body {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
  gap: 12px;
}

.fb-prompt {
  font-size: 13px;
  font-weight: 500;
  color: var(--fb-text-secondary);
  margin: 0;
}

.fb-textarea-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  min-height: 150px;
}

.fb-textarea {
  width: 100%;
  box-sizing: border-box;
  flex: 1;
  min-height: 150px;
  resize: none;
  padding: 12px 14px;
  border: 1px solid var(--fb-border);
  border-radius: 10px;
  background: var(--fb-bg-subtle);
  font: inherit;
  font-size: 14px;
  color: var(--fb-text);
  line-height: 1.5;
  transition: border-color 0.15s, box-shadow 0.15s, background 0.15s;
}

.fb-textarea:focus {
  outline: none;
  background: var(--fb-bg);
  border-color: var(--fb-accent);
  box-shadow: 0 0 0 3px var(--fb-focus-ring);
}

.fb-textarea:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.fb-counter-row {
  display: flex;
  justify-content: flex-end;
}

.fb-counter {
  color: var(--fb-text-tertiary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.fb-counter.is-over {
  color: var(--fb-status-error-text);
  font-weight: 600;
}

/* ---- 状态区与卡片 ---- */
.fb-status-region {
  min-height: 1.4em;
  font-size: 13px;
  color: var(--fb-text-secondary);
}

.fb-status-card {
  border-radius: 10px;
  padding: 12px 14px;
  border: 1px solid;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
}

.fb-status-card p {
  margin: 0;
  line-height: 1.45;
}

.fb-status-card.is-error {
  background: var(--fb-status-error-bg);
  border-color: var(--fb-status-error-border);
  color: var(--fb-status-error-text);
}

.fb-status-card.is-success {
  background: var(--fb-status-success-bg);
  border-color: var(--fb-status-success-border);
  color: var(--fb-status-success-text);
}

.fb-status-card.is-warn {
  background: var(--fb-status-warn-bg);
  border-color: var(--fb-status-warn-border);
  color: var(--fb-status-warn-text);
}

.fb-card-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
}

.fb-task-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--fb-accent);
  font-weight: 500;
  text-decoration: none;
}

.fb-task-link:hover {
  text-decoration: underline;
}

.fb-btn-secondary {
  border: 1px solid var(--fb-border-strong);
  background: var(--fb-bg);
  color: var(--fb-text);
  border-radius: 8px;
  padding: 6px 14px;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}

.fb-btn-secondary:hover {
  background: var(--fb-bg-subtle-hover);
}

/* ---- 底部操作区 ---- */
.fb-footer {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: auto;
}

.fb-submit {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 38px;
  padding: 0 18px;
  border: none;
  border-radius: 980px;
  background: var(--fb-accent);
  color: #fff;
  font: inherit;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s ease, transform 0.1s ease;
}

.fb-submit:hover:not(:disabled) {
  background: var(--fb-accent-hover);
}

.fb-submit:active:not(:disabled) {
  background: var(--fb-accent-active);
  transform: scale(0.99);
}

.fb-submit:disabled {
  background: var(--fb-accent-disabled);
  cursor: not-allowed;
}

.fb-footnote {
  margin: 0;
  font-size: 12px;
  color: var(--fb-text-tertiary);
  text-align: center;
  line-height: 1.4;
}

/* ---- 额度与面板内登录 ---- */
.fb-quota {
  font-size: 12px;
  color: var(--fb-text-secondary);
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.fb-quota-blocked {
  margin: 0;
  font-size: 12px;
  color: var(--fb-status-warn-text);
  text-align: center;
  line-height: 1.4;
}

.fb-login-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--fb-border);
  border-radius: 10px;
  background: var(--fb-bg-subtle);
}

.fb-login-row {
  display: flex;
  gap: 8px;
}

.fb-login-input {
  flex: 1;
  min-width: 0;
  box-sizing: border-box;
  padding: 8px 10px;
  border: 1px solid var(--fb-border-strong);
  border-radius: 8px;
  background: var(--fb-bg);
  color: var(--fb-text);
  font: inherit;
  font-size: 13px;
}

.fb-login-input:focus {
  outline: none;
  border-color: var(--fb-accent);
  box-shadow: 0 0 0 3px var(--fb-focus-ring);
}

.fb-login-input:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.fb-login-error {
  margin: 0;
  font-size: 12px;
  color: var(--fb-status-error-text);
  line-height: 1.4;
}

.fb-login-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

/* ---- 截图区：预览与操作分离 ----
   预览（.fb-screenshot-wrap）只在真的有可渲染的截图 URL 时可见；
   操作区（.fb-screenshot-actions）始终在面板里：没有截图时它是「截取当前页面」。
   这条分工是刻意的——**无截图时整个截图区一起隐藏**会让默认 capture-mode=off
   （不自动截图）的宿主没有任何首次截图 / 补拍入口，而 hidden 的按钮三件套
   （截取当前页面 / 重新截图 / 移除截图）又必须真的不占版面。 */
.fb-shot-area {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* ---- 截图缩略图与操作 ---- */
.fb-screenshot-wrap {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.fb-screenshot-thumb-box {
  position: relative;
  width: 100%;
  height: 130px;
  max-height: 140px;
  border-radius: 10px;
  border: 1px solid var(--fb-border);
  background: var(--fb-bg-subtle);
  overflow: hidden;
  cursor: zoom-in;
  display: flex;
  align-items: center;
  justify-content: center;
}

.fb-screenshot-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
  transition: transform 0.2s ease;
}

.fb-screenshot-thumb-box:hover .fb-screenshot-thumb {
  transform: scale(1.02);
}

.fb-screenshot-badge {
  position: absolute;
  top: 8px;
  left: 8px;
  padding: 2px 8px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  font-size: 11px;
  font-weight: 500;
  letter-spacing: 0.02em;
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  pointer-events: none;
}

.fb-screenshot-zoom-hint {
  position: absolute;
  bottom: 8px;
  right: 8px;
  padding: 3px 8px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  font-size: 11px;
  font-weight: 500;
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.fb-screenshot-thumb-box:hover .fb-screenshot-zoom-hint {
  opacity: 1;
}

.fb-screenshot-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

/* 首个截图入口：默认 capture-mode=off 的宿主里它是**唯一**的截图方式，
   因此比「重新截图 / 移除截图」更显眼（主色填充），但不与「提交」争主次。 */
.fb-btn-capture {
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  border: 1px solid var(--fb-accent);
  background: var(--fb-accent);
  color: #fff;
  cursor: pointer;
  transition: background 0.15s;
}

.fb-btn-capture:hover:not(:disabled) {
  background: var(--fb-accent-hover);
}

.fb-btn-capture:disabled {
  background: var(--fb-accent-disabled);
  border-color: var(--fb-accent-disabled);
  cursor: not-allowed;
}

/* 捕获中：面板在此期间整体 visibility:hidden（截图里不含组件自身 UI），
   所以这个加载态只可能被会话结束后的用户与辅助技术看到。 */
.fb-shot-area.is-capturing .fb-btn-capture {
  cursor: progress;
}

.fb-btn-retake,
.fb-btn-remove {
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  border: 1px solid var(--fb-border-strong);
  background: var(--fb-bg);
  color: var(--fb-text);
  cursor: pointer;
  transition: background 0.15s;
}

.fb-btn-retake:hover,
.fb-btn-remove:hover {
  background: var(--fb-bg-subtle-hover);
}

.fb-btn-remove {
  color: var(--fb-status-error-text);
  border-color: var(--fb-status-error-border);
}

.fb-btn-capture:focus-visible,
.fb-btn-retake:focus-visible,
.fb-btn-remove:focus-visible,
.fb-zoom-close:focus-visible {
  outline: 2px solid var(--fb-accent);
  outline-offset: 2px;
}

/* 窄屏（移动端全屏面板）：截图操作按钮给到触屏可点的高度，并允许换行 */
@media (max-width: 767.98px) {
  .fb-screenshot-actions {
    gap: 10px;
  }

  .fb-btn-capture,
  .fb-btn-retake,
  .fb-btn-remove {
    padding: 7px 12px;
    font-size: 13px;
  }
}

/* ---- 日志附件区 ---- */
.fb-logs-area {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.fb-logs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--fb-text-secondary);
}

.fb-logs-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.fb-log-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  border-radius: 6px;
  background: var(--fb-bg-subtle);
  border: 1px solid var(--fb-border);
  font-size: 12px;
  gap: 8px;
}

.fb-log-info {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.fb-log-name {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fb-log-badge {
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 500;
  background: var(--fb-bg-subtle-hover);
  color: var(--fb-text-secondary);
}

.fb-log-size {
  color: var(--fb-text-tertiary);
  font-size: 11px;
}

.fb-log-btns {
  display: flex;
  align-items: center;
  gap: 6px;
}

.fb-log-btn-preview,
.fb-log-btn-remove,
.fb-btn-add-log {
  padding: 3px 8px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 4px;
  border: 1px solid var(--fb-border-strong);
  background: var(--fb-bg);
  color: var(--fb-text);
  cursor: pointer;
  transition: background 0.15s;
}

.fb-log-btn-preview:hover,
.fb-log-btn-remove:hover,
.fb-btn-add-log:hover {
  background: var(--fb-bg-subtle-hover);
}

.fb-log-btn-remove {
  color: var(--fb-status-error-text);
  border-color: var(--fb-status-error-border);
}

.fb-log-status {
  font-size: 12px;
  color: var(--fb-text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.fb-log-error {
  font-size: 12px;
  color: var(--fb-status-error-text);
  background: var(--fb-status-error-bg);
  border: 1px solid var(--fb-status-error-border);
  padding: 4px 8px;
  border-radius: 4px;
}

.fb-log-preview-modal {
  position: fixed;
  inset: 0;
  z-index: calc(var(--fb-z-index, var(--fb-z-index-default)) + 25);
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  box-sizing: border-box;
  opacity: 0;
  pointer-events: none;
  visibility: hidden;
  transition: opacity 0.2s cubic-bezier(0.16, 1, 0.3, 1), visibility 0.2s;
}

.fb-log-preview-modal.is-open {
  opacity: 1;
  pointer-events: auto;
  visibility: visible;
}

.fb-log-preview-card {
  background: var(--fb-bg);
  color: var(--fb-text);
  border-radius: 12px;
  padding: 16px;
  max-width: 680px;
  width: 100%;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: var(--fb-shadow);
  border: 1px solid var(--fb-border);
}

.fb-log-preview-body {
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.4;
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--fb-bg-subtle);
  padding: 12px;
  border-radius: 6px;
  margin: 12px 0;
  flex: 1;
}

/* ---- 截图大图弹窗 (Zoom Modal) ---- */
.fb-zoom-modal {
  position: fixed;
  inset: 0;
  z-index: calc(var(--fb-z-index, var(--fb-z-index-default)) + 20);
  background: rgba(0, 0, 0, 0.82);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
  box-sizing: border-box;
  cursor: zoom-out;
  opacity: 0;
  pointer-events: none;
  visibility: hidden;
  transition: opacity 0.2s cubic-bezier(0.16, 1, 0.3, 1), visibility 0.2s;
}

.fb-zoom-modal.is-open {
  opacity: 1;
  pointer-events: auto;
  visibility: visible;
}

.fb-zoom-modal img {
  max-width: 92vw;
  max-height: 90vh;
  object-fit: contain;
  border-radius: 8px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.6);
}

.fb-zoom-close {
  position: absolute;
  top: 20px;
  right: 20px;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: none;
  background: rgba(255, 255, 255, 0.18);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}

.fb-zoom-close:hover {
  background: rgba(255, 255, 255, 0.3);
}

/* ---- 头部右侧动作组与服务器设置视图（T6） ---- */
.fb-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.fb-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--fb-text-secondary);
  border-radius: 50%;
  cursor: pointer;
  padding: 0;
  transition: background 0.15s, color 0.15s;
}

.fb-icon-btn:hover {
  background: var(--fb-bg-subtle-hover);
  color: var(--fb-text);
}

.fb-icon-btn:focus-visible {
  outline: 2px solid var(--fb-accent);
  outline-offset: 2px;
}

.fb-settings {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
  gap: 10px;
}

.fb-settings-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--fb-text);
}

.fb-settings-line {
  margin: 0;
  font-size: 12px;
  color: var(--fb-text-secondary);
  display: flex;
  gap: 4px;
  min-width: 0;
}

.fb-settings-label {
  flex-shrink: 0;
}

.fb-settings-value {
  color: var(--fb-text);
  overflow-wrap: anywhere;
  word-break: break-all;
}

.fb-server-input {
  width: 100%;
}

.fb-settings-error {
  margin: 0;
  font-size: 12px;
  color: var(--fb-status-error-text);
  line-height: 1.4;
}

.fb-settings-hint {
  margin: 0;
  font-size: 12px;
  color: var(--fb-status-warn-text);
  line-height: 1.4;
}

.fb-settings-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.fb-settings-confirm {
  border: 1px solid var(--fb-status-warn-border);
  background: var(--fb-status-warn-bg);
  color: var(--fb-status-warn-text);
  border-radius: 10px;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
}

.fb-settings-confirm-text {
  margin: 0;
  line-height: 1.5;
}

/* 窄屏（移动端全屏面板）：设置操作按钮给到触屏可点的高度 */
@media (max-width: 767.98px) {
  .fb-settings-actions .fb-btn-secondary {
    padding: 8px 14px;
    font-size: 13px;
  }
}

/* ---- 尊重 prefers-reduced-motion ---- */
@media (prefers-reduced-motion: reduce) {
  .fb-launcher,
  .fb-panel,
  .fb-submit,
  .fb-close {
    transition: none !important;
  }
  .fb-launcher:hover,
  .fb-launcher:active,
  .fb-submit:active {
    transform: none !important;
  }
}
`
), P = 2048, R = 4e6, I = 5 * 1024 * 1024, kt = 3, St = 0.8, Et = { r: 110, g: 110, b: 115 }, At = "rgb(110, 110, 115)", nt = "截图未完成，可重试或继续文字反馈", ot = 'input[type="password"], [data-feedback-capture-mask]';
class f extends Error {
  reason;
  /** 面向用户的提示（不含内部细节）。 */
  userMessage;
  constructor(t, e, s = nt) {
    super(`[${t}] ${e}`), this.name = "CaptureError", this.reason = t, this.userMessage = s;
  }
}
function v(i) {
  return typeof i == "number" && Number.isFinite(i);
}
function Bt(i, t) {
  if (!i || typeof i != "object")
    throw new f("provider-invalid", "captureProvider 未返回结果对象");
  const e = i.blob;
  if (!(e instanceof Blob))
    throw new f("provider-invalid", "captureProvider 返回值缺少 Blob");
  if (!e.type.toLowerCase().includes("image/png"))
    throw new f("provider-invalid", "截图仅支持 PNG 格式");
  if (e.size === 0 || e.size > I)
    throw new f("provider-invalid", `截图文件大小需在 0–${I} 字节内`);
  if (!v(i.width) || !v(i.height))
    throw new f("provider-invalid", "截图尺寸坐标必须为有限数字");
  const s = Math.round(i.width), r = Math.round(i.height);
  if (s <= 0 || r <= 0)
    throw new f("provider-invalid", "截图尺寸必须为正数");
  if (Math.max(s, r) > P)
    throw new f("provider-invalid", `截图输出边长不能超过 ${P}px`);
  if (s * r > R)
    throw new f("provider-invalid", `截图输出像素不能超过 ${R}`);
  let o = [];
  if (t > 0) {
    if (i.sameFrameMasking !== !0 || !Array.isArray(i.maskedRegions))
      throw new f(
        "provider-unsafe",
        "页面存在可见敏感区域，但 captureProvider 未保证同帧遮挡，拒绝使用该截图"
      );
    for (const a of i.maskedRegions) {
      if (!a || !v(a.x) || !v(a.y) || !v(a.width) || !v(a.height))
        throw new f("provider-unsafe", "遮挡区域坐标非有限，拒绝使用该截图");
      if (a.width <= 0 || a.height <= 0)
        throw new f("provider-unsafe", "遮挡区域尺寸必须为正数");
      if (a.x < -1 || a.y < -1 || a.x + a.width > s + 1 || a.y + a.height > r + 1)
        throw new f("provider-unsafe", "遮挡区域超出截图输出范围");
    }
    if (i.maskedRegions.length === 0)
      throw new f(
        "provider-unsafe",
        `检测到 ${t} 个可见敏感区域，但提供者未遮挡任何区域`
      );
    o = i.maskedRegions.map((a) => ({
      x: a.x,
      y: a.y,
      width: a.width,
      height: a.height
    }));
  } else i.sameFrameMasking === !0 && Array.isArray(i.maskedRegions) && (o = i.maskedRegions.filter(
    (a) => a && v(a.x) && v(a.y) && v(a.width) && v(a.height) && a.width > 0 && a.height > 0
  ));
  let l = s, h = r;
  return i.viewport && v(i.viewport.width) && v(i.viewport.height) && i.viewport.width > 0 && i.viewport.height > 0 && (l = Math.round(i.viewport.width), h = Math.round(i.viewport.height)), { blob: e, viewportWidth: l, viewportHeight: h, outputWidth: s, outputHeight: r, maskedRegions: o };
}
function D(i, t = []) {
  t.push(...Array.from(i.querySelectorAll(ot)));
  const e = Array.from(i.querySelectorAll("*"));
  for (const s of e) {
    const r = s.shadowRoot;
    r && D(r, t);
  }
  return t;
}
function Ct(i, t) {
  return t.getComputedStyle(i).visibility !== "visible" ? !1 : i.getClientRects().length > 0;
}
function _(i, t = []) {
  typeof i.tagName == "string" && t.push(i);
  for (const e of Array.from(i.querySelectorAll("*"))) {
    t.push(e);
    const s = e.shadowRoot;
    s && _(s, t);
  }
  return t;
}
function at(i, t, e, s) {
  const r = s.subtree ? _(i) : [i];
  let o = 1 / 0, l = 1 / 0, h = -1 / 0, a = -1 / 0, d = !1;
  for (const g of r)
    if (Ct(g, t))
      for (const p of Array.from(g.getClientRects())) {
        if (!Number.isFinite(p.left) || !Number.isFinite(p.top) || !Number.isFinite(p.right) || !Number.isFinite(p.bottom))
          throw new f("locate-failed", "敏感节点边界坐标非有限，无法安全遮挡");
        p.width <= 0 || p.height <= 0 || (d = !0, o = Math.min(o, p.left), l = Math.min(l, p.top), h = Math.max(h, p.right), a = Math.max(a, p.bottom));
      }
  if (!d) return null;
  const c = Math.max(0, Math.min(o, e.width)), u = Math.max(0, Math.min(l, e.height)), b = Math.max(0, Math.max(h, c)), y = Math.max(0, Math.max(a, u)), x = Math.min(b, e.width) - c, m = Math.min(y, e.height) - u;
  return x <= 0 || m <= 0 ? null : { x: c, y: u, width: x, height: m };
}
function Lt(i, t = {}) {
  const e = i.defaultView;
  if (!e) return 0;
  const s = { width: e.innerWidth, height: e.innerHeight };
  let r = 0;
  for (const o of D(i)) {
    if (t.ignore?.(o)) continue;
    const l = o.hasAttribute("data-feedback-capture-mask");
    at(o, e, s, { subtree: !!l }) !== null && r++;
  }
  return r;
}
function Pt(i) {
  const t = i.innerWidth, e = i.innerHeight;
  if (!v(t) || !v(e) || t <= 0 || e <= 0)
    throw new f("locate-failed", "无法获得有限的视口尺寸");
  const s = i.devicePixelRatio;
  return {
    width: t,
    height: e,
    scrollX: Number.isFinite(i.scrollX) ? i.scrollX : 0,
    scrollY: Number.isFinite(i.scrollY) ? i.scrollY : 0,
    dpr: v(s) && s > 0 ? s : 1
  };
}
function Rt(i, t) {
  const e = Math.max(i.width, i.height), s = i.width * i.height;
  return Math.min(t, 2, P / e, Math.sqrt(R / s));
}
const It = `
${ot},
[data-feedback-capture-mask] *,
[data-feedback-capture-mask]::before,
[data-feedback-capture-mask]::after,
[data-feedback-capture-mask] *::before,
[data-feedback-capture-mask] *::after,
input[type="password"]::before,
input[type="password"]::after {
  visibility: hidden !important;
  background-image: none !important;
  box-shadow: none !important;
}
`;
function Mt(i, t, e = 0) {
  const s = i.defaultView;
  if (!s) throw new f("locate-failed", "克隆文档缺少可布局的 window，无法定位敏感节点");
  const r = D(i), o = [];
  for (const h of r) {
    const a = at(h, s, t, { subtree: h.hasAttribute("data-feedback-capture-mask") });
    a && o.push(a);
  }
  if (o.length < e)
    throw new f(
      "locate-failed",
      `页面有 ${e} 个可见敏感区域，但克隆上仅定位到 ${o.length} 个，无法保证遮挡`
    );
  const l = i.createElement("style");
  l.setAttribute("data-feedback-capture-mask-style", ""), l.textContent = It, (i.head ?? i.documentElement).appendChild(l);
  for (const h of r) {
    const a = h.hasAttribute("data-feedback-capture-mask") ? _(h) : [h];
    for (const d of a) {
      const c = d;
      c.style && (c.style.setProperty("visibility", "hidden", "important"), c.style.setProperty("background-image", "none", "important"));
      const u = d.tagName ? d.tagName.toUpperCase() : "";
      if ((u === "IMG" || u === "CANVAS" || u === "VIDEO" || u === "IFRAME") && (d.removeAttribute("src"), d.removeAttribute("srcset"), d.removeAttribute("poster")), u === "SOURCE" && (d.removeAttribute("src"), d.removeAttribute("srcset")), u === "IMAGE" && (d.removeAttribute("href"), d.removeAttribute("xlink:href")), u === "INPUT" && d.type === "password")
        try {
          d.value = "";
        } catch {
        }
    }
  }
  return o;
}
function Tt(i, t, e, s, r) {
  if (!Number.isFinite(i.x) || !Number.isFinite(i.y) || !Number.isFinite(i.width) || !Number.isFinite(i.height))
    throw new f("locate-failed", "遮挡区域坐标非有限");
  if (!Number.isFinite(t) || !Number.isFinite(e) || t <= 0 || e <= 0)
    throw new f("verify-failed", "遮挡缩放比例非有限，无法映射到输出位图");
  const o = Math.max(0, Math.floor(i.x * t) - 1), l = Math.max(0, Math.floor(i.y * e) - 1), h = Math.min(s, Math.ceil((i.x + i.width) * t) + 1), a = Math.min(r, Math.ceil((i.y + i.height) * e) + 1), d = h - o, c = a - l;
  if (d <= 0 || c <= 0)
    throw new f("verify-failed", "遮挡区域投影到输出位图后退化，无法保证覆盖");
  return { x: o, y: l, width: d, height: c };
}
function qt(i, t) {
  const e = i.getContext("2d");
  if (!e) throw new f("verify-failed", "无法取得最终位图的 2D 上下文");
  e.save();
  try {
    e.setTransform(1, 0, 0, 1, 0, 0), e.globalAlpha = 1, e.globalCompositeOperation = "source-over", e.fillStyle = At;
    for (const s of t) e.fillRect(s.x, s.y, s.width, s.height);
  } finally {
    e.restore();
  }
}
function Ut(i, t) {
  if (t.length === 0) return;
  const e = i.getContext("2d", { willReadFrequently: !0 });
  if (!e) throw new f("verify-failed", "无法读取最终位图验证遮挡结果");
  for (const s of t) {
    const r = Math.floor(s.x), o = Math.floor(s.y), l = Math.max(1, Math.ceil(s.width)), h = Math.max(1, Math.ceil(s.height));
    if (r < 0 || o < 0 || r + l > i.width || o + h > i.height)
      throw new f("verify-failed", "遮挡矩形超出画布范围，无法验证");
    let a;
    try {
      a = e.getImageData(r, o, l, h).data;
    } catch (m) {
      throw new f("verify-failed", `位图被污染或不可读，无法验证遮挡：${m.message}`);
    }
    const { r: d, g: c, b: u } = Et, b = Math.max(1, Math.floor(l / 33)), y = Math.max(1, Math.floor(h / 33)), x = (m, g) => {
      const p = (g * l + m) * 4;
      return a[p] === d && a[p + 1] === c && a[p + 2] === u && a[p + 3] === 255;
    };
    for (let m = 0; m < h; m += y) {
      for (let g = 0; g < l; g += b)
        if (!x(g, m))
          throw new f("verify-failed", `遮挡验证失败：区域 (${r + g}, ${o + m}) 未被不透明覆盖`);
      if (!x(l - 1, m))
        throw new f("verify-failed", `遮挡验证失败：区域右缘 (${r + l - 1}, ${o + m}) 未被不透明覆盖`);
    }
    if (!x(0, h - 1) || !x(l - 1, h - 1))
      throw new f("verify-failed", "遮挡验证失败：区域底缘未被不透明覆盖");
  }
}
function G(i) {
  return new Promise((t, e) => {
    i.toBlob((s) => s ? t(s) : e(new f("verify-failed", "Canvas toBlob 失败")), "image/png");
  });
}
function zt(i, t) {
  const e = Math.max(1, Math.floor(i.width * t)), s = Math.max(1, Math.floor(i.height * t)), r = document.createElement("canvas");
  r.width = e, r.height = s;
  const o = r.getContext("2d");
  if (!o) throw new f("verify-failed", "缩小重画无法取得 2D 上下文");
  return o.drawImage(i, 0, 0, e, s), r;
}
function Ft(i, t) {
  if (!Number.isFinite(i) || !Number.isFinite(t) || i <= 0 || t <= 0)
    throw new f("verify-failed", "输出位图尺寸非有限");
  if (Math.max(i, t) > P || i * t > R)
    throw new f("verify-failed", "输出位图超出两端统一限制");
}
function q(i) {
  if (i?.aborted) throw new f("aborted", "截图会话已失效");
}
async function Ot() {
  const i = await import("./html2canvas-pro.esm-CQ8baKsv.js");
  return i.default || i;
}
async function Nt(i) {
  q(i.signal);
  const { viewport: t } = i, e = Rt(t, t.dpr);
  let s = [];
  const o = await (i.html2canvas ?? await Ot())(document.documentElement, {
    x: t.scrollX,
    y: t.scrollY,
    width: t.width,
    height: t.height,
    scale: e,
    useCORS: !0,
    allowTaint: !1,
    logging: !1,
    ignoreElements: (u) => i.ignore?.(u) ?? !1,
    onclone: (u) => {
      s = Mt(u, t, i.expectedSensitiveCount);
    }
  });
  if (q(i.signal), !o || !Number.isFinite(o.width) || !Number.isFinite(o.height) || o.width <= 0 || o.height <= 0)
    throw new f("verify-failed", "html2canvas 未产出有效画布");
  let l = o, h = [];
  const a = () => {
    Ft(l.width, l.height);
    const u = l.width / t.width, b = l.height / t.height;
    h = s.map(
      (y) => Tt(y, u, b, l.width, l.height)
    ), h.length > 0 && (qt(l, h), Ut(l, h));
  };
  a();
  let d = await G(l), c = 0;
  for (; d.size > I && c < kt; )
    c++, q(i.signal), l = zt(l, St), a(), d = await G(l);
  if (d.size > I)
    throw new f("size-limit", "缩小重编码三次后仍超过大小限制");
  return {
    blob: d,
    viewportWidth: t.width,
    viewportHeight: t.height,
    outputWidth: l.width,
    outputHeight: l.height,
    // 与最终输出位图一致（含缩小重编码后的坐标）的遮挡区域像素矩形
    maskedRegions: h.map((u) => ({ ...u }))
  };
}
const L = 2e3, Dt = 5e3, _t = 12e4, U = 3e4, Vt = 864e5, Y = "截取当前页面", Ht = "截取中…";
function Z(i) {
  return i === "archived" || i === "failed" || i === "needs_review";
}
function $t(i) {
  return i === void 0 || Number.isNaN(i) ? "" : i < 1024 ? `${i} B` : `${(i / 1024).toFixed(1)} KB`;
}
const E = 3, J = 1024 * 1024, tt = /* @__PURE__ */ new Set([".log", ".txt", ".json", ".jsonl"]);
function et(i) {
  const t = i.lastIndexOf(".");
  return t === -1 ? "" : i.slice(t).toLowerCase();
}
function z(i, t) {
  if (i.length !== t.length) return !1;
  for (let e = 0; e < i.length; e++) {
    const s = i[e], r = t[e];
    if (!s || !r || s.blob !== r.blob || s.filename !== r.filename || s.source !== r.source)
      return !1;
  }
  return !0;
}
async function jt(i) {
  return typeof i.text == "function" ? await i.text() : new Promise((t, e) => {
    const s = new FileReader();
    s.onload = () => t(s.result), s.onerror = () => e(s.error), s.readAsText(i, "utf-8");
  });
}
function n(i, t, e) {
  const s = document.createElement(i);
  return t && (s.className = t), e !== void 0 && (s.textContent = e), s;
}
function it(i) {
  const t = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  t.setAttribute("class", i), t.setAttribute("viewBox", "0 0 20 20"), t.setAttribute("fill", "none"), t.setAttribute("stroke", "currentColor"), t.setAttribute("stroke-width", "1.6"), t.setAttribute("stroke-linecap", "round"), t.setAttribute("stroke-linejoin", "round"), t.setAttribute("aria-hidden", "true");
  const e = document.createElementNS("http://www.w3.org/2000/svg", "path");
  return e.setAttribute(
    "d",
    "M17 10.5a5.5 5.5 0 0 1-5.5 5.5c-1.1 0-2.1-.3-3-.9L4 16l.9-4.5A5.5 5.5 0 1 1 17 10.5z"
  ), t.append(e), t;
}
function Qt() {
  const i = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  i.setAttribute("class", "fb-orb-icon"), i.setAttribute("viewBox", "0 0 24 24"), i.setAttribute("fill", "none"), i.setAttribute("stroke", "currentColor"), i.setAttribute("stroke-width", "1.8"), i.setAttribute("stroke-linecap", "round"), i.setAttribute("stroke-linejoin", "round"), i.setAttribute("aria-hidden", "true");
  const t = document.createElementNS("http://www.w3.org/2000/svg", "circle");
  t.setAttribute("cx", "12"), t.setAttribute("cy", "12"), t.setAttribute("r", "8.5");
  const e = document.createElementNS("http://www.w3.org/2000/svg", "circle");
  return e.setAttribute("cx", "12"), e.setAttribute("cy", "12"), e.setAttribute("r", "2.5"), e.setAttribute("fill", "currentColor"), i.append(t, e), i;
}
function F() {
  const i = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  i.setAttribute("viewBox", "0 0 20 20"), i.setAttribute("width", "14"), i.setAttribute("height", "14"), i.setAttribute("fill", "none"), i.setAttribute("stroke", "currentColor"), i.setAttribute("stroke-width", "2"), i.setAttribute("stroke-linecap", "round"), i.setAttribute("aria-hidden", "true");
  const t = document.createElementNS("http://www.w3.org/2000/svg", "line");
  t.setAttribute("x1", "5"), t.setAttribute("y1", "5"), t.setAttribute("x2", "15"), t.setAttribute("y2", "15");
  const e = document.createElementNS("http://www.w3.org/2000/svg", "line");
  return e.setAttribute("x1", "15"), e.setAttribute("y1", "5"), e.setAttribute("x2", "5"), e.setAttribute("y2", "15"), i.append(t, e), i;
}
function Kt() {
  const i = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  i.setAttribute("viewBox", "0 0 20 20"), i.setAttribute("width", "14"), i.setAttribute("height", "14"), i.setAttribute("fill", "none"), i.setAttribute("stroke", "currentColor"), i.setAttribute("stroke-width", "1.6"), i.setAttribute("stroke-linecap", "round"), i.setAttribute("aria-hidden", "true");
  const t = document.createElementNS("http://www.w3.org/2000/svg", "circle");
  t.setAttribute("cx", "10"), t.setAttribute("cy", "10"), t.setAttribute("r", "2.4"), i.append(t);
  for (const [e, s, r, o] of [
    [10, 2.2, 10, 4.4],
    [10, 15.6, 10, 17.8],
    [2.2, 10, 4.4, 10],
    [15.6, 10, 17.8, 10],
    [4.5, 4.5, 6.1, 6.1],
    [13.9, 13.9, 15.5, 15.5],
    [4.5, 15.5, 6.1, 13.9],
    [13.9, 6.1, 15.5, 4.5]
  ]) {
    const l = document.createElementNS("http://www.w3.org/2000/svg", "line");
    l.setAttribute("x1", String(e)), l.setAttribute("y1", String(s)), l.setAttribute("x2", String(r)), l.setAttribute("y2", String(o)), i.append(l);
  }
  return i;
}
class M extends HTMLElement {
  /** Shadow DOM 模式：默认 open；注册前可设 FeedbackWidget.shadowMode='closed'。 */
  static shadowMode = "open";
  static get observedAttributes() {
    return [
      "api-base",
      "app-id",
      "app-name",
      "app-version",
      "page-label",
      "side",
      "theme",
      "show-launcher",
      "launcher-bottom",
      "launcher-mode",
      "capture-mode"
    ];
  }
  root;
  launcher;
  orb;
  panel;
  metaInfo;
  /** 截图区 = 预览（有截图才可见）+ 操作区（没有截图时是「截取当前页面」）。 */
  shotArea;
  screenshotWrap;
  screenshotThumbBox;
  screenshotThumb;
  /** 首个截图入口（无截图时显示）：默认 capture-mode=off 宿主唯一的手动截图方式。 */
  captureBtn;
  retakeBtn;
  removeBtn;
  zoomModal;
  zoomImg;
  zoomCloseBtn;
  textarea;
  counter;
  submitBtn;
  statusRegion;
  errorRegion;
  /** 今日剩余额度（登录后显示；未登录/未取得时隐藏）。 */
  quotaInfo;
  /** 额度用尽提示（含下次可提交时间）。 */
  quotaBlockedInfo;
  /** 面板内登录表单（点击「登录并提交」展开，不打开新窗口）。 */
  loginPanel;
  loginUsername;
  loginPassword;
  loginErrorEl;
  loginConfirmBtn;
  loginCancelBtn;
  /** T6：面板主体（设置视图展开时整体隐藏）。 */
  body;
  /** T6：服务器设置入口与视图。 */
  settingsBtn;
  settingsView;
  settingsInput;
  settingsCurrent;
  settingsDefaultRow;
  settingsDefaultText;
  settingsErrorEl;
  settingsHintEl;
  settingsSaveBtn;
  settingsRestoreBtn;
  settingsCancelBtn;
  settingsConfirmEl;
  settingsConfirmText;
  /** 日志附件区 */
  logsArea;
  logsCountEl;
  logsList;
  addLogBtn;
  logFileInput;
  logStatusEl;
  logErrorEl;
  logPreviewModal;
  logPreviewTitle;
  logPreviewBody;
  logPreviewCloseBtn;
  _logProvider;
  logsCollecting = !1;
  logTimeout = null;
  phase = "idle";
  polling = !1;
  pollTimer = null;
  pollDelay = L;
  pollStartedAt = 0;
  openState = !1;
  authRequired = !1;
  /** 当前登录账号（登录响应提供；仅用于展示与额度归属）。 */
  authUser = null;
  /** 最近一次取得的每日额度；resetAt 之后重新查询，不在本地擅自重置。 */
  quota = null;
  loginVisible = !1;
  loginBusy = !1;
  loginError = "";
  visibilityHandler = null;
  /** 手动刷新服务端记录状态进行中（按钮禁用 + 防重入）。 */
  refreshing = !1;
  /**
   * T4：当前记录的「等待项」提示（等待配置 / 等待来源确认 / 等待人工归档）。
   * 非空表示记录已妥善保存、只是在等管理员操作——此时**不轮询**（等待不是处理中，
   * 无限轮询没有意义），改为提示 + 手动刷新；管理员处理后刷新即可看到结果。
   */
  waitingNotice = null;
  /**
   * 登录操作序号（T1-A）：每次发起登录递增；面板关闭 / 取消登录 /
   * 组件卸载 / 服务身份变化都会递增使在途登录失效，并清除密码、
   * 登录忙碌状态与待自动提交标记。
   *
   * 登录回调返回后必须同时满足「序号未变 + 身份世代未变 + 组件仍连接」，
   * 才允许写入令牌、更新界面或自动提交；失败回调同样校验，
   * 防止旧登录的错误覆盖新登录。由于 close() 也会递增序号，
   * 「序号仍有效」等价于「面板自登录发起以来没有被关闭」。
   */
  loginSeq = 0;
  /**
   * 额度查询序号（T1-B）：提交成功 / 收到额度错误 / 关闭 / 卸载 /
   * 退到后台 / 身份变化都会递增使在途查询失效；查询结果除校验身份、
   * 令牌外还要校验该序号，避免旧查询覆盖刚扣除后的次数。
   */
  quotaSeq = 0;
  /** 额度查询定时器（resetAt 单次 或 额度用尽后每 30 秒）；同时最多一个。 */
  quotaTimer = null;
  /** 额度查询进行中：同一时刻最多一个额度查询。 */
  quotaInFlight = !1;
  /** 最近一次额度刷新失败（网络等）：保留旧额度，30 秒后重试。 */
  quotaRefreshFailed = !1;
  /**
   * 「待立即刷新」标记（T1-B）：打开面板 / 回到前台的刷新被旧查询或登录
   * 忙碌挡下时登记，不得丢弃；旧查询结束后或忙碌结束后立即补发一次。
   * 多次登记合并为一次；关闭 / 卸载 / 退到后台 / 身份变化 / 401 时清除，
   * 重新打开按当前身份重新登记。
   */
  quotaRefreshPending = !1;
  /**
   * 服务身份世代（epoch）：`api-base` 或 `app-id` 变化时递增。
   * 每个异步操作在开始时捕获当前 epoch，恢复后先校验：
   * epoch 已变 → 该结果是旧服务 / 旧身份的，一律成为 no-op，
   * 绝不写入新身份的状态（提交响应、轮询记录、捕获会话、登录握手回调）。
   */
  identityEpoch = 0;
  /** 灵感球拖拽状态 */
  isDragging = !1;
  orbPointerId = null;
  isCapturing = !1;
  /** 进行中的捕获会话控制器：cancelCapture / 卸载时 abort，使旧会话失效。 */
  captureController = null;
  /** 捕获会话序号：新会话 / 失效操作递增；异步步骤后校验，旧会话结果一律丢弃。 */
  captureSeq = 0;
  /** 正在进行的捕获会话：普通呼出合并到该会话，不重复发起。 */
  captureInFlight = null;
  /**
   * 未提交草稿：组件实例所有（断开重连保留字节并重建自己的预览 URL），
   * 仅存内存，刷新即弃，绝不用 localStorage；appId 变化时整体废弃，
   * 旧捕获/旧草稿不得写入新身份。
   */
  draft = {
    text: "",
    screenshotBlob: null,
    screenshotUrl: null,
    captureInfo: null,
    logs: [],
    logsCollected: !1,
    version: 0
  };
  /** 当前提交快照（冻结）；失败且草稿未变时复用（同 key 同字节重试）。 */
  submitSnapshot = null;
  /** 结果未知的原提交请求：草稿被修改时保留供人工核对，不静默覆盖。 */
  unconfirmedRequest = null;
  /** 日志采集异步操作序号，防止迟到结果写入已切换的身份或草稿。 */
  logOpSeq = 0;
  invalidateLogCollection() {
    this.logOpSeq++, this.logsCollecting = !1, this.logTimeout !== null && (clearTimeout(this.logTimeout), this.logTimeout = null);
  }
  /**
   * 自定义日志提供者（L1-Web）：
   * 自动采集环境日志或诊断信息。返回单个或多个日志文件对象。
   */
  get logProvider() {
    return this._logProvider;
  }
  set logProvider(t) {
    this._logProvider = t, t && this.openState && !this.draft.logsCollected && this.draft.logs.length === 0 && this.collectLogs();
  }
  /**
   * 自定义截图提供者（扩展契约，实施计划 2.1）：
   * 保留原参数与返回值；ctx 新增可选 signal / viewport / sensitiveRegionCount，
   * 结果新增可选 viewport / sameFrameMasking / maskedRegions（输出像素坐标）。
   * 页面无可见敏感区域时旧式回调（只返回 {blob,width,height}）保持兼容。
   * 替换提供者（含置空）会使进行中的旧捕获会话失效。
   */
  _captureProvider;
  get captureProvider() {
    return this._captureProvider;
  }
  set captureProvider(t) {
    t !== this._captureProvider && this.invalidateCaptureSession(), this._captureProvider = t;
  }
  /**
   * T6：本机选择的自定义服务器覆盖（已规范化地址）；null 表示使用宿主 api-base。
   * 覆盖偏好按 localStorage 源 + appId + 规范化默认地址逐槽位隔离，
   * 仅在构造/身份属性变化时加载，不反写宿主配置。
   */
  serverOverride = null;
  /** T6：本机偏好是否可写（读取或写入失败置 false，设置视图据此提示「仅本次生效」）。 */
  serverPrefWritable = !0;
  /** T6：最近一次加载覆盖所用的偏好键——断开重连 / 重复调用不重复读取，避免吞掉「仅本次生效」的会话内选择。 */
  serverOverrideKeyLoaded = "";
  /** T6：设置视图是否展开（未登录也可进入）。 */
  settingsOpen = !1;
  /** T6：设置视图内的「确认切换」块是否展开（有草稿或结果未确认提交时先确认）。 */
  settingsConfirmOpen = !1;
  /** T6：等待确认的目标覆盖（null=恢复默认，undefined=无待确认切换）。 */
  pendingServerTarget = void 0;
  /** 令牌仅存组件实例内存；页面刷新后靠重新握手恢复。 */
  accessToken = null;
  tokenExpiresAt = 0;
  idempotencyKey = null;
  lastFeedbackId = null;
  lastSubmittedText = "";
  lastErrorSummary = null;
  lastRecord = null;
  keydownHandler = null;
  handledKey = null;
  constructor() {
    super(), this.root = this.attachShadow({ mode: M.shadowMode }), this.buildDom(), this.bindEvents(), this.syncUi();
  }
  // ---------- 公开 API ----------
  get apiBase() {
    return this.getAttribute("api-base");
  }
  set apiBase(t) {
    this.reflect("api-base", t);
  }
  /**
   * T6：当前实际使用的服务器地址（只读）：
   * 本机覆盖优先于宿主 `api-base`；未设置覆盖时返回宿主默认值。
   */
  get effectiveApiBase() {
    return this.serverOverride ?? this.apiBase;
  }
  get appId() {
    return this.getAttribute("app-id");
  }
  set appId(t) {
    this.reflect("app-id", t);
  }
  /**
   * 可选的软件名称（随提交上报）。
   * 服务端只在**管理员尚未设置名称**时采用；管理员设置过就绝不会被覆盖。
   */
  get appName() {
    return this.getAttribute("app-name");
  }
  set appName(t) {
    this.reflect("app-name", t);
  }
  get appVersion() {
    return this.getAttribute("app-version");
  }
  set appVersion(t) {
    this.reflect("app-version", t), this.updateMetaInfo();
  }
  get pageLabel() {
    return this.getAttribute("page-label");
  }
  set pageLabel(t) {
    this.reflect("page-label", t), this.updateMetaInfo();
  }
  get side() {
    return this.getAttribute("side") === "left" ? "left" : "right";
  }
  set side(t) {
    this.reflect("side", t === "left" ? "left" : "right");
  }
  get theme() {
    const t = this.getAttribute("theme");
    return t === "light" || t === "dark" ? t : "system";
  }
  set theme(t) {
    t === "system" ? this.removeAttribute("theme") : this.setAttribute("theme", t);
  }
  get showLauncher() {
    return this.getAttribute("show-launcher") !== "false";
  }
  set showLauncher(t) {
    t ? this.removeAttribute("show-launcher") : this.setAttribute("show-launcher", "false");
  }
  get launcherBottom() {
    return this.getAttribute("launcher-bottom") ?? "25%";
  }
  set launcherBottom(t) {
    this.reflect("launcher-bottom", t);
  }
  get launcherMode() {
    return this.getAttribute("launcher-mode") === "orb" ? "orb" : "tab";
  }
  set launcherMode(t) {
    t === "tab" ? this.removeAttribute("launcher-mode") : this.setAttribute("launcher-mode", "orb");
  }
  get captureMode() {
    return this.getAttribute("capture-mode") === "viewport" ? "viewport" : "off";
  }
  set captureMode(t) {
    t === "off" ? this.removeAttribute("capture-mode") : this.setAttribute("capture-mode", "viewport");
  }
  reflect(t, e) {
    e === null || e === "" ? this.removeAttribute(t) : this.setAttribute(t, e);
  }
  open() {
    this.openState || (this.openState = !0, this.launcher.classList.add("is-hidden"), this.launcher.setAttribute("aria-expanded", "true"), this.orb.classList.add("is-hidden"), this.orb.setAttribute("aria-expanded", "true"), this.panel.classList.add("is-open"), this.panel.hidden = !1, this.updateAriaModal(), this.textarea.focus(), this.refreshQuota(), this._logProvider && !this.draft.logsCollected && this.draft.logs.length === 0 && this.collectLogs(), this.keydownHandler = (t) => this.onKeydown(t), document.addEventListener("keydown", this.keydownHandler, !0), this.phase === "tracking" && (this.waitingNotice !== null ? this.refreshLastRecord() : this.resumePolling()));
  }
  close() {
    this.settingsOpen = !1, this.settingsConfirmOpen = !1, this.pendingServerTarget = void 0, this.invalidateLogin(), this.invalidateQuotaRefresh(), this.quotaRefreshPending = !1, this.invalidateLogCollection(), this.invalidateCaptureSession(), this.captureInFlight = null, this.restoreCaptureUi(), this.openState && (this.openState = !1, this.closeZoomModal(), this.closeLogPreview(), this.panel.classList.remove("is-open"), window.setTimeout(() => {
      this.openState || (this.panel.hidden = !0);
    }, 200), this.launcher.classList.remove("is-hidden"), this.launcher.setAttribute("aria-expanded", "false"), this.orb.classList.remove("is-hidden"), this.orb.setAttribute("aria-expanded", "false"), this.keydownHandler && (document.removeEventListener("keydown", this.keydownHandler, !0), this.keydownHandler = null), this.launcherMode === "orb" ? this.orb.focus() : this.launcher.focus());
  }
  cancelCapture() {
    if (this.invalidateCaptureSession(), this.captureInFlight = null, this.orbPointerId !== null)
      try {
        this.orb.releasePointerCapture(this.orbPointerId);
      } catch {
      }
    this.isDragging = !1, this.orb.classList.remove("is-dragging"), this.orb.style.transform = "", this.orbPointerId = null, this.restoreCaptureUi();
  }
  /**
   * 使当前捕获会话失效：序号先行失效 → abort 取消；失效方负责恢复 UI
   * 与清理 in-flight（旧会话的 finally 不会再恢复新会话的 UI）。
   */
  invalidateCaptureSession() {
    this.captureSeq++, this.captureController?.abort(), this.captureController = null, this.captureInFlight = null, this.restoreCaptureUi();
  }
  /** 恢复被捕获流程隐藏的组件 UI（会话失效方负责恢复）。 */
  restoreCaptureUi() {
    this.panel.style.visibility = "", this.launcher.style.visibility = "", this.orb.style.visibility = "", this.isCapturing = !1, this.syncShotUi();
  }
  /** 草稿是否已有内容（有文字、截图或日志的草稿再次呼出时恢复草稿、不重拍）。 */
  isDraftDirty() {
    return this.draft.text.trim().length > 0 || this.draft.screenshotBlob !== null || this.draft.logs.length > 0;
  }
  /** 组件自身 UI 不参与截图，也不计入宿主页面敏感区域。 */
  isOwnFeedbackUi(t) {
    if (t === this || t.closest("feedback-widget") !== null) return !0;
    let e = t;
    for (; e; ) {
      const s = e.getRootNode();
      if (s instanceof ShadowRoot) {
        if (s.host === this || s.host.tagName.toLowerCase() === "feedback-widget") return !0;
        e = s.host;
      } else
        break;
    }
    return !1;
  }
  /**
   * 普通呼出（launcher / orb 点击 / 拖拽落点）：
   * - 已有草稿（文字或截图）→ 恢复草稿打开面板，不重拍；
   * - 正在捕获 → 合并到进行中的会话，不重复发起；
   * - 提交进行中 → 禁止编辑类操作，直接忽略。
   */
  async captureAndOpen(t = {}) {
    if (this.captureInFlight) return this.captureInFlight;
    if (this.phase === "submitting") return;
    if (this.isDraftDirty()) {
      this.openState || this.open(), this.syncUi();
      return;
    }
    const e = this.runCapture(t);
    this.captureInFlight = e;
    try {
      await e;
    } finally {
      this.captureInFlight === e && (this.captureInFlight = null);
    }
  }
  /**
   * 重拍（内部明确入口）：可取消旧会话后重新捕获；只有重拍替换旧截图，
   * 且失败不丢旧图（runCapture 失败路径不触碰草稿）。
   */
  async retakeScreenshot() {
    if (this.phase === "submitting") return;
    this.invalidateCaptureSession(), this.captureInFlight = null;
    const t = this.runCapture({});
    this.captureInFlight = t;
    try {
      await t;
    } finally {
      this.captureInFlight === t && (this.captureInFlight = null);
    }
  }
  /**
   * 面板内的手动截图入口（首次截图与重拍共用同一条路径）。
   *
   * 刻意**不**走 `captureAndOpen()`：那条路径是「呼出」语义——草稿一旦有内容
   * （哪怕只有文字）就只恢复草稿开面板、绝不重拍，于是默认 `capture-mode="off"`
   * 的宿主在用户先输入文字后，就再没有任何补拍入口（本次修复的缺口）。
   * 这里复用显式重拍流程：可替换旧图，失败不丢旧图与文字。
   *
   * 按钮在捕获期间已禁用；这里再挡一次重入（禁用态只是视觉 / a11y 保证，
   * 程序化调用与键盘连击不该产生并发会话）。
   */
  captureFromPanel() {
    if (this.phase === "submitting" || this.polling || this.isCapturing) return;
    const t = this.root.activeElement;
    this.retakeScreenshot().finally(() => this.restorePanelFocus(t));
  }
  /**
   * 捕获期间面板被 `visibility: hidden` 隐藏，真实浏览器会把焦点丢到 body
   * （面板内的按钮 / textarea 全部失焦）。两种情况要把焦点还回去：
   * 焦点已经不在面板内（`null`），或**卡在一个已经隐藏的控件上**
   * （个别引擎不主动移除隐藏元素的焦点）。优先点击前的元素（若它已隐藏 /
   * 禁用则回到 textarea），键盘用户不会在截图后凭空失去落点；
   * 用户已主动聚焦到可见的别处时绝不抢焦点。
   */
  restorePanelFocus(t) {
    if (!this.openState) return;
    const e = this.root.activeElement, s = e !== null && (e.hidden || e.closest("[hidden]") !== null);
    if (e !== null && !s) return;
    (t !== null && t.isConnected && !t.hidden && !t.disabled && t ? t : this.textarea).focus();
  }
  /**
   * 截图区同步：**预览与操作分离**。
   * - 预览只在真的有可渲染 blob URL 时出现：绝不留下空 src 的破图占位
   *   （历史故障与其成因见 styles.ts 的 `[hidden]` 兜底注释）。
   * - 无截图 → 只显示「截取当前页面」，缩略图 / 放大 / 移除全部隐藏；
   *   有截图 → 预览 + 「重新截图 / 移除截图」；移除后回到首个入口，文字保留。
   * - 提交、轮询与捕获会话进行中，三个操作一律禁用（防重入）。
   */
  syncShotUi() {
    if (!this.shotArea) return;
    const t = this.draft.screenshotUrl, e = this.draft.screenshotBlob !== null, s = this.phase === "submitting" || this.polling || this.isCapturing;
    this.screenshotWrap.hidden = t === null, t !== null ? this.screenshotThumb.src = t : this.screenshotThumb.removeAttribute("src"), this.screenshotThumbBox.tabIndex = t === null ? -1 : 0, this.captureBtn.hidden = e, this.retakeBtn.hidden = !e, this.removeBtn.hidden = !e, this.captureBtn.disabled = s, this.retakeBtn.disabled = s, this.removeBtn.disabled = s, this.captureBtn.textContent = this.isCapturing ? Ht : Y, this.isCapturing ? this.captureBtn.setAttribute("aria-busy", "true") : this.captureBtn.removeAttribute("aria-busy"), this.shotArea.classList.toggle("is-capturing", this.isCapturing);
  }
  /** 单次捕获会话主体：每个异步步骤后与更新草稿 / 开面板前都校验会话有效性。 */
  async runCapture(t) {
    const e = ++this.captureSeq, s = this.identityEpoch, r = () => e === this.captureSeq && s === this.identityEpoch, o = new AbortController();
    this.captureController = o;
    const { signal: l } = o;
    this.isCapturing = !0, this.syncShotUi();
    const h = this.openState;
    this.panel.style.visibility = "hidden", this.launcher.style.visibility = "hidden", this.orb.style.visibility = "hidden";
    let a = null;
    try {
      if (l.aborted || !r()) throw new f("aborted", "截图会话已失效");
      if (typeof window > "u") throw new f("locate-failed", "无可用 window 环境");
      const d = Pt(window), c = Lt(document, {
        ignore: (g) => this.isOwnFeedbackUi(g)
      });
      let u, b, y, x, m;
      if (this._captureProvider) {
        const g = await this._captureProvider({
          releasePoint: t.releasePoint,
          signal: l,
          viewport: d,
          sensitiveRegionCount: c
        });
        if (l.aborted || !r()) throw new f("aborted", "截图会话已失效");
        const p = Bt(g, c);
        u = p.blob, b = p.viewportWidth, y = p.viewportHeight, x = p.outputWidth, m = p.outputHeight;
      } else {
        const g = await Nt({
          viewport: d,
          expectedSensitiveCount: c,
          signal: l,
          ignore: (p) => p === this || p.tagName.toLowerCase() === "feedback-widget"
        });
        if (l.aborted || !r()) throw new f("aborted", "截图会话已失效");
        u = g.blob, b = g.viewportWidth, y = g.viewportHeight, x = g.outputWidth, m = g.outputHeight;
      }
      if (!r()) throw new f("aborted", "截图会话已失效");
      this.draft.screenshotUrl && URL.revokeObjectURL(this.draft.screenshotUrl), this.draft.screenshotBlob = u, this.draft.screenshotUrl = URL.createObjectURL(u), this.draft.captureInfo = {
        viewportWidth: b,
        viewportHeight: y,
        pixelWidth: x,
        pixelHeight: m,
        capturedAt: (/* @__PURE__ */ new Date()).toISOString(),
        ...t.releasePoint ? { releasePoint: t.releasePoint } : {}
      }, this.draft.version++;
    } catch (d) {
      d instanceof f && d.reason === "aborted" || (a = d instanceof f ? d.userMessage : nt, console.warn("[Feedback] Screenshot capture failed:", d));
    } finally {
      r() && (this.captureController === o && (this.captureController = null), this.restoreCaptureUi());
    }
    r() && (h || this.open(), this.syncUi(), a && this.renderStatus(a));
  }
  openZoomModal() {
    this.draft.screenshotUrl && (this.zoomImg.src = this.draft.screenshotUrl, this.zoomModal.classList.add("is-open"));
  }
  closeZoomModal() {
    this.zoomModal && this.zoomModal.classList.remove("is-open");
  }
  /** 移除截图：同时移除落点与时间（captureInfo 整体废弃），并递增草稿版本。 */
  removeScreenshot() {
    this.phase === "submitting" || this.polling || (this.draft.screenshotUrl && (URL.revokeObjectURL(this.draft.screenshotUrl), this.draft.screenshotUrl = null), this.draft.screenshotBlob = null, this.draft.captureInfo = null, this.draft.version++, this.syncUi());
  }
  /** 清空草稿（提交成功后 / appId 变化时），字节与预览 URL 一并释放。 */
  clearDraft() {
    this.draft.text = "", this.draft.screenshotUrl && (URL.revokeObjectURL(this.draft.screenshotUrl), this.draft.screenshotUrl = null), this.draft.screenshotBlob = null, this.draft.captureInfo = null, this.draft.logs = [], this.draft.logsCollected = !1, this.draft.version++;
  }
  isMobile() {
    return window.matchMedia("(max-width: 767.98px)").matches;
  }
  updateAriaModal() {
    this.panel.setAttribute("aria-modal", this.isMobile() ? "true" : "false");
  }
  // ---------- 生命周期 ----------
  /** 一次性事件绑定（构造期）：重挂载绝不重复绑定。 */
  bindEvents() {
    this.launcher.addEventListener("click", () => {
      this.openState ? this.close() : this.captureMode === "viewport" ? this.captureAndOpen() : this.open();
    }), this.initOrbGesture(), this.textarea.addEventListener("input", () => {
      this.draft.text = this.textarea.value, this.draft.version++, (this.phase === "archived" || this.phase === "needs_review") && (this.phase = "idle"), this.syncUi();
    }), this.submitBtn.addEventListener("click", () => {
      this.onPrimaryAction();
    });
  }
  connectedCallback() {
    this.hasAttribute("side") || this.setAttribute("side", "right"), this.updateLauncherBottomCss(), this.updateMetaInfo(), this.loadServerOverride(), this.draft.screenshotBlob && !this.draft.screenshotUrl && (this.draft.screenshotUrl = URL.createObjectURL(this.draft.screenshotBlob)), this.visibilityHandler || (this.visibilityHandler = () => {
      document.visibilityState === "visible" ? (this.quotaRefreshFailed = !1, this.refreshQuota()) : (this.invalidateQuotaRefresh(), this.quotaRefreshPending = !1);
    }, document.addEventListener("visibilitychange", this.visibilityHandler)), this.openState && this._logProvider && !this.draft.logsCollected && this.draft.logs.length === 0 && !this.logsCollecting && this.phase !== "submitting" && this.collectLogs(), this.syncUi();
  }
  disconnectedCallback() {
    this.invalidateLogCollection(), this.invalidateCaptureSession(), this.captureInFlight = null, this.stopPolling(), this.invalidateLogin(), this.invalidateQuotaRefresh(), this.quotaRefreshPending = !1, this.visibilityHandler && (document.removeEventListener("visibilitychange", this.visibilityHandler), this.visibilityHandler = null), this.loginPassword.value = "", this.keydownHandler && (document.removeEventListener("keydown", this.keydownHandler, !0), this.keydownHandler = null), this.draft.screenshotUrl && (URL.revokeObjectURL(this.draft.screenshotUrl), this.draft.screenshotUrl = null);
  }
  attributeChangedCallback(t, e, s) {
    if (t === "api-base" && e !== s && (this.settingsOpen = !1, this.settingsConfirmOpen = !1, this.pendingServerTarget = void 0, this.loadServerOverride(), this.identityEpoch++, this.quotaRefreshPending = !1, this.resetForServiceSwitch()), t === "app-id" && e !== s) {
      const r = this.effectiveApiBase;
      this.settingsOpen = !1, this.settingsConfirmOpen = !1, this.pendingServerTarget = void 0, this.loadServerOverride();
      const o = this.effectiveApiBase;
      this.identityEpoch++, this.quotaRefreshPending = !1, this.sameNormalizedBase(r, o) ? this.resetForAppIdSwitch() : this.resetForServiceSwitch();
    }
    t === "launcher-bottom" && this.updateLauncherBottomCss(), (t === "app-version" || t === "page-label") && this.updateMetaInfo();
  }
  /**
   * `api-base` 变化 = 完整身份切换：取消并清空一切属于旧服务的东西
   * （捕获会话 / 提交快照 / 幂等键 / 握手 / 轮询 / 令牌 / 任务态 / 草稿）。
   * 新服务必须重新登录，旧令牌绝不外泄给新基址；旧服务的迟到响应由 epoch 校验丢弃。
   */
  resetForServiceSwitch() {
    this.invalidateLogCollection(), this.invalidateCaptureSession(), this.invalidateLogin(), this.invalidateQuotaRefresh(), this.captureInFlight = null, this.stopPolling(), this.pollDelay = L, this.pollStartedAt = 0, this.submitSnapshot = null, this.idempotencyKey = null, this.unconfirmedRequest = null, this.accessToken = null, this.tokenExpiresAt = 0, this.authRequired = !1, this.pendingSubmit = !1, this.authUser = null, this.quota = null, this.loginVisible = !1, this.loginBusy = !1, this.loginError = "", this.loginPassword.value = "", this.lastFeedbackId = null, this.lastRecord = null, this.lastErrorSummary = null, this.lastSubmittedText = "", this.refreshing = !1, this.phase = "idle", this.clearDraft(), this.textarea.value = "", this.renderStatus(""), this.syncUi();
  }
  /**
   * `app-id` 变化：同一服务内的身份切换——令牌保留（同一服务），
   * 但旧身份的草稿 / 捕获 / 提交结果 / 轮询 / 握手全部作废。
   */
  resetForAppIdSwitch() {
    this.invalidateLogCollection(), this.invalidateCaptureSession(), this.invalidateLogin(), this.invalidateQuotaRefresh(), this.captureInFlight = null, this.restoreCaptureUi(), this.stopPolling(), this.pollDelay = L, this.pollStartedAt = 0, this.clearDraft(), this.textarea.value = "", this.submitSnapshot = null, this.idempotencyKey = null, this.unconfirmedRequest = null, this.lastFeedbackId = null, this.lastRecord = null, this.lastErrorSummary = null, this.lastSubmittedText = "", this.refreshing = !1, this.phase = "idle", this.loginVisible = !1, this.loginBusy = !1, this.loginError = "", this.loginPassword.value = "", this.pendingSubmit = !1, this.renderStatus(""), this.syncUi();
  }
  updateLauncherBottomCss() {
    const t = this.launcherBottom;
    t && this.style.setProperty("--fb-launcher-bottom", t);
  }
  updateMetaInfo() {
    if (!this.metaInfo) return;
    const t = [];
    this.appVersion && t.push(`v${this.appVersion}`), this.pageLabel && t.push(this.pageLabel), this.metaInfo.textContent = t.length > 0 ? t.join(" · ") : "", this.metaInfo.hidden = t.length === 0;
  }
  initOrbGesture() {
    let t = 0, e = 0, s = !1;
    this.orb.addEventListener("pointerdown", (o) => {
      if (o.button === 0) {
        t = o.clientX, e = o.clientY, s = !0, this.isDragging = !1, this.orbPointerId = o.pointerId;
        try {
          this.orb.setPointerCapture(o.pointerId);
        } catch {
        }
      }
    }), this.orb.addEventListener("pointermove", (o) => {
      if (!s || this.orbPointerId !== o.pointerId) return;
      const l = o.clientX - t, h = o.clientY - e;
      this.isDragging || Math.hypot(l, h) > 8 && (this.isDragging = !0, this.orb.classList.add("is-dragging")), this.isDragging && (this.orb.style.transform = `translate3d(${l}px, ${h}px, 0) scale(1.1)`);
    });
    const r = (o, l = !1) => {
      if (!s || this.orbPointerId !== o.pointerId) return;
      s = !1;
      try {
        this.orb.releasePointerCapture(o.pointerId);
      } catch {
      }
      this.orbPointerId = null;
      const h = this.isDragging;
      if (this.isDragging = !1, this.orb.classList.remove("is-dragging"), this.orb.style.transform = "", !l)
        if (h) {
          const a = typeof window < "u" && window.innerWidth || 1, d = typeof window < "u" && window.innerHeight || 1, c = Math.max(0, Math.min(1, o.clientX / a)), u = Math.max(0, Math.min(1, o.clientY / d)), b = {
            x: Math.round(c * 1e4) / 1e4,
            y: Math.round(u * 1e4) / 1e4
          };
          this.captureAndOpen({ releasePoint: b });
        } else
          this.captureMode === "viewport" ? this.captureAndOpen() : this.open();
    };
    this.orb.addEventListener("pointerup", (o) => r(o, !1)), this.orb.addEventListener("pointercancel", (o) => r(o, !0)), this.orb.addEventListener("keydown", (o) => {
      (o.key === " " || o.key === "Enter") && (o.preventDefault(), this.captureMode === "viewport" ? this.captureAndOpen() : this.open());
    });
  }
  // ---------- DOM 构建 ----------
  buildDom() {
    const t = n("style");
    t.textContent = yt, this.launcher = n("button", "fb-launcher fb-fab fb-tab-launcher"), this.launcher.type = "button", this.launcher.setAttribute("aria-label", "打开反馈面板"), this.launcher.setAttribute("aria-expanded", "false"), this.launcher.setAttribute("aria-controls", "fb-panel");
    const e = it("fb-launcher-icon"), s = n("span", void 0, "反馈");
    this.launcher.append(e, s), this.orb = n("button", "fb-launcher fb-orb"), this.orb.type = "button", this.orb.setAttribute("aria-label", "灵感球：拖动指出问题或点击反馈"), this.orb.setAttribute("aria-expanded", "false"), this.orb.setAttribute("aria-controls", "fb-panel"), this.orb.append(Qt()), this.panel = n("div", "fb-panel"), this.panel.id = "fb-panel", this.panel.setAttribute("role", "dialog"), this.panel.setAttribute("aria-label", "记录体验"), this.panel.setAttribute("aria-modal", "false"), this.panel.hidden = !0;
    const r = n("header", "fb-header"), o = n("div", "fb-title-group"), l = it("fb-header-icon"), h = n("h2", void 0, "记录体验");
    this.metaInfo = n("span", "fb-header-meta"), this.metaInfo.hidden = !0, o.append(l, h, this.metaInfo);
    const a = n("div", "fb-header-actions");
    this.settingsBtn = n("button", "fb-icon-btn fb-settings-btn"), this.settingsBtn.type = "button", this.settingsBtn.setAttribute("aria-label", "服务器设置"), this.settingsBtn.title = "服务器设置", this.settingsBtn.append(Kt()), this.settingsBtn.addEventListener("click", () => this.openServerSettings());
    const d = n("button", "fb-close");
    d.type = "button", d.setAttribute("aria-label", "关闭反馈面板"), d.append(F()), d.addEventListener("click", () => this.close()), a.append(this.settingsBtn, d), r.append(o, a);
    const c = n("div", "fb-body");
    this.body = c, this.shotArea = n("div", "fb-shot-area"), this.screenshotWrap = n("div", "fb-screenshot-wrap"), this.screenshotWrap.hidden = !0, this.screenshotThumbBox = n("div", "fb-screenshot-thumb-box"), this.screenshotThumbBox.setAttribute("role", "button"), this.screenshotThumbBox.setAttribute("tabindex", "0"), this.screenshotThumbBox.setAttribute("aria-label", "查看完整截图"), this.screenshotThumb = n("img", "fb-screenshot-thumb"), this.screenshotThumb.alt = "反馈截图缩略图";
    const u = n("span", "fb-screenshot-badge", "当前截图"), b = n("span", "fb-screenshot-zoom-hint", "点击放大");
    this.screenshotThumbBox.append(this.screenshotThumb, u, b), this.screenshotWrap.append(this.screenshotThumbBox);
    const y = n("div", "fb-screenshot-actions");
    this.captureBtn = n("button", "fb-btn-capture", Y), this.captureBtn.type = "button", this.captureBtn.setAttribute("aria-label", "截取当前页面并附加截图"), this.retakeBtn = n("button", "fb-btn-retake", "重新截图"), this.retakeBtn.type = "button", this.retakeBtn.setAttribute("aria-label", "重新捕获屏幕截图"), this.retakeBtn.hidden = !0, this.removeBtn = n("button", "fb-btn-remove", "移除截图"), this.removeBtn.type = "button", this.removeBtn.setAttribute("aria-label", "移除当前截图"), this.removeBtn.hidden = !0, y.append(this.captureBtn, this.retakeBtn, this.removeBtn), this.shotArea.append(this.screenshotWrap, y), this.captureBtn.addEventListener("click", () => this.captureFromPanel()), this.screenshotThumbBox.addEventListener("click", () => this.openZoomModal()), this.screenshotThumbBox.addEventListener("keydown", (w) => {
      (w.key === " " || w.key === "Enter") && (w.preventDefault(), this.openZoomModal());
    }), this.retakeBtn.addEventListener("click", () => {
      this.retakeScreenshot();
    }), this.removeBtn.addEventListener("click", () => this.removeScreenshot());
    const x = n("label", "fb-prompt", "哪里不顺手，或者有什么新想法？");
    x.setAttribute("for", "fb-textarea");
    const m = n("div", "fb-textarea-wrap");
    this.textarea = n("textarea", "fb-textarea"), this.textarea.id = "fb-textarea", this.textarea.placeholder = "刚才哪里不顺手？你希望它怎样改进？", this.textarea.setAttribute("aria-label", "反馈内容"), this.textarea.value = this.draft.text;
    const g = n("div", "fb-counter-row");
    this.counter = n("span", "fb-counter"), g.append(this.counter), m.append(this.textarea, g), this.statusRegion = n("div", "fb-status fb-status-region"), this.statusRegion.setAttribute("role", "status"), this.statusRegion.setAttribute("aria-live", "polite"), this.errorRegion = n("div", "fb-error fb-status-card-wrap"), this.errorRegion.hidden = !0;
    const p = n("footer", "fb-footer");
    this.quotaInfo = n("span", "fb-quota"), this.quotaInfo.hidden = !0, this.quotaInfo.setAttribute("role", "status"), this.quotaBlockedInfo = n("p", "fb-quota-blocked"), this.quotaBlockedInfo.hidden = !0, this.loginPanel = n("div", "fb-login-panel"), this.loginPanel.hidden = !0, this.loginPanel.setAttribute("role", "group"), this.loginPanel.setAttribute("aria-label", "账号登录"), this.loginUsername = n("input", "fb-login-input fb-login-username"), this.loginUsername.type = "text", this.loginUsername.autocomplete = "username", this.loginUsername.placeholder = "用户名", this.loginUsername.setAttribute("aria-label", "用户名"), this.loginPassword = n("input", "fb-login-input fb-login-password"), this.loginPassword.type = "password", this.loginPassword.autocomplete = "current-password", this.loginPassword.placeholder = "密码", this.loginPassword.setAttribute("aria-label", "密码"), this.loginErrorEl = n("p", "fb-login-error"), this.loginErrorEl.hidden = !0, this.loginConfirmBtn = n("button", "fb-btn-secondary fb-login-confirm", "登录并提交"), this.loginConfirmBtn.type = "button", this.loginConfirmBtn.setAttribute("aria-label", "登录并提交反馈"), this.loginCancelBtn = n("button", "fb-btn-secondary fb-login-cancel", "取消"), this.loginCancelBtn.type = "button";
    const V = n("div", "fb-login-actions");
    V.append(this.loginConfirmBtn, this.loginCancelBtn);
    const H = n("div", "fb-login-row");
    H.append(this.loginUsername, this.loginPassword), this.loginPanel.append(H, this.loginErrorEl, V), this.submitBtn = n("button", "fb-submit", "提交"), this.submitBtn.type = "button", this.submitBtn.setAttribute("aria-label", "提交反馈");
    const ht = n("p", "fb-footnote", "会保留原话，整理为候选改进");
    p.append(this.quotaInfo, this.quotaBlockedInfo, this.loginPanel, this.submitBtn, ht), this.loginConfirmBtn.addEventListener("click", () => {
      this.doLogin();
    }), this.loginCancelBtn.addEventListener("click", () => this.cancelLogin()), this.loginPassword.addEventListener("keydown", (w) => {
      w.key === "Enter" && (w.preventDefault(), this.doLogin());
    }), this.logsArea = n("div", "fb-logs-area");
    const $ = n("div", "fb-logs-header"), j = n("span", void 0, "日志附件");
    this.logsCountEl = n("span", "fb-logs-count", " (0/3)"), j.append(this.logsCountEl);
    const Q = n("div", "fb-logs-actions");
    this.addLogBtn = n("button", "fb-btn-add-log", "添加日志"), this.addLogBtn.type = "button", this.addLogBtn.setAttribute("aria-label", "手动添加日志文件"), this.logFileInput = n("input"), this.logFileInput.type = "file", this.logFileInput.accept = ".log,.txt,.json,.jsonl", this.logFileInput.multiple = !0, this.logFileInput.hidden = !0, this.addLogBtn.addEventListener("click", () => {
      this.logFileInput.click();
    }), this.logFileInput.addEventListener("change", () => {
      this.handleLogFileSelect();
    }), Q.append(this.addLogBtn, this.logFileInput), $.append(j, Q), this.logStatusEl = n("span", "fb-log-status"), this.logStatusEl.hidden = !0, this.logErrorEl = n("div", "fb-log-error"), this.logErrorEl.hidden = !0, this.logsList = n("ul", "fb-logs-list"), this.logsArea.append($, this.logStatusEl, this.logErrorEl, this.logsList), c.append(this.shotArea, x, m, this.logsArea, this.statusRegion, this.errorRegion, p), this.panel.append(r, c, this.buildSettingsView()), this.zoomModal = n("div", "fb-zoom-modal"), this.zoomModal.setAttribute("role", "dialog"), this.zoomModal.setAttribute("aria-label", "完整截图预览"), this.zoomImg = n("img"), this.zoomImg.alt = "完整截图预览", this.zoomCloseBtn = n("button", "fb-zoom-close"), this.zoomCloseBtn.type = "button", this.zoomCloseBtn.setAttribute("aria-label", "关闭截图预览"), this.zoomCloseBtn.append(F()), this.zoomModal.append(this.zoomImg, this.zoomCloseBtn), this.zoomModal.addEventListener("click", (w) => {
      (w.target === this.zoomModal || w.target === this.zoomCloseBtn || this.zoomCloseBtn.contains(w.target)) && this.closeZoomModal();
    }), this.logPreviewModal = n("div", "fb-log-preview-modal"), this.logPreviewModal.setAttribute("role", "dialog"), this.logPreviewModal.setAttribute("aria-label", "日志预览");
    const K = n("div", "fb-log-preview-card"), W = n("div", "fb-header");
    this.logPreviewTitle = n("h3", void 0, "日志预览"), this.logPreviewCloseBtn = n("button", "fb-close"), this.logPreviewCloseBtn.type = "button", this.logPreviewCloseBtn.setAttribute("aria-label", "关闭日志预览"), this.logPreviewCloseBtn.append(F()), this.logPreviewCloseBtn.addEventListener("click", () => this.closeLogPreview()), W.append(this.logPreviewTitle, this.logPreviewCloseBtn), this.logPreviewBody = n("pre", "fb-log-preview-body"), K.append(W, this.logPreviewBody), this.logPreviewModal.append(K), this.logPreviewModal.addEventListener("click", (w) => {
      w.target === this.logPreviewModal && this.closeLogPreview();
    }), this.root.append(t, this.launcher, this.orb, this.panel, this.zoomModal, this.logPreviewModal), this.addEventListener("keydown", (w) => this.onKeydown(w));
  }
  /**
   * T6：服务器设置视图（面板内、与主体互斥显示；未登录也可进入）。
   * 提供地址输入、保存、取消、恢复默认与当前有效地址展示；
   * 地址变化且有草稿 / 未确认提交时先展开内联确认块。
   */
  buildSettingsView() {
    this.settingsView = n("div", "fb-settings"), this.settingsView.hidden = !0, this.settingsView.setAttribute("role", "group"), this.settingsView.setAttribute("aria-label", "服务器设置");
    const t = n("h3", "fb-settings-title", "Feedback 服务器"), e = n("p", "fb-settings-line");
    e.append(n("span", "fb-settings-label", "当前使用：")), this.settingsCurrent = n("span", "fb-settings-value"), e.append(this.settingsCurrent), this.settingsDefaultRow = n("p", "fb-settings-line"), this.settingsDefaultRow.hidden = !0, this.settingsDefaultRow.append(n("span", "fb-settings-label", "宿主默认：")), this.settingsDefaultText = n("span", "fb-settings-value"), this.settingsDefaultRow.append(this.settingsDefaultText);
    const s = n("label", "fb-prompt", "服务器地址");
    s.setAttribute("for", "fb-server-input"), this.settingsInput = n("input", "fb-login-input fb-server-input"), this.settingsInput.id = "fb-server-input", this.settingsInput.type = "url", this.settingsInput.placeholder = "https://fb.example.com", this.settingsInput.autocomplete = "off", this.settingsInput.spellcheck = !1, this.settingsInput.setAttribute("aria-label", "服务器地址"), this.settingsInput.addEventListener("keydown", (d) => {
      d.key === "Enter" && !d.isComposing && d.keyCode !== 229 && (d.preventDefault(), this.saveServerSettings());
    }), this.settingsErrorEl = n("p", "fb-settings-error"), this.settingsErrorEl.hidden = !0, this.settingsHintEl = n("p", "fb-settings-hint"), this.settingsHintEl.hidden = !0, this.settingsHintEl.setAttribute("role", "status");
    const r = n("div", "fb-settings-actions");
    this.settingsSaveBtn = n("button", "fb-btn-secondary fb-settings-save", "保存"), this.settingsSaveBtn.type = "button", this.settingsRestoreBtn = n("button", "fb-btn-secondary fb-settings-restore", "恢复默认"), this.settingsRestoreBtn.type = "button", this.settingsCancelBtn = n("button", "fb-btn-secondary fb-settings-cancel", "返回"), this.settingsCancelBtn.type = "button", r.append(this.settingsSaveBtn, this.settingsRestoreBtn, this.settingsCancelBtn), this.settingsConfirmEl = n("div", "fb-settings-confirm"), this.settingsConfirmEl.hidden = !0, this.settingsConfirmText = n("p", "fb-settings-confirm-text");
    const o = n("div", "fb-settings-actions"), l = n("button", "fb-btn-secondary fb-settings-confirm-ok", "清空并切换");
    l.type = "button";
    const h = n("button", "fb-btn-secondary fb-settings-confirm-cancel", "取消");
    h.type = "button", o.append(l, h), this.settingsConfirmEl.append(this.settingsConfirmText, o);
    const a = n("p", "fb-footnote", "支持 http(s) 地址，可包含端口与部署路径前缀；仅本机记住选择。");
    return this.settingsView.append(
      t,
      e,
      this.settingsDefaultRow,
      s,
      this.settingsInput,
      this.settingsErrorEl,
      this.settingsHintEl,
      r,
      this.settingsConfirmEl,
      a
    ), this.settingsSaveBtn.addEventListener("click", () => this.saveServerSettings()), this.settingsRestoreBtn.addEventListener("click", () => this.requestRestoreDefault()), this.settingsCancelBtn.addEventListener("click", () => this.closeServerSettings()), l.addEventListener("click", () => this.confirmServerSwitch()), h.addEventListener("click", () => this.cancelServerSwitch()), this.settingsView;
  }
  // ---------- 日志附件处理 ----------
  async collectLogs() {
    const t = this._logProvider;
    if (!t || this.logsCollecting) return;
    const e = this.identityEpoch, s = this.draft, r = ++this.logOpSeq;
    this.logsCollecting = !0, this.logStatusEl.hidden = !1, this.logStatusEl.textContent = "正在自动采集日志…", this.logErrorEl.hidden = !0, this.logErrorEl.textContent = "", this.syncLogsUi();
    let o = null;
    const l = () => {
      o !== null && this.logTimeout === o && (clearTimeout(o), this.logTimeout = null);
    };
    try {
      const h = new Promise((c, u) => {
        o = setTimeout(() => u(new Error("采集日志超时（超过 3 秒）")), 3e3), this.logTimeout = o;
      }), a = Promise.resolve(t()), d = await Promise.race([a, h]);
      if (l(), !this.isConnected || this.draft !== s || e !== this.identityEpoch || r !== this.logOpSeq || !this.openState || this.phase === "submitting")
        return;
      if (d) {
        const c = Array.isArray(d) ? d : [d];
        for (const u of c) {
          if (!u || !u.filename || !u.blob) continue;
          if (this.draft.logs.length >= E) {
            this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `最多附加 ${E} 个日志文件`;
            break;
          }
          const b = et(u.filename);
          if (!tt.has(b)) {
            this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `日志 ${u.filename} 格式不受支持（仅支持 .log / .txt / .json / .jsonl）`;
            continue;
          }
          if (u.blob.size > J) {
            this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `日志 ${u.filename} 超过 1MiB 限制`;
            continue;
          }
          if (u.blob.size === 0) {
            this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `日志 ${u.filename} 不能为空文件`;
            continue;
          }
          this.draft.logs.push({
            filename: u.filename,
            source: "auto",
            blob: u.blob,
            byteSize: u.blob.size
          }), this.draft.version++;
        }
      }
      if (!this.isConnected || this.draft !== s || e !== this.identityEpoch || r !== this.logOpSeq || !this.openState)
        return;
      this.draft.logsCollected = !0, this.logStatusEl.hidden = !0, this.logStatusEl.textContent = "";
    } catch (h) {
      if (l(), !this.isConnected || this.draft !== s || e !== this.identityEpoch || r !== this.logOpSeq || !this.openState || this.phase === "submitting")
        return;
      const a = h instanceof Error ? h.message : "日志自动采集失败";
      this.logErrorEl.hidden = !1, this.logErrorEl.textContent = a, this.logStatusEl.hidden = !0, this.logStatusEl.textContent = "";
    } finally {
      l(), this.isConnected && this.draft === s && e === this.identityEpoch && r === this.logOpSeq && (this.logsCollecting = !1, this.syncLogsUi(), this.syncUi());
    }
  }
  handleLogFileSelect() {
    if (this.phase === "submitting") return;
    const t = Array.from(this.logFileInput.files || []);
    if (t.length !== 0) {
      this.logErrorEl.hidden = !0, this.logErrorEl.textContent = "";
      for (const e of t) {
        if (this.draft.logs.length >= E) {
          this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `最多附加 ${E} 个日志文件`;
          break;
        }
        const s = et(e.name);
        if (!tt.has(s)) {
          this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `日志 ${e.name} 格式不受支持（仅支持 .log / .txt / .json / .jsonl）`;
          continue;
        }
        if (e.size > J) {
          this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `日志 ${e.name} 超过 1MiB 限制`;
          continue;
        }
        if (e.size === 0) {
          this.logErrorEl.hidden = !1, this.logErrorEl.textContent = `日志 ${e.name} 不能为空文件`;
          continue;
        }
        this.draft.logs.push({
          filename: e.name,
          source: "manual",
          blob: e,
          byteSize: e.size
        }), this.draft.version++;
      }
      this.logFileInput.value = "", this.syncLogsUi(), this.syncUi();
    }
  }
  removeLog(t) {
    t >= 0 && t < this.draft.logs.length && (this.draft.logs.splice(t, 1), this.draft.version++, this.syncLogsUi(), this.syncUi());
  }
  async openLogPreview(t) {
    this.logPreviewTitle.textContent = t.filename, this.logPreviewBody.textContent = "读取中…", this.logPreviewModal.classList.add("is-open");
    try {
      const e = await jt(t.blob);
      this.logPreviewBody.textContent = e;
    } catch {
      this.logPreviewBody.textContent = "读取日志内容失败";
    }
  }
  closeLogPreview() {
    this.logPreviewModal && this.logPreviewModal.classList.remove("is-open");
  }
  syncLogsUi() {
    const t = this.draft.logs;
    this.logsCountEl.textContent = ` (${t.length}/${E})`;
    const e = this.phase === "submitting" || this.polling || this.logsCollecting;
    this.addLogBtn.disabled = e || t.length >= E, this.addLogBtn.hidden = t.length >= E, this.logsList.innerHTML = "";
    for (let s = 0; s < t.length; s++) {
      const r = t[s];
      if (!r) continue;
      const o = n("li", "fb-log-item"), l = n("div", "fb-log-info"), h = n("span", "fb-log-name", r.filename);
      h.title = r.filename;
      const a = n("span", "fb-log-badge", r.source === "auto" ? "自动" : "手动"), d = n("span", "fb-log-size", $t(r.byteSize || r.blob.size));
      l.append(h, a, d);
      const c = n("div", "fb-log-btns"), u = n("button", "fb-log-btn-preview", "预览");
      u.type = "button", u.setAttribute("aria-label", `预览 ${r.filename}`), u.addEventListener("click", () => {
        this.openLogPreview(r);
      });
      const b = n("button", "fb-log-btn-remove", "删除");
      b.type = "button", b.setAttribute("aria-label", `删除 ${r.filename}`), b.disabled = e, b.addEventListener("click", () => {
        this.removeLog(s);
      }), c.append(u, b), o.append(l, c), this.logsList.append(o);
    }
  }
  // ---------- 提交流程与认证 ----------
  tokenValid() {
    return this.accessToken !== null && Date.now() < this.tokenExpiresAt;
  }
  failPhase(t) {
    this.phase = "failed", this.lastErrorSummary = t, this.renderStatus(""), this.syncUi();
  }
  async onPrimaryAction() {
    if (!(this.phase === "submitting" || this.polling)) {
      if (!this.tokenValid()) {
        if (C(this.textarea.value) === 0) {
          this.textarea.focus();
          return;
        }
        this.beginLogin(!0);
        return;
      }
      await this.submit();
    }
  }
  /** 4xx 视为服务端明确处理并拒绝（未保存）；其余（网络错误/5xx/408/429）结果未知。 */
  isUnknownOutcome(t) {
    return t instanceof k ? t.status >= 500 || t.status === 408 || t.status === 429 : !0;
  }
  async submit() {
    if (this.phase === "submitting") return;
    const t = this.identityEpoch, e = this.effectiveApiBase, s = this.appId;
    if (!e || !s) {
      this.failPhase("缺少必填配置：api-base / app-id");
      return;
    }
    const r = this.submitSnapshot, o = r !== null && r.unknownOutcome && r.draftVersion === this.draft.version && r.text === this.textarea.value && r.blob === this.draft.screenshotBlob && z(r.logs, this.draft.logs);
    if (this.quota !== null && this.quota.remaining <= 0 && !o) {
      this.phase = "quota", this.lastErrorSummary = "今日提交次数已用完，请在额度刷新后重试。文字与截图已保留。", this.renderStatus(""), this.syncUi();
      return;
    }
    let l = this.submitSnapshot;
    const h = l !== null && l.draftVersion === this.draft.version && l.text === this.textarea.value && l.blob === this.draft.screenshotBlob && z(l.logs, this.draft.logs) && l.apiBase === e && l.appId === s;
    if (!l || !h) {
      l && l.unknownOutcome && (this.unconfirmedRequest = {
        key: l.key,
        capturedAt: l.captureInfo?.capturedAt ?? null,
        textSummary: l.text.slice(0, 40)
      });
      const c = this.textarea.value, u = C(c);
      if (u < 1 || u > S) {
        this.failPhase(`反馈内容需为 1–${S} 字`);
        return;
      }
      const b = {};
      this.appVersion && (b.appVersion = this.appVersion), this.pageLabel && (b.pageLabel = this.pageLabel), l = {
        key: ct(),
        apiBase: e,
        appId: s,
        context: b,
        text: c,
        blob: this.draft.screenshotBlob,
        captureInfo: this.draft.captureInfo ? { ...this.draft.captureInfo } : null,
        logs: [...this.draft.logs],
        draftVersion: this.draft.version,
        unknownOutcome: !1
      }, this.submitSnapshot = l, this.idempotencyKey = l.key;
    }
    const a = l;
    if (this.lastSubmittedText = a.text, !this.tokenValid()) {
      this.beginLogin(!0);
      return;
    }
    this.invalidateCaptureSession(), this.captureInFlight = null, this.restoreCaptureUi(), this.phase = "submitting", this.lastErrorSummary = null, this.waitingNotice = null, this.renderStatus("提交中…"), this.syncUi();
    const d = this.accessToken;
    try {
      const c = await gt(a.apiBase, this.accessToken, {
        idempotencyKey: a.key,
        appId: a.appId,
        ...this.appName ? { appName: this.appName } : {},
        text: a.text,
        ...Object.keys(a.context).length ? { context: a.context } : {},
        ...a.captureInfo ? { capture: a.captureInfo } : {},
        ...a.blob ? { screenshot: a.blob } : {},
        ...a.logs.length > 0 ? { logs: a.logs } : {}
      });
      if (t !== this.identityEpoch) return;
      this.invalidateQuotaRefresh(), c.quota && (this.quota = c.quota), c.user && (this.authUser = c.user), this.lastFeedbackId = c.feedbackId, this.submitSnapshot = null, this.idempotencyKey = null, this.unconfirmedRequest = null, this.clearDraft(), this.textarea.value = "", this.authRequired = !1, this.lastRecord = {
        id: c.feedbackId,
        status: c.status,
        createdAt: "",
        updatedAt: "",
        errorSummary: null,
        kaneoUrl: null
      }, this.phase = "tracking", this.waitingNotice = B(c.collectionState), this.renderStatus(this.waitingNotice ?? "已保存，正在整理"), this.dispatchEvent(
        new CustomEvent("feedback-submitted", {
          detail: { feedbackId: c.feedbackId, status: c.status, replayed: c.replayed === !0 },
          bubbles: !0,
          composed: !0
        })
      ), this.syncUi(), this.waitingNotice === null && this.startPolling(c.feedbackId), this.consumeQuotaRefreshIntent();
    } catch (c) {
      if (t !== this.identityEpoch) return;
      if (c instanceof k && c.status === 401) {
        if (d !== this.accessToken) {
          this.phase = "idle", this.renderStatus("登录已更新，请重新提交"), this.syncUi();
          return;
        }
        this.accessToken = null, this.tokenExpiresAt = 0, this.authRequired = !0, this.phase = "idle", this.invalidateQuotaRefresh(), this.quotaRefreshPending = !1, this.renderStatus("需要登录（登录已过期）"), this.syncUi();
        return;
      }
      if (this.phase = "failed", c instanceof k && c.code === "daily_quota_exceeded") {
        a.unknownOutcome = !1, this.invalidateQuotaRefresh(), c.quota && (this.quota = c.quota), this.phase = "quota", this.lastErrorSummary = "今日提交次数已用完，请在额度刷新后重试。文字与截图已保留。", this.renderStatus(""), this.syncUi(), this.consumeQuotaRefreshIntent();
        return;
      }
      c instanceof k && c.code === "idempotency_conflict" ? (a.unknownOutcome = !1, this.lastErrorSummary = "提交冲突：同一提交标识已对应不同内容。请修改内容重新提交，或“再记一条”。") : (a.unknownOutcome = this.isUnknownOutcome(c), this.lastErrorSummary = c instanceof Error ? c.message : String(c)), this.renderStatus(""), this.syncUi();
    }
  }
  // ---------- 轮询 ----------
  startPolling(t) {
    this.stopPolling(), this.polling = !0, this.pollDelay = L, this.pollStartedAt = Date.now(), this.syncUi(), this.schedulePoll(t);
  }
  schedulePoll(t) {
    this.polling && (this.pollTimer = setTimeout(() => {
      this.pollTick(t);
    }, this.pollDelay), this.pollDelay = Math.min(this.pollDelay * 2, Dt));
  }
  async pollTick(t) {
    if (!this.polling) return;
    const e = this.identityEpoch, s = this.effectiveApiBase;
    if (!s || !this.tokenValid()) {
      this.polling = !1, this.tokenValid() || (this.authRequired = !0, this.renderStatus("登录已过期，请重新登录后继续查看进度。"), this.syncUi());
      return;
    }
    let r;
    const o = this.accessToken;
    try {
      r = await X(s, o, t);
    } catch (l) {
      if (e !== this.identityEpoch) return;
      if (l instanceof k && l.status === 401) {
        if (o !== this.accessToken) {
          this.schedulePoll(t);
          return;
        }
        this.polling = !1, this.accessToken = null, this.authRequired = !0, this.renderStatus("登录已过期，请重新登录后继续查看进度。"), this.syncUi();
        return;
      }
      if (l instanceof k && l.status === 404) {
        this.polling = !1, this.phase = "failed", this.lastErrorSummary = "未找到反馈记录", this.renderStatus(""), this.syncUi();
        return;
      }
      r = this.lastRecord ?? {
        id: t,
        status: "received",
        createdAt: "",
        updatedAt: "",
        errorSummary: null,
        kaneoUrl: null
      };
    }
    if (e === this.identityEpoch && this.polling) {
      if (Z(r.status)) {
        this.stopPolling(), this.applyRecordState(r);
        return;
      }
      if (this.lastRecord = r, B(r.collectionState) !== null) {
        this.stopPolling(), this.applyRecordState(r);
        return;
      }
      if (Date.now() - this.pollStartedAt + this.pollDelay > _t) {
        this.polling = !1, this.renderStatus("已保存，后台正在整理中，稍后可在管理页查看。"), this.syncUi();
        return;
      }
      this.renderStatus("已保存，正在整理"), this.schedulePoll(t);
    }
  }
  /**
   * 依记录状态更新阶段 / 结果 / 状态文本（轮询与手动刷新共用）。
   * 返回 true 表示已进入终态（调用方应停止轮询）。
   */
  applyRecordState(t) {
    this.lastRecord = t;
    const e = B(t.collectionState);
    return e !== null ? (this.waitingNotice = e, this.phase = "tracking", this.lastErrorSummary = null, this.renderStatus(e), this.syncUi(), !0) : (this.waitingNotice = null, t.status === "archived" ? (this.phase = "archived", this.renderStatus("已归档"), this.syncUi(), !0) : t.status === "failed" ? (this.phase = "failed", this.lastErrorSummary = t.errorSummary ?? null, this.renderStatus(""), this.syncUi(), !0) : t.status === "needs_review" ? (this.phase = "needs_review", this.lastErrorSummary = null, this.renderStatus(""), this.syncUi(), !0) : !1);
  }
  async resumePolling() {
    this.lastFeedbackId && !this.polling && this.tokenValid() && this.startPolling(this.lastFeedbackId);
  }
  stopPolling() {
    this.polling = !1, this.pollTimer !== null && (clearTimeout(this.pollTimer), this.pollTimer = null);
  }
  /**
   * 服务端已接收但后台处理失败时的手动刷新：只读 `GET /api/feedback/:id`，
   * 更新 phase / lastRecord / lastErrorSummary / 状态文本。绝不重复 POST。
   * 与轮询共用身份世代校验：身份切换后的迟到结果一律丢弃。
   */
  async refreshLastRecord() {
    const t = this.lastFeedbackId;
    if (!t || this.refreshing) return;
    const e = this.effectiveApiBase;
    if (!e) {
      this.failPhase("缺少必填配置：api-base / app-id");
      return;
    }
    if (!this.tokenValid()) {
      this.authRequired = !0, this.renderStatus("需要登录（登录已过期）"), this.syncUi();
      return;
    }
    const s = this.identityEpoch, r = this.accessToken;
    this.refreshing = !0, this.renderStatus("正在刷新状态…"), this.syncUi();
    try {
      const o = await X(e, r, t);
      if (s !== this.identityEpoch) return;
      if (Z(o.status)) {
        this.stopPolling(), this.applyRecordState(o);
        return;
      }
      if (B(o.collectionState) !== null) {
        this.stopPolling(), this.applyRecordState(o);
        return;
      }
      this.waitingNotice = null, this.lastRecord = o, this.phase = "tracking", this.renderStatus("已保存，正在整理"), this.syncUi(), this.polling || this.startPolling(t);
    } catch (o) {
      if (s !== this.identityEpoch) return;
      if (o instanceof k && o.status === 401) {
        if (r !== this.accessToken)
          return;
        this.accessToken = null, this.tokenExpiresAt = 0, this.authRequired = !0, this.renderStatus("登录已过期，请重新登录后继续查看进度。"), this.syncUi();
        return;
      }
      if (o instanceof k && o.status === 404) {
        this.lastErrorSummary = "未找到反馈记录", this.renderStatus("未找到反馈记录。"), this.syncUi();
        return;
      }
      this.lastErrorSummary = o instanceof Error ? o.message : String(o), this.renderStatus("刷新失败，请稍后再试。"), this.syncUi();
    } finally {
      this.refreshing = !1, this.syncUi();
    }
  }
  /**
   * 复制反馈标识：剪贴板不可用（或写入失败）时静默降级，绝不抛错、绝不发请求。
   */
  copyFeedbackId() {
    const t = this.lastFeedbackId;
    if (t)
      try {
        const e = typeof navigator > "u" ? void 0 : navigator.clipboard;
        if (!e || typeof e.writeText != "function") return;
        Promise.resolve(e.writeText(t)).then(() => this.renderStatus("反馈标识已复制")).catch(() => {
        });
      } catch {
      }
  }
  // ---------- 服务器设置（T6） ----------
  /** 按当前（appId, api-base）槽位读取本机覆盖；键未变时跳过，保留会话内未落盘的选择。 */
  loadServerOverride() {
    const t = rt(this.appId ?? "", this.apiBase ?? "");
    if (t !== this.serverOverrideKeyLoaded) {
      this.serverOverrideKeyLoaded = t;
      try {
        const e = window.localStorage.getItem(t);
        if (e === null) {
          this.serverOverride = null;
          return;
        }
        const s = A(e);
        this.serverOverride = s.ok ? s.base : null;
      } catch {
        this.serverOverride = null, this.serverPrefWritable = !1;
      }
    }
  }
  /** 两个地址规范化后是否相同（不可规范化的按去空白原值比较）。 */
  sameNormalizedBase(t, e) {
    if (t === null || e === null) return t === e;
    const s = A(t), r = A(e);
    return (s.ok ? s.base : t.trim()) === (r.ok ? r.base : e.trim());
  }
  pageIsHttps() {
    return typeof window < "u" && typeof window.location < "u" && window.location.protocol === "https:";
  }
  openServerSettings() {
    this.settingsOpen = !0, this.settingsConfirmOpen = !1, this.pendingServerTarget = void 0, this.settingsInput.value = this.effectiveApiBase ?? "", this.settingsErrorEl.hidden = !0, this.settingsErrorEl.textContent = "", this.settingsHintEl.hidden = !0, this.settingsHintEl.textContent = "", this.syncUi(), this.settingsInput.focus();
  }
  closeServerSettings() {
    this.settingsOpen = !1, this.settingsConfirmOpen = !1, this.pendingServerTarget = void 0, this.syncUi();
  }
  setSettingsHint(t) {
    this.settingsHintEl.textContent = t, this.settingsHintEl.hidden = t === "";
  }
  /**
   * 写入 / 清除本机覆盖偏好；返回是否落盘成功（失败时调用方提示「仅本次生效」）。
   * 偏好只存地址——不持久化草稿、密码或令牌。
   */
  persistServerOverride(t) {
    const e = vt(this.appId ?? "", this.apiBase ?? "", t);
    return e || (this.serverPrefWritable = !1), e;
  }
  /**
   * 保存设置视图输入：校验 → 与当前有效地址相同仅落盘（不触发身份重置）→
   * 不同且有草稿 / 未确认提交时先展开确认块 → 确认后切换。
   */
  saveServerSettings() {
    const t = A(this.settingsInput.value, { pageIsHttps: this.pageIsHttps() });
    if (!t.ok) {
      this.settingsErrorEl.textContent = t.reason, this.settingsErrorEl.hidden = !1, this.setSettingsHint("");
      return;
    }
    this.settingsErrorEl.hidden = !0, this.settingsErrorEl.textContent = "";
    const e = this.sameNormalizedBase(t.base, this.apiBase) ? null : t.base, s = e ?? this.apiBase;
    if (this.sameNormalizedBase(s, this.effectiveApiBase)) {
      const r = this.persistServerOverride(e);
      this.serverOverride = e, this.setSettingsHint(r ? "已保存。" : "服务器地址仅本次生效（偏好未能保存到本机）。"), this.syncUi();
      return;
    }
    this.requestServerSwitch(e);
  }
  /** 「恢复默认」：与保存共用确认与提交流程，目标覆盖为 null。 */
  requestRestoreDefault() {
    this.sameNormalizedBase(this.apiBase, this.effectiveApiBase) || this.requestServerSwitch(null);
  }
  /** 地址即将变化：有草稿或结果未确认提交时先展开内联确认块，否则直接切换。 */
  requestServerSwitch(t) {
    const e = this.unconfirmedRequest !== null || this.submitSnapshot?.unknownOutcome === !0 || this.phase === "submitting";
    if (this.isDraftDirty() || e || this.polling) {
      this.settingsConfirmOpen = !0, this.pendingServerTarget = t, this.settingsConfirmText.textContent = e ? "切换服务器将清空当前草稿（文字、截图与日志）。已有一条提交的结果尚未确认，旧服务器可能已接收——切换不会撤回，也不会自动向新服务器重发。" : "切换服务器将清空当前草稿（文字、截图与日志）。", this.syncUi();
      return;
    }
    this.commitServerSwitch(t);
  }
  confirmServerSwitch() {
    if (this.pendingServerTarget === void 0) return;
    const t = this.pendingServerTarget;
    this.pendingServerTarget = void 0, this.commitServerSwitch(t);
  }
  cancelServerSwitch() {
    this.settingsConfirmOpen = !1, this.pendingServerTarget = void 0, this.syncUi();
  }
  /**
   * 应用覆盖并切换身份：相同规范化地址不触发重置由调用方保证；
   * 这里落盘偏好 → epoch 递增作废旧服务在途请求 → 完整服务切换重置 → 探测连通性。
   */
  commitServerSwitch(t) {
    const e = this.persistServerOverride(t);
    this.settingsConfirmOpen = !1, this.pendingServerTarget = void 0, this.serverOverride = t, this.identityEpoch++, this.quotaRefreshPending = !1, this.resetForServiceSwitch(), this.settingsOpen = !0, this.setSettingsHint(
      e ? "已保存。" : "服务器地址已切换，但仅本次生效（偏好未能保存到本机）。"
    ), this.syncUi(), this.probeServer(this.effectiveApiBase);
  }
  /**
   * 保存后探测新地址连通性（GET /healthz，不带凭据）：
   * 失败只提示，不回切——连接失败不自动换回其他服务器（T6）。
   */
  async probeServer(t) {
    if (!t) return;
    const e = this.identityEpoch, s = new AbortController(), r = setTimeout(() => s.abort(), 5e3);
    try {
      await fetch(N(t, "/healthz"), {
        method: "GET",
        signal: s.signal,
        credentials: "omit"
      });
    } catch {
      e === this.identityEpoch && this.settingsOpen && this.setSettingsHint("已保存，但暂时无法连接该服务器，请确认地址可用（不会自动切回）。");
    } finally {
      clearTimeout(r);
    }
  }
  /** 设置视图的只读展示同步（当前有效地址 / 宿主默认 / 确认块可见性）。 */
  syncSettingsUi() {
    if (!this.settingsView) return;
    this.settingsView.hidden = !this.settingsOpen, this.body && (this.body.hidden = this.settingsOpen), this.settingsCurrent.textContent = this.effectiveApiBase ?? "—";
    const t = this.apiBase, e = this.serverOverride !== null;
    this.settingsDefaultRow.hidden = !e || t === null, this.settingsDefaultText.textContent = e ? t ?? "" : "", this.settingsRestoreBtn.disabled = !e, this.settingsConfirmEl.hidden = !this.settingsConfirmOpen;
  }
  // ---------- 面板内登录 ----------
  pendingSubmit = !1;
  /** 展开面板内账号密码表单（不打开新窗口）；pendingSubmit=true 时登录成功后只提交一次。 */
  beginLogin(t) {
    const e = this.effectiveApiBase, s = this.appId;
    if (!e || !s) {
      this.failPhase("缺少必填配置：api-base / app-id");
      return;
    }
    this.authRequired = !0, this.pendingSubmit = t, this.loginVisible = !0, this.loginError = "", this.syncUi(), this.loginUsername.focus(), this.renderStatus("需要登录：请输入账号密码");
  }
  /** 取消登录：收起表单并让在途登录失效，草稿与截图保留。 */
  cancelLogin() {
    this.invalidateLogin(), this.loginVisible = !1, this.renderStatus(this.phase === "tracking" ? "已保存，正在整理" : ""), this.syncUi();
  }
  /**
   * 使尚未完成的登录接续失效：递增登录序号，清除密码、登录忙碌状态与
   * 待自动提交标记。**不撤销**已经建立的有效登录态，也不动草稿与截图。
   * 调用点：关闭面板、取消登录、组件卸载、服务身份变化。
   */
  invalidateLogin() {
    this.loginSeq++, this.loginBusy = !1, this.pendingSubmit = !1, this.loginPassword.value = "";
  }
  /**
   * 登录回调是否仍然有效：序号未变（面板自登录发起以来没有被关闭、
   * 也没有被取消 / 卸载 / 换身份）+ 服务身份世代未变 + 组件仍连接。
   */
  loginStillValid(t, e) {
    return t === this.loginSeq && e === this.identityEpoch && this.isConnected;
  }
  async doLogin() {
    if (this.loginBusy) return;
    const t = this.effectiveApiBase, e = this.appId;
    if (!t || !e) {
      this.failPhase("缺少必填配置：api-base / app-id");
      return;
    }
    const s = this.loginUsername.value.trim(), r = this.loginPassword.value;
    if (!s || !r) {
      this.loginError = "请输入用户名和密码", this.syncUi();
      return;
    }
    const o = this.identityEpoch, l = ++this.loginSeq;
    this.loginBusy = !0, this.loginError = "", this.syncUi();
    try {
      const h = await bt(t, { username: s, password: r, clientLabel: xt(e), appId: e });
      if (!this.loginStillValid(l, o)) return;
      this.accessToken = h.token;
      const a = Date.parse(h.expiresAt);
      this.tokenExpiresAt = Number.isFinite(a) ? a : Date.now() + 15 * 6e4, this.authUser = h.user, this.quota = h.quota, this.authRequired = !1, this.loginVisible = !1, this.loginBusy = !1, this.loginError = "", this.quotaRefreshFailed = !1, this.loginPassword.value = "", this.renderStatus("已登录。"), this.syncUi(), this.consumeQuotaRefreshIntent();
      const d = this.pendingSubmit;
      this.pendingSubmit = !1, d && C(this.textarea.value) > 0 ? this.submit() : this.phase === "tracking" && this.resumePolling();
    } catch (h) {
      if (!this.loginStillValid(l, o)) return;
      this.loginBusy = !1, this.loginPassword.value = "", this.loginError = wt(h), this.renderStatus(""), this.syncUi();
    }
  }
  // ---------- 额度刷新与生命周期 ----------
  cancelQuotaTimer() {
    this.quotaTimer !== null && (clearTimeout(this.quotaTimer), this.quotaTimer = null);
  }
  /**
   * 使在途额度查询失效并取消定时查询：关闭 / 卸载 / 退到后台 / 身份变化 /
   * 提交成功 / 收到额度错误时调用。旧查询结果不得再写回额度。
   */
  invalidateQuotaRefresh() {
    this.quotaSeq++, this.cancelQuotaTimer();
  }
  /**
   * 统一调度额度查询（同一时刻最多一个定时器）：
   * - 未登录 / 面板未打开 / 应用不在前台 → 不排程；
   * - 刷新失败 → 30 秒后重试（不对过期 resetAt 立即循环请求）；
   * - 额度用尽 → 每 30 秒一次，并与 resetAt 合并取更早者
   *   （后台调高额度后最多 30 秒即可恢复，无需关闭重开面板）；
   * - 其它状态 → 只在服务端 resetAt 排单次查询，到期只向服务端取最新值。
   */
  scheduleQuotaRefresh() {
    if (this.cancelQuotaTimer(), !this.tokenValid() || !this.openState || typeof document < "u" && document.visibilityState !== "visible" || this.quotaInFlight) return;
    if (this.quotaRefreshFailed) {
      this.quotaTimer = setTimeout(() => {
        this.refreshQuota();
      }, U);
      return;
    }
    const t = this.quota;
    if (!t || typeof t.remaining != "number" || typeof t.resetAt != "string") return;
    const e = Date.parse(t.resetAt), s = Number.isFinite(e) ? e - Date.now() : Number.NaN;
    let r;
    t.remaining <= 0 ? r = s > 0 && s < U ? s : U : s > 0 ? r = s : r = null, !(r === null || r <= 0) && (r = Math.min(r, Vt), this.quotaTimer = setTimeout(() => {
      this.refreshQuota();
    }, r));
  }
  /**
   * 刷新额度与会话信息：打开面板、应用恢复前台、提交完成、额度用尽轮询时调用。
   * 不在本地擅自重置——跨过 resetAt 后由服务端返回新的 used / remaining。
   * 结果除校验身份与令牌外还校验查询序号：提交成功后的旧查询不得把次数加回。
   * 被旧查询 / 登录忙碌挡下时登记「待立即刷新」，结束后立即补发（T1-B）。
   */
  async refreshQuota() {
    const t = this.effectiveApiBase;
    if (!t || !this.tokenValid() || this.loginBusy || this.quotaInFlight) {
      t && this.tokenValid() && this.openState && (this.loginBusy || this.quotaInFlight) && (typeof document > "u" || document.visibilityState === "visible") && (this.quotaRefreshPending = !0);
      return;
    }
    const e = this.identityEpoch, s = this.accessToken, r = ++this.quotaSeq;
    this.quotaInFlight = !0;
    try {
      const o = await pt(t, s);
      if (!this.quotaResultValid(r, e, s)) return;
      this.authUser = o.user, this.quota = o.quota, this.quotaRefreshFailed = !1, this.syncUi();
    } catch (o) {
      if (!this.quotaResultValid(r, e, s)) return;
      if (o instanceof k && o.status === 401) {
        this.accessToken = null, this.tokenExpiresAt = 0, this.authRequired = !0, this.authUser = null, this.quota = null, this.quotaRefreshFailed = !1, this.quotaRefreshPending = !1, this.invalidateQuotaRefresh(), this.renderStatus("登录已过期，请重新登录。"), this.syncUi();
        return;
      }
      this.quotaRefreshFailed = !0;
    } finally {
      this.quotaInFlight = !1, this.consumeQuotaRefreshIntent();
    }
  }
  /**
   * 消费「待立即刷新」意图（T1-B）：优先于定时规则——旧查询结束 / 忙碌结束
   * 后条件仍满足时立即补发一次并清除标记；否则按现有 30 秒 / resetAt 规则
   * 排定时器（不会紧密循环，也不会因一次跳过而永久停摆）。
   */
  consumeQuotaRefreshIntent() {
    if (this.quotaRefreshPending && this.tokenValid() && this.openState && !this.loginBusy && !this.quotaInFlight && (typeof document > "u" || document.visibilityState === "visible")) {
      this.quotaRefreshPending = !1, this.refreshQuota();
      return;
    }
    this.scheduleQuotaRefresh();
  }
  /** 额度查询结果是否仍然有效：序号 + 身份世代 + 令牌三者都要匹配。 */
  quotaResultValid(t, e, s) {
    return t === this.quotaSeq && e === this.identityEpoch && s === this.accessToken;
  }
  /** 公开 API：展开面板内登录表单（保留旧签名的返回值）。 */
  startLogin() {
    return this.beginLogin(!1), "";
  }
  /**
   * 宿主注入会话：与面板内登录等价，但凭据由宿主提供——例如同源管理后台
   * 以自身 Cookie 会话调 `POST /api/auth/handshake` 换取的握手令牌。
   * 令牌同样仅存内存；宿主负责到期前续注（重新调用本方法）或在自身
   * 会话结束时调用 dropSession()。注入后组件自动拉取会话身份与额度。
   * 注入非法参数（空令牌 / 不可解析的过期时刻）按 no-op 处理。
   */
  adoptSession(t) {
    const e = typeof t?.accessToken == "string" ? t.accessToken : "", s = t?.expiresAt, r = typeof s == "number" ? s : Date.parse(typeof s == "string" ? s : "");
    !e || !Number.isFinite(r) || (this.identityEpoch++, this.accessToken = e, this.tokenExpiresAt = r, this.authRequired = !1, this.loginVisible = !1, this.loginBusy = !1, this.loginError = "", this.quotaRefreshFailed = !1, this.renderStatus("已登录。"), this.syncUi(), this.refreshQuota());
  }
  /**
   * 宿主清除注入的会话（如宿主自身退出登录）：丢弃令牌与身份，回到
   * 需要登录态；草稿 / 截图 / 日志保留（与令牌被撤销同语义）。
   */
  dropSession() {
    this.accessToken !== null && (this.identityEpoch++, this.accessToken = null, this.tokenExpiresAt = 0, this.authUser = null, this.quota = null, this.quotaRefreshFailed = !1, this.quotaRefreshPending = !1, this.invalidateQuotaRefresh(), this.authRequired = !0, this.renderStatus("需要登录"), this.syncUi());
  }
  resetToCompose() {
    this.stopPolling(), this.phase = "idle", this.lastFeedbackId = null, this.lastRecord = null, this.lastErrorSummary = null, this.waitingNotice = null, this.idempotencyKey = null, this.submitSnapshot = null, this.unconfirmedRequest = null, this.textarea.value = "", this.clearDraft(), this.renderStatus(""), this.syncUi(), this.textarea.focus();
  }
  // ---------- 渲染与 UI 同步 ----------
  renderStatus(t) {
    this.statusRegion.textContent = t;
  }
  syncUi() {
    const t = C(this.textarea.value);
    this.counter.textContent = `${t}/${S}`, this.counter.classList.toggle("is-over", t > S);
    const e = this.tokenValid(), s = this.phase === "submitting" || this.polling, r = this.submitSnapshot, o = r !== null && r.unknownOutcome && r.draftVersion === this.draft.version && r.text === this.textarea.value && r.blob === this.draft.screenshotBlob && z(r.logs, this.draft.logs), l = e && this.quota !== null && this.quota.remaining <= 0 && !o;
    if (e && this.quota ? (this.quotaInfo.hidden = !1, this.quotaInfo.textContent = `今日剩余 ${this.quota.remaining} 次`) : (this.quotaInfo.hidden = !0, this.quotaInfo.textContent = ""), l && this.quota) {
      const h = new Date(this.quota.resetAt);
      this.quotaBlockedInfo.hidden = !1, this.quotaBlockedInfo.textContent = Number.isFinite(h.getTime()) ? `今日提交次数已用完，下次可提交时间：${h.toLocaleString()}` : "今日提交次数已用完，请在额度刷新后重试";
    } else
      this.quotaBlockedInfo.hidden = !0, this.quotaBlockedInfo.textContent = "";
    if (this.loginPanel.hidden = !this.loginVisible, this.loginUsername.disabled = this.loginBusy, this.loginPassword.disabled = this.loginBusy, this.loginConfirmBtn.disabled = this.loginBusy, this.loginCancelBtn.disabled = !1, this.loginConfirmBtn.textContent = this.loginBusy ? "登录中…" : "登录并提交", this.loginErrorEl.hidden = this.loginError === "", this.loginErrorEl.textContent = this.loginError, this.syncShotUi(), this.syncLogsUi(), this.syncSettingsUi(), this.textarea.disabled = this.phase === "submitting", e ? this.phase === "submitting" ? (this.submitBtn.className = "fb-submit", this.submitBtn.textContent = "提交中…", this.submitBtn.disabled = !0) : this.phase === "archived" ? (this.submitBtn.className = "fb-submit", this.submitBtn.textContent = "已归档", this.submitBtn.disabled = !0) : this.phase === "failed" && this.idempotencyKey !== null && !this.lastRecord ? (this.submitBtn.className = "fb-submit", this.submitBtn.textContent = "重试提交", this.submitBtn.disabled = s || t === 0 || t > S || l) : (this.submitBtn.className = "fb-submit", this.submitBtn.textContent = "提交", this.submitBtn.disabled = s || t === 0 || t > S || this.phase === "tracking" || l) : (this.submitBtn.className = "fb-submit fb-login", this.submitBtn.textContent = "登录并提交", this.submitBtn.disabled = t === 0 || t > S || s, !this.statusRegion.textContent && t > 0 && this.renderStatus("需要登录")), this.errorRegion.textContent = "", this.errorRegion.hidden = !0, this.phase === "archived") {
      this.errorRegion.hidden = !1;
      const h = n("div", "fb-status-card is-success");
      h.append(n("p", void 0, "反馈已归档。感谢你的支持！"));
      const a = n("div", "fb-card-actions");
      if (this.lastRecord?.kaneoUrl) {
        const c = n("a", "fb-task-link", "查看任务");
        c.href = this.lastRecord.kaneoUrl, c.target = "_blank", c.rel = "noopener", c.setAttribute("aria-label", "打开 Kaneo 归档任务"), a.append(c);
      }
      const d = n("button", "fb-btn-secondary", "再记一条");
      d.type = "button", d.addEventListener("click", () => this.resetToCompose()), a.append(d), h.append(a), this.errorRegion.append(h);
    } else if (this.phase === "failed" && this.lastRecord) {
      this.errorRegion.hidden = !1;
      const h = n("div", "fb-status-card is-warn");
      h.append(n("p", void 0, "原话已保存，后台整理未完成，可在管理页处理。请勿重复提交。")), this.lastErrorSummary && h.append(n("p", "fb-header-meta", `详情：${this.lastErrorSummary}`));
      const a = this.lastFeedbackId ?? this.lastRecord.id;
      h.append(
        n("p", "fb-header-meta", `反馈标识：${a}（可在管理页据此找回原话与截图）`)
      );
      const d = n("div", "fb-card-actions"), c = n(
        "button",
        "fb-btn-secondary fb-refresh-btn",
        this.refreshing ? "刷新中…" : "刷新状态"
      );
      c.type = "button", c.setAttribute("aria-label", "刷新反馈处理状态"), c.disabled = this.refreshing, c.addEventListener("click", () => {
        this.refreshLastRecord();
      });
      const u = n("button", "fb-btn-secondary fb-copy-id-btn", "复制标识");
      u.type = "button", u.setAttribute("aria-label", "复制反馈标识"), u.addEventListener("click", () => this.copyFeedbackId());
      const b = n("button", "fb-btn-secondary", "再记一条");
      b.type = "button", b.addEventListener("click", () => this.resetToCompose()), d.append(c, u, b), h.append(d), this.errorRegion.append(h);
    } else if (this.phase === "needs_review") {
      this.errorRegion.hidden = !1;
      const h = n("div", "fb-status-card is-warn");
      h.append(n("p", void 0, "归档结果待确认。原话已保存，将在管理页人工复核。"));
      const a = n("div", "fb-card-actions"), d = n("button", "fb-btn-secondary", "再记一条");
      d.type = "button", d.addEventListener("click", () => this.resetToCompose()), a.append(d), h.append(a), this.errorRegion.append(h);
    } else if (this.phase === "quota") {
      this.errorRegion.hidden = !1;
      const h = n("div", "fb-status-card is-warn");
      if (h.append(n("p", void 0, "今日提交次数已用完。文字与截图已保留，额度刷新后可直接再提交。")), this.quota) {
        const a = new Date(this.quota.resetAt);
        Number.isFinite(a.getTime()) && h.append(n("p", "fb-header-meta", `下次可提交时间：${a.toLocaleString()}`));
      }
      this.errorRegion.append(h);
    } else if (this.phase === "failed" && !this.lastRecord) {
      this.errorRegion.hidden = !1;
      const h = n("div", "fb-status-card is-error");
      if (h.append(n("p", "fb-error-summary", "尚未确认保存，请检查网络后重试。")), this.unconfirmedRequest) {
        const c = this.unconfirmedRequest;
        h.append(
          n(
            "p",
            "fb-header-meta",
            `已保留一条结果未知的原请求（标识 ${c.key.slice(0, 8)}…，截图时间 ${c.capturedAt ?? "未知"}），请先核对其是否已被服务端接收，避免重复提交。`
          )
        );
      }
      this.lastErrorSummary && h.append(n("p", "fb-header-meta", this.lastErrorSummary));
      const a = n("div", "fb-card-actions"), d = n("button", "fb-btn-secondary fb-retry fb-retry-btn", "重试提交");
      d.type = "button", d.addEventListener("click", () => {
        this.submit();
      }), a.append(d), h.append(a), this.errorRegion.append(h);
    } else if (this.phase === "tracking") {
      this.errorRegion.hidden = !1;
      const h = n("div", "fb-status-card is-warn");
      if (this.waitingNotice !== null) {
        h.append(n("p", void 0, this.waitingNotice)), h.append(n("p", "fb-header-meta", "无需重复提交；管理员处理后点「刷新状态」即可查看结果。"));
        const a = n("div", "fb-card-actions"), d = n("button", "fb-btn-secondary fb-refresh-btn", this.refreshing ? "刷新中…" : "刷新状态");
        d.type = "button", d.setAttribute("aria-label", "刷新反馈处理状态"), d.disabled = this.refreshing, d.addEventListener("click", () => {
          this.refreshLastRecord();
        });
        const c = n("button", "fb-btn-secondary", "再记一条");
        c.type = "button", c.addEventListener("click", () => this.resetToCompose()), a.append(d, c), h.append(a);
      } else
        h.append(n("p", void 0, "已保存，正在整理。您可以关闭面板，我们将继续在后台处理。"));
      this.errorRegion.append(h);
    }
  }
  // ---------- 键盘与焦点 ----------
  onKeydown(t) {
    if (this.handledKey !== t) {
      if (this.handledKey = t, (t.metaKey || t.ctrlKey) && t.key === "Enter" && !t.isComposing && t.keyCode !== 229 && !this.submitBtn.disabled) {
        t.preventDefault(), this.onPrimaryAction();
        return;
      }
      if (t.key === "Escape") {
        if (this.logPreviewModal && this.logPreviewModal.classList.contains("is-open")) {
          t.preventDefault(), this.closeLogPreview();
          return;
        }
        if (this.zoomModal && this.zoomModal.classList.contains("is-open")) {
          t.preventDefault(), this.closeZoomModal();
          return;
        }
        if (this.settingsOpen) {
          t.preventDefault(), this.closeServerSettings();
          return;
        }
        if (this.openState && (this.root.activeElement !== null || this.contains(document.activeElement) || this === document.activeElement)) {
          t.preventDefault(), this.close();
          return;
        }
      }
      if (t.key === "Tab" && this.openState && this.isMobile()) {
        const e = this.focusables();
        if (e.length === 0) return;
        const s = this.root.activeElement, r = s ? e.indexOf(s) : -1;
        if (r === -1) {
          t.preventDefault(), e[0]?.focus();
          return;
        }
        t.shiftKey && r === 0 ? (t.preventDefault(), e[e.length - 1]?.focus()) : !t.shiftKey && r === e.length - 1 && (t.preventDefault(), e[0]?.focus());
      }
    }
  }
  focusables() {
    const t = this.panel.querySelectorAll(
      'button:not([disabled]), textarea:not([disabled]), input:not([disabled]), a[href], [tabindex="0"]'
    );
    return Array.from(t).filter((e) => e.closest("[hidden]") === null);
  }
}
const O = "feedback-widget";
function lt(i = O) {
  return typeof customElements < "u" && !customElements.get(i) && customElements.define(i, M), M;
}
lt();
function Wt(i = {}) {
  lt();
  let t = document.querySelector(O);
  return t ? (i.apiBase && (t.apiBase = i.apiBase), i.appId && (t.appId = i.appId), i.appVersion !== void 0 && (t.appVersion = i.appVersion), i.pageLabel !== void 0 && (t.pageLabel = i.pageLabel), i.side && (t.side = i.side), i.theme && (t.theme = i.theme), i.showLauncher !== void 0 && (t.showLauncher = i.showLauncher), i.launcherBottom && (t.launcherBottom = i.launcherBottom), i.launcherMode && (t.launcherMode = i.launcherMode), i.captureMode && (t.captureMode = i.captureMode), i.logProvider !== void 0 && (t.logProvider = i.logProvider)) : (t = document.createElement(O), i.apiBase && (t.apiBase = i.apiBase), i.appId && (t.appId = i.appId), i.appVersion && (t.appVersion = i.appVersion), i.pageLabel && (t.pageLabel = i.pageLabel), i.side && (t.side = i.side), i.theme && (t.theme = i.theme), i.showLauncher !== void 0 && (t.showLauncher = i.showLauncher), i.launcherBottom && (t.launcherBottom = i.launcherBottom), i.launcherMode && (t.launcherMode = i.launcherMode), i.captureMode && (t.captureMode = i.captureMode), i.logProvider !== void 0 && (t.logProvider = i.logProvider), document.body.appendChild(t)), t.open(), t;
}
export {
  O as FEEDBACK_ELEMENT_TAG,
  M as FeedbackWidget,
  lt as defineFeedbackWidget,
  A as normalizeServerBase,
  Wt as openFeedback
};
