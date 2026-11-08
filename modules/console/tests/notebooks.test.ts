import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { ref, effectScope } from "../web/node_modules/vue/index.js";
import { notebookApi } from "../../../packages/api-client/src/api/notebooks.ts";
import { http } from "../../../packages/api-client/src/transport.ts";
import {
  newNotebookForm,
  newNoteForm,
  notebookFormFrom,
  notebookPayload,
  notePayload,
  validateNoteSlug,
  validateNotebookID,
  isMigrationError,
  notebookRoute,
  notebookCountsLabel,
  noteStatusLabel,
  formatNoteTime,
  failureToast,
} from "../web/src/modules/notebooks/notebook-form.ts";
import { useStellarForm } from "../web/src/components/editor/useStellarForm.ts";
import { notebookFields, noteFields } from "../web/src/components/editor/stellar-form-fields.ts";

test("new note has explicit notebook membership and starts without an implicit publication", () => {
  const form = newNoteForm("工作本");
  assert.deepEqual(JSON.parse(form.config).collection, { profile: "notebook", id: "工作本" });
  form.slug = "工作/记录";
  form.tags = "工作/记录\n随笔";
  form.content = "{% note title:原文 %}\n正文\n{% endnote %}";
  const payload = notePayload("工作本", form);
  assert.equal(payload.content_md, form.content);
  assert.deepEqual(payload.tags, ["工作/记录", "随笔"]);
  assert.equal(payload.status, undefined);
  assert.equal(payload.stellar_version, 2);
});

test("notebook and note save preserve original JSON text including large numbers and explicit empties", () => {
  const raw =
    ' {"name":"工作","leftbar":{"widgets":[{"custom":{"large":900719925474099312345,"zero":0,"flag":false,"nil":null}}]}} ';
  const blog = ' {"empty":"","large":900719925474099312345,"flag":false,"nil":null} ';
  const notebook = newNotebookForm();
  notebook.id = "work";
  notebook.config = raw;
  notebook.blog = blog;
  const payload = notebookPayload(notebook);
  assert.equal(payload.stellar_config, raw);
  assert.equal(payload.blog_extensions, blog);
  const note = newNoteForm("work");
  note.slug = "first";
  note.config = raw;
  note.blog = blog;
  assert.equal(notePayload("work", note).stellar_config, raw);
  assert.equal(notePayload("work", note).blog_extensions, blog);
  for (const invalid of ["null", "[]", "broken", "1"]) {
    note.config = invalid;
    assert.throws(() => notePayload("work", note));
  }
});

test("invalid source identities never reach saving, while unicode and nested note paths remain usable", () => {
  for (const id of ["", ".", "..", "a/b", "a\\b", "a\u0000b"]) assert.throws(() => validateNotebookID(id));
  for (const slug of ["", ".", "..", "a//b", "a/../b", "/absolute", "a\\b"])
    assert.throws(() => validateNoteSlug(slug));
  assert.equal(validateNotebookID(" 工作笔记 "), "工作笔记");
  assert.equal(validateNoteSlug(" 随笔/记录 "), "随笔/记录");
  assert.equal(notebookRoute("a#b"), "/notebooks/a%23b/notes");
});

test("migration errors are distinguished from notebook in-use and duplicate identity conflicts", () => {
  assert.equal(
    isMigrationError({ response: { status: 409, data: { error: "stellar migration required: notebooks/work" } } }),
    true,
  );
  assert.equal(isMigrationError({ response: { status: 409, data: { type: "stellar_migration_required" } } }), true);
  assert.equal(isMigrationError({ response: { status: 409, data: { error: "notebook still has notes" } } }), false);
  assert.equal(isMigrationError({ response: { status: 403, data: { error: "只能管理自己的笔记" } } }), false);
});

test("note save, publish, withdrawal and deletion use distinct backend operations and raw metadata", async () => {
  const original = http.defaults.adapter;
  const calls: Array<{ method?: string; url?: string; data?: unknown; params?: unknown }> = [];
  http.defaults.adapter = async (config) => {
    calls.push({
      method: config.method,
      url: config.url,
      data: config.data ? JSON.parse(config.data) : undefined,
      params: config.params,
    });
    return { config, status: 200, statusText: "OK", headers: {}, data: {} };
  };
  try {
    const form = newNoteForm("a#b");
    form.slug = "记录";
    form.config = ' {"title":"正文","listing":{"order":9007199254740993}} ';
    const payload = notePayload("a#b", form);
    await notebookApi.updateNote("a#b", 42, payload);
    await notebookApi.publishNote("a#b", 42, payload);
    await notebookApi.saveNoteDraft("a#b", 42, payload);
    await notebookApi.deleteNote("a#b", 42);
    await notebookApi.listNotes("a#b", { status: "draft", q: "记录", tag: "工作/记录", page: 2, size: 20 });
    assert.deepEqual(
      calls.slice(0, 3).map((call) => [call.method, call.url]),
      [
        ["put", "/notebooks/a%23b/notes/42"],
        ["post", "/notebooks/a%23b/notes/42/publish"],
        ["put", "/notebooks/a%23b/notes/42/draft"],
      ],
    );
    for (const call of calls.slice(0, 3)) assert.deepEqual(call.data, payload);
    assert.equal(calls[3]?.method, "delete");
    assert.deepEqual(calls[4]?.params, { status: "draft", q: "记录", tag: "工作/记录", page: 2, size: 20 });
  } finally {
    http.defaults.adapter = original;
  }
});

test("notebook basic fields patch only the touched JSON nodes and keep every other original token", () => {
  const scope = effectScope();
  try {
    scope.run(() => {
      const raw =
        ' {"name":"工作","description":"旧描述","listing":{"order":2},"custom":{"large":900719925474099312345,"nil":null}} ';
      const form = ref<any>(
        notebookFormFrom({
          id: "work",
          stellar_config: raw,
          blog_extensions: "{}",
          name: "过期列",
          description: "旧描述",
          icon: "",
          cover: "",
          sort: 0,
        }),
      );
      const error = useStellarForm(() => form.value, notebookFields, "config", "blog");
      assert.equal(form.value.name, "工作");
      assert.equal(form.value.sort, 2);
      assert.equal(form.value.config, raw);

      form.value.name = "工作笔记";
      assert.equal(form.value.config, raw.replace('"工作"', '"工作笔记"'));
      form.value.sort = 5;
      assert.equal(form.value.config, raw.replace('"工作"', '"工作笔记"').replace('"order":2', '"order":5'));
      // 清空或输入非数字时移除排序字段，而不是把空串写进上游配置。
      for (const cleared of ["", "-", "abc"]) {
        form.value.sort = 7;
        form.value.sort = cleared;
        assert.equal(JSON.parse(form.value.config).listing.order, undefined, `sort ${JSON.stringify(cleared)}`);
      }
      assert.match(form.value.config, /"large":900719925474099312345,"nil":null/);
      assert.equal(form.value.id, "work");
      assert.equal(error.value, "");

      // 重新载入一份没有 listing.order 的配置：排序回到 0，名称按原文回填。
      form.value = notebookFormFrom({
        id: "work",
        stellar_config: '{"name":"重新载入"}',
        blog_extensions: "{}",
        name: "过期列",
        description: "",
        icon: "",
        cover: "",
        sort: 9,
      });
      assert.equal(form.value.name, "重新载入");
      assert.equal(form.value.sort, 0);
      assert.equal(form.value.config, '{"name":"重新载入"}');
    });
  } finally {
    scope.stop();
  }
});

test("note title and tags sync with the front matter without disturbing collection membership", () => {
  const scope = effectScope();
  try {
    scope.run(() => {
      const raw = '{"title":"旧标题","collection":{"profile":"notebook","id":"work"},"tags":["a/b"]}';
      const form = ref<any>({ slug: "first", content: "", tags: "", title: "", config: raw, blog: "{}" });
      const error = useStellarForm(() => form.value, noteFields, "config", "blog");
      assert.equal(form.value.title, "旧标题");
      assert.equal(form.value.tags, "a/b");
      assert.equal(form.value.config, raw);

      // 标题留空表示“用 Slug 代替”：移除 title，而不是写入空串。
      form.value.title = "";
      let config = JSON.parse(form.value.config);
      assert.equal(config.title, undefined);
      assert.deepEqual(config.collection, { profile: "notebook", id: "work" });
      form.value.title = "新标题";
      config = JSON.parse(form.value.config);
      assert.equal(config.title, "新标题");
      assert.deepEqual(config.collection, { profile: "notebook", id: "work" });
      form.value.tags = "工作/记录\n随笔\n";
      assert.deepEqual(JSON.parse(form.value.config).tags, ["工作/记录", "随笔"]);
      assert.equal(error.value, "");

      // 新建笔记的初始 Front matter 带默认标题；用户改标题后集合归属仍在。
      const fresh = ref<any>(newNoteForm("work"));
      useStellarForm(() => fresh.value, noteFields, "config", "blog");
      assert.equal(fresh.value.title, "新笔记");
      fresh.value.title = "第一篇";
      assert.deepEqual(JSON.parse(fresh.value.config), {
        title: "第一篇",
        collection: { profile: "notebook", id: "work" },
      });
    });
  } finally {
    scope.stop();
  }
});

test("notebook labels describe counts, status and time without surprises", () => {
  assert.equal(notebookCountsLabel({ note_count: 0, published_count: 0 }), "还没有笔记");
  assert.equal(notebookCountsLabel({ note_count: 3, published_count: 1 }), "3 篇笔记 · 1 篇已发布");
  assert.equal(noteStatusLabel("published"), "已发布");
  assert.equal(noteStatusLabel("draft"), "草稿");
  assert.equal(formatNoteTime(""), "");
  assert.equal(formatNoteTime("not a date"), "");
  assert.equal(formatNoteTime(new Date(2026, 10, 7, 9, 5).toISOString()), "2026-11-07 09:05");
});

test("notebook screens explain the migration gate and open public pages safely", () => {
  const dir = new URL("../web/src/modules/notebooks/", import.meta.url);
  for (const name of ["NotebookList", "NoteList", "NotebookEditor", "NoteEditor"]) {
    const source = readFileSync(new URL(`${name}.vue`, dir), "utf8");
    assert.match(source, /<NotebookMigrationNotice v-if="migration"/, `${name} must explain the pending migration`);
    for (const anchor of source.match(/<a\b[^>]*>/g) ?? [])
      if (anchor.includes('target="_blank"'))
        assert.match(anchor, /rel="noopener noreferrer"/, `${name} opens public pages in a new tab safely`);
  }
  // 只有已经能访问的页面才给“在博客中查看”：笔记本要有已发布笔记，笔记必须已发布。
  assert.match(readFileSync(new URL("NotebookList.vue", dir), "utf8"), /public_path && \w+\.published_count/);
  assert.match(readFileSync(new URL("NoteList.vue", dir), "utf8"), /v-if="note\.public_path"/);
  assert.match(readFileSync(new URL("NoteEditor.vue", dir), "utf8"), /status === 'published' && publicPath/);
});

test("notebook requests leave error display to the page, and migration failures get a short toast", async () => {
  const original = http.defaults.adapter;
  const muted: boolean[] = [];
  http.defaults.adapter = async (config) => {
    muted.push(config.mute === true);
    return { config, status: 200, statusText: "OK", headers: {}, data: {} };
  };
  try {
    await notebookApi.list();
    await notebookApi.get("a");
    await notebookApi.create({});
    await notebookApi.update("a", {});
    await notebookApi.delete("a");
    await notebookApi.listNotes("a");
    await notebookApi.getNote("a", 1);
    await notebookApi.createNote("a", {});
    await notebookApi.updateNote("a", 1, {});
    await notebookApi.deleteNote("a", 1);
    await notebookApi.publishNote("a", 1);
    await notebookApi.saveNoteDraft("a", 1);
    assert.equal(muted.length, 12);
    assert.ok(muted.every(Boolean), "every notebook request must mute the global toast");
  } finally {
    http.defaults.adapter = original;
  }
  const raw = "stellar content migration required: notebooks schema: missing column icon; notes schema: missing column slug";
  assert.equal(failureToast(raw, true), "需要先迁移到 Stellar 2.0，详见页面顶部的说明");
  assert.equal(failureToast("保存笔记失败", false), "保存笔记失败");
});
