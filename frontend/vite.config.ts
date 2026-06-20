import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import path from "node:path";

// https://vite.dev/config/
// Note: the optional @wailsio/runtime typed-events Vite plugin is omitted — it is
// incompatible with Vite 8's Rolldown bundler, and this app uses string-named
// runtime events rather than the generated typed-event API.
export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: {
      "~": path.resolve(__dirname, "src"),
    },
  },
  server: {
    port: 9245,
    strictPort: true,
  },
  build: {
    target: "esnext",
  },
});
