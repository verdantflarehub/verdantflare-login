import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 4174,
    proxy: {
      "/api/auth": "http://localhost:8088",
      "/healthz": "http://localhost:8088",
    },
  },
  preview: { port: 4174 },
});
