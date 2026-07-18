/* reading.js: 文章页阅读进度条 + 正文图片灯箱。
   - 进度条仅 article.md-text.content 存在时启用
   - 灯箱：点击正文 img，ESC / 遮罩关闭
   - PJAX：scroll/keydown 只绑一次（window.__readingBound）；pjax:complete 重置进度并重绑图片 */
(function () {
  'use strict';

  var progressEl = null;
  var lightboxEl = null;
  var lightboxImg = null;
  var lightboxBackdrop = null;
  var articleEl = null;
  var rafId = 0;
  var lastFocus = null;

  function getArticle() {
    return document.querySelector('article.md-text.content');
  }

  function isContentImage(img) {
    if (!img || img.tagName !== 'IMG') return false;
    if (img.closest('pre, .highlight, .gist, a[data-fancybox]')) return false;
    return true;
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

  function openLightbox(img) {
    if (!lightboxEl || !lightboxImg) return;

    lastFocus = document.activeElement;
    lightboxImg.src = img.currentSrc || img.src;
    lightboxImg.alt = img.alt || '';
    lightboxEl.hidden = false;
    document.body.classList.add('reading-lightbox-open');

    if (lightboxBackdrop) {
      lightboxBackdrop.focus();
    }
  }

  function closeLightbox() {
    if (!lightboxEl || lightboxEl.hidden) return;

    lightboxEl.hidden = true;
    document.body.classList.remove('reading-lightbox-open');
    if (lightboxImg) {
      lightboxImg.removeAttribute('src');
      lightboxImg.alt = '';
    }

    if (lastFocus && typeof lastFocus.focus === 'function') {
      lastFocus.focus();
    }
    lastFocus = null;
  }

  function onKeydown(e) {
    if (e.key === 'Escape') closeLightbox();
  }

  function onImageClick(e) {
    var article = getArticle();
    if (!article || !article.contains(e.target)) return;
    if (!isContentImage(e.target)) return;

    e.preventDefault();
    openLightbox(e.target);
  }

  function bindGlobalOnce() {
    if (window.__readingBound) return;
    window.__readingBound = true;

    window.addEventListener('scroll', onScroll, { passive: true });
    document.addEventListener('keydown', onKeydown);
    document.addEventListener('click', onImageClick);

    if (lightboxBackdrop) {
      lightboxBackdrop.addEventListener('click', closeLightbox);
    }
    if (lightboxEl) {
      lightboxEl.addEventListener('click', function (e) {
        if (e.target === lightboxEl) closeLightbox();
      });
    }
  }

  function init() {
    progressEl = document.getElementById('reading-progress');
    lightboxEl = document.getElementById('reading-lightbox');
    lightboxImg = lightboxEl ? lightboxEl.querySelector('.reading-lightbox__img') : null;
    lightboxBackdrop = lightboxEl ? lightboxEl.querySelector('.reading-lightbox__backdrop') : null;
    articleEl = getArticle();

    bindGlobalOnce();
    closeLightbox();
    updateProgress();
  }

  document.addEventListener('DOMContentLoaded', init);
  document.addEventListener('pjax:complete', init);
})();
