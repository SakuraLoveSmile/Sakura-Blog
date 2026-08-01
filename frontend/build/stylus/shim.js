// hexo-config shim: 用 Stylus JS API 的 define() 注入 hexo-config 函数
// 模拟 Hexo 的 hexo-config('a.b.c') 行为，从 config.json 按点路径取值
// 不修改任何 .styl 源文件

const fs = require('fs')
const path = require('path')

const configPath = path.join(__dirname, 'config.json')
const config = JSON.parse(fs.readFileSync(configPath, 'utf8'))

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
// Stellar 中调用形式：hexo-config('style.font-size.body')
// 参数 param 是 stylus nodes.String，取其 val 属性得到 JS 字符串
function hexoConfig(param) {
  const keyPath = param && param.val ? param.val : String(param)
  const jsVal = getByPath(config, keyPath)
  return toStylusValue(jsVal)
}

// 注册到 stylus renderer
function setup(renderer) {
  renderer.define('hexo-config', hexoConfig)
}

module.exports = setup
module.exports.hexoConfig = hexoConfig
module.exports.config = config
