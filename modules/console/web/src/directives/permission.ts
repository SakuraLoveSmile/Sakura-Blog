import type { Directive } from "vue";
import { useAuthStore } from "@/stores/auth";

type PermissionBinding = string | string[];

function allowed(binding: PermissionBinding): boolean {
  const required = Array.isArray(binding) ? binding : [binding];
  return required.length === 0 || required.some((permission) => useAuthStore().hasPermission(permission));
}

function applyPermission(el: HTMLElement, binding: PermissionBinding): void {
  const visible = allowed(binding);
  el.hidden = !visible;
  el.setAttribute("aria-hidden", visible ? "false" : "true");
}

/** Hide an element unless the current user has at least one named permission. */
export const permissionDirective: Directive<HTMLElement, PermissionBinding> = {
  mounted(el, binding) {
    applyPermission(el, binding.value);
  },
  updated(el, binding) {
    applyPermission(el, binding.value);
  },
};
