<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { postApi } from "@/api";

const route = useRoute();
const router = useRouter();

const isEdit = computed(() => !!route.params.slug);
const slug = computed(() => route.params.slug as string);

const form = ref({
  slug: "",
  title: "",
  excerpt: "",
  cover: "",
  category: "",
  topic: "",
  tags: "",
  content_md: "",
  status: "draft",
  pinned: false,
  password: "",
  publish_at: "",
});

const loading = ref(false);
const saving = ref(false);
const error = ref("");

onMounted(async () => {
  if (!isEdit.value) return;
  loading.value = true;
  try {
    const { data } = await postApi.get(slug.value);
    form.value = {
      slug: data.slug,
      title: data.title,
      excerpt: data.excerpt,
      cover: data.cover,
      category: data.category,
      topic: data.topic,
      tags: data.tags,
      content_md: data.content_md,
      status: data.status,
      pinned: data.pinned,
      password: data.password,
      publish_at: data.publish_at || "",
    };
  } catch (e: any) {
    error.value = "加载失败";
  } finally {
    loading.value = false;
  }
});

async function handleSave() {
  saving.value = true;
  error.value = "";
  try {
    if (isEdit.value) {
      await postApi.update(slug.value, form.value);
    } else {
      await postApi.create(form.value);
    }
    router.push("/posts");
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
      <h2 class="text-xl font-semibold text-gray-800 dark:text-gray-100">
        {{ isEdit ? "编辑文章" : "新建文章" }}
      </h2>
      <div class="flex gap-2">
        <router-link to="/posts" class="px-4 py-2 text-sm text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200">取消</router-link>
        <button
          @click="handleSave"
          :disabled="saving"
          class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50"
        >
          {{ saving ? "保存中..." : "保存" }}
        </button>
      </div>
    </div>

    <p v-if="error" class="mb-4 text-sm text-red-600 bg-red-50 px-3 py-2 rounded-lg dark:text-red-400 dark:bg-red-500/10">{{ error }}</p>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Editor main -->
      <div class="lg:col-span-2 space-y-4">
        <input
          v-model="form.title"
          placeholder="文章标题"
          class="w-full px-4 py-3 text-lg font-medium border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"
        />
        <textarea
          v-model="form.content_md"
          placeholder="Markdown 内容..."
          rows="24"
          class="w-full px-4 py-3 text-sm font-mono border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500 resize-y dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"
        ></textarea>
      </div>

      <!-- Settings sidebar -->
      <div class="space-y-4">
        <div class="bg-white rounded-xl border border-gray-200 p-4 space-y-3 dark:bg-gray-800 dark:border-gray-700">
          <h3 class="text-sm font-medium text-gray-700 mb-2 dark:text-gray-300">发布设置</h3>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">Slug</label>
            <input v-model="form.slug" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">状态</label>
            <select v-model="form.status" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100">
              <option value="draft">草稿</option>
              <option value="published">发布</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <input type="checkbox" v-model="form.pinned" id="pinned" class="rounded" />
            <label for="pinned" class="text-sm text-gray-600 dark:text-gray-400">置顶</label>
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">定时发布</label>
            <input v-model="form.publish_at" type="datetime-local" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100" />
          </div>
        </div>

        <div class="bg-white rounded-xl border border-gray-200 p-4 space-y-3 dark:bg-gray-800 dark:border-gray-700">
          <h3 class="text-sm font-medium text-gray-700 mb-2 dark:text-gray-300">分类与标签</h3>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">分类</label>
            <input v-model="form.category" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">专栏</label>
            <input v-model="form.topic" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">标签（逗号分隔）</label>
            <input v-model="form.tags" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
        </div>

        <div class="bg-white rounded-xl border border-gray-200 p-4 space-y-3 dark:bg-gray-800 dark:border-gray-700">
          <h3 class="text-sm font-medium text-gray-700 mb-2 dark:text-gray-300">其他</h3>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">摘要</label>
            <textarea v-model="form.excerpt" rows="3" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg resize-y focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"></textarea>
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">封面 URL</label>
            <input v-model="form.cover" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1 dark:text-gray-400">阅读密码</label>
            <input v-model="form.password" type="password" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
