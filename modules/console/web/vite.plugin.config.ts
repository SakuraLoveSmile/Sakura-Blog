import { defineConfig } from "vite";

// Plugin bundles are built separately from the host Console. Shared host
// dependencies stay external and are resolved by the running Console.
export default defineConfig({
  build: {
    lib: {
      entry: "src/plugin-entry.ts",
      formats: ["es"],
      fileName: "index",
    },
    rollupOptions: {
      external: [
        "vue",
        "vue-router",
        "pinia",
        "@iconify/vue",
        "@blog/api-client",
        "@blog/components",
        "@blog/editor",
        "@blog/shared",
      ],
    },
    outDir: "dist-plugin",
    emptyOutDir: true,
  },
});
