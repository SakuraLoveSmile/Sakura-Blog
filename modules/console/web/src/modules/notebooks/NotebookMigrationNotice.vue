<script setup lang="ts">
// 旧版笔记本和笔记尚未迁移到 Stellar 2.0 时的统一说明：讲清影响与下一步，后端原始信息收进“技术详情”。
defineProps<{ detail?: string }>();
</script>

<template>
  <section class="notebooks-notice" role="alert" aria-labelledby="notebooks-notice-title">
    <h2 id="notebooks-notice-title" class="notebooks-notice-title">需要先迁移到 Stellar 2.0</h2>
    <p>这个站点里还有旧版笔记本和笔记没有迁移。迁移完成前：</p>
    <ul>
      <li>博客前台的所有页面（包括笔记本）都只显示“站点内容正在升级”，访客看不到任何笔记；</li>
      <li>这里不能查看、编辑或新建笔记，避免旧数据被改坏。</li>
    </ul>
    <p>请站点管理员在服务器上审阅迁移预览，确认笔记本和笔记的归属与结构后执行迁移，再重启服务。</p>
    <details v-if="detail" class="notebooks-notice-detail">
      <summary>技术详情</summary>
      <pre>{{ detail }}</pre>
    </details>
    <div v-if="$slots.default" class="notebooks-actions mt-3"><slot /></div>
  </section>
</template>
