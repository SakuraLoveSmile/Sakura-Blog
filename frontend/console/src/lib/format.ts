/**
 * Spec 62：日期/相对时间/字节格式化统一实现。
 * 原 7-10 处组件私有函数、4 套互相矛盾的相对时间行为收敛到此单一实现，
 * 边界语义（供单测锁定）：
 *  - 相对时间：今天内「刚刚 / X 分钟前 / X 小时前」→ 昨天「昨天 HH:MM」→
 *    ≤30 天「X 天前」→ 更早绝对时间「YYYY-MM-DD HH:MM」；非法输入回退原串截断。
 *  - 字节：1024 进位，B/KB/MB/GB/TB，≥100 取整、其余保留 1 位小数。
 */

const pad = (n: number) => String(n).padStart(2, "0");

const DAY_MS = 24 * 60 * 60 * 1000;

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function hm(d: Date): string {
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** 相对时间。now 可注入便于单测锁定边界。 */
export function formatRelative(dateStr: string, now: Date = new Date()): string {
  if (!dateStr) return "";
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr.slice(0, 16).replace("T", " ");

  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterdayStart = new Date(todayStart.getTime() - DAY_MS);

  if (sameDay(d, todayStart)) {
    const diffMin = Math.floor((now.getTime() - d.getTime()) / 60000);
    if (diffMin < 1) return "刚刚";
    if (diffMin < 60) return `${diffMin} 分钟前`;
    return `${Math.floor(diffMin / 60)} 小时前`;
  }
  if (sameDay(d, yesterdayStart)) {
    return `昨天 ${hm(d)}`;
  }
  // 自然日差（目标日零点 → 今日零点）：今天/昨天已分流，此处 ≥2 天
  const targetStart = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const diffDay = Math.round((todayStart.getTime() - targetStart.getTime()) / DAY_MS);
  if (diffDay <= 30) return `${diffDay} 天前`;
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${hm(d)}`;
}

/** 绝对时间「YYYY-MM-DD HH:MM」；非法输入回退原串截断。 */
export function formatDateTime(s: string): string {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s.slice(0, 16).replace("T", " ");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${hm(d)}`;
}

/** 绝对日期「YYYY-MM-DD」。 */
export function formatDate(s: string): string {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s.slice(0, 10);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** 字节格式化（1024 进位）：B / KB / MB / GB / TB。 */
export function formatBytes(bytes: number): string {
  if (!isFinite(bytes) || bytes < 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  const text = i === 0 || v >= 100 ? String(Math.round(v)) : v.toFixed(1);
  return `${text} ${units[i]}`;
}
