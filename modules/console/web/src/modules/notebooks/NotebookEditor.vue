<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { notebookApi, getApiErrorMessage } from "@/api";
import AttachmentField from "@/components/AttachmentField.vue";
import IconPicker from "@/components/IconPicker.vue";
import StellarContentConfig from "@/components/editor/StellarContentConfig.vue";
import { notebookFields } from "@/components/editor/stellar-form-fields";
import { useStellarForm } from "@/components/editor/useStellarForm";
import { VPageHeader, VButton, VCard, VLoading, Toast } from "@/components/ui";
import NotebookMigrationNotice from "./NotebookMigrationNotice.vue";
import {
  failureToast,
  newNotebookForm,
  notebookFormFrom,
  notebookPayload,
  isMigrationError,
  notebookRoute,
} from "./notebook-form";
import { confirmNotebookAction, useNotebookDraft } from "./useNotebookDraft";
import "./notebooks.css";
const route = useRoute(),
  router = useRouter();
const persisted = ref(""),
  form = ref(newNotebookForm()),
  loading = ref(false),
  saving = ref(false),
  deleting = ref(false),
  ready = ref(false);
const error = ref(""),
  migration = ref(false),
  baseline = ref(""),
  publicPath = ref(""),
  publishedCount = ref(0);
// 名称、描述等基本字段与 JSON 原文双向同步；原文无效时在页首提示，保存仍由服务端严格校验。
const stellarFormError = useStellarForm(() => form.value, notebookFields, "config", "blog");
const busy = computed(() => saving.value || deleting.value);
const dirty = computed(() => ready.value && JSON.stringify(form.value) !== baseline.value);
useNotebookDraft(dirty, busy);
let request = 0;
function accept(data: Parameters<typeof notebookFormFrom>[0] & { public_path: string; published_count: number }) {
  form.value = notebookFormFrom(data);
  publicPath.value = data.public_path;
  publishedCount.value = data.published_count;
  baseline.value = JSON.stringify(form.value);
}
async function load() {
  const current = ++request;
  persisted.value = String(route.params.notebook || "");
  ready.value = false;
  error.value = "";
  migration.value = false;
  if (!persisted.value) {
    form.value = newNotebookForm();
    publicPath.value = "";
    publishedCount.value = 0;
    baseline.value = JSON.stringify(form.value);
    ready.value = true;
    return;
  }
  loading.value = true;
  try {
    const { data } = await notebookApi.get(persisted.value);
    if (current !== request) return;
    accept(data);
    ready.value = true;
  } catch (e) {
    if (current !== request) return;
    migration.value = isMigrationError(e);
    error.value = getApiErrorMessage(e, "加载笔记本失败");
  } finally {
    if (current === request) loading.value = false;
  }
}
async function save() {
  if (!ready.value || busy.value) return;
  error.value = "";
  saving.value = true;
  try {
    const payload = notebookPayload(form.value);
    const { data } = persisted.value
      ? await notebookApi.update(persisted.value, payload)
      : await notebookApi.create(payload);
    accept(data);
    persisted.value = data.id;
    Toast.success("笔记本已保存");
    saving.value = false;
    await router.replace(notebookRoute(data.id, "edit"));
  } catch (e) {
    migration.value = isMigrationError(e);
    error.value = getApiErrorMessage(e, "保存笔记本失败");
    Toast.error(failureToast(error.value, migration.value));
  } finally {
    saving.value = false;
  }
}
async function remove() {
  if (
    !persisted.value ||
    busy.value ||
    !(await confirmNotebookAction("删除笔记本？", "仅空笔记本可以删除。此操作不会级联删除笔记。"))
  )
    return;
  deleting.value = true;
  error.value = "";
  try {
    await notebookApi.delete(persisted.value);
    ready.value = false;
    baseline.value = JSON.stringify(form.value);
    Toast.success("笔记本已删除");
    deleting.value = false;
    await router.push("/notebooks");
  } catch (e) {
    error.value = getApiErrorMessage(e, "删除失败，请先删除本笔记本内的笔记");
    Toast.error(error.value);
  } finally {
    deleting.value = false;
  }
}
watch(() => route.params.notebook, load, { immediate: true });
</script>
<template>
  <div>
    <VPageHeader
      :title="persisted ? '编辑笔记本' : '新建笔记本'"
      :subtitle="dirty ? '有未保存的修改' : '设置笔记本的名称、外观和在博客里的显示方式'"
    >
      <template #actions
        ><a
          v-if="publicPath && publishedCount"
          class="notebooks-ext-link"
          :href="publicPath"
          target="_blank"
          rel="noopener noreferrer"
          ><Icon icon="ri:external-link-line" aria-hidden="true" />在博客中查看</a
        ><VButton route="/notebooks" :disabled="busy">返回</VButton
        ><VButton v-if="persisted" :route="notebookRoute(persisted)" :disabled="busy">管理笔记</VButton
        ><VButton type="primary" :disabled="!ready || loading || deleting" :loading="saving" @click="save"
          >保存</VButton
        ></template
      >
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
      <form v-else-if="ready" @submit.prevent="save">
        <fieldset :disabled="busy" :inert="busy" class="min-w-0 border-0 p-0 m-0">
          <VCard title="基本信息">
            <label class="notebooks-field"
              ><span>笔记本 ID</span
              ><input
                v-model="form.id"
                class="notebooks-control"
                :disabled="!!persisted"
                placeholder="例如 journal"
                required
            /></label>
            <p class="notebooks-help">
              ID 是笔记本的固定标识，保存后不能修改；博客里的默认地址是 /notebooks/ID，也可以在高级配置里用 route.path
              改。
            </p>
            <label class="notebooks-field"
              ><span>名称</span><input v-model="form.name" class="notebooks-control" placeholder="例如：工作笔记"
            /></label>
            <label class="notebooks-field"
              ><span>描述</span
              ><textarea
                v-model="form.description"
                class="notebooks-control"
                rows="3"
                placeholder="显示在博客的笔记本卡片和首页上"
              />
            </label>
            <label class="notebooks-field"
              ><span>排序</span><input v-model.number="form.sort" class="notebooks-control" type="number" step="1" />
              <p class="notebooks-help">控制笔记本在博客总索引里的顺序，数字小的排在前面。</p></label
            >
          </VCard>
          <VCard title="外观" class="mt-4">
            <div class="notebooks-field notebooks-shared">
              <span>图标</span>
              <IconPicker v-model="form.icon" placeholder="选择 icons.yml 图标或图片 URL" />
            </div>
            <div class="notebooks-field notebooks-shared">
              <span>封面</span>
              <AttachmentField v-model="form.cover" placeholder="输入图片 URL 或从附件库选择" show-thumb />
            </div>
          </VCard>
          <StellarContentConfig
            class="mt-4"
            v-model:config="form.config"
            v-model:blog="form.blog"
            :version="2"
            profile="notebook"
            collection
          />
        </fieldset>
        <div v-if="persisted" class="notebooks-actions mt-4">
          <VButton type="danger" :loading="deleting" :disabled="saving" @click="remove">删除空笔记本</VButton>
        </div>
      </form>
    </div>
  </div>
</template>
