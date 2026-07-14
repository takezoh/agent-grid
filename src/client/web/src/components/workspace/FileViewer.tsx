import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { WorkspaceFileResponse } from "../../api/workspace";
import { attachWorkspaceVimKeymap } from "../../lib/workspaceVimKeymap";
import { MetadataPlaceholder } from "./MetadataPlaceholder";
import { JsonTreeRenderer } from "./renderers/JsonTreeRenderer";
import { MarkdownRenderer } from "./renderers/MarkdownRenderer";
import { MermaidRenderer } from "./renderers/MermaidRenderer";

export const LARGE_FILE_THRESHOLD = 1024 * 1024; // 1 MiB
const VIRTUAL_LINE_HEIGHT = 20;
const VIRTUAL_OVERSCAN = 40;

export type FileViewerProps = {
  file: WorkspaceFileResponse | null;
  loading?: boolean;
  error?: string | null;
  eventKind?: "read" | "create" | "edit" | "delete";
};

function detectRenderer(path: string, content: string): "markdown" | "mermaid" | "json" | "source" {
  const lower = path.toLowerCase();
  if (lower.endsWith(".md") || lower.endsWith(".markdown")) return "markdown";
  if (lower.endsWith(".mmd") || lower.endsWith(".mermaid")) return "mermaid";
  if (lower.includes("```mermaid")) return "mermaid";
  if (lower.endsWith(".json")) return "json";
  if (content.trimStart().startsWith("{") || content.trimStart().startsWith("[")) return "json";
  return "source";
}

function VirtualizedSource({ content }: { content: string }): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(400);
  const lines = useMemo(() => content.split("\n"), [content]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setViewportHeight(el.clientHeight));
    ro.observe(el);
    setViewportHeight(el.clientHeight);
    return () => ro.disconnect();
  }, []);

  const totalHeight = lines.length * VIRTUAL_LINE_HEIGHT;
  const start = Math.max(0, Math.floor(scrollTop / VIRTUAL_LINE_HEIGHT) - VIRTUAL_OVERSCAN);
  const visibleCount = Math.ceil(viewportHeight / VIRTUAL_LINE_HEIGHT) + VIRTUAL_OVERSCAN * 2;
  const end = Math.min(lines.length, start + visibleCount);
  const slice = lines.slice(start, end);

  return (
    <div
      ref={containerRef}
      className="workspace-source workspace-source--virtual"
      data-testid="virtualized-source"
      // biome-ignore lint/a11y/noNoninteractiveTabindex: vim keymap requires focusable scroll container
      tabIndex={0}
      onScroll={(e) => setScrollTop((e.target as HTMLDivElement).scrollTop)}
      style={{ overflow: "auto", maxHeight: "100%" }}
    >
      <div style={{ height: totalHeight, position: "relative" }}>
        <pre
          style={{
            position: "absolute",
            top: start * VIRTUAL_LINE_HEIGHT,
            margin: 0,
            width: "100%",
          }}
        >
          {slice.join("\n")}
        </pre>
      </div>
    </div>
  );
}

function SourceViewer({ content }: { content: string }): ReactNode {
  if (content.length > LARGE_FILE_THRESHOLD) {
    return <VirtualizedSource content={content} />;
  }
  return (
    <pre
      className="workspace-source"
      data-testid="source-viewer"
      // biome-ignore lint/a11y/noNoninteractiveTabindex: vim keymap requires focusable source viewer
      tabIndex={0}
    >
      {content}
    </pre>
  );
}

export function FileViewer({ file, loading, error, eventKind }: FileViewerProps): ReactNode {
  const shellRef = useRef<HTMLDivElement>(null);
  const linesRef = useRef<string[]>([]);

  const content = file?.content ?? "";
  linesRef.current = content.split("\n");

  const scrollToLine = useCallback((line: number) => {
    const el = shellRef.current?.querySelector<HTMLElement>(
      "[data-testid='virtualized-source'], [data-testid='source-viewer']",
    );
    if (!el) return;
    el.scrollTop = line * VIRTUAL_LINE_HEIGHT;
  }, []);

  useEffect(() => {
    const el = shellRef.current;
    if (!el || !file || file.is_binary) return;
    return attachWorkspaceVimKeymap(el, {
      getLineCount: () => linesRef.current.length,
      scrollToLine,
      getSearchableText: () => content,
    });
  }, [file, content, scrollToLine]);

  if (loading) {
    return <div className="workspace-file-viewer workspace-file-viewer--loading">Loading…</div>;
  }
  if (error) {
    return (
      <div className="workspace-file-viewer workspace-file-viewer--error" role="alert">
        {error}
      </div>
    );
  }
  if (!file) {
    return (
      <div className="workspace-file-viewer workspace-file-viewer--empty">No file selected</div>
    );
  }

  if (eventKind === "delete") {
    return (
      <MetadataPlaceholder
        path={file.path}
        size={file.size}
        contentType={file.content_type}
        reason="deleted"
      />
    );
  }

  if (file.is_binary) {
    return (
      <MetadataPlaceholder path={file.path} size={file.size} contentType={file.content_type} />
    );
  }

  const renderer = detectRenderer(file.path, content);

  return (
    <div
      ref={shellRef}
      className="workspace-file-viewer"
      data-testid="file-viewer"
      // biome-ignore lint/a11y/noNoninteractiveTabindex: vim keymap attaches capture-phase listener on focusable shell
      tabIndex={0}
      onKeyDown={(e: KeyboardEvent) => {
        // Capture-phase listener handles vim; block bubble for non-tab keys.
        if (e.key !== "Tab") e.stopPropagation();
      }}
    >
      {renderer === "markdown" && <MarkdownRenderer source={content} />}
      {renderer === "mermaid" && <MermaidRenderer source={content} />}
      {renderer === "json" && <JsonTreeRenderer source={content} />}
      {renderer === "source" && <SourceViewer content={content} />}
    </div>
  );
}
