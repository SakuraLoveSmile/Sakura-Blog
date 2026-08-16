// 编译脚本：把 Stellar 的 main.styl 编译成 main.css
// 用法: node build/stylus/compile.js
//
// Stellar 1.41 将 swiper/fancybox 与评论样式（artalk 等）移出 main.styl 编译入口
// （上游改为按需外置 `<link>` 加载），本项目不引入按需加载，继续单文件 main.css，
// 故在主入口之后追加编译这些移出的样式段，产物等效 1.40 及之前的编译范围。

const fs = require('fs')
const path = require('path')
const stylus = require('stylus')

// 主题源在仓库容器根（worktree 的上一层）：stylus → build → web → stellar → modules → worktree 根 → 容器根，共 6 层
const STELLAR_CSS_DIR = path.resolve(__dirname, '../../../../../../resource/hexo-theme-stellar/source/css')
const OUT_CSS = path.resolve(__dirname, '../../assets/css/main.css')

// 追加编译的样式段（1.40 主入口曾包含、1.41 移出；顺序对齐 1.40 产物内的出现顺序）
const EXTRA_STYLES = [
  'plugins/swiper.styl',
  'plugins/fancybox.styl',
  'comments/artalk.styl',
]

function render(stylPath) {
  return new Promise((resolve, reject) => {
    const renderer = stylus(fs.readFileSync(stylPath, 'utf8'))
      .set('filename', stylPath)     // 让 @import 能解析相对路径
      .set('paths', [STELLAR_CSS_DIR]) // 额外搜索路径
      .set('compress', false)          // 开发期不压缩，便于排查
      .set('sourcemap', false)
    // 注入 hexo-config shim
    require('./shim')(renderer)
    renderer.render((err, css) => (err ? reject(err) : resolve(css)))
  })
}

async function main() {
  const chunks = [await render(path.join(STELLAR_CSS_DIR, 'main.styl'))]
  for (const rel of EXTRA_STYLES) {
    chunks.push(await render(path.join(STELLAR_CSS_DIR, rel)))
  }
  const css = chunks.join('\n')
  fs.mkdirSync(path.dirname(OUT_CSS), { recursive: true })
  fs.writeFileSync(OUT_CSS, css)
  const sizeKB = (Buffer.byteLength(css) / 1024).toFixed(1)
  console.log(`编译成功: ${OUT_CSS} (${sizeKB} KB，追加段: ${EXTRA_STYLES.join(', ')})`)
}

main().catch((err) => {
  console.error('编译失败:')
  console.error(err.message)
  if (err.stack) console.error(err.stack)
  process.exit(1)
})
