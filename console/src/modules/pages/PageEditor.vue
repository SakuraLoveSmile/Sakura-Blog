<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { pageApi } from "@/api";

const route = useRoute();
const router = useRouter();

const slugParam = computed(() => route.params.slug as string);
const isNew = computed(() => slugParam.value === "new");

const form = ref({ slug: "", title: "", content_md: "", status: "draft" });
const saving = ref(false);
const error = ref("");

onMounted(async () => {
  if (isNew.value) return;
  try {
    const { data } = await pageApi.get(slugParam.value);
    form.value = { slug: data.slug, title: data.title, content_md: data.content_md, status: data.status };
  } catch {
    error.value = "加载失败";
  }
});

async function handleSave() {
  saving.value = true;
  error.value = "";
  try {
    if (isNew.value) {
      await pageApi.create(form.value);
    } else {
      await pageApi.update(slugParam.value, form.value);
    }
    router.push("/pages");
  } catch (e: any) {
    error.value = e.response?.data?.error || "保存失败";
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <h2 class="text-xl font-semibold text-gray-800">{{ isNew ? "新建页面" : "编辑页面" }}</h2>
      <div class="flex gap-2">
        <router-link to="/pages" class="px-4 py-2 text-sm text-gray-600 hover:text-gray-800">取消</router-link>
        <button @click="handleSave" :disabled="saving"
          class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50">
          {{ saving ? "保存中..." : "保存" }}
        </button>
      </div>
    </div>

    <p v-if="error" class="mb-4 text-sm text-red-600 bg-red-50 px-3 py-2 rounded-lg">{{ error }}</p>

    <div class="max-w-3xl space-y-4">
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs text-gray-500 mb-1">标题</label>
          <input v-model="form.title" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
        <div>
          <label class="block text-xs text-gray-500 mb-1">Slug</label>
          <input v-model="form.slug" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500" />
        </div>
      </div>
      <div>
        <label class="block text-xs text-gray-500 mb-1">状态</label>
        <select v-model="form.status" class="w-40 px-3 py-2 text-sm border border-gray-200 rounded-lg">
          <option value="draft">草稿</option>
          <option value="published">发布</option>
        </select>
      </div>
      <div>
        <label class="block text-xs text-gray-500 mb-1">内容（Markdown）</label>
        <textarea v-model="form.content_md" rows="20"
          class="w-full px-4 py-3 text-sm font-mono border border-gray-200 rounded-xl resize-y focus:outline-none focus:ring-2 focus:ring-indigo-500"></textarea>
      </div>
    </div>
  </div>
</template>
