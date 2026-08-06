// 行级 diff（LCS 动态规划 + 前后缀裁剪），供 RevisionDiff.vue 渲染版本对比。
// 纯函数零依赖，行为测试见 tests/diff.test.ts（node:test 原生运行）。
// 大输入兜底：裁剪后中段规模超限时退化为整段替换，保证性能上限可控。

export type DiffHunk = {
 type: 'same' | 'add' | 'delete';
 lines: string[];
};

export interface DiffResult {
 hunks: DiffHunk[];
 unchangedCount: number;
 addedCount: number;
 deletedCount: number;
}

type RawOp = { type: DiffHunk['type']; line: string };

// 裁剪后中段 LCS 的 DP 规模上限（约 400 万格），超限退化为整段替换
const MAX_DP_CELLS = 4_000_000;

export function myersDiff(before: string, after: string): DiffResult {
 if (before === '' && after === '') {
 return { hunks: [], unchangedCount: 0, addedCount: 0, deletedCount: 0 };
 }
 const a = before.split('\n');
 const b = after.split('\n');

 // 公共前缀裁剪
 let start = 0;
 while (start < a.length && start < b.length && a[start] === b[start]) start++;
 // 公共后缀裁剪（不越过前缀边界）
 let endA = a.length;
 let endB = b.length;
 while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
 endA--;
 endB--;
 }

 const ops: RawOp[] = [];
 for (let i = 0; i < start; i++) ops.push({ type: 'same', line: a[i] });
 diffMiddle(a.slice(start, endA), b.slice(start, endB), ops);
 for (let i = endA; i < a.length; i++) ops.push({ type: 'same', line: a[i] });

 return groupOps(ops);
}

// 中段 diff：LCS DP 回溯，同一变更块内 delete 先于 add 输出
function diffMiddle(a: string[], b: string[], out: RawOp[]): void {
 const m = a.length;
 const n = b.length;
 if (m === 0) {
 for (const line of b) out.push({ type: 'add', line });
 return;
 }
 if (n === 0) {
 for (const line of a) out.push({ type: 'delete', line });
 return;
 }
 if (m * n > MAX_DP_CELLS) {
 for (const line of a) out.push({ type: 'delete', line });
 for (const line of b) out.push({ type: 'add', line });
 return;
 }

 // dp[i][j] = a[i:] 与 b[j:] 的 LCS 长度（一维扁平存储）
 const w = n + 1;
 const dp = new Uint32Array((m + 1) * w);
 for (let i = m - 1; i >= 0; i--) {
 for (let j = n - 1; j >= 0; j--) {
 dp[i * w + j] =
 a[i] === b[j]
 ? dp[(i + 1) * w + j + 1] + 1
 : Math.max(dp[(i + 1) * w + j], dp[i * w + j + 1]);
 }
 }

 let i = 0;
 let j = 0;
 while (i < m && j < n) {
 if (a[i] === b[j]) {
 out.push({ type: 'same', line: a[i] });
 i++;
 j++;
 } else if (dp[(i + 1) * w + j] >= dp[i * w + j + 1]) {
 out.push({ type: 'delete', line: a[i] });
 i++;
 } else {
 out.push({ type: 'add', line: b[j] });
 j++;
 }
 }
 while (i < m) out.push({ type: 'delete', line: a[i++] });
 while (j < n) out.push({ type: 'add', line: b[j++] });
}

// 连续同类型行合并为一个 hunk，并统计三类行数
function groupOps(ops: RawOp[]): DiffResult {
 const hunks: DiffHunk[] = [];
 let unchangedCount = 0;
 let addedCount = 0;
 let deletedCount = 0;

 for (const op of ops) {
 if (op.type === 'same') unchangedCount++;
 else if (op.type === 'add') addedCount++;
 else deletedCount++;

 const last = hunks[hunks.length - 1];
 if (last && last.type === op.type) {
 last.lines.push(op.line);
 } else {
 hunks.push({ type: op.type, lines: [op.line] });
 }
 }
 return { hunks, unchangedCount, addedCount, deletedCount };
}
