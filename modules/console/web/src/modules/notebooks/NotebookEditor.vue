<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { notebookApi, getApiErrorMessage } from "@/api";
import { VPageHeader, VButton, VCard, VLoading, Toast } from "@/components/ui";
import { newNotebookForm, notebookPayload, isMigrationError, notebookRoute } from "./notebook-form";
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
  baseline = ref("");
const busy = computed(() => saving.value || deleting.value);
const dirty = computed(() => ready.value && JSON.stringify(form.value) !== baseline.value);
useNotebookDraft(dirty, busy);
let request = 0;
async function load() {
  const current = ++request;
  persisted.value = String(route.params.notebook || "");
  ready.value = false;
  error.value = "";
  migration.value = false;
  if (!persisted.value) {
    form.value = newNotebookForm();
    baseline.value = JSON.stringify(form.value);
    ready.value = true;
    return;
  }
  loading.value = true;
  try {
    const { data } = await notebookApi.get(persisted.value);
    if (current !== request) return;
    form.value = { id: data.id, config: data.stellar_config, blog: data.blog_extensions };
    baseline.value = JSON.stringify(form.value);
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
    form.value = { id: data.id, config: data.stellar_config, blog: data.blog_extensions };
    baseline.value = JSON.stringify(form.value);
    persisted.value = data.id;
    Toast.success("笔记本已保存");
    saving.value = false;
    await router.replace(notebookRoute(data.id, "edit"));
  } catch (e) {
    migration.value = isMigrationError(e);
    error.value = getApiErrorMessage(e, "保存笔记本失败");
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
      :subtitle="dirty ? '有未保存的修改' : '集合配置按上游 2.0 规则校验'"
    >
      <template #actions
        ><VButton route="/notebooks" :disabled="busy">返回</VButton
        ><VButton v-if="persisted" :route="notebookRoute(persisted)" :disabled="busy">管理笔记</VButton
        ><VButton type="primary" :disabled="!ready || loading || deleting" :loading="saving" @click="save"
          >保存</VButton
        ></template
      >
    </VPageHeader>
    <div class="notebooks-body">
      <div v-if="error" class="notebooks-error" role="alert">
        <strong v-if="migration">需要迁移到 Stellar 2.0。请先查看迁移预览。<br /></strong>{{ error
        }}<VButton v-if="!ready" class="mt-3" @click="load">重试</VButton>
      </div>
      <VLoading v-if="loading" />
      <form v-else-if="ready" @submit.prevent="save">
        <fieldset :disabled="busy" :inert="busy" class="min-w-0 border-0 p-0 m-0">
          <VCard title="集合身份">
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
              ID 是固定集合身份，保存后不能修改；公开路径由集合的 route 配置决定。名称、描述、图标、封面等在下方集合
              JSON 中设置。
            </p>
          </VCard>
          <div class="notebooks-grid mt-4">
            <VCard title="Stellar 2.0 集合配置"
              ><label class="notebooks-field"
                ><span>完整 JSON 原文</span
                ><textarea
                  v-model="form.config"
                  class="notebooks-control notebooks-json"
                  rows="18"
                  spellcheck="false"
                  required
                />
              </label>
              <p class="notebooks-help">
                使用上游 snake_case 字段；服务端严格验证。修改配置不会自动迁移旧内容。
              </p></VCard
            >
            <VCard title="独立 Blog 扩展"
              ><label class="notebooks-field"
                ><span>扩展 JSON 原文</span
                ><textarea
                  v-model="form.blog"
                  class="notebooks-control notebooks-json"
                  rows="18"
                  spellcheck="false"
                  required
                />
              </label>
              <p class="notebooks-help">
                Blog 扩展单独保存，不放入上游配置结构。原文直接传输，不重编码数字或空值。
              </p></VCard
            >
          </div>
        </fieldset>
        <div v-if="persisted" class="notebooks-actions mt-4">
          <VButton type="danger" :loading="deleting" :disabled="saving" @click="remove">删除空笔记本</VButton>
        </div>
      </form>
    </div>
  </div>
</template>
