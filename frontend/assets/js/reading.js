/* reading.js: 文章页阅读进度条。
   - 进度条仅 article.md-text.content 存在时启用
   - 图片灯箱已由 fancybox 接管（base.html 懒加载绑定），此文件不再处理图片点击
   - PJAX：scroll 只绑一次（window.__readingBound）；pjax:complete 重置进度 */
(function () {
  'use strict';

  var progressEl = null;
  var articleEl = null;
  var rafId = 0;

  function getArticle() {
    return document.querySelector('article.md-text.content');
  }

  function updateProgress() {
    rafId = 0;
    if (!progressEl) return;

    if (!articleEl) {
      progressEl.style.width = '0%';
      progressEl.hidden = true;
      return;
    }

    progressEl.hidden = false;
    var rect = articleEl.getBoundingClientRect();
    var scrollTop = window.pageYOffset || document.documentElement.scrollTop;
    var articleTop = rect.top + scrollTop;
    var articleHeight = articleEl.offsetHeight;
    var viewport = window.innerHeight;
    var scrollable = articleHeight - viewport;

    if (scrollable <= 0) {
      progressEl.style.width = '0%';
      return;
    }

    var scrolled = scrollTop - articleTop;
    var pct = Math.min(100, Math.max(0, (scrolled / scrollable) * 100));
    progressEl.style.width = pct + '%';
  }

  function onScroll() {
    if (!rafId) rafId = requestAnimationFrame(updateProgress);
  }

  function bindGlobalOnce() {
    if (window.__readingBound) return;
    window.__readingBound = true;

    window.addEventListener('scroll', onScroll, { passive: true });
  }

  function init() {
    progressEl = document.getElementById('reading-progress');
    articleEl = getArticle();

    bindGlobalOnce();
    updateProgress();
  }

  document.addEventListener('DOMContentLoaded', init);
  document.addEventListener('pjax:complete', init);
})();
