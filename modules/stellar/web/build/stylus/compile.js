// 从 upstream.json 锁定的干净 Stellar 源生成全部整站外观。
// 用法：node build/stylus/compile.js [--clean]；保留旧参数，所有构建都锁定源。
// main.css 是 card 的兼容入口；整站选择外观时只加载对应 main-<preset>.css。
// 上游源与共享 resource 工作树不被修改。

const fs = require('fs')
const os = require('os')
const path = require('path')
const { execFileSync } = require('child_process')
const stylus = require('stylus')
const setup = require('./shim')

const REPO_DIR = path.resolve(__dirname, '../../../../../resource/hexo-theme-stellar')
const OUT_DIR = path.resolve(__dirname, '../../assets/css')
const PRESETS = ['card', 'glass', 'minimal', 'flat']
const EXTRA_STYLES = ['plugins/swiper.styl', 'plugins/fancybox.styl', 'comments/artalk.styl']
// 可选样式保留上游顺序：加载选配后，再加载 main-appearance-<preset>.css。
// 尤其 tilt 的列表阴影须由 Minimal/Flat 的最终 Appearance 再次收口。
const DEFINES = [
  "@import '_defines/const'",
  "@import '_custom'",
  "@import '_defines/theme_base'",
  "@import '_defines/theme_colorful'",
  "@import '_defines/func'",
].join('\n')

function render(stylPath, searchDir, config) {
  return renderSource(fs.readFileSync(stylPath, 'utf8'), stylPath, searchDir, config)
}

function renderSource(source, filename, searchDir, config) {
  return new Promise((resolve, reject) => {
    const renderer = stylus(source)
      .set('filename', filename)
      .set('paths', [searchDir])
      .set('compress', false)
      .set('sourcemap', false)
    setup(renderer, config)
    renderer.render((err, css) => (err ? reject(err) : resolve(css)))
  })
}

async function compile({ repositoryDir = REPO_DIR, outputDir = OUT_DIR, config = setup.config } = {}) {
  const upstream = require('./upstream.json')
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'stellar-css-clean-'))
  try {
    // 不经 shell 插值，也不 checkout/reset 共用源；从锁定提交读取文件。
    // pre-commit 会导出当前 Blog 仓库的 Git 本地环境，cwd 本身无法覆盖它。
    const gitEnv = { ...process.env }
    for (const key of ['GIT_DIR', 'GIT_WORK_TREE', 'GIT_COMMON_DIR', 'GIT_INDEX_FILE', 'GIT_PREFIX', 'GIT_OBJECT_DIRECTORY', 'GIT_ALTERNATE_OBJECT_DIRECTORIES', 'GIT_IMPLICIT_WORK_TREE', 'GIT_GRAFT_FILE', 'GIT_SHALLOW_FILE', 'GIT_INTERNAL_SUPER_PREFIX']) {
      delete gitEnv[key]
    }
    const archive = execFileSync('git', ['archive', upstream.commit, 'source/css'], { cwd: repositoryDir, env: gitEnv })
    execFileSync('tar', ['-x', '-C', tmpDir], { input: archive })
    const cssDir = path.join(tmpDir, 'source/css')
    const outputs = {}
    for (const preset of PRESETS) {
      const presetConfig = { ...config, appearance: { ...config.appearance, preset } }
      const chunks = [await render(path.join(cssDir, 'main.styl'), cssDir, presetConfig)]
      for (const rel of EXTRA_STYLES) {
        chunks.push(await render(path.join(cssDir, rel), cssDir, presetConfig))
      }
      chunks.push(await render(path.join(__dirname, 'local.styl'), cssDir, presetConfig))
      outputs[`main-${preset}.css`] = chunks.join('\n')
      // _custom/theme_base 含全局声明，不能重复输出到最终层；先借其定义编译，
      // 再截取明确的哨兵之后的 Appearance 实现（不解析或重写上游规则）。
      const appearance = await renderSource(`${DEFINES}\n.stellar-appearance-boundary\n  display: none\n@require '_appearances/_mixins'\n@import '_appearances/${preset}'`, path.join(cssDir, 'appearance.styl'), cssDir, presetConfig)
      outputs[`main-appearance-${preset}.css`] = afterBoundary(appearance)
    }
    for (const [feature, flags] of Object.entries({ spotlight: { spotlight: true, tilt: false }, tilt: { spotlight: false, tilt: true } })) {
      const featureConfig = { ...config, features: { ...config.features, card_hover: flags } }
      const css = await renderSource(`${DEFINES}\n.stellar-appearance-boundary\n  display: none\n@import '_plugins/card-hover'`, path.join(cssDir, 'feature.styl'), cssDir, featureConfig)
      outputs[`main-card-hover-${feature}.css`] = afterBoundary(css)
    }
    for (const [name, source, featureConfig] of [
      ['main-katex.css', '_plugins/katex', config],
      ['main-lazyload-blur.css', '_plugins/lazyload', { ...config, features: { ...config.features, lazy_loading: { ...config.features.lazy_loading, transition: 'blur' } } }],
    ]) {
      const css = await renderSource(`${DEFINES}\n.stellar-appearance-boundary\n  display: none\n@import '${source}'`, path.join(cssDir, 'feature.styl'), cssDir, featureConfig)
      outputs[name] = afterBoundary(css)
    }
    outputs['main.css'] = outputs['main-card.css']
    // 所有外观编译成功后才写产物，避免某个预设失败留下半套新文件。
    fs.mkdirSync(outputDir, { recursive: true })
    for (const [name, css] of Object.entries(outputs)) {
      fs.writeFileSync(path.join(outputDir, name), css)
    }
    return outputs
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true })
  }
}

function afterBoundary(css) {
  const boundary = '.stellar-appearance-boundary {\n  display: none;\n}\n'
  const index = css.indexOf(boundary)
  if (index < 0) throw new Error('Stylus 编译边界丢失')
  return css.slice(index + boundary.length)
}

if (require.main === module) {
  compile().then((outputs) => {
    const upstream = require('./upstream.json')
    console.log(`使用锁定干净上游版本编译 (commit: ${upstream.commit.slice(0, 8)}, tag: ${upstream.tag})`)
    for (const [name, css] of Object.entries(outputs)) {
      console.log(`编译成功: ${path.join(OUT_DIR, name)} (${(Buffer.byteLength(css) / 1024).toFixed(1)} KB)`)
    }
  }).catch((err) => {
    console.error('编译失败:')
    console.error(err.stack || err.message)
    process.exitCode = 1
  })
}

module.exports = { compile, renderSource, PRESETS }
