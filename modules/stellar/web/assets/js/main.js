// 平滑滚动动画（1.41）：短距离 300ms、长距离最多 600ms，easeOutCubic；
// 显式 instant，避免与全局 scroll-behavior: smooth 叠加导致滚动变慢。
let smoothScrollRAF = 0;
function cancelSmoothScroll() {
  if (smoothScrollRAF) {
    cancelAnimationFrame(smoothScrollRAF);
    smoothScrollRAF = 0;
  }
}
function smoothScrollTo(targetY) {
  cancelSmoothScroll();
  targetY = Math.max(0, targetY);
  const startY = window.scrollY;
  const diff = targetY - startY;
  if (Math.abs(diff) < 2) {
    return;
  }
  const duration = Math.min(600, Math.max(300, Math.abs(diff) * 0.15));
  const startTime = performance.now();
  function step(now) {
    const t = Math.min(1, (now - startTime) / duration);
    const eased = 1 - Math.pow(1 - t, 3); // easeOutCubic
    window.scrollTo({ top: startY + diff * eased, behavior: 'instant' });
    if (t < 1) {
      smoothScrollRAF = requestAnimationFrame(step);
    } else {
      smoothScrollRAF = 0;
    }
  }
  smoothScrollRAF = requestAnimationFrame(step);
}

// 通用页内锚点平滑滚动（1.41）：标题锚点、TOC 链接、navbar 页内导航、脚注回链等。
// 已被其他处理器拦截的点击通过 defaultPrevented 跳过，避免重复滚动；
// #start 锚点贴顶滚动，其余锚点与 TOC 点击保持一致（32px offset）。
let anchorClickBound = false;
function bindAnchorClick() {
  if (anchorClickBound) return;
  anchorClickBound = true;
  document.addEventListener('click', function (e) {
    const link = e.target.closest && e.target.closest('a[href^="#"]');
    if (!link || e.defaultPrevented) {
      return;
    }
    const href = link.getAttribute('href');
    if (!href || href.indexOf('#') !== 0) {
      return;
    }
    let id = href.slice(1);
    try {
      id = decodeURIComponent(id);
    } catch (err) {
      // 片段含非法编码时按原样查找
    }
    const target = id && document.getElementById(id);
    if (!target) {
      return;
    }
    e.preventDefault();
    const offset = id === 'start' ? 0 : 32;
    const targetY = target.getBoundingClientRect().top + window.scrollY - offset;
    smoothScrollTo(targetY);
    if (window.history && window.history.pushState) {
      window.history.pushState(null, '', href);
    }
  });
}

// utils
const util = {

  // https://github.com/jerryc127/hexo-theme-butterfly
  diffDate: (d, more = false) => {
    const dateNow = new Date()
    const datePost = new Date(d)
    const dateDiff = dateNow.getTime() - datePost.getTime()
    const minute = 1000 * 60
    const hour = minute * 60
    const day = hour * 24

    let result
    if (more) {
      const dayCount = dateDiff / day
      const hourCount = dateDiff / hour
      const minuteCount = dateDiff / minute

      if (dayCount > 14) {
        result = null
      } else if (dayCount >= 1) {
        result = parseInt(dayCount) + ' ' + ctx.date_suffix.day
      } else if (hourCount >= 1) {
        result = parseInt(hourCount) + ' ' + ctx.date_suffix.hour
      } else if (minuteCount >= 1) {
        result = parseInt(minuteCount) + ' ' + ctx.date_suffix.min
      } else {
        result = ctx.date_suffix.just
      }
    } else {
      result = parseInt(dateDiff / day)
    }
    return result
  },

  copy: (id, msg) => {
    const el = document.getElementById(id);
    if (el) {
      el.select();
      document.execCommand("Copy");
      if (msg && msg.length > 0) {
        hud.toast(msg, 2500);
      }
    }
  },

  toggle: (id) => {
    const el = document.getElementById(id);
    if (el) {
      el.classList.toggle("display");
    }
  },

  scrollTop: () => {
    window.scrollTo({ top: 0, behavior: "smooth" });
  },

  scrollComment: () => {
    document.getElementById('comments').scrollIntoView({ behavior: "smooth" });
  },

  viewportLazyload: (target, func, enabled = true) => {
    if (!enabled || !("IntersectionObserver" in window)) {
      func();
      return;
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries[0].intersectionRatio > 0) {
        func();
        observer.disconnect();
      }
    });
    observer.observe(target);
  }
}

const hud = {
  toast: (msg, duration) => {
    const d = Number(isNaN(duration) ? 2000 : duration);
    var el = document.createElement('div');
    el.classList.add('toast');
    el.classList.add('show');
    el.innerHTML = msg;
    document.body.appendChild(el);

    setTimeout(function () { document.body.removeChild(el) }, d);

  },

}

// defines

const l_body = document.querySelector('.l_body');

// sidebar：侧栏开关（对齐 Stellar scripts/sidebar.ejs，四个方法全量实现）。
// rightbar 用于移动端右栏（TOC）开关；toggleTOC 切换 #data-toc 的 collapse class（手风琴）。
const sidebar = {
  leftbar: () => { if (l_body) { l_body.toggleAttribute('leftbar'); l_body.removeAttribute('rightbar'); } },
  rightbar: () => { if (l_body) { l_body.toggleAttribute('rightbar'); l_body.removeAttribute('leftbar'); } },
  dismiss: () => { if (l_body) { l_body.removeAttribute('leftbar'); l_body.removeAttribute('rightbar'); } },
  toggleTOC: () => { const el = document.querySelector('#data-toc'); if (el) el.classList.toggle('collapse'); },
}

// TOC scroll-spy 共享状态：segs/root 每次 init.toc 刷新，scroll 监听器只注册一次，
// PJAX 导航后重复 init 不叠加绑定（Stellar 原版依赖 jQuery，本 demo 未加载，原生重写）。
const tocState = { segs: [], root: null, bound: false };

function activeTOC() {
  const root = tocState.root;
  if (!root) return;
  const scrollOffset = 32;
  const scrollTop = window.scrollY || document.documentElement.scrollTop;
  let topSeg = null;
  let topSegTop = -Infinity;
  for (const seg of tocState.segs) {
    const docTop = seg.getBoundingClientRect().top + scrollTop;
    if (docTop > scrollTop + scrollOffset) continue;
    if (docTop >= topSegTop) { topSeg = seg; topSegTop = docTop; }
  }
  const links = root.querySelectorAll('a.toc-link');
  links.forEach(l => l.classList.remove('active', 'is-active', 'ui-collection__item'));
  if (!topSeg) return;
  // 不用属性选择器拼 id（避免特殊字符转义问题），直接比对 getAttribute
  for (const l of links) {
    if (l.getAttribute('href') === '#' + topSeg.id) {
      l.classList.add('active', 'is-active', 'ui-collection__item');
      break;
    }
  }
}

function scrollTOC() {
  const e0 = document.querySelector('#data-toc .toc');
  const e1 = document.querySelector('#data-toc a.toc-link.active');
  if (e0 == null || e1 == null) {
    return;
  }
  const offsetBottom = e1.getBoundingClientRect().bottom - e0.getBoundingClientRect().bottom + 100;
  const offsetTop = e1.getBoundingClientRect().top - e0.getBoundingClientRect().top - 64;
  if (offsetTop < 0) {
    e0.scrollBy({ top: offsetTop, behavior: "smooth" });
  } else if (offsetBottom > 0) {
    e0.scrollBy({ top: offsetBottom, behavior: "smooth" });
  }
}

const init = {
  toc: () => {
    // 每次 init 重新收集 h1-h6 标题与 TOC 根节点（PJAX 后 DOM 已替换）
    tocState.segs = Array.from(document.querySelectorAll('article.md-text h1[id], article.md-text h2[id], article.md-text h3[id], article.md-text h4[id], article.md-text h5[id], article.md-text h6[id]'));
    tocState.root = document.getElementById('data-toc');
    if (!tocState.bound) {
      tocState.bound = true;
      let timeout = null;
      window.addEventListener('scroll', function () {
        activeTOC();
        if (timeout !== null) clearTimeout(timeout);
        timeout = setTimeout(scrollTOC, 50);
      });
    }
    activeTOC(); // 初始高亮
  },
  navbarPin: () => {
    // 1.41：列表页 navbar top 背景条未吸顶为卡片样式、吸顶恢复玻璃（.navbar-blur.pinned）。
    // 吸顶判定直接测 navbar 实际视口位置而非 scrollY 推算：移动端浏览器顶栏伸缩会改变
    // scrollY（展开顶栏时 scrollY 减小），即使 navbar 仍吸顶也可能跌破阈值导致玻璃误消失。
    // 无轮播页（wiki）顶部即吸顶，需滚动达阈值才切玻璃。
    const navbars = document.querySelectorAll('.navbar.top');
    if (navbars.length === 0) {
      return;
    }
    const TOLERANCE = 2; // 视口顶部允许偏差（px），吸收亚像素/取整误差
    const SCROLL_THRESHOLD = 2; // 滚动阈值（px），未达阈值保持卡片样式
    let states = [];
    function update() {
      const scrolled = window.scrollY >= SCROLL_THRESHOLD;
      states.forEach((state) => {
        const top = state.navbar.getBoundingClientRect().top;
        state.bar.classList.toggle('pinned', scrolled && top <= state.stickyTop + TOLERANCE);
      });
    }
    function measure() {
      states = [];
      navbars.forEach((navbar) => {
        const bar = navbar.querySelector('.navbar-blur');
        if (bar == null) {
          return;
        }
        // getComputedStyle().top 自动兼容桌面 var(--gap-page) 与移动端 8pt
        const stickyTop = parseFloat(getComputedStyle(navbar).top) || 16;
        states.push({ navbar: navbar, bar: bar, stickyTop: stickyTop });
      });
      update();
    }
    let ticking = false;
    window.addEventListener('scroll', () => {
      if (ticking) {
        return;
      }
      ticking = true;
      window.requestAnimationFrame(() => {
        update();
        ticking = false;
      });
    }, { passive: true });
    // 顶栏伸缩不一定触发 scroll，兜底监听 visualViewport 尺寸变化
    if (window.visualViewport) {
      window.visualViewport.addEventListener('resize', update);
    }
    window.addEventListener('resize', measure);
    window.addEventListener('pageshow', measure);
    measure();
  },
  sidebar: () => {
    // 移动端点 TOC 链接收起左侧栏。事件委托绑一次即可（TOC 节点 PJAX 后会被替换，委托到 document 不受影响）
    if (tocState.sidebarBound) return;
    tocState.sidebarBound = true;
    document.addEventListener('click', (e) => {
      const link = e.target.closest && e.target.closest('#data-toc a.toc-link');
      if (link) window.sidebar.dismiss();
    });
  },
  wikiEmptyCard: () => {
    // 文库空卡片（无本地文档且无外链，带 data-wiki-empty）：点击 toast 提示文案。
    // document 级事件委托只绑一次，PJAX 换页后 DOM 替换不受影响。
    if (init.wikiEmptyBound) return;
    init.wikiEmptyBound = true;
    document.addEventListener('click', (e) => {
      const card = e.target.closest && e.target.closest('a[data-wiki-empty]');
      if (!card) return;
      e.preventDefault();
      hud.toast(card.getAttribute('data-wiki-empty'), 2500);
    });
  },
  wikiHeroCopy: () => {
    if (init.wikiHeroCopyBound) return;
    init.wikiHeroCopyBound = true;
    document.addEventListener('click', function (e) {
      const button = e.target.closest && e.target.closest('.wiki-copy[data-copy-text]');
      if (!button) return;
      const text = button.getAttribute('data-copy-text') || '';
      const done = () => hud.toast('已复制', 1800);
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done).catch(function () {});
        return;
      }
      const area = document.createElement('textarea');
      area.value = text;
      area.style.position = 'fixed';
      area.style.opacity = '0';
      document.body.appendChild(area);
      area.select();
      try { document.execCommand('copy'); done(); } catch (err) {}
      document.body.removeChild(area);
    });
  },
  wikiCover: () => {
    // 上游 Wiki Hero：终端预览与 Galaxy WebGL 背景按页面需要初始化。
    document.querySelectorAll('.wiki-cover-terminal').forEach(function (terminal) {
      const code = terminal.querySelector('pre code');
      const tabs = terminal.querySelectorAll('[role="tab"]');
      function renderCodes(value) {
        if (!code) return;
        const lines = value.split(/\r?\n/).filter(function (line) {
          return line.trim().length > 0;
        });
        terminal.dataset.codes = lines.join('\n');
        code.innerHTML = '';
        lines.forEach(function (line) {
          const row = document.createElement('span');
          row.textContent = line;
          code.appendChild(row);
        });
      }
      if (tabs.length > 0) {
        renderCodes(tabs[0].getAttribute('data-codes') || '');
      }
      tabs.forEach(function (tab) {
        tab.addEventListener('click', function () {
          tabs.forEach(function (item) {
            const active = item === tab;
            item.classList.toggle('active', active);
            item.setAttribute('aria-selected', active ? 'true' : 'false');
          });
          renderCodes(tab.getAttribute('data-codes') || '');
        });
      });
      const copy = terminal.querySelector('.wiki-cover-copy');
      if (copy) {
        copy.addEventListener('click', function () {
          const value = terminal.dataset.codes || '';
          if (!value || !navigator.clipboard) return;
          navigator.clipboard.writeText(value).then(function () {
            hud.toast(copy.getAttribute('data-copy-message') || 'Copied', 2500);
          }).catch(function () {});
        });
      }
    });

    const galaxyCanvases = document.querySelectorAll('.wiki-cover-background.galaxy canvas');
    const reduceMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduceMotion || galaxyCanvases.length === 0 || !window.utils || !utils.js) return;
    utils.js('/assets/js/plugins/galaxy.js', { defer: true }).then(function () {
      if (stellar.galaxy && typeof stellar.galaxy.mountAll === 'function') {
        stellar.galaxy.mountAll(galaxyCanvases);
      }
    }).catch(function () {});
  },
  relativeDate: (selector) => {
    selector.forEach(item => {
      const $this = item
      const timeVal = $this.getAttribute('datetime')
      let relativeValue = util.diffDate(timeVal, true)
      if (relativeValue) {
        $this.innerText = relativeValue
      }
    })
  },
  /**
   * Tabs tag listener (without twitter bootstrap).
   */
  registerTabsTag: function () {
    // Binding `nav-tabs` & `tab-content` by real time permalink changing.
    document.querySelectorAll('.tabs .nav-tabs .tab').forEach(element => {
      element.addEventListener('click', event => {
        event.preventDefault();
        // Prevent selected tab to select again.
        if (element.classList.contains('active')) return;
        // Add & Remove active class on `nav-tabs` & `tab-content`.
        [...element.parentNode.children].forEach(target => {
          target.classList.toggle('active', target === element);
        });
        // https://stackoverflow.com/questions/20306204/using-queryselector-with-ids-that-are-numbers
        const tActive = document.getElementById(element.querySelector('a').getAttribute('href').replace('#', ''));
        [...tActive.parentNode.children].forEach(target => {
          target.classList.toggle('active', target === tActive);
        });
        // Trigger event
        tActive.dispatchEvent(new Event('tabs:click', {
          bubbles: true
        }));
      });
    });

    window.dispatchEvent(new Event('tabs:register'));
  },

  canonicalCheck: () => {
    const canonical = window.canonical;
    // 本 demo 未注入 canonical 配置，直接跳过（Stellar 原版由 defines.ejs 注入）
    if (!canonical) return;
    function originStatusCheck() {
      return new Promise((resolve) => {
        if (window.canonical.originalHost === window.location.hostname) {
          resolve(true);
          return;
        }
        const scriptUrl = `https://${window.canonical.originalHost}${window.canonical.param.checklink}`;
        const script = document.createElement('script');
        script.src = scriptUrl;
        script.type = 'text/javascript';
        script.onload = function () { resolve(true); };
        script.onerror = function () { resolve(false); };
        document.head.appendChild(script);
      });
    }
    async function showTip(isOfficial = false) {
      const meta = document.createElement('meta');
      meta.name = 'robots';
      meta.content = 'noindex, nofollow';
      document.head.appendChild(meta);
      const notice = document.createElement('div');
      const originalURL = `https://${canonical.originalHost}`;
      const currentURL = canonical.param.permalink.startsWith("http") ? canonical.param.permalink : originalURL;
      if (isOfficial) {
        const closeEnable = window.localStorage.getItem('Stellar.canonical.closeEnable') === 'true'
        const closedToday = window.localStorage.getItem('Stellar.canonical.closeTime') === new Date().toDateString()
        if ((closeEnable && closedToday) || !(await originStatusCheck())) return;
        notice.className = 'canonical-tip official';
        notice.innerHTML = `
          <a href="${currentURL}" target="_self" rel="noopener noreferrer">
          本站为官方备用站，仅供应急。点击移步主站<br>${originalURL}
          </a>
          ${canonical.closeEnable ? '<button id="canonical-close">' + canonical.closeText || '关闭提示' + '</button>' : ''}
        `;
      } else {
        notice.className = 'canonical-tip unofficial';
        notice.innerHTML = `
        <a href="${currentURL}" target="_self" rel="noopener noreferrer">
        <div class="headline icon">☠️</div>
        本站为非法克隆站，请前往官方源站访问。<br>
        源站：${originalURL}
        </a>
        `;
      }
      document.body.appendChild(notice);
      const closeBtn = notice.querySelector('#canonical-close');
      if (closeBtn) {
        closeBtn.addEventListener('click', function () {
          window.localStorage.setItem('Stellar.canonical.closeEnable', "true")
          window.localStorage.setItem('Stellar.canonical.closeTime', new Date().toDateString())
          notice.style.display = 'none';
        });
      }
    }
    if (!canonical.originalHost) return;
    const currentURL = new URL(window.location.href);
    const currentHost = currentURL.hostname.replace(/^www\./, '');
    if (currentHost == 'localhost') return;
    const encodedCurrentHost = window.btoa(currentHost);
    const isCurrentHostValid = canonical.encoded === encodedCurrentHost;
    const canonicalTag = document.querySelector('link[rel="canonical"]');
    if (!canonicalTag) {
      if (isCurrentHostValid) {
        return;
      }
      if (canonical.officialHosts?.includes(currentHost)) {
        showTip(true);
        return;
      }
      showTip(false);
      return;
    }
    const canonicalURL = new URL(canonicalTag.href);
    const canonicalHost = canonicalURL.hostname.replace(/^www\./, '');
    const encodedCanonicalHost = window.btoa(canonicalHost);
    const isCanonicalHostValid = canonical.encoded === encodedCanonicalHost;
    if (isCanonicalHostValid && isCurrentHostValid) {
      return;
    }
    showTip(canonical.officialHosts?.includes(currentHost));
  }

}


// Stellar namespace
window.stellar = window.stellar || {};

/**
 * Initialize page components
 * Called on initial load and after PJAX navigation
 */
stellar.initPage = function () {
  init.toc();
  init.sidebar();
  init.navbarPin();
  bindAnchorClick();
  init.wikiEmptyCard();
  init.wikiHeroCopy();
  init.wikiCover();
  init.relativeDate(document.querySelectorAll('#post-meta time'));
  init.registerTabsTag();

  // wrapLazyloadImages：将 goldmark 输出的 img 转为 lazy-load 形式
  if (typeof window.wrapLazyloadImages === 'function') {
    document.querySelectorAll('.md-text.content').forEach(function (el) {
      window.wrapLazyloadImages(el);
    });
  }
  
  // Reinitialize comments after PJAX navigation
  if (stellar.initComments) {
    for (const commentSystem in stellar.initComments) {
      if (typeof stellar.initComments[commentSystem] === 'function') {
        stellar.initComments[commentSystem]();
      }
    }
  }
};

// Initial page load
stellar.initPage();
init.canonicalCheck();
