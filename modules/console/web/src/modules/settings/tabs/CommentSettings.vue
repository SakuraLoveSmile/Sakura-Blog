<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { ref } from "vue";
import { settingsApi } from "@/api";
import { Toast, VButton, VSwitch } from "@/components/ui";

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
    if (data.ok) {
      Toast.success("测试邮件已发送");
    } else {
      Toast.error(data.error || "发送失败");
    }
  } catch {
    testOk.value = false;
    testResult.value = "请求失败";
    Toast.error("测试邮件发送失败");
  } finally {
    testing.value = false;
  }
}

function handleSubmit() {
  emit("save");
}
</script>

<template>
  <form class="settings-comment-form" @submit.prevent="handleSubmit">
    <section class="settings-section settings-section-first">
      <div class="settings-section-heading">
        <h2>基础控制</h2>
        <p>控制评论系统的启停、前台呈现与新评论审核策略。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">启用评论系统</div>
            <p class="settings-help">关闭后前台不加载评论模块（关闭需重启服务断开接口路由）。</p>
          </div>
          <VSwitch v-model="data.comments.enabled" aria-label="启用评论系统" />
        </div>

        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">首页显示评论区</div>
            <p class="settings-help">对齐 site_tree.home.comments 开关；文章页独立评论不受此开关影响。</p>
          </div>
          <VSwitch v-model="data.site_tree.home.comments" aria-label="首页显示评论区" />
        </div>

        <div class="settings-switch">
          <div class="settings-switch-copy">
            <div class="settings-switch-name">先审后发（评论审核）</div>
            <p class="settings-help">开启后，访客提交的新评论需博主在后台审核通过后才会公开展示。</p>
          </div>
          <VSwitch v-model="data.comments.moderation" aria-label="先审后发" />
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>呈现与身份</h2>
        <p>自定义评论区标题文案、博主专属身份徽章与头像镜像源。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-field">
          <label for="settings-comment-title">评论区标题</label>
          <input id="settings-comment-title" v-model="data.comments.title" :class="inputCls" placeholder="评论" />
        </div>

        <div class="settings-field">
          <label for="settings-comment-admin-badge">博主徽章文案</label>
          <input id="settings-comment-admin-badge" v-model="data.comments.admin_badge" :class="inputCls" placeholder="博主" />
        </div>

        <div class="settings-field">
          <label for="settings-comment-admin-email">博主邮箱</label>
          <input id="settings-comment-admin-email" v-model="data.comments.admin_email" type="email" :class="inputCls" placeholder="admin@example.com" />
        </div>

        <div class="settings-field">
          <label for="settings-comment-gravatar">Gravatar 镜像地址</label>
          <input id="settings-comment-gravatar" v-model="data.comments.gravatar_mirror" :class="inputCls" placeholder="https://cravatar.cn/avatar/" />
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section-heading">
        <h2>邮件通知（SMTP）</h2>
        <p>配置 SMTP 发信服务，在收到新评论或回复时向博主发送邮件提醒（主机留空则关闭通知）。</p>
      </div>

      <div class="settings-section-body">
        <div class="settings-form-grid settings-form-grid-2">
          <div class="settings-field">
            <label for="settings-smtp-host">SMTP 主机</label>
            <input id="settings-smtp-host" v-model="data.comments.notify.smtp_host" :class="inputCls" placeholder="smtp.example.com" />
          </div>

          <div class="settings-field">
            <label for="settings-smtp-port">端口</label>
            <input id="settings-smtp-port" v-model.number="data.comments.notify.smtp_port" type="number" :class="inputCls" placeholder="465" />
          </div>
        </div>

        <div class="settings-form-grid settings-form-grid-2">
          <div class="settings-field">
            <label for="settings-smtp-user">用户名</label>
            <input id="settings-smtp-user" v-model="data.comments.notify.username" :class="inputCls" placeholder="notify@example.com" />
          </div>

          <div class="settings-field">
            <label for="settings-smtp-pass">密码 / 授权码</label>
            <input id="settings-smtp-pass" v-model="data.comments.notify.password" type="password" :class="inputCls" placeholder="••••••••" />
          </div>
        </div>

        <div class="settings-field">
          <label for="settings-smtp-from">发件人（留空使用用户名）</label>
          <input id="settings-smtp-from" v-model="data.comments.notify.from" :class="inputCls" placeholder="博客评论通知 <notify@example.com>" />
        </div>

        <div class="settings-smtp-test-row">
          <VButton type="secondary" size="sm" :disabled="testing" @click="testMail">
            <template #icon><Icon icon="ri:mail-line" /></template>
            {{ testing ? "正在发送测试邮件..." : "发送测试邮件" }}
          </VButton>
          <span v-if="testResult" :class="testOk ? 'settings-test-ok' : 'settings-test-error'">
            <Icon :icon="testOk ? 'ri:checkbox-circle-fill' : 'ri:close-circle-fill'" />
            {{ testResult }}
          </span>
        </div>
      </div>
    </section>

    <div class="settings-comment-actions">
      <div class="settings-comment-actions-inner">
        <VButton type="primary" size="md" native-type="submit" :disabled="Boolean(saving)" :loading="saving === 'comments' || saving === 'site_tree'">
          <template #icon><Icon icon="ri:save-line" /></template>
          保存评论设置
        </VButton>
      </div>
    </div>
  </form>
</template>

<style scoped>
.settings-comment-form {
  background: var(--color-card);
}

.settings-section {
  margin: 0;
  padding: 28px;
  border-bottom: 1px solid var(--color-border);
}

.settings-section-first {
  padding-top: 28px;
}

.settings-section-heading {
  max-width: 860px;
  margin-bottom: 18px;
}

.settings-section-heading h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--color-foreground);
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.settings-section-heading h2::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 16px;
  border-radius: 2px;
  background: var(--color-primary);
  flex-shrink: 0;
}

.settings-section-heading p,
.settings-help {
  margin: 6px 0 0;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}

/* 内嵌分组面板：消除多余嵌套卡片背景、边框、圆角与阴影，由外层 VCard 统一提供卡片边界 */
.settings-section-body {
  max-width: 860px;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 0;
  box-shadow: none;
}

.settings-field,
.settings-switch {
  max-width: 860px;
  margin: 0 0 20px;
  padding: 0 0 20px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
}

.settings-section-body > :last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: 0;
}

.settings-field > label {
  display: block;
  margin-bottom: 10px;
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
}

.settings-field > .apple-input {
  display: block;
  width: min(100%, 860px);
  max-width: 860px;
}

.settings-form-grid {
  display: grid;
  gap: 16px;
  margin: 0 0 20px;
  padding: 0 0 20px;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 60%, transparent);
}

.settings-form-grid .settings-field {
  margin: 0;
  padding: 0;
  border-bottom: 0;
}

.settings-form-grid-2 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.settings-switch {
  display: flex;
  min-height: 44px;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.settings-switch-copy {
  min-width: 0;
}

.settings-switch-name {
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 550;
  line-height: 1.4;
}

.settings-switch .settings-help {
  margin-top: 6px;
}

.settings-switch :deep(button[role="switch"]) {
  flex: 0 0 auto;
  margin-top: 2px;
}

.settings-smtp-test-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-top: 6px;
}

.settings-test-ok {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--color-state-success, #34c759);
  font-size: 13px;
  font-weight: 500;
}

.settings-test-error {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--color-state-error, #ff453a);
  font-size: 13px;
  font-weight: 500;
}

/* ================= 操作底栏 ================= */
.settings-comment-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  padding: 16px 28px calc(16px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-border);
  background: var(--color-card);
}

.settings-comment-actions-inner {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

@media (max-width: 767px) {
  .settings-section {
    padding-left: 16px;
    padding-right: 16px;
  }
}

@media (max-width: 640px) {
  .settings-section {
    padding-top: 24px;
    padding-bottom: 24px;
  }

  .settings-form-grid-2 {
    grid-template-columns: 1fr;
  }

  .settings-switch {
    gap: 12px;
  }

  .settings-smtp-test-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .settings-comment-actions {
    padding: 14px 16px calc(14px + env(safe-area-inset-bottom));
  }
}
</style>
