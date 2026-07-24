import { closeSync, openSync } from "node:fs";

closeSync(openSync(new URL("../dist/.gitkeep", import.meta.url), "a"));
