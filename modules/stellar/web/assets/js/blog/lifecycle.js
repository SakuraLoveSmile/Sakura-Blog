// Blog 扩展拥有自己的 region 生命周期，不向严格 Stellar 配置添加字段。
(function () {
  'use strict';
  function createScope(root, context = {}) {
    const documentRef = root.ownerDocument || root;
    const view = documentRef.defaultView || window;
    const controller = new AbortController();
    const cleanups = [];
    const timeouts = new Set(), intervals = new Set(), frames = new Set();
    const active = () => !controller.signal.aborted;
    function cleanup() {
      if (!active()) return;
      controller.abort();
      for (const id of timeouts) view.clearTimeout(id);
      for (const id of intervals) view.clearInterval(id);
      for (const id of frames) view.cancelAnimationFrame(id);
      cleanups.splice(0).reverse().forEach(fn => fn());
    }
    context.signal?.addEventListener('abort', cleanup, { once: true });
    cleanups.push(() => context.signal?.removeEventListener('abort', cleanup));
    context.onCleanup?.(cleanup);
    function listen(target, event, handler, options) {
      if (!target || !active()) return;
      const guarded = function (...args) { if (active()) handler.apply(this, args); };
      target.addEventListener(event, guarded, options);
      cleanups.push(() => target.removeEventListener?.(event, guarded, options));
    }
    const doc = new Proxy(documentRef, { get(target, key) {
      if (key === 'querySelector') return selector => root.querySelector(selector);
      if (key === 'querySelectorAll') return selector => root.querySelectorAll(selector);
      if (key === 'getElementById') return id => {
        const element = target.getElementById(id);
        return root === target || root.contains(element) ? element : null;
      };
      if (key === 'getElementsByClassName') return name => root.getElementsByClassName(name);
      if (key === 'addEventListener') return (event, handler, options) => listen(root, event, handler, options);
      const value = Reflect.get(target, key, target);
      return typeof value === 'function' ? value.bind(target) : value;
    } });
    const wrapResponse = response => {
      controller.signal.throwIfAborted();
      return new Proxy(response, { get(target, key) {
        if (key === 'json' || key === 'text') return async () => {
          const result = await target[key](); controller.signal.throwIfAborted(); return result;
        };
        const value = Reflect.get(target, key, target);
        return typeof value === 'function' ? value.bind(target) : value;
      } });
    };
    const request = (url, options = {}) => view.fetch(url, { ...options, signal: controller.signal }).then(wrapResponse);
    const clientRequest = (url, options = {}) => context.request
      ? context.request.request(url, { ...options, signal: controller.signal }).then(wrapResponse)
      : request(url, options);
    // 经典 promise 链在卸载后停止回调，包括错误提示和 finally 的按钮回写。
    function guard(promise) {
      const then = (success, failure) => guard(Promise.prototype.then.call(promise,
        value => active() ? (success ? success(value) : value) : undefined,
        error => active() ? (failure ? failure(error) : Promise.reject(error)) : undefined));
      Object.defineProperties(promise, {
        then: { value: then, configurable: true },
        catch: { value: failure => then(undefined, failure), configurable: true },
        finally: { value: handler => then(value => { handler(); return value; }, error => { handler(); throw error; }), configurable: true }
      });
      return promise;
    }
    function timer(kind, callback, delay) {
      const set = kind === 'setInterval' ? intervals : timeouts;
      const id = view[kind](() => { if (kind !== 'setInterval') set.delete(id); if (active()) callback(); }, delay);
      set.add(id); return id;
    }
    function observer(Type, callback, options) {
      const instance = new view[Type]((...args) => { if (active()) callback(...args); }, options);
      cleanups.push(() => instance.disconnect()); return instance;
    }
    return { document: doc, signal: controller.signal, active, cleanup, listen, request, clientRequest,
      fetch: (url, options) => guard(request(url, options)),
      setTimeout: (fn, delay) => timer('setTimeout', fn, delay),
      clearTimeout: id => { timeouts.delete(id); view.clearTimeout(id); },
      setInterval: (fn, delay) => timer('setInterval', fn, delay),
      clearInterval: id => { intervals.delete(id); view.clearInterval(id); },
      requestAnimationFrame: fn => { const id = view.requestAnimationFrame(() => { frames.delete(id); if (active()) fn(); }); frames.add(id); return id; },
      observer, onCleanup: fn => cleanups.push(fn)
    };
  }
  window.blogRuntime = Object.assign(window.blogRuntime || {}, { createScope });
})();
