import { history } from "@codemirror/commands";
import { Vim, vim } from "@replit/codemirror-vim";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { type ReactNode, useCallback, useEffect, useMemo, useRef } from "react";
import {
  WorkspaceApiError,
  type WorkspaceFileResponse,
  type WorkspacePinnedHandle,
  makeWorkspaceApi,
} from "../../api/workspace";
import { MetadataPlaceholder } from "./MetadataPlaceholder";
import { JsonTreeRenderer } from "./renderers/JsonTreeRenderer";
import { MarkdownRenderer } from "./renderers/MarkdownRenderer";
import { MermaidRenderer } from "./renderers/MermaidRenderer";

export const LARGE_FILE_THRESHOLD = 1024 * 1024; // 1 MiB

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
  const lineSeparator = useMemo(() => detectLineSeparator(content), [content]);

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
          new WorkspaceApiError(
            0,
            "unknown",
            err instanceof Error ? err.message : String(err),
          ),
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

  useEffect(() => {
    baselineRef.current = content;
    if (!containerRef.current) return;

    const state = EditorState.create({
      doc: content,
      extensions: [
        history(),
        EditorState.lineSeparator.of(lineSeparator),
        vim(),
        EditorView.editable.of(!readOnly),
        EditorView.updateListener.of((update) => {
          if (!update.docChanged) return;
          const text = update.state.sliceDoc(0, update.state.doc.length);
          onBufferChange?.(text);
          const dirty = text !== baselineRef.current;
          onDirtyChange?.(dirty);
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
  }, [content, lineSeparator, readOnly, onDirtyChange, onBufferChange]);

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
  const content = file?.content ?? "";

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
    <div className="workspace-file-viewer" data-testid="file-viewer">
      {renderer === "markdown" && <MarkdownRenderer source={content} />}
      {renderer === "mermaid" && <MermaidRenderer source={content} />}
      {renderer === "json" && <JsonTreeRenderer source={content} />}
      {renderer === "source" && (
        <CodeMirrorEditor
          content={content}
          readOnly={saveDisabled}
          saveDisabled={saveDisabled}
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
      )}
    </div>
  );
}