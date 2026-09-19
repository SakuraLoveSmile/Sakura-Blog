import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { resolveSafeRedirect } from "../web/src/utils/redirect.ts";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

function read(relativePath: string): string {
  return readFileSync(join(repoRoot, relativePath), "utf8");
}

test("resolveSafeRedirect accepts valid Console internal paths", () => {
  assert.equal(resolveSafeRedirect("/dashboard"), "/dashboard");
  assert.equal(resolveSafeRedirect("/posts"), "/posts");
  assert.equal(resolveSafeRedirect("/settings?tab=seo"), "/settings?tab=seo");
  assert.equal(resolveSafeRedirect("/appearance?tab=advanced#header"), "/appearance?tab=advanced#header");
});

test("resolveSafeRedirect falls back to fromQuery when redirect is empty", () => {
  assert.equal(resolveSafeRedirect("", "/settings?tab=seo"), "/settings?tab=seo");
  assert.equal(resolveSafeRedirect(undefined, "/posts"), "/posts");
  assert.equal(resolveSafeRedirect(null, "/profile"), "/profile");
});

test("resolveSafeRedirect prioritizes redirect over from", () => {
  assert.equal(resolveSafeRedirect("/settings", "/posts"), "/settings");
});

test("resolveSafeRedirect blocks protocol-relative external URLs", () => {
  assert.equal(resolveSafeRedirect("//evil.com"), "/dashboard");
  assert.equal(resolveSafeRedirect("//attacker.org/phish"), "/dashboard");
  assert.equal(resolveSafeRedirect("", "//evil.com"), "/dashboard");
});

test("resolveSafeRedirect blocks external absolute URLs and javascript protocols", () => {
  assert.equal(resolveSafeRedirect("https://evil.com"), "/dashboard");
  assert.equal(resolveSafeRedirect("http://evil.com/dashboard"), "/dashboard");
  assert.equal(resolveSafeRedirect("javascript:alert(1)"), "/dashboard");
  assert.equal(resolveSafeRedirect("data:text/html,<script>"), "/dashboard");
});

test("resolveSafeRedirect prevents login redirect loop", () => {
  assert.equal(resolveSafeRedirect("/login"), "/dashboard");
  assert.equal(resolveSafeRedirect("/login/"), "/dashboard");
  assert.equal(resolveSafeRedirect("/login?redirect=/settings"), "/dashboard");
  assert.equal(resolveSafeRedirect("", "/login"), "/dashboard");
});

test("resolveSafeRedirect blocks control characters and newlines", () => {
  assert.equal(resolveSafeRedirect("/posts\r\nSet-Cookie: bad"), "/dashboard");
  assert.equal(resolveSafeRedirect("/posts\tinjection"), "/dashboard");
});

test("resolveSafeRedirect handles empty or non-string inputs safely", () => {
  assert.equal(resolveSafeRedirect(undefined, undefined), "/dashboard");
  assert.equal(resolveSafeRedirect(null, null), "/dashboard");
  assert.equal(resolveSafeRedirect(123 as any, {} as any), "/dashboard");
  assert.equal(resolveSafeRedirect("   ", "   "), "/dashboard");
  assert.equal(resolveSafeRedirect(undefined, undefined, "/custom-fallback"), "/custom-fallback");
});

test("Login.vue removes rememberMe and social login buttons", () => {
  const loginCode = read("modules/console/web/src/modules/Login.vue");
  assert.doesNotMatch(loginCode, /rememberMe/);
  assert.doesNotMatch(loginCode, /保持登录/);
  assert.doesNotMatch(loginCode, /handleSocialLogin/);
  assert.doesNotMatch(loginCode, /funnyMessages/);
  assert.doesNotMatch(loginCode, /bubble-tip-global/);
});

test("router/index.ts passes redirect parameter for unauthenticated routes", () => {
  const routerCode = read("modules/console/web/src/router/index.ts");
  assert.match(routerCode, /query:\s*\{\s*redirect:\s*to\.fullPath\s*\}/);
  assert.doesNotMatch(routerCode, /query:\s*\{\s*from:\s*to\.fullPath\s*\}/);
});
