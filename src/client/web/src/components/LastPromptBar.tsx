// LastPromptBar — 2-3 line header above the TERMINAL content showing the
// most recent user prompt (View.last_user_prompt).
//
// Placement: first flex-column child inside `.terminal-slot` (App.tsx
// terminalSlot), above TerminalPane's `.terminal-host`. The slot's
// visibility toggle (ADR-0065 data-active) hides the bar together with the
// terminal when a log tab is active, so it is a TERMINAL-only header.
//
// Visibility gate: driver present in LAST_PROMPT_DRIVERS (lib/lastPrompt.ts)
// — claude / codex / gemini / shell. Grok and generic sessions (root_driver
// is the command's first token) fail the whitelist and render no bar. The
// bar keeps a fixed 3-line-high box even while the prompt is empty so xterm
// geometry does not jitter on every prompt submit.

import type { JSX } from "react";
import "../css/last-prompt-bar.css";
import { driverShowsLastPrompt } from "../lib/lastPrompt";

export interface LastPromptBarProps {
  /** activeSession.root_driver. Drivers outside the whitelist render no bar. */
  driver: string | null | undefined;
  /** activeSession.view.last_user_prompt. Empty shows a placeholder. */
  prompt: string | undefined;
}

export function LastPromptBar({ driver, prompt }: LastPromptBarProps): JSX.Element | null {
  if (!driverShowsLastPrompt(driver)) return null;
  const text = prompt?.trim() ?? "";

  return (
    <div className="last-prompt-bar" data-driver={driver ?? ""} aria-label="Last user prompt">
      <span className="last-prompt-bar__label" aria-hidden="true">
        PROMPT
      </span>
      {text ? (
        <span className="last-prompt-bar__text">{text}</span>
      ) : (
        <span className="last-prompt-bar__text last-prompt-bar__text--empty">No prompt yet</span>
      )}
    </div>
  );
}
