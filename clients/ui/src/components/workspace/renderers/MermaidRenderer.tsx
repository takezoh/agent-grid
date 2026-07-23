import mermaid from "mermaid";
import { type ReactNode, useEffect, useId, useState } from "react";

const PARSE_TIMEOUT_MS = 300;

export type MermaidRendererProps = {
  source: string;
};

type RenderOutcome = "loading" | "ok" | "parse_error" | "timeout";

export function MermaidRenderer({ source }: MermaidRendererProps): ReactNode {
  const id = useId().replace(/:/g, "");
  const [outcome, setOutcome] = useState<RenderOutcome>("loading");
  const [svg, setSvg] = useState("");

  useEffect(() => {
    let cancelled = false;
    setOutcome("loading");
    const timer = window.setTimeout(() => {
      if (!cancelled) setOutcome("timeout");
    }, PARSE_TIMEOUT_MS);

    void (async () => {
      try {
        mermaid.initialize({ startOnLoad: false, securityLevel: "strict" });
        const { svg: rendered } = await mermaid.render(`mermaid-${id}`, source);
        if (cancelled) return;
        window.clearTimeout(timer);
        setSvg(rendered);
        setOutcome("ok");
      } catch {
        if (cancelled) return;
        window.clearTimeout(timer);
        setOutcome("parse_error");
      }
    })();

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [source, id]);

  if (outcome === "ok") {
    return (
      <div
        className="workspace-renderer workspace-renderer--mermaid"
        data-testid="mermaid-renderer"
        // biome-ignore lint/security/noDangerouslySetInnerHtml: mermaid SVG output
        dangerouslySetInnerHTML={{ __html: svg }}
      />
    );
  }

  if (outcome === "loading") {
    return <div className="workspace-renderer workspace-renderer--loading">Rendering diagram…</div>;
  }

  const reason = outcome === "timeout" ? "timeout" : "parse-error";
  return (
    <div className="workspace-renderer workspace-renderer--fallback" data-reason={reason}>
      {/* biome-ignore lint/a11y/useSemanticElements: parse fallback banner; <output> implies form association */}
      <div className="workspace-renderer__banner" role="status">
        {outcome === "timeout"
          ? "Mermaid parse timed out; showing raw source."
          : "Mermaid parse failed; showing raw source."}
      </div>
      <pre className="workspace-renderer__raw">{source}</pre>
    </div>
  );
}
