import { defineConfig } from "vite";
import baseConfig from "./vite.config.ts";

export default defineConfig({
  ...baseConfig,
  base: "/uc/",
  build: {
    ...baseConfig.build,
    outDir: "dist-uc",
    emptyOutDir: false,
    rollupOptions: {
      ...baseConfig.build?.rollupOptions,
      input: { index: "uc.html" },
    },
  },
});
