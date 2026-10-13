// 编译脚本：把 Stellar 的 main.styl 编译成 main.css
// 用法: node build/stylus/compile.js [--clean]
//
// Stellar 2.0 的主入口使用 appearance/tags/features 配置；swiper/fancybox 与评论
// 样式仍由上游按需外置加载。本项目继续单文件 main.css，追加这些样式与本地兼容层。
//
// --clean 选项：从 upstream.json 锁定的 git commit 提取干净 Stylus 源编译，
// 避免工作树脏状态或本地改动污染产物。

const fs = require('fs')
const os = require('os')
const path = require('path')
const { execSync } = require('child_process')
const stylus = require('stylus')

const REPO_DIR = path.resolve(__dirname, '../../../../../resource/hexo-theme-stellar')
const DEFAULT_STELLAR_CSS_DIR = path.join(REPO_DIR, 'source/css')
const OUT_CSS = path.resolve(__dirname, '../../assets/css/main.css')

// 单文件中继续保留轮播、灯箱与 Artalk 的主题样式。
const EXTRA_STYLES = [
  'plugins/swiper.styl',
  'plugins/fancybox.styl',
  'comments/artalk.styl',
]

function render(stylPath, searchDir) {
  return new Promise((resolve, reject) => {
    const renderer = stylus(fs.readFileSync(stylPath, 'utf8'))
      .set('filename', stylPath)     // 让 @import 能解析相对路径
      .set('paths', [searchDir])     // 额外搜索路径
      .set('compress', false)        // 开发期不压缩，便于排查
      .set('sourcemap', false)
    // 注入 hexo-config shim
    require('./shim')(renderer)
    renderer.render((err, css) => (err ? reject(err) : resolve(css)))
  })
}

async function main() {
  const isClean = process.argv.includes('--clean')
  let cssDir = DEFAULT_STELLAR_CSS_DIR
  let tmpDir = null

  if (isClean) {
    const upstream = require('./upstream.json')
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'stellar-css-clean-'))
    try {
      execSync(`git archive ${upstream.commit} source/css | tar -x -C "${tmpDir}"`, { cwd: REPO_DIR })
      cssDir = path.join(tmpDir, 'source/css')
      console.log(`使用锁定干净上游版本编译 (commit: ${upstream.commit.slice(0, 8)}, tag: ${upstream.tag})`)
    } catch (e) {
      fs.rmSync(tmpDir, { recursive: true, force: true })
      throw new Error(`从锁定 commit 提取干净源失败: ${e.message}`)
    }
  }

  try {
    const chunks = [await render(path.join(cssDir, 'main.styl'), cssDir)]
    for (const rel of EXTRA_STYLES) {
      chunks.push(await render(path.join(cssDir, rel), cssDir))
    }
    chunks.push(await render(path.join(__dirname, 'local.styl'), cssDir))
    const css = chunks.join('\n')
    fs.mkdirSync(path.dirname(OUT_CSS), { recursive: true })
    fs.writeFileSync(OUT_CSS, css)
    const sizeKB = (Buffer.byteLength(css) / 1024).toFixed(1)
    console.log(`编译成功: ${OUT_CSS} (${sizeKB} KB，追加段: ${EXTRA_STYLES.join(', ')})`)
  } finally {
    if (tmpDir) {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    }
  }
}

main().catch((err) => {
  console.error('编译失败:')
  console.error(err.message)
  if (err.stack) console.error(err.stack)
  process.exit(1)
})

