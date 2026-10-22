// tagcloud.js: 标签云 widget 交互（3D 球体版）。
// 默认静态模式：SSR 静态云直接可见（JS 加载后保持静态）；
// 动态模式：3D 球体标签云（Fibonacci 球面分布，透视投影近大远小/近清远淡，
// 自动旋转 + 拖拽惯性），用户点击切换按钮后激活。
// 全站 defer 加载：遍历 .widget-wrapper.tagcloud 存在性初始化（支持多实例）+
// pjax:complete 重初始化，元素移出 DOM 后动画循环自清理；数据源为 SSR 静态云 DOM，不发请求。
(function () {
  function mount(rootRegion, context) {
  var scope = window.blogRuntime.createScope(rootRegion, context);
  var document = scope.document;
  var requestAnimationFrame = scope.requestAnimationFrame;
  var setTimeout = scope.setTimeout, clearTimeout = scope.clearTimeout;
  'use strict';

  var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  var instances = [];

  // 三维旋转（先绕 Y 轴再绕 X 轴），返回旋转后的坐标
  function rotate3d(x, y, z, ry, rx) {
    var cy = Math.cos(ry), sy = Math.sin(ry);
    var x1 = x * cy + z * sy;
    var z1 = -x * sy + z * cy;
    var cx = Math.cos(rx), sx = Math.sin(rx);
    return { x: x1, y: y * cx - z1 * sx, z: y * sx + z1 * cx };
  }

  function create(root) {
    var cloud = root.querySelector('.tag-cloud');
    var staticCloud = root.querySelector('.tag-cloud-static');
    var modeSwitch = root.querySelector('.tag-mode-switch');
    if (!cloud || !staticCloud || !modeSwitch) return null;

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

    // 同一 root 重复 create（如 pjax 重初始化）时清理旧球体标签
    cloud.querySelectorAll('a.tag').forEach(function (a) { a.remove(); });

    var N = tags.length;
    // 边界处理：N=1 居中放置，N=2 在赤道平面水平对称
    // Fibonacci 球面分布：单位球面上均匀撒点（Y 轴为竖直轴，绕 Y 旋转呈地球自转感）
    var golden = Math.PI * (3 - Math.sqrt(5));
    if (N === 1) {
      tags[0].px = 0; tags[0].py = 0; tags[0].pz = 1;
    } else if (N === 2) {
      tags[0].px = 1; tags[0].py = 0; tags[0].pz = 0;
      tags[1].px = -1; tags[1].py = 0; tags[1].pz = 0;
    } else {
      tags.forEach(function (t, i) {
        var yy = 1 - (i / (N - 1)) * 2;
        var r = Math.sqrt(1 - yy * yy);
        t.px = Math.cos(golden * i) * r;
        t.py = yy;
        t.pz = Math.sin(golden * i) * r;
      });
    }
    tags.forEach(function (t) {
      t.el = document.createElement('a');
      t.el.className = 'tag tag-sphere tag-' + t.level;
      t.el.textContent = t.name;
      t.el.href = t.href;
      t.el.title = t.title;
      cloud.appendChild(t.el);
    });

    var cloudWidth = 0, cloudHeight = 0, radius = 60;
    var rotY = 0, rotX = -0.35;   // 初始绕 X 轴倾角，呈现球体立体感
    var rotSpeed = 0.0045;        // 自动旋转角速度（rad/帧，约 23s 一圈）
    var velY = 0, velX = 0;       // 拖拽释放后的惯性角速度
    var isDragging = false, autoRotate = true;
    var lastX = 0, lastY = 0, startX = 0, startY = 0;
    var hasDragged = false, suppressClick = false, suppressTimer = 0;
    var rafId = 0, stopped = false, mode = 'static', driftTime = 0;

    // 透视投影：深度 → 缩放与透明度（近大远小、近清远淡）
    function project() {
      for (var i = 0; i < N; i++) {
        var t = tags[i];
        var p = rotate3d(t.px, t.py, t.pz, rotY, rotX);
        var depth = (p.z + 1) / 2;              // 0（背面）~ 1（正面）
        var persp = 0.8 + depth * 0.4;          // 缩放 0.8x ~ 1.2x
        var el = t.el;
        el.style.left = (cloudWidth / 2 + p.x * radius * persp) + 'px';
        el.style.top = (cloudHeight / 2 + p.y * radius * persp) + 'px';
        el.style.opacity = (0.3 + 0.7 * depth).toFixed(3);
        el.style.zIndex = Math.round(depth * 100);
        el.style.transform = 'translate(-50%, -50%) scale(' + persp.toFixed(3) + ')';
      }
    }

    function animate() {
      if (stopped || !cloud.isConnected) { stop(); return; }
      if (!isDragging && autoRotate && mode === 'dynamic') {
        driftTime += 0.01;
        rotY += rotSpeed;
        // 俯仰摆动（缓动趋近目标角，从任意拖拽姿态平滑回到呼吸节奏）
        var targetRX = -0.35 + Math.sin(driftTime * 0.5) * 0.08;
        rotX += (targetRX - rotX) * 0.02;
      }
      // 拖拽惯性（衰减至阈值以下停止）
      if (!isDragging && (Math.abs(velY) > 0.0001 || Math.abs(velX) > 0.0001)) {
        rotY += velY;
        rotX += velX;
        rotX = Math.max(-1.2, Math.min(1.2, rotX));
        velY *= 0.94;
        velX *= 0.94;
      }
      if (mode === 'dynamic') project();
      // 静态模式且无拖拽/惯性时停止 RAF，避免空转耗电；切回动态时由 switchMode 重启
      if (mode === 'dynamic' || isDragging || Math.abs(velY) > 0.0001 || Math.abs(velX) > 0.0001) {
        rafId = requestAnimationFrame(animate);
      } else {
        rafId = 0;
      }
    }

    function onStart(e) {
      if (mode !== 'dynamic') return;
      isDragging = true;
      autoRotate = false;
      hasDragged = false;
      cloud.classList.add('grabbing');
      var p = e.touches ? e.touches[0] : e;
      lastX = startX = p.clientX;
      lastY = startY = p.clientY;
      velY = velX = 0;
    }

    function onMove(e) {
      if (!isDragging) return;
      var p = e.touches ? e.touches[0] : e;
      var dx = p.clientX - lastX;
      var dy = p.clientY - lastY;
      if (Math.abs(p.clientX - startX) > 5 || Math.abs(p.clientY - startY) > 5) hasDragged = true;
      // 触屏与侧栏外页面纵向滚动共存（touch-action: pan-y），仅取水平位移；鼠标拖拽双轴自由
      velY = dx * 0.006;
      velX = e.touches ? 0 : dy * 0.006;
      rotY += velY;
      rotX += velX;
      rotX = Math.max(-1.2, Math.min(1.2, rotX));
      project();
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
      // 停顿后恢复自动旋转（reduced-motion 下永不恢复）
      var delay = hasDragged ? 2500 : 500;
      setTimeout(function () { if (!isDragging && mode === 'dynamic' && !stopped) autoRotate = !reduced; }, delay);
    }

    // 捕获阶段拦截：拖拽后吞掉 click（pjax.js 的 document 冒泡监听在捕获被 stopPropagation 后不会触发）；
    // 正常点击放行并补一个涟漪反馈，跳转交给默认行为 / PJAX。
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

    // 鼠标悬停暂停自动旋转，移出恢复
    function onEnter() { if (!isDragging) autoRotate = false; }
    function onLeave() { if (!isDragging && mode === 'dynamic') autoRotate = !reduced; }

    function onResize() {
      cloudWidth = cloud.offsetWidth;
      cloudHeight = cloud.offsetHeight;
      radius = Math.min(cloudWidth, cloudHeight) * 0.3;
      if (mode === 'dynamic') project();
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
        autoRotate = false;
      } else {
        staticCloud.setAttribute('hidden', '');
        cloud.removeAttribute('hidden');
        cloudWidth = cloud.offsetWidth;
        cloudHeight = cloud.offsetHeight;
        radius = Math.min(cloudWidth, cloudHeight) * 0.3;
        project();
        autoRotate = !reduced;
        if (!rafId && !stopped) rafId = requestAnimationFrame(animate);
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

    scope.listen(cloud, 'mousedown', onStart);
    scope.listen(document, 'mousemove', onMove);
    scope.listen(document, 'mouseup', onEnd);
    scope.listen(cloud, 'touchstart', onStart, { passive: true });
    scope.listen(document, 'touchmove', onMove, { passive: true });
    scope.listen(document, 'touchend', onEnd);
    scope.listen(cloud, 'click', onClickCapture, true);
    scope.listen(cloud, 'mouseenter', onEnter);
    scope.listen(cloud, 'mouseleave', onLeave);
    scope.listen(modeSwitch, 'click', onModeClick);
    scope.listen(window, 'resize', onResize);

    // 默认静态模式：SSR 静态云已可见，球体标签已构建但隐藏。
    // 启动首帧 RAF，animate 在静态无交互时会自行停止；切到动态时由 switchMode 重启。
    root.classList.add('is-static');
    rafId = requestAnimationFrame(animate);

    return { stop: stop };
  }

  function init() {
    var roots = document.querySelectorAll('.widget-wrapper.tagcloud');
    if (!roots.length) return;
    instances.forEach(function (it) { it.stop(); });
    instances = [];
    roots.forEach(function (root) {
      var it = create(root);
      if (it) instances.push(it);
    });
  }

  init();
  scope.onCleanup(function () { instances.forEach(function (it) { it.stop(); }); });
  return scope.cleanup;
  }
  window.blogTagcloud = { mount: mount };
})();
