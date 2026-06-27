// useFavicon — keep <link rel="icon"> in sync with the aggregate session status.
//
// Owns its subscription imperatively in a single mount-only effect:
//   - zustand.subscribe diffs the derived top kind and repaints only on
//     transition. The caller does NOT re-render on every status tick (which
//     a React selector would force) — the hook is side-effect-only.
//   - The data-theme MutationObserver lives in the same effect, with a
//     1-rAF guard so getComputedStyle is called after the browser has
//     applied the new [data-theme] selector (mirrors useXtermTheme).
//
// Colors are resolved via a hidden probe element wearing
// `.session-status-<kind>`, so view.css remains the single source of truth
// for status → CSS variable mapping.

import { useLayoutEffect } from "react";
import type { StatusKind } from "../components/StatusIcon";
import {
  FALLBACK_COLORS,
  type FaviconColors,
  PRIORITY,
  buildFaviconSvg,
  selectTopStatus,
  svgToDataUri,
} from "../lib/favicon";
import { useDaemonStore } from "../store/daemon";

const FAVICON_ID = "app-favicon";

function readColors(): FaviconColors {
  const probe = document.createElement("span");
  probe.style.display = "none";
  document.body.appendChild(probe);
  const out = { ...FALLBACK_COLORS };
  try {
    for (const kind of PRIORITY) {
      probe.className = `session-status-${kind}`;
      const c = getComputedStyle(probe).color.trim();
      if (c) out[kind] = c;
    }
  } finally {
    probe.remove();
  }
  return out;
}

function paint(kind: StatusKind): void {
  const link = document.getElementById(FAVICON_ID) as HTMLLinkElement | null;
  if (!link) return;
  link.href = svgToDataUri(buildFaviconSvg(kind, readColors()));
}

export function useFavicon(): void {
  useLayoutEffect(() => {
    const repaint = (): void => paint(selectTopStatus(useDaemonStore.getState().sessions));
    repaint();

    let lastKind = selectTopStatus(useDaemonStore.getState().sessions);
    const unsubscribe = useDaemonStore.subscribe((s) => {
      const next = selectTopStatus(s.sessions);
      if (next !== lastKind) {
        lastKind = next;
        paint(next);
      }
    });

    // attributeFilter guarantees only data-theme records reach this callback.
    let rafId: number | null = null;
    const observer = new MutationObserver(() => {
      if (rafId !== null) cancelAnimationFrame(rafId);
      rafId = requestAnimationFrame(() => {
        repaint();
        rafId = null;
      });
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    return () => {
      unsubscribe();
      observer.disconnect();
      if (rafId !== null) cancelAnimationFrame(rafId);
    };
  }, []);
}
