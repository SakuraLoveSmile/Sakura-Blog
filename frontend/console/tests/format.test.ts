// 行为面：统一格式化工具（src/lib/format.ts）——相对时间/绝对时间/字节边界。
// 机械校验入口：make test → console-test → node --test tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { formatRelative, formatDateTime, formatDate, formatBytes } from "../src/lib/format.ts";

// 固定 now：2026-08-13 12:00 本地时间（与测试机时区无关，用本地构造）
const NOW = new Date(2026, 7, 13, 12, 0, 0); // 2026-08-13 12:00
const at = (d: Date, h: number, m: number, s = 0) => new Date(d.getFullYear(), d.getMonth(), d.getDate(), h, m, s);

test("formatRelative: 刚刚/分钟前/小时前（今天内）", () => {
  assert.equal(formatRelative(at(NOW, 11, 59, 50).toISOString(), NOW), "刚刚");
  assert.equal(formatRelative(at(NOW, 11, 58).toISOString(), NOW), "2 分钟前");
  assert.equal(formatRelative(at(NOW, 10, 0).toISOString(), NOW), "2 小时前");
});

test("formatRelative: 昨天 HH:MM", () => {
  const yesterday = new Date(NOW.getFullYear(), NOW.getMonth(), NOW.getDate() - 1, 23, 5);
  assert.equal(formatRelative(yesterday.toISOString(), NOW), "昨天 23:05");
});

test("formatRelative: 30 天内 X 天前 / 更早绝对时间", () => {
  const fiveDaysAgo = new Date(NOW.getTime() - 5 * 24 * 3600 * 1000);
  assert.equal(formatRelative(fiveDaysAgo.toISOString(), NOW), "5 天前");
  // 31 天前 → 绝对时间（跨月边界：2026-07-13）
  const monthAgo = new Date(NOW.getTime() - 31 * 24 * 3600 * 1000);
  const got = formatRelative(monthAgo.toISOString(), NOW);
  assert.match(got, /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/, `absolute fallback: ${got}`);
});

test("formatRelative: 非法输入回退原串截断；空串返回空", () => {
  assert.equal(formatRelative(""), "");
  assert.equal(formatRelative("not-a-date", NOW), "not-a-date");
  assert.equal(formatRelative("2026-08-13T10:30:00+08:00", NOW).length > 0, true);
});

test("formatDateTime / formatDate: 规范输出与非法回退", () => {
  // 本地时区往返：构造本地 08:05 → ISO → 解析回本地 → 输出与构造一致
  const iso = new Date(2026, 7, 13, 8, 5).toISOString();
  assert.equal(formatDateTime(iso), "2026-08-13 08:05");
  assert.equal(formatDateTime("garbage"), "garbage");
  assert.equal(formatDate(iso), "2026-08-13");
  assert.equal(formatDate("garbage"), "garbage");
});

test("formatBytes: 1024 进位与边界", () => {
  assert.equal(formatBytes(0), "0 B");
  assert.equal(formatBytes(1023), "1023 B");
  assert.equal(formatBytes(1024), "1.0 KB");
  assert.equal(formatBytes(1536), "1.5 KB");
  assert.equal(formatBytes(1048576), "1.0 MB");
  assert.equal(formatBytes(1073741824), "1.0 GB");
  assert.equal(formatBytes(-5), "0 B");
  assert.equal(formatBytes(NaN), "0 B");
});
