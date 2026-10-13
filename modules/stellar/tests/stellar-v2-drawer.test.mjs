import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import vm from 'node:vm';
const baselineRef = process.env.BLOG_DRAWER_SOURCE_REF || (process.env.BLOG_DRAWER_BASELINE === '1' ? 'e576465' : '');
const source = baselineRef
 ? execFileSync('git',['show',baselineRef+':modules/stellar/web/assets/js/main.js'],{encoding:'utf8'})
 : readFileSync(new URL('../web/assets/js/main.js',import.meta.url),'utf8');
const layout = readFileSync(new URL('../web/templates/layout/stellar.html',import.meta.url),'utf8');
class Element extends EventTarget {
 constructor(id,document){super();this.id=id;this.document=document;this.attrs=new Map();this.children=[];this.dataset={};this.parentElement=null;this.visible=true;this.open=false;this.isConnected=true;this.style={setProperty(){},removeProperty(){}};this.classes=new Set();}
 set inert(value){this._inert=Boolean(value);value?this.attrs.set('inert',''):this.attrs.delete('inert');}get inert(){return this._inert||false;}
 setAttribute(name,value){this.attrs.set(name,String(value));}getAttribute(name){return this.attrs.get(name)??null;}hasAttribute(name){return this.attrs.has(name);}removeAttribute(name){this.attrs.delete(name);}
 append(...children){for(const child of children){this.children.push(child);child.parentElement=this;}}
 contains(target){return target===this||this.children.some(child=>child.contains(target));}
 matches(selector){return selector.split(',').some(part=>{part=part.trim();if(part==='[hidden]')return this.hasAttribute('hidden');if(part==='[inert]')return this.inert;if(part==='button:not([disabled])')return this.tag==='button'&&!this.hasAttribute('disabled');if(part==='a[href]')return this.tag==='a'&&this.hasAttribute('href');if(part.startsWith('[tabindex]'))return this.hasAttribute('tabindex')&&this.getAttribute('tabindex')!=='-1';if(part.startsWith('[data-shell-action'))return this.dataset.shellAction&&(part==='[data-shell-action]'||part.includes('"'+this.dataset.shellAction+'"'));if(part==='.site-shell')return this.id==='shell';if(part==='.site-dock')return this.id==='dock';if(part.startsWith('.site-scrim'))return this.id==='scrim';if(part.startsWith('#'))return this.id===part.slice(1);if(part.startsWith('.'))return this.classes.has(part.slice(1));return false;});}
 closest(selector){for(let element=this;element;element=element.parentElement)if(element.matches(selector))return element;return null;}
 querySelectorAll(selector){const found=[];for(const child of this.children){if(child.matches(selector))found.push(child);found.push(...child.querySelectorAll(selector));}return found;}
 querySelector(selector){return this.querySelectorAll(selector)[0]||null;}
 get classList(){return {add:value=>this.classes.add(value),remove:value=>this.classes.delete(value),contains:value=>this.classes.has(value),toggle:(value,on)=>{if(on??!this.classes.has(value)){this.classes.add(value);return true;}this.classes.delete(value);return false;}};}
 getClientRects(){return this.visible&&!this.closest('[hidden]')?[{}]:[];}
 focus(){if(this.closest('[inert]'))return;this.document.activeElement=this;const event=new Event('focusin');Object.defineProperty(event,'target',{value:this});this.document.dispatchEvent(event);}
 blur(){if(this.document.activeElement===this)this.document.activeElement=this.document.body;}
 replaceChildren(){this.children=[];}
 showModal(){this.returnFocus=this.document.activeElement;this.open=true;}close(){this.open=false;this.returnFocus?.focus();}
}
function fixture({desktopLeft=false,desktopRight=false,reducedMotion=true}={}){
 const document=new EventTarget();const node=(id,tag)=>{const element=new Element(id,document);element.tag=tag;return element;};
 const root=node('root'),body=node('body'),shell=node('shell'),workspace=node('workspace'),main=node('main'),cover=node('site-cover'),topbar=node('topbar-region'),left=node('leftbar-region'),right=node('rightbar-region'),dock=node('dock'),scrim=node('scrim','button'),dialog=node('site-search-dialog');
 assert.match(layout,/class="site-scrim"[^>]*data-shell-action="dismiss-drawer"[^>]*tabindex="-1"/);scrim.setAttribute('tabindex','-1');scrim.dataset.shellAction='dismiss-drawer';
 root.append(body);body.append(cover,shell);shell.append(topbar,workspace,scrim,dock,dialog);workspace.append(left,main,right);
 const leftFirst=node('left-first','a'),leftLast=node('left-last','button'),rightFirst=node('right-first','a'),leftTrigger=node('left-trigger','button'),rightTrigger=node('right-trigger','button'),searchTrigger=node('search-trigger','button'),mainLink=node('main-link','a');leftFirst.setAttribute('href','/wiki');rightFirst.setAttribute('href','/docs');mainLink.setAttribute('href','/post');left.append(leftFirst,searchTrigger,leftLast);right.append(rightFirst);main.append(mainLink);dock.append(leftTrigger,rightTrigger);leftTrigger.dataset.shellAction='toggle-leftbar-drawer';rightTrigger.dataset.shellAction='toggle-rightbar-drawer';searchTrigger.dataset.shellAction='open-search';
 const searchInput=node('search-input');searchInput.classes.add('search-input');dialog.append(searchInput);document.documentElement=root;document.body=body;document.activeElement=mainLink;
 document.querySelector=selector=>root.querySelector(selector);document.querySelectorAll=selector=>root.querySelectorAll(selector);document.getElementById=id=>id==='main'?main:root.querySelector('#'+id);
 const window=new EventTarget(),leftMedia=new EventTarget(),rightMedia=new EventTarget(),reduced=new EventTarget();leftMedia.matches=!desktopLeft;rightMedia.matches=!desktopRight;reduced.matches=reducedMotion;window.matchMedia=query=>query.includes('prefers-reduced')?reduced:query.includes('768')?leftMedia:rightMedia;window.innerWidth=390;window.localStorage={getItem(){return null;},setItem(){}};window.canonical={};window.stellar={};document.defaultView=window;
 const timers=new Map();let timerId=0;
 const sandbox={document,window,stellar:window.stellar,localStorage:window.localStorage,console,Event,Map,WeakMap,AbortController,setTimeout:(callback,delay)=>{timers.set(++timerId,{callback,delay});return timerId;},clearTimeout:id=>timers.delete(id),utils:{dom(){return {click(){}};},qsa(){return [];}}};vm.createContext(sandbox);vm.runInContext(source,sandbox);
 function click(target){if(target.tag==='button'||target.tag==='a')target.focus();const event=new Event('click',{cancelable:true});Object.defineProperty(event,'target',{value:target});document.dispatchEvent(event);return event;}
 function key(key,shiftKey=false){const event=new Event('keydown',{cancelable:true});Object.defineProperties(event,{key:{value:key},shiftKey:{value:shiftKey}});document.dispatchEvent(event);return event;}
 return{timers,document,window,root,main,cover,topbar,left,right,dock,scrim,dialog,leftFirst,leftLast,rightFirst,leftTrigger,rightTrigger,searchTrigger,searchInput,mainLink,leftMedia,rightMedia,click,key};
}
test('实际main消费者打开drawer隔离全部背景与另一panel，Tab与Shift+Tab循环包含可用遮罩',()=>{
 const f=fixture();f.main.setAttribute('aria-hidden','false');f.leftTrigger.focus();f.click(f.leftTrigger);assert.equal(f.left.inert,false);assert.equal(f.document.activeElement?.id,f.leftFirst.id);
 for(const node of [f.main,f.cover,f.topbar,f.right,f.dock]){assert.equal(node.inert,true);assert.equal(node.getAttribute('aria-hidden'),'true');}assert.equal(f.scrim.getAttribute('tabindex'),'0');
 f.scrim.focus();assert.equal(f.key('Tab').defaultPrevented,true);assert.equal(f.document.activeElement?.id,f.leftFirst.id);assert.equal(f.key('Tab',true).defaultPrevented,true);assert.equal(f.document.activeElement?.id,f.scrim.id);f.leftLast.focus();assert.equal(f.key('Tab').defaultPrevented,false);
 f.mainLink.focus();assert.equal(f.document.activeElement?.id,f.leftLast.id);assert.equal(f.key('Escape').defaultPrevented,true);assert.equal(f.main.inert,false);assert.equal(f.main.getAttribute('aria-hidden'),'false');assert.equal(f.scrim.getAttribute('tabindex'),'-1');assert.equal(f.document.activeElement?.id,f.leftTrigger.id);
});
test('中等断点右抽屉也隔离桌面可见leftbar，遮罩和跨断点关闭后恢复焦点/原状态',()=>{
 const f=fixture({desktopLeft:true});f.rightTrigger.focus();f.click(f.rightTrigger);assert.equal(f.left.inert,true);assert.equal(f.document.activeElement?.id,f.rightFirst.id);f.click(f.scrim);assert.equal(f.left.inert,false);assert.equal(f.main.inert,false);assert.equal(f.document.activeElement?.id,f.rightTrigger.id);
 f.click(f.rightTrigger);f.rightMedia.matches=false;f.rightMedia.dispatchEvent(new Event('change'));assert.equal(f.main.inert,false);assert.equal(f.left.inert,false);assert.equal(f.right.inert,false);assert.equal(f.scrim.getAttribute('tabindex'),'-1');
});
test('嵌套search原生dialog拥有Tab/Escape，关闭search后drawer继续捕获并最终恢复触发焦点',()=>{
 const f=fixture();f.click(f.leftTrigger);f.key('Enter');f.click(f.searchTrigger);assert.equal(f.dialog.open,true);assert.equal(f.document.activeElement?.id,f.searchInput.id);assert.equal(f.key('Tab').defaultPrevented,false);f.key('Escape');assert.equal(f.dialog.open,false);assert.equal(f.document.activeElement?.id,f.searchTrigger.id);assert.equal(f.main.inert,true);f.key('Escape');assert.equal(f.main.inert,false);assert.equal(f.document.activeElement?.id,f.leftTrigger.id);
});
test('空panel可聚焦且临时tabindex在关闭后还原；pagehide完全恢复，BFCache pageshow可再次打开',()=>{
 const f=fixture();f.left.children=[];f.click(f.leftTrigger);assert.equal(f.document.activeElement?.id,f.left.id);assert.equal(f.left.getAttribute('tabindex'),'-1');f.key('Escape');assert.equal(f.left.getAttribute('tabindex'),null);
 f.main.inert=true;f.main.setAttribute('aria-hidden','true');f.click(f.rightTrigger);f.window.dispatchEvent(new Event('pagehide'));assert.equal(f.main.inert,true);assert.equal(f.main.getAttribute('aria-hidden'),'true');assert.equal(f.left.inert,false);assert.equal(f.right.inert,false);assert.equal(f.scrim.getAttribute('tabindex'),'-1');
 const show=new Event('pageshow');Object.defineProperty(show,'persisted',{value:true});f.window.dispatchEvent(show);assert.equal(f.left.inert,true);assert.equal(f.right.inert,true);f.click(f.rightTrigger);assert.equal(f.right.inert,false);f.key('Escape');assert.equal(f.main.inert,true);
});
test('普通motion嵌套搜索Escape开始退场，animationend只关闭search并归还drawer焦点',()=>{
 const f=fixture({reducedMotion:false});f.click(f.leftTrigger);f.key('Enter');f.click(f.searchTrigger);f.key('Escape');
 assert.equal(f.dialog.open,true);assert.equal(f.dialog.classList.contains('is-closing'),true);assert.equal(f.timers.size,1);assert.equal([...f.timers.values()][0].delay,260);
 const cancel=new Event('cancel',{cancelable:true});f.dialog.dispatchEvent(cancel);assert.equal(cancel.defaultPrevented,true);assert.equal(f.timers.size,1);assert.equal(f.main.inert,true);
 const unrelated=new Event('animationend');Object.defineProperty(unrelated,'animationName',{value:'other-animation'});f.dialog.dispatchEvent(unrelated);assert.equal(f.dialog.open,true);
 const end=new Event('animationend');Object.defineProperty(end,'animationName',{value:'search-dialog-out'});f.dialog.dispatchEvent(end);assert.equal(f.dialog.open,false);assert.equal(f.timers.size,0);assert.equal(f.dialog.classList.contains('is-closing'),false);assert.equal(f.document.activeElement?.id,f.searchTrigger.id);assert.equal(f.main.inert,true);f.key('Escape');assert.equal(f.main.inert,false);assert.equal(f.document.activeElement?.id,f.leftTrigger.id);
});
test('search退场动画不触发时260ms兜底闭合；重新打开会取消旧关闭请求',()=>{
 const f=fixture({reducedMotion:false});f.click(f.leftTrigger);f.key('Enter');f.click(f.searchTrigger);f.key('Escape');const first=[...f.timers.values()][0];f.click(f.searchTrigger);assert.equal(f.timers.size,0);assert.equal(f.dialog.open,true);assert.equal(f.dialog.classList.contains('is-closing'),false);
 f.key('Escape');const pending=[...f.timers.entries()][0];assert.notEqual(pending[1],first);f.timers.delete(pending[0]);pending[1].callback();assert.equal(f.dialog.open,false);assert.equal(f.root.hasAttribute('data-search-open'),false);assert.equal(f.main.inert,true);assert.equal(f.document.activeElement?.id,f.searchTrigger.id);f.key('Escape');assert.equal(f.main.inert,false);
});
test('pointer打开再Escape关闭必须恢复触发焦点：desktop/mobile、正常motion/减少motion都覆盖',()=>{
 for(const desktop of [false,true])for(const reducedMotion of [false,true]){
  const f=fixture({desktopLeft:desktop,desktopRight:desktop,reducedMotion});if(!desktop)f.click(f.leftTrigger);
  f.document.dispatchEvent(new Event('pointerdown'));f.click(f.searchTrigger);assert.equal(f.dialog.open,true);f.key('Escape');
  if(!reducedMotion){const [id,pending]=[...f.timers.entries()][0];f.timers.delete(id);pending.callback();}
  assert.equal(f.dialog.open,false);assert.equal(f.document.activeElement?.id,f.searchTrigger.id,`desktop=${desktop} reducedMotion=${reducedMotion}`);assert.equal(f.main.inert,!desktop);
  if(!desktop){f.key('Escape');assert.equal(f.main.inert,false);assert.equal(f.document.activeElement?.id,f.leftTrigger.id);}
 }
});
test('pointer打开后的原生cancel恢复触发焦点，退场中Escape会提升原pointer关闭的恢复策略',()=>{
 for(const viaCancel of [false,true]){
  const f=fixture({reducedMotion:false});f.click(f.leftTrigger);f.document.dispatchEvent(new Event('pointerdown'));f.click(f.searchTrigger);
  if(viaCancel){const event=new Event('cancel',{cancelable:true});f.dialog.dispatchEvent(event);assert.equal(event.defaultPrevented,true);}
  else{const pointerClose=new Event('click');Object.defineProperty(pointerClose,'target',{value:f.dialog});f.dialog.dispatchEvent(pointerClose);assert.equal(f.dialog.classList.contains('is-closing'),true);f.key('Escape');}
  const [id,pending]=[...f.timers.entries()][0];f.timers.delete(id);pending.callback();assert.equal(f.dialog.open,false);assert.equal(f.document.activeElement?.id,f.searchTrigger.id);assert.equal(f.main.inert,true);
 }
});
