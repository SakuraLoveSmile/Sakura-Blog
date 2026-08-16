/**
 * heti-init.js - Chinese typography enhancement (Spec 36)
 * Applies only to the .md-text article container.
 * The vendored heti-addon.min.js exposes a global Heti class (UMD):
 *   new Heti('.heti').autoSpacing()  - auto queue on DOMContentLoaded
 *   heti.spacingElements(elements)   - process a list of elements directly
 * We use spacingElements() directly since init runs on DOMContentLoaded
 * where autoSpacing()'s internal readyState check would
 * miss the already-fired DOMContentLoaded event.
 * CSS (heti.min.css) is injected conditionally by base.html.
 */
(function () {
  'use strict';

  function init() {
    var article = document.querySelector('.md-text');
    if (!article) return;
    if (article.classList.contains('heti')) return; // already initialized
    if (typeof Heti === 'undefined') return; // library not loaded yet

    article.classList.add('heti');

    new Heti().spacingElements(document.querySelectorAll('.heti'));
  }

  document.addEventListener('DOMContentLoaded', init);
})();
