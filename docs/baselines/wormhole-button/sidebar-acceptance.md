# 虫洞侧栏入口验收

任务：BLG-20261031-01。唯一布局参照：`sidebar-before.jpg`。

- [x] 桌面展开左侧栏：官方按钮完整显示在底部、actions 后、设置之前。
- [x] 桌面收起栏：虫洞和其他普通 widget 一样隐藏，不裁切标志；页脚入口保留。
- [x] 手机左侧抽屉：展开显示官方按钮，关闭时不可聚焦，设置控件仍可用。
- [x] 四种 appearance，light、dark、auto 浅色/深色使用对应官方图像，无横向溢出。
- [x] 点击区不少于 44px，键盘焦点可见，外链目标及新标签策略正确。
- [x] 当前预览加载的 CSS 与本次源码指纹一致。

实测：四种 appearance × 四种配色状态 × 桌面/手机抽屉共 32 组通过，另含 16 次关闭 inert 和 4 次收起栏检查。数据及限制见 `sidebar-browser-verification.json`，真实加载资源见 `sidebar-checks-verification.json`。
