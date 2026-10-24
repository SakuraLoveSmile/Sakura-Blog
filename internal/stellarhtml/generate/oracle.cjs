'use strict';
// Development-only oracle. Production and Go tests do not execute JavaScript.
// Usage: NODE_PATH=<isolated deps>/node_modules node oracle.cjs <upstream root> <output>
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(process.argv[2]);
const link = require(path.join(root, 'scripts/filters/lib/md_link'));
const table = require(path.join(root, 'scripts/filters/lib/md_table'));
const lazy = require(path.join(root, 'scripts/filters/lib/img_lazyload'));
const error = require(path.join(root, 'scripts/filters/lib/img_onerror'));
const fallback = require(path.join(root, 'scripts/lib/image-fallback'));
const icon = '<svg viewBox="0 0 1 1"><path d="M0 0"/></svg>';
const endpoint = 'https://metadata.example/info?url={href}&source=blog';
const bases = ['https://example.com/', 'https://EXAMPLE.com:443/docs/', 'https://user:pass@example.com:444/docs/index.html?base=yes#old', 'http://example.com/docs/', 'https://例子.测试/路径/', 'https://[2001:db8::1]/docs/'];
const hrefs = ['', '#anchor', 'https://example.com', '/docs/a', '../a', '?q=hello world', 'chapter#你好', './%2e%2e/a', '//cdn.example.com/image?x=1&y=2', 'mailto:a@example.com', 'tel:123', 'data:text/plain,Hello', 'javascript:alert(1)', 'https:example.com', 'http:example.com', 'http:/example.com', 'https:\\example.com\\path', '\\path', 'https://127.1/a', 'https://0x7f.01/a', 'https://0177.1/a', 'https://example.com:000443/a', 'https://example.com:65536/', 'https://ex%61mple.com/a', 'https://例子.测试/你好?q=你好', 'https://example.com/a/%2E/b/..', "https://example.com?q='hello'", 'https://u:p@EXAMPLE.com/a', 'https://a@b@example.com/a', 'https://example.com/%invalid', 'https://example.com/a%2Fb', 'https://example.com/a//b/../', 'https://example.com/a./..//c', 'https://example.com/a?x=`{}"<>#a`', 'http://[::ffff:192.0.2.128]/', 'https://example.com/a\n\tb', 'https://example.com:0/', 'http://4294967295/', 'http://256.1/', 'http://example.123/', 'ftp://example.com/a', 'https://', './a b', '/a%2e./b', '/a/%2e./b', '/a/.%2e/b', 'https://example.com/?', 'https://example.com/#', 'HTTP://example.com:080/'];
const contents=[];
function addContent(input, base=bases[0], enabled=true) {
  const theme={services:{siteInfo:{provider:enabled?'test':null,test:{endpoint}}}};
  const ctx={config:{url:base},stellar:{config:theme},utils:{icon:()=>icon}};
  const data={content:input};table.processPost.call(ctx,data);link.processPost.call(ctx,data);
  contents.push({input,base,theme,expected:data.content});
}
const escape = s => s.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;');
for(const base of bases)for(const href of hrefs)addContent(`<p><a href="${escape(href)}">Link 文本</a></p>`,base);
for(const input of [
  '<p>Plain text</p>', '<table><tr><td>a</td></tr></table>',
  '<div class="md-table-scroll"><table><tr><td>a</td></tr></table></div>',
  '<div class="highlight"><table><tr><td>a</td></tr></table></div>',
  '<div class="tag-plugin table"><table><tr><td>a</td></tr></table></div>',
  '<table><tr><td><a href="/x">X</a></td></tr></table>',
  '<a href="/x" class="custom">X</a>', '<a href="/x" class="">X</a>',
  '<a href="/x" role="link">X</a>', '<a href="/x" role="">X</a>',
  '<a href="/x" cardlink>X</a>', '<a href="/x" data-md-link>X</a>',
  '<a href="/x" data-siteinfo-api="existing">X</a>',
  '<pre><a href="/x">X</a></pre>', '<code><a href="/x">X</a></code>',
  '<div class="footnotes"><a href="/x">X</a></div>',
  '<a href="/x"><img src="a"></a>', '<a href="/x"><svg></svg> X</a>',
  '<a href="/x"> </a>', '<a href="/x"><strong>X</strong> Y</a>',
  '<a>X</a>', '<script data-stellar-script="core">const x="<a href=x>";</script><a href="/a">A</a>',
  '<!-- <table></table> --><a href="/x">X</a>', '<style>a{content:"<table>"}</style><a href="/x">X</a>'
]) {addContent(input);addContent(input,bases[0],false)}
const images=[];
const fallbacks=['/fallback.svg','../fallback.png','https://example.com/fallback.svg?a="&b=1','data:image/svg+xml,'+encodeURIComponent('<svg><!-- attribution --><text>图 &amp; 字</text></svg>'),'data:image/svg+xml;base64,'+Buffer.from('<svg></svg>').toString('base64'),'data:image/svg+xml,%invalid'];
const inputs=['<img src="x.png">','<IMG\n SRC="x.png" alt="a > b"/>',"<img src='x.png' >",'<img src=x.png>','<img src="x.png" class="custom">','<img src=x.png class>','<img src="x.png" onerror="custom()">','<img no-lazy src=x.png>','<img no-lazy="" src=x.png>','<img class="no-lazy" src="x.png">','<img src="x.png" srcset="x.png 1x, y.png 2x">','<img src="x.png" srcset = "x.png 1x">','<img data-src="x.png" src="data:image/png;base64,AA==">','<img src="data:image/svg+xml,inline">','<img src="">','<img alt="no source">','<img src="x.png?a=1&amp;b=2">','<img src="x.png" class="a&amp;b">','<script data-stellar-script="core">const x=`<img src="fake">`;</script><img src="real">','<style>.x{content:\'<img src="fake">\'}</style><img src="real">','<!-- <img src="fake"> --><img src="real">','<image src="not-img">','<p>No images</p>'];
for(const base of ['https://example.com/','https://example.com/docs/'])for(const value of fallbacks)for(const input of inputs){
 const ctx={config:{url:base,root:new URL(base).pathname,relative_link:true},utils:{iconData:()=>value}};
 images.push({input,base,fallback:value,expected:error.processSite.call(ctx,lazy.lazyProcess(input))});
}
const registerUtils=require(path.join(root,'scripts/events/lib/utils'));
const iconValues={'a:svg':'<svg><!-- comment --><path/></svg>','a:url':'https://example.com/a.svg','a:relative':'/a.svg','a:text':'plain','b:svg':' <svg></svg>'};
const icons=[];const ctx={stellar:{data:{icons:iconValues}}};registerUtils(ctx);
for(const key of [...Object.keys(iconValues),'missing:key'])for(const inline of [true,false])for(const args of ['','no-lazy','class="custom"'])icons.push({key,inline,args,expected:ctx.utils.icon(key,args,inline)});
const fallbackAssets=[];for(const value of [...fallbacks,'data:image/svg+xml;base64,PHN2Zz4<noise/>PC9zdmc+','data:image/svg+xml;utf8,<svg></svg>']){const asset=fallback.imageFallbackAsset(value);fallbackAssets.push({value,expected:asset?{path:asset.path,body:asset.data.toString('base64')}:null})}
fs.writeFileSync(process.argv[3],JSON.stringify({upstream:'9d11978a24fe6610e3a7dbab741cf62634539d05',contents,images,iconValues,icons,fallbackAssets},null,2)+'\n');
