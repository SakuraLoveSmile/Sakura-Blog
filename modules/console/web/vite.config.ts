import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  base: "/console/",
  plugins: [
    vue({
      template: {
        compilerOptions: {
          // <feedback-widget> 是 Feedback 服务的 Custom Element（运行时 ESM 注册），
          // 不是 Vue 组件——告诉模板编译器直接透传该标签。
          isCustomElement: (tag) => tag === "feedback-widget",
        },
      },
    }),
    tailwindcss(),
    // Mock 数据：dev 模式下拦截备份/认证接口，无需启动后端服务
    {
      name: "mock-dev",
      configureServer(server) {
        const mock = (req: any, res: any, next: any) => {
          const url = req.url || "";
          if (url.startsWith("/api/v1/auth/check")) {
            res.setHeader("Content-Type", "application/json");
            res.end(JSON.stringify({ authenticated: true, passwordRequired: false }));
            return;
          }
          if (url.startsWith("/api/v1/profiles/me")) {
            res.setHeader("Content-Type", "application/json");
            res.end(
              JSON.stringify({
                username: "admin",
                display_name: "管理员",
                email: "admin@example.com",
                avatar: "",
                has_2fa: false,
              })
            );
            return;
          }
          if (url.startsWith("/api/v1/backup/status")) {
            res.setHeader("Content-Type", "application/json");
            res.end(
              JSON.stringify({
                auto_backup: true,
                backup_keep: 5,
                backups: [
                  { name: "blog-backup-20260801-231125.zip", size: 15728640, mod_time: "2026-08-01T23:11:25Z" },
                  { name: "blog-backup-20260801-231229.zip", size: 16148072, mod_time: "2026-08-01T23:12:29Z" },
                  { name: "blog-backup-20260801-231641.zip", size: 15728640, mod_time: "2026-08-01T23:16:41Z" },
                  { name: "blog-backup-20260801-232819.zip", size: 15938355, mod_time: "2026-08-01T23:28:19Z" },
                  { name: "blog-backup-20260801-233431.zip", size: 15728640, mod_time: "2026-08-01T23:34:31Z" },
                  { name: "blog-backup-20260801-235240.zip", size: 16148072, mod_time: "2026-08-01T23:35:30Z" },
                  { name: "blog-backup-20260801-235240.zip", size: 15728640, mod_time: "2026-08-01T23:52:40Z" },
                  { name: "blog-backup-20260802-000945.zip", size: 15728640, mod_time: "2026-08-02T00:09:45Z" },
                ],
              })
            );
            return;
          }
          next();
        };
        // 插入到 proxy middleware（route: "/api"）之前，确保 mock 先拦截
        const stack = server.middlewares.stack;
        const idx = stack.findIndex((l: any) => l.route === "/api");
        if (idx !== -1) {
          stack.splice(idx, 0, { route: "/api/v1", handle: mock });
        }
      },
    },
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/uploads": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});