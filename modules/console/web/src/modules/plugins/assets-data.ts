// 素材速查静态数据（表情）：名称清单取自 cdn-x/emoticons@3.1.0 官方 CDN 目录（jsDelivr tree API），
// twemoji 为精选常用 Unicode 码点。来源 URL 模板与后端 shortcode.go emojiSources 保持一致，
// 新增/修改表情源时两侧需同步。

export interface EmojiSource {
  /** 来源标识，对应 {% emoji <id> name %}；default 可省略来源直接 {% emoji name %} */
  id: string;
  label: string;
  /** CDN URL 模板（{name} 占位），供画廊预览 */
  urlTemplate: string;
  names: string[];
}

const QQ = "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/qq/{name}.gif";
const ARU = "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/aru/{name}.gif";
const TIEBA = "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/tieba/{name}.png";
const BLOBCAT = "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/blobcat/{name}.gif";
const TWEMOJI = "https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/svg/{name}.svg";

export const emojiSources: EmojiSource[] = [
  {
    id: "default",
    label: "QQ 表情（默认源）",
    urlTemplate: QQ,
    names: ["NO","OK","aini","aiqing","aoman","baiyan","bangbangtang","baojin","baoquan","bianbian","bianpao","bishi","bizui","cahan","caidao","chajin","che","chexiang","ciya","dabing","daku","dangao","dao","deng","denglong","deyi","diaoxie","duoyun","facai","fadai","fadou","fan","fanu","feiji","feiwen","fendou","fengche","ganga","gaotieyouchetou","gaotiezuochetou","gouwu","gouyin","guzhang","haixiu","hanxiao","haqian","hecai","huaixiao","huatong","huidong","huitou","ji-e","jidong","jiewu","jiezhi","jingkong","jingya","kafei","keai","kelian","ketou","koubi","ku","kuaikule","kulou","kun","lanqiu","lazhu","lenghan","liuhan","liulei","liwu","meigui","miantiao","naiping","nanguo","naohuo","naozhong","piaochong","piezui","pijiu","pingpang","qian","qiang","qiaoda","qingwa","qinqin","qiqiu","qiudale","quantou","ruo","se","shafa","shandian","shengli","shouqiang","shuai","shuangxi","shui","taiyang","tiaopi","tiaosheng","tiaotiao","touxiao","tu","weiqu","weixiao","wen","wushou","xia","xiangjiao","xianwen","xiayu","xigua","xin","xinsui","xiongmao","xu","yaowan","yewan","yinxian","yiwen","yongbao","youhengheng","youjian","youtaiji","yun","yusan","zaijian","zhadan","zhemo","zhijin","zhouma","zhuakuang","zhuanquan","zhutou","zuohengheng","zuotaiji","zuqiu"],
  },
  {
    id: "tieba",
    label: "贴吧表情",
    urlTemplate: TIEBA,
    names: ["a","aini","aixin","bianbian","bishi","bugaoxing","caihong","cha","damuzhi","dangao","dengpao","guai","haha","han","hehe","heixian","honglingjin","huaji","huaxin","jingku","jingya","ku","kuanghan","landeli","lazhu","lei","liwu","meigui","mianqiang","nidongde","nu","ok","pen","qianbi","shafa","shengli","shouzhi","shuijiao","suanshuang","taikaixin","taiyang","tu","tushe","wabi","weiqu","what","wuzuixiao","xiangjiao","xiaoguai","xiaohonglian","xiaoku","xiaoyan","xili","xingxingyueliang","xinsui","yamete","yaowan","yinxian","yinyue","yiwen","zhenbang"],
  },
  {
    id: "aru",
    label: "阿鲁表情",
    urlTemplate: ARU,
    names: ["aru0000","aru0010","aru0020","aru0030","aru0040","aru0050","aru0060","aru0070","aru0080","aru0090","aru0100","aru0110","aru0120","aru0130","aru0140","aru0150","aru0160","aru0170","aru0171","aru0180","aru0181","aru0190","aru0200","aru0210","aru0220","aru0230","aru0240","aru0241","aru0250","aru0260","aru0270","aru0280","aru0290","aru0300","aru0310","aru0320","aru0330","aru0331","aru0340","aru0350","aru0360","aru0370","aru0380","aru0390","aru0391","aru0400","aru0410","aru0420","aru0430","aru0431","aru0440","aru0450","aru0451","aru0460","aru0461","aru0462","aru0470","aru0480","aru0490","aru0500","aru0510","aru0520","aru0530","aru0540","aru0550","aru0560","aru0570","aru0580","aru0590","aru0600","aru0610","aru0620","aru1010","aru1011","aru1020","aru1030","aru1040","aru1050","aru1060","aru1070","aru1080","aru2010","aru2020","aru2030","aru2040","aru2050","aru2060","aru2070","aru2080","aru2090","aru2100","aru2101","aru2110","aru2120","aru2130","aru2140","aru2141","aru2145","aru2150","aru2151","aru2152","aru2160","aru2161","aru2165","aru2170","aru2171","aru3010","aru3011","aru3020","aru3030","aru3040","aru3050","aru3060","aru3061","aru4010","aru4020","aru4021","aru4022","aru4023","aru4024","aru4025","aru4030","aru4031","aru4040","aru4041","aru4042","aru4100","aru4110","aru4111","aru4112","aru4113","aru4114","aru4115","aru4120","aru4121","aru4130","aru4140","aru4200","aru4201","aru4210","aru4211","aru4212","aru4213","aru4220","aru4300","aru4301","aru4302","aru4303","aru4304","aru4310","aru5000","aru5001","aru5002","aru5003","aru5010","aru5020","aru5030","aru5040","aru5050","aru5060","aru5069","aru5070","aru5080","aru5090","aru5091","aru5100","aru5110","aru5120","aru5130","aru5131","aru5140","aru5141","aru5150","aru5151","aru5160","aru5161","aru5170","aru5180","aru5190","aru5200","aru5210","aru5220","aru5230","aru6000","aru6001","aru6010","aru6020","aru6030","aru6040","aru6050","aru6051","aru6060","aru6061","aru6070","aru6080","aru6090","aru6091","aru6092","aru6100","aru6110","aru6120","aru6121","aru6130","aru6131","aru6132","aru6140","aru6150","aru6151","aru6160","aru6500","aru6510","aru6520","aru6530","aru7000","aru7010","aru7011","aru7012","aru7020","aru7021","aru7030","aru7040","aru7045","aru7046","aru7050","aru7051","aru7052","aru7053","aru7054","aru7060","aru7061","aru7062","aru7070","aru7071","aru7080","aru7081","aru7090","aru7100","aru7110","aru7111","aru7112","aru7113","aru7200","aru7201","aru7210","aru7220","aru7230","aru8000","aru8001","aru8010","aru8020","aru8021","aru8030","aru8040","aru8041","aru8042","aru8050","aru8051","aru8060","aru8062","aru8070","aru8071","aru8072","aru8080","aru8081","aru8082","aru8083","aru8084","aru8085","aru8090","aru8091","aru8092","aru8100","aru8101","aru8102","aru8103","aru8110","aru8111","arux010","arux011","arux012","arux013","arux014","arux021","arux022","arux031","arux032","arux041","arux042","arux050","arux060","arux061","arux062","arux063","arux069","arux070","arux071","arux080","arux081","arux082","arux083","arux090","arux091","arux092","arux093","arux094","arux099"],
  },
  {
    id: "blobcat",
    label: "猫咪表情",
    urlTemplate: BLOBCAT,
    names: ["0_0","amused","angel","angery","angry","angry2","angrypuff","approach","attention","attentionrev","aww","banhammerr","blush","bongo","bongo1","bongofaster","bongohyper","bop","bounce","bouncefast","box","broken","caged","clipboard","cmereyou","code","coffee","coffee2","coffeespit","comftears","comfy","comfynight","comfyslep","comfysweat","comfyuee","cool","couple","cozywave","cry","cyclone","dab","dancing","dead","dealwithit","derp","died","disturbed","drool","enjoy","enjoyrain","enjoysnow","evil","facepalm","fearful","fingerguns","flip","flower","gay","glare","glarezoom","glassesdown","googlycry","googlytrash","happypaws","headphones","heart","heartbreak","heartbroken","hearteyes","heartpride","heartrainbow","hero","highfive","hissss","hug2","hungry","hyperthink","idea","kissheart","love","lurk","maracas","maracasfast","melt","meltsob","meltsoblove","meltthumb","nervous","nervouspleading","neutral","nod","nodfast","nogood","nom","nomcookie","notif","notlikethis","openmouth","owo","party","party2","party2fast","party2faster","peek","peek2","peekaboo","peekwave","pensivepray","photo","police","presentred","rave","reach","reachrev","read","reeee","rollingeyes","sad","sadreach","saitama","scared","shocked","sip","sippy","sipsweat","sipzoom","smiletear","snapped","snuggle","sob","sobglasses","sparkle","spin","spinrainbow","stretchy1","stretchy2","stretchy3","supersayin","sweats","tap","think","thinking","thinkingcool","thinksmirk","tired","trampoline","triumph","umm","verified","waitwhat","wave","wavemelt","waverev","whaaaaat","woah","wobble","yandere"],
  },
  {
    id: "twemoji",
    label: "Twemoji（Unicode 码点）",
    urlTemplate: TWEMOJI,
    names: ["1f44d","1f44e","1f44f","1f600","1f602","1f60d","1f62d","1f631","1f914","1f634","1f60e","1f973","1f44b","1f4aa","1f64f","1f44c","2764","1f494","1f525","2b50","1f389","1f381","1f31f","26a1","1f308","2615","1f37a","1f4bb","1f680","1f30d"],
  },
];
