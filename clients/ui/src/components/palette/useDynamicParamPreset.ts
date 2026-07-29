// useDynamicParamPreset — small hook that materializes the leading-option
// preset for a `kind: 'dynamic-options'` ParamDef.
//
// Why a dedicated hook (not inline useEffect in ParamSelectPhase):
//   1. ParamSelectPhase.tsx is right at the 500-line ceiling (AGENTS.md);
//      pulling the effect out keeps the component under the limit and
//      colocates the dependency contract with the helper that owns it.
//   2. The leading-preset rule is dynamic-options-specific by spec
//      (palette-bugfix FR-A1) — static-options and text MUST NOT fire it.
//      Centralizing the gate in a hook makes that rule a single function
//      rather than a scatter of `if (param.kind === ...)` across the
//      component.
//
// Behavior: when (a) param.kind === 'dynamic-options', (b) options has at
// least one entry, and (c) paramValues[param.id] is still undefined, call
// setParam(param.id, options[0].value). Subsequent renders are no-ops
// because (c) flips false once setParam lands. The effect deps are
// (materializeKey, options) so a daemon snapshot diff that produces a
// fresh options array re-runs the gate; consumers should memoize the
// options array (e.g. via useMemo on (kind, materializeKey, snapshot))
// so a render with identical content doesn't churn the effect.

import { useEffect } from "react";
import type { ParamDef, ParamOption } from "../../lib/tools";

export function useDynamicParamPreset(
  param: ParamDef,
  options: ParamOption[] | null,
  currentValue: unknown,
  setParam: (key: string, value: unknown) => void,
): void {
  // Re-run when currentValue is cleared (e.g. selectProject setParams wipe) so the
  // leading-option preset is restored. A defined currentValue short-circuits, so a
  // later user choice is never clobbered. materializeKey is included so a source
  // change re-gates the leading option even when the options array identity is stable.
  const materializeKey = param.kind === "dynamic-options" ? param.materializeKey : "";
  useEffect(() => {
    if (param.kind !== "dynamic-options") return;
    if (options === null || options.length === 0) return;
    if (currentValue !== undefined) return;
    const first = options[0];
    if (first === undefined) return;
    setParam(param.id, first.value);
    // materializeKey intentionally referenced for dep semantics.
    void materializeKey;
  }, [materializeKey, options, currentValue, param.kind, param.id, setParam]);
}
