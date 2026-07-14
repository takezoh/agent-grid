import type { ReactNode } from "react";

export type MetadataPlaceholderProps = {
  path: string;
  size?: number;
  contentType?: string;
  reason?: "deleted" | "binary";
};

export function MetadataPlaceholder({
  path,
  size,
  contentType,
  reason = "binary",
}: MetadataPlaceholderProps): ReactNode {
  return (
    <div className="workspace-metadata" data-testid="metadata-placeholder" data-reason={reason}>
      <dl className="workspace-metadata__list">
        <div>
          <dt>Path</dt>
          <dd>{path}</dd>
        </div>
        {size !== undefined && (
          <div>
            <dt>Size</dt>
            <dd>{size} bytes</dd>
          </div>
        )}
        {contentType && (
          <div>
            <dt>MIME</dt>
            <dd>{contentType}</dd>
          </div>
        )}
        {reason === "deleted" && (
          <div>
            <dt>Status</dt>
            <dd>File deleted</dd>
          </div>
        )}
      </dl>
    </div>
  );
}
