<script setup lang="ts">
import { ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { revisionApi } from "@/api";
import { VButton, Dialog, Toast } from "@/components/ui";

// 文章版本历史面板（Spec 22）：列表 → 选中查看与当前内容的逐行 diff → 一键回滚。
const props = defineProps<{
  open: boolean;
  slug: string;
  currentContent: string;
  currentWordCount: number;
}>();
const emit = defineEmits<{
  (e: "close"): void;
  (e: "restored"): void;
}>();

interface RevisionMeta {
  id: number;
  title: string;
  word_count: number;
  created_at: string;
}

const metas = ref<RevisionMeta[]>([]);
const loadingList = ref(false);
const selected = ref<RevisionMeta | null>(null);
const diffLines = ref<DiffLine[]>([]);
const loadingDiff = ref(false);
const restoring = ref(false);
const error = ref("");

watch(
  () => props.open,
  (v) => {
    if (v) {
      selected.value = null;
      diffLines.value = [];
      error.value = "";
      loadList();
    }
  }
);

async function loadList() {
  if (!props.slug) return;
  loadingList.value = true;
  try {
    const { data } = await revisionApi.list(props.slug);
    metas.value = data.items || [];
  } catch (e: any) {
    error.value = e.response?.data?.error || "加载版本列表失败";
  } finally {
    loadingList.value = false;
  }
}

async function selectRevision(m: RevisionMeta) {
  selected.value = m;
  loadingDiff.value = true;
  error.value = "";
  try {
    const { data } = await revisionApi.get(m.id);
    // 对比对象：版本内容（旧）vs 当前内容（新）；新增绿、删除红
    diffLines.value = computeDiff(data.content_md || "", props.currentContent || "");
  } catch (e: any) {
    error.value = e.response?.data?.error || "加载版本内容失败";
  } finally {
    loadingDiff.value = false;
  }
}

async function handleRestore() {
  if (!selected.value) return;
  const target = selected.value;
  Dialog.warning({
    title: "确认回滚到该版本？",
    description: "当前内容会先存为新快照，可随时再回滚。",
    confirmType: "danger",
    onConfirm: async () => {
      restoring.value = true;
      error.value = "";
      try {
        await revisionApi.restore(target.id);
        Toast.success("已回滚");
        emit("restored");
        emit("close");
      } catch (e: any) {
        error.value = e.response?.data?.error || "回滚失败";
      } finally {
        restoring.value = false;
      }
    },
  });
}

function fmtTime(s: string): string {
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  return d.toLocaleString("zh-CN", { hour12: false });
}

function wordDelta(m: RevisionMeta): string {
  const d = props.currentWordCount - m.word_count;
  if (d > 0) return `+${d}`;
  return `${d}`;
}

// --- 纯前端逐行 LCS diff（不引依赖）---
interface DiffLine {
  type: "add" | "del" | "ctx";
  text: string;
}

function computeDiff(oldStr: string, newStr: string): DiffLine[] {
  const a = oldStr.split("\n");
  const b = newStr.split("\n");
  const m = a.length;
  const n = b.length;
  // 超大文本降级：避免 DP 表内存爆炸，直接整体标删/标增
  if (m * n > 5_000_000) {
    const out: DiffLine[] = [];
    a.forEach((t) => out.push({ type: "del", text: t }));
    b.forEach((t) => out.push({ type: "add", text: t }));
    return out;
  }
  const dp = new Int32Array((m + 1) * (n + 1));
  const at = (i: number, j: number) => dp[i * (n + 1) + j];
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      dp[i * (n + 1) + j] =
        a[i] === b[j]
          ? at(i + 1, j + 1) + 1
          : Math.max(at(i + 1, j), at(i, j + 1));
    }
  }
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) {
      out.push({ type: "ctx", text: a[i] });
      i++;
      j++;
    } else if (at(i + 1, j) >= at(i, j + 1)) {
      out.push({ type: "del", text: a[i] });
      i++;
    } else {
      out.push({ type: "add", text: b[j] });
      j++;
    }
  }
  while (i < m) out.push({ type: "del", text: a[i++] });
  while (j < n) out.push({ type: "add", text: b[j++] });
  return out;
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 flex justify-end bg-black/40"
    @click.self="emit('close')"
  >
    <div class="w-full max-w-3xl h-full flex flex-col bg-white shadow-xl dark:bg-gray-800">
      <div class="flex items-center justify-between px-5 py-3 border-b border-gray-100 dark:border-gray-700">
        <h3 class="text-sm font-medium text-gray-800 dark:text-gray-100">版本历史</h3>
        <button @click="emit('close')" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
          <Icon icon="ri:close-line" class="text-xl" />
        </button>
      </div>

      <p v-if="error" class="mx-5 mt-3 text-sm text-red-600 bg-red-50 px-3 py-2 rounded-lg dark:text-red-400 dark:bg-red-500/10">{{ error }}</p>

      <div class="flex-1 grid grid-cols-1 md:grid-cols-5 gap-0 overflow-hidden">
        <!-- 版本列表 -->
        <div class="md:col-span-2 overflow-y-auto border-r border-gray-100 dark:border-gray-700">
          <p v-if="loadingList" class="p-4 text-sm text-gray-500 dark:text-gray-400">加载中...</p>
          <p v-else-if="metas.length === 0" class="p-4 text-sm text-gray-500 dark:text-gray-400">暂无历史版本</p>
          <button
            v-for="m in metas"
            :key="m.id"
            @click="selectRevision(m)"
            class="w-full text-left px-4 py-3 border-b border-gray-50 hover:bg-gray-50 dark:border-gray-700/50 dark:hover:bg-gray-700/40"
            :class="{ 'bg-primary/10 dark:bg-primary/10': selected?.id === m.id }"
          >
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium text-gray-700 truncate dark:text-gray-200">{{ m.title || "（无标题）" }}</span>
              <span
                class="ml-2 shrink-0 text-xs tabular-nums"
                :class="wordDelta(m).startsWith('+') ? 'text-green-600 dark:text-green-400' : 'text-gray-400'"
              >{{ wordDelta(m) }}</span>
            </div>
            <div class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">{{ fmtTime(m.created_at) }}</div>
          </button>
        </div>

        <!-- diff 视图 -->
        <div class="md:col-span-3 overflow-y-auto">
          <div v-if="!selected" class="p-6 text-sm text-gray-400 dark:text-gray-500">选择左侧版本查看与当前内容的差异</div>
          <div v-else-if="loadingDiff" class="p-6 text-sm text-gray-500 dark:text-gray-400">加载差异中...</div>
          <div v-else>
            <div class="sticky top-0 z-10 flex items-center justify-between px-4 py-2 bg-gray-50 border-b border-gray-100 dark:bg-gray-800 dark:border-gray-700">
              <span class="text-xs text-gray-500 dark:text-gray-400">对比当前内容（<span class="text-green-600 dark:text-green-400">新增</span> / <span class="text-red-600 dark:text-red-400">删除</span>）</span>
              <VButton type="danger" size="sm" :loading="restoring" @click="handleRestore">回滚到此版本</VButton>
            </div>
            <pre class="text-xs leading-5 font-mono"><template v-for="(line, idx) in diffLines" :key="idx"><span
              class="block px-4 whitespace-pre-wrap break-words"
              :class="{
                'bg-green-50 text-green-800 dark:bg-green-500/10 dark:text-green-300': line.type === 'add',
                'bg-red-50 text-red-800 dark:bg-red-500/10 dark:text-red-300': line.type === 'del',
                'text-gray-600 dark:text-gray-400': line.type === 'ctx'
              }"
            >{{ line.type === 'add' ? '+ ' : line.type === 'del' ? '- ' : '  ' }}{{ line.text }}</span></template></pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
