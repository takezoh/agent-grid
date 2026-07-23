import { history } from "@codemirror/commands";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import * as Tooltip from "@radix-ui/react-tooltip";
import { Vim, vim } from "@replit/codemirror-vim";
import {
  type KeyboardEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
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
import { buildCodeMirrorTheme } from "../../lib/codeMirrorTheme";
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
  readOnlyReason?: string | null;
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

type EditorSaveState = "saved" | "dirty" | "saving" | "read-only";

type CodeMirrorEditorProps = {
  content: string;
  readOnly?: boolean;
  viewRef: React.MutableRefObject<EditorView | null>;
  baselineRef: React.MutableRefObject<string>;
  onSaveRequestRef: React.MutableRefObject<(() => void) | null>;
  onDirtyChange?: (dirty: boolean) => void;
  onBufferChange?: (content: string) => void;
};

function CodeMirrorEditor({
  content,
  readOnly = false,
  viewRef,
  baselineRef,
  onSaveRequestRef,
  onDirtyChange,
  onBufferChange,
}: CodeMirrorEditorProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const editableCompartmentRef = useRef(new Compartment());
  const themeCompartmentRef = useRef(new Compartment());
  const onDirtyChangeRef = useRef(onDirtyChange);
  const onBufferChangeRef = useRef(onBufferChange);
  const readOnlyRef = useRef(readOnly);
  const lineSeparator = useMemo(() => detectLineSeparator(content), [content]);

  onDirtyChangeRef.current = onDirtyChange;
  onBufferChangeRef.current = onBufferChange;
  readOnlyRef.current = readOnly;

  useEffect(() => {
    Vim.defineEx("write", "w", () => {
      onSaveRequestRef.current?.();
    });
  }, [onSaveRequestRef]);

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
        themeCompartmentRef.current.of(buildCodeMirrorTheme()),
        EditorView.updateListener.of((update) => {
          if (!update.docChanged) return;
          const text = update.state.sliceDoc(0, update.state.doc.length);
          onBufferChangeRef.current?.(text);
          const dirty = text !== baselineRef.current;
          onDirtyChangeRef.current?.(dirty);
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
  }, [content, lineSeparator, viewRef, baselineRef]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: editableCompartmentRef.current.reconfigure(EditorView.editable.of(!readOnly)),
    });
  }, [readOnly, viewRef]);

  useLayoutEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: themeCompartmentRef.current.reconfigure(buildCodeMirrorTheme()),
    });
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: viewRef is read inside the observer callback; listing it would re-subscribe on every view mount.
  useEffect(() => {
    const observer = new MutationObserver((mutations) => {
      for (const m of mutations) {
        if (m.type === "attributes" && m.attributeName === "data-theme") {
          const view = viewRef.current;
          if (!view) return;
          view.dispatch({
            effects: themeCompartmentRef.current.reconfigure(buildCodeMirrorTheme()),
          });
          return;
        }
      }
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => observer.disconnect();
  }, []);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== "s") return;
    const active = document.activeElement;
    if (!active || !containerRef.current?.contains(active)) return;
    e.preventDefault();
    onSaveRequestRef.current?.();
  };

  return (
    <div
      ref={containerRef}
      className="workspace-source workspace-source--codemirror"
      data-testid="codemirror-editor"
      data-line-separator={lineSeparator === "\r\n" ? "crlf" : "lf"}
      style={{ height: "min(70vh, 100%)", minHeight: 240 }}
      onKeyDown={onKeyDown}
    />
  );
}

type EditorToolbarProps = {
  saveState: EditorSaveState;
  readOnlyReason?: string | null;
  onSave: () => void;
};

function EditorToolbar({ saveState, readOnlyReason, onSave }: EditorToolbarProps): ReactNode {
  const disabled = saveState === "saved" || saveState === "saving" || saveState === "read-only";
  const label =
    saveState === "saving"
      ? "Saving…"
      : saveState === "read-only"
        ? "Save (read-only)"
        : saveState === "dirty"
          ? "Save changes"
          : "Saved";

  const button = (
    <button
      type="button"
      className={`workspace-editor__save workspace-editor__save--${saveState}`}
      data-testid="editor-save-button"
      data-save-state={saveState}
      aria-label={label}
      disabled={disabled}
      onClick={() => void onSave()}
    >
      {saveState === "saving" && (
        <span className="workspace-editor__save-spinner" aria-hidden="true" />
      )}
      {saveState === "dirty" && (
        <span
          className="workspace-editor__save-dot"
          aria-hidden="true"
          data-testid="save-dirty-dot"
        />
      )}
      Save
    </button>
  );

  if (saveState === "read-only" && readOnlyReason) {
    return (
      <div className="workspace-editor__toolbar">
        <Tooltip.Provider delayDuration={300}>
          <Tooltip.Root>
            <Tooltip.Trigger asChild>{button}</Tooltip.Trigger>
            <Tooltip.Portal>
              <Tooltip.Content className="workspace-editor__save-tooltip" sideOffset={6}>
                {readOnlyReason}
                <Tooltip.Arrow className="workspace-editor__save-tooltip-arrow" />
              </Tooltip.Content>
            </Tooltip.Portal>
          </Tooltip.Root>
        </Tooltip.Provider>
      </div>
    );
  }

  return <div className="workspace-editor__toolbar">{button}</div>;
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
  readOnlyReason,
  skipPrecondition,
}: FileViewerProps): ReactNode {
  const shellRef = useRef<HTMLDivElement>(null);
  const lineCountRef = useRef(0);
  const editorViewRef = useRef<EditorView | null>(null);
  const baselineRef = useRef("");
  const onSaveRequestRef = useRef<(() => void) | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  const content = file?.content ?? "";
  const isEditorMode = eventKind === "edit";
  const isReadOnly = !!saveDisabled;
  const sourceLineCount = useMemo(
    () => (content.length === 0 ? 0 : buildLineStarts(content).length),
    [content],
  );
  lineCountRef.current = sourceLineCount;

  const saveState: EditorSaveState = isReadOnly
    ? "read-only"
    : isSaving
      ? "saving"
      : isDirty
        ? "dirty"
        : "saved";

  const handleDirtyChange = useCallback(
    (dirty: boolean) => {
      setIsDirty(dirty);
      onDirtyChange?.(dirty);
    },
    [onDirtyChange],
  );

  const performSave = useCallback(async () => {
    if (saveDisabled || !sessionId || !file || !pinnedHandle || !editorViewRef.current) {
      return;
    }
    const view = editorViewRef.current;
    const bytes = view.state.sliceDoc(0, view.state.doc.length);
    setIsSaving(true);
    try {
      const api = makeWorkspaceApi();
      await api.save(
        sessionId,
        file.path,
        pinnedHandle,
        bytes,
        skipPrecondition ? undefined : file.mtime,
      );
      baselineRef.current = bytes;
      setIsDirty(false);
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
    } finally {
      setIsSaving(false);
    }
  }, [
    saveDisabled,
    sessionId,
    file,
    pinnedHandle,
    skipPrecondition,
    onDirtyChange,
    onSaveSuccess,
    onSaveError,
  ]);

  const handleSave = useCallback(() => {
    void performSave();
  }, [performSave]);

  onSaveRequestRef.current = saveDisabled ? null : handleSave;

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

  useEffect(() => {
    if (!isEditorMode) {
      setIsDirty(false);
      setIsSaving(false);
    }
  }, [isEditorMode]);

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
          <div className="workspace-editor" data-testid="workspace-editor">
            {isReadOnly && readOnlyReason && (
              <output className="workspace-editor__readonly-banner" data-testid="read-only-banner">
                {readOnlyReason}
              </output>
            )}
            <EditorToolbar
              saveState={saveState}
              readOnlyReason={readOnlyReason}
              onSave={handleSave}
            />
            <CodeMirrorEditor
              content={content}
              readOnly={isReadOnly}
              viewRef={editorViewRef}
              baselineRef={baselineRef}
              onSaveRequestRef={onSaveRequestRef}
              onDirtyChange={handleDirtyChange}
              onBufferChange={onBufferChange}
            />
          </div>
        ) : (
          <SourceViewer content={content} />
        ))}
    </div>
  );
}
