/**
 * codeMirrorTheme — FR-037
 *
 * Builds CodeMirror 6 theme extensions from semantic CSS tokens so the editor
 * follows both dark and light themes (same signal path as useXtermTheme).
 */

import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import type { Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { tags as t } from "@lezer/highlight";

const EDITOR_TOKEN_NAMES = [
  "--editor-fg",
  "--editor-bg",
  "--editor-gutter-fg",
  "--editor-gutter-bg",
  "--editor-line-highlight",
  "--editor-selection",
  "--editor-cursor",
  "--diff-add-bg",
  "--diff-remove-bg",
  "--status-info-text",
  "--status-warn-text",
  "--accent",
] as const;

type EditorTokenName = (typeof EDITOR_TOKEN_NAMES)[number];

function readToken(style: CSSStyleDeclaration, name: EditorTokenName): string | undefined {
  const value = style.getPropertyValue(name).trim();
  return value || undefined;
}

function readEditorTokens(): Record<EditorTokenName, string> | null {
  const style = getComputedStyle(document.documentElement);
  const tokens = {} as Record<EditorTokenName, string>;
  for (const name of EDITOR_TOKEN_NAMES) {
    const value = readToken(style, name);
    if (!value) return null;
    tokens[name] = value;
  }
  return tokens;
}

/** Build a CodeMirror theme extension from current CSS custom properties. */
export function buildCodeMirrorTheme(): Extension {
  const tok = readEditorTokens();
  if (!tok) {
    return EditorView.theme({
      "&": { height: "100%", fontFamily: "var(--font-mono, monospace)", fontSize: "0.85rem" },
      ".cm-scroller": { overflow: "auto", maxHeight: "100%" },
      ".cm-content": { whiteSpace: "pre-wrap", wordBreak: "break-word" },
    });
  }

  const highlight = HighlightStyle.define([
    { tag: t.keyword, color: tok["--editor-cursor"] },
    { tag: [t.string, t.special(t.string)], color: tok["--status-info-text"] },
    { tag: [t.number, t.bool, t.null], color: tok["--status-warn-text"] },
    { tag: [t.comment, t.lineComment, t.blockComment], color: tok["--editor-gutter-fg"] },
    { tag: [t.variableName, t.propertyName], color: tok["--editor-fg"] },
    { tag: [t.typeName, t.className], color: tok["--accent"] },
  ]);

  return [
    EditorView.theme(
      {
        "&": {
          height: "100%",
          fontFamily: "var(--font-mono, monospace)",
          fontSize: "0.85rem",
          color: tok["--editor-fg"],
          backgroundColor: tok["--editor-bg"],
        },
        ".cm-scroller": {
          overflow: "auto",
          maxHeight: "100%",
          backgroundColor: tok["--editor-bg"],
        },
        ".cm-content": {
          whiteSpace: "pre-wrap",
          wordBreak: "break-word",
          caretColor: tok["--editor-cursor"],
        },
        ".cm-gutters": {
          backgroundColor: tok["--editor-gutter-bg"],
          color: tok["--editor-gutter-fg"],
          borderRight: "1px solid var(--border-hairline)",
        },
        ".cm-activeLine": { backgroundColor: tok["--editor-line-highlight"] },
        ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": {
          backgroundColor: `${tok["--editor-selection"]} !important`,
        },
        "&.cm-focused .cm-cursor": { borderLeftColor: tok["--editor-cursor"] },
      },
      { dark: document.documentElement.dataset.theme !== "light" },
    ),
    syntaxHighlighting(highlight),
  ];
}
