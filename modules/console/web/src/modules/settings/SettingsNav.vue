<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { RouterLink, useRoute } from "vue-router";

// 设置的分组导航。桌面是左侧纵向目录，手机复用同一份数据显示为“分组列表”，每项带一句说明。
interface NavPage {
  key: string;
  label: string;
  summary: string;
  dirty: boolean;
}
interface NavGroup {
  key: string;
  label: string;
  pages: NavPage[];
}
withDefaults(defineProps<{ groups: NavGroup[]; active?: string; variant?: "nav" | "list" }>(), {
  active: "",
  variant: "nav",
});

const route = useRoute();
const target = (page: string) => ({ path: route.path, query: { ...route.query, tab: page } });
</script>

<template>
  <nav class="settings-nav" :class="`is-${variant}`" aria-label="设置分类">
    <div v-for="group in groups" :key="group.key" class="settings-nav-group">
      <p :id="`settings-nav-${group.key}`" class="settings-nav-heading">{{ group.label }}</p>
      <ul class="settings-nav-list" :aria-labelledby="`settings-nav-${group.key}`">
        <li v-for="page in group.pages" :key="page.key">
          <RouterLink v-slot="{ href, navigate }" :to="target(page.key)" custom>
            <a
              :href="href"
              class="settings-nav-item"
              :class="{ 'is-active': page.key === active }"
              :aria-current="page.key === active ? 'page' : undefined"
              :data-settings-page="page.key"
              @click="navigate"
            >
              <span class="settings-nav-text">
                <span class="settings-nav-label">{{ page.label }}</span>
                <span v-if="variant === 'list'" class="settings-nav-summary">{{ page.summary }}</span>
              </span>
              <span v-if="page.dirty" class="settings-nav-dot" role="img" aria-label="有未保存修改"></span>
              <Icon v-if="variant === 'list'" icon="ri:arrow-right-s-line" class="settings-nav-chevron" aria-hidden="true" />
            </a>
          </RouterLink>
        </li>
      </ul>
    </div>
    <RouterLink class="settings-nav-link" :to="{ path: '/assistant', query: { tab: 'settings' } }">
      <span>AI 与知识库 · 在「助手」中设置</span>
      <Icon icon="ri:arrow-right-up-line" aria-hidden="true" />
    </RouterLink>
  </nav>
</template>

<style scoped>
.settings-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.settings-nav-heading {
  margin: 0;
  padding: 16px 10px 6px;
  color: var(--color-text-tertiary);
  font-size: 12px;
  font-weight: 500;
}
.settings-nav-group:first-child .settings-nav-heading {
  padding-top: 2px;
}
.settings-nav-list {
  display: flex;
  flex-direction: column;
  /* 选中与悬停的底色是整块的，项之间留一点空隙才不会连成一片。 */
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.settings-nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 32px;
  padding: 0 10px;
  border-radius: var(--radius-sm);
  color: var(--color-text-secondary);
  font-size: 13px;
  text-decoration: none;
}
.settings-nav-item:hover {
  background: color-mix(in srgb, var(--color-accent) 60%, transparent);
  color: var(--color-foreground);
}
.settings-nav-item.is-active {
  background: var(--color-accent);
  color: var(--color-foreground);
  font-weight: 500;
}
.settings-nav-item:focus-visible,
.settings-nav-link:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}
.settings-nav-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.settings-nav-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--color-state-warning);
}
.settings-nav-link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 16px;
  padding: 10px;
  border-top: 1px solid var(--color-border);
  color: var(--color-text-tertiary);
  font-size: 12px;
  text-decoration: none;
}
.settings-nav-link:hover {
  color: var(--color-foreground);
}

/* 手机：分组列表。每项带说明，整行是 ≥56px 的点击区。 */
.settings-nav.is-list {
  gap: 0;
}
.is-list .settings-nav-heading {
  padding: 20px 0 8px;
}
.is-list .settings-nav-group:first-child .settings-nav-heading {
  padding-top: 8px;
}
.is-list .settings-nav-list {
  gap: 0;
  border-top: 1px solid var(--color-border);
}
.is-list .settings-nav-list > li + li {
  border-top: 1px solid var(--color-border);
}
.is-list .settings-nav-item {
  min-height: 56px;
  padding: 8px 0;
  border-radius: 0;
  font-size: 14px;
}
.is-list .settings-nav-item:hover,
.is-list .settings-nav-item.is-active {
  background: transparent;
}
.is-list .settings-nav-label {
  color: var(--color-foreground);
  font-weight: 500;
}
.is-list .settings-nav-summary {
  margin-top: 2px;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.is-list .settings-nav-chevron {
  flex: 0 0 auto;
  color: var(--color-text-tertiary);
}
.is-list .settings-nav-link {
  min-height: 44px;
  margin-top: 24px;
  padding: 10px 0;
  font-size: 13px;
}
</style>
