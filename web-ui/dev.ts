import { watch } from "fs";
import { join } from "path";
import { $ } from "bun";

const API_TARGET = process.env.API_URL || "http://localhost:8080";
const PORT = parseInt(process.env.PORT || "3000");

// Initial build
console.log("🔨 Building...");
await $`bun run build.ts`.quiet();

// Watch src/ and rebuild
let buildTimer: ReturnType<typeof setTimeout> | null = null;
const rebuild = () => {
  if (buildTimer) clearTimeout(buildTimer);
  buildTimer = setTimeout(async () => {
    console.log("🔄 Rebuilding...");
    try {
      await $`bun run build.ts`.quiet();
      console.log("✅ Rebuild complete");
    } catch {
      console.error("❌ Rebuild failed");
    }
  }, 200);
};

watch(join(import.meta.dir, "src"), { recursive: true }, rebuild);
watch(join(import.meta.dir, "public"), { recursive: true }, rebuild);

// Proxy paths to API
const proxyPaths = [
  "/api/",
  "/login",
  "/logout",
  "/subscribe",
  "/convert",
  "/health",
  "/version",
  "/rulesets/",
];

function shouldProxy(pathname: string): boolean {
  return proxyPaths.some((p) => pathname.startsWith(p));
}

const server = Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);

    // Proxy API requests
    if (shouldProxy(url.pathname)) {
      const target = `${API_TARGET}${url.pathname}${url.search}`;
      const headers = new Headers(req.headers);
      headers.set("host", new URL(API_TARGET).host);
      try {
        return await fetch(target, {
          method: req.method,
          headers,
          body: req.body,
          redirect: "manual",
        });
      } catch {
        return new Response("API proxy error", { status: 502 });
      }
    }

    // Serve static files from dist/
    const distDir = join(import.meta.dir, "dist");
    let filePath = join(distDir, url.pathname);

    let file = Bun.file(filePath);
    if (await file.exists()) {
      return new Response(file);
    }

    // SPA fallback → index.html
    file = Bun.file(join(distDir, "index.html"));
    return new Response(file);
  },
});

console.log(`🚀 Dev server: http://localhost:${server.port}`);
console.log(`📡 API proxy → ${API_TARGET}`);
console.log(`👀 Watching src/ for changes...`);
