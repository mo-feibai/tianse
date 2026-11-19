import { readFileSync } from "node:fs";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import wails from "@wailsio/runtime/plugins/vite";

// The version has one home: the VERSION file at the repository root. The Go
// build gets it through -ldflags, the installer through /DAppVersion, and the
// UI through this define. Keeping a second copy in a .vue file is how the two
// ends drift apart.
function readVersion(): string {
  try {
    return readFileSync(new URL("../VERSION", import.meta.url), "utf8").trim();
  } catch {
    return "dev";
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(readVersion()),
  },
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [vue(), wails("./bindings")],
});
