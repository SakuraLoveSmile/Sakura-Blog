<script setup lang="ts">
import { computed } from "vue";
import { useMutation, useQueryClient } from "@tanstack/vue-query";
import { usePagedList } from "@/composables/usePagedList";
import { shareApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VButton, VEmpty, VLoading, VPagination, Dialog, Toast } from "@/components/ui";
import { formatRelative } from "@/lib/format";

/** 分享链接响应模型 */
interface ShareRow {
  id: number;
  token: string;
  post_slug: string;
  post_title: string;
  share_type: string;
  expires_at: string;
  created_at: string;
  url: string;
}

const queryClient = useQueryClient();

const {
  page,
  size,
  items: rawShares,
  total,
  isLoading,
  refetch,
} = usePagedList<ShareRow>({
  apiFn: (params: Record<string, unknown>) => shareApi.list(params),
  keyFn: (params: Record<string, unknown>) => queryKeys.shares(params),
});

const shares = computed<ShareRow[]>(() => rawShares.value);

// 统计数据
const stats = computed(() => {
  const all = shares.value;
  const now = new Date().getTime();
  let activeCount = 0;
  let expiredCount = 0;
  for (const s of all) {
    if (s.expires_at && new Date(s.expires_at).getTime() < now) expiredCount++;
    else activeCount++;
  }
  return {
    total: total.value,
    activeCount,
    expiredCount,
  };
});

const deleteMutation = useMutation({
  mutationFn: (id: number) => shareApi.delete(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.shares() });
    Toast.success("分享链接已删除并失效");
  },
});

function handleDelete(share: ShareRow) {
  Dialog.warning({
    title: "确定删除该分享链接吗？",
    description: `删除后该临时直链将立即失效，外部访客将无法再通过此链接访问文章。`,
    confirmType: "danger",
    onConfirm: () => deleteMutation.mutate(share.id),
  });
}

function buildFullUrl(url: string): string {
  return window.location.origin + url;
}

async function handleCopy(url: string) {
  const fullUrl = buildFullUrl(url);
  try {
    await navigator.clipboard.writeText(fullUrl);
    Toast.success("分享链接已复制到剪贴板");
  } catch {
    Toast.error("复制失败，请手动复制");
  }
}

function isExpired(expiresAt: string): boolean {
  if (!expiresAt) return false;
  return new Date(expiresAt).getTime() < new Date().getTime();
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="文章分享链接">
      <template #icon>
        <Icon icon="ri:share-forward-line" class="text-primary" />
      </template>
      <template #subtitle>管理已生成的未发布文章临时预览或公开分享令牌与访问有效期</template>
      <template #actions>
        <VButton :route="{ path: '/posts' }">
          <template #icon><Icon icon="ri:article-line" /></template>
          返回文章列表
        </VButton>
        <VButton @click="refetch()">
          <template #icon><Icon icon="ri:refresh-line" /></template>
          刷新
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <!-- 概览数据指标卡 -->
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
        <div class="stats-card">
          <div class="stats-icon text-primary bg-primary/10">
            <Icon icon="ri:share-line" />
          </div>
          <div>
            <div class="stats-label">全部生成链接</div>
            <div class="stats-value">{{ stats.total }} <span class="stats-sub">条分享记录</span></div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
            <Icon icon="ri:checkbox-circle-line" />
          </div>
          <div>
            <div class="stats-label">正常有效链接</div>
            <div class="stats-value text-[#30D158]">
              {{ stats.activeCount }} <span class="stats-sub">仍可正常访问</span>
            </div>
          </div>
        </div>

        <div class="stats-card">
          <div class="stats-icon text-[#FF453A] bg-[#FF453A]/10">
            <Icon icon="ri:timer-line" />
          </div>
          <div>
            <div class="stats-label">已过期失效</div>
            <div class="stats-value text-[#FF453A]">
              {{ stats.expiredCount }} <span class="stats-sub">已过有效时限</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 分享链接表格卡片 -->
      <VCard>
        <VLoading v-if="isLoading" />
        <VEmpty
          v-else-if="!shares.length"
          title="暂无任何分享链接"
          message="可在「文章管理」列表中针对指定文章点击「生成分享链接」"
        />

        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[760px] text-left">
            <thead>
              <tr class="border-b border-border text-xs font-medium uppercase tracking-wide text-text-quaternary">
                <th class="px-4 py-3">关联文章</th>
                <th class="px-4 py-3">分享类型</th>
                <th class="px-4 py-3">分享直链 / 令牌</th>
                <th class="px-4 py-3">有效期状态</th>
                <th class="px-4 py-3">生成时间</th>
                <th class="w-32 px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border/60">
              <tr v-for="s in shares" :key="s.id" class="transition-colors hover:bg-secondary/40">
                <td class="px-4 py-3.5">
                  <div class="font-semibold text-foreground text-sm truncate max-w-[280px]">
                    {{ s.post_title || s.post_slug }}
                  </div>
                  <div class="text-xs font-mono text-text-quaternary">/{{ s.post_slug }}</div>
                </td>
                <td class="px-4 py-3.5 text-xs">
                  <span
                    class="rounded-full px-2 py-0.5 text-[10px] font-medium"
                    :class="
                      s.share_type === 'draft' ? 'bg-[#FF9500]/10 text-[#FF9500]' : 'bg-[#30D158]/10 text-[#30D158]'
                    "
                  >
                    {{ s.share_type === "draft" ? "草稿预览" : "公开发布" }}
                  </span>
                </td>
                <td class="px-4 py-3.5 font-mono text-xs text-text-secondary">
                  <div class="flex items-center gap-1.5">
                    <span class="truncate max-w-[220px]" :title="buildFullUrl(s.url)">{{ s.url }}</span>
                    <button
                      type="button"
                      class="text-primary hover:underline p-1 text-xs"
                      title="复制完整链接"
                      @click="handleCopy(s.url)"
                    >
                      <Icon icon="ri:file-copy-line" />
                    </button>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs">
                  <span
                    v-if="isExpired(s.expires_at)"
                    class="rounded px-2 py-0.5 bg-[#FF453A]/10 text-[#FF453A] font-medium"
                  >
                    已过期 ({{ formatRelative(s.expires_at) }})
                  </span>
                  <span v-else-if="s.expires_at" class="text-text-tertiary">
                    {{ formatRelative(s.expires_at) }} 到期
                  </span>
                  <span v-else class="text-[#30D158]">永久有效</span>
                </td>
                <td class="px-4 py-3.5 text-xs text-text-quaternary">
                  {{ formatRelative(s.created_at) }}
                </td>
                <td class="px-4 py-3.5 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <button
                      type="button"
                      class="action-icon-btn text-primary"
                      title="复制分享链接"
                      @click="handleCopy(s.url)"
                    >
                      <Icon icon="ri:link" />
                    </button>
                    <button
                      type="button"
                      class="action-icon-btn text-state-error"
                      title="删除链接"
                      @click="handleDelete(s)"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 分页 -->
        <div v-if="total > size" class="flex justify-center pt-4">
          <VPagination
            :page="page"
            :size="size"
            :total="total"
            @update:page="page = $event"
            @update:size="size = $event"
          />
        </div>
      </VCard>
    </div>
  </div>
</template>

<style scoped>
/* 统计卡片 */
.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-xs);
}
.stats-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 8%, transparent), transparent);
  pointer-events: none;
}
.stats-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  font-size: 20px;
  flex-shrink: 0;
}
.stats-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-quaternary);
  text-transform: uppercase;
  letter-spacing: 0.2px;
}
.stats-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.2;
}
.stats-sub {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-tertiary);
  margin-top: 1px;
}

/* 操作小按钮 */
.action-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  color: var(--color-text-tertiary);
  background: transparent;
  border: 1px solid transparent;
  transition: all var(--duration-fast) var(--ease-standard);
  cursor: pointer;
}
.action-icon-btn:hover {
  background: var(--color-secondary);
  color: var(--color-foreground);
  border-color: var(--color-border);
}

@media (max-width: 640px) {
  .stats-card {
    padding: 10px 12px;
    gap: 8px;
  }
  .stats-icon {
    width: 32px;
    height: 32px;
    font-size: 16px;
  }
  .stats-value {
    font-size: 14px;
  }
}
</style>
