import {mergeConfig} from "vite";
import base from "./vite.config.ts";

// Temporary: the sync check against a scratch backend on 6699. Delete after testing.
export default mergeConfig(base, {
  cacheDir: "node_modules/.vite-sync-scratch",
  server: {port: 5174, strictPort: true, proxy: {"/api": {target: "http://127.0.0.1:6699", changeOrigin: true}}},
});
