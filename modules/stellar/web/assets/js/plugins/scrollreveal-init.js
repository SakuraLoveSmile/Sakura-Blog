/**
 * scrollreveal-init.js - Scroll entrance animations (Spec 36)
 * Applies to Stellar's .post-card and .slide-up reveal targets.
 * Automatically disabled when the user prefers reduced motion.
 */
(function () {
  'use strict';

  var sr = null;

  function shouldAnimate() {
    return !!document.querySelector('.post-card, .slide-up');
  }

  function revealFallback() {
    document.documentElement.classList.add('sr-fallback');
  }

  function init() {
    if (!shouldAnimate()) return;
    if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      revealFallback();
      return;
    }
    if (typeof ScrollReveal === 'undefined') {
      revealFallback();
      return;
    }

    if (sr) {
      sr.clean();
    }
    sr = ScrollReveal();

    var cfg = (typeof ctx !== 'undefined' && ctx.plugins && ctx.plugins.scrollreveal) || {};
    var revealCfg = {
      distance: cfg.distance || '60px',
      duration: cfg.duration || 600,
      interval: cfg.interval || 100,
      origin: 'bottom',
      easing: 'cubic-bezier(0.6, 0.2, 0.1, 1)',
      reset: false
    };
    // scale（1.44 scrollreveal.scale，0.1~1）：仅显式配置（>0）时传递，缺省走 ScrollReveal 内置默认 1
    if (cfg.scale > 0 && cfg.scale <= 1) {
      revealCfg.scale = cfg.scale;
    }
    sr.reveal('.post-card, .slide-up', revealCfg);
  }

  document.addEventListener('DOMContentLoaded', init);
})();
