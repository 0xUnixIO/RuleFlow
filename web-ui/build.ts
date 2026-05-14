import { $ } from "bun";
import { cpSync, mkdirSync, existsSync } from "fs";

const isWatch = process.argv.includes("--watch");
const isMinify = process.argv.includes("--minify");

mkdirSync("dist", { recursive: true });

// Build JS/TSX
const result = await Bun.build({
  entrypoints: ["./src/main.tsx"],
  outdir: "./dist",
  splitting: true,
  minify: isMinify,
  target: "browser",
  format: "esm",
  naming: {
    entry: "assets/[name]-[hash].[ext]",
    chunk: "assets/[name]-[hash].[ext]",
    asset: "assets/[name]-[hash].[ext]",
  },
  define: {
    "process.env.NODE_ENV": isMinify ? '"production"' : '"development"',
  },
});

if (!result.success) {
  console.error("❌ Build failed:");
  for (const log of result.logs) {
    console.error(log);
  }
  process.exit(1);
}

// Find the entry JS file
const entryFile = result.outputs.find((o) => o.kind === "entry-point");
const jsPath = entryFile
  ? "/" + entryFile.path.split("/dist/")[1]
  : "/assets/main.js";

// Build Tailwind CSS
const minifyFlag = isMinify ? "--minify" : "";
await $`bunx @tailwindcss/cli -i src/index.css -o dist/assets/index.css ${minifyFlag}`.quiet();

// Copy public assets
if (existsSync("public")) {
  cpSync("public", "dist", { recursive: true });
}

// Generate index.html
const html = `<!DOCTYPE html>
<html lang="zh-CN" class="dark">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
  <link rel="stylesheet" href="/assets/index.css" />
  <title>RuleFlow</title>
</head>
<body>
  <div id="root"></div>
  <script type="module" src="${jsPath}"></script>
</body>
</html>`;

await Bun.write("dist/index.html", html);

console.log(`✅ Build complete → dist/ (minify: ${isMinify})`);
for (const output of result.outputs) {
  const size = (output.size / 1024).toFixed(1);
  const rel = output.path.split("/dist/")[1] || output.path;
  console.log(`   ${rel} (${size} KB)`);
}
