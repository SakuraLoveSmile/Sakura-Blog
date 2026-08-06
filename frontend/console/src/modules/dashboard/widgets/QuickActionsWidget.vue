<script setup lang="ts">
import { ref, computed, onMounted } from"vue";
import { dashboardApi } from"@/api";
import { Icon } from"@iconify/vue";
import { VCard, VButton, VModal, Toast } from"@/components/ui";

// 可用快捷操作池：前端定义全部可选操作（key/label/icon/to），用户从中选择。
// key 与后端 settings 表存储的标识一致，to 为 console 路由路径。
const availableActions = [
 { key:"write_post", label:"写文章", icon:"ri:quill-pen-line", to:"/posts/new" },
 { key:"posts", label:"文章管理", icon:"ri:file-list-3-line", to:"/posts" },
 { key:"review_comments", label:"审核评论", icon:"ri:chat-check-line", to:"/comments" },
 { key:"upload_attachment", label:"上传附件", icon:"ri:upload-cloud-line", to:"/attachments" },
 { key:"pages", label:"页面管理", icon:"ri:pages-line", to:"/pages" },
 { key:"moments", label:"瞬间管理", icon:"ri:flashlight-line", to:"/moments" },
 { key:"notes", label:"笔记编辑", icon:"ri:sticky-note-line", to:"/notes-page" },
 { key:"taxonomy", label:"分类标签", icon:"ri:price-tag-3-line", to:"/taxonomy" },
 { key:"friends", label:"友链管理", icon:"ri:link", to:"/friends" },
 { key:"plugins", label:"插件库", icon:"ri:puzzle-line", to:"/plugins" },
 { key:"settings", label:"系统设置", icon:"ri:settings-3-line", to:"/settings" },
 { key:"backup", label:"备份管理", icon:"ri:archive-line", to:"/backup" },
];

const MAX_QUICK_ACTIONS = 8;

// 用户选中的快捷操作 key 列表（从后端加载）
const selectedKeys = ref<string[]>([]);
// 编辑弹窗的临时选中列表（保存前不影响展示）
const editKeys = ref<string[]>([]);
const showEditModal = ref(false);
const savingActions = ref(false);

/** 根据选中 key 列表映射出完整的操作对象（供模板渲染） */
const quickActions = computed(() =>
 selectedKeys.value
 .map((k) => availableActions.find((a) => a.key === k))
 .filter((a) => a != null) as typeof availableActions
);

/** 从后端加载用户自定义的快捷操作 */
onMounted(async () => {
 try {
 const { data } = await dashboardApi.quickActions();
 selectedKeys.value = data.items || [];
 } catch {
 selectedKeys.value = ["write_post","review_comments","upload_attachment","settings"];
 }
});

/** 打开编辑弹窗，用当前选中状态初始化临时列表 */
function openEditModal() {
 editKeys.value = [...selectedKeys.value];
 showEditModal.value = true;
}

/** 切换某项的选中状态（已选则取消，未选且未达上限则添加） */
function toggleAction(key: string) {
 const idx = editKeys.value.indexOf(key);
 if (idx >= 0) {
 editKeys.value.splice(idx, 1);
 } else if (editKeys.value.length < MAX_QUICK_ACTIONS) {
 editKeys.value.push(key);
 }
}

/** 保存编辑结果到后端，成功后更新展示列表并关闭弹窗 */
async function saveQuickActions() {
 savingActions.value = true;
 try {
 const { data } = await dashboardApi.saveQuickActions(editKeys.value);
 selectedKeys.value = data.items || [];
 showEditModal.value = false;
 Toast.success("快捷操作已保存");
 } catch (e: any) {
 Toast.error(e?.response?.data?.error ||"保存失败");
 } finally {
 savingActions.value = false;
 }
}
</script>

<template>
 <VCard title="快捷操作">
 <template #actions>
 <button
 class="flex items-center gap-1 text-xs text-text-tertiary transition-colors hover:text-foreground"
 @click="openEditModal"
 >
 <Icon icon="ri:edit-line" />
 编辑
 </button>
 </template>
 <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
 <router-link
 v-for="action in quickActions"
 :key="action.to"
 :to="action.to"
 class="flex flex-col items-center gap-2 rounded-base border border-border py-4 text-text-secondary transition-all hover:border-[var(--color-border-strong)] hover:bg-accent"
 >
 <Icon :icon="action.icon" class="text-2xl" />
 <span class="text-xs">{{ action.label }}</span>
 </router-link>
 <p v-if="!quickActions.length" class="col-span-full py-6 text-center text-xs text-text-tertiary">
 暂无快捷操作，点击右上角「编辑」添加
 </p>
 </div>

 <!-- 自定义快捷操作弹窗：从可用池中选择，上限 8 项 -->
 <VModal v-model:visible="showEditModal" title="自定义快捷操作" :width="520">
 <p class="mb-3 text-xs text-text-tertiary">
 从下方选择要展示的快捷操作（最多 {{ MAX_QUICK_ACTIONS }} 项），已选 {{ editKeys.length }} 项
 </p>
 <div class="grid grid-cols-3 gap-2 sm:grid-cols-4">
 <button
 v-for="a in availableActions"
 :key="a.key"
 class="flex flex-col items-center gap-1.5 rounded-base border py-3 transition-all"
 :class="
 editKeys.includes(a.key)
 ? 'border-primary/60 bg-primary/5 text-foreground'
 : 'border-border text-text-tertiary hover:border-[var(--color-border-strong)] hover:bg-accent'
"
 @click="toggleAction(a.key)"
 >
 <Icon :icon="a.icon" class="text-xl" />
 <span class="text-xs">{{ a.label }}</span>
 </button>
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <VButton size="sm" @click="showEditModal = false">取消</VButton>
 <VButton size="sm" type="secondary" :loading="savingActions" @click="saveQuickActions">保存</VButton>
 </div>
 </template>
 </VModal>
 </VCard>
</template>
