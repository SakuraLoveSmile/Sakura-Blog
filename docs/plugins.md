# Plugin runtime and package contract

The project has two extension lanes:

1. Compile-time Vue modules use `definePlugin` and are collected by
   `import.meta.glob`. They may contribute routes, navigation, and widgets.
2. A runtime bundle is a same-origin ES module served from `/plugins/<id>/`.
   Its default export must have the same `id` as the backend manifest. The
   bundle is built with `vite.plugin.config.ts`; Vue, Vue Router, Pinia, and
   the `@blog/*` packages are externals supplied by the host Console.

   A bundle-only server manifest uses `"kind": "bundle"` and a
   `"bundle": "/plugins/<id>/dist/index.js"` entry; it does not execute in
   Goja.

Script plugins are separate from browser bundles. Their `manifest.json` and
`main.js` are loaded from the server's `plugins/<id>/` directory and execute
inside the Goja runtime. Only `shortcode`, `widget`, `content_filter`, and
`notification_webhook` hooks are accepted. A script must declare
`plugin:script`, a matching `hook:<name>` permission, and `network:fetch` plus
an explicit hostname allowlist before it can call the host `fetch` function.
There is no filesystem, process, module-loader, or unrestricted network global.

The reusable host packages are publishable under the `@blog/*` scope. Their
package manifests keep shared Vue/editor dependencies as peer dependencies so
plugin bundles do not ship a second Vue runtime. Use `pnpm packages:build` and
`pnpm packages:publish:dry` before a release; publishing itself remains an
explicit release action.
