import { test } from "node:test";
import assert from "node:assert/strict";
import { notebookApi } from "../../../packages/api-client/src/api/notebooks.ts";
import { http } from "../../../packages/api-client/src/transport.ts";
import {
  newNotebookForm,
  newNoteForm,
  notebookPayload,
  notePayload,
  validateNoteSlug,
  validateNotebookID,
  isMigrationError,
  notebookRoute,
} from "../web/src/modules/notebooks/notebook-form.ts";

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
