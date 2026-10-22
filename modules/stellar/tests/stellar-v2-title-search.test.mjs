import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import { mount as runtimeMount } from '../web/assets/js/runtime/extensions/blog-title-search.js';
const source=readFileSync(new URL('../web/assets/js/blog/search/title-search.js',import.meta.url),'utf8');
const stock=readFileSync(new URL('../web/assets/js/search/local-search.js',import.meta.url),'utf8');
const tick=()=>new Promise(resolve=>setImmediate(resolve));
class Element extends EventTarget {
 constructor(tag='div',doc){super();this.tagName=tag.toUpperCase();this.nodeType=1;this.ownerDocument=doc;this.children=[];this.attrs=new Map();this.className='';this.style={};this.value='';this.dataset={};this._text='';this.parentNode=null;}
 set textContent(value){this._text=String(value);this.children=[];}get textContent(){return this._text+this.children.map(child=>child.textContent).join('');}
 set innerHTML(value){assert.equal(value,'');this._text='';this.children=[];}get innerHTML(){return this.textContent;}
 appendChild(node){if(node.nodeType===11){for(const child of [...node.children])this.appendChild(child);return node;}node.parentNode=this;this.children.push(node);return node;}
 replaceChildren(...nodes){this.children=[];this._text='';nodes.forEach(node=>this.appendChild(node));}
 setAttribute(k,v){this.attrs.set(k,String(v));}getAttribute(k){return this.attrs.get(k)||null;}
 get classList(){const node=this;return {contains:value=>node.className.split(/\s+/).includes(value),toggle(value,on){const values=new Set(node.className.split(/\s+/));on?values.add(value):values.delete(value);node.className=[...values].join(' ');}};}
 matches(selector){if(selector.startsWith('[')){const match=selector.match(/^\[([^=\]]+)(?:="([^"]*)")?\]$/);return match&&this.attrs.has(match[1])&&(match[2]===undefined||this.getAttribute(match[1])===match[2]);}if(selector.startsWith('#'))return this.id===selector.slice(1);if(selector.startsWith('.'))return this.classList.contains(selector.slice(1));return this.tagName.toLowerCase()===selector;}
 closest(selector){for(let node=this;node;node=node.parentNode)if(node.matches(selector))return node;return null;}
 querySelectorAll(selector){const parts=selector.split(' '),found=[];const visit=node=>{for(const child of node.children){if(child.matches(parts.at(-1))&&(parts.length===1||child.parentNode.closest(parts[0])))found.push(child);visit(child);}};visit(this);return found;}
 querySelector(selector){return this.querySelectorAll(selector)[0]||null;}
}
function fixture({content=false,ttl=60,index,script=source}={}){
 const document=new Element('document');document.nodeType=9;document.baseURI='https://fixture.test/sub/';document.createElement=tag=>new Element(tag,document);document.createTextNode=value=>{const node=new Element('text',document);node.textContent=value;return node;};document.createDocumentFragment=()=>{const node=new Element('fragment',document);node.nodeType=11;return node;};
 const wrapper=document.createElement('div');wrapper.className='search-wrapper';wrapper.id='search-wrapper';const input=document.createElement('input');input.className='search-input';input.id='search-input';const result=document.createElement('div');result.className='search-result';result.id='search-result';wrapper.appendChild(input);wrapper.appendChild(result);document.appendChild(wrapper);
 const values=new Map(),requests=[],timers=new Map(),observers=[];let timerID=0,clock=0;
 const storage={getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value),removeItem:key=>values.delete(key)};
 const window={};const sandbox={window,document,localStorage:storage,ctx:{root:'/sub/',ui:{classes:{interactive:'ui-interactive',interactiveSpotlight:'spotlight'}},search:{path:'/search.json',more:'More',no_more:'End',local_search:{content,lazy_load:true,cache_ttl:ttl}}},AbortController,Event,Set,WeakMap,performance:{now:()=>clock++},console,setTimeout:fn=>{timers.set(++timerID,fn);return timerID;},clearTimeout:id=>timers.delete(id),MutationObserver:class{constructor(fn){this.fn=fn;observers.push(this);}observe(){}disconnect(){this.disconnected=true;}},fetch:(url,options)=>new Promise(resolve=>requests.push({url,options,resolve}))};
 vm.runInNewContext(script,sandbox);const provider=window.stellarTitleSearch||window.stellarLocalSearch;
 const cache={key:(url,base)=>'search_cache_v6:'+new URL(url,base).href,clear:target=>{for(const key of [...values.keys()])if(key.startsWith('search_cache_v6:'))target.removeItem(key);return {ok:true};}};
 const mount=()=>provider.mount(document,cache);
 const focus=()=>{const event=new Event('focusin');Object.defineProperty(event,'target',{value:input});document.dispatchEvent(event);};
 const query=value=>{input.value=value;input.dispatchEvent(new Event('input'));};
 function runTimers(){let count=0;while(timers.size){assert.ok(++count<10000);const [id,callback]=timers.entries().next().value;timers.delete(id);callback();}}
 async function resolveRequest(number,data=index){requests[number].resolve({ok:true,json:async()=>data});await tick();runTimers();}
 const results=()=>result.querySelectorAll('li').map(node=>({title:node.querySelector('.search-result-title').textContent,href:node.querySelector('a').href,snippet:node.querySelector('.search-result-content')?.textContent}));
 return{document,input,result,wrapper,values,requests,timers,observers,provider,mount,focus,query,runTimers,resolveRequest,results,window};
}
const titleIndex=[{title:'Alpha标题',path:'https://fixture.test/sub/post/alpha/',tags:['Rocket'],categories:['Science'],domains:['blog']},{title:'Beta标题',path:'https://fixture.test/sub/wiki/docs/intro/',tags:['Guide'],categories:['Manual'],domains:['wiki:docs']},{title:'Empty body',path:'https://fixture.test/sub/page/empty/',content:'',domains:['page']},{title:'Cached secret',path:'https://fixture.test/sub/page/cached/',content:'hidden-body-word',anchors:[{id:'private',text:'Private',offset:0}]}];

test('关闭正文索引仍按标题/标签/分类匹配，缺省与空content不伪造摘要，domain保持隔离',async()=>{
 const f=fixture({index:titleIndex});const cleanup=f.mount();f.focus();f.query('Alpha');await f.resolveRequest(0);assert.deepEqual(f.results(),[{title:'Alpha标题',href:'https://fixture.test/sub/post/alpha/?kw=Alpha',snippet:undefined}]);
 for(const term of ['Rocket','Science']){f.query(term);f.runTimers();assert.equal(f.results()[0].title,'Alpha标题');assert.equal(f.results()[0].snippet,undefined);}
 f.query('Empty');f.runTimers();assert.equal(f.results()[0].title,'Empty body');
 f.input.setAttribute('data-domain','wiki:docs');f.query('标题');f.runTimers();assert.deepEqual(f.results().map(item=>item.title),['Beta标题']);
 f.input.setAttribute('data-domain','');f.query('hidden-body-word');f.runTimers();assert.deepEqual(f.results(),[]);cleanup();
});
test('原版stock确实丢弃标题-only索引，修复验证针对实际脚本而非造正文字段',async()=>{
 const f=fixture({index:titleIndex,script:stock});const cleanup=f.mount();f.focus();f.query('Alpha');await f.resolveRequest(0);assert.deepEqual(f.results(),[]);cleanup();
});
test('重复挂载/聚焦只有一个请求与处理器；快速换词/清空/IME取消旧批次',async()=>{
 const f=fixture({index:titleIndex});const first=f.mount(),second=f.mount();assert.equal(first,second);f.focus();f.focus();assert.equal(f.requests.length,1);f.query('Alpha');await f.resolveRequest(0);
 f.query('Alpha');f.query('Beta');f.runTimers();assert.deepEqual(f.results().map(item=>item.title),['Beta标题']);
 f.query('Alpha');f.query('');f.runTimers();assert.deepEqual(f.results(),[]);
 f.query('Alpha');f.input.dispatchEvent(new Event('compositionstart'));f.runTimers();assert.deepEqual(f.results(),[]);f.input.value='Beta';f.input.dispatchEvent(new Event('compositionend'));f.runTimers();assert.equal(f.results()[0].title,'Beta标题');
 first();second();assert.equal(f.timers.size,0);assert.ok(f.observers.every(observer=>observer.disconnected));f.query('Alpha');f.runTimers();assert.equal(f.results()[0].title,'Beta标题');
});
test('卸载取消待取索引及迟到正文解码；重挂不会使用旧结果/重写新状态',async()=>{
 const f=fixture();const first=f.mount();f.focus();assert.equal(f.requests.length,1);first();assert.equal(f.requests[0].options.signal.aborted,true);
 const next=f.mount();f.focus();f.query('Beta');assert.equal(f.requests.length,2);await f.resolveRequest(0,[titleIndex[0]]);assert.deepEqual(f.results(),[]);await f.resolveRequest(1,[titleIndex[1]]);assert.equal(f.results()[0].title,'Beta标题');
 next();const final=f.mount();f.focus();f.query('Beta');f.runTimers();assert.equal(f.requests.length,2);final();
 const uncached=fixture({ttl:0});const old=uncached.mount();uncached.focus();let resolveBody;uncached.requests[0].resolve({ok:true,json:()=>new Promise(resolve=>{resolveBody=resolve;})});await tick();old();resolveBody([titleIndex[0]]);await tick();assert.equal(uncached.values.size,0);assert.deepEqual(uncached.results(),[]);
});
test('缓存采用v6+路径TTL，清理内存后重新拉取；启用正文时保留章节锚点行为',async()=>{
 const f=fixture({index:[{title:'Article',path:'https://fixture.test/post/a/',content:'intro Heading needle',anchors:[{id:'heading',text:'Heading',offset:6}]}],content:true});const cleanup=f.mount();f.focus();f.query('needle');await f.resolveRequest(0);assert.match(f.results()[0].href,/#heading$/);assert.match(f.results()[0].snippet,/needle/);
 const [key,value]=f.values.entries().next().value;assert.equal(key,'search_cache_v6:https://fixture.test/sub/search.json');assert.equal(JSON.parse(value).ttl,60);cleanup();f.provider.clearCache();assert.equal(f.values.size,0);const next=f.mount();f.focus();assert.equal(f.requests.length,2);next();
});

test('标题搜索runtime复用实际官方快捷键并幂等挂载；settings清理后索引重新获取',async t=>{
 const f=fixture({index:titleIndex});let opened=0;const trigger=f.document.createElement('button');trigger.setAttribute('data-shell-action','open-search');trigger.click=()=>opened++;f.document.appendChild(trigger);
 vm.runInNewContext(readFileSync(new URL('../web/assets/js/search/shortcut.js',import.meta.url),'utf8'),{document:f.document,window:f.window});
 const previous=Object.getOwnPropertyDescriptor(globalThis,'window');Object.defineProperty(globalThis,'window',{configurable:true,value:f.window});t.after(()=>{if(previous)Object.defineProperty(globalThis,'window',previous);else delete globalThis.window;});
 const assets=[];const callbacks=[];const context={signal:new AbortController().signal,extension:{config:{provider:'local',assets:{provider:'/assets/js/blog/search/title-search.js',shortcut:'/assets/js/search/shortcut.js'}}},assets:{script:async path=>assets.push(path)},onCleanup:fn=>callbacks.push(fn)};
 const [first,second]=await Promise.all([runtimeMount(f.document,context),runtimeMount(f.document,context)]);assert.equal(first,second);assert.equal(callbacks.length,1);assert.deepEqual(assets,['/assets/js/blog/search/title-search.js','/assets/js/search/shortcut.js']);
 const shortcut=new Event('keydown',{cancelable:true});for(const [key,value]of Object.entries({key:'k',ctrlKey:true,target:trigger}))Object.defineProperty(shortcut,key,{value});f.document.dispatchEvent(shortcut);assert.equal(opened,1);assert.equal(shortcut.defaultPrevented,true);
 f.focus();f.query('Alpha');await f.resolveRequest(0);assert.equal(f.results()[0].title,'Alpha标题');
 const clear=f.document.createElement('button');clear.setAttribute('data-cache-action','search');clear.dataset.cacheAction='search';f.document.appendChild(clear);const event=new Event('click');Object.defineProperty(event,'target',{value:clear});f.document.dispatchEvent(event);for(const key of [...f.values.keys()])f.values.delete(key);
 f.focus();assert.equal(f.requests.length,2);first();second();assert.equal(f.requests[1].options.signal.aborted,true);
 const after=new Event('keydown',{cancelable:true});for(const [key,value]of Object.entries({key:'k',ctrlKey:true,target:trigger}))Object.defineProperty(after,key,{value});f.document.dispatchEvent(after);assert.equal(opened,1);
});
