import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { ref, effectScope } from "../web/node_modules/vue/index.js";
import { wikiProjectDisplayName } from "../web/src/modules/wiki/wiki-project-name.ts";
import { useStellarForm } from "../web/src/components/editor/useStellarForm.ts";
import { wikiFields } from "../web/src/components/editor/stellar-form-fields.ts";

test("strict Wiki collection labels use name while legacy name/title choices stay untouched", () => {
  const project = { stellar_version: 2, id: "internal-id", name: "集合显示名", title: "旧标题" };
  assert.equal(wikiProjectDisplayName(project), "集合显示名");
  assert.equal(wikiProjectDisplayName({ ...project, name: "" }), "internal-id");
  assert.equal(wikiProjectDisplayName({ ...project, stellar_version: 0 }), "旧标题");
  assert.deepEqual(project, { stellar_version: 2, id: "internal-id", name: "集合显示名", title: "旧标题" });
});

test("Wiki name edits and reopen preserve original JSON tokens and never use the internal ID as a name", () => {
  const scope = effectScope();
  try {
    scope.run(() => {
      const raw =
        ' { "name" : "集合显示名", "headline":"独立 Hero 标题", "route":{"homepage":"intro"}, "visibility":{"listed":false}, "hero":{"background":{"effect":{"type":"strands","options":{"seed":9007199254740993,"flag":false,"empty":null}}}} } ';
      const blog = ' {"content":{"collection_order":0,"wiki_external_url":"https://example.test/wiki"}} ';
      const form = ref<any>({ stellar_version: 2, id: "internal-id", name: "旧列", stellar_config: raw, blog_extensions: blog, hero: {} });
      const error = useStellarForm(() => form.value, wikiFields);
      assert.equal(form.value.name, "集合显示名");
      assert.equal(form.value.stellar_config, raw);
      form.value.name = "新的集合显示名";
      const savedRaw = raw.replace('"集合显示名"', '"新的集合显示名"');
      assert.equal(form.value.stellar_config, savedRaw);
      assert.equal(form.value.blog_extensions, blog);
      assert.equal(form.value.id, "internal-id");
      assert.equal(error.value, "");
      form.value = { stellar_version: 2, id: "internal-id", name: "过期列", stellar_config: savedRaw, blog_extensions: blog, hero: {} };
      assert.equal(form.value.name, "新的集合显示名");
      assert.equal(form.value.hero.title, "独立 Hero 标题");
      assert.equal(form.value.stellar_config, savedRaw);
      assert.equal(form.value.blog_extensions, blog);
      form.value.stellar_config = '{"headline":"Hero 仍独立"}';
      assert.equal(form.value.name, "");
      assert.equal(form.value.stellar_config, '{"headline":"Hero 仍独立"}');
    });
  } finally {
    scope.stop();
  }
});

test("Wiki editor exposes exactly one collection name control and keeps its API title projection", () => {
  const view = readFileSync(new URL("../web/src/modules/wiki/WikiProjectsView.vue", import.meta.url), "utf8");
  assert.equal(view.match(/v-model="form.name"/g)?.length, 1);
  assert.match(view, /集合显示名（name）/);
  assert.doesNotMatch(view, /form\.title|title: p\.title|project\.title \|\|/);
  assert.match(view, /name: f\.name\.trim\(\),\s+title: f\.name\.trim\(\)/);
  assert.match(view, /:disabled="isEditing"/);
});
