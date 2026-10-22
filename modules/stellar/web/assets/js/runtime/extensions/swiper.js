function queryAll(root, selector) { return [...(root.matches?.(selector) ? [root] : []), ...root.querySelectorAll(selector)]; }

export async function mount(root, context) {
  const config = context.extension.config;
  await Promise.all([
    context.assets.style(config.assets.localCss),
    context.assets.style(config.assets.css),
    context.assets.script(config.assets.js)
  ]);
  context.signal?.throwIfAborted();
  const instances = [];
  const cleanup = () => {
    for (const instance of instances.splice(0)) instance.destroy?.(true, true);
  };
  try {
    for (const element of queryAll(root, '#swiper-api')) {
      instances.push(new window.Swiper(element, {
        slidesPerView: 'auto',
        spaceBetween: 8,
        centeredSlides: true,
        effect: element.getAttribute('effect') || '',
        rewind: true,
        pagination: { el: element.querySelector('.swiper-pagination'), clickable: true },
        navigation: {
          nextEl: element.querySelector('.swiper-button-next'),
          prevEl: element.querySelector('.swiper-button-prev')
        }
      }));
    }
  } catch (error) {
    cleanup();
    throw error;
  }
  return cleanup;
}
