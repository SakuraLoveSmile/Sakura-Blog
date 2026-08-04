// local-search.js: 站内搜索，移植自 Stellar source/js/search/local-search.js（去 jQuery）。
// Spec 25：优先请求服务端 /api/search（FTS5 bm25 + snippet 高亮），
// 失败/不可用时降级回 /search.json 本地匹配（原有逻辑保留）。
// 数据：GET ctx.search.path（/search.json），localStorage 缓存 search_cache_v1。
// 入口：DOMContentLoaded + pjax:complete —— PJAX 就地更新 l_left 后搜索框是新节点，需重新绑定。
// 绑定标记挂在 DOM 节点上（_searchInitialized/_localSearchBound/_searchObserved），重复 init 不叠加。
var searchCache = null;
var searchCacheKey = 'search_cache_v1';
var serverSearchOK = true; // /api/search 失败一次后本会话内不再重试，直接走本地降级

// 搜索结果的类型展示名（对齐 /api/search 的 type 字段：post/page/moment）。
var SEARCH_TYPE_LABELS = { post: '文章', page: '页面', moment: '瞬间' };

var searchFunc = function(path, filter, wrapperId, searchId, contentId) {
  var $input = document.getElementById(searchId);
  if (!$input || $input._searchInitialized === true) return;
  if ($input._searchInitialized === 'pending' && !searchCache) return;

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

  function initSearch(datas) {
    if (!$input) { return; }
    if ($input._searchInitialized === true) return; // 防止重复绑定
    $input._searchInitialized = true;

    var $resultContent = document.getElementById(contentId);
    var $wrapper = document.getElementById(wrapperId);
    var searchSeq = 0; // 丢弃过时响应，避免乱序覆盖
    var activeType = ''; // 结果类型 tab：''=全部 / post / page / moment
    var lastItems = []; // 最近一次服务端响应条目，tab 切换时客户端过滤（无需重新请求）

    // 单条搜索结果 li 构建（服务端/本地共用 DOM 结构：span.title + p.snippet）
    function resultItem(item) {
      const li = document.createElement('li');
      const a = document.createElement('a');
      a.href = item.path;
      const titleSpan = document.createElement('span');
      titleSpan.className = 'search-result-title';
      titleSpan.textContent = item.title?.trim() || 'Untitled';
      a.appendChild(titleSpan);
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
    // 降级规则：FTS 结果非空但全部被 data-filter 滤掉（当前项目无命中）时，
    // 回退本地搜索（/search.json 索引已含 wiki 文档，同样受 filter 限定）。
    function renderServerResults(items, value) {
      lastItems = items;
      renderTabsAndList(items, value);
    }

    function renderTabsAndList(items, value) {
      $resultContent.innerHTML = '';
      if (!items.length) return;

      // 类型 tab：全部/文章/页面/瞬间，点击切换 activeType 后客户端过滤重渲染
      const tabs = document.createElement('div');
      tabs.className = 'search-result-tabs';
      ['', 'post', 'page', 'moment'].forEach(function(type) {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.textContent = type === '' ? '全部' : (SEARCH_TYPE_LABELS[type] || type);
        if (type === activeType) btn.classList.add('active');
        btn.addEventListener('click', function() {
          if (activeType === type) return;
          activeType = type;
          renderTabsAndList(lastItems, value);
        });
        tabs.appendChild(btn);
      });
      $resultContent.appendChild(tabs);

      const ul = document.createElement('ul');
      ul.className = 'search-result-list';
      const visible = items.filter(function(item) {
        if (filter && !item.path.includes(filter)) return false;
        return !activeType || item.type === activeType;
      });
      if (activeType) {
        // 单类型视图：平铺，保持服务端排序
        visible.forEach(function(item) { ul.appendChild(resultItem(item)); });
      } else {
        // 全部视图：按类型分组展示
        ['post', 'page', 'moment'].forEach(function(type) {
          const group = visible.filter(function(item) { return item.type === type; });
          if (!group.length) return;
          const li = document.createElement('li');
          li.className = 'search-result-group-title';
          li.textContent = SEARCH_TYPE_LABELS[type] || type;
          ul.appendChild(li);
          group.forEach(function(item) { ul.appendChild(resultItem(item)); });
        });
      }
      if (ul.children.length) {
        $resultContent.appendChild(ul);
      } else if (filter) {
        localSearch(value);
      }
    }

    // 服务端搜索请求（失败一次后本会话降级本地）
    function serverSearch(value) {
      if (!serverSearchOK) {
        localSearch(value);
        return;
      }
      var seq = ++searchSeq;
      fetch('/api/search?q=' + encodeURIComponent(value) + '&limit=20')
        .then(function(res) {
          if (!res.ok) throw new Error('search api ' + res.status);
          return res.json();
        })
        .then(function(json) {
          if (seq !== searchSeq) return; // 过时响应丢弃
          renderServerResults(json.items || [], value);
        })
        .catch(function() {
          serverSearchOK = false; // 降级后本会话内不再请求服务端
          localSearch(value);
        });
    }

    // 本地全文匹配（原逻辑，作 /api/search 不可用时的降级路径）
    function localSearch(value) {
      var resultList = [];
      var keywords = getAllCombinations(value.toLowerCase().split(" "))
        .sort(function(a,b) { return b.split(" ").length - a.split(" ").length; });
      $resultContent.innerHTML = "";

      (datas || searchCache || []).forEach(function(data) {
        if (!data.content?.trim().length) return;
        if (filter && !data.path.includes(filter)) return;

        var matches = 0;
        var dataTitle = data.title?.trim() || 'Untitled';
        var dataTitleLowerCase = dataTitle.toLowerCase();
        var dataContent = data.content;
        var dataContentLowerCase = dataContent.toLowerCase();
        var dataUrl = data.path.startsWith('//') ? data.path.slice(1) : data.path;
        dataUrl = dataUrl.replace(/\/?index\.html$/, '/'); // index.html → /
        dataUrl = dataUrl.replace(/\.html$/, '/'); // xxx.html → xxx/

        var indexTitle = -1;
        var indexContent = -1;
        var firstOccur = -1;

        keywords.forEach(function(keyword) {
          indexTitle = dataTitleLowerCase.indexOf(keyword);
          indexContent = dataContentLowerCase.indexOf(keyword);
          if (indexTitle >= 0 || indexContent >= 0) {
            matches += 1;
            if (indexContent < 0) indexContent = 0;
            if (firstOccur < 0) firstOccur = indexContent;
          }
        });

        if (matches > 0) {
          const li = document.createElement('li');
          const a = document.createElement('a');
          a.href = dataUrl;

          const titleSpan = document.createElement('span');
          titleSpan.className = 'search-result-title';
          titleSpan.textContent = dataTitle;
          a.appendChild(titleSpan);

          if (firstOccur >= 0) {
            var start = Math.max(0, firstOccur - 20);
            var end = Math.min(dataContent.length, firstOccur + 80);
            if (start === 0) end = 100;
            var matchContent = dataContent.substring(start, end);

            var regS = new RegExp(keywords.map(escapeRegExp).join("|"), "gi");
            matchContent = matchContent.replace(regS, function(keyword) {
              return "<span class=\"search-keyword\">" + keyword + "</span>";
            });

            const para = document.createElement('p');
            para.className = 'search-result-content';
            para.innerHTML = matchContent + '...';
            a.appendChild(para);
          }

          li.appendChild(a);
          resultList.push({ rank: matches, element: li });
        }
      });

      if (resultList.length) {
        resultList.sort(function(a, b) {
          return b.rank - a.rank;
        });

        const ul = document.createElement('ul');
        ul.className = 'search-result-list';
        resultList.forEach(function(item) {
          ul.appendChild(item.element);
        });

        $resultContent.innerHTML = '';
        $resultContent.appendChild(ul);
      }
    }

    $input.addEventListener("input", function() {
      var value = this.value.trim();
      $resultContent.innerHTML = "";
      if (value.length <= 0) {
        $wrapper.setAttribute('searching', 'false');
        return;
      }
      $wrapper.setAttribute('searching', 'true');
      serverSearch(value);
    });
  }

  if (serverSearchOK || searchCache) {
    // 服务端搜索可用时无需等本地索引，立即绑定（datas 可为 null，降级时兜底 searchCache）
    initSearch(searchCache);
  } else {
    $input._searchInitialized = 'pending';
    // 数据还没准备好，延迟初始化
    const timer = setInterval(() => {
      if (searchCache) {
        clearInterval(timer);
        initSearch(searchCache);
      }
    }, 100);
  }
};

// 预取搜索索引：先用 localStorage 缓存秒开，再后台拉新覆盖。
function preloadSearchData() {
  var path = ctx.search.path;
  if (path.startsWith('/')) {
    path = path.substring(1);
  }
  path = ctx.root + path;

  try {
    var cached = localStorage.getItem(searchCacheKey);
    if (cached) {
      searchCache = JSON.parse(cached);
    }
  } catch (e) {
    console.warn('搜索缓存解析失败', e);
  }

  fetch(path)
    .then(res => res.json())
    .then(json => {
      searchCache = json;
      try {
        localStorage.setItem(searchCacheKey, JSON.stringify(json));
      } catch (e) {
        console.warn('搜索缓存写入失败', e);
      }
    });
}

// 绑定搜索框事件。PJAX 换侧栏后 input 是新节点（无绑定标记），会重新走一遍绑定。
function initLocalSearch() {
  var inputArea = document.querySelector('input#search-input');
  if (!inputArea) return;
  var resultArea = document.querySelector('div#search-result');
  if (!resultArea) return;

  if (!inputArea._localSearchBound) {
    inputArea._localSearchBound = true;
    inputArea.addEventListener('focus', function() {
      var path = ctx.search.path;
      if (path.startsWith('/')) {
        path = path.substring(1);
      }
      path = ctx.root + path;
      const filter = inputArea.getAttribute('data-filter') || '';
      searchFunc(path, filter, 'search-wrapper', 'search-input', 'search-result');
    });
    inputArea.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
      }
    });
  }

  // 结果区增删 li 时切换 .noresult（控制"没有找到内容！"显隐）
  if (!resultArea._searchObserved) {
    resultArea._searchObserved = true;
    const observer = new MutationObserver(function() {
      const hasResults = resultArea.querySelector(".search-result-list li");
      const wrapper = document.querySelector('.search-wrapper');
      if (wrapper) wrapper.classList.toggle('noresult', !hasResults);
    });
    observer.observe(resultArea, { childList: true, subtree: true });
  }
}

preloadSearchData();
document.addEventListener('DOMContentLoaded', initLocalSearch);
document.addEventListener('pjax:complete', initLocalSearch);
