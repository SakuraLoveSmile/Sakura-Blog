// 开发期读取锁定的官方实现；生产和 Go 测试不依赖 Node。
"use strict";
const fs = require("node:fs"), path = require("node:path"), vm = require("node:vm"), os = require("node:os");
const upstream = path.resolve(process.argv[2]);
const adapters = Object.fromEntries(["post", "wiki", "topic", "notebook"].map(id => {
  const module = {exports: {}};
  vm.runInNewContext(fs.readFileSync(path.join(upstream, `scripts/lib/collection-pipeline/adapters/${id}.js`), "utf8"), {module, require: () => ({})});
  return [id, module.exports];
}));
function loadOfficial(name) {
  const file = path.join(upstream, name), module = {exports: {}};
  vm.runInNewContext(fs.readFileSync(file, "utf8"), {module, __dirname: path.dirname(file), process, console,
    require: id => id === "./collection-pipeline/registry" ? {getProfileAdapter: id => adapters[id]} : require(id.startsWith(".") ? path.resolve(path.dirname(file), id) : id)});
  return module.exports;
}
const notes = loadOfficial("scripts/lib/new-note.js"), doctor = loadOfficial("scripts/lib/doctor.js");
const root = fs.mkdtempSync(path.join(os.tmpdir(), "stellar-cli-oracle-"));
const source = path.join(root,"source");
fs.mkdirSync(path.join(source,"_data/notebooks"),{recursive:true});
fs.writeFileSync(path.join(source,"_data/notebooks/journal.yml"),'name: Journal\n');
fs.writeFileSync(path.join(root,"_config.yml"),'theme: stellar\n');
const titles = ["Hello", " Hello 世界 ", "", " ", ".", "..", "../escape", "/absolute", "a/b", "a\\b", "a:b", "a<b", "a>b", 'a"b', "a|b", "a?b", "a*b", "nul\0", "line\nbreak", "a\rb", "a\tb", "résumé", "a % b", "emoji 📝", "name.md", "a.b", "~home", "con", "a#b", "a@b", "\ufeffTitle\ufeff", "\u0085Title\u0085"];
const noteCases = titles.map(title => {
  try {const plan=notes.buildNewNotePlan({baseDir:root,sourceDir:source,notebook:"journal",title,tags:"go, go, GO,a/b",generatedAt:"2026-10-13T04:34:00.000Z"});return {title,ok:true,normalizedTitle:plan.title,tags:plan.tags};}
  catch(error){return {title,ok:false};}
});
const tagCases = ["", "go, go,GO,a/b", ",,,", " go ,, sql ", "a\nb", "\na\n", "\0", "a\tb", "a\rb", "😀, 😀,表", "a/b,a/b,c/d", "false,null,0", "\ufeffgo\ufeff,sql", "\u0085go\u0085,sql"].map(input=>{try{return {input,ok:true,tags:notes.normalizeTags(input)}}catch(error){return {input,ok:false}}});
const themes = [{}, {topbar:{enabled:true,widgets:["ghrepo"]}}, {leftbar:{widgets:["menu","settings","toc","recent"]}}, {rightbar:{widgets:["spacer","unknown"]}}, {profiles:{post:{topbar:{enabled:true,widgets:[{type:"tree",name:"Custom"}]}}}}];
const doctorCases = themes.map(theme=>{
  fs.writeFileSync(path.join(root,"_config.stellar.yml"),JSON.stringify(theme));
  const result=doctor.runDoctor({baseDir:root,nodeVersion:"22.0.0",hexoVersion:"8.0.0"});
  return {theme, warnings:result.warnings.filter(w=>w.widget).map(({code,widget,layout,region,supported})=>({code,widget,layout,region,supported}))};
});
fs.rmSync(root,{recursive:true});
fs.writeFileSync(process.argv[3],JSON.stringify({commit:"9d11978a24fe6610e3a7dbab741cf62634539d05",noteCases,tagCases,doctorCases},null,2)+"\n");
