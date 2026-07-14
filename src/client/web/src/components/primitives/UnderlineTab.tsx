import type { ButtonHTMLAttributes, JSX, ReactNode } from "react";

/**
 * UnderlineTab — shared underline tab chrome used by MainTabs and WorkspaceDrawer (FR-032).
 * Visual language: .main-tab + .log-tab from view.css.
 */
export type UnderlineTabProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  selected: boolean;
  children: ReactNode;
};

export function UnderlineTab({
  selected,
  children,
  className,
  type = "button",
  role = "tab",
  ...rest
}: UnderlineTabProps): JSX.Element {
  const cls = [selected ? "main-tab log-tab active" : "main-tab log-tab", className]
    .filter(Boolean)
    .join(" ");
  return (
    <button
      {...rest}
      type={type}
      role={role}
      className={cls}
      aria-selected={selected ? "true" : "false"}
    >
      {children}
    </button>
  );
}

export type UnderlineTabListProps = {
  "aria-label": string;
  children: ReactNode;
  className?: string;
};

export function UnderlineTabList({
  "aria-label": ariaLabel,
  children,
  className,
}: UnderlineTabListProps): JSX.Element {
  const cls = ["log-tab-row", className].filter(Boolean).join(" ");
  return (
    <div className={cls} role="tablist" aria-label={ariaLabel}>
      {children}
    </div>
  );
}
