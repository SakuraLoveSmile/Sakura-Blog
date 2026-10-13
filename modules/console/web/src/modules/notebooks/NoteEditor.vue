<script setup lang="ts">
import { useStellarForm } from "@/components/editor/useStellarForm";
import { noteFields } from "@/components/editor/stellar-form-fields";
import { ref, computed, watch, provide, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { notebookApi, getApiErrorMessage, type NotebookNoteResponse } from "@/api";
import ContentEditor from "@/components/ContentEditor.vue";
import { stellarEditorContextKey } from "@/components/editor/stellar-context";
import { VPageHeader, VButton, VCard, VLoading, Toast } from "@/components/ui";
import { newNoteForm, notePayload, isMigrationError, notebookRoute } from "./notebook-form";
import { confirmNotebookAction, useNotebookDraft } from "./useNotebookDraft";
import "./notebooks.css";
const route = useRoute(),
  router = useRouter();
const notebook = computed(() => String(route.params.notebook || ""));
const id = ref(0),
  title = ref(""),
  bookName = ref(""),
  status = ref("draft"),
  form = ref(newNoteForm(notebook.value));
const stellarFormError = useStellarForm(() => form.value, noteFields, "config", "blog");
const loading = ref(false),
  saving = ref(false),
  deleting = ref(false),
  ready = ref(false),
  error = ref(""),
  migration = ref(false),
  baseline = ref("");
// 与共享 ContentEditor 的 expose 契约一致，必须先提交富文本的待发送修改。
const editorRef = ref<{ clearDraft: () => void; flushPendingChanges: () => Promise<void> } | null>(null);
const editorPending = ref(false);
const busy = computed(() => saving.value || deleting.value);
const dirty = computed(() => ready.value && (editorPending.value || JSON.stringify(form.value) !== baseline.value));
async function flushEditor() {
  await editorRef.value?.flushPendingChanges();
  await nextTick();
}
useNotebookDraft(dirty, busy, flushEditor);
provide(
  stellarEditorContextKey,
  computed(() => ({ stellar_version: 2, stellar_config: form.value.config, blog_extensions: form.value.blog })),
);
let request = 0;
function accept(note: NotebookNoteResponse) {
  id.value = note.id;
  title.value = note.title;
  status.value = note.status;
  form.value = {
    slug: note.slug,
    content: note.content_md,
    tags: note.tags.join("\n"),
    config: note.stellar_config,
    blog: note.blog_extensions,
  };
  baseline.value = JSON.stringify(form.value);
}
async function load() {
  const current = ++request;
  ready.value = false;
  editorPending.value = false;
  loading.value = true;
  error.value = "";
  migration.value = false;
  id.value = Number(route.params.id || 0);
  try {
    const { data: book } = await notebookApi.get(notebook.value);
    if (current !== request) return;
    bookName.value = book.name;
    if (id.value) {
      const { data } = await notebookApi.getNote(notebook.value, id.value);
      if (current !== request) return;
      accept(data);
    } else {
      form.value = newNoteForm(notebook.value);
      status.value = "draft";
      title.value = "";
      baseline.value = JSON.stringify(form.value);
    }
    ready.value = true;
  } catch (e) {
    if (current !== request) return;
    migration.value = isMigrationError(e);
    error.value = getApiErrorMessage(e, "加载笔记失败");
  } finally {
    if (current === request) loading.value = false;
  }
}
async function save(action: "save" | "publish" | "withdraw" = "save") {
  if (!ready.value || busy.value) return;
  if (
    action === "withdraw" &&
    !(await confirmNotebookAction("撤回已发布笔记？", "当前修改将保存为草稿，公开页面将不再展示这篇笔记。"))
  )
    return;
  saving.value = true;
  error.value = "";
  try {
    // 共享编辑器在保存前提交尚未发出的富文本修改。
    await flushEditor();
    const payload = notePayload(notebook.value, form.value);
    let result;
    if (!id.value)
      result = await notebookApi.createNote(notebook.value, {
        ...payload,
        status: action === "publish" ? "published" : "draft",
      });
    else if (action === "publish") result = await notebookApi.publishNote(notebook.value, id.value, payload);
    else if (action === "withdraw") result = await notebookApi.saveNoteDraft(notebook.value, id.value, payload);
    else result = await notebookApi.updateNote(notebook.value, id.value, payload);
    accept(result.data);
    editorRef.value?.clearDraft();
    Toast.success(
      action === "publish" ? "笔记已发布" : action === "withdraw" ? "笔记已撤回并保存为草稿" : "笔记已保存",
    );
    saving.value = false;
    await router.replace(notebookRoute(notebook.value, `notes/${id.value}/edit`));
  } catch (e) {
    migration.value = isMigrationError(e);
    error.value = getApiErrorMessage(e, "保存笔记失败");
  } finally {
    saving.value = false;
  }
}
async function remove() {
  if (!id.value || busy.value || !(await confirmNotebookAction("删除笔记？", "笔记正文和配置将删除，此操作不能撤销。")))
    return;
  deleting.value = true;
  error.value = "";
  try {
    await notebookApi.deleteNote(notebook.value, id.value);
    ready.value = false;
    baseline.value = JSON.stringify(form.value);
    editorRef.value?.clearDraft();
    Toast.success("笔记已删除");
    deleting.value = false;
    await router.push(notebookRoute(notebook.value));
  } catch (e) {
    error.value = getApiErrorMessage(e, "删除笔记失败");
  } finally {
    deleting.value = false;
  }
}
watch(() => [route.params.notebook, route.params.id], load, { immediate: true });
</script>
<template>
  <div>
    <VPageHeader
      :title="id ? title || '编辑笔记' : '新建笔记'"
      :subtitle="`${bookName || notebook} · ${status === 'published' ? '已发布' : '草稿'}${dirty ? ' · 有未保存的修改' : ''}`"
      actions-layout="stack-below-lg"
    >
      <template #actions>
        <VButton :route="notebookRoute(notebook)" :disabled="busy">返回笔记列表</VButton>
        <VButton :disabled="!ready || loading || deleting" :loading="saving" @click="save()">{{
          status === "published" ? "保存更新" : "保存草稿"
        }}</VButton>
        <VButton type="primary" :disabled="!ready || loading || busy" @click="save('publish')">{{
          status === "published" ? "更新并发布" : "发布"
        }}</VButton>
        <VButton v-if="id && status === 'published'" :disabled="busy || !ready" @click="save('withdraw')">撤稿</VButton>
      </template>
    </VPageHeader>
    <div class="notebooks-body">
      <p v-if="stellarFormError" role="alert" class="notebooks-error">{{ stellarFormError }}</p>
      <div v-if="error" class="notebooks-error" role="alert">
        <strong v-if="migration">需要迁移到 Stellar 2.0。先查看迁移预览。<br /></strong>{{ error
        }}<VButton v-if="!ready" class="mt-3" @click="load">重试</VButton>
      </div>
      <VLoading v-if="loading" />
      <div v-else-if="ready">
        <fieldset :disabled="busy" :inert="busy" class="min-w-0 border-0 m-0 p-0">
          <VCard title="笔记身份与标签"
            ><div class="notebooks-grid">
              <label class="notebooks-field"
                ><span>Slug</span
                ><input v-model="form.slug" class="notebooks-control" placeholder="例如 first-note 或 工作/记录" />
                <p class="notebooks-help">同一笔记本内唯一；可以使用相对路径，不能包含 . 或 .. 路径段。</p></label
              >
              <label class="notebooks-field"
                ><span>标签（每行一个）</span
                ><textarea v-model="form.tags" class="notebooks-control" rows="3" placeholder="工作/记录&#10;随笔" />
                <p class="notebooks-help">层级标签使用 a/b；标签保留为字符串列表。</p></label
              >
            </div></VCard
          >
          <div class="notebooks-grid mt-4">
            <VCard title="Stellar 2.0 Front matter"
              ><label class="notebooks-field"
                ><span>完整 JSON 原文</span
                ><textarea
                  v-model="form.config"
                  class="notebooks-control notebooks-json"
                  rows="12"
                  spellcheck="false"
                />
              </label>
              <p class="notebooks-help">
                标题与表现字段从这里读取。collection.profile 必须为 notebook，collection.id 必须为当前笔记本
                ID。预览使用当前配置候选。
              </p></VCard
            >
            <VCard title="独立 Blog 扩展"
              ><label class="notebooks-field"
                ><span>扩展 JSON 原文</span
                ><textarea v-model="form.blog" class="notebooks-control notebooks-json" rows="12" spellcheck="false" />
              </label>
              <p class="notebooks-help">原文独立保存，保留全部扩展参数、数字与 null。</p></VCard
            >
          </div>
          <div class="mt-4">
            <ContentEditor
              ref="editorRef"
              :key="`${notebook}:${id || 'new'}`"
              v-model="form.content"
              :storage-key="`notebook:${notebook}:note:${id || 'new'}`"
              placeholder="开始写作…"
              @save="save()"
              @pending-change="editorPending = $event"
            />
          </div>
        </fieldset>
        <p v-if="status === 'published'" class="notebooks-help">
          保存更新会立即更新公开笔记；需要隐藏时请撤稿，撤稿后保存为草稿。
        </p>
        <div v-if="id" class="notebooks-actions mt-4">
          <VButton type="danger" :disabled="saving" :loading="deleting" @click="remove">删除笔记</VButton>
        </div>
      </div>
    </div>
  </div>
</template>
