import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import type { Pluggable } from "unified";

/** Rehype-sanitize schema: default plus code-fence class allowlist. */
export const markdownSanitizeSchema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    code: [...(defaultSchema.attributes?.code ?? []), "className"],
    span: [...(defaultSchema.attributes?.span ?? []), "className"],
  },
};

export const rehypeSanitizePlugin = rehypeSanitize(markdownSanitizeSchema) as unknown as Pluggable;

/** Forbidden DOM tokens that must never appear after sanitization. */
export const FORBIDDEN_MARKDOWN_SELECTORS =
  "script,[onclick],a[href^='javascript'],a[href^='data'],img[src^='http']";

export function containsForbiddenMarkdownTokens(root: ParentNode): boolean {
  return root.querySelector(FORBIDDEN_MARKDOWN_SELECTORS) !== null;
}
