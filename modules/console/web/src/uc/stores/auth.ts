import { defineStore } from "pinia";
import { ref } from "vue";
import { queryClient } from "@/api/query-client";

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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
    ...init,
  });
  const data = (await response.json().catch(() => ({}))) as T & { error?: string };
  init?.signal?.throwIfAborted();
  if (!response.ok) throw new Error(data.error || "请求失败");
  return data;
}

export const useUCAuthStore = defineStore("uc-auth", () => {
  const user = ref<PublicUser | null>(null);
  const initialized = ref(false);
  let sessionRevision = 0;

  function setSession(next: PublicUser | null) {
    if (user.value?.id !== next?.id || user.value?.role !== next?.role) {
      sessionRevision++;
      queryClient.clear();
    }
    user.value = next;
  }

  async function fetchMe(signal?: AbortSignal) {
    const revision = sessionRevision;
    try {
      const data = await request<PublicUser>("/api/auth/me", { signal });
      if (revision !== sessionRevision) return user.value;
      setSession(data.verified ? data : null);
    } catch {
      if (signal?.aborted || revision !== sessionRevision) return user.value;
      clearSession();
    } finally {
      initialized.value = true;
    }
    return user.value;
  }

  async function login(username: string, password: string, code?: string) {
    const data = await request<{ user: PublicUser }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password, code }),
    });
    setSession(data.user);
    return data.user;
  }

  async function register(
    data: {
      username: string;
      password: string;
      display_name: string;
      email: string;
      invitation_code: string;
    },
    signal?: AbortSignal,
  ) {
    const result = await request<{ user: PublicUser }>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify(data),
      signal,
    });
    signal?.throwIfAborted();
    setSession(result.user);
    return result.user;
  }

  async function upgrade(password: string) {
    const result = await request<{ user: PublicUser }>("/api/auth/upgrade", {
      method: "POST",
      body: JSON.stringify({ password }),
    });
    setSession(result.user);
    return result.user;
  }

  function clearSession() {
    sessionRevision++;
    queryClient.clear();
    user.value = null;
  }

  async function logout(signal?: AbortSignal) {
    const revision = sessionRevision;
    await request("/api/auth/logout", { method: "POST", body: "{}", signal });
    if (revision !== sessionRevision) return;
    clearSession();
  }

  return { user, initialized, fetchMe, login, register, upgrade, logout, clearSession };
});
