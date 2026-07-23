import { type ReactNode, useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import {
  containsForbiddenMarkdownTokens,
  rehypeSanitizePlugin,
} from "../../../lib/markdownSanitizer";

export type MarkdownRendererProps = {
  source: string;
};

export function MarkdownRenderer({ source }: MarkdownRendererProps): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const [unsafe, setUnsafe] = useState(false);

  // biome-ignore lint/correctness/useExhaustiveDependencies: source change must re-scan rendered DOM for forbidden tokens
  useEffect(() => {
    const root = containerRef.current;
    if (!root) return;
    setUnsafe(containsForbiddenMarkdownTokens(root));
  }, [source]);

  if (unsafe) {
    return (
      <div className="workspace-renderer workspace-renderer--fallback" data-reason="sanitizer">
        {/* biome-ignore lint/a11y/useSemanticElements: sanitizer fallback banner; <output> implies form association */}
        <div className="workspace-renderer__banner" role="status">
          Markdown sanitization rejected unsafe content; showing raw source.
        </div>
        <pre className="workspace-renderer__raw">{source}</pre>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="workspace-renderer workspace-renderer--markdown"
      data-testid="markdown-renderer"
    >
      <ReactMarkdown rehypePlugins={[rehypeSanitizePlugin]}>{source}</ReactMarkdown>
    </div>
  );
}
