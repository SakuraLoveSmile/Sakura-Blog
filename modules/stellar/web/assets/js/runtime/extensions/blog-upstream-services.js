// 完整调用官方 services，同时将独立 Blog 数据服务子树留给其所有者。
const official = await import(`./services.js${new URL(import.meta.url).search}`);
export function mount(root, context) {
  const accepts = element => !element.closest?.('[data-blog-service]');
  const scoped = new Proxy(root, { get(target, key) {
    if (key === 'querySelectorAll') return selector => [...target.querySelectorAll(selector)].filter(accepts);
    if (key === 'querySelector') return selector => [...target.querySelectorAll(selector)].find(accepts) || null;
    if (key === 'getElementsByClassName') return name => [...target.getElementsByClassName(name)].filter(accepts);
    if (key === 'contains') return element => target.contains(element) && accepts(element);
    const value = Reflect.get(target, key, target);
    return typeof value === 'function' ? value.bind(target) : value;
  } });
  return official.mount(scoped, context);
}
