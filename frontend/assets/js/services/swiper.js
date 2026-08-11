// swiper.js | 对齐原版 layout/_plugins/swiper.ejs
// 页面存在 #swiper-api 时按需加载 swiper 并初始化；PJAX 换页后重新检测。
// Spec 53 Part B：依赖本地化（对齐 Spec 28 原则，替代 unpkg CDN），版本锁定 10.3.1，
// 文件在 assets/vendor/swiper/，随 embed.go 内嵌分发。
const SWIPER_CSS = '/assets/vendor/swiper/swiper-bundle.min.css';
const SWIPER_JS = '/assets/vendor/swiper/swiper-bundle.min.js';

function initSwiper() {
  const swiper_api = document.getElementById('swiper-api');
  if (swiper_api == undefined) return;
  // 已初始化则跳过（swiper 会在容器上挂 swiper-initialized 类）
  if (swiper_api.classList.contains('swiper-initialized')) return;
  utils.css(SWIPER_CSS);
  utils.js(SWIPER_JS, { defer: true }).then(function () {
    if (typeof Swiper === 'undefined') return;
    const effect = swiper_api.getAttribute('effect') || '';
    const options = {
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
    };
    // Spec 49：coverflow/cards 效果参数透传（对齐 Swiper 10.3 文档，值与原版一致硬编码）
    if (effect === 'coverflow') {
      options.coverflowEffect = {
        rotate: 50,
        stretch: 0,
        depth: 100,
        modifier: 1,
        slideShadows: true,
      };
    }
    if (effect === 'cards') {
      options.cardsEffect = {
        perSlideOffset: 8,
        perSlideRotate: 2,
        slideShadows: true,
      };
    }
    new Swiper('.swiper#swiper-api', options);
  });
}

if (document.readyState === 'loading') {
  window.addEventListener('DOMContentLoaded', initSwiper);
} else {
  initSwiper();
}
