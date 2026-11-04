document.currentScript.stellarMount = function (root, context) {
  const utils = context.serviceUtils;
  const listen = (node, type, handler) => node?.addEventListener(type, handler, { signal: context.signal });
  const fetch = (url, options = {}) => window.fetch(url, { ...options, ...(options.method === 'POST' ? {} : { signal: context.signal }) });
function getRatingKey(id) {
  return `rating-${id}`;
}

function hasRated(id) {
  return !!localStorage.getItem(getRatingKey(id));
}

function getRatedValue(id) {
  return parseInt(localStorage.getItem(getRatingKey(id)) || '0');
}

function storeRating(id, value) {
  localStorage.setItem(getRatingKey(id), value);
}

function removeRating(id) {
  localStorage.removeItem(getRatingKey(id));
}

function clearHover(el) {
  el.querySelectorAll('.star').forEach(s => s.classList.remove('hover'));
}

function updatePreview(el, avg) {
  const rounded = Math.floor(avg);
  el.querySelectorAll('.star').forEach(s => {
    const v = parseInt(s.dataset.value);
    s.classList.toggle('preview', v <= rounded);
  });
}

function setupHoverEffect(el) {
  const stars = el.querySelectorAll('.star');
  if (!stars.length) return;

  stars.forEach(star => {
    const value = parseInt(star.dataset.value);

    listen(star, 'mouseenter', () => {
      stars.forEach(s => {
        s.classList.remove('preview');
        const v = parseInt(s.dataset.value);
        s.classList.toggle('hover', v <= value);
      });
    });

    listen(star, 'mouseleave', () => {
      clearHover(el);
      // 恢复平均分预览
      const avg = parseFloat(el.querySelector('.avg')?.textContent.replace(/[()]/g, '') || '0');
      updatePreview(el, avg);
    });
  });
}

async function loadRating(el) {
  const id = el.dataset.id;
  const api = el.dataset.api;
  if (!id || !api) return;

  try {
    // 原生评分接口：GET /api/rating?id=xxx → {avg, count}
    const res = await fetch(`${api}?id=${encodeURIComponent(id)}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    context.signal.throwIfAborted();
    const avg = (Number(data.avg) || 0).toFixed(1);
    const totalVotes = Number(data.count) || 0;

    // 设置平均分
    let avgEl = el.querySelector('.avg');
    if (!avgEl) {
      avgEl = document.createElement('span');
      avgEl.className = 'avg';
      el.appendChild(avgEl);
    }
    avgEl.textContent = `(${avg})`;

    // 设置评分人数
    let countEl = el.querySelector('.count');
    if (!countEl) {
      countEl = document.createElement('span');
      countEl.className = 'count';
      el.appendChild(countEl);
    }
    countEl.textContent = `${totalVotes}`;

    updatePreview(el, avg);
  } catch (error) {
    void error;
  }
}

async function submitRating(el, value) {
  const id = el.dataset.id;
  const api = el.dataset.api;
  if (!id || !api || hasRated(id)) return;

  storeRating(id, value);
  el.classList.add('rated');

  try {
    // 原生评分接口：POST /api/rating，JSON {id, score}
    const res = await fetch(api, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id, score: Number(value) })
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    if (!context.signal.aborted) loadRating(el);
  } catch (error) {
    void error;
    removeRating(id);
    el.classList.remove('rated');
  }
}

function initRatings() {
  root.querySelectorAll('.ds-rating').forEach(el => {
    const { id, api } = el.dataset;
    if (!id || !api) return;

    loadRating(el);
    setupHoverEffect(el);

    if (hasRated(id)) {
      el.classList.add('rated');
    }

    el.querySelectorAll('.star').forEach(star => {
      const value = star.dataset.value;
      listen(star, 'click', () => {
        if (!hasRated(id)) submitRating(el, value);
      });
    });
  });
}

initRatings();

};
