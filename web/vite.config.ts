import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Vite owns this folder and empties it on every build. The Makefile syncs
  // the result into internal/webui/dist, which the Go binary embeds and which
  // keeps a committed placeholder so a fresh clone compiles.
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    port: 5173,
    proxy: { "/api": "http://127.0.0.1:8080" },
  },
});
