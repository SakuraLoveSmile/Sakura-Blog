import { defineStore } from "pinia";
import { ref } from "vue";
import { authApi, profileApi, type UserProfile } from "@/api";

const emptyUser = (): UserProfile => ({
  username: "",
  display_name: "",
  email: "",
  avatar: "",
  has_2fa: false,
});

export const useAuthStore = defineStore("auth", () => {
  const authenticated = ref(false);
  const passwordRequired = ref(true);
  const initialized = ref(false);
  // Spec 45：当前登录用户资料（旧 cookie 无用户时保持默认值）
  const user = ref<UserProfile>(emptyUser());

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
    checkAuth,
    login,
    verify2FA,
    fetchUser,
    logout,
  };
});
