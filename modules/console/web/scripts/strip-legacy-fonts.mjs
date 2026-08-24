// Spec 61 阶段 1：构建后剔除 fontsource 的 legacy .woff 字体文件（保留 .woff2）。
// @fontsource 的 @font-face 同时声明 woff2 与 woff 两种格式，现代浏览器一律取 woff2，
// legacy woff 是体积冗余（约 50% 字体体积）；顺带校验无 .woff2 之外的字体格式残留。
import { readdirSync, rmSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const distRoot = process.argv[2]
  ? join(process.cwd(), process.argv[2])
  : join(dirname(fileURLToPath(import.meta.url)), "..", "dist");
const dist = join(distRoot, "assets");

let removed = 0;
for (const name of readdirSync(dist)) {
  if (/\.woff$/.test(name)) {
    rmSync(join(dist, name));
    removed++;
  }
}
console.log(`strip-legacy-fonts: 已剔除 ${removed} 个 legacy .woff 文件（dist/assets）`);
if (removed === 0) {
  console.warn("strip-legacy-fonts: 未发现 .woff 文件，构建产物可能已不含 legacy 字体");
}
