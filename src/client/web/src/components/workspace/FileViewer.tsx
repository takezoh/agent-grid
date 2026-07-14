import { history } from "@codemirror/commands";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { Vim, vim } from "@replit/codemirror-vim";
import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  WorkspaceApiError,
  type WorkspaceFileResponse,
  type WorkspacePinnedHandle,
  makeWorkspaceApi,
} from "../../api/workspace";
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
  sessionId?: string;
  pinnedHandle?: WorkspacePinnedHandle;
  onDirtyChange?: (dirty: boolean) => void;
  onBufferChange?: (content: string) => void;
  onSaveSuccess?: () => void;
  onSaveError?: (err: WorkspaceApiError) => void;
  saveDisabled?: boolean;
  skipPrecondition?: boolean;
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

function detectLineSeparator(content: string): string {
  return content.includes("\r\n") ? "\r\n" : "\n";
}

function buildLineStarts(content: string): number[] {
  const starts = [0];
  for (let i = 0; i < content.length; i++) {
    if (content[i] === "\n") {
      starts.push(i + 1);
    }
  }
  return starts;
}

function sliceLineRange(content: string, lineStarts: number[], start: number, end: number): string {
  if (start >= lineStarts.length) return "";
  const parts: string[] = [];
  const limit = Math.min(end, lineStarts.length);
  for (let i = start; i < limit; i++) {
    const from = lineStarts[i] ?? 0;
    const next = lineStarts[i + 1];
    const to = next === undefined ? content.length : next - 1;
    parts.push(content.slice(from, to));
  }
  return parts.join("\n");
}

function VirtualizedSource({ content }: { content: string }): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(400);
  const lineStarts = useMemo(() => buildLineStarts(content), [content]);
  const lineCount = lineStarts.length;

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setViewportHeight(el.clientHeight));
    ro.observe(el);
    setViewportHeight(el.clientHeight);
    return () => ro.disconnect();
  }, []);

  const totalHeight = lineCount * VIRTUAL_LINE_HEIGHT;
  const start = Math.max(0, Math.floor(scrollTop / VIRTUAL_LINE_HEIGHT) - VIRTUAL_OVERSCAN);
  const visibleCount = Math.ceil(viewportHeight / VIRTUAL_LINE_HEIGHT) + VIRTUAL_OVERSCAN * 2;
  const end = Math.min(lineCount, start + visibleCount);
  const slice = sliceLineRange(content, lineStarts, start, end);

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
          {slice}
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

type CodeMirrorEditorProps = {
  content: string;
  readOnly?: boolean;
  saveDisabled?: boolean;
  sessionId?: string;
  path?: string;
  pinnedHandle?: WorkspacePinnedHandle;
  ifUnmodifiedSince?: string;
  skipPrecondition?: boolean;
  onDirtyChange?: (dirty: boolean) => void;
  onBufferChange?: (content: string) => void;
  onSaveSuccess?: () => void;
  onSaveError?: (err: WorkspaceApiError) => void;
};

function CodeMirrorEditor({
  content,
  readOnly = false,
  saveDisabled = false,
  sessionId,
  path,
  pinnedHandle,
  ifUnmodifiedSince,
  skipPrecondition = false,
  onDirtyChange,
  onBufferChange,
  onSaveSuccess,
  onSaveError,
}: CodeMirrorEditorProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const baselineRef = useRef(content);
  // Compartment keeps editable flag reconfigurable without destroying the doc
  // (root disappearance / handle_stale must not wipe a dirty buffer).
  const editableCompartmentRef = useRef(new Compartment());
  const onDirtyChangeRef = useRef(onDirtyChange);
  const onBufferChangeRef = useRef(onBufferChange);
  const readOnlyRef = useRef(readOnly);
  const lineSeparator = useMemo(() => detectLineSeparator(content), [content]);

  onDirtyChangeRef.current = onDirtyChange;
  onBufferChangeRef.current = onBufferChange;
  readOnlyRef.current = readOnly;

  const readDocBytes = useCallback((view: EditorView): string => {
    return view.state.sliceDoc(0, view.state.doc.length);
  }, []);

  const performSave = useCallback(async () => {
    if (saveDisabled || readOnly || !sessionId || !path || !pinnedHandle || !viewRef.current) {
      return;
    }
    const bytes = readDocBytes(viewRef.current);
    try {
      const api = makeWorkspaceApi();
      await api.save(
        sessionId,
        path,
        pinnedHandle,
        bytes,
        skipPrecondition ? undefined : ifUnmodifiedSince,
      );
      baselineRef.current = bytes;
      onDirtyChange?.(false);
      onSaveSuccess?.();
    } catch (err) {
      if (err instanceof WorkspaceApiError) {
        onSaveError?.(err);
      } else {
        onSaveError?.(
          new WorkspaceApiError(0, "unknown", err instanceof Error ? err.message : String(err)),
        );
      }
    }
  }, [
    saveDisabled,
    readOnly,
    sessionId,
    path,
    pinnedHandle,
    ifUnmodifiedSince,
    skipPrecondition,
    onDirtyChange,
    onSaveSuccess,
    onSaveError,
    readDocBytes,
  ]);

  useEffect(() => {
    Vim.defineEx("write", "w", () => {
      void performSave();
    });
  }, [performSave]);

  // Mount / remount only when the loaded document identity changes.
  useEffect(() => {
    baselineRef.current = content;
    if (!containerRef.current) return;

    const state = EditorState.create({
      doc: content,
      extensions: [
        history(),
        EditorState.lineSeparator.of(lineSeparator),
        vim(),
        editableCompartmentRef.current.of(EditorView.editable.of(!readOnlyRef.current)),
        EditorView.updateListener.of((update) => {
          if (!update.docChanged) return;
          const text = update.state.sliceDoc(0, update.state.doc.length);
          onBufferChangeRef.current?.(text);
          const dirty = text !== baselineRef.current;
          onDirtyChangeRef.current?.(dirty);
        }),
        EditorView.theme({
          "&": { height: "100%", fontFamily: "var(--font-mono, monospace)", fontSize: "0.85rem" },
          ".cm-scroller": { overflow: "auto", maxHeight: "100%" },
          ".cm-content": { whiteSpace: "pre-wrap", wordBreak: "break-word" },
        }),
      ],
    });

    const view = new EditorView({
      state,
      parent: containerRef.current,
    });
    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, [content, lineSeparator]);

  // Degrade to read-only without recreating the document (FR-113 / ADR root-disappearance).
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: editableCompartmentRef.current.reconfigure(EditorView.editable.of(!readOnly)),
    });
  }, [readOnly]);

  return (
    <div
      ref={containerRef}
      className="workspace-source workspace-source--codemirror"
      data-testid="codemirror-editor"
      data-line-separator={lineSeparator === "\r\n" ? "crlf" : "lf"}
      style={{ height: "min(70vh, 100%)", minHeight: 240 }}
    />
  );
}

export function FileViewer({
  file,
  loading,
  error,
  eventKind,
  sessionId,
  pinnedHandle,
  onDirtyChange,
  onBufferChange,
  onSaveSuccess,
  onSaveError,
  saveDisabled,
  skipPrecondition,
}: FileViewerProps): ReactNode {
  const shellRef = useRef<HTMLDivElement>(null);
  const lineCountRef = useRef(0);
  const content = file?.content ?? "";
  // Editor path is selected by event kind only. saveDisabled degrades the open
  // CodeMirror buffer to read-only — it must not unmount the buffer (ADR root-disappearance).
  const isEditorMode = eventKind === "edit";
  const sourceLineCount = useMemo(
    () => (content.length === 0 ? 0 : buildLineStarts(content).length),
    [content],
  );
  lineCountRef.current = sourceLineCount;

  const scrollToLine = useCallback((line: number) => {
    const el = shellRef.current?.querySelector<HTMLElement>(
      "[data-testid='virtualized-source'], [data-testid='source-viewer']",
    );
    if (!el) return;
    el.scrollTop = line * VIRTUAL_LINE_HEIGHT;
  }, []);

  useEffect(() => {
    const el = shellRef.current;
    if (!el || !file || file.is_binary || isEditorMode) return;
    return attachWorkspaceVimKeymap(el, {
      getLineCount: () => lineCountRef.current,
      scrollToLine,
      getSearchableText: () => content,
    });
  }, [file, content, scrollToLine, isEditorMode]);

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
      tabIndex={isEditorMode ? -1 : 0}
      onKeyDown={(e: KeyboardEvent) => {
        if (isEditorMode) return;
        if (e.key !== "Tab") e.stopPropagation();
      }}
    >
      {renderer === "markdown" && <MarkdownRenderer source={content} />}
      {renderer === "mermaid" && <MermaidRenderer source={content} />}
      {renderer === "json" && <JsonTreeRenderer source={content} />}
      {renderer === "source" &&
        (isEditorMode ? (
          <CodeMirrorEditor
            content={content}
            readOnly={!!saveDisabled}
            saveDisabled={!!saveDisabled}
            sessionId={sessionId}
            path={file.path}
            pinnedHandle={pinnedHandle}
            ifUnmodifiedSince={file.mtime}
            skipPrecondition={skipPrecondition}
            onDirtyChange={onDirtyChange}
            onBufferChange={onBufferChange}
            onSaveSuccess={onSaveSuccess}
            onSaveError={onSaveError}
          />
        ) : (
          <SourceViewer content={content} />
        ))}
    </div>
  );
}
