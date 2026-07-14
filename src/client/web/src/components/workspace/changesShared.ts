import type { FileEventKind } from "../../store/workspaceActivity";

/** Operation glyphs per kind (FR-017). */
export const KIND_GLYPH: Record<FileEventKind, string> = {
  read: "R",
  create: "A",
  edit: "M",
  delete: "D",
};
