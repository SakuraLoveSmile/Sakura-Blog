import { defineStore } from "pinia";
import { ref } from "vue";

export interface PublicUser {
  id: number;
  username: string;
  nickname: string;
  email: string;
  avatar: string;
  role: "reader" | "author" | "admin" | string;
  verified: boolean;
  is_admin: boolean;
}

interface RegisterChallenge {
  token: string;
  question: string;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
    ...init,
  });
  const data = (await response.json().catch(() => ({}))) as T & { error?: string };
  if (response.status === 401) useUCAuthStore().clearSession();
  if (!response.ok) throw new Error(data.error || "请求失败");
  return data;
}

export const useUCAuthStore = defineStore("uc-auth", () => {
  const user = ref<PublicUser | null>(null);
  const initialized = ref(false);

  async function fetchMe() {
    try {
      const data = await request<PublicUser>("/api/auth/me");
      user.value = data.verified ? data : null;
    } catch {
      user.value = null;
    } finally {
      initialized.value = true;
    }
    return user.value;
  }

  async function login(username: string, password: string) {
    const data = await request<{ user: PublicUser }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
    user.value = data.user;
    return data.user;
  }

  async function challenge() {
    return request<RegisterChallenge>("/api/auth/register/challenge");
  }

  async function register(data: {
    username: string;
    password: string;
    display_name: string;
    email: string;
    captcha_token: string;
    captcha_answer: string;
  }) {
    const result = await request<{ user: PublicUser }>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify(data),
    });
    user.value = result.user;
    return result.user;
  }

  async function upgrade(password: string) {
    const result = await request<{ user: PublicUser }>("/api/auth/upgrade", {
      method: "POST",
      body: JSON.stringify({ password }),
    });
    user.value = result.user;
    return result.user;
  }

  function clearSession() {
    user.value = null;
  }

  async function logout() {
    await request("/api/auth/logout", { method: "POST", body: "{}" });
    user.value = null;
  }

  return { user, initialized, fetchMe, login, challenge, register, upgrade, logout, clearSession };
});
