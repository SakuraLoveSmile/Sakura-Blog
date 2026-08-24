<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { ucApi, type UCAttachmentItem } from "@blog/api-client";

const items = ref<UCAttachmentItem[]>([]);
const total = ref(0);
const page = ref(1);
const size = 24;
const loading = ref(false);
const uploading = ref(false);
const url = ref("");
const error = ref("");
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size)));

async function load() {
  loading.value = true;
  try {
    const { data } = await ucApi.attachments.list({ page: page.value, size, kind: "image" });
    items.value = data.items;
    total.value = data.total;
  } catch (err) {
    error.value = err instanceof Error ? err.message : "附件加载失败";
  } finally {
    loading.value = false;
  }
}

async function upload(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  uploading.value = true;
  error.value = "";
  try {
    await ucApi.attachments.upload(file);
    await load();
  } catch (err) {
    error.value = err instanceof Error ? err.message : "上传失败";
  } finally {
    uploading.value = false;
    (event.target as HTMLInputElement).value = "";
  }
}

async function importURL() {
  if (!url.value.trim()) return;
  uploading.value = true;
  error.value = "";
  try {
    await ucApi.attachments.importURL(url.value.trim());
    url.value = "";
    await load();
  } catch (err) {
    error.value = err instanceof Error ? err.message : "导入失败";
  } finally {
    uploading.value = false;
  }
}

async function remove(item: UCAttachmentItem) {
  if (!window.confirm(`删除「${item.filename}」？`)) return;
  await ucApi.attachments.delete(item.id);
  await load();
}

onMounted(load);
</script>

<template>
  <div>
    <div class="mb-7 flex flex-wrap items-end justify-between gap-4">
      <div>
        <p class="mb-2 text-xs font-semibold uppercase tracking-[0.18em] text-primary">My media</p>
        <h1 class="text-3xl font-semibold tracking-tight">我的附件</h1>
        <p class="mt-2 text-sm text-text-tertiary">只显示你上传的图片。</p>
      </div>
      <label class="uc-primary-button cursor-pointer"
        ><Icon icon="ri:upload-2-line" />{{ uploading ? "处理中…" : "上传图片"
        }}<input class="sr-only" type="file" accept="image/*" :disabled="uploading" @change="upload"
      /></label>
    </div>
    <div class="mb-5 flex flex-wrap gap-2 rounded-2xl border border-border bg-card p-3">
      <input
        v-model="url"
        class="uc-input min-w-0 flex-1"
        type="url"
        placeholder="从 URL 导入图片"
        @keyup.enter="importURL"
      />
      <button type="button" class="uc-secondary-button" :disabled="uploading || !url.trim()" @click="importURL">
        导入 URL
      </button>
    </div>
    <p
      v-if="error"
      class="mb-4 rounded-xl border border-state-error/30 bg-state-error/10 px-4 py-3 text-sm text-state-error"
    >
      {{ error }}
    </p>
    <div
      v-if="loading"
      class="rounded-2xl border border-border bg-card px-5 py-12 text-center text-sm text-text-tertiary"
    >
      加载中…
    </div>
    <div v-else-if="!items.length" class="rounded-2xl border border-border bg-card px-5 py-14 text-center">
      <Icon icon="ri:image-line" class="mx-auto h-9 w-9 text-text-quaternary" />
      <p class="mt-3 text-sm text-text-secondary">还没有附件</p>
    </div>
    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      <article
        v-for="item in items"
        :key="item.id"
        class="group overflow-hidden rounded-2xl border border-border bg-card"
      >
        <div class="aspect-square bg-secondary">
          <img
            :src="item.thumb_url || item.url"
            :alt="item.filename"
            class="h-full w-full object-cover"
            loading="lazy"
          />
        </div>
        <div class="p-3">
          <p class="truncate text-xs text-text-secondary" :title="item.filename">{{ item.filename }}</p>
          <button
            type="button"
            class="mt-2 text-xs text-state-error opacity-80 hover:opacity-100"
            @click="remove(item)"
          >
            删除
          </button>
        </div>
      </article>
    </div>
    <div v-if="!loading && totalPages > 1" class="mt-5 flex items-center justify-between text-sm text-text-tertiary">
      <span>共 {{ total }} 项</span>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="uc-secondary-button"
          :disabled="page <= 1"
          @click="
            page--;
            load();
          "
        >
          上一页
        </button>
        <span class="min-w-16 text-center">{{ page }} / {{ totalPages }}</span>
        <button
          type="button"
          class="uc-secondary-button"
          :disabled="page >= totalPages"
          @click="
            page++;
            load();
          "
        >
          下一页
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.uc-primary-button,
.uc-secondary-button {
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
.uc-secondary-button {
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
}
.uc-secondary-button:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
.uc-secondary-button:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}
.uc-input {
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
  background: var(--color-input);
  padding: 0.7rem 0.8rem;
  color: var(--color-foreground);
  outline: none;
}
.uc-input:focus {
  border-color: var(--color-ring);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-ring) 18%, transparent);
}
</style>
