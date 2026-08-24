<script setup lang="ts">
import { onMounted, ref } from "vue";
import { VCard, VButton, VSwitch, Toast } from "@/components/ui";
import { seoApi } from "@/api";

const loading = ref(true);
const saving = ref(false);
const cfg = ref<any>({
  push: { enabled: false, indexnow_key: "", baidu_token: "", engines: [] },
  crawler: { enabled: true },
  traffic: { enabled: true },
});

const inputCls =
  "w-full rounded-base border border-border px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-border-strong dark:bg-secondary dark:text-foreground dark:placeholder-gray-500";

async function load() {
  loading.value = true;
  try {
    const { data } = await seoApi.getSettings();
    cfg.value = { ...cfg.value, ...data };
    if (!Array.isArray(cfg.value.push.engines)) cfg.value.push.engines = [];
  } finally {
    loading.value = false;
  }
}
onMounted(load);

function toggleEngine(name: string, on: boolean) {
  const arr: string[] = cfg.value.push.engines;
  const idx = arr.indexOf(name);
  if (on && idx === -1) arr.push(name);
  if (!on && idx !== -1) arr.splice(idx, 1);
}

async function save() {
  saving.value = true;
  try {
    await seoApi.saveSettings(cfg.value);
    Toast.success("保存成功", { description: "SEO 设置已更新" });
  } catch {
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div v-if="!loading" class="space-y-4">
    <!-- 功能开关 -->
    <VCard title="功能开关">
      <div class="space-y-4">
        <label class="flex items-center justify-between">
          <span class="text-sm text-text-quaternary dark:text-foreground"
            >站点推送（文章发布时自动推送到搜索引擎）</span
          >
          <VSwitch v-model="cfg.push.enabled" />
        </label>
        <label class="flex items-center justify-between">
          <span class="text-sm text-text-quaternary dark:text-foreground">爬虫记录（记录搜索引擎爬虫访问）</span>
          <VSwitch v-model="cfg.crawler.enabled" />
        </label>
        <label class="flex items-center justify-between">
          <span class="text-sm text-text-quaternary dark:text-foreground">搜索流量（统计搜索引擎来源访问）</span>
          <VSwitch v-model="cfg.traffic.enabled" />
        </label>
      </div>
    </VCard>

    <!-- 推送引擎 -->
    <VCard title="推送引擎">
      <div class="space-y-4">
        <div class="flex gap-4">
          <label class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              :checked="cfg.push.engines.includes('indexnow')"
              @change="(e: any) => toggleEngine('indexnow', e.target.checked)"
            />
            IndexNow（Bing/Yandex 等）
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              :checked="cfg.push.engines.includes('baidu')"
              @change="(e: any) => toggleEngine('baidu', e.target.checked)"
            />
            百度
          </label>
        </div>
        <div>
          <label class="mb-1 block text-xs text-text-quaternary dark:text-muted-foreground">IndexNow Key</label>
          <input v-model="cfg.push.indexnow_key" :class="inputCls" placeholder="留空表示不修改已保存的 Key" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-text-quaternary dark:text-muted-foreground">百度推送 Token</label>
          <input
            v-model="cfg.push.baidu_token"
            :class="inputCls"
            placeholder="百度站长平台 → 普通收录 → 推送接口 token"
          />
        </div>
      </div>
    </VCard>

    <!-- 搜索引擎验证已统一到站点级 SEO 设置 -->
    <VCard title="搜索引擎验证">
      <p class="text-sm text-text-quaternary dark:text-muted-foreground">
        站长验证码已统一到「设置 → SEO」页面配置（Google / Bing / 百度），避免两处重复填写互相覆盖。
      </p>
    </VCard>

    <div class="flex justify-end">
      <VButton type="secondary" :loading="saving" @click="save">保存设置</VButton>
    </div>
  </div>
</template>
