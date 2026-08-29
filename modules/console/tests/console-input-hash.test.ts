import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const hashScript = join(repoRoot, "scripts/console-input-hash.sh");

function writeFixture(root: string, relative: string, content: string): void {
  const path = join(root, relative);
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, content);
}

function runHash(root: string, ...args: string[]): string {
  return execFileSync(hashScript, args, {
    encoding: "utf8",
    env: { ...process.env, CONSOLE_INPUT_ROOT: root },
  }).trim();
}

test("console input hash covers workspace inputs and excludes dist output", () => {
  const root = mkdtempSync(join(tmpdir(), "sakura-console-input-"));
  try {
    writeFixture(root, "package.json", "{}\n");
    writeFixture(root, "pnpm-workspace.yaml", "packages: []\n");
    writeFixture(root, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n");
    writeFixture(root, "modules/console/web/src/main.ts", "export const value = 1;\n");
    writeFixture(root, "modules/console/web/index.html", "<!doctype html>\n");
    writeFixture(root, "modules/console/web/uc.html", "<!doctype html>\n");
    writeFixture(root, "modules/console/web/public/runtime.json", "{\"version\":1}\n");
    writeFixture(root, "modules/console/web/scripts/build.mjs", "export const build = 1;\n");
    writeFixture(root, "modules/console/web/vite.config.ts", "export default {};\n");
    writeFixture(root, "modules/console/web/package.json", "{}\n");
    writeFixture(root, "packages/editor/src/index.ts", "export const editor = 1;\n");
    writeFixture(root, "packages/editor/package.json", "{}\n");
    writeFixture(root, "modules/console/web/dist/index.js", "old output\n");

    const initial = runHash(root);
    const lockInitial = runHash(root, "--lock");

    writeFixture(root, "modules/console/web/src/main.ts", "export const value = 2;\n");
    const afterConsoleSource = runHash(root);
    assert.notEqual(afterConsoleSource, initial);

    writeFixture(root, "packages/editor/src/index.ts", "export const editor = 2;\n");
    const afterWorkspaceSource = runHash(root);
    assert.notEqual(afterWorkspaceSource, afterConsoleSource);

    writeFixture(root, "modules/console/web/vite.config.ts", "export default { build: {} };\n");
    const afterViteConfig = runHash(root);
    assert.notEqual(afterViteConfig, afterWorkspaceSource);

    writeFixture(root, "modules/console/web/index.html", "<!doctype html><main>changed</main>\n");
    const afterHtml = runHash(root);
    assert.notEqual(afterHtml, afterViteConfig);

    writeFixture(root, "modules/console/web/public/runtime.json", "{\"version\":2}\n");
    const afterPublic = runHash(root);
    assert.notEqual(afterPublic, afterHtml);

    writeFixture(root, "modules/console/web/scripts/build.mjs", "export const build = 2;\n");
    const afterBuildScript = runHash(root);
    assert.notEqual(afterBuildScript, afterPublic);

    writeFixture(root, "packages/editor/package.json", "{\"version\": 2}\n");
    const afterPackageManifest = runHash(root);
    assert.notEqual(afterPackageManifest, afterBuildScript);

    writeFixture(root, "modules/console/web/dist/index.js", "new output\n");
    assert.equal(runHash(root), afterPackageManifest);

    writeFixture(root, "pnpm-lock.yaml", "lockfileVersion: '9.1'\n");
    assert.notEqual(runHash(root), afterPackageManifest);
    assert.notEqual(runHash(root, "--lock"), lockInitial);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
