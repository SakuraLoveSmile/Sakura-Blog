(function () {
// local-search.js: 站内搜索，自建架构（Spec 25 FTS5 服务端优先 + /search.json 本地降级）
// 保留 Go 站内搜索增强：懒加载与缓存 TTL（search.local_search.lazy_load/cache_ttl）、
// 结果按内容标题章节拆分、点击直达章节锚点（href 带 ?kw= 与 #anchor，目标页
// highlight.js 按 kw 高亮关键词）。
// 入口：focus 触发；2.0 runtime 负责挂载/卸载，重新挂载不会叠加事件。
// 绑定标记挂在 DOM 节点上（_searchInitialized/_localSearchBound/_searchObserved），重复 init 不叠加。

var searchCache = null;          // 当前可用的搜索数据（数组）
var searchCacheEntry = null;     // 最近读取/写入的缓存条目 { ts, ttl, data }
var searchCacheKey = 'search_cache_v4';
var searchFetchController = null;
var searchFetchPromise = null;   // 单飞：同一时刻只允许一个请求
var serverSearchOK = true;       // /api/search 失败一次后本会话内不再重试，直接走本地降级

// 懒加载与缓存 TTL（1.41）：ctx 注入，缺省 true / 86400；cache_ttl=0 表示不缓存
var searchLazyLoad = true;
var searchCacheTtl = 86400;
var searchIncludeContent = true;
var searchScope = 'all';
var searchRoots = new WeakMap();
var mountedInputs = new Set();
try {
  var searchCfg = ctx.search && ctx.search.local_search;
  if (searchCfg) {
    searchLazyLoad = searchCfg.lazy_load !== false;
    searchIncludeContent = searchCfg.content !== false;
    searchScope = searchCfg.field || 'all';
    searchCacheTtl = typeof searchCfg.cache_ttl === 'number' ? Math.max(0, searchCfg.cache_ttl) : 86400;
  }
} catch (e) {}

// 搜索结果的类型展示名（对齐 /api/search 的 type 字段：post/page/moment）。
var SEARCH_TYPE_LABELS = { post: '文章', page: '页面', wiki: '知识库', note: '笔记', moment: '瞬间' };

function sitePath(path) {
  var root = '/' + (ctx.root || '/').replace(/^\/+|\/+$/g, '');
  if (root !== '/') root += '/';
  if (/^https?:\/\//.test(path)) return path;
  if (path.startsWith(root)) return path;
  return root + path.replace(/^\/+/, '');
}

function getSearchPath() { return sitePath(ctx.search.path); }

function readSearchCache() {
  try {
    var raw = localStorage.getItem(searchCacheKey);
    if (!raw) return null;
    var entry = JSON.parse(raw);
    if (!entry || typeof entry.ts !== 'number' || !Array.isArray(entry.data)) return null;
    if (entry.scope !== searchScope || entry.content !== searchIncludeContent) return null;
    return entry;
  } catch (e) {
    console.warn('搜索缓存读取失败', e);
    return null;
  }
}

function writeSearchCache(data) {
  if (searchCacheTtl <= 0) return;
  try {
    var entry = { ts: Date.now(), ttl: searchCacheTtl, scope: searchScope, content: searchIncludeContent, data: data };
    localStorage.setItem(searchCacheKey, JSON.stringify(entry));
    searchCacheEntry = entry;
  } catch (e) {
    console.warn('搜索缓存写入失败', e);
  }
}

function isCacheFresh(entry) {
  return !!(entry && typeof entry.ts === 'number' && entry.ttl > 0 && Date.now() - entry.ts < Math.min(entry.ttl, searchCacheTtl) * 1000);
}

// 从 localStorage 恢复缓存到内存（ttl=0 时不使用历史缓存）
function loadCacheIntoMemory() {
  if (searchCacheTtl <= 0) {
    searchCacheEntry = null;
    return null;
  }
  if (!searchCacheEntry) {
    searchCacheEntry = readSearchCache();
  }
  if (searchCacheEntry && Array.isArray(searchCacheEntry.data)) {
    searchCache = searchCacheEntry.data;
  }
  return searchCacheEntry;
}

// 是否需要发起请求：不缓存 / 无缓存 / 缓存过期
function needsFetch() {
  if (searchCacheTtl <= 0) return true;
  var entry = searchCacheEntry || loadCacheIntoMemory();
  return !isCacheFresh(entry);
}

// 拉取搜索索引（单飞），成功后更新内存与 localStorage
function fetchSearchData(path) {
  if (searchFetchPromise) return searchFetchPromise;
  var controller = new AbortController();
  searchFetchController = controller;
  var request = fetch(path, { signal: controller.signal })
    .then(function(res) {
      if (!res.ok) throw new Error('search fetch failed: ' + res.status);
      return res.json();
    })
    .then(function(json) {
      controller.signal.throwIfAborted();
      if (!Array.isArray(json)) throw new Error('search data is not an array');
      searchCache = json;
      writeSearchCache(json);
      return json;
    })
    .catch(function(err) {
      if (!controller.signal.aborted) console.warn('搜索数据加载失败', err);
      throw err;
    })
    .finally(function() {
      if (searchFetchPromise === request) searchFetchPromise = null;
      if (searchFetchController === controller) searchFetchController = null;
    });
  searchFetchPromise = request;
  return request;
}

function bindLocalSearch(input, target, type, handler) {
  target.addEventListener(type, handler);
  input._searchCleanups = input._searchCleanups || [];
  input._searchCleanups.push(function() { target.removeEventListener(type, handler); });
}

var searchFunc = function(path, filter, $wrapper, $input, $resultContent) {
  if (!$input || $input._searchInitialized === true) return;
  if ($input._searchInitialized === 'pending') return; // 数据加载中，等待完成后初始化
  var lifecycle = $input._searchLifecycle;
  function isActive() { return $input._searchLifecycle === lifecycle && $input.isConnected !== false; }

  function getAllCombinations(keywords) {
    const result = [];
    const maxLen = 3; // 组合最大长度
    for (let i = 0; i < keywords.length; i++) {
      for (let j = i + 1; j <= Math.min(i + maxLen, keywords.length); j++) {
        result.push(keywords.slice(i, j).join(" "));
      }
    }
    return result;
  }

  function escapeRegExp(string) {
    return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  // 关键词组合（含原始大小写）：按组合长度降序，用于匹配与跳转高亮
  function buildKeywordPairs(value) {
    const tokens = value.trim().split(/\s+/);
    const combos = getAllCombinations(tokens.map(function(t) { return t.toLowerCase(); }));
    const originalCombos = getAllCombinations(tokens);
    const pairs = combos.map(function(kw, i) {
      return { kw: kw, original: originalCombos[i] };
    });
    return pairs.sort(function(a, b) {
      return b.kw.split(" ").length - a.kw.split(" ").length;
    });
  }

  function getCardHoverApi() {
    if (typeof stellar === 'undefined' || !stellar.cardHover) return null;
    return stellar.cardHover;
  }

  function unmountResultCards(root) {
    var cardHover = getCardHoverApi();
    if (cardHover && typeof cardHover.unmountAll === 'function') {
      cardHover.unmountAll(root);
    }
  }

  function mountResultCards(root) {
    var cardHover = getCardHoverApi();
    if (cardHover && typeof cardHover.mountAll === 'function') {
      cardHover.mountAll(root);
    }
  }

  function initSearch() {
    if (!$input || !isActive()) return;
    if ($input._searchInitialized === true) return; // 防止重复绑定
    $input._searchInitialized = true;

    var serverController = null;
    var searchSeq = 0; // 丢弃过时响应，避免乱序覆盖
    $input._searchCleanups.push(function() { searchSeq += 1; serverController?.abort(); });
    var activeType = ''; // 结果类型 tab：''=全部 / post / page / moment
    var lastItems = []; // 最近一次服务端响应条目，tab 切换时客户端过滤（无需重新请求）

    // 单条搜索结果 li 构建（服务端/本地共用 DOM 结构）。
    // 2.0 结果卡：文章标题、章节名和摘要共用同一个可点击链接；
    // href 统一拼 ?kw=首词（+ #锚点），目标页 highlight.js 高亮并定位。
    function resultItem(item, kw) {
      const li = document.createElement('li');

      const titleSpan = document.createElement('span');
      titleSpan.className = 'search-result-title';
      titleSpan.textContent = item.title?.trim() || 'Untitled';
      const a = document.createElement('a');
      a.className = ctx.ui.classes.interactiveSpotlight;
      a.href = item.path + (kw ? '?kw=' + encodeURIComponent(kw) : '') + (item.anchor_id ? '#' + encodeURIComponent(item.anchor_id) : '');
      a.appendChild(titleSpan);

      if (item.anchor_text) {
        const sectionSpan = document.createElement('span');
        sectionSpan.className = 'search-result-section';
        sectionSpan.textContent = item.anchor_text;
        a.appendChild(sectionSpan);
      }
      if (item.snippet) {
        const para = document.createElement('p');
        para.className = 'search-result-content';
        para.innerHTML = item.snippet;
        a.appendChild(para);
      }
      li.appendChild(a);
      return li;
    }

    // 服务端搜索结果渲染：类型 tab 栏 + 结果列表（snippet 已含 .search-keyword 高亮）。
    // 全部视图按类型分组（组标题 文章/页面/瞬间），单类型视图平铺；tab 点击走客户端过滤。
    // 保留旧扩展 data-filter 的路径筛选；strict domain/scope 由公开快照 API 精确筛选。
    // 普通搜索的空结果仍保持为空，类型 tab 的空结果也不扩大搜索范围。
    function renderServerResults(items, value, kw, seq) {
      lastItems = items;
      if (filter && !items.some(function(item) { return item.path.includes(filter); })) {
        runLocalSearch(value, seq);
        return;
      }
      renderTabsAndList(items, value, kw);
    }

    function renderTabsAndList(items, value, kw) {
      unmountResultCards($resultContent);
      $resultContent.innerHTML = '';
      if (!items.length) return;

      // 类型 tab：全部/文章/页面/瞬间，点击切换 activeType 后客户端过滤重渲染
      const tabs = document.createElement('div');
      tabs.className = 'search-result-tabs';
      [''].concat(Object.keys(SEARCH_TYPE_LABELS)).forEach(function(type) {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.textContent = type === '' ? '全部' : (SEARCH_TYPE_LABELS[type] || type);
        if (type === activeType) btn.classList.add('active');
        btn.addEventListener('click', function() {
          if (activeType === type) return;
          activeType = type;
          renderTabsAndList(lastItems, value, kw);
        });
        tabs.appendChild(btn);
      });
      $resultContent.appendChild(tabs);

      const ul = document.createElement('ul');
      ul.className = 'search-result-list ui-collection ui-collection-adapter';
      ul.dataset.layout = 'list';
      ul.dataset.variant = 'summary';
      ul.dataset.density = 'compact';
      ul.dataset.uiSurface = 'glass';
      const visible = items.filter(function(item) {
        if (filter && !item.path.includes(filter)) return false;
        return !activeType || item.type === activeType;
      });
      if (activeType) {
        // 单类型视图：平铺，保持服务端排序
        visible.forEach(function(item) { ul.appendChild(resultItem(item, kw)); });
      } else {
        // 全部视图：按类型分组展示
        Object.keys(SEARCH_TYPE_LABELS).forEach(function(type) {
          const group = visible.filter(function(item) { return item.type === type; });
          if (!group.length) return;
          const li = document.createElement('li');
          li.className = 'search-result-group-title';
          li.textContent = SEARCH_TYPE_LABELS[type] || type;
          ul.appendChild(li);
          group.forEach(function(item) { ul.appendChild(resultItem(item, kw)); });
        });
      }
      if (ul.children.length) {
        $resultContent.appendChild(ul);
        mountResultCards(ul);
      }
    }

    // JSON 降级与后台刷新共享一个索引请求；等待完成后再检索，不能先用空缓存。
    function runLocalSearch(value, seq) {
      if (!isActive() || seq !== searchSeq) return;
      loadCacheIntoMemory();
      if (searchCache && !searchFetchPromise && !needsFetch()) { localSearch(value); return; }
      (searchFetchPromise || fetchSearchData(path)).then(function() {
        if (isActive() && seq === searchSeq) localSearch(value);
      }).catch(function() {
        if (!isActive() || seq !== searchSeq) return;
        // 后台刷新失败不丢弃已有可用索引；仍按当前词与项目过滤检索。
        if (searchCache) localSearch(value);
        else $wrapper.setAttribute('searching', 'false');
      });
    }

    // 服务端搜索请求（失败一次后本会话降级本地，不增加重试）。
    function serverSearch(value) {
      var seq = ++searchSeq;
      if (!serverSearchOK) {
        runLocalSearch(value, seq);
        return;
      }
      serverController?.abort();
      serverController = new AbortController();
      var parameters = new URLSearchParams({ q: value, limit: '20', scope: searchScope, include_content: String(searchIncludeContent), domain: $input.getAttribute('data-domain') || '' });
      fetch(sitePath('api/search') + '?' + parameters, { signal: serverController.signal })
        .then(function(res) {
          if (!res.ok) throw new Error('search api ' + res.status);
          return res.json();
        })
        .then(function(json) {
          if (!isActive() || seq !== searchSeq) return; // 卸载/过时响应丢弃
          renderServerResults(json.items || [], value, value.trim().split(/\s+/)[0] || '', seq);
        })
        .catch(function() {
          if (!isActive() || seq !== searchSeq) return;
          serverSearchOK = false; // 降级后本会话内不再请求服务端
          runLocalSearch(value, seq);
        });
    }

    // 本地全文匹配（/api/search 不可用时的降级路径）。
    // 1.41 章节化：索引条目 anchors 把正文切成章节，逐章节独立匹配出结果
    // （每条带章节名与 kw+锚点跳转链接）；anchors 缺失回退页面级单结果；
    // 标题命中仅在无任何章节命中时兜底。
    function localSearch(value) {
      var resultList = [];
      var pairs = buildKeywordPairs(value);
      var firstKw = pairs.length ? pairs[pairs.length - 1].original : '';
      unmountResultCards($resultContent);
      $resultContent.innerHTML = "";

      (searchCache || []).forEach(function(data) {
        if (!data || typeof data.path !== 'string') return;
        if (filter && !data.path.includes(filter)) return;
        var domain = $input.getAttribute('data-domain') || '';
        if (domain && !data.domains?.includes(domain)) return;

        var dataTitle = data.title?.trim() || 'Untitled';
        var dataTitleLowerCase = dataTitle.toLowerCase();
        var dataContent = searchIncludeContent && typeof data.content === 'string' ? data.content : '';
        var dataContentLowerCase = dataContent.toLowerCase();
        var dataUrl = data.path.startsWith('//') ? data.path.slice(1) : data.path;
        dataUrl = dataUrl.replace(/\/?index\.html$/, '/'); // index.html → /
        dataUrl = dataUrl.replace(/\.html$/, '/'); // xxx.html → xxx/

        // 章节划分：anchors 缺失/为空 → 整页一段（回退页面级行为）
        var anchors = dataContent && Array.isArray(data.anchors) ? data.anchors : [];
        var sections = [];
        if (anchors.length === 0) {
          sections.push({ anchor: null, offset: 0, end: dataContent.length });
        } else {
          for (var s = 0; s <= anchors.length; s++) {
            sections.push({
              anchor: s === 0 ? null : anchors[s - 1],
              offset: s === 0 ? 0 : anchors[s - 1].offset,
              end: s === anchors.length ? dataContent.length : anchors[s].offset
            });
          }
        }

        // 标题命中（仅当无任何章节命中时兜底为页面级结果）
        var titleMatches = 0;
        var titleBestKw = null;
        pairs.forEach(function(p) {
          var metadata = [dataTitleLowerCase].concat(data.tags || [], data.categories || []);
          if (metadata.some(function(term) { return typeof term === 'string' && term.toLowerCase().indexOf(p.kw) >= 0; })) {
            titleMatches += 1;
            if (titleBestKw == null) titleBestKw = p.original;
          }
        });

        var hasSectionResult = false;

        // 逐章节匹配
        sections.forEach(function(section) {
          var secText = dataContent.slice(section.offset, section.end);
          if (!secText.trim().length) return;
          var secLower = dataContentLowerCase.slice(section.offset, section.end);
          var secMatches = 0;
          var secFirst = -1;
          var secBestKw = null;
          pairs.forEach(function(p) {
            var pos = secLower.indexOf(p.kw);
            if (pos < 0) return;
            secMatches += 1;
            if (secFirst < 0 || pos < secFirst) secFirst = pos;
            if (secBestKw == null) secBestKw = p.original;
          });
          if (secMatches === 0) return;

          var href = dataUrl;
          if (section.anchor) {
            href += '?kw=' + encodeURIComponent(secBestKw) + '#' + encodeURIComponent(section.anchor.id);
          } else {
            href += '?kw=' + encodeURIComponent(secBestKw);
          }
          resultList.push({
            rank: secMatches,
            offset: section.offset,
            element: buildLocalResultElement(dataTitle, section.anchor ? section.anchor.text : null, secText, secFirst, pairs, href)
          });
          hasSectionResult = true;
        });

        if (titleMatches > 0 && !hasSectionResult) {
          resultList.push({
            rank: titleMatches,
            offset: Number.MAX_SAFE_INTEGER,
            element: buildLocalResultElement(dataTitle, null, null, -1, pairs, dataUrl + '?kw=' + encodeURIComponent(titleBestKw || firstKw))
          });
        }
      });

      if (resultList.length) {
        resultList.sort(function(a, b) {
          return b.rank - a.rank || a.offset - b.offset;
        });

        const ul = document.createElement('ul');
        ul.className = 'search-result-list ui-collection ui-collection-adapter';
        ul.dataset.layout = 'list';
        ul.dataset.variant = 'summary';
        ul.dataset.density = 'compact';
        ul.dataset.uiSurface = 'glass';
        resultList.forEach(function(item) {
          ul.appendChild(item.element);
        });
        $resultContent.innerHTML = '';
        $resultContent.appendChild(ul);
        mountResultCards(ul);
      }
    }

    // 本地降级路径的单条结果 DOM（对齐上游 2.0 buildResultElement）
    function buildLocalResultElement(dataTitle, sectionName, secText, secFirst, pairs, href) {
      const li = document.createElement('li');

      const titleSpan = document.createElement('span');
      titleSpan.className = 'search-result-title';
      titleSpan.textContent = dataTitle;
      const a = document.createElement('a');
      a.className = ctx.ui.classes.interactiveSpotlight;
      a.href = href;
      a.appendChild(titleSpan);

      if (sectionName) {
        const sectionSpan = document.createElement('span');
        sectionSpan.className = 'search-result-section';
        sectionSpan.textContent = sectionName;
        a.appendChild(sectionSpan);
      }

      if (secText && secFirst >= 0) {
        var start = Math.max(0, secFirst - 20);
        var end = Math.min(secText.length, secFirst + 80);
        if (start === 0) end = 100;
        var matchContent = secText.substring(start, end).replace(/[&<>"']/g, function(character) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[character]; });

        var regS = new RegExp(pairs.map(function(p) { return escapeRegExp(p.kw); }).join("|"), "gi");
        matchContent = matchContent.replace(regS, function(keyword) {
          return "<span class=\"search-keyword\">" + keyword + "</span>";
        });

        const para = document.createElement('p');
        para.className = 'search-result-content';
        para.innerHTML = matchContent + '...';
        a.appendChild(para);
      }

      li.appendChild(a);
      return li;
    }

    bindLocalSearch($input, $input, "input", function() {
      var value = this.value.trim();
      searchSeq += 1;
      serverController?.abort();
      unmountResultCards($resultContent);
      $resultContent.innerHTML = "";
      if (value.length <= 0) {
        $wrapper.setAttribute('searching', 'false');
        return;
      }
      $wrapper.setAttribute('searching', 'true');
      serverSearch(value);
    });

    bindLocalSearch($input, $input, "keydown", function(e) {
      if (e.key == 'Enter') {
        e.preventDefault();
      }
    });

    // 同步当前状态：已有文字立即执行一次搜索，否则清除加载态
    if ($input.value && $input.value.trim().length > 0) {
      $input.dispatchEvent(new Event('input'));
    } else {
      $wrapper.setAttribute('searching', 'false');
    }
  }

  // 服务端搜索可用无需等本地索引，立即初始化（降级时兜底 searchCache）
  if (serverSearchOK) {
    initSearch();
    return;
  }
  // 已有可用数据（新鲜或过期）→ 立即初始化
  if (searchCache) {
    initSearch();
    return;
  }
  // 尝试从 localStorage 恢复
  loadCacheIntoMemory();
  if (searchCache) {
    initSearch();
    return;
  }
  // 无数据：标记等待，拉取完成后初始化
  $input._searchInitialized = 'pending';
  fetchSearchData(path)
    .then(function() {
      if (isActive() && $input._searchInitialized === 'pending' && searchCache) {
        initSearch();
      }
    })
    .catch(function() {
      if (!isActive()) return;
      if ($input._searchInitialized === 'pending') {
        $input._searchInitialized = undefined; // 允许下次聚焦重试
      }
      if ($wrapper) $wrapper.setAttribute('searching', 'false');
    });
};

// 使用上游 class 契约：对话框和可选 sidebar widget 各自绑定自己的输入、结果与状态。
function initLocalSearch(root) {
  var inputs = [];
  function bindWrapper(wrapper) {
    var input = wrapper.querySelector('.search-input');
    var result = wrapper.querySelector('.search-result');
    if (!input || !result || inputs.includes(input)) return;
    inputs.push(input);
    mountedInputs.add(input);
    input._searchLifecycle = (input._searchLifecycle || 0) + 1;
    input._searchCleanups = [];
    function initialize() {
      var path = getSearchPath();
      if (input._searchInitialized === true) {
        if (needsFetch()) fetchSearchData(path).catch(function() {});
        return;
      }
      searchFunc(path, input.getAttribute('data-filter') || '', wrapper, input, result);
      if (searchCache && needsFetch()) fetchSearchData(path).catch(function() {});
    }
    bindLocalSearch(input, input, 'focus', initialize);
    var observer = new MutationObserver(function() {
      wrapper.classList.toggle('noresult', !result.querySelector('.search-result-list li'));
    });
    observer.observe(result, { childList: true, subtree: true });
    input._searchCleanups.push(function() { observer.disconnect(); });
    if (input === document.activeElement || input.value.trim()) initialize();
    return initialize;
  }
  root.querySelectorAll('.search-wrapper').forEach(bindWrapper);
  // partial navigation 会替换 sidebar region；新 widget 通过同一 document 的 focusin 按需接入。
  var onFocus = function(event) {
    var input = event.target;
    if (!input?.classList?.contains('search-input') || inputs.includes(input)) return;
    var wrapper = input.closest('.search-wrapper');
    if (wrapper) bindWrapper(wrapper)?.();
  };
  root.addEventListener('focusin', onFocus);
  inputs.cleanupRoot = function() { root.removeEventListener('focusin', onFocus); };
  return inputs;
}

function disposeInputs(inputs) {
  inputs.forEach(function(input) {
    input._searchLifecycle = (input._searchLifecycle || 0) + 1;
    (input._searchCleanups || []).reverse().forEach(function(cleanup) { cleanup(); });
    input._searchCleanups = [];
    input._searchInitialized = undefined;
    mountedInputs.delete(input);
  });
}

window.blogLocalSearch = {
  mount: function(root, storage) {
    // 保留原调用形态给已有宿主；runtime 明确传入本次 document。
    if (!root?.querySelectorAll) { storage = root; root = document; }
    if (searchRoots.has(root)) return searchRoots.get(root);
    var previousPolicy = searchScope + ':' + searchIncludeContent;
    var cfg = ctx.search.local_search || {};
    searchIncludeContent = cfg.content !== false;
    searchScope = cfg.field || 'all';
    searchLazyLoad = cfg.lazy_load !== false;
    searchCacheTtl = typeof cfg.cache_ttl === 'number' ? Math.max(0, cfg.cache_ttl) : 86400;
    var key = storage?.key ? storage.key(getSearchPath(), document.baseURI) : searchCacheKey;
    if (key !== searchCacheKey || previousPolicy !== searchScope + ':' + searchIncludeContent) {
      this.clearMemory(); searchCacheKey = key; serverSearchOK = true;
    }
    if (!searchLazyLoad) {
      loadCacheIntoMemory();
      if (needsFetch()) fetchSearchData(getSearchPath()).catch(function() {});
    }
    var inputs = initLocalSearch(root);
    var cleanup = function() {
      if (searchRoots.get(root) !== cleanup) return;
      inputs.cleanupRoot();
      disposeInputs(inputs);
      searchRoots.delete(root);
      if (!mountedInputs.size) window.blogLocalSearch.clearMemory();
    };
    searchRoots.set(root, cleanup);
    return cleanup;
  },
  clearMemory: function() {
    searchFetchController?.abort();
    searchFetchController = null; searchFetchPromise = null;
    searchCache = null; searchCacheEntry = null;
  },
  unmount: function(root) { searchRoots.get(root || document)?.(); }
};

})();
