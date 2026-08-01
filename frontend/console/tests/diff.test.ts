// 行为面：版本对比行级 diff（src/modules/posts/diff.ts）。
// 机械校验入口：make test → console-test → npm run test → node --test tests/*.test.ts
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { myersDiff } from '../src/modules/posts/diff.ts';

test('diff: empty texts', () => {
  const result = myersDiff('', '');
  assert.deepStrictEqual(result.hunks, []);
  assert.strictEqual(result.unchangedCount, 0);
  assert.strictEqual(result.addedCount, 0);
  assert.strictEqual(result.deletedCount, 0);
});

test('diff: identical texts', () => {
  const before = 'line1\nline2\nline3';
  const after = before;
  const result = myersDiff(before, after);
  assert.strictEqual(result.hunks.length, 1);
  assert.strictEqual(result.hunks[0].type, 'same');
  assert.deepStrictEqual(result.hunks[0].lines, ['line1', 'line2', 'line3']);
  assert.strictEqual(result.unchangedCount, 3);
  assert.strictEqual(result.addedCount, 0);
  assert.strictEqual(result.deletedCount, 0);
});

test('diff: completely different', () => {
  const before = 'a\nb\nc';
  const after = 'x\ny\nz';
  const result = myersDiff(before, after);
  // All rows are different
  const deleted = result.hunks.filter(h => h.type === 'delete');
  const added = result.hunks.filter(h => h.type === 'add');
  assert.strictEqual(deleted[0]?.lines.length, 3);
  assert.strictEqual(added[0]?.lines.length, 3);
  assert.strictEqual(result.deletedCount, 3);
  assert.strictEqual(result.addedCount, 3);
  assert.strictEqual(result.unchangedCount, 0);
});

test('diff: insertions only', () => {
  const before = 'line1\nline3';
  const after = 'line1\nNEW\nline3';
  const result = myersDiff(before, after);
  const same = result.hunks.filter(h => h.type === 'same').flat();
  const add = result.hunks.find(h => h.type === 'add');
  assert.strictEqual(same.length, 2);
  assert.ok(add?.lines.includes('NEW'));
  assert.strictEqual(result.addedCount, 1);
  assert.strictEqual(result.deletedCount, 0);
});

test('diff: deletions only', () => {
  const before = 'line1\nDEL\nline3';
  const after = 'line1\nline3';
  const result = myersDiff(before, after);
  const same = result.hunks.filter(h => h.type === 'same').flat();
  const del = result.hunks.find(h => h.type === 'delete');
  assert.strictEqual(same.length, 2);
  assert.ok(del?.lines.includes('DEL'));
  assert.strictEqual(result.deletedCount, 1);
  assert.strictEqual(result.addedCount, 0);
});

test('diff: interleaved changes', () => {
  const before = 'A\nB\nC\nD\nE';
  const after = 'A\nX\nC\nY\nE';
  const result = myersDiff(before, after);
  // A same, B delete, X add, C same, D delete, Y add, E same
  assert.deepStrictEqual(result.hunks.map(h => h.type), [
    'same', 'delete', 'add', 'same', 'delete', 'add', 'same'
  ]);
  assert.strictEqual(result.unchangedCount, 3); // A, C, E
  assert.strictEqual(result.addedCount, 2);     // X, Y
  assert.strictEqual(result.deletedCount, 2);   // B, D
});

test('diff: performance with large file (5000 lines)', { timeout: 1000 }, () => {
  const largeBefore = Array.from({length: 5000}, (_, i) => `line${i}`);
  const largeAfter = [...largeBefore.slice(0, 4999), 'modified'];
  
  const start = Date.now();
  const result = myersDiff(largeBefore.join('\n'), largeAfter.join('\n'));
  const elapsed = Date.now() - start;
  
  assert.ok(elapsed < 200, `Performance exceeded limit: ${elapsed}ms`);
  assert.strictEqual(result.addedCount, 1);
  assert.strictEqual(result.deletedCount, 1);
  assert.strictEqual(result.unchangedCount, 4999);
});
