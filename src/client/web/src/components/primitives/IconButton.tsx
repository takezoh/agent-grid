// IconButton — the single icon-button primitive (ADR 0075, WCAG 2.5.5 / 4.1.2).
//
// Every mobile FAB (KeyboardFAB / JumpToLatestFAB / mode toggles) and every
// FontSizeControl button wraps this primitive thinly so the cross-cutting
// affordances live in exactly one place:
//   - renders a real <button type="button"> (not a div) so keyboard / a11y work
//   - guarantees a 44×44px touch target via icon-button.css (WCAG 2.5.5)
//   - requires a non-empty aria-label (WCAG 4.1.2) — enforced at runtime + type
//   - supports optional aria-pressed for toggle FABs
//   - calls preventDefault() on pointerdown so tapping the FAB does NOT steal
//     focus from the terminal helper textarea. This is the load-bearing
//     mechanism KeyboardFAB relies on (FR-MOB-FAB-PD-001) and is supplied here
//     so no FAB re-implements it.
//
// Visual language matches .command-search-trigger / SessionDrawer close: it adds
// no ad-hoc colors or sizes, only existing theme tokens (ADR 0059).

import type { ButtonHTMLAttributes, JSX, PointerEvent, ReactNode } from "react";
import "../../css/icon-button.css";

export interface IconButtonProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label" | "type"> {
  /** Required, non-empty accessible name (WCAG 4.1.2). */
  "aria-label": string;
  /** Optional toggle state for mode FABs. */
  "aria-pressed"?: boolean;
  /** Icon glyph / element. Rendered before any children. */
  icon?: ReactNode;
  children?: ReactNode;
}

export function IconButton({
  "aria-label": ariaLabel,
  "aria-pressed": ariaPressed,
  icon,
  children,
  className,
  onPointerDown,
  ...rest
}: IconButtonProps): JSX.Element {
  if (typeof ariaLabel !== "string" || ariaLabel.trim() === "") {
    // Fail loudly: a silent empty label ships an unlabeled control to screen
    // readers (the UAC-024 counterexample). Surfacing it at call time keeps the
    // contract un-bypassable.
    throw new Error("IconButton: a non-empty aria-label is required (WCAG 4.1.2).");
  }

  const handlePointerDown = (event: PointerEvent<HTMLButtonElement>): void => {
    // Stop the button from grabbing focus on tap so the terminal helper
    // textarea keeps the soft keyboard open (FR-MOB-FAB-PD-001).
    event.preventDefault();
    onPointerDown?.(event);
  };

  const cls = ["icon-button", className].filter(Boolean).join(" ");

  return (
    <button
      {...rest}
      type="button"
      className={cls}
      aria-label={ariaLabel}
      aria-pressed={ariaPressed}
      onPointerDown={handlePointerDown}
    >
      {icon}
      {children}
    </button>
  );
}
