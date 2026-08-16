// 编译脚本：把 Stellar 的 main.styl 编译成 main.css
// 用法: node build/stylus/compile.js

const fs = require('fs')
const path = require('path')
const stylus = require('stylus')

const STELLAR_CSS_DIR = path.resolve(__dirname, '../../../../resource/hexo-theme-stellar/source/css')
const MAIN_STYL = path.join(STELLAR_CSS_DIR, 'main.styl')
const OUT_CSS = path.resolve(__dirname, '../../assets/css/main.css')

const src = fs.readFileSync(MAIN_STYL, 'utf8')

const renderer = stylus(src)
  .set('filename', MAIN_STYL)     // 让 @import 能解析相对路径
  .set('paths', [STELLAR_CSS_DIR]) // 额外搜索路径
  .set('compress', false)          // 开发期不压缩，便于排查
  .set('sourcemap', false)

// 注入 hexo-config shim
require('./shim')(renderer)

renderer.render((err, css) => {
  if (err) {
    console.error('编译失败:')
    console.error(err.message)
    if (err.stack) console.error(err.stack)
    process.exit(1)
  }
  fs.mkdirSync(path.dirname(OUT_CSS), { recursive: true })
  fs.writeFileSync(OUT_CSS, css)
  const sizeKB = (Buffer.byteLength(css) / 1024).toFixed(1)
  console.log(`编译成功: ${OUT_CSS} (${sizeKB} KB)`)
})
