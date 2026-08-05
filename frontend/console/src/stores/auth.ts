import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { authApi, profileApi, type UserProfile } from "@/api";

const emptyUser = (): UserProfile => ({
  username: "",
  display_name: "",
  email: "",
  avatar: "",
  has_2fa: false,
  role: "",
});

export const useAuthStore = defineStore("auth", () => {
  const authenticated = ref(false);
  const passwordRequired = ref(true);
  const initialized = ref(false);
  // Spec 45：当前登录用户资料（旧 cookie 无用户时保持默认值）
  const user = ref<UserProfile>(emptyUser());
  // 多用户体系：登录态角色（auth/check 返回，老 cookie/无密码站点场景兜底）
  const role = ref("");

  // 多用户体系：admin 才有系统管理权限。
  // 无密码站点保持历史全放行语义（视为 admin）；有密码站点按角色判定
  // （优先 profile 的角色，fallback 到 auth/check 的 role，兼容旧版 admin cookie）。
  const isAdmin = computed(
    () => !passwordRequired.value || user.value.role === "admin" || role.value === "admin"
  );

  // 拉取当前用户资料；旧 cookie（userID=0）下 profile 返回 404，静默保持默认值
  async function fetchUser() {
    try {
      const { data } = await profileApi.get();
      user.value = { ...emptyUser(), ...data };
    } catch {
      user.value = emptyUser();
    }
  }

  async function checkAuth() {
    try {
      const { data } = await authApi.check();
      authenticated.value = data.authenticated;
      passwordRequired.value = data.passwordRequired;
      role.value = data.role || "";
      if (data.authenticated) await fetchUser();
    } catch {
      authenticated.value = false;
    } finally {
      initialized.value = true;
    }
  }

  async function login(username: string, password: string) {
    const { data } = await authApi.login(username, password);
    if (data.ok) {
      authenticated.value = true;
      await fetchUser();
    }
    return data;
  }

  // 两步登录第二步：验证 TOTP code
  async function verify2FA(token: string, code: string) {
    const { data } = await authApi.verify2FA(token, code);
    if (data.ok) {
      authenticated.value = true;
      await fetchUser();
    }
    return data;
  }

  async function logout() {
    await authApi.logout();
    authenticated.value = false;
    user.value = emptyUser();
  }

  return {
    authenticated,
    passwordRequired,
    initialized,
    user,
    role,
    isAdmin,
    checkAuth,
    login,
    verify2FA,
    fetchUser,
    logout,
  };
});
