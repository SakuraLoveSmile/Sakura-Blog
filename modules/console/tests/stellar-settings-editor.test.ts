import { test } from "node:test";
import assert from "node:assert/strict";
import { useStellarSettings } from "../web/src/modules/settings/useStellarSettings.ts";

const original =
  ' { "version":2, "theme":{"article":{"style":"tech"},"comments":{"giscus":{"params":{"large":9007199254740993,"no":false,"nil":null,"zero":0}}}}, "blog_extensions":{"native_comments":{"notify":{"password":"****"}}} } ';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

function fixture(options: { confirmReload?: () => boolean } = {}) {
  const calls: { action: string; document?: string }[] = [];
  const api = {
    get: async () => {
      calls.push({ action: "get" });
      return { data: original };
    },
    save: async (document: string) => {
      calls.push({ action: "save", document });
      return { data: document };
    },
    validate: async (document: string) => {
      calls.push({ action: "validate", document });
      return { data: { valid: true, issues: [] } };
    },
  };
  return { api, calls, editor: useStellarSettings({ api, ...options }) };
}

test("shared settings patches only the selected field and transports the exact masked raw document", async () => {
  const { editor, calls } = fixture();
  assert.equal(await editor.load(), true);
  assert.equal(editor.draft.value, original);
  assert.equal(editor.loadRevision.value, 1);
  assert.equal(editor.updateField("theme.article.style", "story"), true);
  const expected = original.replace('"tech"', '"story"');
  assert.equal(editor.draft.value, expected);
  assert.equal(editor.dirty.value, true);
  assert.equal(await editor.submit("validate"), true);
  assert.equal(editor.showValidation.value, true);
  assert.equal(await editor.submit("save"), true);
  assert.equal(editor.draft.value, expected);
  assert.equal(editor.saved.value, expected);
  assert.equal(editor.dirty.value, false);
  assert.deepEqual(calls, [
    { action: "get" },
    { action: "validate", document: expected },
    { action: "save", document: expected },
  ]);
});

test("failed save retains the exact draft, saved baseline and diagnostics until the draft changes", async () => {
  const { editor, api } = fixture();
  await editor.load();
  editor.updateField("theme.article.style", "unsupported");
  const draft = editor.draft.value;
  const issue = { source: "theme", path: "article.style", code: "invalid_value", expected: "tech | story" };
  api.save = async () => {
    throw { response: { data: JSON.stringify({ status: 422, detail: "文章类型无效", issues: [issue] }) } };
  };
  assert.equal(await editor.submit("save"), false);
  assert.equal(editor.draft.value, draft);
  assert.equal(editor.saved.value, original);
  assert.equal(editor.dirty.value, true);
  assert.equal(editor.busy.value, "");
  assert.equal(editor.showValidation.value, true);
  assert.equal(editor.validationMessage.value, "文章类型无效");
  assert.deepEqual(editor.issues.value, [issue]);
  editor.updateField("theme.article.style", "story");
  assert.equal(editor.showValidation.value, false);
});

test("save response cannot replace edits made while saving, and all network actions share a lock", async () => {
  const { editor, api, calls } = fixture();
  await editor.load();
  editor.updateField("theme.article.style", "story");
  const submitted = editor.draft.value;
  const pending = deferred<{ data: string }>();
  api.save = async (document) => {
    calls.push({ action: "save", document });
    return pending.promise;
  };
  const saving = editor.submit("save");
  assert.equal(editor.busy.value, "save");
  assert.equal(editor.updateField("blog_extensions.content.banner_image", "/new.jpg"), true);
  const edited = editor.draft.value;
  assert.equal(await editor.load(), false);
  assert.equal(await editor.submit("validate"), false);
  assert.equal(await editor.submit("save"), false);
  pending.resolve({ data: submitted + "\n" });
  assert.equal(await saving, true);
  assert.equal(editor.saved.value, submitted + "\n");
  assert.equal(editor.draft.value, edited);
  assert.equal(editor.dirty.value, true);
  assert.deepEqual(
    calls.map((call) => call.action),
    ["get", "save"],
  );
});

test("reload confirms only dirty state and successful reload clears local pending inputs", async () => {
  let confirms = 0;
  let allow = false;
  const { editor, calls } = fixture({
    confirmReload: () => {
      confirms++;
      return allow;
    },
  });
  await editor.load();
  assert.equal(confirms, 0);
  editor.pendingInput.value = true;
  assert.equal(editor.dirty.value, true);
  assert.equal(await editor.load(), false);
  assert.equal(editor.pendingInput.value, true);
  assert.equal(editor.loadRevision.value, 1);
  assert.equal(calls.length, 1);
  allow = true;
  assert.equal(await editor.load(), true);
  assert.equal(confirms, 2);
  assert.equal(editor.pendingInput.value, false);
  assert.equal(editor.dirty.value, false);
  assert.equal(editor.loadRevision.value, 2);
});

test("pending structured input blocks persistence without sending a partial document", async () => {
  const { editor, calls } = fixture();
  await editor.load();
  editor.pendingInput.value = true;
  assert.equal(await editor.submit("save"), false);
  assert.equal(editor.validationMessage.value, "部分设置还有未完成的输入，请修正后再保存");
  assert.equal(editor.draft.value, original);
  assert.equal(editor.saved.value, original);
  assert.equal(editor.dirty.value, true);
  assert.deepEqual(
    calls.map((call) => call.action),
    ["get"],
  );
});

test("failed reload preserves the unsaved raw document and cannot race another reload", async () => {
  const { editor, api, calls } = fixture({ confirmReload: () => true });
  await editor.load();
  editor.updateField("theme.article.style", "story");
  const draft = editor.draft.value;
  const pending = deferred<{ data: string }>();
  api.get = async () => {
    calls.push({ action: "get" });
    return pending.promise;
  };
  const reading = editor.load();
  assert.equal(await editor.load(), false);
  assert.equal(await editor.submit("save"), false);
  pending.reject(new Error("offline"));
  assert.equal(await reading, false);
  assert.equal(editor.draft.value, draft);
  assert.equal(editor.saved.value, original);
  assert.equal(editor.loadError.value, "offline");
  assert.equal(editor.loadRevision.value, 1);
  assert.equal(editor.dirty.value, true);
  assert.equal(calls.length, 2);
});

test("migration failure permits read-only validation but never saves the configuration", async () => {
  const { editor, api, calls } = fixture();
  const migration = { source: "settings", path: "stellar_v2", reason: "missing", sections: ["style"] };
  api.get = async () => {
    throw { response: { data: { status: 409, detail: "需要迁移", issues: [], migration } } };
  };
  assert.equal(await editor.load(), false);
  const draft = editor.draft.value;
  assert.deepEqual(editor.migration.value, migration);
  assert.equal(await editor.submit("save"), false);
  assert.equal(await editor.submit("validate"), true);
  assert.equal(editor.draft.value, draft);
  assert.equal(editor.migration.value?.reason, "missing");
  assert.deepEqual(
    calls.map((call) => call.action),
    ["validate"],
  );
});

test("invalid JSON and section shapes block form patches without replacing the draft", async () => {
  const { editor } = fixture();
  await editor.load();
  for (const invalid of ["{broken", "[]", "null", '{"theme":false}', '{"blog_extensions":[]}']) {
    editor.draft.value = invalid;
    assert.ok(editor.parseError.value);
    assert.equal(editor.updateField("theme.article.style", "story"), false);
    assert.equal(editor.draft.value, invalid);
  }
  editor.draft.value = original;
  assert.equal(editor.parseError.value, "");
  assert.throws(() => editor.updateField("theme.comments.giscus.params", null, "{broken"));
  assert.equal(editor.draft.value, original);
});

test("validation results for an older request are hidden after editing the shared draft", async () => {
  const { editor, api } = fixture();
  await editor.load();
  const pending = deferred<{ data: { valid: boolean; issues: [] } }>();
  api.validate = async () => pending.promise;
  const checking = editor.submit("validate");
  editor.updateField("theme.article.style", "story");
  pending.resolve({ data: { valid: true, issues: [] } });
  assert.equal(await checking, true);
  assert.equal(editor.showValidation.value, false);
  assert.equal(editor.dirty.value, true);
  assert.equal(editor.saved.value, original);
});
