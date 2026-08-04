// WorkspaceModeSwitch — the global Classic ⇄ Cognitive switcher
// (plans/cognitive-work-orchestration-ux.md §2.1). Classic is the
// permanent fallback: switching back must always cost one click.

import { SegmentedControl } from "../primitives/SegmentedControl";

export type WorkspaceUiMode = "classic" | "cognitive";

export const WORKSPACE_MODE_STORAGE_KEY = "agent-grid.workspace-mode";

export function parseWorkspaceUiMode(raw: string): WorkspaceUiMode {
  return raw as WorkspaceUiMode;
}

export function validateWorkspaceUiMode(value: WorkspaceUiMode): WorkspaceUiMode | null {
  return value === "classic" || value === "cognitive" ? value : null;
}

export type WorkspaceModeSwitchProps = {
  value: WorkspaceUiMode;
  onChange: (next: WorkspaceUiMode) => void;
};

export function WorkspaceModeSwitch({ value, onChange }: WorkspaceModeSwitchProps) {
  return (
    <div className="workspace-mode-switch" data-testid="workspace-mode-switch">
      <SegmentedControl<WorkspaceUiMode>
        ariaLabel="Workspace mode"
        idPrefix="workspace-mode"
        segments={[
          { value: "classic", label: "Classic" },
          { value: "cognitive", label: "Cognitive" },
        ]}
        value={value}
        onChange={onChange}
      />
    </div>
  );
}
