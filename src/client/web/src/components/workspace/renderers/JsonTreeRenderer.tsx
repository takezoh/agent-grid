import { type ReactNode, useEffect, useMemo, useState } from "react";

const PARSE_TIMEOUT_MS = 300;
const BATCH_SIZE = 100;

export type JsonTreeRendererProps = {
  source: string;
};

type ParseOutcome = "loading" | "ok" | "parse_error" | "timeout";

type JsonNodeProps = {
  name: string;
  value: unknown;
  depth: number;
};

function JsonNode({ name, value, depth }: JsonNodeProps): ReactNode {
  const [open, setOpen] = useState(depth < 2);
  const isObject = value !== null && typeof value === "object";
  const isArray = Array.isArray(value);
  const entries = isObject
    ? isArray
      ? (value as unknown[]).map((v, i) => [String(i), v] as const)
      : Object.entries(value as Record<string, unknown>)
    : [];

  if (!isObject) {
    return (
      <div className="json-tree__leaf" style={{ paddingLeft: depth * 12 }}>
        <span className="json-tree__key">{name}: </span>
        <span className="json-tree__value">{JSON.stringify(value)}</span>
      </div>
    );
  }

  return (
    <div className="json-tree__branch" style={{ paddingLeft: depth * 12 }}>
      <button type="button" className="json-tree__toggle" onClick={() => setOpen((v) => !v)}>
        {open ? "▼" : "▶"} {name}
        {isArray ? ` [${entries.length}]` : ` {${entries.length}}`}
      </button>
      {open &&
        entries.map(([k, v]) => (
          <JsonNode key={`${name}-${k}`} name={k} value={v} depth={depth + 1} />
        ))}
    </div>
  );
}

export function JsonTreeRenderer({ source }: JsonTreeRendererProps): ReactNode {
  const [outcome, setOutcome] = useState<ParseOutcome>("loading");
  const [parsed, setParsed] = useState<unknown>(null);
  const [visibleKeys, setVisibleKeys] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setOutcome("loading");
    const timer = window.setTimeout(() => {
      if (!cancelled) setOutcome("timeout");
    }, PARSE_TIMEOUT_MS);

    window.setTimeout(() => {
      if (cancelled) return;
      try {
        const value = JSON.parse(source) as unknown;
        window.clearTimeout(timer);
        setParsed(value);
        setOutcome("ok");
        setVisibleKeys(0);
      } catch {
        window.clearTimeout(timer);
        setOutcome("parse_error");
      }
    }, 0);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [source]);

  const topEntries = useMemo(() => {
    if (parsed === null || typeof parsed !== "object") return [];
    if (Array.isArray(parsed)) return parsed.map((v, i) => [String(i), v] as const);
    return Object.entries(parsed as Record<string, unknown>);
  }, [parsed]);

  useEffect(() => {
    if (outcome !== "ok") return;
    if (visibleKeys >= topEntries.length) return;
    const id = window.requestAnimationFrame(() => {
      setVisibleKeys((n) => Math.min(n + BATCH_SIZE, topEntries.length));
    });
    return () => window.cancelAnimationFrame(id);
  }, [outcome, visibleKeys, topEntries.length]);

  if (outcome === "loading") {
    return <div className="workspace-renderer workspace-renderer--loading">Parsing JSON…</div>;
  }

  if (outcome !== "ok") {
    const reason = outcome === "timeout" ? "timeout" : "parse-error";
    return (
      <div className="workspace-renderer workspace-renderer--fallback" data-reason={reason}>
        {/* biome-ignore lint/a11y/useSemanticElements: parse fallback banner; <output> implies form association */}
        <div className="workspace-renderer__banner" role="status">
          {outcome === "timeout"
            ? "JSON parse timed out; showing raw text."
            : "Invalid JSON; showing raw text."}
        </div>
        <pre className="workspace-renderer__raw">{source}</pre>
      </div>
    );
  }

  return (
    <div className="workspace-renderer workspace-renderer--json" data-testid="json-tree-renderer">
      {topEntries.slice(0, visibleKeys).map(([k, v]) => (
        <JsonNode key={k} name={k} value={v} depth={0} />
      ))}
    </div>
  );
}
