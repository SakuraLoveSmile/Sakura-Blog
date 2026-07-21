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

    star.addEventListener('mouseenter', () => {
      stars.forEach(s => {
        s.classList.remove('preview');
        const v = parseInt(s.dataset.value);
        s.classList.toggle('hover', v <= value);
      });
    });

    star.addEventListener('mouseleave', () => {
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
    const res = await fetch(`${api}?id=${encodeURIComponent(id)}`);
    const data = await res.json();
    const avg = Number(data.avg || 0);
    const totalVotes = Number(data.count || 0);

    // 设置平均分
    let avgEl = el.querySelector('.avg');
    if (!avgEl) {
      avgEl = document.createElement('span');
      avgEl.className = 'avg';
      el.appendChild(avgEl);
    }
    avgEl.textContent = `(${avg.toFixed(1)})`;

    // 设置评分人数
    let countEl = el.querySelector('.count');
    if (!countEl) {
      countEl = document.createElement('span');
      countEl.className = 'count';
      el.appendChild(countEl);
    }
    countEl.textContent = `${totalVotes}`;

    updatePreview(el, avg);
  } catch (e) {
    console.warn(`[rating] 加载失败: id=${id}`, e);
  }
}

async function submitRating(el, value) {
  const id = el.dataset.id;
  const api = el.dataset.api;
  if (!id || !api || hasRated(id)) return;

  storeRating(id, value);
  el.classList.add('rated');

  try {
    await fetch(api, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id, score: parseInt(value) })
    });
    loadRating(el);
  } catch (e) {
    console.warn(`[rating] 提交失败: id=${id}`, e);
  }
}

function initRatings() {
  document.querySelectorAll('.ds-rating').forEach(el => {
    const { id, api } = el.dataset;
    if (!id || !api) return;

    loadRating(el);
    setupHoverEffect(el);

    if (hasRated(id)) {
      el.classList.add('rated');
    }

    el.querySelectorAll('.star').forEach(star => {
      const value = star.dataset.value;
      star.addEventListener('click', () => {
        if (!hasRated(id)) submitRating(el, value);
      });
    });
  });
}

if (document.readyState === 'loading') {
  window.addEventListener('DOMContentLoaded', initRatings);
} else {
  initRatings();
}
// PJAX 换页后重新扫描初始化（base.html 的 services scan 只加载本脚本一次）
document.addEventListener('pjax:complete', initRatings);