<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { ref } from "vue";
import { settingsApi } from "@/api";
import { VButton, VCard, VSwitch, Toast } from "@/components/ui";

defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [] }>();
const inputCls = "apple-input";
const testing = ref(false);
const testResult = ref("");
const testOk = ref(false);

async function testMail() {
  testing.value = true;
  testResult.value = "";
  try {
    const { data } = await settingsApi.testNotify();
    testOk.value = data.ok;
    testResult.value = data.ok ? "测试邮件已发送，请查收" : data.error || "发送失败";
  } catch {
    testOk.value = false;
    testResult.value = "请求失败";
    Toast.error("测试邮件发送失败");
  } finally {
    testing.value = false;
  }
}
</script>

<template>
  <VCard title="评论设置">
    <div class="settings-switch-row"><div><div class="settings-switch-name">首页显示评论区</div><div class="settings-switch-desc">对齐 site_tree.home.comments；文章页评论不受此开关影响</div></div><VSwitch v-model="data.site_tree.home.comments" /></div>
    <div class="settings-switch-row"><div><div class="settings-switch-name">启用评论</div><div class="settings-switch-desc">关闭需重启才断开路由</div></div><VSwitch v-model="data.comments.enabled" /></div>
    <div class="settings-switch-row"><div><div class="settings-switch-name">先审后发</div><div class="settings-switch-desc">新评论需审核后才显示</div></div><VSwitch v-model="data.comments.moderation" /></div>
    <div class="settings-form-grid settings-form-grid-2">
      <div class="settings-field"><label>评论区标题</label><input v-model="data.comments.title" :class="inputCls" /></div>
      <div class="settings-field"><label>博主徽章文案</label><input v-model="data.comments.admin_badge" :class="inputCls" /></div>
      <div class="settings-field"><label>博主邮箱</label><input v-model="data.comments.admin_email" :class="inputCls" /></div>
      <div class="settings-field"><label>Gravatar 镜像</label><input v-model="data.comments.gravatar_mirror" :class="inputCls" /></div>
    </div>
    <div class="settings-subsection"><p class="settings-subsection-title">邮件通知（SMTP，主机留空则关闭）</p><div class="settings-form-grid settings-form-grid-2"><div class="settings-field"><label>SMTP 主机</label><input v-model="data.comments.notify.smtp_host" :class="inputCls" /></div><div class="settings-field"><label>端口</label><input v-model.number="data.comments.notify.smtp_port" type="number" :class="inputCls" /></div><div class="settings-field"><label>用户名</label><input v-model="data.comments.notify.username" :class="inputCls" /></div><div class="settings-field"><label>密码</label><input v-model="data.comments.notify.password" type="password" :class="inputCls" /></div><div class="settings-field"><label>发件人（留空用用户名）</label><input v-model="data.comments.notify.from" :class="inputCls" /></div></div></div>
    <div class="settings-card-actions"><VButton type="primary" size="sm" :loading="saving === 'comments' || saving === 'site_tree'" @click="emit('save')"><template #icon><Icon icon="ri:save-line" /></template>保存评论设置</VButton><VButton type="secondary" size="sm" :disabled="testing" @click="testMail">{{ testing ? "发送中..." : "发送测试邮件" }}</VButton><span v-if="testResult" :class="testOk ? 'settings-test-ok' : 'settings-test-error'">{{ testResult }}</span></div>
  </VCard>
</template>
