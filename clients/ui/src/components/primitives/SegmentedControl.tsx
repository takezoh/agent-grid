// SegmentedControl.tsx — WAI-ARIA radiogroup pattern primitive (FR-THEME-007 / FR-A11Y-001)
// Provides role='radiogroup' + role='radio' + aria-checked + roving tabindex
// + ArrowLeft/Right/Home/End focus + Space/Enter manual activation.
//
// Styling uses CSS class `.segmented-control` / `.segmented-control__segment`
// (defined in app.css) which wires --row-radius / --row-padding-y/x / --focus-ring
// tokens for theme-aware colours, hover/active/disabled states, and WCAG 2.4.7
// (Focus Visible) via :focus-visible { outline: 2px solid var(--focus-ring) }.
//
// Focus management uses a ref array (not document.getElementById) so that two
// SegmentedControl instances on the same page never collide regardless of idPrefix.
import type { JSX, KeyboardEvent, ReactNode } from "react";
import { useRef } from "react";

export type Segment<T> = {
  value: T;
  label: ReactNode;
  ariaLabel?: string;
};

export type SegmentedControlProps<T> = {
  ariaLabel: string;
  segments: Segment<T>[];
  value: T;
  onChange: (next: T) => void;
  idPrefix?: string;
};

export function SegmentedControl<T>({
  ariaLabel,
  segments,
  value,
  onChange,
  idPrefix = "seg",
}: SegmentedControlProps<T>): JSX.Element {
  // Roving focus via refs — avoids DOM id collision when multiple instances share
  // the default idPrefix or when idPrefix is omitted entirely.
  const buttonRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const currentIndex = segments.findIndex((s) => s.value === value);
  // When value matches no segment (caller bug / prop mismatch), fall back to index 0
  // so the radiogroup remains keyboard-reachable. Always warn — not gated on DEV —
  // so regressions surface in production logs too (primitive bugs should be visible).
  const focusedIndex = currentIndex >= 0 ? currentIndex : 0;
  if (currentIndex < 0) {
    console.warn(
      `[SegmentedControl] value prop does not match any segment. idPrefix="${idPrefix}", value=${String(value)}. Falling back to index 0 for tabIndex to keep the group keyboard-reachable.`,
    );
  }

  function focusSegment(index: number): void {
    buttonRefs.current[index]?.focus();
  }

  function handleKeyDown(e: KeyboardEvent<HTMLButtonElement>, index: number): void {
    switch (e.key) {
      case "ArrowRight": {
        e.preventDefault();
        const nextIndex = (index + 1) % segments.length;
        focusSegment(nextIndex);
        break;
      }
      case "ArrowLeft": {
        e.preventDefault();
        const prevIndex = (index - 1 + segments.length) % segments.length;
        focusSegment(prevIndex);
        break;
      }
      case "Home": {
        e.preventDefault();
        focusSegment(0);
        break;
      }
      case "End": {
        e.preventDefault();
        focusSegment(segments.length - 1);
        break;
      }
      case " ":
      case "Enter": {
        e.preventDefault();
        const seg = segments[index];
        // index comes from segments.map so out-of-range is impossible in normal use;
        // the guard exists to satisfy the type checker. Always warn (not DEV-gated)
        // so internal regressions remain visible after future refactors.
        if (seg !== undefined) {
          onChange(seg.value);
        } else {
          console.warn(
            `[SegmentedControl] Space/Enter fired for out-of-range index ${index}. This indicates a bug in SegmentedControl internals.`,
          );
        }
        break;
      }
      default:
        break;
    }
  }

  return (
    <div role="radiogroup" aria-label={ariaLabel} className="segmented-control">
      {segments.map((seg, index) => {
        const isChecked = seg.value === value;
        const isFocused = focusedIndex === index;
        return (
          <button
            key={String(seg.value)}
            id={`${idPrefix}-${index}`}
            ref={(el) => {
              buttonRefs.current[index] = el;
            }}
            type="button"
            // biome-ignore lint/a11y/useSemanticElements: WAI-ARIA radiogroup with button role='radio' is the standard pattern for segmented controls. <input type="radio"> carries implicit form association semantics and its native keyboard contract (auto-activation on focus) conflicts with the manual activation required by FR-THEME-007; the div+button approach gives full control over roving tabindex and Space/Enter manual activation.
            role="radio"
            aria-checked={isChecked}
            aria-label={seg.ariaLabel}
            tabIndex={isFocused ? 0 : -1}
            className="segmented-control__segment"
            onKeyDown={(e) => handleKeyDown(e, index)}
            onClick={() => onChange(seg.value)}
          >
            {seg.label}
          </button>
        );
      })}
    </div>
  );
}
