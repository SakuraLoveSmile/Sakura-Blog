---
title: Stellar 主题 CSS 编译方案
date: 2026-07-15
excerpt: 用 shim 注入 hexo-config 函数，脱离 Hexo 直接编译 Stylus 源码。
tags: [CSS, Stellar, Stylus]
categories: [技术, 前端]
---

## 背景问题

Stellar 主题的 Stylus 源码大量调用 `hexo-config()` 读取 Hexo 配置，脱离 Hexo 环境编译会报 undefined。

## 解决方案

用 Stylus JS API 的 `define()` 注入一个 shim 函数，按点路径从预提取的 config.json 取值。

```js
stylus.define('hexo-config', (path) => getValue(config, path))
```

## 类型坑

返回字符串 `"false"` 在 `if` 里会判真，布尔判断必须返回真正的 `Boolean`。
