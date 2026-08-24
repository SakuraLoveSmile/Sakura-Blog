<script setup lang="ts">
import { ref, computed } from "vue";
import { useQuery } from "@tanstack/vue-query";
import { authApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import { Icon } from "@iconify/vue";
import {
  VPageHeader,
  VCard,
  VButton,
  VEntity,
  VEntityField,
  VEmpty,
  VLoading,
  VPagination,
  FilterDropdown,
  VTag,
} from "@/components/ui";

/** 登录日志条目（Wave 3a） */
interface LoginLogRow {
  id: number;
  user_id: number;
  username: string;
  ip: string;
  ua: string;
  status: string;
  source: string;
  created_at: string;
}

const page = ref(1);
const size = ref(20);
const status = ref<string | undefined>(undefined);

/** 响应式查询参数：page/size/status 变化自动重取 */
const queryParams = computed(() => ({
  page: page.value,
  size: size.value,
  status: status.value,
}));

const {
  data: queryData,
  isLoading,
  isFetching,
  refetch,
} = useQuery({
  queryKey: computed(() => queryKeys.loginLogs(queryParams.value)),
  queryFn: () => authApi.loginLogs(queryParams.value).then(({ data }) => data),
});

const logs = computed<LoginLogRow[]>(() => queryData.value?.items || []);
const total = computed(() => queryData.value?.total || 0);

/** 状态筛选变更：回到第一页 */
function onStatusChange(value: string | undefined) {
  status.value = value;
  page.value = 1;
}

/** 格式化日期时间：2026-07-27T14:27:00Z -> 2026-07-27 14:27 */
function formatDateTime(dateStr: string): string {
  if (!dateStr) return "";
  return dateStr.slice(0, 16).replace("T", "");
}

/** 登录来源中文标签 */
function sourceLabel(source: string): string {
  if (source === "password") return "用户密码";
  if (source === "legacy") return "旧版 admin";
  if (source === "2fa") return "两步验证";
  return source || "-";
}

/** UA 截断展示：过长时取前 80 字符 */
function shortUA(ua: string): string {
  if (!ua) return "-";
  return ua.length > 80 ? ua.slice(0, 80) + "…" : ua;
}
</script>

<template>
  <div>
    <VPageHeader title="登录日志">
      <template #icon>
        <Icon icon="ri:shield-user-line" class="mr-2 self-center text-xl" />
      </template>
    </VPageHeader>

    <div class="m-0 md:m-4">
      <VCard :body-class="['!p-0']">
        <!-- 筛选头部：状态筛选 + 刷新 -->
        <template #header>
          <div class="block w-full bg-secondary px-4 py-3">
            <div class="flex items-center justify-between">
              <FilterDropdown
                label="全部状态"
                :items="[
                  { label: '全部状态', value: undefined },
                  { label: '成功', value: 'success' },
                  { label: '失败', value: 'fail' },
                ]"
                :model-value="status"
                @update:model-value="onStatusChange"
              />
              <div class="group cursor-pointer rounded p-1 hover:bg-accent hover:bg-accent" @click="refetch()">
                <Icon
                  icon="ri:refresh-line"
                  :class="{ 'animate-spin text-foreground ': isFetching }"
                  class="h-4 w-4 text-text-tertiary group-hover:text-foreground group-hover:text-foreground"
                />
              </div>
            </div>
          </div>
        </template>

        <!-- Loading -->
        <VLoading v-if="isLoading" />

        <!-- Empty -->
        <Transition v-else-if="!logs.length" appear name="fade">
          <VEmpty message="登录成功或失败时这里会产生记录" title="暂无登录日志">
            <template #actions>
              <VButton @click="refetch">刷新</VButton>
            </template>
          </VEmpty>
        </Transition>

        <!-- Entity list -->
        <Transition v-else appear name="fade">
          <div class="overflow-x-auto">
            <ul class="box-border h-full w-full min-w-max md:min-w-0 divide-y divide-[var(--color-border)]" role="list">
              <li v-for="log in logs" :key="log.id">
                <VEntity>
                  <template #start>
                    <VEntityField width="12rem">
                      <template #title>
                        <span class="truncate text-sm font-medium text-foreground">
                          {{ log.username || "(未知用户)" }}
                        </span>
                      </template>
                      <template #description>
                        <span class="truncate font-mono text-xs text-text-quaternary">
                          {{ log.ip || "-" }}
                        </span>
                      </template>
                    </VEntityField>
                  </template>
                  <template #end>
                    <!-- 来源 -->
                    <VEntityField>
                      <template #description>
                        <VTag>{{ sourceLabel(log.source) }}</VTag>
                      </template>
                    </VEntityField>

                    <!-- 结果 -->
                    <VEntityField>
                      <template #description>
                        <VTag
                          :class="
                            log.status === 'success'
                              ? 'bg-state-success/10 text-state-success bg-state-success/15 text-state-success'
                              : 'bg-state-error/10 text-state-error bg-state-error/15 text-state-error'
                          "
                        >
                          {{ log.status === "success" ? "成功" : "失败" }}
                        </VTag>
                      </template>
                    </VEntityField>

                    <!-- UA -->
                    <VEntityField width="22rem">
                      <template #description>
                        <span class="block truncate text-xs text-text-quaternary" :title="log.ua">
                          {{ shortUA(log.ua) }}
                        </span>
                      </template>
                    </VEntityField>

                    <!-- 时间 -->
                    <VEntityField>
                      <template #description>
                        <span class="truncate text-xs tabular-nums text-text-tertiary">
                          {{ formatDateTime(log.created_at) }}
                        </span>
                      </template>
                    </VEntityField>
                  </template>
                </VEntity>
              </li>
            </ul>
          </div>
        </Transition>

        <!-- Footer: pagination -->
        <template #footer>
          <div class="bg-card sm:flex sm:items-center sm:justify-end">
            <VPagination v-model:page="page" v-model:size="size" :total="total" :size-options="[20, 50, 100]" />
          </div>
        </template>
      </VCard>
    </div>
  </div>
</template>
