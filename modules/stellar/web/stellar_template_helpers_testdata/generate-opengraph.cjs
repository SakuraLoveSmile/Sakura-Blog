// 仅开发再生成；生产与 Go 测试读取固定 head.ejs + Hexo 8 的实际输出。
'use strict';
const fs=require('node:fs'),path=require('node:path'),os=require('node:os'),cp=require('node:child_process'),crypto=require('node:crypto'),vm=require('node:vm');
const helper=require('hexo/dist/plugins/helper/open_graph'),util=require('hexo-util'),moment=require('moment'),helperRequire=require('node:module').createRequire(require.resolve('hexo/dist/plugins/helper/open_graph'));
if(require('hexo/package.json').version!=='8.0.0'||require('hexo-util/package.json').version!=='4.0.0'||helperRequire('hexo-util/package.json').version!=='3.3.0')throw new Error('Requires existing Hexo 8.0.0 / hexo-util 4.0.0 and helper hexo-util 3.3.0 oracle dependencies');
const commit='9d11978a24fe6610e3a7dbab741cf62634539d05',repository=path.resolve(process.argv[2]),tmp=fs.mkdtempSync(path.join(os.tmpdir(),'stellar-og-oracle-'));
const env={...process.env};for(const k of ['GIT_DIR','GIT_WORK_TREE','GIT_COMMON_DIR','GIT_INDEX_FILE','GIT_PREFIX','GIT_OBJECT_DIRECTORY','GIT_ALTERNATE_OBJECT_DIRECTORIES'])delete env[k];
try {
 cp.execFileSync('tar',['-x','-C',tmp],{input:cp.execFileSync('git',['archive',commit,'scripts','_config.yml','layout/_partial/head.ejs'],{cwd:repository,env})});
 const source=fs.readFileSync(path.join(tmp,'layout/_partial/head.ejs'),'utf8'),script=source.slice(2,source.indexOf('%>'))+'\nrender_open_graph();';
 const models=require(path.join(tmp,'scripts/lib/models')),schema=require(path.join(tmp,'scripts/lib/config-schema')),contentConfig=require(path.join(tmp,'scripts/lib/content-config')),seo=require(path.join(tmp,'scripts/lib/seo'));
 const baseMarkdown='OG body & **detail**.\n\n![One](body-one.png) ![Two](/body-two.png)',baseContent='<p>OG body &amp; <strong>detail</strong>.</p>\n<p><img src="body-one.png" alt="One"> <img src="/body-two.png" alt="Two"></p>\n';
 const cases=[
  {name:'defaults',options:{}},
  {name:'all-options',options:{title:'Ignored title',site_name:'Ignored site',url:'https://social.example/share/item?word=hello%20world&x=1',description:' Explicit <b>description</b>\nnext & line ',type:'profile',language:'en',author:'Explicit Author',image:['relative.png','/root.png','data:image/png;base64,AAAA'],twitter_card:'summary_large_image',twitter_image:'twitter.png',twitter_id:'user',twitter_site:'@site',fb_admins:['10','20'],fb_app_id:'123',date:true,updated:true}},
  {name:'null-fallback',options:{url:null,image:null,description:null,type:null,language:null,author:null,twitter_card:null,twitter_image:null,twitter_id:null,twitter_site:null,fb_admins:null,fb_app_id:null,date:null,updated:null}},
  {name:'empty-fallback',options:{url:'',image:'',description:'',type:'',language:'',author:'',twitter_card:'',twitter_image:'',twitter_id:'',twitter_site:''}},
  {name:'images-alias',options:{image:null,images:['first.png','second.png']}},
  {name:'empty-image-array',options:{image:[]}},
  {name:'photos-fallback',options:{image:null},photos:['photo.png','/photo-root.png']},
  {name:'cover',options:{},cover:'/cover.png'},
  {name:'data-image',options:{image:'data:image/png;base64,AAAA'}},
  {name:'at-prefix',options:{twitter_id:'@already',twitter_site:'site-without-at'}},
  {name:'language-zh',options:{language:'zh-tw'}},
  {name:'language-unknown',options:{language:'unknown'}},
  {name:'date-false',options:{date:false,updated:false}},
  {name:'date-string',options:{date:'2020-01-01',updated:'2021-01-01'}},
  {name:'empty-content-null-image',options:{image:null,description:null},markdown:'',html:''},
  {name:'title-html-entities',options:{description:'Entity &amp; <b>markup</b> &#x2F; "quotes"'},title:'OG <b>Title</b> &amp; "quotes"'},
  {name:'url-query-encoding',options:{url:'https://social.example/share?word=a+b&bare&encoded=%26%3D%2F&space=hello%20world'}},
  {name:'non-string-description',options:{description:{x:1}}},
  {name:'null-array-image',options:{image:[null]}},
  {name:'invalid-url-without-images',options:{url:'https://[',image:[]},markdown:'',html:''},
  {name:'invalid-twitter-id',options:{twitter_id:true}},
  {name:'invalid-base-url',options:{url:'not-a-base',image:'image.png'}},
  {name:'invalid-image-url',options:{image:'https://['}},
  {name:'invalid-twitter-image-url',options:{twitter_image:'https://['}},
 ];
 cases.push(...[
  {
    "name": "image-unicode-host",
    "options": {
      "image": "https://例え.テスト/画像.png"
    }
  },
  {
    "name": "twitter-url-unicode-host",
    "options": {
      "twitter_image": "https://例え.テスト/画像.png"
    }
  },
  {
    "name": "url-unicode-host",
    "options": {
      "url": "https://例え.テスト/文書?検索=テスト",
      "image": "画像.png",
      "twitter_image": "twitter.png"
    }
  },
  {
    "name": "url-special-backslash",
    "options": {
      "url": "https://example.test/dir\\article?x=1",
      "image": "..\\image.png",
      "twitter_image": ".\\twitter.png"
    }
  },
  {
    "name": "url-host-normalization",
    "options": {
      "url": " https://EXAMPLE.test:443/path?x=1 ",
      "image": "image.png"
    }
  },
  {
    "name": "image-host-normalization",
    "options": {
      "image": " https://EXAMPLE.test:443/image.png ",
      "twitter_image": "HTTPS://EXAMPLE.test:443/social.png"
    }
  },
  {
    "name": "url-literal-percent",
    "options": {
      "url": "https://example.test/%oops?bad=%oops&valid=%20",
      "image": "relative%oops.png",
      "twitter_image": "twitter%oops.png"
    }
  },
  {
    "name": "opaque-mailto-relative",
    "options": {
      "url": "mailto:a@example.test",
      "image": "relative.png"
    }
  },
  {
    "name": "opaque-mailto-twitter-relative",
    "options": {
      "url": "mailto:a@example.test",
      "image": "https://example.test/image.png",
      "twitter_image": "relative.png"
    }
  },
  {
    "name": "opaque-mailto-absolute",
    "options": {
      "url": "mailto:a@example.test",
      "image": "https://example.test/image.png",
      "twitter_image": "https://example.test/twitter.png"
    }
  },
  {
    "name": "opaque-mailto-fragment",
    "options": {
      "url": "mailto:a@example.test",
      "image": "#part",
      "twitter_image": "#social"
    }
  },
  {
    "name": "opaque-data-relative",
    "options": {
      "url": "data:text/plain,data",
      "image": "image.png"
    }
  },
  {
    "name": "url-file-base",
    "options": {
      "url": "file:///tmp/dir/page",
      "image": "../image.png",
      "twitter_image": "twitter.png"
    }
  },
  {
    "name": "url-blob-absolute",
    "options": {
      "url": "blob:https://example.test/id",
      "image": "https://example.test/image.png",
      "twitter_image": "https://example.test/twitter.png"
    }
  },
  {
    "name": "image-invalid-port",
    "options": {
      "image": "https://example.test:99999/image.png"
    }
  },
  {
    "name": "url-invalid-port",
    "options": {
      "url": "https://example.test:99999/page"
    }
  },
  {
    "name": "url-empty-query",
    "options": {
      "url": "https://example.test/path?"
    }
  }
]);
 cases.push({name:'url-query-invalid-utf8',options:{url:'https://example.test/path?bad=%FF%FF&partial=%0p&incomplete=%E2%82'}});
 const entries=[];
 for(const profile of ['post','page','wiki','notebook'])for(const test of cases){
  const raw={title:test.title||'OG Title',date:'2024-02-03T04:05:06Z',updated:'2024-03-04T05:06:07Z',tags:['z-tag','a-tag'],seo:{open_graph:test.options}};
  if(profile==='wiki'||profile==='notebook')raw.collection={profile,id:'og'};
  if(test.cover)raw.cover=test.cover;if(test.photos)raw.photos=test.photos;
  const theme=schema.parseStellarConfig({themeConfig:{open_graph:{enabled:true,twitter_id:'theme-user'},canonical:{host:'canonical.fixture.test'}}});
  const frontMatter=contentConfig.parsePageConfig(raw),coll=contentConfig.parseCollectionConfig({name:'OG Collection'},'<collection>',{collectionId:'og'});
  const pagePath={post:'/post/og-fixture',page:'/og-page',wiki:'/wiki/og/entry',notebook:'/notebooks/og/entry'}[profile];
  const markdown=test.markdown===undefined?baseMarkdown:test.markdown,html=test.html===undefined?baseContent:test.html,outputs={},errors={};
  for(const flow of ['SSR','offline','preview']){
   const origin=flow==='preview'?'https://preview.test':'https://fixture.example';
   const site={title:'OG Site',url:origin+'/sub/',avatar:'/avatar.png',author:'Default Author',language:'en',description:'Site description',keywords:['site-keyword']};
   const page={title:raw.title,layout:profile==='post'?'post':'page',path:'sub'+pagePath,permalink:(flow==='preview'?origin:'https://canonical.fixture.test')+'/sub'+pagePath,content:html,excerpt:'',date:raw.date,updated:raw.updated,tags:raw.tags,lang:'en'};
   let viewModel;
   if(profile!=='page')viewModel=models[{post:'buildPostPageViewModel',wiki:'buildWikiPageViewModel',notebook:'buildNotebookPageViewModel'}[profile]]({stellarConfig:theme,siteConfig:site,frontMatter,page,collectionId:'og',collectionConfig:coll,collectionState:{},runtimeData:{}});
   const helperPage={...page,date:moment.utc(page.date),updated:moment.utc(page.updated)};if(raw.photos)helperPage.photos=raw.photos;
   const ctx={viewModel,page:helperPage,config:site,stellar_config:k=>theme[k],content_config:()=>frontMatter,first_content_image:seo.firstContentImage,gravatar:email=>`https://www.gravatar.com/avatar/${crypto.createHash('md5').update(email.toLowerCase()).digest('hex')}`,strip_html:util.stripHTML,open_graph:function(args){return helper.call({config:site,page:helperPage,url:page.permalink,is_post:()=>profile==='post'},args)},process:{env:{}},is_home:()=>false};
   try {outputs[flow]=vm.runInNewContext(script,ctx);}catch(error){errors[flow]={name:error.name,message:error.message};}
  }
  entries.push({name:profile+'/'+test.name,profile,path:pagePath,rawConfig:raw,markdown,outputs,errors});
 }
 const helperPath=require.resolve('hexo/dist/plugins/helper/open_graph');
 const fingerprints={head:crypto.createHash('sha256').update(source).digest('hex'),helper:crypto.createHash('sha256').update(fs.readFileSync(helperPath)).digest('hex')};
 fs.writeFileSync(path.join(__dirname,'opengraph.json'),JSON.stringify({commit,hexo:'8.0.0',hexoUtil:'4.0.0',hexoHelperUtil:helperRequire('hexo-util/package.json').version,fingerprints,entries},null,2)+'\n');
 console.log('Generated',entries.length,'fixed head + real helper cases (SSR/offline/preview each)');
}finally{fs.rmSync(tmp,{recursive:true,force:true})}
