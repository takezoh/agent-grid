import react from "@vitejs/plugin-react-swc";
import { defineConfig } from "vite";

export default defineConfig({
  // The same build is loaded from file: by the packaged Electron Workspace.
  base: "./",
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    assetsInlineLimit: 0,
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        // 単一 chunk 維持(ADR 0019)
        manualChunks: undefined,
        inlineDynamicImports: true,
      },
    },
  },
});
