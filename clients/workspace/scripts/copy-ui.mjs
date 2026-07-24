import { copyFileSync, cpSync, mkdirSync, rmSync } from "node:fs";
import { resolve } from "node:path";

const source = resolve(import.meta.dirname, "../../ui/dist");
const destination = resolve(import.meta.dirname, "../dist/ui");

rmSync(destination, { recursive: true, force: true });
mkdirSync(destination, { recursive: true });
cpSync(source, destination, { recursive: true });

copyFileSync(
  resolve(import.meta.dirname, "../src/preload/electron-preload.cjs"),
  resolve(import.meta.dirname, "../dist/preload/electron-preload.cjs"),
);
