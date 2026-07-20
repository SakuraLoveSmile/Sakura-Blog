import { defineStore } from "pinia";
import { ref } from "vue";
import { authApi } from "@/api";

export const useAuthStore = defineStore("auth", () => {
  const authenticated = ref(false);
  const passwordRequired = ref(true);
  const initialized = ref(false);

  async function checkAuth() {
    try {
      const { data } = await authApi.check();
      authenticated.value = data.authenticated;
      passwordRequired.value = data.passwordRequired;
    } catch {
      authenticated.value = false;
    } finally {
      initialized.value = true;
    }
  }

  async function login(password: string) {
    const { data } = await authApi.login(password);
    if (data.ok) {
      authenticated.value = true;
    }
    return data;
  }

  async function logout() {
    await authApi.logout();
    authenticated.value = false;
  }

  return { authenticated, passwordRequired, initialized, checkAuth, login, logout };
});
