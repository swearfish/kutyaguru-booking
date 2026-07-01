import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), wails("./bindings")],
  // This is a Wails desktop app: assets are served from local disk, so bundle
  // size has no runtime cost. Raise the limit to silence the cosmetic warning.
  build: {
    chunkSizeWarningLimit: 2000,
  },
});
