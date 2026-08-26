<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VButton } from "@/components/ui";
import IconPicker from "@/components/IconPicker.vue";

// 首页 Hero cover 区（site.yml cover 段入库版）：title/description + features 按钮列表。
const props = defineProps<{
  data: any;
  saving: string;
}>();

const emit = defineEmits<{ (e: "save"): void }>();

const inputCls = "apple-input";
const rowBtnCls = "text-text-quaternary hover:text-text-tertiary hover:text-foreground disabled:opacity-30";

function addFeature() {
  // Spec 62：前端专用稳定 key（增删列表 v-for 不复用错位）
  props.data.features.push({ title: "", url: "", icon: "", _key: crypto.randomUUID() });
}
function removeAt(i: number) {
  props.data.features.splice(i, 1);
}
function moveItem(i: number, dir: -1 | 1) {
  const j = i + dir;
  const arr = props.data.features;
  if (j < 0 || j >= arr.length) return;
  [arr[i], arr[j]] = [arr[j], arr[i]];
}
</script>

<template>
  <div class="space-y-4">
    <p class="text-xs text-text-quaternary">
      首页 Hero 区：title 非空时首页第一页渲染（对齐 Stellar wiki_cover），留空则不渲染。
    </p>
    <div class="grid grid-cols-2 gap-4">
      <div>
        <label class="block text-xs text-text-tertiary mb-1">标题（留空关闭 Hero）</label
        ><input v-model="data.title" :class="inputCls" placeholder="Welcome" />
      </div>
      <div>
        <label class="block text-xs text-text-tertiary mb-1">描述</label
        ><input v-model="data.description" :class="inputCls" placeholder="站点简介" />
      </div>
    </div>
    <div>
      <label class="block text-xs text-text-quaternary mb-1">Scheme（预留字段，暂不生效，留空即可）</label>
      <input v-model="data.scheme" :class="inputCls" placeholder="留空即可" />
    </div>

    <div class="space-y-2">
      <p class="text-xs text-text-quaternary">功能按钮（features）</p>
      <div v-for="(f, i) in data.features" :key="f._key || i" class="grid grid-cols-12 gap-2 items-center">
        <input v-model="f.title" placeholder="按钮文案" :class="inputCls" class="col-span-4" />
        <input v-model="f.url" placeholder="/url 或 https://..." :class="inputCls" class="col-span-4" />
        <div class="col-span-2">
          <IconPicker v-model="f.icon" compact placeholder="图标（icons.yml key）" />
        </div>
        <div class="col-span-2 flex items-center gap-1.5 justify-end">
          <button
            type="button"
            title="上移"
            @click="moveItem(Number(i), -1)"
            :disabled="Number(i) === 0"
            :class="rowBtnCls"
          >
            <Icon icon="ri:arrow-up-s-line" />
          </button>
          <button
            type="button"
            title="下移"
            @click="moveItem(Number(i), 1)"
            :disabled="Number(i) === data.features.length - 1"
            :class="rowBtnCls"
          >
            <Icon icon="ri:arrow-down-s-line" />
          </button>
          <button type="button" title="删除按钮" @click="removeAt(Number(i))" class="text-state-error hover:text-state-error">
            <Icon icon="ri:delete-bin-line" />
          </button>
        </div>
      </div>
      <VButton type="secondary" size="sm" @click="addFeature">
        <template #icon><Icon icon="ri:add-line" /></template>
        添加按钮
      </VButton>
    </div>

    <div>
      <VButton type="primary" size="sm" :loading="saving === 'cover'" @click="emit('save')">保存 Hero</VButton>
    </div>
  </div>
</template>
