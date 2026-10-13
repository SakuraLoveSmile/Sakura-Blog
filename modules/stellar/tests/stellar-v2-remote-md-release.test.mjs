import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const source = name => readFileSync(new URL('../web/assets/js/'+name,import.meta.url),'utf8');
const flush = () => new Promise(resolve => setImmediate(resolve));
const classes = values => {const set=new Set(values);return {add:v=>set.add(v),remove:v=>set.delete(v),contains:v=>set.has(v)};};
function releaseFixture() {
 const value={textContent:''};const el={dataset:{api:'https://github.test/repos/a/b/tags',repo:'a/b',projectName:'Stellar'},classList:classes(['is-loading']),querySelector:s=>s==='.wiki-cover-release-value'?value:null,querySelectorAll:()=>[],getAttribute:()=> '0',matches:s=>s==='[data-blog-service="wiki-release"]',remove(){this.removed=true;}};
 const document={currentScript:{}};const requests=[],errors=[],controller=new AbortController();const window={document,fetch:(url,options)=>new Promise((resolve,reject)=>requests.push({url,options,resolve,reject}))};document.defaultView=window;el.ownerDocument=document;
 const sandbox={document,window,URL,AbortController,WeakSet,Set,Promise};vm.createContext(sandbox);vm.runInContext(source('blog/lifecycle.js'),sandbox);vm.runInContext(source('blog/services/wiki-release.js'),sandbox);
 const context={signal:controller.signal,reportError:error=>errors.push(error),onCleanup(){}};
 const mount=()=>document.currentScript.stellarMount(el,context);return {el,value,requests,errors,controller,mount};
}
test('Hero consumes actual ds-ghinfo DOM and first official tag, only one successful consumer',async()=>{
 const f=releaseFixture();const cleanup=f.mount();assert.equal(f.requests.length,1);assert.equal(f.requests[0].url,f.el.dataset.api);
 f.requests[0].resolve({ok:true,json:async()=>[{name:'v2.0.0'}]});await flush();
 assert.equal(f.value.textContent,'Stellar v2.0.0');assert.equal(f.el.href,'https://github.com/a/b/tree/v2.0.0');assert.equal(f.el.classList.contains('loaded'),true);assert.equal(f.el.classList.contains('is-loading'),false);
 f.mount();assert.equal(f.requests.length,1);cleanup();assert.equal(f.requests[0].options.signal.aborted,true);
});
test('Hero handles release objects and hides failed/empty/unsafe release badge',async()=>{
 for(const data of [{tag_name:'v1',html_url:'https://github.com/a/b/releases/tag/v1'},[],[{name:'v1',html_url:'javascript:alert(1)'}]]) {
 const f=releaseFixture();f.mount();f.requests[0].resolve({ok:true,json:async()=>data});await flush();
 if(data.tag_name){assert.equal(f.el.href,data.html_url);}else {assert.equal(f.el.removed,true);}
 }
 const f=releaseFixture();f.mount();f.requests[0].resolve({ok:false,status:502});await flush();assert.equal(f.el.removed,true);assert.equal(f.errors.length,1);
});
test('Hero unload cancels network and ignores late decoded success/failure',async()=>{
 const f=releaseFixture();const cleanup=f.mount();let resolve;f.requests[0].resolve({ok:true,json:()=>new Promise(r=>{resolve=r;})});await flush();cleanup();resolve([{name:'late'}]);await flush();
 assert.equal(f.requests[0].options.signal.aborted,true);assert.equal(f.value.textContent,'');assert.equal(f.el.removed,undefined);assert.equal(f.errors.length,0);
 const failed=releaseFixture();failed.mount();failed.controller.abort();failed.requests[0].reject(new Error('late failure'));await flush();assert.equal(failed.el.removed,undefined);assert.equal(failed.errors.length,0);
});

function mdFixture(attrs={}) {
 const eventLog=[],requests=[];const controller=new AbortController();const heading={tagName:'H2',textContent:'Section title',firstChild:null,insertBefore(a){this.anchor=a;}};const h1={tagName:'H1',remove(){this.removed=true;}};
 const link={tagName:'A',attrs:{href:'../doc.md'},getAttribute(k){return this.attrs[k];},setAttribute(k,v){this.attrs[k]=v;}};const image={...link,tagName:'IMG',attrs:{src:'img.png'}};
 const tmp={childNodes:[h1,heading,link,image],querySelectorAll(s){if(s==='h1,h2,h3,h4,h5,h6')return h1.removed?[heading]:[h1,heading];if(s==='img[src],a[href]')return [link,image];if(s==='a[href]')return[link];return[];}};
 const document={currentScript:{},createElement(tag){if(tag==='div')return tmp;return {setAttribute(k,v){this[k]=v;}};},dispatchEvent:e=>eventLog.push(e)};
 const el={getAttribute:k=>({src:'https://raw.test/a/b/README.md?raw=1','data-heading':'true',...attrs}[k]||null),closest:()=>({querySelectorAll:()=>[{id:'section-title'}]}),replaceChildren(...children){this.children=children;},replaceWith(...children){this.replaced=children;}};
 const sandbox={document,URL,Set,CustomEvent:class {constructor(type,options){this.type=type;this.detail=options.detail;}}};vm.runInNewContext(source('services/mdrender.js'),sandbox);
 const context={signal:controller.signal,remoteMarkdownAPI:'/sub/api/stellar/remote-md',serviceUtils:{request(el,url,callback){requests.push(url);return callback(context.response);}}};
 return {document,el,tmp,link,image,heading,h1,controller,context,requests,eventLog,mount:()=>document.currentScript.stellarMount({getElementsByClassName:()=>[el]},context)};
}
test('remote md uses protected API and sanitized response, preserves relative links/headings/TOC event and cache key',async()=>{
 const f=mdFixture();f.context.response={ok:true,json:async()=>({html:'<h2>Section title</h2>',source_url:'https://raw.test/a/b/README.md?raw=1'})};await f.mount();
 assert.equal(f.requests[0],'/sub/api/stellar/remote-md?url='+encodeURIComponent('https://raw.test/a/b/README.md?raw=1'));
 assert.equal(f.tmp.innerHTML,'<h2>Section title</h2>');assert.equal(f.link.attrs.href,'https://raw.test/a/doc.md');assert.equal(f.image.attrs.src,'https://raw.test/a/b/img.png');assert.equal(f.h1.removed,true);assert.equal(f.heading.id,'section-title-1');assert.equal(f.heading.anchor.href,'#section-title-1');assert.equal(f.eventLog[0].type,'stellar:mdrender');assert.equal(f.eventLog[0].detail.links[0],f.link);assert.equal(f.el.children.length,4);
 await f.mount();assert.equal(f.requests[0],f.requests[1]);
});
test('remote md honors GitHub base and wrap=false; HTTP errors and late bodies never update DOM',async()=>{
 const f=mdFixture({'data-base':'https://mirror.test/a/b/main/','data-replace':'true'});f.context.response={ok:true,json:async()=>({html:'<p>safe</p>',source_url:'https://other.test/README.md'})};await f.mount();assert.equal(f.image.attrs.src,'https://mirror.test/a/b/main/img.png');assert.equal(f.el.replaced.length,4);
 const failed=mdFixture();failed.context.response={ok:false,status:502};await assert.rejects(failed.mount(),/502/);assert.equal(failed.el.children,undefined);assert.equal(failed.eventLog.length,0);
 const late=mdFixture();let resolve;late.context.response={ok:true,json:()=>new Promise(r=>{resolve=r;})};const pending=late.mount();late.controller.abort();resolve({html:'late',source_url:'https://raw.test/a'});await assert.rejects(pending);assert.equal(late.tmp.innerHTML,undefined);assert.equal(late.eventLog.length,0);
});
