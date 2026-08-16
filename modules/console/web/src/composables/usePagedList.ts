import { computed, ref, watch } from "vue";
import { useQuery } from "@tanstack/vue-query";

/**
 * Spec 62：列表页分页/搜索/查询样板（usePagedList）。
 * 统一 8 个分页页手写的同一套样板：page/size、300ms 关键字防抖、
 * queryParams computed、useQuery、items/total 派生。
 * 差异点（筛选项/参数名/客户端排序）由调用方通过 extraParams/参数名选项补齐。
 * 示范迁移：PostList / ShareList；其余列表页后续轮次迁移。
 */

export interface PagedListApi<T> {
  (params: Record<string, unknown>): Promise<{ data: { items: T[]; total: number } }>;
}

export interface UsePagedListOptions<T> {
  apiFn: PagedListApi<T>;
  keyFn: (params: Record<string, unknown>) => readonly unknown[];
  /** 追加的筛选参数（status/category/tag 等），随 page/size/keyword 一起进 queryKey */
  extraParams?: () => Record<string, unknown>;
  /** 关键字请求参数名，默认 "keyword"（PostList 用 "search"） */
  keywordParam?: string;
  pageSize?: number;
}

export function usePagedList<T>(opts: UsePagedListOptions<T>) {
  const page = ref(1);
  const size = ref(opts.pageSize ?? 20);
  const keyword = ref("");
  const debouncedKeyword = ref("");

  // 300ms 防抖：避免每次按键触发请求
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  watch(keyword, () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      debouncedKeyword.value = keyword.value;
      page.value = 1;
    }, 300);
  });

  const queryParams = computed(() => {
    const params: Record<string, unknown> = { page: page.value, size: size.value };
    if (debouncedKeyword.value.trim()) {
      params[opts.keywordParam ?? "keyword"] = debouncedKeyword.value.trim();
    }
    Object.assign(params, opts.extraParams?.());
    return params;
  });

  const query = useQuery({
    queryKey: computed(() => opts.keyFn(queryParams.value) as unknown[]),
    queryFn: () => opts.apiFn(queryParams.value).then((r) => r.data),
  });

  const items = computed<T[]>(() => query.data.value?.items || []);
  const total = computed(() => query.data.value?.total || 0);

  return { page, size, keyword, debouncedKeyword, queryParams, items, total, ...query };
}
