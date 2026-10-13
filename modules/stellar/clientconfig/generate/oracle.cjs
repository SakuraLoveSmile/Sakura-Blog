// 固定 Stellar 2.0.0 官方输入与行为；生产只消费 Go 实现和静态资源默认值。
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const root=path.resolve(process.argv[2]),out=path.resolve(process.argv[3]);
const load=name=>require(path.join(root,name));
const {parseStellarConfig}=load('scripts/lib/config-schema');
const {CONFIG_DEFAULTS}=load('scripts/schema/config-schema');
const {buildBrowserRuntimeManifest}=load('scripts/lib/browser-runtime');
const {resolveCommentsModel,normalizeThemeComments}=load('scripts/lib/comments');
const {clientAssets,resetClientAssets}=load('scripts/lib/client-assets');
const INTERNAL=load('scripts/lib/internal-constants');
const {processNavigation}=load('scripts/lib/partial-navigation');
const {pageViewModelsFor}=load('scripts/lib/page-view-model-registry');
const {load:htmlLoad}=require('cheerio');
const deep=(a,b)=>{const r=structuredClone(a||{});for(const[k,v]of Object.entries(b||{})){if(v==null)continue;r[k]=v&&typeof v==='object'&&!Array.isArray(v)&&r[k]&&typeof r[k]==='object'&&!Array.isArray(r[k])?deep(r[k],v):structuredClone(v)}return r};
const rewrite=v=>typeof v==='string'&&/^\/(js|css)\//.test(v)?'/assets'+v:Array.isArray(v)?v.map(rewrite):v&&typeof v==='object'?Object.fromEntries(Object.entries(v).map(([k,x])=>[k,rewrite(x)])):v;
const yaml=require('js-yaml');
const languages={};for(const language of ['zh-CN','zh-TW','en']){const t=yaml.load(fs.readFileSync(path.join(root,'languages/'+language+'.yml'),'utf8'));languages[language]={copy:{label:t.btn.copy,copied:t.message.copied,denied:t.message.copy_denied,unsupported:t.message.copy_unsupported},colorScheme:t.message.color_scheme_switched,dateSuffix:Object.fromEntries(['just','min','hour','day'].map(key=>[key,t.meta.date_suffix[key]])),search:{more:t.meta.more,no_more:t.search.no_more}}}
const sourceFiles=['_config.yml','scripts/lib/internal-constants.js','scripts/lib/browser-runtime.js','scripts/lib/contribution-registry.js','scripts/lib/client-assets.js','scripts/lib/comments.js','scripts/lib/partial-navigation.js','scripts/lib/resource-assets.js','scripts/helpers/extension_assets.js','layout/_partial/scripts/runtime.ejs',...['zh-CN','zh-TW','en'].map(id=>'languages/'+id+'.yml')];
fs.writeFileSync(path.join(path.dirname(out),'../resources.json'),JSON.stringify({tag:'2.0.0',commit:'9d11978a24fe6610e3a7dbab741cf62634539d05',inputHashes:Object.fromEntries(sourceFiles.map(file=>[file,crypto.createHash('sha256').update(fs.readFileSync(path.join(root,file))).digest('hex')])),constants:INTERNAL,defaults:CONFIG_DEFAULTS,languages},null,2)+'\n');
const fixtures=[];
function add(name,raw={},page={},collection={},profile='post'){
 const theme=parseStellarConfig({themeConfig:raw});
 const messages={copy:{label:'Copy',copied:'Copied',denied:'Denied',unsupported:'Unsupported'},colorScheme:{light:'Light',dark:'Dark',auto:'Auto'},dateSuffix:{just:'Just',min:'m',hour:'h',day:'d'},search:{more:'More',no_more:'End'}};
 const input={Theme:theme,Page:page,Collection:collection,Profile:profile,Root:'/sub/',PageURL:'https://fixture.test/sub/page/',SiteURL:'https://fixture.test/sub/',SiteTitle:'Site',Data:{title:'Page'},Messages:messages,Icons:{'default:link':'<!-- remove --><svg>link</svg>'}};
 let overrides=deep(collection.comments,page.comments);
 if(profile==='home'||profile==='error'){const v=theme.profiles[profile].comments;if(v)overrides=deep(typeof v==='object'?v:{enabled:true},overrides)}
 const comments=resolveCommentsModel(theme,overrides,'Page');
 const manifest=buildBrowserRuntimeManifest({root:input.Root,extensions:theme,assets:INTERNAL.assets,render:page.render||{},comments,colorScheme:theme.appearance.colorScheme,messages});
 const hexo={stellar:{config:theme,data:{icons:input.Icons}},config:{root:input.Root}};resetClientAssets();
 const clientData=JSON.parse(clientAssets(hexo).js.data.match(/^window.stellarClientData=(.*);\nwindow.stellarIcons=/s)[1]);
 clientData.ctx.date_suffix=messages.dateSuffix;if(clientData.ctx.search)Object.assign(clientData.ctx.search,messages.search);clientData.canonical.param={permalink:input.PageURL,checklink:rewrite(clientData.ctx.services.video.js)};
 const expectedManifest=rewrite(manifest);for(const [id,module]of [['services','blog-upstream-services'],['page-controls','blog-page-controls'],['lazy-loading','blog-lazy-loading']])expectedManifest.extensions.find(e=>e.id===id).module='/assets/js/runtime/extensions/'+module+'.js';
 fixtures.push({name,input,manifest:expectedManifest,clientData:rewrite(clientData),comments:rewrite(comments)});
}
add('default');
for(const scheme of ['auto','light','dark'])for(const preset of ['card','glass','minimal','flat'])add('appearance-'+scheme+'-'+preset,{appearance:{color_scheme:scheme,preset}});
for(const flag of ['partial_navigation','color_scheme_switch','lightbox','link_prefetch','reveal','heti'])for(const enabled of [true,false])add('feature-'+flag+'-'+enabled,{features:{[flag]:{enabled}}});
add('all-custom-assets',{search:{provider:'algolia',algolia:{js:'js/custom-algolia.js',appId:'app',apiKey:'key',indexName:'index',custom:{nested:42}}},features:{swiper:{js:'./js/swiper.js',css:'css/swiper.css'},lazy_loading:{js:null,transition:'blur'},lightbox:{enabled:true,js:'js/lightbox.js',css:'css/lightbox.css',selector:'aside:has(a[href])'},link_prefetch:{js:'js/prefetch.js'},heti:{enabled:true,js:'js/heti.js',css:'css/heti.css'},math:{provider:'mathjax',mathjax:{js:'js/math.js',tex:{macros:{x:'X'}}}},diagrams:{provider:'mermaid',mermaid:{js:'js/diagram.js',theme:'forest'}},card_hover:{spotlight:true,tilt:true}},services:{markdown:{js:'js/marked.js'},site_info:{site_info_api:{endpoint:'https://info.test?url={href}'}}}});
for(const provider of ['local','algolia',null])add('search-'+provider,{search:{provider}});
for(const provider of ['katex','mathjax',null])add('math-'+provider,{features:{math:{provider}}});
add('katex-custom-sri',{features:{math:{provider:'katex',katex:{css:'css/custom.css',css_integrity:'sha384-demo'}}}});
add('diagrams-page-override',{features:{diagrams:{provider:'mermaid',mermaid:{theme:'forest',js:'js/a.js'}}}},{render:{diagrams:{js:'ignored.js',flowchart:{htmlLabels:false},theme:'dark'}}});
add('diagrams-page-disabled',{features:{diagrams:{provider:'mermaid'}}},{render:{diagrams:false}});
add('math-page-override',{features:{math:{provider:'mathjax'}}},{render:{math:'katex'}});
for(const provider of ['beaudar','utterances','giscus','twikoo','waline','artalk',null]){
 add('comments-'+provider,{comments:{provider},appearance:{color_scheme:'dark'}});
 if(provider)add('comments-override-'+provider,{comments:{provider}},{comments:{title:'Page comments',id:'custom',options:{js:'js/custom-comments.js',css:'css/custom-comments.css',meta_css:'css/custom-meta.css',custom:{enabled:false}}}});
}
add('comments-artalk-derived',{comments:{provider:'artalk',artalk:{server:' https://comments.test/ ',js:null,css:null}}});
add('comments-disabled',{comments:{provider:'waline'}},{comments:{enabled:false}});
add('comments-inheritance',{comments:{provider:'giscus'}},{comments:{options:{custom:{second:2},js:null}}},{comments:{id:'collection',options:{custom:{first:1},js:'js/collection.js'}}});
add('home-comments',{comments:{provider:'waline'},profiles:{home:{comments:{enabled:true,options:{lang:'en'}}}}},{},{},'home');
const manifest={version:1,root:'/sub/',policy:INTERNAL.runtime,extensions:[{id:'document',module:'/assets/js/runtime/extensions/partial-navigation.js',scope:'document',when:{always:true},config:{}},{id:'persistent',module:'/assets/js/runtime/extensions/page-controls.js',scope:'region',when:{selector:'.persistent:has(a[href])'},config:{setting:1}},{id:'page',module:'/assets/js/runtime/extensions/reveal.js',scope:'region',when:{selector:'.content'},config:{duration:100}}]};
const baseHTML=(body,extra='',current='one')=>`<!doctype html><html><head><title data-stellar-page-meta>${current}</title><link data-stellar-page-style href="/${current}.css"><style id="shared">shared</style></head><body data-page-type="${current}"><div class="site-shell" data-regions="leftbar rightbar"><aside id="leftbar-region"><div class="persistent"><a href="two/" class="ui-collection__item${current==='one'?' is-active':''}" ${current==='one'?'aria-current="page"':''}>Two${current==='one'?'<span class="ui-collection__indicator"></span>':''}</a></div></aside><main id="main" class="content">${body}</main><aside id="rightbar-region">${current}</aside></div><script data-stellar-script="defines">page('${current}')</script><script data-stellar-script="core" type="application/json" id="stellar-runtime-config">${JSON.stringify(manifest)}</script>${extra}</body></html>`;
const navCases=[];for(const[name,html]of [['one',baseHTML('one')],['two',baseHTML('two','','two')],['shell-change',baseHTML('one').replace('shared</style>','changed</style>')],['unmanaged',baseHTML('one','<script>run()</script>')],['json-safe',baseHTML('one','<script type="application/ld+json">{}</script>')],['links',baseHTML('<a href="two/">same</a><a href="/sub/other/">other</a><a href="https://else.test/sub/two/">external</a><a href="%XX">invalid</a>')]]){
 const hexo={config:{root:'/sub/',url:'https://fixture.test/sub/'},stellar:{config:{features:{partialNavigation:{enabled:true}}}}};const registry=pageViewModelsFor(hexo);const page={path:'one/index.html',permalink:'https://fixture.test/sub/one/'};registry.setPageViewModel(page,{collection:{profile:'wiki',id:'docs'},item:{route:{permalink:page.permalink}}});registry.setNavigationMembers([{page:{path:'one/two/index.html'},profile:'wiki',collectionId:'docs'},{page:{path:'other/index.html'},profile:'wiki',collectionId:'other'}]);
 const output=processNavigation.call(hexo,html,{page});const $=htmlLoad(output);const nav=JSON.parse($('#stellar-navigation-config').text()||'null');navCases.push({name,html,options:{CollectionProfile:'wiki',CollectionID:'docs',PageURL:page.permalink,Root:'/sub/',Members:{'/sub/one/two/':JSON.stringify(['wiki','docs']),'/sub/other/':JSON.stringify(['wiki','other'])}},eligible:!!nav,marked:$('a[data-stellar-navigation]').map((_,n)=>$(n).attr('href')).get(),signature:nav?.signature||null});
}
const corpus=JSON.stringify({tag:'2.0.0',commit:'9d11978a24fe6610e3a7dbab741cf62634539d05',fixtures,navigation:navCases},null,2)+'\n';fs.writeFileSync(out,out.endsWith('.gz')?require('node:zlib').gzipSync(corpus):corpus);
