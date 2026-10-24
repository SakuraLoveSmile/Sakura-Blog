import { test } from "node:test";
import assert from "node:assert/strict";
import { ref, effectScope } from "../web/node_modules/vue/index.js";
import { patchJSONField, readJSONFieldRaw, readJSONField } from "../web/src/components/editor/stellar-form-json.ts";
import { useStellarForm } from "../web/src/components/editor/useStellarForm.ts";
import {
  postFields,
  wikiFields,
  pageFields,
  collectionFields,
  noteFields,
} from "../web/src/components/editor/stellar-form-fields.ts";
const bag = '{"large":9007199254740993,"no":false,"nil":null,"zero":0}';

test("editing a basic field preserves untouched raw tokens and nested parameter bags", () => {
  const original =
    ' { "title" : "旧标题", "leftbar":{"widgets":[{"override":"custom","params":' +
    bag +
    '}]},"comments":{"enabled":false,"title":null}} ';
  const edited = patchJSONField(original, "title", "新标题");
  assert.equal(edited, original.replace('"旧标题"', '"新标题"'));
  const widget = JSON.parse(readJSONFieldRaw(edited, "leftbar.widgets")!);
  widget[0].params.zero = 2;
  const next = patchJSONField(edited, "leftbar.widgets", widget);
  assert.ok(next.includes('"large":9007199254740993'));
  assert.ok(next.includes('"no":false'));
  assert.ok(next.includes('"nil":null'));
  assert.equal(JSON.parse(next).leftbar.widgets[0].params.zero, 2);
  assert.deepEqual(JSON.parse(patchJSONField(next, "leftbar.widgets", [])).leftbar.widgets, []);
  assert.equal(JSON.parse(patchJSONField(next, "leftbar.widgets", undefined)).leftbar.widgets, undefined);
});

test("raw parameters, escaped keys, nested insertion and array labels preserve values", () => {
  const raw = patchJSONField('{"a\\\"b":false}', "hero.background.effect.options", null, bag);
  assert.equal(readJSONFieldRaw(raw, "hero.background.effect.options"), bag);
  assert.equal(JSON.parse(raw)['a"b'], false);
  const action = patchJSONField('{"hero":{"actions":[{"title":"旧","url":"#start"}]}}', "hero.actions.0.title", "新");
  assert.deepEqual(JSON.parse(action).hero.actions, [{ title: "新", url: "#start" }]);
  assert.throws(() => patchJSONField(raw, "title", "x", "invalid"));
});
function inScope(run: () => void) {
  const scope = effectScope();
  try {
    scope.run(run);
  } finally {
    scope.stop();
  }
}
test("post and page controls patch strict snake_case, raw edits refill, replacement records do not migrate", () =>
  inScope(() => {
    const form = ref<any>({
      stellar_version: 2,
      stellar_config:
        '{"title":"原文","cover":"/cover","article":{"author":"alice","ai_label":null},"comments":{"extra":' +
        bag +
        "}}",
      blog_extensions: "{}",
      title: "旧列",
      categories: [],
      authors: [],
      tags: "",
    });
    const error = useStellarForm(() => form.value, postFields);
    assert.equal(form.value.title, "原文");
    assert.equal(form.value.author, "alice");
    form.value.title = "基本控件标题";
    form.value.cover = "/new";
    form.value.banner = "/detail";
    assert.equal(JSON.parse(form.value.stellar_config).title, "基本控件标题");
    assert.equal(JSON.parse(form.value.stellar_config).cover, "/new");
    assert.equal(JSON.parse(form.value.blog_extensions).content.banner_image, "/detail");
    assert.ok(form.value.stellar_config.includes(bag));
    form.value.stellar_config = '{"title":"JSON修改","article":{"author":null},"comments":{"extra":' + bag + "}}";
    assert.equal(form.value.title, "JSON修改");
    assert.equal(form.value.author, "");
    assert.equal(form.value.cover, "");
    form.value.stellar_config = "{broken";
    form.value.title = "不得覆盖原文";
    assert.equal(form.value.stellar_config, "{broken");
    assert.equal(form.value.title, "JSON修改");
    assert.ok(error.value);
    form.value = { stellar_version: 0, stellar_config: "{}", blog_extensions: "{}", title: "旧内容", author: "旧作者" };
    assert.equal(form.value.title, "旧内容");
    assert.equal(form.value.stellar_config, "{}");
    const page = ref<any>({
      stellar_version: 2,
      stellar_config:
        '{"title":"页面","leftbar":{"brand":{"name":"品牌"},"widgets":[{"override":"custom", "option":' + bag + "}]}}",
      blog_extensions: "{}",
      banner_info: {},
      title: "页面",
    });
    useStellarForm(() => page.value, pageFields);
    page.value.banner_info.title = "横幅标题";
    page.value.author = "alice";
    page.value.rightbar_widgets = [];
    page.value.leftbar_widgets[0].params.option.zero = 5;
    const p = JSON.parse(page.value.stellar_config);
    assert.equal(p.banner.headline, "横幅标题");
    page.value.banner_info.title = "";
    assert.equal(JSON.parse(page.value.stellar_config).banner.headline, undefined);
    assert.equal(p.article.author, "alice");
    assert.deepEqual(p.rightbar.widgets, []);
    assert.equal(p.leftbar.brand.name, "品牌");
    assert.ok(page.value.stellar_config.includes("9007199254740993"));
    page.value.leftbar_widgets = null;
    assert.equal(JSON.parse(page.value.stellar_config).leftbar.widgets, undefined);
  }));

test("scalar post categories submit as a list without rewriting raw configuration or splitting category names", () =>
  inScope(() => {
    const original =
      ' { "title": "导入文章", "categories" : "应用,工具，指南/Android", "comments":{"extra":' + bag + "} }";
    const form = ref<any>({
      stellar_version: 2,
      stellar_config: original,
      blog_extensions: "{}",
      categories: [],
    });
    const error = useStellarForm(() => form.value, postFields);
    const expected = ["应用,工具，指南/Android"];
    assert.deepEqual(form.value.categories, expected);
    assert.deepEqual(JSON.parse(JSON.stringify(form.value)).categories, expected);
    assert.equal(form.value.stellar_config, original);
    form.value.title = "保存时修改标题";
    assert.equal(readJSONFieldRaw(form.value.stellar_config, "categories"), '"应用,工具，指南/Android"');
    assert.deepEqual(form.value.categories, expected);
    assert.ok(form.value.stellar_config.includes(bag));
    form.value.categories = [...form.value.categories, "其他/分类"];
    assert.deepEqual(JSON.parse(form.value.stellar_config).categories, [...expected, "其他/分类"]);
    form.value.categories = [];
    assert.deepEqual(JSON.parse(form.value.stellar_config).categories, []);
    assert.equal(error.value, "");
  }));

test("post category lists, empty values and replaced records keep an array in the form", () =>
  inScope(() => {
    const form = ref<any>({ stellar_version: 2, stellar_config: "{}", blog_extensions: "{}", categories: [] });
    useStellarForm(() => form.value, postFields);
    for (const [raw, expected] of [
      ['{"categories":["技术/Go","随笔"]}', ["技术/Go", "随笔"]],
      ['{"categories":[]}', []],
      ['{"categories":null}', []],
      ['{"categories":""}', []],
      ['{"categories":"  "}', []],
      ["{}", []],
      ['{"categories":"应用使用指南"}', ["应用使用指南"]],
    ] as const) {
      form.value.stellar_config = raw;
      assert.deepEqual(JSON.parse(JSON.stringify(form.value.categories)), expected);
      assert.equal(form.value.stellar_config, raw);
    }
    form.value = {
      stellar_version: 2,
      stellar_config: '{"categories":"重新加载分类"}',
      blog_extensions: "{}",
      categories: ["宿主列"],
    };
    assert.deepEqual(JSON.parse(JSON.stringify(form.value)).categories, ["重新加载分类"]);
  }));

test("collection and Hero controls preserve options, command labels, route, visibility and unknown action data", () =>
  inScope(() => {
    const raw =
      '{"name":"集合","visibility":{"listed":false,"searchable":false},"route":{"homepage":"intro"},"hero":{"enabled":true,"background":{"effect":{"type":"strands","options":' +
      bag +
      '}},"preview":{"type":"terminal","commands":[{"label":"安装","codes":"npm i"}]},"actions":[{"title":"旧","url":"#start"}]}}';
    const form = ref<any>({ stellar_version: 2, stellar_config: raw, blog_extensions: "{}", hero: {}, visible: true });
    useStellarForm(() => form.value, wikiFields);
    assert.equal(form.value.hero.animationType, "strands");
    assert.equal(form.value.hero.animationParamsText, bag);
    assert.equal(form.value.visible, true); // listed=false不把整个集合变成私密，也不反向设公开
    form.value.name = "新集合";
    form.value.hero.commands[0].codes = "npm install\nnpm run build";
    form.value.hero.start = "开始阅读";
    const saved = JSON.parse(form.value.stellar_config);
    assert.equal(saved.name, "新集合");
    assert.deepEqual(saved.hero.preview.commands, [{ label: "安装", codes: "npm install\nnpm run build" }]);
    assert.deepEqual(saved.hero.actions, [{ title: "开始阅读", url: "#start" }]);
    assert.deepEqual(saved.visibility, { listed: false, searchable: false });
    assert.equal(saved.route.homepage, "intro");
    assert.ok(form.value.stellar_config.includes(bag));
    const topic = ref<any>({ stellar_version: 2, stellar_config: '{"name":"专题"}', blog_extensions: "{}" });
    useStellarForm(() => topic.value, collectionFields);
    topic.value.sort = 3;
    topic.value.cover = "/topic.jpg";
    assert.equal(JSON.parse(topic.value.blog_extensions).content.collection_order, 3);
    assert.equal(JSON.parse(topic.value.stellar_config).cover, "/topic.jpg");
    const note = ref<any>({ config: '{"title":"笔记","tags":["工作/旧"]}', blog: "{}", tags: "" });
    useStellarForm(() => note.value, noteFields, "config", "blog");
    note.value.tags = "工作/新\n随笔";
    assert.deepEqual(JSON.parse(note.value.config).tags, ["工作/新", "随笔"]);
  }));

test("array removal and reordering preserve distinct large integer tokens through Vue field mapping", () =>
  inScope(() => {
    const raw =
      '{"leftbar":{"widgets":[{"override":"custom","n":9007199254740992},{"override":"custom","n":9007199254740993}]}}';
    const widgets = readJSONField(raw, "leftbar.widgets") as any[];
    assert.ok(patchJSONField(raw, "leftbar.widgets", widgets.slice(1)).includes("9007199254740993"));
    assert.equal(
      readJSONFieldRaw(patchJSONField(raw, "leftbar.widgets", widgets.slice().reverse()), "leftbar.widgets"),
      '[{"override":"custom","n":9007199254740993},{"override":"custom","n":9007199254740992}]',
    );
    assert.throws(() => patchJSONField(raw, "leftbar.widgets", JSON.parse(raw).leftbar.widgets.slice(1)), /精度歧义/);
    const form = ref<any>({ stellar_version: 2, stellar_config: raw, blog_extensions: "{}", banner_info: {} });
    useStellarForm(() => form.value, pageFields);
    form.value.leftbar_widgets = form.value.leftbar_widgets.slice().reverse();
    assert.ok(
      readJSONFieldRaw(form.value.stellar_config, "leftbar.widgets")!.startsWith(
        '[{"override":"custom","n":9007199254740993}',
      ),
    );
    form.value.leftbar_widgets = form.value.leftbar_widgets.slice(1);
    assert.ok(form.value.stellar_config.includes("9007199254740992"));
  }));

test("Hero option drafts remain editable and block save until valid without rolling back", () =>
  inScope(() => {
    const form = ref<any>({
      stellar_version: 2,
      stellar_config: '{"hero":{"background":{"effect":{"type":"strands","options":{"seed":false}}}}}',
      blog_extensions: "{}",
      hero: {},
    });
    const error = useStellarForm(() => form.value, wikiFields);
    form.value.hero.animationParamsText = "{";
    assert.equal(form.value.hero.animationParamsText, "{");
    assert.ok(error.value);
    assert.equal(readJSONFieldRaw(form.value.stellar_config, "hero.background.effect.options"), '{"seed":false}');
    form.value.name = "改名";
    assert.equal(form.value.hero.animationParamsText, "{");
    assert.ok(error.value);
    form.value.hero.animationParamsText = '{"seed":null}';
    assert.equal(error.value, "");
    assert.equal(readJSONFieldRaw(form.value.stellar_config, "hero.background.effect.options"), '{"seed":null}');
  }));

test("terminal tab edits keep each multiline code and label separate", () =>
  inScope(() => {
    const raw =
      '{"hero":{"preview":{"type":"terminal","commands":[{"label":"macOS","codes":"brew install\\nblog init"},{"label":"Linux","codes":"apt install\\nblog init"}]}}}';
    const form = ref<any>({ stellar_version: 2, stellar_config: raw, blog_extensions: "{}", hero: {} });
    useStellarForm(() => form.value, wikiFields);
    form.value.name = "新集合";
    assert.deepEqual(
      JSON.parse(form.value.stellar_config).hero.preview.commands,
      JSON.parse(raw).hero.preview.commands,
    );
    form.value.hero.commands[1].codes = "apt update\nsudo apt install";
    form.value.hero.commands.splice(0, 1);
    assert.deepEqual(JSON.parse(form.value.stellar_config).hero.preview.commands, [
      { label: "Linux", codes: "apt update\nsudo apt install" },
    ]);
  }));
