/**
 * 内部安全重定向解析工具
 * 规范：
 * 1. 优先读取 redirect 参数，兼容读取旧 from 参数；
 * 2. 仅允许 Console 内部绝对路径（以单一 / 开头，拒绝 // 协议相对地址与 :// 绝对 URL）；
 * 3. 拦截任何 /login 路径，防止登录重定向死循环；
 * 4. 遇到非法字符、外部地址或空值时，一律回退至 /dashboard。
 */
export function resolveSafeRedirect(
  redirectQuery?: unknown,
  fromQuery?: unknown,
  defaultPath: string = "/dashboard",
): string {
  const candidate =
    typeof redirectQuery === "string" && redirectQuery.trim() !== ""
      ? redirectQuery.trim()
      : typeof fromQuery === "string" && fromQuery.trim() !== ""
        ? fromQuery.trim()
        : "";

  if (!candidate) {
    return defaultPath;
  }

  // 必须以单个 / 开头，且不能以 // 开头（防止协议相对地址如 //evil.com）
  if (!candidate.startsWith("/") || candidate.startsWith("//")) {
    return defaultPath;
  }

  // 不能包含协议标识（如 http://, javascript:, data: 等）
  if (candidate.includes("://")) {
    return defaultPath;
  }

  // 不能包含控制字符、换行或回车（防止 CRLF 注入或解析歧义）
  if (/[\r\n\t\0]/.test(candidate)) {
    return defaultPath;
  }

  // 防止重定向死循环：若目标为 /login 本身或带有登录路径，回退到仪表盘
  const pathOnly = candidate.split("?")[0].split("#")[0];
  if (pathOnly === "/login" || pathOnly === "/login/") {
    return defaultPath;
  }

  return candidate;
}
