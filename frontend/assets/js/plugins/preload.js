/**
 * preload.js - flying-pages initialization (Spec 36)
 * Hover-intent link prefetching, complementary to PJAX navigation.
 * flying-pages reads its config from window.FPConfig; this script only
 * provides a fallback if the config was not already injected by base.html.
 */
(function () {
  'use strict';

  function init() {
    if (!window.FPConfig) {
      window.FPConfig = {
        delay: 0,
        ignoreKeywords: ['/console', '/api'],
        maxRPS: 5,
        hoverDelay: 25
      };
    }
  }

  init();
})();
