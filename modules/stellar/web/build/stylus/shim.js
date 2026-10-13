// hexo-config shim: 用 Stylus JS API 的 define() 注入 hexo-config 函数
// 模拟 Hexo 的 hexo-config('a.b.c') 行为，从 config.json 按点路径取值
// 不修改任何 .styl 源文件

const fs = require('fs')
const path = require('path')

const configPath = path.join(__dirname, 'config.json')
const config = JSON.parse(fs.readFileSync(configPath, 'utf8'))

// 将已有设置注入的变量接到 2.0 编译期配置，未设置时保留原默认值。
// 新七档圆角优先，历史四档作为下一层 fallback；不改上游源或配置 schema。
const runtimeVariables = {
  'appearance.typography.font_family.body': ['--font-family-body'],
  'appearance.typography.font_family.code': ['--font-family-code'],
  'appearance.typography.font_size.root': ['--font-size-root'],
  'appearance.typography.font_size.inline_code': ['--font-size-inline-code'],
  'appearance.typography.font_size.code_block': ['--font-size-code-block'],
  'appearance.typography.font_smoothing': ['--font-smoothing'],
  'appearance.typography.content_align': ['--content-align'],
  'appearance.shape.corner': ['--corner-shape'],
  'appearance.code_block.scrollbar_width': ['--code-scrollbar-width'],
  'appearance.shape.radius.card_large': ['--radius-card-l', '--radius-card'],
  'appearance.shape.radius.card': ['--radius-card'],
  'appearance.shape.radius.card_small': ['--radius-card-s', '--radius-block'],
  'appearance.shape.radius.bar': ['--radius-bar', '--radius-navbar'],
  'appearance.shape.radius.image_large': ['--radius-image-l', '--radius-image'],
  'appearance.shape.radius.image': ['--radius-image'],
  'appearance.shape.radius.image_small': ['--radius-image-s', '--radius-image'],
  'appearance.gradients.primary_action': ['--gradient-start'],
  'appearance.gradients.search_bar': ['--gradient-searchbar'],
}
for (let weight = 100; weight <= 900; weight += 100) {
  runtimeVariables[`appearance.typography.font_weight.${weight}`] = [`--font-weight-${weight}`]
}

// 按点路径取值，缺失返回 undefined（复现 Hexo hexo-config 的 falsy 行为）
function getByPath(obj, keyPath) {
  if (!keyPath || typeof keyPath !== 'string') return undefined
  const parts = keyPath.split('.')
  let cur = obj
  for (const p of parts) {
    if (cur == null || typeof cur !== 'object') return undefined
    cur = cur[p]
  }
  return cur
}

// 将 JS 值转为 Stylus 兼容的返回值
// Stylus 的 define() 接受 JS 原生值并自动转换：
//   JS string -> nodes.String
//   JS boolean -> nodes.Boolean
//   JS number -> nodes.Unit
//   JS array -> nodes.List
//   JS null/undefined -> nodes.Null (falsy，使 if 分支不进入)
function toStylusValue(jsVal) {
  const stylus = require('stylus')
  const nodes = stylus.nodes

  if (jsVal === undefined || jsVal === null) {
    return nodes.null
  }
  if (typeof jsVal === 'boolean') {
    return new nodes.Boolean(jsVal)
  }
  if (typeof jsVal === 'number') {
    return new nodes.Unit(jsVal)
  }
  if (typeof jsVal === 'string') {
    return new nodes.String(jsVal)
  }
  if (Array.isArray(jsVal)) {
    // Stylus List: 用逗号分隔的 expression
    const list = new nodes.Expression()
    for (const item of jsVal) {
      list.push(toStylusValue(item))
    }
    return list
  }
  // 对象等其他类型：JSON 字符串化
  return new nodes.String(JSON.stringify(jsVal))
}

// hexo-config shim 函数
// Stellar 中调用形式：hexo-config('appearance.typography.font_size.root')
// 参数 param 是 stylus nodes.String，取其 val 属性得到 JS 字符串
function hexoConfig(param, source = config) {
  const keyPath = param && param.val ? param.val : String(param)
  let jsVal = getByPath(source, keyPath)
  if ((typeof jsVal === 'string' || typeof jsVal === 'number') && runtimeVariables[keyPath]) {
    jsVal = runtimeVariables[keyPath].reduceRight((value, variable) => `var(${variable}, ${value})`, jsVal)
    // 字重在上游直接作为 CSS 值使用，没有 convert() 去除 String 的引号。
    // 运行时变量本身是 CSS 表达式，不是需要引号包裹的配置文案。
    return new (require('stylus').nodes.Literal)(jsVal)
  }
  return toStylusValue(jsVal)
}

// 注册到 stylus renderer
function setup(renderer, source = config) {
  renderer.define('hexo-config', (param) => hexoConfig(param, source))
}

module.exports = setup
module.exports.hexoConfig = hexoConfig
module.exports.config = config
module.exports.runtimeVariables = runtimeVariables
