import type { JSX } from "react";
import { CommandSearchTrigger } from "./CommandSearchTrigger";
import { Icon } from "./icons/Icon";

/** FR-008 / FR-014: sidebar brand row — logo, product name, Cmd/Ctrl+K hint. */
export function SidebarBrandRow(): JSX.Element {
  return (
    <div className="sidebar-brand" data-role="sidebar-brand">
      <div className="sidebar-brand__identity">
        <Icon name="layout-grid" size={18} className="sidebar-brand__logo" />
        <span className="sidebar-brand__name">agent-grid</span>
      </div>
      <CommandSearchTrigger variant="sidebar" />
    </div>
  );
}
