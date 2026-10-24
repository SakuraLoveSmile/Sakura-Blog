import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import { execFileSync } from 'node:child_process';
import { mount as runtimeMount } from '../web/assets/js/runtime/extensions/blog-search.js';
const script = process.env.BLOG_SEARCH_BASELINE === '1'
 ? execFileSync('git', ['show', 'c4f4029:modules/stellar/web/assets/js/blog/search/local-search.js'], {encoding:'utf8'})
 : readFileSync(new URL('../web/assets/js/blog/search/local-search.js', import.meta.url), 'utf8');
const dialog = readFileSync(new URL('../web/templates/partial/stellar/search_dialog.html', import.meta.url), 'utf8');
const tick = () => new Promise(resolve => setImmediate(resolve));
class Element extends EventTarget {
 constructor(tag='div',doc){super();this.tagName=tag.toUpperCase();this.nodeType=1;this.ownerDocument=doc;this.children=[];this.attrs=new Map();this.className='';this.style={};this.value='';this.dataset={};this._text='';this.parentNode=null;}
 set textContent(value){this._text=String(value);this.children=[];}get textContent(){return this._text+this.children.map(child=>child.textContent).join('');}
 set innerHTML(value){this._text=String(value);this.children=[];}get innerHTML(){return this.textContent;}
 appendChild(node){if(node.nodeType===11){for(const child of [...node.children])this.appendChild(child);return node;}node.parentNode=this;this.children.push(node);return node;}
 replaceChildren(...nodes){this.children=[];this._text='';nodes.forEach(node=>this.appendChild(node));}
 setAttribute(k,v){this.attrs.set(k,String(v));}getAttribute(k){return this.attrs.get(k)||null;}
 get classList(){const node=this;return {contains:value=>node.className.split(/\s+/).includes(value),add(value){node.className+=' '+value;},remove(value){node.className=node.className.split(/\s+/).filter(name=>name!==value).join(' ');},toggle(value,on){const values=new Set(node.className.split(/\s+/));on?values.add(value):values.delete(value);node.className=[...values].join(' ');}};}
 matches(selector){if(selector.startsWith('[')){const match=selector.match(/^\[([^=\]]+)(?:="([^"]*)")?\]$/);return match&&this.attrs.has(match[1])&&(match[2]===undefined||this.getAttribute(match[1])===match[2]);}if(selector.startsWith('#'))return this.id===selector.slice(1);if(selector.startsWith('.'))return this.classList.contains(selector.slice(1));return this.tagName.toLowerCase()===selector;}
 closest(selector){for(let node=this;node;node=node.parentNode)if(node.matches(selector))return node;return null;}
 querySelectorAll(selector){const parts=selector.split(' '),found=[];const visit=node=>{for(const child of node.children){if(child.matches(parts.at(-1))&&(parts.length===1||child.parentNode.closest(parts[0])))found.push(child);visit(child);}};visit(this);return found;}
 querySelector(selector){return this.querySelectorAll(selector)[0]||null;}
}
function fixture({content=true,ttl=60,scope='all',path='/sub/search.json',interactiveSpotlight='ui-interactive card-hover card-hover--spotlight'}={}) {
 const document=new Element('document');document.nodeType=9;document.baseURI='https://fixture.test/sub/';document.createElement=tag=>new Element(tag,document);
 const wrappers=[];
 function wrapperFromMarkup(markup) {
  function element(cls) {
   const tag=markup.match(new RegExp('<([a-z]+)[^>]*class="[^"]*\\b'+cls+'\\b[^\"]*"[^>]*>'))?.[0];assert.ok(tag,cls+' must exist in real template');
   const node=document.createElement(tag.match(/^<(\w+)/)[1]);node.className=tag.match(/class="([^"]*)"/)[1];node.id=tag.match(/id="([^"]*)"/)?.[1]||'';return node;
  }
  const wrapper=element('search-wrapper'),input=element('search-input'),result=element('search-result');wrapper.appendChild(input);wrapper.appendChild(result);document.appendChild(wrapper);wrappers.push({wrapper,input,result});return wrappers.at(-1);
 }
 const main=wrapperFromMarkup(dialog);
 const requests=[],values=new Map(),observers=[];const storage={getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value),removeItem:key=>values.delete(key)};
 const hover={active:new Set(),mounts:[],unmounts:[]};
 const stellar={cardHover:{mountAll(root){const links=root.querySelectorAll('a');hover.mounts.push(links);links.forEach(link=>{assert.equal(hover.active.has(link),false,'同一结果不得重复挂载hover');hover.active.add(link);});},unmountAll(root){const links=root.querySelectorAll('a');hover.unmounts.push(links);links.forEach(link=>hover.active.delete(link));}}};
 const window={stellar};document.defaultView=window;const sandbox={window,document,stellar,ctx:{root:'/sub/',ui:{classes:{interactiveSpotlight}},search:{path,local_search:{content,field:scope,lazy_load:true,cache_ttl:ttl}}},URLSearchParams,AbortController,Event,WeakMap,Set,console,localStorage:storage,MutationObserver:class{constructor(callback){this.callback=callback;observers.push(this);}observe(){}disconnect(){this.disconnected=true;}},fetch:(url,options)=>new Promise(resolve=>requests.push({url,options,resolve}))};
 vm.runInNewContext(script,sandbox);
 const provider=window.blogLocalSearch,cache={key:(url,base)=>'search_cache_v6:'+new URL(url,base).href};
 const mount=()=>provider.mount(document,cache);
 const focus=(input=main.input)=>input.dispatchEvent(new Event('focus'));
 const query=(value,input=main.input)=>{input.value=value;input.dispatchEvent(new Event('input'));};
 async function respond(index,json,ok=true){requests[index].resolve({ok,status:ok?200:503,json:async()=>json});await tick();await tick();}
 return {...main,window,document,requests,values,observers,provider,mount,focus,query,respond,wrapperFromMarkup,hover,interactiveSpotlight};
}
test('真实strict search_dialog DOM输入会请求Go API，子目录只拼一次，全部profile类型显示',async()=>{
 const f=fixture();assert.equal(f.input.id,'site-search-input');const first=f.mount();assert.equal(f.mount(),first);f.focus();f.query('needle');assert.equal(f.requests.length,1);
 const url=new URL(f.requests[0].url,'https://fixture.test');assert.equal(url.pathname,'/sub/api/search');assert.deepEqual(Object.fromEntries(url.searchParams),{q:'needle',limit:'20',scope:'all',include_content:'true',domain:''});
 await f.respond(0,{items:[{type:'post',title:'文章',path:'/sub/post/a'},{type:'page',title:'页面',path:'/sub/page/a'},{type:'wiki',title:'知识库内容',path:'/sub/wiki/a'},{type:'note',title:'笔记内容',path:'/sub/notebooks/a'}]});
 assert.deepEqual(f.result.querySelectorAll('a').map(node=>node.href),['/sub/post/a?kw=needle','/sub/page/a?kw=needle','/sub/wiki/a?kw=needle','/sub/notebooks/a?kw=needle']);first();assert.ok(f.observers.every(observer=>observer.disconnected));
});
test('sidebar与dialog分别搜索并更新自己的状态，domain/关闭正文/scope参数精确传递',async()=>{
 const f=fixture({scope:'page',content:false});const side=f.wrapperFromMarkup('<div class="search-wrapper" id="widget-wrapper"><input class="search-input" id="widget-input"><div class="search-result" id="widget-result"></div></div>');const cleanup=f.mount();
 side.input.setAttribute('data-domain','wiki:guide');f.focus(side.input);f.query('Guide',side.input);const url=new URL(f.requests[0].url,'https://fixture.test');assert.equal(url.searchParams.get('domain'),'wiki:guide');assert.equal(url.searchParams.get('scope'),'page');assert.equal(url.searchParams.get('include_content'),'false');
 await f.respond(0,{items:[{type:'wiki',title:'Guide',path:'/sub/wiki/guide/start'}]});assert.equal(side.result.querySelectorAll('a').length,1);assert.equal(f.result.querySelectorAll('a').length,0);assert.equal(side.wrapper.getAttribute('searching'),'true');assert.equal(f.wrapper.getAttribute('searching'),null);cleanup();
});
test('快速换词/清空/卸载取消请求，迟到响应不写DOM，重复挂载没有叠加监听',async()=>{
 const f=fixture();const cleanup=f.mount();f.mount();f.focus();f.query('first');f.query('second');assert.equal(f.requests.length,2);assert.equal(f.requests[0].options.signal.aborted,true);f.query('');assert.equal(f.requests[1].options.signal.aborted,true);assert.equal(f.wrapper.getAttribute('searching'),'false');
 await f.respond(0,{items:[{type:'post',title:'Late',path:'/late'}]});assert.equal(f.result.querySelectorAll('a').length,0);f.query('third');cleanup();assert.equal(f.requests[2].options.signal.aborted,true);await f.respond(2,{items:[{type:'post',title:'Late',path:'/late'}]});assert.equal(f.result.querySelectorAll('a').length,0);
 f.input.value='';const next=f.mount();f.focus();f.query('new');assert.equal(f.requests.length,4);next();
});
test('Go失败后索引fallback遵守domain/include_content，标题标签分类可搜且不伪造正文',async()=>{
 const f=fixture({content:false});const cleanup=f.mount();f.input.setAttribute('data-domain','wiki:guide');f.focus();f.query('Rocket');await f.respond(0,{},false);assert.equal(f.requests[1].url,'/sub/search.json');
 await f.respond(1,[{title:'Guide',tags:['Rocket'],path:'/sub/wiki/guide/start',domains:['wiki:guide'],content:'secret-body'},{title:'Rocket outside',path:'/outside',domains:['blog']}]);assert.deepEqual(f.result.querySelectorAll('a').map(node=>node.href),['/sub/wiki/guide/start?kw=Rocket']);assert.equal(f.result.querySelectorAll('.search-result-content').length,0);
 f.query('secret-body');assert.equal(f.result.querySelectorAll('a').length,0);assert.equal(f.requests.length,2);const [key,value]=f.values.entries().next().value;assert.equal(key,'search_cache_v6:https://fixture.test/sub/search.json');assert.equal(JSON.parse(value).ttl,60);cleanup();
});
test('索引JSON正文解码在卸载后到达不污染缓存，cache_ttl=0不落盘',async()=>{
 const f=fixture({ttl:0,path:'search.json'});const cleanup=f.mount();f.focus();f.query('word');await f.respond(0,{},false);let resolveBody;f.requests[1].resolve({ok:true,json:()=>new Promise(resolve=>{resolveBody=resolve;})});await tick();cleanup();assert.equal(f.requests[1].options.signal.aborted,true);resolveBody([{title:'word',path:'/late',content:'word'}]);await tick();await tick();assert.equal(f.values.size,0);assert.equal(f.result.querySelectorAll('a').length,0);
});
test('Go runtime重复并发挂载只载一批资源，卸载后清除搜索/快捷键/缓存事件',async()=>{
 const f=fixture();vm.runInNewContext(readFileSync(new URL('../web/assets/js/search/shortcut.js',import.meta.url),'utf8'),{window:f.window,document:f.document});const assets=[],callbacks=[];const context={signal:new AbortController().signal,assets:{script:async path=>assets.push(path)},onCleanup:callback=>callbacks.push(callback)};
 const [first,second]=await Promise.all([runtimeMount(f.document,context),runtimeMount(f.document,context)]);assert.equal(first,second);assert.equal(callbacks.length,1);assert.deepEqual(assets,['/assets/js/blog/search/local-search.js','/assets/js/search/shortcut.js']);f.focus();f.query('word');assert.equal(f.requests.length,1);first();second();assert.equal(f.requests[0].options.signal.aborted,true);f.query('another');assert.equal(f.requests.length,1);
});
test('partial navigation新增sidebar搜索框可按需接入，卸载也移除新增监听',async()=>{
 const f=fixture();const cleanup=f.mount();const side=f.wrapperFromMarkup('<div class="search-wrapper" id="new-widget"><input class="search-input" id="new-input"><div class="search-result" id="new-result"></div></div>');
 const event=new Event('focusin');Object.defineProperty(event,'target',{value:side.input});f.document.dispatchEvent(event);f.query('new sidebar',side.input);assert.equal(f.requests.length,1);await f.respond(0,{items:[{type:'wiki',title:'New sidebar',path:'/sub/wiki/new'}]});assert.equal(side.result.querySelectorAll('a').length,1);cleanup();f.query('next',side.input);assert.equal(f.requests.length,1);assert.ok(f.observers.every(observer=>observer.disconnected));
});
test('TTL过期会刷新索引，关闭正文不复用其他搜索策略的存量缓存',async()=>{
 const f=fixture({content:false,ttl:60});f.values.set('search_cache_v6:https://fixture.test/sub/search.json',JSON.stringify({ts:Date.now(),ttl:86400,scope:'all',content:true,data:[{title:'Rocket secret',path:'/cached'}]}));const cleanup=f.mount();f.focus();f.query('Rocket');await f.respond(0,{},false);assert.equal(f.requests.length,2);await f.respond(1,[{title:'Rocket',path:'/current'}]);assert.equal(f.result.querySelector('a').href,'/current?kw=Rocket');cleanup();
 const [key,raw]=f.values.entries().next().value;const entry=JSON.parse(raw);entry.ts=Date.now()-61000;f.values.set(key,JSON.stringify(entry));f.input.value='';const next=f.mount();f.focus();f.query('Rocket');assert.equal(f.requests.length,3);assert.equal(f.requests[2].url,'/sub/search.json');next();assert.equal(f.requests[2].options.signal.aborted,true);
});
test('服务端标题与章节摘要共用结果链接，无摘要仍有可点击名称并消费UI类',async()=>{
 const f=fixture({interactiveSpotlight:'fixture-interactive fixture-spotlight'});const cleanup=f.mount();f.focus();f.query('Needle');
 await f.respond(0,{items:[{type:'post',title:'<img src=x> Needle',path:'/sub/post/title-only'},{type:'wiki',title:'Guide',path:'/sub/wiki/guide',anchor_id:'section one',anchor_text:'Section one',snippet:'<span class="search-keyword">Needle</span>'}]});
 const links=f.result.querySelectorAll('a');assert.equal(links.length,2);
 assert.equal(links[0].querySelector('.search-result-title')?.textContent,'<img src=x> Needle');assert.equal(links[0].textContent,'<img src=x> Needle');assert.equal(links[0].href,'/sub/post/title-only?kw=Needle');
 assert.equal(links[1].querySelector('.search-result-title')?.textContent,'Guide');assert.equal(links[1].querySelector('.search-result-section')?.textContent,'Section one');assert.equal(links[1].href,'/sub/wiki/guide?kw=Needle#section%20one');
 assert.ok(links.every(link=>link.className===f.interactiveSpotlight));cleanup();
});
test('fallback标题或标签命中无正文时链接仍包含标题并消费同一UI类',async()=>{
 const f=fixture({content:false,interactiveSpotlight:'fixture-interactive fixture-spotlight'});const cleanup=f.mount();f.focus();f.query('Needle');await f.respond(0,{},false);
 await f.respond(1,[{title:'Needle title',path:'/sub/post/title-only'},{title:'Tagged guide',tags:['Needle'],path:'/sub/wiki/tagged',content:'正文不参与搜索'}]);
 const links=f.result.querySelectorAll('a');assert.deepEqual(links.map(link=>link.querySelector('.search-result-title')?.textContent),['Needle title','Tagged guide']);assert.deepEqual(links.map(link=>link.textContent),['Needle title','Tagged guide']);assert.ok(links.every(link=>link.className===f.interactiveSpotlight));assert.equal(f.result.querySelectorAll('.search-result-content').length,0);assert.equal(f.hover.active.size,2);cleanup();
});
test('服务端结果与类型切换卸载旧hover，每批新链接只挂载一次，空类型清空交互',async()=>{
 const f=fixture();const cleanup=f.mount();f.focus();f.query('Needle');await f.respond(0,{items:[{type:'post',title:'Post Needle',path:'/sub/post/a'},{type:'page',title:'Page Needle',path:'/sub/page/a'}]});
 const initialLinks=f.result.querySelectorAll('a');assert.equal(f.hover.mounts.length,1);assert.equal(f.hover.active.size,2);
 f.result.querySelectorAll('button').find(button=>button.textContent==='文章').dispatchEvent(new Event('click'));
 const postLinks=f.result.querySelectorAll('a');assert.equal(postLinks.length,1);assert.equal(f.hover.mounts.length,2);assert.ok(initialLinks.every(link=>!f.hover.active.has(link)));assert.equal(f.hover.active.has(postLinks[0]),true);
 f.result.querySelectorAll('button').find(button=>button.textContent==='文章').dispatchEvent(new Event('click'));assert.equal(f.hover.mounts.length,2,'再次点击当前类型不重复挂载');
 f.result.querySelectorAll('button').find(button=>button.textContent==='笔记').dispatchEvent(new Event('click'));assert.equal(f.result.querySelectorAll('a').length,0);assert.equal(f.hover.active.size,0);assert.equal(f.hover.mounts.length,2,'空类型不创建hover');
 f.result.querySelectorAll('button').find(button=>button.textContent==='全部').dispatchEvent(new Event('click'));assert.equal(f.hover.mounts.length,3);assert.equal(f.hover.active.size,2);f.query('');assert.equal(f.hover.active.size,0);cleanup();
});
