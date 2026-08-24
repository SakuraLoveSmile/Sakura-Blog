import { gzip } from "node:zlib";
import { promisify } from "node:util";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

const gzipAsync = promisify(gzip);
const distRoot = resolve(process.argv[2] ?? "dist");

async function filesUnder(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) files.push(...(await filesUnder(path)));
    else if (!entry.name.endsWith(".gz")) files.push(path);
  }
  return files;
}

const files = await filesUnder(distRoot);
for (const file of files) {
  const compressed = await gzipAsync(await readFile(file), { level: 9, mtime: 0 });
  await writeFile(`${file}.gz`, compressed);
}

console.log(`compress-assets: generated ${files.length} gzip assets`);
