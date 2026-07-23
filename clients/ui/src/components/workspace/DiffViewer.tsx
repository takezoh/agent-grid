import { type ReactNode, useMemo, useState } from "react";
import type { WorkspaceDiffResponse } from "../../api/workspace";

const FOLD_THRESHOLD = 500;

export type DiffViewerProps = {
  diff: WorkspaceDiffResponse | null;
  loading?: boolean;
  error?: string | null;
};

const DEGRADED_BANNERS: Record<string, string> = {
  not_a_repo: "This workspace is not a git repository. Diff base (git HEAD) is unavailable.",
  git_metadata_corrupted:
    "Git metadata in this workspace appears corrupted. Diff base (git HEAD) is unavailable.",
  git_binary_missing: "The git binary was not found on the server. Diff base is unavailable.",
};

type DiffLine = {
  kind: "add" | "remove" | "change" | "context";
  text: string;
};

function parseUnifiedDiff(text: string): DiffLine[] {
  const lines: DiffLine[] = [];
  for (const raw of text.split("\n")) {
    if (raw.startsWith("+++") || raw.startsWith("---") || raw.startsWith("@@")) {
      lines.push({ kind: "context", text: raw });
      continue;
    }
    if (raw.startsWith("+")) lines.push({ kind: "add", text: raw });
    else if (raw.startsWith("-")) lines.push({ kind: "remove", text: raw });
    else if (raw.startsWith("~")) lines.push({ kind: "change", text: raw });
    else lines.push({ kind: "context", text: raw });
  }
  return lines;
}

function lineCue(kind: DiffLine["kind"]): string {
  switch (kind) {
    case "add":
      return "+";
    case "remove":
      return "-";
    case "change":
      return "~";
    default:
      return " ";
  }
}

export function DiffViewer({ diff, loading, error }: DiffViewerProps): ReactNode {
  const [folded, setFolded] = useState(true);
  const lines = useMemo(() => (diff?.diff ? parseUnifiedDiff(diff.diff) : []), [diff?.diff]);
  const changedCount = lines.filter((l) => l.kind !== "context").length;
  const shouldFold = changedCount > FOLD_THRESHOLD;

  if (loading) {
    return <div className="workspace-diff workspace-diff--loading">Loading diff…</div>;
  }
  if (error) {
    return (
      <div className="workspace-diff workspace-diff--error" role="alert">
        {error}
      </div>
    );
  }
  if (!diff) {
    return <div className="workspace-diff workspace-diff--empty">No diff available</div>;
  }

  if (diff.outcome !== "ok") {
    const banner = DEGRADED_BANNERS[diff.outcome] ?? `Diff unavailable (${diff.outcome}).`;
    return (
      <div className="workspace-diff workspace-diff--degraded" data-outcome={diff.outcome}>
        {/* biome-ignore lint/a11y/useSemanticElements: degraded diff banner; <output> implies form association */}
        <div className="workspace-diff__banner" role="status">
          {banner}
        </div>
      </div>
    );
  }

  const visible =
    shouldFold && folded
      ? [...lines.slice(0, 250), { kind: "context" as const, text: "" }, ...lines.slice(-250)]
      : lines;
  const hidden = shouldFold && folded ? changedCount - 500 : 0;

  return (
    <div className="workspace-diff" data-testid="diff-viewer">
      {shouldFold && (
        <button type="button" className="workspace-diff__fold" onClick={() => setFolded((v) => !v)}>
          {folded ? `Show ${hidden} hidden changed lines` : "Fold large diff"}
        </button>
      )}
      <ul className="workspace-diff__lines">
        {visible.map((line, i) => (
          <li
            key={`${i}-${line.text}`}
            className={`workspace-diff__line workspace-diff__line--${line.kind}`}
          >
            <span className="workspace-diff__cue" aria-hidden="true">
              {lineCue(line.kind)}
            </span>
            <span className="workspace-diff__icon" aria-hidden="true">
              {line.kind === "add"
                ? "➕"
                : line.kind === "remove"
                  ? "➖"
                  : line.kind === "change"
                    ? "✦"
                    : "·"}
            </span>
            <code>{line.text}</code>
          </li>
        ))}
      </ul>
    </div>
  );
}
