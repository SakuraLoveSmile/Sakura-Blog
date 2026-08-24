# Permissions

Authorization is represented by `roles`, `permissions`, `role_permissions`,
and `user_roles`. The legacy `users.role` column remains as a migration and
compatibility mirror for old readers; new authorization reads use the
association tables.

The backend places the resolved permission set in the request context and
protects routes with named permissions such as `content:write` and
`system:manage`. The Console receives the same set in `/auth/check` and
`/profile`, uses `meta.permissions` for routes/navigation, and provides the
`v-permission` directive for individual controls.

Adding a capability requires a permission seed, a role mapping, a backend
gate, and (when visible in Console) a manifest/UI permission declaration. A
role name alone must not be used as an authorization decision in new code.
