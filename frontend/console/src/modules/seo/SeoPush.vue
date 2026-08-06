<script setup lang="ts">
import { onMounted, ref } from"vue";
import { Icon } from"@iconify/vue";
import { VCard, VButton, VEmpty, Toast } from"@/components/ui";
import { seoApi } from"@/api";

const urls = ref("");
const pushing = ref(false);
const logs = ref<any[]>([]);
const total = ref(0);

async function loadLogs() {
 const { data } = await seoApi.pushLogs({ page: 1, size: 50 });
 logs.value = data.items || [];
 total.value = data.total || 0;
}
onMounted(loadLogs);

async function push() {
 const list = urls.value.split("\n").map((s) => s.trim()).filter(Boolean);
 if (!list.length) {
 Toast.error("请输入至少一个 URL（每行一个）");
 return;
 }
 pushing.value = true;
 try {
 const { data } = await seoApi.pushManual(list);
 Toast.success("推送完成", { description: `成功 ${data.success} · 失败 ${data.failed}` });
 urls.value ="";
 await loadLogs();
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"推送失败");
 } finally {
 pushing.value = false;
 }
}

async function clearLogs() {
 await seoApi.clearPushLogs();
 await loadLogs();
 Toast.success("已清理记录");
}
</script>

<template>
 <div class="space-y-4">
 <VCard title="手动推送">
 <div class="space-y-3">
 <textarea
 v-model="urls"
 rows="4"
 class="w-full resize-y rounded-base border border-border px-3 py-2 text-sm outline-none focus:border-primary"
 placeholder="每行一个完整 URL，如 https://example.com/post/hello"
 ></textarea>
 <div class="flex items-center justify-between">
 <p class="text-xs text-text-quaternary">推送引擎与密钥在「设置」中配置；文章发布时也会自动推送。</p>
 <VButton type="secondary" :loading="pushing" @click="push">立即推送</VButton>
 </div>
 </div>
 </VCard>

 <VCard :body-class="['!p-0']">
 <template #header>
 <div class="flex w-full items-center justify-between px-4 py-3">
 <span class="text-sm font-semibold text-foreground">推送记录（{{ total }}）</span>
 <button class="text-xs text-red-500 hover:underline" @click="clearLogs">清理记录</button>
 </div>
 </template>
 <VEmpty v-if="!logs.length" title="暂无推送记录" message="启用推送并发布文章后，这里会显示推送结果" />
 <table v-else class="w-full text-sm">
 <thead class="text-xs text-text-quaternary">
 <tr class="border-b border-border">
 <th class="px-4 py-2 text-left font-normal">引擎</th>
 <th class="px-4 py-2 text-left font-normal">URL</th>
 <th class="px-4 py-2 text-left font-normal">状态</th>
 <th class="px-4 py-2 text-left font-normal">时间</th>
 </tr>
 </thead>
 <tbody>
 <tr v-for="l in logs" :key="l.id" class="border-b border-border /50">
 <td class="px-4 py-2 text-text-tertiary">{{ l.engine }}</td>
 <td class="max-w-xs truncate px-4 py-2 text-text-tertiary">{{ l.url }}</td>
 <td class="px-4 py-2">
 <span :class="l.status === 'success' ? 'text-green-600' : 'text-red-500'">
 <Icon :icon="l.status === 'success' ? 'ri:checkbox-circle-fill' : 'ri:close-circle-fill'" />
 {{ l.status ==="success" ?"成功" :"失败" }}
 </span>
 </td>
 <td class="px-4 py-2 text-xs text-text-quaternary">{{ l.created_at }}</td>
 </tr>
 </tbody>
 </table>
 </VCard>
 </div>
</template>
