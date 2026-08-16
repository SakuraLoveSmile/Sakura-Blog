/**
 * scrollreveal-init.js - Scroll entrance animations (Spec 36)
 * Applies only to .post-card containers on index/list pages.
 * Automatically disabled when the user prefers reduced motion.
 */
(function () {
  'use strict';

  var sr = null;

  function shouldAnimate() {
    if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return false;
    return !!document.querySelector('.post-card');
  }

  function init() {
    if (!shouldAnimate()) return;
    if (typeof ScrollReveal === 'undefined') return;

    if (sr) {
      sr.clean();
    }
    sr = ScrollReveal();

    var cfg = (typeof ctx !== 'undefined' && ctx.plugins && ctx.plugins.scrollreveal) || {};
    sr.reveal('.post-card', {
      distance: cfg.distance || '60px',
      duration: cfg.duration || 600,
      interval: cfg.interval || 100,
      origin: 'bottom',
      easing: 'cubic-bezier(0.6, 0.2, 0.1, 1)',
      reset: false
    });
  }

  document.addEventListener('DOMContentLoaded', init);
})();
