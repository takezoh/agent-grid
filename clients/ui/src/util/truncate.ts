/**
 * Character-count truncation for user-supplied labels rendered in
 * unconstrained-width UI surfaces (context menu labels, confirm dialog
 * body text) where CSS text-overflow ellipsis isn't applicable because the
 * container itself must not grow to fit arbitrary title length.
 */

export const SESSION_LABEL_MAX_LENGTH = 80;

/** Truncates `text` to at most `maxLength` characters, appending an ellipsis. */
export function truncateLabel(text: string, maxLength: number = SESSION_LABEL_MAX_LENGTH): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength - 1).trimEnd()}…`;
}
