<script setup lang="ts">
import { ref, onMounted } from"vue";
import { useRouter } from"vue-router";
import { authApi, profileApi } from"@/api";
import { useAuthStore } from"@/stores/auth";
import { Icon } from"@iconify/vue";
import AttachmentPickerModal from"@/components/AttachmentPickerModal.vue";
import { VPageHeader, VCard, VModal, Toast } from"@/components/ui";

const router = useRouter();
const auth = useAuthStore();

// --- 基本信息 ---
const form = ref({ display_name:"", email:"", avatar:"" });
const savingProfile = ref(false);
const showAvatarPicker = ref(false);

/** 头像 URL 选中回调（复用附件选择弹窗） */
function handleAvatarSelect(url: string) {
 form.value.avatar = url;
}

async function loadProfile() {
 await auth.fetchUser();
 form.value = {
 display_name: auth.user.display_name,
 email: auth.user.email,
 avatar: auth.user.avatar,
 };
}

async function saveProfile() {
 savingProfile.value = true;
 try {
 await profileApi.update(form.value);
 await auth.fetchUser();
 Toast.success("资料已保存");
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"保存失败");
 } finally {
 savingProfile.value = false;
 }
}

// --- 修改密码 ---
const pwd = ref({ current:"", newPwd:"", confirm:"" });
const savingPwd = ref(false);

async function changePassword() {
 if (!pwd.value.current) {
 Toast.error("请输入当前密码");
 return;
 }
 if (pwd.value.newPwd !== pwd.value.confirm) {
 Toast.error("两次输入的新密码不一致");
 return;
 }
 if (!pwd.value.newPwd) {
 Toast.error("新密码不能为空");
 return;
 }
 savingPwd.value = true;
 try {
 // 后端轮换 auth_secret，改密后所有已登录会话失效，需重新登录
 await authApi.changePassword(pwd.value.current, pwd.value.newPwd);
 pwd.value = { current:"", newPwd:"", confirm:"" };
 Toast.success("密码已修改，请使用新密码重新登录");
 await auth.logout();
 router.push("/login");
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"修改失败");
 } finally {
 savingPwd.value = false;
 }
}

// --- 2FA 状态与开启/关闭/恢复码管理 ---
const has2FA = ref(false);
const setupLoading = ref(false);
const setupData = ref<{
 secret: string;
 auth_url: string;
 recovery_codes: string[];
} | null>(null);
const setupCode = ref("");
const confirming = ref(false);
const showRecovery = ref(false);
const recoveryCodes = ref<string[]>([]);
const loadingRecovery = ref(false);
const showDisableModal = ref(false);
const disableCode = ref("");
const disabling = ref(false);

onMounted(async () => {
 await loadProfile();
 has2FA.value = auth.user.has_2fa;
 loadTokenStatus();
});

/** 开启：获取密钥 + otpauth URL + 恢复码 */
async function startSetup() {
 setupLoading.value = true;
 try {
 const { data } = await profileApi.setup2FA();
 setupData.value = data;
 setupCode.value ="";
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"获取 2FA 配置失败");
 } finally {
 setupLoading.value = false;
 }
}

/** 输入 TOTP 验证码确认开启 */
async function confirmSetup() {
 if (!setupData.value) return;
 if (!/^\d{6}$/.test(setupCode.value)) {
 Toast.error("请输入 6 位数字验证码");
 return;
 }
 confirming.value = true;
 try {
 const data = setupData.value;
 await profileApi.confirm2FA({
 secret: data.secret,
 code: setupCode.value,
 recovery_codes: data.recovery_codes,
 });
 has2FA.value = true;
 // 展示本次生成的恢复码，提醒妥善保存
 recoveryCodes.value = data.recovery_codes;
 showRecovery.value = true;
 setupData.value = null;
 setupCode.value ="";
 Toast.success("两步验证已开启，请妥善保存恢复码");
 auth.fetchUser().catch(() => {});
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"确认失败");
 } finally {
 confirming.value = false;
 }
}

/** 查看恢复码列表 */
async function loadRecoveryCodes() {
 loadingRecovery.value = true;
 try {
 const { data } = await profileApi.getRecoveryCodes();
 recoveryCodes.value = data.recovery_codes || [];
 showRecovery.value = true;
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"获取恢复码失败");
 } finally {
 loadingRecovery.value = false;
 }
}

function openDisableModal() {
 disableCode.value ="";
 showDisableModal.value = true;
}

/** 输入 TOTP 验证码确认关闭 */
async function confirmDisable() {
 if (!/^\d{6}$/.test(disableCode.value)) {
 Toast.error("请输入 6 位数字验证码");
 return;
 }
 disabling.value = true;
 try {
 await profileApi.disable2FA(disableCode.value);
 has2FA.value = false;
 showDisableModal.value = false;
 showRecovery.value = false;
 recoveryCodes.value = [];
 disableCode.value ="";
 Toast.success("两步验证已关闭");
 auth.fetchUser().catch(() => {});
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"关闭失败");
 } finally {
 disabling.value = false;
 }
}

/** 复制文本（优先 Clipboard API，失败时降级 execCommand） */
async function copyText(text: string) {
 try {
 await navigator.clipboard.writeText(text);
 } catch {
 const ta = document.createElement("textarea");
 ta.value = text;
 document.body.appendChild(ta);
 ta.select();
 document.execCommand("copy");
 ta.remove();
 }
 Toast.success("已复制");
}

// --- API Token（Wave 3a：Bearer 认证凭证，明文仅生成时展示一次） ---
const hasToken = ref(false);
const generating = ref(false);
const revoking = ref(false);
const showTokenModal = ref(false);
const generatedToken = ref("");
const showRevokeModal = ref(false);

/** 查询当前是否已启用 API Token */
async function loadTokenStatus() {
 try {
 const { data } = await profileApi.apiTokenStatus();
 hasToken.value = !!data.has_token;
 } catch (e: any) {
 // 状态查询失败不阻塞页面，按钮操作时会再次校验
 Toast.error(e?.response?.data?.error || e?.message ||"查询 Token 状态失败");
 }
}

/** 生成新 API Token：明文仅本次弹窗展示一次，关闭后不可再查看 */
async function generateToken() {
 generating.value = true;
 try {
 const { data } = await profileApi.generateApiToken();
 generatedToken.value = data.token;
 hasToken.value = true;
 showTokenModal.value = true;
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"生成失败");
 } finally {
 generating.value = false;
 }
}

/** 撤销 API Token：确认后置空哈希，所有使用该 Token 的调用立即 401 */
async function confirmRevoke() {
 revoking.value = true;
 try {
 await profileApi.revokeApiToken();
 hasToken.value = false;
 showRevokeModal.value = false;
 Toast.success("API Token 已撤销，Bearer 调用立即失效");
 } catch (e: any) {
 Toast.error(e?.response?.data?.error || e?.message ||"撤销失败");
 } finally {
 revoking.value = false;
 }
}

const inputCls = "apple-input";
const btnCls =
"px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg border border-secondary-700/30 hover:opacity-90 disabled:opacity-50 shadow-sm";
const btnClsDanger =
"px-4 py-2 bg-state-error/100 text-white text-sm font-medium rounded-lg hover:opacity-90 disabled:opacity-50";
</script>

<template>
 <div>
 <VPageHeader title="个人资料">
 <template #icon>
 <Icon icon="ri:user-settings-line" class="mr-2 self-center text-xl" />
 </template>
 </VPageHeader>

 <div class="mx-auto max-w-2xl space-y-8 p-6">
 <!-- 基本信息区 -->
 <VCard title="基本信息">
 <div class="space-y-4">
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">显示名称</label>
 <input v-model="form.display_name" :class="inputCls" placeholder="输入显示名称" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">邮箱</label>
 <input v-model="form.email" type="email" :class="inputCls" placeholder="you@example.com" />
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">头像</label>
 <div class="flex items-center gap-4">
 <img
 v-if="form.avatar"
 :src="form.avatar"
 alt="avatar"
 class="h-16 w-16 rounded-full border border-border object-cover"
 />
 <span
 v-else
 class="flex h-16 w-16 items-center justify-center rounded-full bg-primary/15"
 >
 <Icon icon="ri:user-3-line" class="text-2xl text-primary" />
 </span>
 <div class="flex flex-col gap-2">
 <button
 class="rounded-lg bg-primary/10 px-3 py-2 text-sm text-secondary transition-colors hover:bg-primary/20 bg-primary/15 text-primary"
 @click="showAvatarPicker = true"
 >
 <Icon icon="ri:image-upload-line" class="mr-1" />选择头像
 </button>
 <button
 v-if="form.avatar"
 class="text-xs text-text-quaternary transition-colors hover:text-state-error"
 @click="form.avatar = ''"
 >
 清除头像
 </button>
 </div>
 </div>
 </div>
 <div>
 <button :disabled="savingProfile" :class="btnCls" @click="saveProfile">
 {{ savingProfile ?"保存中..." :"保存" }}
 </button>
 </div>
 </div>
 </VCard>

 <!-- 修改密码区 -->
 <VCard title="修改密码">
 <div class="space-y-4">
 <p class="text-xs text-text-quaternary">
 修改后所有已登录会话将失效，需使用新密码重新登录。
 </p>
 <input v-model="pwd.current" type="password" placeholder="当前密码" autocomplete="current-password" :class="inputCls" />
 <input v-model="pwd.newPwd" type="password" placeholder="新密码" autocomplete="new-password" :class="inputCls" />
 <input v-model="pwd.confirm" type="password" placeholder="确认新密码" autocomplete="new-password" :class="inputCls" />
 <div>
 <button :disabled="savingPwd" :class="btnCls" @click="changePassword">
 {{ savingPwd ?"提交中..." :"修改密码" }}
 </button>
 </div>
 </div>
 </VCard>

 <!-- 2FA 区 -->
 <VCard title="两步验证（TOTP）">
 <div class="space-y-4">
 <!-- 未开启 -->
 <template v-if="!has2FA">
 <p class="flex items-center gap-2 text-sm text-text-tertiary">
 <Icon icon="ri:shield-line" /> 两步验证未开启
 </p>
 <p class="text-xs text-text-quaternary">
 开启后，登录时需要输入认证器 App 生成的 6 位动态验证码，可有效提升账户安全性。
 </p>

 <div v-if="!setupData">
 <button :disabled="setupLoading" :class="btnCls" @click="startSetup">
 {{ setupLoading ?"获取中..." :"开启两步验证" }}
 </button>
 </div>

 <!-- 开启流程：密钥 + otpauth URL + 验证码确认 -->
 <div
 v-else
 class="space-y-4 rounded-lg border border-border p-4"
 >
 <p class="text-sm font-medium text-text-secondary">
 <span class="mr-1 text-primary">1.</span>添加密钥到认证器 App
 </p>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">
 密钥（Secret）
 </label>
 <div class="flex items-center gap-2">
 <code
 class="flex-1 break-all rounded bg-secondary px-2 py-1.5 font-mono text-xs"
 >{{ setupData.secret }}</code>
 <button
 class="flex-shrink-0 rounded px-2 py-1 text-xs text-secondary transition-colors hover:opacity-80"
 @click="copyText(setupData.secret)"
 >
 <Icon icon="ri:file-copy-line" class="mr-0.5" />复制
 </button>
 </div>
 </div>
 <div>
 <label class="mb-1 block text-xs text-text-tertiary">
 otpauth:// URL（可手动输入或用于生成二维码）
 </label>
 <code
 class="block break-all rounded bg-secondary px-2 py-1.5 font-mono text-[11px] leading-relaxed"
 >{{ setupData.auth_url }}</code>
 </div>
 <p class="text-xs text-text-quaternary">
 请勿泄露密钥。无法扫码时，可在认证器 App 中手动输入以上密钥。
 </p>

 <div>
 <label class="mb-1 block text-xs text-text-tertiary">
 <span class="mr-1 text-primary">2.</span>输入验证码确认
 </label>
 <input
 v-model="setupCode"
 type="text"
 inputmode="numeric"
 maxlength="6"
 placeholder="6 位动态验证码"
 :class="inputCls"
 @keyup.enter="confirmSetup"
 />
 </div>
 <div class="flex items-center gap-3">
 <button :disabled="confirming" :class="btnCls" @click="confirmSetup">
 {{ confirming ?"确认中..." :"确认开启" }}
 </button>
 <button
 class="text-xs text-text-quaternary transition-colors hover:text-text-tertiary hover:text-foreground"
 @click="setupData = null"
 >
 取消
 </button>
 </div>
 </div>
 </template>

 <!-- 已开启 -->
 <template v-else>
 <p class="flex items-center gap-2 text-sm text-state-success text-state-success">
 <Icon icon="ri:shield-check-line" /> 两步验证已开启
 </p>
 <p class="text-xs text-text-quaternary">
 登录时需要输入认证器 App 生成的 6 位动态验证码。
 </p>
 <div class="flex flex-wrap gap-3">
 <button :disabled="loadingRecovery" :class="btnCls" @click="loadRecoveryCodes">
 {{ loadingRecovery ?"加载中..." :"查看恢复码" }}
 </button>
 <button :class="btnClsDanger" @click="openDisableModal">关闭两步验证</button>
 </div>

 <!-- 恢复码列表 -->
 <div
 v-if="showRecovery"
 class="rounded-lg border border-state-warning bg-state-warning/10 p-4 border-state-warning/60 bg-state-warning/15"
 >
 <p class="flex items-center gap-1 text-xs font-medium text-state-warning text-state-warning">
 <Icon icon="ri:alert-line" />
 恢复码仅显示一次/仅可使用一次，请立即妥善保存。
 </p>
 <div class="mt-3 flex flex-wrap gap-2">
 <code
 v-for="(code, i) in recoveryCodes"
 :key="i"
 class="rounded bg-card px-2 py-1 font-mono text-xs"
 >{{ code }}</code>
 </div>
 <p v-if="!recoveryCodes.length" class="mt-2 text-xs text-text-quaternary">
 暂无可用恢复码
 </p>
 </div>
 </template>
 </div>
 </VCard>

 <!-- API Token 区（Wave 3a：Bearer 认证凭证） -->
 <VCard title="API Token（Bearer 认证）">
 <div class="space-y-4">
 <template v-if="!hasToken">
 <p class="flex items-center gap-2 text-sm text-text-tertiary">
 <Icon icon="ri:key-2-line" /> 未生成 API Token
 </p>
 <p class="text-xs text-text-quaternary">
 生成后可通过 <code class="rounded bg-secondary px-1 py-0.5 font-mono text-[11px]">Authorization: Bearer
 &lt;token&gt;</code> 调用管理端 API（脚本/第三方工具）。明文仅生成时展示一次，请立即保存。
 </p>
 <button :disabled="generating" :class="btnCls" @click="generateToken">
 {{ generating ?"生成中..." :"生成 API Token" }}
 </button>
 </template>

 <template v-else>
 <p class="flex items-center gap-2 text-sm text-state-success text-state-success">
 <Icon icon="ri:key-2-fill" /> API Token 已启用
 </p>
 <p class="text-xs text-text-quaternary">
 轮换 Token 将撤销旧 Token 并生成新的（旧 Token 立即失效）。
 </p>
 <div class="flex flex-wrap gap-3">
 <button :disabled="generating" :class="btnCls" @click="generateToken">
 {{ generating ?"生成中..." :"轮换 Token" }}
 </button>
 <button :class="btnClsDanger" @click="showRevokeModal = true">撤销 Token</button>
 </div>
 </template>
 </div>
 </VCard>

 <!-- 关闭 2FA 确认弹窗 -->
 <VModal
 :visible="showDisableModal"
 title="关闭两步验证"
 @update:visible="showDisableModal = $event"
 @close="showDisableModal = false"
 >
 <div class="space-y-4">
 <p class="text-xs leading-relaxed text-text-tertiary">
 关闭后登录将不再需要动态验证码，账户安全性将降低。请输入当前认证器 App
 生成的 6 位验证码以确认操作。
 </p>
 <input
 v-model="disableCode"
 type="text"
 inputmode="numeric"
 maxlength="6"
 placeholder="6 位动态验证码"
 :class="inputCls"
 @keyup.enter="confirmDisable"
 />
 </div>
 <template #footer>
 <div class="flex justify-end gap-2">
 <button
 class="rounded-lg px-4 py-2 text-sm text-text-tertiary transition-colors hover:bg-secondary hover:bg-accent"
 @click="showDisableModal = false"
 >
 取消
 </button>
 <button :disabled="disabling" :class="btnClsDanger" @click="confirmDisable">
 {{ disabling ?"关闭中..." :"确认关闭" }}
 </button>
 </div>
 </template>
 </VModal>

 <!-- API Token 明文展示弹窗（仅生成时展示一次） -->
 <VModal :visible="showTokenModal" title="API Token 已生成" @update:visible="showTokenModal = $event" @close="showTokenModal = false">
 <div class="space-y-4">
 <p class="flex items-center gap-1 text-xs font-medium text-state-warning text-state-warning">
 <Icon icon="ri:alert-line" />
 明文仅本次展示，关闭后将无法再次查看，请立即复制并妥善保存。
 </p>
 <code
 class="block break-all rounded bg-secondary px-2 py-1.5 font-mono text-xs leading-relaxed"
 >{{ generatedToken }}</code>
 <div class="flex items-center gap-3">
 <button :class="btnCls" @click="copyText(generatedToken)">
 <Icon icon="ri:file-copy-line" class="mr-0.5" />复制 Token
 </button>
 <button
 class="text-xs text-text-quaternary transition-colors hover:text-text-tertiary hover:text-foreground"
 @click="showTokenModal = false"
 >
 我已保存
 </button>
 </div>
 </div>
 </VModal>

 <!-- 撤销 Token 确认弹窗 -->
 <VModal :visible="showRevokeModal" title="撤销 API Token" @update:visible="showRevokeModal = $event" @close="showRevokeModal = false">
 <p class="text-xs leading-relaxed text-text-tertiary">
 撤销后所有使用该 Token 的调用将立即返回 401，且无法恢复。如需继续使用需重新生成。
 </p>
 <template #footer>
 <div class="flex justify-end gap-2">
 <button
 class="rounded-lg px-4 py-2 text-sm text-text-tertiary transition-colors hover:bg-secondary hover:bg-accent"
 @click="showRevokeModal = false"
 >
 取消
 </button>
 <button :disabled="revoking" :class="btnClsDanger" @click="confirmRevoke">
 {{ revoking ?"撤销中..." :"确认撤销" }}
 </button>
 </div>
 </template>
 </VModal>

 <!-- 头像选择器 -->
 <AttachmentPickerModal
 :open="showAvatarPicker"
 @select="handleAvatarSelect"
 @close="showAvatarPicker = false"
 />
 </div>
 </div>
</template>

<style scoped>
/* ===== 移动端适配 ===== */
@media (max-width: 640px) {
  .max-w-2xl.space-y-8 {
    padding: 12px 14px !important;
    gap: 16px !important;
  }

  /* 卡片标题更紧凑 */
  :deep(.card-header) {
    padding: 12px 14px !important;
    font-size: 14px !important;
  }
  :deep(.card-body) {
    padding: 14px !important;
  }

  /* 表单字段更紧凑 */
  .flex.flex-wrap.gap-3 {
    gap: 10px;
  }

  /* 2FA 密钥和恢复码字体缩小 */
  .font-mono,
  .break-all {
    font-size: 11px !important;
    word-break: break-all;
  }

  /* API Token 卡片 */
  .rounded-lg.px-4.py-2 {
    padding: 6px 10px !important;
    font-size: 12px !important;
  }

  /* 按钮更紧凑 */
  button {
    font-size: 12px !important;
  }

  /* 代码片段 */
  code {
    font-size: 10.5px !important;
  }
}
</style>
