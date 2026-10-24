import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';
import { mount as lazyMount } from '../web/assets/js/runtime/extensions/blog-lazy-loading.js';
import { mount as gistMount } from '../web/assets/js/runtime/extensions/blog-gist.js';
import { mount as controlsMount } from '../web/assets/js/runtime/extensions/blog-page-controls.js';
import { mount as servicesMount } from '../web/assets/js/runtime/extensions/blog-upstream-services.js';
import { mount as nativeMount } from '../web/assets/js/runtime/extensions/blog-native.js';

function classes(values = []) { const set = new Set(values);return { contains: v => set.has(v), add: v => set.add(v), remove: v => set.delete(v) }; }
class Element extends EventTarget {
  constructor(tag='DIV') { super();this.tagName=tag;this.nodeType=1;this.attrs=new Map();this.classList=classes();this.style={};this.map={};this.isConnected=true; }
  getAttribute(k) { return this.attrs.get(k)||null; }
  setAttribute(k,v) { this.attrs.set(k,v); }
  removeAttribute(k) { this.attrs.delete(k); }
  querySelectorAll(s) { return this.map[s] || []; }
  querySelector(s) { return this.querySelectorAll(s)[0]||null; }
  matches() { return false; }
  contains(element) { return Object.values(this.map).some(items => items.includes(element)); }
  closest(s) { return this.closestMap?.[s]||null; }
}
function fixture(t) {
  const root = new Element();root.nodeType=9;root.baseURI='https://fixture.test/';root.documentElement=new Element();
  const view=new EventTarget();view.crypto=webcrypto;root.defaultView=view;const observers=[];
  class Observer { constructor(fn) { this.fn=fn;this.disconnected=false;observers.push(this); } observe() {} disconnect() { this.disconnected=true; } }
  view.MutationObserver=Observer;
  const saved = new Map(['window','document','location','MutationObserver','IntersectionObserver'].map(key=>[key,Object.getOwnPropertyDescriptor(globalThis,key)]));
  for(const [key,value] of Object.entries({window:view,document:root,location:{search:''},MutationObserver:Observer,IntersectionObserver:Observer}))Object.defineProperty(globalThis,key,{configurable:true,value});
  t.after(()=>{for(const [key,value]of saved){if(value)Object.defineProperty(globalThis,key,value);else delete globalThis[key];}});
  const callbacks=[];const errors=[];const controller=new AbortController();
  const context={signal:controller.signal,extension:{config:{asset:'/fixture/lazy.js'}},assets:{script:async()=>{}},onCleanup:fn=>callbacks.push(fn),reportError:error=>errors.push(error)};
  return {root,view,context,controller,observers,errors,callbacks};
}
function dispatch(target,name,properties){const event=new Event(name,{cancelable:true});Object.keys(properties).forEach(key=>Object.defineProperty(event,key,{value:properties[key]}));target.dispatchEvent(event);return event;}
function imageFixture(f){const image=new Element('IMG');image.classList=classes(['lazy']);image.setAttribute('data-src','/original.png');image.setAttribute('data-fallback','/fallback.png');const spinner={removed:false,remove(){this.removed=true;}};const wrapper=new Element();wrapper.map['.lazy-icon']=[spinner];image.closestMap={'.lazy-box':wrapper};f.root.map['img.lazy[data-src]']=[image];f.root.documentElement.map['img']=[image];return{image,spinner};}

test('lazy资源失败显式报错并保留原图src，fallback只尝试一次且加载完成移除spinner',async t=>{
 const f=fixture(t),{image,spinner}=imageFixture(f);f.context.assets.script=async()=>{throw new Error('fixture loader unavailable');};
 const cleanup=await lazyMount(f.root,f.context);assert.equal(f.errors.length,1);assert.equal(image.getAttribute('src'),'/original.png');
 dispatch(f.root,'error',{target:image});assert.equal(image.getAttribute('src'),'/fallback.png');assert.equal(image.getAttribute('data-src'),'/fallback.png');
 dispatch(f.root,'error',{target:image});assert.equal(image.getAttribute('src'),'/fallback.png');assert.equal(spinner.removed,true);
 dispatch(f.root,'load',{target:image});assert.equal(image.classList.contains('loaded'),true);assert.equal(image.classList.contains('error'),false);
 cleanup();image.setAttribute('src','/new.png');dispatch(f.root,'error',{target:image});assert.equal(image.getAttribute('src'),'/new.png');assert.ok(f.observers.every(observer=>observer.disconnected));
});
test('lazy正常资源仍调用官方实例；取消加载期间不安装迟到fallback或写旧DOM',async t=>{
 const f=fixture(t),{image}=imageFixture(f);let destroyed=0,updated=0;
 f.view.LazyLoad=class{update(){updated++;}destroy(){destroyed++;}};
 const cleanup=await lazyMount(f.root,f.context);assert.ok(updated>0);assert.equal(image.getAttribute('src'),null);cleanup();assert.equal(destroyed,1);
 let reject;f.context.assets.script=()=>new Promise((_resolve,rejecter)=>{reject=rejecter;});const pending=lazyMount(f.root,f.context);
 await Promise.resolve();f.controller.abort();reject(new Error('late'));await assert.rejects(pending);assert.equal(image.getAttribute('src'),null);assert.equal(f.errors.length,0);assert.equal(f.view.wrapLazyloadImages,undefined);
});
test('Gist只接受本frame/current-token/null-origin尺寸，卸载取消子文档且重挂拒绝旧消息',t=>{
 const f=fixture(t),frame=new Element('IFRAME');frame.contentWindow={postMessage(){}};frame.ownerDocument=f.root;frame.setAttribute('src','/stellar/embed/gist?gist=test%2Fab');f.root.map['iframe[data-stellar-gist]']=[frame];
 const cleanup=gistMount(f.root,f.context);const token=new URL(frame.getAttribute('src')).hash.slice(1).split('=')[1];
 const message=(overrides={})=>dispatch(f.view,'message',{origin:'null',source:frame.contentWindow,data:{type:'stellar:gist:resize',height:320,token},...overrides});
 message({origin:'https://fixture.test'});message({source:{}});message({data:{type:'stellar:gist:resize',height:320,token:'old'}});assert.equal(frame.style.height,undefined);
 message();assert.equal(frame.style.height,'320px');cleanup();assert.equal(frame.getAttribute('src'),'about:blank');
 const again=gistMount(f.root,f.context);const nextToken=new URL(frame.getAttribute('src')).hash.slice(1).split('=')[1];assert.notEqual(nextToken,token);
 message({data:{type:'stellar:gist:resize',height:640,token}});assert.equal(frame.style.height,'320px');message({data:{type:'stellar:gist:resize',height:640,token:nextToken}});assert.equal(frame.style.height,'640px');again();
});
test('Gist子文档测量携带fragment代号，hash变更重发同高度；pagehide释放observer/RAF',()=>{
 const view=new EventTarget();const sent=[],frames=new Map();let index=0;view.parent={postMessage:data=>sent.push(data)};view.location={hash:'#stellar-gist-token=one'};view.requestAnimationFrame=fn=>{frames.set(++index,fn);return index;};view.cancelAnimationFrame=id=>frames.delete(id);
 let observer;class ResizeObserver{constructor(fn){this.fn=fn;observer=this;}observe(){}disconnect(){this.disconnected=true;}}
 vm.runInNewContext(readFileSync(new URL('../web/assets/js/blog-gist.js',import.meta.url),'utf8'),{window:view,document:{body:{getBoundingClientRect:()=>({height:200.5})}},URLSearchParams,ResizeObserver});
 function flush(){const pending=[...frames.values()];frames.clear();pending.forEach(fn=>fn());}flush();assert.equal(sent[0].height,201);assert.equal(sent[0].token,'one');
 view.location.hash='#stellar-gist-token=two';view.dispatchEvent(new Event('hashchange'));flush();assert.equal(sent[1].token,'two');assert.equal(sent[1].height,201);
 observer.fn();assert.equal(frames.size,1);view.dispatchEvent(new Event('pagehide'));assert.equal(frames.size,0);assert.equal(observer.disconnected,true);view.dispatchEvent(new Event('hashchange'));assert.equal(frames.size,0);
});
test('Blog原生评论只挂独立容器；TOC控件跳Blog目标并清理监听，不覆盖官方评论',async t=>{
 const f=fixture(t);const blog=new Element(),official=new Element(),control=new Element();blog.getBoundingClientRect=()=>({top:200});control.closestMap={'[data-blog-action="scroll-comments"], [data-comments-target="blog-comments"]':control};
 f.root.map['#blog-comments']=[blog];f.root.map['control']=[control];f.root.getElementById=id=>id==='blog-comments'?blog:official;
 let mounted;f.view.stellar={comments:{mount:root=>{mounted=root;return()=>{};}}};f.context.extension.id='blog-comments';await nativeMount(f.root,f.context);assert.equal(mounted,blog);
 const scroll=[];f.view.scrollY=50;f.view.scrollTo=value=>scroll.push(value);const cleanup=await controlsMount(f.root,f.context);
 const event=dispatch(f.root,'click',{target:control});assert.equal(event.defaultPrevented,true);assert.equal(scroll[0].top,218);cleanup();dispatch(f.root,'click',{target:control});assert.equal(scroll.length,1);
});

test('stock services排除Blog归属DOM，regional卸载后取消信号与迟到正文不再回写',async t=>{
 const f=fixture(t),stock=new Element(),blog=new Element();blog.closestMap={'[data-blog-service]':blog};
 f.root.getElementsByClassName=()=>[stock,blog];f.root.documentElement.map['service']=[stock,blog];
 f.context.legacy={ctx:{services:{friends:{js:'/fixture/friends.js'}},icons:{}}};
 let resolveBody,requestOptions,writes=0;const mounted=[];let pending;
 f.view.utils={request(_element,_url,callback,_failure,options){requestOptions=options;return Promise.resolve(callback({json:()=>new Promise(resolve=>{resolveBody=resolve;})}));}};
 f.context.assets.script=async()=>({stellarMount(root,context){const nodes=root.getElementsByClassName('ds-friends');mounted.push(...nodes);pending=context.serviceUtils.request(nodes[0],'/fixture/data',async response=>{await response.json();writes++;});}});
 const cleanup=await servicesMount(f.root,f.context);assert.deepEqual(mounted,[stock]);assert.equal(requestOptions.signal,f.context.signal);
 f.controller.abort();cleanup();resolveBody({});await pending;assert.equal(writes,0);assert.ok(f.observers.every(observer=>observer.disconnected));
});
