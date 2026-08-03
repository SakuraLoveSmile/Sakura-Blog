// tagcloud.js: 标签云 widget 交互（按 resource/design/tagcloud.design 重设计）。
// 动态模式：3x3 平铺无缝循环拖拽云（自动漂移 / 拖拽惯性 / 点击涟漪）；
// 静态模式：SSR 静态云兜底（JS 不可用时默认可见）。
// 全站 defer 加载：遍历 .widget-wrapper.tagcloud 存在性初始化（支持多实例）+
// pjax:complete 重初始化，元素移出 DOM 后动画循环自清理；数据源为 SSR 静态云 DOM，不发请求。
(function () {
  'use strict';

  var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var instances = [];

  function frac(x) { return x - Math.floor(x); }

  function shuffle(arr) {
    var a = arr.slice();
    for (var i = a.length - 1; i > 0; i--) {
      var j = Math.floor(Math.random() * (i + 1));
      var t = a[i]; a[i] = a[j]; a[j] = t;
    }
    return a;
  }

  function create(root) {
    var cloud = root.querySelector('.tag-cloud');
    var inner = cloud && cloud.querySelector('.tag-cloud-inner');
    var staticCloud = root.querySelector('.tag-cloud-static');
    var modeSwitch = root.querySelector('.tag-mode-switch');
    var hint = root.querySelector('.tag-hint');
    if (!cloud || !inner || !staticCloud || !modeSwitch || !hint) return null;

    // ── 数据：从 SSR 静态云读 name/href/title/level（唯一数据源）──
    var tags = [];
    staticCloud.querySelectorAll('a.tag-static').forEach(function (a) {
      var m = a.className.match(/tag-(xl|lg|md|sm|xs)/);
      tags.push({
        name: a.textContent.trim(),
        href: a.getAttribute('href'),
        title: a.getAttribute('title') || '',
        level: m ? m[1] : 'md'
      });
    });
    if (!tags.length) return null;

    var cloudWidth = 0, cloudHeight = 0;
    var offsetX = 0, offsetY = 0, velocityX = 0, velocityY = 0;
    var isDragging = false, autoDrift = false, driftTime = 0;
    var lastX = 0, lastY = 0, startX = 0, startY = 0;
    var hasDragged = false, suppressClick = false, suppressTimer = 0;
    var rafId = 0, stopped = false, mode = 'static';

    // 在一个 tile 内铺标签：6 列 x 4 行错位网格 + 确定性小偏移（sin 种子，各 tile 内容一致才可无缝循环）。
    // 槽位有限（24）时按洗牌序循环取标签，保证所有标签都进入动态云（多出的标签次轮出现）。
    function fillTile(tileEl, list) {
      var cols = 6, rows = 4, slots = cols * rows;
      for (var i = 0; i < slots; i++) {
        var item = list[i % list.length];
        var row = Math.floor(i / cols);
        var col = i % cols;
        var colOffset = (row % 2 === 1) ? 0.5 : 0;
        var seed = row * 31 + col * 17;
        var r1 = frac(Math.sin(seed) * 10000);
        var r2 = frac(Math.sin(seed + 99) * 10000);
        var a = document.createElement('a');
        a.className = 'tag tag-' + item.level;
        a.textContent = item.name;
        a.href = item.href;
        a.title = item.title;
        a.style.left = (((col + colOffset) / cols) * 100 + (r1 - 0.5) * 5) + '%';
        a.style.top = (((row + 0.5) / rows) * 100 + (r2 - 0.5) * 6) + '%';
        a.style.setProperty('--delay', (r1 * 0.5 + 0.15).toFixed(2) + 's');
        tileEl.appendChild(a);
      }
    }

    function buildTiles() {
      cloudWidth = cloud.offsetWidth;
      cloudHeight = cloud.offsetHeight;
      inner.textContent = '';
      var list = shuffle(tags);
      for (var ty = 0; ty < 3; ty++) {
        for (var tx = 0; tx < 3; tx++) {
          var tile = document.createElement('div');
          tile.className = 'tag-tile';
          tile.style.left = (tx * 33.333) + '%';
          tile.style.top = (ty * 33.333) + '%';
          fillTile(tile, list);
          inner.appendChild(tile);
        }
      }
    }

    function applyTransform() {
      inner.style.transform = 'translate3d(' + offsetX + 'px, ' + offsetY + 'px, 0)';
    }

    // 无缝循环：偏移超出一个 tile（即视口尺寸）时归位，视觉上完全一致
    function wrapOffset() {
      if (!cloudWidth || !cloudHeight) return;
      while (offsetX >= cloudWidth) offsetX -= cloudWidth;
      while (offsetX < 0) offsetX += cloudWidth;
      while (offsetY >= cloudHeight) offsetY -= cloudHeight;
      while (offsetY < 0) offsetY += cloudHeight;
    }

    function createRipple(x, y) {
      var rect = cloud.getBoundingClientRect();
      var r = document.createElement('div');
      r.className = 'ripple';
      r.style.left = (x - rect.left) + 'px';
      r.style.top = (y - rect.top) + 'px';
      r.style.width = '40px';
      r.style.height = '40px';
      cloud.appendChild(r);
      setTimeout(function () { r.remove(); }, 600);
    }

    function animate() {
      if (stopped || !cloud.isConnected) { stop(); return; }
      if (!isDragging && autoDrift && mode === 'dynamic') {
        driftTime += 0.008;
        offsetX += Math.sin(driftTime) * 0.15;
        offsetY += Math.cos(driftTime * 0.7) * 0.08 + Math.sin(driftTime * 1.3) * 0.05;
        wrapOffset();
        applyTransform();
      }
      // 拖拽惯性（衰减至阈值以下停止）
      if (!isDragging && (Math.abs(velocityX) > 0.05 || Math.abs(velocityY) > 0.05)) {
        offsetX += velocityX;
        offsetY += velocityY;
        velocityX *= 0.95;
        velocityY *= 0.95;
        wrapOffset();
        applyTransform();
      }
      rafId = requestAnimationFrame(animate);
    }

    function onStart(e) {
      if (mode !== 'dynamic') return;
      isDragging = true;
      autoDrift = false;
      hasDragged = false;
      cloud.classList.add('grabbing');
      var p = e.touches ? e.touches[0] : e;
      lastX = startX = p.clientX;
      lastY = startY = p.clientY;
      velocityX = velocityY = 0;
    }

    function onMove(e) {
      if (!isDragging) return;
      var p = e.touches ? e.touches[0] : e;
      var dx = p.clientX - lastX;
      var dy = p.clientY - lastY;
      if (Math.abs(p.clientX - startX) > 5 || Math.abs(p.clientY - startY) > 5) hasDragged = true;
      // 触屏与侧栏外页面纵向滚动共存（touch-action: pan-y），仅取水平位移；鼠标拖拽双轴自由
      velocityX = dx;
      velocityY = e.touches ? 0 : dy;
      offsetX += dx;
      offsetY += e.touches ? 0 : dy;
      wrapOffset();
      applyTransform();
      lastX = p.clientX;
      lastY = p.clientY;
    }

    function onEnd() {
      if (!isDragging) return;
      isDragging = false;
      cloud.classList.remove('grabbing');
      if (hasDragged) {
        // 拖拽结束后抑制紧随的 click（防止误触跳转标签页）
        suppressClick = true;
        clearTimeout(suppressTimer);
        suppressTimer = setTimeout(function () { suppressClick = false; }, 350);
      }
      // 停顿后恢复自动漂移（reduced-motion 下永不恢复）
      var delay = hasDragged ? 2500 : 500;
      setTimeout(function () { if (!isDragging && mode === 'dynamic' && !stopped) autoDrift = !reduced; }, delay);
    }

    // 捕获阶段拦截：拖拽后吞掉 click（pjax.js 的 document 冒泡监听在捕获被 stopPropagation 后不会触发）；
    // 正常点击放行并补一个涟漪反馈，跳转交给默认行为 / PJAX。
    function onClickCapture(e) {
      if (suppressClick) {
        suppressClick = false;
        e.preventDefault();
        e.stopPropagation();
        return;
      }
      if (e.target.closest && e.target.closest('a.tag')) {
        createRipple(e.clientX, e.clientY);
      }
    }

    // 鼠标悬停暂停漂移，移出恢复
    function onEnter() { if (!isDragging) autoDrift = false; }
    function onLeave() { if (!isDragging && mode === 'dynamic') autoDrift = !reduced; }

    function onResize() {
      cloudWidth = cloud.offsetWidth;
      cloudHeight = cloud.offsetHeight;
      wrapOffset();
      applyTransform();
    }

    // ── 模式切换 ──
    function switchMode(m) {
      if (m === mode) return;
      mode = m;
      modeSwitch.querySelectorAll('button').forEach(function (b) {
        b.classList.toggle('active', b.dataset.mode === m);
      });
      if (m === 'static') {
        cloud.setAttribute('hidden', '');
        staticCloud.removeAttribute('hidden');
        autoDrift = false;
        hint.textContent = hint.dataset.hintSorted;
      } else {
        staticCloud.setAttribute('hidden', '');
        cloud.removeAttribute('hidden');
        autoDrift = !reduced;
        hint.textContent = '';
        var dot = document.createElement('span');
        dot.className = 'dot';
        var txt = document.createElement('span');
        txt.textContent = hint.dataset.hintDrag;
        hint.appendChild(dot);
        hint.appendChild(txt);
      }
      root.classList.toggle('is-static', m === 'static');
    }

    function onModeClick(e) {
      var b = e.target.closest ? e.target.closest('button[data-mode]') : null;
      if (b) switchMode(b.dataset.mode);
    }

    function stop() {
      if (stopped) return;
      stopped = true;
      cancelAnimationFrame(rafId);
      clearTimeout(suppressTimer);
      cloud.removeEventListener('mousedown', onStart);
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onEnd);
      cloud.removeEventListener('touchstart', onStart);
      document.removeEventListener('touchmove', onMove);
      document.removeEventListener('touchend', onEnd);
      cloud.removeEventListener('click', onClickCapture, true);
      cloud.removeEventListener('mouseenter', onEnter);
      cloud.removeEventListener('mouseleave', onLeave);
      modeSwitch.removeEventListener('click', onModeClick);
      window.removeEventListener('resize', onResize);
    }

    buildTiles();
    modeSwitch.addEventListener('click', onModeClick);
    rafId = requestAnimationFrame(animate);
    switchMode('dynamic'); // 设计稿默认动态模式；静态保留为无 JS 兜底

    return { stop: stop };
  }

  function init() {
    var roots = document.querySelectorAll('.widget-wrapper.tagcloud .widget-body');
    if (!roots.length) return;
    instances.forEach(function (it) { it.stop(); });
    instances = [];
    roots.forEach(function (root) {
      var it = create(root);
      if (it) instances.push(it);
    });
  }

  document.addEventListener('DOMContentLoaded', init);
  document.addEventListener('pjax:complete', init);
})();
