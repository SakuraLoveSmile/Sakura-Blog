<script setup lang="ts">
import { provide, computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { ucApi, type UCPostPayload } from "@blog/api-client";
import StellarContentConfig from "@/components/editor/StellarContentConfig.vue";
import { stellarEditorContextKey } from "@/components/editor/stellar-context";
import MarkdownEditor from "@/components/MarkdownEditor.vue";

const route = useRoute();
const router = useRouter();
const isEdit = computed(() => typeof route.params.slug === "string");
const form = ref<UCPostPayload>({
  stellar_version: 2,
  stellar_config: "{}",
  blog_extensions: "{}",
  title: "",
  excerpt: "",
  content_md: "",
  status: "draft",
  tags: "",
});
provide(
  stellarEditorContextKey,
  computed(() => ({
    stellar_version: form.value.stellar_version,
    stellar_config: form.value.stellar_config,
    blog_extensions: form.value.blog_extensions,
  })),
);
const loading = ref(false);
const saving = ref(false);
const error = ref("");

async function load() {
  if (!isEdit.value) return;
  loading.value = true;
  try {
    const { data } = await ucApi.posts.get(String(route.params.slug));
    const draft = data.has_draft && data.draft ? data.draft : {};
    form.value = {
      stellar_version: draft.stellar_version ?? data.stellar_version ?? 0,
      stellar_config: draft.stellar_config ?? data.stellar_config ?? "{}",
      blog_extensions: draft.blog_extensions ?? data.blog_extensions ?? "{}",
      title: draft.title ?? data.title,
      excerpt: draft.excerpt ?? data.excerpt,
      content_md: draft.content_md ?? data.content_md,
      status: draft.status === "published" || data.status === "published" ? "published" : "draft",
      tags: draft.tags ?? data.tags,
    };
  } catch (err) {
    error.value = err instanceof Error ? err.message : "文章加载失败";
  } finally {
    loading.value = false;
  }
}

async function save(status: "draft" | "published" = "draft") {
  saving.value = true;
  error.value = "";
  const payload = { ...form.value, status };
  try {
    if (isEdit.value) await ucApi.posts.update(String(route.params.slug), payload);
    else {
      const { data } = await ucApi.posts.create(payload);
      await router.replace(`/posts/${data.slug}`);
    }
    form.value.status = status;
  } catch (err) {
    error.value = err instanceof Error ? err.message : "保存失败";
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div>
    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div class="flex min-w-0 items-center gap-3">
        <button type="button" class="uc-icon-button" title="返回" @click="router.push('/posts')">
          <Icon icon="ri:arrow-left-line" />
        </button>
        <div class="min-w-0">
          <p class="text-xs font-medium text-text-tertiary">文章编辑</p>
          <h1 class="truncate text-2xl font-semibold tracking-tight">{{ isEdit ? "编辑文章" : "新建文章" }}</h1>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="uc-secondary-button" :disabled="saving || loading" @click="save('draft')">
          保存草稿
        </button>
        <button type="button" class="uc-primary-button" :disabled="saving || loading" @click="save('published')">
          发布
        </button>
      </div>
    </div>
    <div
      v-if="error"
      class="mb-4 rounded-xl border border-state-error/30 bg-state-error/10 px-4 py-3 text-sm text-state-error"
    >
      {{ error }}
    </div>
    <div
      v-if="loading"
      class="rounded-2xl border border-border bg-card px-5 py-12 text-center text-sm text-text-tertiary"
    >
      加载中…
    </div>
    <form v-else class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_19rem]" @submit.prevent="save('draft')">
      <section class="min-w-0 rounded-2xl border border-border bg-card p-4 sm:p-6">
        <input v-model="form.title" class="uc-title-input" placeholder="文章标题" maxlength="200" required />
        <textarea
          v-model="form.excerpt"
          class="uc-excerpt-input mt-3"
          rows="2"
          placeholder="摘要（可选）"
          maxlength="500"
        />
        <StellarContentConfig
          v-model:config="form.stellar_config"
          v-model:blog="form.blog_extensions"
          :version="form.stellar_version"
          profile="post"
          :content="form.content_md"
          class="mt-5"
        />
        <div class="mt-5 min-h-[26rem] overflow-hidden rounded-xl border border-border">
          <MarkdownEditor v-model="form.content_md" :storage-key="`uc-post:${route.params.slug || 'new'}`" />
        </div>
      </section>
      <aside class="h-fit rounded-2xl border border-border bg-card p-4 sm:p-5">
        <h2 class="text-sm font-semibold">文章信息</h2>
        <label class="uc-label mt-5" for="uc-tags">标签</label>
        <input id="uc-tags" v-model="form.tags" class="uc-input" placeholder="用逗号分隔" />
        <p class="mt-5 text-xs leading-5 text-text-tertiary">
          这是受限编辑器。站点设置、分类、专栏、置顶和阅读密码由 Console 管理员维护。
        </p>
      </aside>
    </form>
  </div>
</template>

<style scoped>
.uc-primary-button,
.uc-secondary-button,
.uc-icon-button {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  border-radius: 9999px;
  padding: 0.65rem 1rem;
  font-size: 0.8rem;
  font-weight: 600;
  transition: 150ms var(--ease-standard);
}
.uc-primary-button {
  background: var(--color-primary);
  color: var(--color-primary-foreground);
}
.uc-primary-button:hover {
  background: var(--color-primary-hover);
}
.uc-secondary-button,
.uc-icon-button {
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
}
.uc-secondary-button:hover,
.uc-icon-button:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
.uc-primary-button:disabled,
.uc-secondary-button:disabled {
  cursor: wait;
  opacity: 0.5;
}
.uc-icon-button {
  min-width: 44px;
  padding: 0;
}
.uc-title-input {
  width: 100%;
  background: transparent;
  color: var(--color-foreground);
  font-size: clamp(1.5rem, 4vw, 2.2rem);
  font-weight: 600;
  letter-spacing: -0.03em;
  outline: none;
}
.uc-excerpt-input,
.uc-input {
  width: 100%;
  resize: vertical;
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
  background: var(--color-input);
  padding: 0.7rem 0.8rem;
  color: var(--color-foreground);
  outline: none;
}
.uc-excerpt-input {
  border-color: transparent;
  background: transparent;
  padding-inline: 0;
}
.uc-excerpt-input:focus,
.uc-input:focus {
  border-color: var(--color-ring);
}
.uc-label {
  display: block;
  margin-bottom: 0.45rem;
  font-size: 0.8rem;
  color: var(--color-text-secondary);
}
</style>
