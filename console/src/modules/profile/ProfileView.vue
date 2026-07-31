<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { authApi, profileApi } from "@/api";
import { useAuthStore } from "@/stores/auth";
import { Icon } from "@iconify/vue";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { VPageHeader, VCard, VModal, Toast } from "@/components/ui";

const router = useRouter();
const auth = useAuthStore();

// --- 基本信息 ---
const form = ref({ display_name: "", email: "", avatar: "" });
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
    Toast.error(e?.response?.data?.error || e?.message || "保存失败");
  } finally {
    savingProfile.value = false;
  }
}

// --- 修改密码 ---
const pwd = ref({ current: "", newPwd: "", confirm: "" });
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
    pwd.value = { current: "", newPwd: "", confirm: "" };
    Toast.success("密码已修改，请使用新密码重新登录");
    await auth.logout();
    router.push("/login");
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "修改失败");
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
});

/** 开启：获取密钥 + otpauth URL + 恢复码 */
async function startSetup() {
  setupLoading.value = true;
  try {
    const { data } = await profileApi.setup2FA();
    setupData.value = data;
    setupCode.value = "";
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "获取 2FA 配置失败");
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
    setupCode.value = "";
    Toast.success("两步验证已开启，请妥善保存恢复码");
    auth.fetchUser().catch(() => {});
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "确认失败");
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
    Toast.error(e?.response?.data?.error || e?.message || "获取恢复码失败");
  } finally {
    loadingRecovery.value = false;
  }
}

function openDisableModal() {
  disableCode.value = "";
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
    disableCode.value = "";
    Toast.success("两步验证已关闭");
    auth.fetchUser().catch(() => {});
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || "关闭失败");
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

const inputCls =
  "w-full px-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500";
const btnCls =
  "px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg hover:opacity-90 disabled:opacity-50";
const btnClsDanger =
  "px-4 py-2 bg-red-500 text-white text-sm font-medium rounded-lg hover:opacity-90 disabled:opacity-50";
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
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">显示名称</label>
            <input v-model="form.display_name" :class="inputCls" placeholder="输入显示名称" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">邮箱</label>
            <input v-model="form.email" type="email" :class="inputCls" placeholder="you@example.com" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">头像</label>
            <div class="flex items-center gap-4">
              <img
                v-if="form.avatar"
                :src="form.avatar"
                alt="avatar"
                class="h-16 w-16 rounded-full border border-gray-200 object-cover dark:border-gray-600"
              />
              <span
                v-else
                class="flex h-16 w-16 items-center justify-center rounded-full bg-primary/15"
              >
                <Icon icon="ri:user-3-line" class="text-2xl text-primary" />
              </span>
              <div class="flex flex-col gap-2">
                <button
                  class="rounded-lg bg-primary/10 px-3 py-2 text-sm text-secondary transition-colors hover:bg-primary/20 dark:bg-primary/15 dark:text-primary"
                  @click="showAvatarPicker = true"
                >
                  <Icon icon="ri:image-upload-line" class="mr-1" />选择头像
                </button>
                <button
                  v-if="form.avatar"
                  class="text-xs text-gray-400 transition-colors hover:text-red-500"
                  @click="form.avatar = ''"
                >
                  清除头像
                </button>
              </div>
            </div>
          </div>
          <div>
            <button :disabled="savingProfile" :class="btnCls" @click="saveProfile">
              {{ savingProfile ? "保存中..." : "保存" }}
            </button>
          </div>
        </div>
      </VCard>

      <!-- 修改密码区 -->
      <VCard title="修改密码">
        <div class="space-y-4">
          <p class="text-xs text-gray-400 dark:text-gray-500">
            修改后所有已登录会话将失效，需使用新密码重新登录。
          </p>
          <input v-model="pwd.current" type="password" placeholder="当前密码" autocomplete="current-password" :class="inputCls" />
          <input v-model="pwd.newPwd" type="password" placeholder="新密码" autocomplete="new-password" :class="inputCls" />
          <input v-model="pwd.confirm" type="password" placeholder="确认新密码" autocomplete="new-password" :class="inputCls" />
          <div>
            <button :disabled="savingPwd" :class="btnCls" @click="changePassword">
              {{ savingPwd ? "提交中..." : "修改密码" }}
            </button>
          </div>
        </div>
      </VCard>

      <!-- 2FA 区 -->
      <VCard title="两步验证（TOTP）">
        <div class="space-y-4">
          <!-- 未开启 -->
          <template v-if="!has2FA">
            <p class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
              <Icon icon="ri:shield-line" /> 两步验证未开启
            </p>
            <p class="text-xs text-gray-400 dark:text-gray-500">
              开启后，登录时需要输入认证器 App 生成的 6 位动态验证码，可有效提升账户安全性。
            </p>

            <div v-if="!setupData">
              <button :disabled="setupLoading" :class="btnCls" @click="startSetup">
                {{ setupLoading ? "获取中..." : "开启两步验证" }}
              </button>
            </div>

            <!-- 开启流程：密钥 + otpauth URL + 验证码确认 -->
            <div
              v-else
              class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-gray-600"
            >
              <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
                <span class="mr-1 text-primary">1.</span>添加密钥到认证器 App
              </p>
              <div>
                <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
                  密钥（Secret）
                </label>
                <div class="flex items-center gap-2">
                  <code
                    class="flex-1 break-all rounded bg-gray-100 px-2 py-1.5 font-mono text-xs dark:bg-gray-700 dark:text-gray-200"
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
                <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
                  otpauth:// URL（可手动输入或用于生成二维码）
                </label>
                <code
                  class="block break-all rounded bg-gray-100 px-2 py-1.5 font-mono text-[11px] leading-relaxed dark:bg-gray-700 dark:text-gray-200"
                >{{ setupData.auth_url }}</code>
              </div>
              <p class="text-xs text-gray-400 dark:text-gray-500">
                请勿泄露密钥。无法扫码时，可在认证器 App 中手动输入以上密钥。
              </p>

              <div>
                <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
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
                  {{ confirming ? "确认中..." : "确认开启" }}
                </button>
                <button
                  class="text-xs text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-300"
                  @click="setupData = null"
                >
                  取消
                </button>
              </div>
            </div>
          </template>

          <!-- 已开启 -->
          <template v-else>
            <p class="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
              <Icon icon="ri:shield-check-line" /> 两步验证已开启
            </p>
            <p class="text-xs text-gray-400 dark:text-gray-500">
              登录时需要输入认证器 App 生成的 6 位动态验证码。
            </p>
            <div class="flex flex-wrap gap-3">
              <button :disabled="loadingRecovery" :class="btnCls" @click="loadRecoveryCodes">
                {{ loadingRecovery ? "加载中..." : "查看恢复码" }}
              </button>
              <button :class="btnClsDanger" @click="openDisableModal">关闭两步验证</button>
            </div>

            <!-- 恢复码列表 -->
            <div
              v-if="showRecovery"
              class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/60 dark:bg-amber-900/20"
            >
              <p class="flex items-center gap-1 text-xs font-medium text-amber-700 dark:text-amber-400">
                <Icon icon="ri:alert-line" />
                恢复码仅显示一次/仅可使用一次，请立即妥善保存。
              </p>
              <div class="mt-3 flex flex-wrap gap-2">
                <code
                  v-for="(code, i) in recoveryCodes"
                  :key="i"
                  class="rounded bg-white px-2 py-1 font-mono text-xs dark:bg-gray-800 dark:text-gray-200"
                >{{ code }}</code>
              </div>
              <p v-if="!recoveryCodes.length" class="mt-2 text-xs text-gray-400">
                暂无可用恢复码
              </p>
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
          <p class="text-xs leading-relaxed text-gray-500 dark:text-gray-400">
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
              class="rounded-lg px-4 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
              @click="showDisableModal = false"
            >
              取消
            </button>
            <button :disabled="disabling" :class="btnClsDanger" @click="confirmDisable">
              {{ disabling ? "关闭中..." : "确认关闭" }}
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
