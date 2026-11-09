<script setup lang="ts">
import { useStellarForm } from "@/components/editor/useStellarForm";
import { noteFields } from "@/components/editor/stellar-form-fields";
import { ref, computed, watch, provide, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { notebookApi, getApiErrorMessage, type NotebookNoteResponse } from "@/api";
import ContentEditor from "@/components/ContentEditor.vue";
import StellarContentConfig from "@/components/editor/StellarContentConfig.vue";
import { stellarEditorContextKey } from "@/components/editor/stellar-context";
import { VPageHeader, VButton, VCard, VLoading, Toast } from "@/components/ui";
import NotebookMigrationNotice from "./NotebookMigrationNotice.vue";
import { failureToast, newNoteForm, notePayload, isMigrationError, notebookRoute } from "./notebook-form";
import { confirmNotebookAction, useNotebookDraft } from "./useNotebookDraft";
import "./notebooks.css";
const route = useRoute(),
  router = useRouter();
const notebook = computed(() => String(route.params.notebook || ""));
const id = ref(0),
  title = ref(""),
  bookName = ref(""),
  status = ref("draft"),
  publicPath = ref(""),
  form = ref(newNoteForm(notebook.value));
// 标题与标签和 Front matter 原文双向同步；原文无效时在页首提示，保存仍由服务端严格校验。
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
  publicPath.value = note.public_path;
  form.value = {
    slug: note.slug,
    content: note.content_md,
    tags: note.tags.join("\n"),
    config: note.stellar_config,
    blog: note.blog_extensions,
    // 标题由 useStellarForm 按 config 回填；config 没有 title 时保持为空，前台回退到 Slug。
    title: "",
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
      publicPath.value = "";
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
    Toast.error(failureToast(error.value, migration.value));
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
    Toast.error(error.value);
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
        <a
          v-if="status === 'published' && publicPath"
          class="notebooks-ext-link"
          :href="publicPath"
          target="_blank"
          rel="noopener noreferrer"
          ><Icon icon="ri:external-link-line" aria-hidden="true" />在博客中查看</a
        >
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
      <NotebookMigrationNotice v-if="migration" :detail="error"
        ><VButton v-if="!ready" :loading="loading" @click="load">重试</VButton></NotebookMigrationNotice
      >
      <div v-else-if="error" class="notebooks-error" role="alert">
        {{ error }}<VButton v-if="!ready" class="mt-3" @click="load">重试</VButton>
      </div>
      <VLoading v-if="loading" />
      <div v-else-if="ready">
        <fieldset :disabled="busy" :inert="busy" class="min-w-0 border-0 m-0 p-0">
          <VCard title="基本信息"
            ><div class="notebooks-grid notebooks-grid-form">
              <label class="notebooks-field"
                ><span>标题</span><input v-model="form.title" class="notebooks-control" placeholder="不填时使用 Slug" />
                <p class="notebooks-help">显示在博客的笔记页和列表里。</p></label
              >
              <label class="notebooks-field"
                ><span>Slug</span
                ><input v-model="form.slug" class="notebooks-control" placeholder="例如 first-note 或 工作/记录" />
                <p class="notebooks-help">同一笔记本内唯一；可以使用相对路径，不能包含 . 或 .. 路径段。</p></label
              >
            </div>
            <label class="notebooks-field"
              ><span>标签（每行一个）</span
              ><textarea v-model="form.tags" class="notebooks-control" rows="3" placeholder="工作/记录&#10;随笔" />
              <p class="notebooks-help">层级标签使用 a/b；标签保留为字符串列表。</p></label
            >
          </VCard>
          <p class="notebooks-help">
            Front matter 里 collection.profile 必须为 notebook，collection.id 必须是当前笔记本 ID；预览使用当前配置。
          </p>
          <div class="mt-4 notebooks-editor-shell">
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
          <StellarContentConfig
            class="mt-4"
            v-model:config="form.config"
            v-model:blog="form.blog"
            :version="2"
            profile="note"
          />
        </fieldset>
        <p v-if="status === 'published'" class="notebooks-help">
          保存更新会立即更新公开笔记；需要隐藏时请撤稿，撤稿后保存为草稿。
        </p>
        <p v-else class="notebooks-help">这是草稿，博客前台不会显示；点“发布”后才会公开。</p>
        <div v-if="id" class="notebooks-actions mt-4">
          <VButton type="danger" :disabled="saving" :loading="deleting" @click="remove">删除笔记</VButton>
        </div>
      </div>
    </div>
  </div>
</template>
