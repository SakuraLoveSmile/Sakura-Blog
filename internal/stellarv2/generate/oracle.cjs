// 官方解析器作为 oracle；生成固定夹具，Go 测试无需安装/运行 Node。
"use strict";
const fs = require("node:fs"), path = require("node:path"), zlib = require("node:zlib"), crypto = require("node:crypto");
const upstream = path.resolve(process.argv[2]);
const load = name => require(path.join(upstream,name));
const {parseStellarConfig} = load("scripts/lib/config-schema");
const {parsePageConfig,parseCollectionConfig,validateCollectionProfileConfig,validatePageProfileConfig} = load("scripts/lib/content-config");
const {CONFIG_SCHEMA} = load("scripts/schema/config-schema");
const {FRONT_MATTER_CONFIG_SCHEMA,COLLECTION_CONFIG_SCHEMA} = load("scripts/schema/content-config-schema");
// 能力声明与渲染依赖分离：这里只执行 adapter 的声明，不调用 build/prepare。
const vm = require("node:vm");
function getProfileAdapter(profile) {
  const module = {exports:{}};
  vm.runInNewContext(fs.readFileSync(path.join(upstream,`scripts/lib/collection-pipeline/adapters/${profile}.js`),"utf8"), {module,require:()=>({})});
  return module.exports;
}
const membership = load("scripts/lib/content-membership");
function canonical(value) { if (Array.isArray(value)) return value.map(canonical);if(value&&typeof value==="object")return Object.fromEntries(Object.keys(value).sort().map(key=>[key,canonical(value[key])]));return value; }
function digest(value) { return crypto.createHash("sha256").update(JSON.stringify(canonical(value))).digest("hex"); }
function normalizeIssues(issues) { return issues.map(({severity,...issue})=>issue).sort((a,b)=>JSON.stringify(canonical(a)).localeCompare(JSON.stringify(canonical(b)))); }
const fixtures=[], upstreamCrashes=[];
function fixture(kind,label,input,recover=false) {
  const source=kind==="theme"?"oracle.yml":kind==="collection"?"source/_data/wiki/oracle.yml":"source/oracle.md";
  const diagnostics=[];let value=null,issues=[],failed=false;
  try {
    const options={mode:recover?"recover":undefined,onIssues:current=>diagnostics.push(...current)};
    value=kind==="theme"?parseStellarConfig({themeConfig:input,source,...options}):kind==="page"?parsePageConfig(input,source,options):parseCollectionConfig(input,source,options);
    issues=diagnostics;
  }catch(error){if(!Array.isArray(error.issues)){upstreamCrashes.push({kind,label,recover,error:String(error)});return;}failed=true;issues=error.issues;}
  fixtures.push({kind,label,input,recover,source,failed,valueHash:failed?null:digest(value),issues:normalizeIssues(issues)});
}
function set(root,tokens,value) { let current=root;for(let i=0;i<tokens.length-1;i++){const k=tokens[i],next=tokens[i+1];if(!current[k]||typeof current[k]!=="object")current[k]=typeof next==="number"?[]:{};current=current[k];}current[tokens.at(-1)]=structuredClone(value); }
function sweep(kind,node,tokens=[]) {
  const label=tokens.join(".")||"root";
  const candidates=[null,false,0,-1,1.5,"__invalid__",[],{},...(node.values||[])];
  if(node.minimum!==undefined)candidates.push(node.minimum-1,node.minimum);
  if(node.maximum!==undefined)candidates.push(node.maximum+1,node.maximum);
  if(node.exclusiveMinimum!==undefined)candidates.push(node.exclusiveMinimum,node.exclusiveMinimum+1);
  if(node.default?.kind==="literal"&&node.default.value!==undefined)candidates.push(node.default.value);
  const seen=new Set();for(const candidate of candidates){const key=JSON.stringify(candidate);if(seen.has(key))continue;seen.add(key);let input=tokens.length?(kind==="collection"?{name:"Oracle"}:{}):candidate;if(tokens.length)set(input,tokens,candidate);for(const recover of [false,true])fixture(kind,`${label}=${key.slice(0,140)}`,input,recover);}
  for(const [key,child] of Object.entries(node.properties||{}))sweep(kind,child,[...tokens,key]);
  if(node.items)sweep(kind,node.items,[...tokens,0]);
  if(node.additionalProperties)sweep(kind,node.additionalProperties,[...tokens,"oracle.key"]);
}
for(const [kind,schema]of[["theme",CONFIG_SCHEMA],["page",FRONT_MATTER_CONFIG_SCHEMA],["collection",COLLECTION_CONFIG_SCHEMA]])sweep(kind,schema);
const examples = [
 ["theme","official recovery mixed list",{unknown:true,appearance:{preset:"unsupported",colors:{primary:"red; display:none"}},article:{category_colors:{"release.v2":"red; display:none",stable:"blue"}},leftbar:{menu:[{id:"home",title:"Home",url:"/"},{id:"unsafe",title:"Unsafe",url:"javascript:alert(1)"},{type:"unsupported"},{id:"missing-url",title:"Missing URL"}]}}],
 ["theme","official provider bags",{comments:{provider:"giscus",giscus:{"data-repo":"owner/repo",nested_option:{enabled:true}}},canonical:{host:null},preconnect:[" https://cdn.example.com/ ","https://cdn.example.com"]}],
 ["theme","active menu default recovery",{leftbar:{menu:[{id:"custom",title:"Custom",url:"/custom/"}]}}],
 ["theme","array replace clears inherited menu",{profiles:{post:{leftbar:{menu:[],widgets:[]},active_menu:null}},leftbar:{menu:[]}}],
 ["theme","custom KaTeX integrity",{features:{math:{katex:{css:"/katex-custom.css"}}}}],
 ["theme","nullable resources open bag",{comments:{waline:{js:null,css:null,meta_css:null,"nest.option":{arbitrary_value:false}}}}],
 ["theme","footer action union",{leftbar:{footer:{actions:[{type:"spacer"},{type:"link",icon:"home",url:"/"},{type:"button",title:"Action",icon:"go",onclick:"go()"},{type:"dropdown",title:"More",items:[{type:"link",title:"Example",url:"https://example.com"},{type:"button",title:"Button",onclick:"go()"}]}]}}}],
 ["page","official full frontmatter",{title:"Page",date:"2026-10-13",collection:{profile:"wiki",id:"docs"},leftbar:{widgets:[]},comments:{provider:"custom",options:{nested_value:{enabled:true}}},render:{math:"katex",diagrams:{theme:"dark"}},footer:{license:false,share:[]}}],
 ["collection","official collection",{name:"Docs",route:{path:"/wiki/docs/"},topbar:{enabled:true,brand:{name:"Docs",href:"/wiki/docs/"},menu:[]},leftbar:{brand:{source:"collection",style:"compact",back_button:false,search:true},widgets:["tree",{layout:"custom",option:true}]}}],
 ["collection","navigation tree bad mixed items",{name:"Docs",navigation:{tree:{"release.2":["intro",12,"start"],empty:[]}}}],
 ["collection","recovery safe effect mixed options",{name:"Docs",hero:{background:{effect:{type:"galaxy",options:{starSpeed:1,wrong:true,density:"bad"},runtime:{pause_when_hidden:true,unknown:false}}}}}],
 ["page","structural identity remains fatal",{collection:{profile:"unknown",id:42},banner:{unknown:true}}],
];
for(const[k,label,input]of examples)for(const recover of[false,true])fixture(k,label,input,recover);
// WHATWG URL 与 Go net/url 边界不同，单独覆盖，避免仅验证常规 HTTPS。
for(const value of ["/","//host/path","../image.png","./image.png","image.png","/safe/../escape","#fragment","#","mailto:","mailto:test@example.com","tel:","tel:123","https:","https:example.com","https:/example.com","https:///example.com","https:\\example.com","https://example.com","HTTPS://EXAMPLE.COM"," https://example.com ","https://ex\nample.com","https://ex\tample.com","https://example.com:65536","https://example.com:99999","https://example.com:-1","http://999.0.0.1","http://127.1","http://0x7f.1","http://[xyz]","http://[::1]","http://%65xample.com","https://例子.测试/path","https://user:pass@example.com","http://","data:image/png;base64,AAAA","data:text/html,unsafe","javascript:alert(1)","url(foo)","https://example.com/\"x","https://example.com/<x>","https://example.com/%20","http://foo_bar","http://foo!bar","http://foo~bar","http://foo'bar","http://foo;bar","http://foo$bar","http://foo,bar","http://foo{bar","http://foo}bar","http://foo&bar","http://foo=bar","http://-foo","http://foo-","http://.foo","http://foo..bar","http://foo.0xFFFFFFFFFFFFFFFFFFFF","http://foo.0xzz","http://xn--","http://foo!例子.测试","http://foo%25bar","http://foo^bar","mailto://","mailto://@","mailto://:0","mailto://[xyz]","mailto://example.com:65536","mailto://%zz","tel://[::1]:123","tel://foo\\bar"]){
 for(const recover of[false,true]){
  fixture("theme",`nav URL ${JSON.stringify(value)}`,{topbar:{brand:{href:value}}},recover);
  fixture("theme",`HTTP URL ${JSON.stringify(value)}`,{services:{github:{api_url:value}}},recover);
  fixture("theme",`Resource ${JSON.stringify(value)}`,{fallbacks:{avatar:value}},recover);
 }
}
const {heroEffectDefinitions}=load("scripts/lib/hero-effect-registry");
for(const effect of heroEffectDefinitions()){
  fixture("collection",`${effect.id} official complete defaults`,{name:"Oracle",hero:{background:{effect:{type:effect.id,options:effect.defaults}}}});
  for(const [key,rule]of Object.entries(effect.options))for(const value of[null,false,0,-1,1.5,"invalid",[],["#ffffff"],[1,2],{},rule.values?.[0]].filter(v=>v!==undefined))for(const recover of[false,true])fixture("collection",`${effect.id}.${key}=${JSON.stringify(value)}`,{name:"Oracle",hero:{background:{effect:{type:effect.id,options:{[key]:value}}}}},recover);
}
const profiles=[];
for(const profile of["post","wiki","topic","notebook"]){for(const kind of["page","collection"]){const config={hero:{enabled:true},route:{start:"intro"},navigation:{tree:["intro"]},listing:{priority:2,order:3,excerptLength:50,perPage:0,sort:{field:"title",direction:"asc"}}};const diagnostics=[];try{const value=kind==="page"?validatePageProfileConfig(config,"oracle",profile,getProfileAdapter(profile).config,{onIssues:issues=>diagnostics.push(...issues)}):validateCollectionProfileConfig(config,"oracle",profile,getProfileAdapter(profile).config,{onIssues:issues=>diagnostics.push(...issues)});profiles.push({kind,profile,config,value,issues:normalizeIssues(diagnostics)});}catch(e){profiles.push({kind,profile,config,failed:true,issues:normalizeIssues(e.issues)});}}}
const collections={"wiki/docs":{route:{path:"custom/docs/"},navigation:{tree:{Start:["intro","setup"]}}},"wiki/other":{route:{path:"custom/docs/"},navigation:{tree:["shared"]}},"topic/tutorial":{route:{start:"hello"}},"notebooks/journal":{}};
const members=[];const registry=membership.createCollectionRegistry(Object.entries(collections));
for(const kind of["pages","posts"])for(const source of["wiki/docs/intro.md","custom/docs/intro.md","custom/docs/shared.md","notebooks/journal/first.md","wiki/missing/intro.md","_posts/topics/tutorial/hello.md","_posts/hello.md","random.md"])for(const collection of[undefined,{profile:"wiki",id:"docs"},{profile:"wiki",id:"other"},{profile:"notebook",id:"journal"},{profile:"topic",id:"tutorial"}]){const config=collection?{collection}:{};const input={kind,source,config};const result=membership.resolveContentMembership({...input,registry});members.push({...input,result});}
fs.writeFileSync(path.join(__dirname,"../testdata/oracle.json.gz"),zlib.gzipSync(JSON.stringify({fixtures,upstreamCrashes,profiles,collections,members}),{level:9}));
process.stdout.write(JSON.stringify({fixtures:fixtures.length,upstreamCrashes,profiles:profiles.length,members:members.length},null,2)+"\n");
