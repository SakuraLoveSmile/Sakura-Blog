---
title: "KaTeX 按需加载验证"
date: 2026-08-13
category: "测试"
tags: [测试, KaTeX, Math]
excerpt: "Spec 61 阶段 1：math-mermaid-loader 按需加载回归验证（展示公式与行内公式）。"
pinned: false
---

## 数学公式渲染验证

### 展示公式

$$E = mc^2$$

$$\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt{\pi}}{2}$$

### 行内公式

质能方程 $E = mc^2$ 是行内公式示例；欧拉恒等式 $e^{i\pi} + 1 = 0$ 同理。

### 圆括号定界符

\[
f(x) = \sum_{n=0}^{\infty} \frac{f^{(n)}(a)}{n!}(x-a)^n
\]

以及行内 \( ax^2 + bx + c = 0 \) 形式。
