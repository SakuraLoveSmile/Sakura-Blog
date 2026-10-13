// 仅开发使用：从锁定的官方 Stylus 源编译、提取差分 token。
// 用法：node oracle.cjs <shared-upstream-git-repository>
const fs=require('node:fs'),path=require('node:path'),os=require('node:os'),cp=require('node:child_process');
const stylus=require('../../web/node_modules/stylus');
const commit='9d11978a24fe6610e3a7dbab741cf62634539d05';
const repository=path.resolve(process.argv[2]);
const tmp=fs.mkdtempSync(path.join(os.tmpdir(),'stellar-appearance-oracle-'));
const env={...process.env};for(const key of ['GIT_DIR','GIT_WORK_TREE','GIT_COMMON_DIR','GIT_INDEX_FILE','GIT_PREFIX','GIT_OBJECT_DIRECTORY','GIT_ALTERNATE_OBJECT_DIRECTORIES'])delete env[key];
cp.execFileSync('tar',['-x','-C',tmp],{input:cp.execFileSync('git',['archive',commit,'source/css'],{cwd:repository,env})});
const defaults=JSON.parse(fs.readFileSync(path.resolve(__dirname,'../../web/build/stylus/config.json'),'utf8'));
function merge(a,b){const result=structuredClone(a);for(const[k,v]of Object.entries(b)){result[k]=v&&typeof v==='object'&&!Array.isArray(v)?merge(result[k]||{},v):v;}return result;}
function value(v){const n=stylus.nodes;if(v==null)return n.null;if(typeof v==='string')return new n.String(v);if(typeof v==='number')return new n.Unit(v);if(typeof v==='boolean')return new n.Boolean(v);const a=new n.Expression();v.forEach(x=>a.push(value(x)));return a;}
function render(config){const filename=path.join(tmp,'source/css/main.styl');return new Promise((resolve,reject)=>stylus(fs.readFileSync(filename,'utf8')).set('filename',filename).set('paths',[path.dirname(filename)]).define('hexo-config',param=>{let v=config;for(const key of param.val.split('.'))v=v?.[key];return value(v)}).render((err,css)=>err?reject(err):resolve(css)));}
function normalizeSelector(s){return s.trim().replace(/"/g,"'").replace(/\s+/g,' ').replace(/\s*,\s*/g,',');}
function tokens(css){const output={};const rules=/([^{}]+)\{([^{}]*)\}/g;let m;while((m=rules.exec(css))){const selector=normalizeSelector(m[1]);if(selector.startsWith('@'))continue;const declarations={};for(const declaration of m[2].split(';')){const i=declaration.indexOf(':');if(i>0)declarations[declaration.slice(0,i).trim()]=declaration.slice(i+1).trim()};Object.assign(output[selector]??={},declarations);}return output;}
const fixtures=[
 {name:'defaults',input:{}},
 {name:'glass-gradient',input:{appearance:{preset:'glass',backgrounds:{leftbar:{opacity:.625,backdrop:{radius:'84px'},gradient:{light:['#111','#222','#333','#444'],dark:['#555','#666','#777','#888']}}}}}},
 {name:'glass-image',input:{appearance:{preset:'glass',backgrounds:{leftbar:{type:'image',image:'/background.jpg',opacity:.8,backdrop:{radius:'42px'}},page:{image:'/page.jpg',backdrop:{radius:'32px',saturation:'175%',overlay:'rgba(20, 30, 40, .6)'}}}}}},
 {name:'glass-none',input:{appearance:{preset:'glass',backgrounds:{leftbar:{type:'none'}}}}},
 {name:'glass-null-image-fallback',input:{appearance:{preset:'glass',backgrounds:{leftbar:{type:'image',image:null}}}}},
 {name:'minimal-hex-named-rgba',input:{appearance:{preset:'minimal',colors:{primary:'#369',accent:'rebeccapurple',link:'rgba(30, 80, 120, .4)'}}}},
 {name:'flat-rgb-hsla',input:{appearance:{preset:'flat',colors:{primary:'rgb(34, 79, 121)',accent:'hsla(274, 43%, 58%, .25)',link:'#224466aa'}}}},
 {name:'negative-hue',input:{appearance:{colors:{primary:'hsl(-30, 60%, 40%)',accent:'transparent',link:'red'}}}},
 {name:'rgb-percent',input:{appearance:{colors:{primary:'rgb(20%, 45%, 80%)',accent:'#abcd',link:'#112233'}}}},
 {name:'all-parameters',input:{appearance:{preset:'glass',typography:{font_smoothing:'none',font_family:{body:'Georgia, serif',code:'Consolas, monospace'},font_size:{root:'18px',inline_code:'92%',code_block:'0.9rem'},font_weight:{100:120,200:240,300:350,400:450,500:650,600:670,700:820,800:850,900:970},content_align:'justify'},shape:{corner:'bevel',radius:{card_large:'30px',card:'22px',card_small:'14px',bar:'9px',image_large:'32px',image:'18px',image_small:'6px'}},gradients:{primary_action:'linear-gradient(to right, red, blue)',search_bar:'linear-gradient(to right, lime, gold)'},code_block:{scrollbar_width:'7px',highlight_stylesheet:null}},tags:{checkbox:{interactive:true},note:{border:false}}}},
];
const targets=[':root',".colorful[color='theme']",".colorful[color='accent']",":root[data-theme='dark'] .colorful[color='theme']",":root[data-theme='dark'] .colorful[color='accent']",":root:not([data-theme]) .colorful[color='theme']",":root:not([data-theme]) .colorful[color='accent']",":root[data-theme='dark'] .tag-plugin.tag",'body > .site-background','body > .site-background .site-background__blur','.site-region--leftbar .site-region__decoration',":root[data-theme='dark'] .site-region--leftbar .site-region__decoration",':root:not([data-theme]) .site-region--leftbar .site-region__decoration',"html[data-leftbar-state='collapsed'] .site-shell:not([data-drawer='leftbar']) .site-region--leftbar .site-region__decoration"];
(async()=>{try{for(const fixture of fixtures){const all=tokens(await render(merge(defaults,fixture.input)));fixture.tokens=Object.fromEntries(targets.filter(key=>all[key]).map(key=>[key,all[key]]));}fs.writeFileSync(path.resolve(__dirname,'../testdata/stylus-oracle.json'),JSON.stringify({commit,fixtures},null,2)+'\n');}finally{fs.rmSync(tmp,{recursive:true,force:true})}})().catch(error=>{console.error(error);process.exitCode=1});
