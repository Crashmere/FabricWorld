import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
export default defineConfig({
  plugins: [vue()],
  base: "/fabricworld/",
  server: {
    proxy: {
      "/fabricworld/api": {
        target: "http://127.0.0.1:18082",
        rewrite: (p) => p.replace("/fabricworld", ""),
      },
      "/fabricworld/media": {
        target: "http://127.0.0.1:18082",
        rewrite: (p) => p.replace("/fabricworld", ""),
      },
    },
  },
});
