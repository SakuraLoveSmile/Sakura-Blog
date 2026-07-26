<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { pageApi } from "@/api";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, VCard, Toast } from "@/components/ui";

const route = useRoute();
const router = useRouter();

const slugParam = computed(() => route.params.slug as string);
const isNew = computed(() => slugParam.value === "new");

const form = ref({ slug: "", title: "", content_md: "", status: "draft" });
const saving = ref(false);
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);

const inputCls =
  "w-full border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500";

onMounted(async () => {
  if (isNew.value) return;
  try {
    const { data } = await pageApi.get(slugParam.value);
    form.value = { slug: data.slug, title: data.title, content_md: data.content_md, status: data.status };
  } catch {
    Toast.error("加载失败");
  }
});

async function handleSave() {
  saving.value = true;
  try {
    if (isNew.value) {
      await pageApi.create(form.value);
    } else {
      await pageApi.update(slugParam.value, form.value);
    }
    editorRef.value?.clearDraft();
    Toast.success("保存成功");
    router.push("/pages");
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "保存失败");
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div>
    <VPageHeader :title="isNew ? '新建页面' : '编辑页面'">
      <template #icon>
        <Icon icon="ri:pages-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <div class="flex items-center gap-2">
          <VButton @click="router.push('/pages')">取消</VButton>
          <VButton type="secondary" :loading="saving" @click="handleSave">保存</VButton>
        </div>
      </template>
    </VPageHeader>

    <div class="m-0 max-w-5xl space-y-4 md:m-4">
      <VCard>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">标题</label>
            <input v-model="form.title" :class="inputCls" style="border-radius: 4px" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">Slug</label>
            <input v-model="form.slug" :class="inputCls" style="border-radius: 4px" />
          </div>
        </div>
        <div class="mt-4">
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">状态</label>
          <select
            v-model="form.status"
            class="w-40 border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            style="border-radius: 4px"
          >
            <option value="draft">草稿</option>
            <option value="published">发布</option>
          </select>
        </div>
      </VCard>

      <VCard>
        <template #header>
          <span class="text-base font-bold text-gray-900 dark:text-gray-100">内容（Markdown）</span>
        </template>
        <MarkdownEditor
          ref="editorRef"
          v-model="form.content_md"
          :storage-key="'page:' + (isNew ? 'new' : slugParam)"
          @save="handleSave"
        />
      </VCard>
    </div>
  </div>
</template>
