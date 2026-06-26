// ParamListbox — listbox variant rendered by ParamSelectPhase for
// kind: 'static-options' and kind: 'dynamic-options' (N>=1) ParamDefs.
//
// Filter-as-you-type (web-ui-fixes 2026-06-24): each listbox carries its
// own combobox input that filters options by case-insensitive substring
// match. The input gets focus when `focused` is true so a user landing on
// the Project / Command param can immediately start typing to narrow the
// picker, then press Enter to commit. ArrowUp / ArrowDown navigate the
// VISIBLE list, so navigation never lands on a hidden row.
//
// Selection model:
//   - `value` is the persisted store value (param.id).
//   - currentIdx is the visible-list index of `value`. When the filter
//     hides the selected option it's -1; Enter then auto-promotes the
//     first visible option before calling onEnter (matches the user's
//     expectation: "type to filter, press Enter to take the obvious row").
//   - mousedown commits the clicked option AND triggers onEnter (mouse
//     click is the user saying "this one, advance"). preventDefault keeps
//     the combobox input from losing focus.

import { useEffect, useMemo, useRef, useState } from "react";
import type { JSX } from "react";
import type { ParamOption } from "../../lib/tools";

export interface ParamListboxProps {
  paramId: string;
  label: string;
  options: ReadonlyArray<ParamOption>;
  value: unknown;
  focused: boolean;
  disabled: boolean;
  composing: boolean;
  onSelect: (v: unknown) => void;
  onEnter: () => void;
  onCompositionStart: () => void;
  onCompositionEnd: () => void;
  // inputRef: ParamSelectPhase passes its commandInputRef here for the
  // command listbox so the chip-visibility focus fallback (FR-022) can
  // .focus() the filter input when a chip's visibility flips off.
  inputRef?: React.Ref<HTMLInputElement>;
}

function optionIndexOf(options: ReadonlyArray<ParamOption>, value: unknown): number {
  if (value === undefined) return -1;
  for (let i = 0; i < options.length; i++) {
    const opt = options[i];
    if (opt !== undefined && opt.value === value) return i;
  }
  return -1;
}

function filterOptions(
  options: ReadonlyArray<ParamOption>,
  filter: string,
): ReadonlyArray<ParamOption> {
  const q = filter.trim().toLowerCase();
  if (q === "") return options;
  return options.filter((o) => o.label.toLowerCase().includes(q));
}

export function ParamListbox(props: ParamListboxProps): JSX.Element {
  const {
    paramId,
    label,
    options,
    value,
    focused,
    disabled,
    composing,
    onSelect,
    onEnter,
    onCompositionStart,
    onCompositionEnd,
  } = props;

  const localInputRef = useRef<HTMLInputElement | null>(null);
  const [filter, setFilter] = useState("");
  const filterInputId = `palette-param-${paramId}-input`;

  // Compose localInputRef with the optional external inputRef so both the
  // internal focused-effect and the caller's ref (e.g. ParamSelectPhase's
  // commandInputRef for FR-022 chip-visibility focus fallback) land on the
  // same input element. We use a callback ref so React can clean up
  // gracefully across mount/unmount cycles.
  const setInputRef = (el: HTMLInputElement | null) => {
    localInputRef.current = el;
    const ext = props.inputRef;
    if (typeof ext === "function") ext(el);
    else if (ext && typeof ext === "object")
      (ext as React.MutableRefObject<HTMLInputElement | null>).current = el;
  };

  const visible = useMemo(() => filterOptions(options, filter), [options, filter]);
  const currentIdx = optionIndexOf(visible, value);

  // FR-029-style focus: when this param becomes the focused row, drop focus
  // into the filter input so the user can type immediately. Without this,
  // the listbox sits passive and selection looks broken — the visible
  // motivation behind the "project list not selectable" bug report.
  useEffect(() => {
    if (!focused) return;
    if (disabled) return;
    const el = localInputRef.current;
    if (el !== null) el.focus();
  }, [focused, disabled]);

  // Reset the filter when the param loses focus so re-entering doesn't
  // surprise the user with a stale narrow set.
  useEffect(() => {
    if (!focused) setFilter("");
  }, [focused]);

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (composing) return;
    if (e.nativeEvent.isComposing) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      if (visible.length === 0) return;
      const nextIdx = currentIdx + 1 >= visible.length ? 0 : currentIdx + 1;
      const nextOpt = visible[nextIdx];
      if (nextOpt !== undefined) onSelect(nextOpt.value);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (visible.length === 0) return;
      const nextIdx = currentIdx <= 0 ? visible.length - 1 : currentIdx - 1;
      const nextOpt = visible[nextIdx];
      if (nextOpt !== undefined) onSelect(nextOpt.value);
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      if (visible.length === 0) return;
      if (currentIdx < 0) {
        const first = visible[0];
        if (first !== undefined) onSelect(first.value);
      }
      onEnter();
      return;
    }
  };

  return (
    <fieldset
      className={`palette-param ${focused ? "focused" : ""}`}
      data-param-id={paramId}
      aria-label={label}
    >
      <label className="palette-param-label" htmlFor={filterInputId}>
        {label}
      </label>
      <input
        ref={setInputRef}
        id={filterInputId}
        className="palette-param-filter"
        type="text"
        role="combobox"
        aria-controls={`palette-param-${paramId}`}
        aria-expanded="true"
        aria-autocomplete="list"
        aria-activedescendant={
          currentIdx >= 0 ? `palette-param-${paramId}-opt-${currentIdx}` : undefined
        }
        value={filter}
        disabled={disabled}
        placeholder={`Filter ${label.toLowerCase()}...`}
        onChange={(e) => {
          if (composing) return;
          setFilter(e.target.value);
        }}
        onKeyDown={onKeyDown}
        onCompositionStart={onCompositionStart}
        onCompositionEnd={onCompositionEnd}
        data-testid={`palette-param-${paramId}-filter`}
      />
      <div
        id={`palette-param-${paramId}`}
        className="palette-param-listbox"
        // biome-ignore lint/a11y/useSemanticElements: ARIA listbox pattern uses div+role=listbox; <select> cannot host aria-activedescendant
        role="listbox"
        tabIndex={-1}
        aria-activedescendant={
          currentIdx >= 0 ? `palette-param-${paramId}-opt-${currentIdx}` : undefined
        }
        aria-disabled={disabled || undefined}
      >
        {visible.length === 0 ? (
          <div className="palette-param-listbox__empty" role="presentation">
            No matches
          </div>
        ) : (
          visible.map((opt, i) => {
            const selected = i === currentIdx;
            const optKey = `${paramId}-${opt.label}-${i}`;
            return (
              // biome-ignore lint/a11y/useFocusableInteractive: focus stays on the combobox input via aria-activedescendant; options are not individually tabbable
              <div
                key={optKey}
                id={`palette-param-${paramId}-opt-${i}`}
                // biome-ignore lint/a11y/useSemanticElements: ARIA listbox uses div+role=option; <option> only works inside <select>
                role="option"
                aria-selected={selected}
                className={`palette-param-option ${selected ? "selected" : ""}`}
                onMouseDown={(e) => {
                  e.preventDefault();
                  if (disabled) return;
                  onSelect(opt.value);
                  onEnter();
                }}
              >
                {opt.label}
              </div>
            );
          })
        )}
      </div>
    </fieldset>
  );
}
