// Per-driver visibility of the Last User Prompt terminal header.
//
// The driver-name → visible table is hardcoded web-side, mirroring the
// DRIVER_SHORTCUTS pattern (lib/driverShortcuts.ts). This is a whitelist by
// necessity: generic-driver sessions carry the command's first token (e.g.
// "bash", "python") as root_driver — there is no literal "generic" on the
// wire — so "hide for generic and grok" can only be expressed as "show for
// the known prompt-capable drivers".

export const LAST_PROMPT_DRIVERS: ReadonlySet<string> = new Set([
  "claude",
  "codex",
  "gemini",
  "shell",
]);

export function driverShowsLastPrompt(driver: string | null | undefined): boolean {
  if (!driver) return false;
  return LAST_PROMPT_DRIVERS.has(driver);
}
