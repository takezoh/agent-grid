// CognitivePanel — Phase 0 value probe of the Cognitive Workspace
// (plans/cognitive-work-orchestration-ux.md §27 Phase 0).
//
// Lists resume capsules across the fleet with the trust contract applied:
// freshness is always visible, capsules older than their session's latest
// activity are marked stale, and a one-action wrong-flag feeds the §1.4
// capsule-trust criterion. This panel is deliberately minimal — no new
// navigation, no orchestration; the Classic Workspace remains the fallback.

import { useCallback, useEffect, useMemo, useState } from "react";
import { type FleetResumeCapsule, makeCapsulesApi } from "../../api/capsules";
import { formatElapsed, useNow1Hz } from "../../hooks/useNow1Hz";
import { useDaemonStore } from "../../store/daemon";
import "../../css/cognitive.css";

export type CognitivePanelProps = {
  /** True while the Cognitive layer is the visible workspace. */
  active: boolean;
  fetchFn?: typeof fetch;
};

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; capsules: FleetResumeCapsule[]; errors: { serverId: string }[] }
  | { kind: "error"; message: string };

function baseName(path: string): string {
  const trimmed = path.replace(/[/\\]+$/, "");
  const idx = Math.max(trimmed.lastIndexOf("/"), trimmed.lastIndexOf("\\"));
  return idx >= 0 ? trimmed.slice(idx + 1) : trimmed;
}

function parseTime(value: string | undefined): number | null {
  if (!value) return null;
  const ms = Date.parse(value);
  return Number.isNaN(ms) ? null : ms;
}

export function CognitivePanel({ active, fetchFn }: CognitivePanelProps) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [reloadNonce, setReloadNonce] = useState(0);
  const now = useNow1Hz();
  const sessions = useDaemonStore((s) => s.sessions);

  const servers = useMemo(() => {
    const seen = new Map<string, string | undefined>();
    for (const session of sessions) {
      if (session.serverId && !seen.has(session.serverId)) {
        seen.set(session.serverId, session.serverLabel);
      }
    }
    return [...seen.entries()].map(([serverId, serverLabel]) => ({ serverId, serverLabel }));
  }, [sessions]);

  const activityBySessionKey = useMemo(() => {
    const map = new Map<string, string | undefined>();
    for (const session of sessions) {
      map.set(session.id, session.status.changed_at ?? session.created_at);
    }
    return map;
  }, [sessions]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: reloadNonce is the intentional manual-refresh trigger and is not read in the body
  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    setState({ kind: "loading" });
    void makeCapsulesApi(fetchFn)
      .listAll(servers)
      .then((result) => {
        if (cancelled) return;
        setState({ kind: "ready", capsules: result.capsules, errors: result.errors });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState({ kind: "error", message: err instanceof Error ? err.message : String(err) });
      });
    return () => {
      cancelled = true;
    };
  }, [active, fetchFn, servers, reloadNonce]);

  const refresh = useCallback(() => setReloadNonce((n) => n + 1), []);

  const toggleFlag = useCallback(
    (capsule: FleetResumeCapsule) => {
      const nextFlagged = !capsule.flagged_wrong;
      void makeCapsulesApi(fetchFn)
        .flag(capsule.fleetSessionKey, nextFlagged)
        .then(() => {
          setState((prev) => {
            if (prev.kind !== "ready") return prev;
            return {
              ...prev,
              capsules: prev.capsules.map((c) =>
                c.fleetSessionKey === capsule.fleetSessionKey
                  ? { ...c, flagged_wrong: nextFlagged }
                  : c,
              ),
            };
          });
        })
        .catch(() => {
          // Flagging is best effort; the loaded list stays visible.
        });
    },
    [fetchFn],
  );

  return (
    <section className="cognitive-panel" aria-label="cognitive workspace">
      <header className="cognitive-panel__header">
        <div>
          <h2 className="cognitive-panel__title">Resume capsules</h2>
          <p className="cognitive-panel__subtitle">
            Where each session left off — resume without rereading history.
          </p>
        </div>
        <button type="button" className="cognitive-panel__refresh" onClick={refresh}>
          Refresh
        </button>
      </header>
      {state.kind === "loading" && <div className="cognitive-empty">Loading capsules…</div>}
      {state.kind === "error" && (
        <div className="cognitive-empty">Failed to load capsules: {state.message}</div>
      )}
      {state.kind === "ready" && state.capsules.length === 0 && (
        <div className="cognitive-empty">
          <p>No resume capsules yet.</p>
          <p className="cognitive-empty__hint">
            Capsules are generated automatically from session activity once the server has a capsule
            command configured ([driver] capsule_command in settings.toml).
          </p>
        </div>
      )}
      {state.kind === "ready" && state.capsules.length > 0 && (
        <div className="cognitive-list">
          {state.capsules.map((capsule) => (
            <CapsuleCard
              key={capsule.fleetSessionKey}
              capsule={capsule}
              now={now}
              sessionActivityAt={activityBySessionKey.get(capsule.fleetSessionKey)}
              onToggleFlag={toggleFlag}
            />
          ))}
        </div>
      )}
      {state.kind === "ready" && state.errors.length > 0 && (
        <div className="cognitive-panel__errors">
          Some servers could not be reached: {state.errors.map((e) => e.serverId).join(", ")}
        </div>
      )}
    </section>
  );
}

type CapsuleCardProps = {
  capsule: FleetResumeCapsule;
  now: number;
  sessionActivityAt: string | undefined;
  onToggleFlag: (capsule: FleetResumeCapsule) => void;
};

function CapsuleCard({ capsule, now, sessionActivityAt, onToggleFlag }: CapsuleCardProps) {
  const generatedMs = parseTime(capsule.generated_at);
  const activityMs = parseTime(sessionActivityAt);
  // Stale = the session has progressed since this capsule was generated.
  // A capsule must never silently present itself as current (§12.1).
  const stale = generatedMs !== null && activityMs !== null && activityMs > generatedMs;
  const age = generatedMs !== null ? formatElapsed(now - generatedMs) : "";

  return (
    <article
      className="cognitive-card"
      data-stale={stale ? "true" : "false"}
      data-flagged={capsule.flagged_wrong ? "true" : "false"}
    >
      <div className="cognitive-card__meta">
        <span className="cognitive-card__project">
          {capsule.title || baseName(capsule.project)}
        </span>
        {capsule.serverLabel && <span>{capsule.serverLabel}</span>}
        {age && <span>Generated {age} ago</span>}
        {capsule.source_turns ? <span>{capsule.source_turns} turns</span> : null}
        {stale && <span className="cognitive-badge cognitive-badge--stale">Stale</span>}
        {capsule.flagged_wrong && (
          <span className="cognitive-badge cognitive-badge--flagged">Flagged wrong</span>
        )}
      </div>
      {capsule.objective && <h3 className="cognitive-card__objective">{capsule.objective}</h3>}
      {capsule.current_state && <p className="cognitive-card__state">{capsule.current_state}</p>}
      {capsule.next_action && (
        <p className="cognitive-card__next">
          <span className="cognitive-card__label">Next</span> {capsule.next_action}
        </p>
      )}
      {capsule.unresolved && capsule.unresolved.length > 0 && (
        <ul className="cognitive-card__unresolved">
          {capsule.unresolved.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      )}
      {capsule.last_error && (
        <p className="cognitive-card__error">Last generation failed: {capsule.last_error}</p>
      )}
      <div className="cognitive-card__actions">
        <button
          type="button"
          className="cognitive-card__flag"
          onClick={() => onToggleFlag(capsule)}
        >
          {capsule.flagged_wrong ? "Clear wrong-flag" : "Mark as wrong"}
        </button>
      </div>
    </article>
  );
}
