// 生成官方 Regions/Widgets/Brand 行为夹具；生产代码不执行 JS。
"use strict";
const fs=require("node:fs"),path=require("node:path"),crypto=require("node:crypto"),zlib=require("node:zlib"),yaml=require("js-yaml");
const root=path.resolve(process.argv[2]);const load=name=>require(path.join(root,name));
const regions=load("scripts/lib/regions"),widgets=load("scripts/lib/widget-registry"),brand=load("scripts/lib/brand"),layout=load("scripts/lib/layout-config");
const defaults=yaml.load(fs.readFileSync(path.join(root,"_data/widgets.yml"),"utf8"));
const fixtures=[];const add=(kind,input,value)=>fixtures.push({kind,input,value});
const catalog={...defaults,custom:{layout:"custom",nested:{setting:[false,null,0]}},restricted:{layout:"toc",presentations:["rightbar"]},empty:{layout:"markdown",presentations:[]},alias:{layout:"timeline",limit:5},menu:{layout:"custom",presentations:["rightbar"]},settings:{layout:"custom"},spacer:{layout:"custom"},off:null,false:false};
for(const widgetLayout of Object.keys(widgets.PRESENTATIONS_BY_LAYOUT))catalog["layout-"+widgetLayout]={layout:widgetLayout,extra:{value:false}};
const specifications=[...Object.keys(catalog),"menu","settings","spacer","missing","brand","actions",null,false,0,[],{},...Object.keys(widgets.PRESENTATIONS_BY_LAYOUT).map(layout=>({layout})),
 {layout:"custom"},{layout:"brand"},{layout:"actions"},{layout:"timeline",presentations:["topbar"]},{override:"restricted",layout:"toc",presentations:["topbar"]},
 {override:"alias",title:"Alias Override",nested:{other:2}},{override:"missing",layout:"toc"},{override:"missing"},{override:"menu",layout:"toc",system:false},
 {override:"menu",layout:"markdown"},{override:"spacer",layout:"tree"},{override:"",layout:"toc"},{layout:12},{override:false,layout:"markdown"}];
for(const region of["topbar","leftbar","rightbar","drawer","leftbarRail",""])for(const contentOnly of[false,true]){
 const options={region,profile:"post",contentOnly};
 add("widgets",{widgets:specifications,catalog,options},widgets.resolveRegionWidgets(specifications,catalog,options));
 for(const specification of specifications)add("widget",{specification,catalog,options,index:4},widgets.resolveWidget(specification,catalog,{...options,index:4}));
}
for(const profile of[null,"","wiki","blog_index"])add("widgets",{widgets:["missing","toc","custom"],catalog,options:{region:"topbar",profile}},widgets.resolveRegionWidgets(["missing","toc","custom"],catalog,{region:"topbar",profile}));
for(const list of[undefined,null,false,"toc",{},["toc","toc","missing","tree"]])add("widgets",{widgets:list,catalog,options:{region:"rightbar"}},widgets.resolveRegionWidgets(list,catalog,{region:"rightbar"}));
const global={topbar:{enabled:true,brand:{name:"Top",image:{src:"/top.png",variant:"icon"}},menu:[{id:"top",url:"/"}],widgets:["spacer","menu","toc"]},leftbar:{enabled:true,brand:{source:"site",name:"Site",tagline:"Global",search:true,style:"regular",image:{src:"/site.png",variant:"avatar"}},menu:[{id:"home",url:"/"}],menuColumns:4,footer:{actions:[{type:"link",icon:"home",url:"/"}]},widgets:["recent","tree"]},rightbar:{enabled:true,widgets:["toc"]}};
const sources={site:global.leftbar.brand,collection:{name:"Collection",tagline:"Docs",style:"compact",image:{src:"/docs.png",variant:"plain"},backButton:true},off:false,empty:null};
const overrides=[{},null,{enabled:false},{enabled:true},{brand:false},{brand:null},{brand:{name:null}},{brand:{image:{variant:"icon"}}},{brand:{image:{src:null}}},{brand:{source:"collection",tagline:null}},{brand:{source:"site",search:false}},{brand:{source:"off",name:"Reset"}},{brand:{source:"empty"}},{brand:{source:"unknown",name:"Keep"}},{menu:[]},{menu:null},{menu:[{id:"second"}]},{widgets:[]},{widgets:null},{widgets:["menu","settings","alias","custom","missing"]},{footer:{actions:[]}},{footer:{actions:null}},{menuColumns:null},{menuColumns:1}];
for(const region of["topbar","leftbar","rightbar"])for(const middle of overrides)for(const final of overrides){const layers=[global,{[region]:middle},{[region]:final}];const options={layers,catalog,brandSources:sources,profile:"wiki",defaultState:"collapsed"};add("regions",options,regions.resolveRegions(options));add("cascade",{layers,brandSources:sources},regions.cascadeRegions(layers,{brandSources:sources}));}
for(const options of[{}, {layers:[]},{layers:[global]},{layers:[global,{leftbar:{enabled:false,widgets:["missing"]},topbar:{enabled:false,widgets:["missing"]},rightbar:{enabled:false,widgets:["missing"]}}],catalog},{layers:[global],catalog,defaultState:""}])add("regions",options,regions.resolveRegions(options));
const bases=[null,false,true,{},global.leftbar.brand,sources.collection,{image:null},{image:{src:null,variant:"icon"}}];
const brandOverrides=[null,false,true,0,[],{},...overrides.filter(x=>x&&Object.hasOwn(x,"brand")).map(x=>x.brand),{image:null},{image:{src:false}},{source:"collection",image:{variant:"avatar"}}];
for(const base of bases)for(const override of brandOverrides)add("merge",{base,override,sources},regions.mergeBrand(base,override,sources));
for(const b of [...bases,...brandOverrides,{name:null,tagline:"Text",href:"/",other:true,image:{src:"/",variant:null,other:true}},{name:"NoImage",image:{}},{image:[]},{image:false}])add("brand",{brand:b},brand.normalizeBrand(b));
for(const profileKey of["home","blogIndex","wikiIndex","notebooks","notebook","post","wiki","note","author","page","error","settings","blog_index","wiki_index",null,""])add("mobile",{profileKey},brand.shouldShowMobileBrand({profileKey}));
const configs=[{}, {title:"Blog",subtitle:"Hello",avatar:"/avatar.png"},{title:null,subtitle:false,avatar:0},{title:"{config.subtitle}",subtitle:"{config.avatar}",avatar:"Final"},{title:"$$ $& $` $' $1"},{title:42,subtitle:["one",null,"two"],avatar:{src:"/a.png"}}];
for(const config of configs)for(const value of[null,false,42,"","{config.title}|{config.subtitle}|{config.avatar}","a{config.title}b{config.title}c","{config.other}"]){add("tokens",{config,value},brand.replaceConfigTokens(value,config));}
const profiles=[{}, {activeMenu:null},{activeMenu:"post"},{activeMenu:""}];for(const profile of profiles)add("navigation",{profile},layout.toRenderNavigation(profile));
for(const value of[null,12,"","/","///blog///","/blog/","/404.html","/file.xml","foo.bar/child","blog//"," /blog/ "])add("path",{value},{profilePath:layout.profilePath(value),generatorPath:layout.generatorPath(value)});
for(const configured of[{},global,{leftbar:{brand:false}},{leftbar:{brand:null}},{leftbar:{brand:{style:"",name:"Site"}}}]){
 add("siteBrand",{globalConfig:configured},layout.siteBrandModel(configured));
 for(const profile of[{}, {leftbar:{brand:{source:"collection",search:false},widgets:[]},rightbar:{enabled:false}},{leftbar:{brand:{source:"site"}},topbar:{brand:false}}])for(const defaults of[null,{leftbar:{brand:{style:"compact"},widgets:["tree"]}}]){
 const options={brandSources:sources,layers:[{leftbar:{menu:[]}},{}]};add("layout",{globalConfig:configured,profile,defaults,options},layout.toRenderRegions(configured,profile,defaults,options));}
}
const inputs=["scripts/lib/regions.js","scripts/lib/widget-registry.js","scripts/lib/layout-config.js","scripts/lib/brand.js","_data/widgets.yml"];
const metadata={tag:"2.0.0",commit:"9d11978a24fe6610e3a7dbab741cf62634539d05",inputHashes:Object.fromEntries(inputs.map(name=>[name,crypto.createHash("sha256").update(fs.readFileSync(path.join(root,name))).digest("hex")]))};
fs.writeFileSync(path.join(__dirname,"../testdata/regions-oracle.json.gz"),zlib.gzipSync(JSON.stringify({metadata,fixtures}),{level:9}));
console.log(JSON.stringify({fixtures:fixtures.length,kinds:[...new Set(fixtures.map(f=>f.kind))]},null,2));
