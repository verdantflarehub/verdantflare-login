import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

const centerContextDevProxy = {
  name: "center-context-dev-proxy",
  configureServer(server) {
    server.middlewares.use("/api/control/context", async (request, response, next) => {
      if (request.method !== "GET" || request.url?.split("?")[0] !== "/") return next();
      try {
        const session = await fetch("http://localhost:8088/api/auth/session", {
          headers: { Cookie: request.headers.cookie || "" },
        });
        if (!session.ok) {
          response.writeHead(session.status, { "Cache-Control": "no-store" });
          response.end();
          return;
        }
        const subject = session.headers.get("X-VF-Login-Subject");
        if (!subject) throw new Error("Login session did not include a subject");
        const context = await fetch("http://localhost:8080/api/control/context", {
          headers: { "X-VF-Login-Subject": subject },
        });
        response.writeHead(context.status, {
          "Cache-Control": "no-store",
          "Content-Type": context.headers.get("Content-Type") || "application/json",
        });
        response.end(Buffer.from(await context.arrayBuffer()));
      } catch {
        response.writeHead(502, { "Cache-Control": "no-store" });
        response.end();
      }
    });
  },
};

export default defineConfig({
  plugins: [vue(), centerContextDevProxy],
  server: {
    port: 4174,
    proxy: {
      "/api/auth": "http://localhost:8088",
      "/healthz": "http://localhost:8088",
    },
  },
  preview: { port: 4174 },
});
