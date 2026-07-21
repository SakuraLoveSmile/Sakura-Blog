// swiper.js | 对齐原版 layout/_plugins/swiper.ejs
// 页面存在 #swiper-api 时按需加载 swiper CDN 并初始化；PJAX 换页后重新检测。
const SWIPER_CSS = 'https://unpkg.com/swiper@10.3/swiper-bundle.min.css';
const SWIPER_JS = 'https://unpkg.com/swiper@10.3/swiper-bundle.min.js';

function initSwiper() {
  const swiper_api = document.getElementById('swiper-api');
  if (swiper_api == undefined) return;
  // 已初始化则跳过（swiper 会在容器上挂 swiper-initialized 类）
  if (swiper_api.classList.contains('swiper-initialized')) return;
  utils.css(SWIPER_CSS);
  utils.js(SWIPER_JS, { defer: true }).then(function () {
    if (typeof Swiper === 'undefined') return;
    const effect = swiper_api.getAttribute('effect') || '';
    new Swiper('.swiper#swiper-api', {
      slidesPerView: 'auto',
      spaceBetween: 8,
      centeredSlides: true,
      effect: effect,
      rewind: true,
      pagination: {
        el: '.swiper-pagination',
        clickable: true,
      },
      navigation: {
        nextEl: '.swiper-button-next',
        prevEl: '.swiper-button-prev',
      },
    });
  });
}

if (document.readyState === 'loading') {
  window.addEventListener('DOMContentLoaded', initSwiper);
} else {
  initSwiper();
}
// PJAX 换页后重新检测初始化
document.addEventListener('pjax:complete', initSwiper);
