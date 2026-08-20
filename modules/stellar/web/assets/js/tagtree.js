// Stellar 1.43.0 notebook tagtree interaction.
(function () {
  "use strict";
  var conf = window.__STELLAR_TAGTREE__ || {};
  document.querySelectorAll('.tag-subtree.is-parent > .tag-tree__link > .tag-switcher-wrapper').forEach(function (switcher) {
    switcher.addEventListener('click', function (event) {
      var parent = event.target.closest('.tag-subtree.is-parent');
      if (parent) parent.classList.toggle('is-expanded');
      event.preventDefault();
      event.stopPropagation();
    });
  });
  var activeTag = new URLSearchParams(window.location.search).get('tag');
  if (!activeTag) return;
  // Compare attribute values directly so tag names cannot become CSS syntax.
  var node = Array.prototype.find.call(
    document.querySelectorAll('.tag-subtree[data-tag]'),
    function (el) { return el.getAttribute('data-tag') === activeTag; }
  );
  if (!node) return;
  var link = node.querySelector(':scope > .tag-tree__link');
  if (link) link.classList.add('is-active');
  var current = node;
  while (current) {
    current.classList.add('is-expanded');
    if (!conf.expand_active) current = current.parentElement && current.parentElement.closest('.tag-subtree.is-parent');
    else current = current.parentElement && current.parentElement.closest('.tag-subtree.is-parent');
  }
})();
