import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web");
const read = (path) => readFileSync(join(ROOT, path), "utf8");

test("data cache runtime exposes service TTLs and bounded dc namespace eviction", () => {
  const base = read("templates/layout/base.html");
  assert.match(base, /data_cache: \{[^\n]*ttl: \{\{ jsonify \.Site\.DataCache\.TTL \}\}/);
  assert.match(base, /max_entries: \{\{ \.Site\.DataCache\.EffectiveMaxEntries \}\}/);
  assert.match(base, /_dcTtl: \(service\)/);
  assert.match(base, /if \(dc\.enable === false\) return 0/);
  assert.match(base, /key\.startsWith\('dc:'\)/);
  assert.match(base, /entries\.sort\(\(a, b\) => a\.t - b\.t\)/);
  assert.match(base, /request: \(el, url, callback, onFailure, options\)/);
  assert.match(base, /const service = options && options\.service/);
});

test("dynamic services pass their configured cache service names", () => {
  for (const [file, service] of [
    ["assets/js/services/sites.js", "sites"],
    ["assets/js/services/friends.js", "friends"],
    ["assets/js/services/friends_and_posts.js", "friends_and_posts"],
    ["assets/js/services/siteinfo.js", "siteinfo"],
  ]) {
    assert.match(read(file), new RegExp(`service: '${service}'`));
  }
});
