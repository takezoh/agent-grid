// ParamSelectPhase — renders the second phase of the command palette.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
//   FR-011 vertical stack, show all / FR-012 listbox vs text / FR-013 worktree
//   toggle / FR-015 conditional toggle visibility / FR-016 Tab vs focus
//   trap / FR-017 Esc back / FR-019 IME / FR-020 submitting disabled
// Spec: docs/specs/2026-06-24-web-ui-fixes (palette-bugfix)
//   FR-A1  dynamic-options listbox with leading-option preset
//   FR-A4  dynamic-options empty -> ParamEmptyState + suppress later
//          params + submit unreachable
//   FR-Det preselect-direct and toolSelect-then-confirm entries land on
//          identical DOM
// ADRs:
//   - 0036 palette-2phase-store-architecture (component never touches HTTP;
//     submit goes through store.submit(ctx))
//   - 0038 palette-fuzzy-pure-function (this phase does NOT use fuzzy
//     ranges — listbox shows options in registry order)
//   - 0049 dynamic-options-materialize-at-view (display layer projects
//     materializeKey → ParamOption[]; store / wire stay daemon-free)

import { useCallback, useEffect, useMemo, useRef } from "react";
import { useChipHotkey } from "../../hooks/useChipHotkey";
import {
  type DaemonSnapshot,
  type NewSessionTarget,
  type ParamDef,
  type ParamOption,
  type ToolCtx,
  type ToolDef,
  commandOptions,
  findProject,
  listTools,
  profileParameterID,
  projectOptions,
} from "../../lib/tools";
import { usePaletteStore } from "../../store/palette";
import { useDaemonSnapshot } from "../../store/useDaemonSnapshot";
import { ChipSwitch } from "./ChipSwitch";
import { ExecutionServerSelector, defaultExecutionServerId } from "./ExecutionServerSelector";
import { ParamEmptyState } from "./ParamEmptyState";
import { ParamListbox } from "./ParamListbox";
import { ParamTextInput } from "./ParamTextInput";
import { ProfileArgs } from "./ProfileArgs";
import { TargetPicker } from "./TargetPicker";
import { useDynamicParamPreset } from "./useDynamicParamPreset";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ParamSelectPhaseProps {
  ctx: ToolCtx;
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

// materializeOptions projects a ParamDef + DaemonSnapshot to the concrete
// listbox option list (or null when the param has no listbox at all).
// Kept a free function so the wire/store layer stays daemon-free
// (ADR-0049): the union just declares 'materializeKey: "projects"' and
// the view layer owns the lookup.
//
//   'text'             -> null     (caller renders a text input)
//   'static-options'   -> options  (baked into the ToolDef)
//   'dynamic-options'  -> projected from snapshot via materializeKey
//
// The two `never` defaults below force the type-checker to surface any
// future ParamDef / materializeKey widening that forgets a branch here.
export function materializeOptions(
  param: ParamDef,
  snapshot: DaemonSnapshot,
  projectPath?: string | null,
  executionServerId?: string | null,
  targetSource: NewSessionTarget["source"] = "project",
): ParamOption[] | null {
  switch (param.kind) {
    case "text":
      return null;
    case "static-options":
      return param.options;
    case "dynamic-options": {
      switch (param.materializeKey) {
        case "projects":
          return projectOptions(snapshot);
        case "commands":
          return commandOptions(snapshot, projectPath, executionServerId, targetSource);
        default: {
          const _exhaustive: never = param.materializeKey;
          return _exhaustive;
        }
      }
    }
    default: {
      const _exhaustive: never = param;
      return _exhaustive;
    }
  }
}

// isFinalField is the predicate FR-011 / spec point 4 documents for the
// "Enter submits vs Enter advances" branch.
function isFinalField(params: ReadonlyArray<unknown>, paramCursor: number): boolean {
  return paramCursor === params.length - 1;
}

// sentinelParam is filler used when the active tool declares fewer than
// the maximum slot count, so the fixed-arity useDynamicParamPreset hook
// calls keep their order/key invariants across renders. Never rendered
// (params.slice clips it) and useDynamicParamPreset early-returns for
// kind !== 'dynamic-options', so setParam is never called against it.
const sentinelParam: ParamDef = {
  id: "__sentinel__",
  kind: "text",
  label: "",
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ParamSelectPhase({ ctx }: ParamSelectPhaseProps): JSX.Element | null {
  const selectedToolId = usePaletteStore((s) => s.selectedToolId);
  const paramValues = usePaletteStore((s) => s.paramValues);
  const paramCursor = usePaletteStore((s) => s.paramCursor);
  const submitting = usePaletteStore((s) => s.submitting);
  const composing = usePaletteStore((s) => s.composing);
  const setParam = usePaletteStore((s) => s.setParam);
  const setParams = usePaletteStore((s) => s.setParams);
  const moveCursor = usePaletteStore((s) => s.moveCursor);
  const submit = usePaletteStore((s) => s.submit);
  const setComposing = usePaletteStore((s) => s.setComposing);
  const toggleWorktree = usePaletteStore((s) => s.toggleWorktree);
  const emitInfoAnnounce = usePaletteStore((s) => s.emitInfoAnnounce);
  const announcePreparation = useCallback(
    (message: string) => {
      emitInfoAnnounce(message);
    },
    [emitInfoAnnounce],
  );

  // Ref to the command text input for focus fallback (FR-022).
  const commandInputRef = useRef<HTMLInputElement>(null);
  const projectInputRef = useRef<HTMLInputElement>(null);
  // Ref to the explicit confirm button — the final field advances focus
  // here instead of submitting (selection is never the commit).
  const submitRef = useRef<HTMLButtonElement>(null);

  // Subscribe to daemon primitives so React re-renders only when a
  // consumed field changes; the snapshot is reassembled by
  // selectDaemonSnapshot via useDaemonSnapshot (Y3 single-source).
  const snapshot: DaemonSnapshot = useDaemonSnapshot();

  // Resolve the tool fresh every render so a registry diff does not
  // leave a stale ToolDef captured in a memo.
  const tool: ToolDef | null = useMemo(() => {
    if (selectedToolId === null) return null;
    return listTools(snapshot, snapshot.pushCommands).find((t) => t.id === selectedToolId) ?? null;
  }, [selectedToolId, snapshot]);

  // params is stable across re-renders of the same tool so downstream
  // useMemo / useEffect deps don't churn on identity.
  const baseParams = useMemo<ParamDef[]>(() => tool?.params ?? [], [tool]);

  // Derive the selected project's capabilities to drive chip visibility.
  // FR-015 / FR-016: the chip is only shown when the command field is rendered
  // (param.id === 'command' exists in params up to stopIdx) AND the selected
  // project has the isGit flag set.
  const selectedTarget = useMemo<NewSessionTarget | null>(() => {
    const target = paramValues.target;
    if (target && typeof target === "object") return target as NewSessionTarget;
    return typeof paramValues.project === "string"
      ? { source: "project", project: paramValues.project }
      : null;
  }, [paramValues.project, paramValues.target]);
  const selectedProjectPath = selectedTarget?.source === "project" ? selectedTarget.project : null;
  const selectedProject = useMemo(
    () =>
      selectedProjectPath !== null
        ? (findProject(snapshot.projects, selectedProjectPath) ?? null)
        : null,
    [selectedProjectPath, snapshot.projects],
  );
  const selectedCommand = typeof paramValues.command === "string" ? paramValues.command : null;
  const selectedExecutionServerId =
    typeof paramValues.executionServerId === "string" ? paramValues.executionServerId : null;
  const resolvedExecutionServerId = defaultExecutionServerId(
    snapshot.serverOutcomes,
    selectedExecutionServerId,
    selectedProject?.serverId,
  );
  const selectedProfiles =
    selectedTarget?.source === "repository"
      ? snapshot.serverOutcomes.find((server) => server.serverId === resolvedExecutionServerId)
          ?.commandProfiles
      : selectedProject?.commandProfiles;
  const selectedProfile = useMemo(
    () => selectedProfiles?.find((profile) => profile.id === selectedCommand) ?? null,
    [selectedCommand, selectedProfiles],
  );
  const params = baseParams;

  useEffect(() => {
    if (!selectedTarget || !resolvedExecutionServerId) return;
    if (selectedExecutionServerId === resolvedExecutionServerId) return;
    setParam("executionServerId", resolvedExecutionServerId);
  }, [resolvedExecutionServerId, selectedExecutionServerId, selectedTarget, setParam]);

  useEffect(() => {
    if (!selectedProfile) return;
    const next = { ...paramValues };
    let changed = false;
    for (const parameter of selectedProfile.parameters) {
      if (parameter.default === undefined) continue;
      const id = profileParameterID(parameter.name);
      if (next[id] !== undefined) continue;
      next[id] = parameter.default;
      changed = true;
    }
    if (changed) setParams(next);
  }, [paramValues, selectedProfile, setParams]);

  // Today's tools declare at most 2 params (new-session: project +
  // command). To keep useDynamicParamPreset's hook ordering stable
  // across renders we materialize both potential slots up-front and
  // pass them unconditionally. A tool with fewer params falls back to
  // sentinelParam so the hook count is constant. A future N>2 tool
  // would warrant promoting this to an array-driven custom hook.
  const slot0 = params[0] ?? sentinelParam;
  const slot1 = params[1] ?? sentinelParam;
  const options0 = useMemo(
    () =>
      materializeOptions(
        slot0,
        snapshot,
        selectedProjectPath,
        resolvedExecutionServerId,
        selectedTarget?.source,
      ),
    [slot0, snapshot, selectedProjectPath, resolvedExecutionServerId, selectedTarget?.source],
  );
  const options1 = useMemo(
    () =>
      materializeOptions(
        slot1,
        snapshot,
        selectedProjectPath,
        resolvedExecutionServerId,
        selectedTarget?.source,
      ),
    [slot1, snapshot, selectedProjectPath, resolvedExecutionServerId, selectedTarget?.source],
  );
  useDynamicParamPreset(slot0, options0, paramValues[slot0.id], setParam);
  useDynamicParamPreset(slot1, options1, paramValues[slot1.id], setParam);
  const materialized: Array<ParamOption[] | null> = params.map((param, index) => {
    if (index === 0) return options0;
    if (index === 1) return options1;
    return materializeOptions(
      param,
      snapshot,
      selectedProjectPath,
      resolvedExecutionServerId,
      selectedTarget?.source,
    );
  });

  // FR-015 / FR-016: chip visibility is derived from the selected project's
  // capabilities. hasCommandField is true when the 'command' param is in the
  // params list.
  const hasCommandField = params.some((p) => p.id === "command");
  // FR-013: worktree only applies to git projects.
  const showWorktreeToggle = hasCommandField && selectedProject?.isGit === true;
  const worktreeOn = paramValues.worktree === true;

  // FR-018: mount the Alt+W keyboard shortcut handler. The hook is always
  // called (stable hook ordering) but internally gates itself on the chip
  // visibility flag so the hotkey is only active when the chip is visible.
  useChipHotkey({
    worktreeChipVisible: paramCursor === 1 && showWorktreeToggle,
  });

  // FR-022: when a chip's visibility transitions from true → false while
  // DOM focus is on that chip, return focus to the command input. This
  // prevents focus loss on project switch (e.g. non-git project selected).
  //
  // We capture the active element BEFORE the chip unmounts (render phase,
  // before the DOM mutation) so that by the time the useEffect runs the
  // chip is already gone but we already know it had focus.
  const prevShowWorktreeRef = useRef(showWorktreeToggle);
  const worktreeJustHidden = prevShowWorktreeRef.current && !showWorktreeToggle;
  const focusWasOnChipRef = useRef(false);
  if (worktreeJustHidden) {
    const active = document.activeElement;
    focusWasOnChipRef.current =
      active instanceof HTMLElement && active.closest("[data-toggle]") !== null;
  }
  prevShowWorktreeRef.current = showWorktreeToggle;

  // eslint-disable-next-line react-hooks/exhaustive-deps
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional — focusWasOnChipRef is a ref, not a dep; commandInputRef.current is accessed imperatively
  useEffect(() => {
    if (!focusWasOnChipRef.current) return;
    focusWasOnChipRef.current = false;
    const fallback = commandInputRef.current ?? projectInputRef.current;
    if (fallback !== null) {
      fallback.focus();
      return;
    }
    moveCursor(-1);
  }, [showWorktreeToggle, moveCursor]);

  if (tool === null) {
    // Bug at the shell level (CommandPalette should render
    // ToolSelectPhase when selectedToolId is null). Render null so the
    // user sees nothing rather than crashing.
    return null;
  }

  // advanceOrConfirm: Enter on a field. Final field → move focus to the
  // explicit confirm button (selection alone never submits — submission
  // is a distinct user action); otherwise → moveCursor(+1).
  function advanceOrSubmit(): void {
    if (composing) return;
    if (isFinalField(params, paramCursor)) {
      submitRef.current?.focus();
      return;
    }
    moveCursor(+1);
  }

  function selectProject(value: unknown): void {
    const projectTarget = typeof value === "string" ? value : "";
    const project = findProject(snapshot.projects, projectTarget);
    // Preserve command/profile fields so scoped project re-bind does not wipe
    // the command leading-option preset mid-flow (smoke Enter → confirm).
    const next: Record<string, unknown> = {
      ...paramValues,
      project: projectTarget,
      target: { source: "project", project: projectTarget } satisfies NewSessionTarget,
    };
    if (project?.isGit !== true) next.worktree = undefined;
    setParams(next);
  }

  function selectRepository(repository: {
    repository_id: string;
    owner: string;
    name: string;
  }): void {
    setParams({
      target: { source: "repository", repository } satisfies NewSessionTarget,
      executionServerId: resolvedExecutionServerId,
    });
  }

  function selectCommand(value: unknown): void {
    const command = typeof value === "string" ? value : "";
    const profile = selectedProject?.commandProfiles?.find((candidate) => candidate.id === command);
    const next: Record<string, unknown> = {
      project: paramValues.project,
      target: paramValues.target,
      command,
    };
    if (paramValues.executionServerId !== undefined) {
      next.executionServerId = paramValues.executionServerId;
    }
    if (paramValues.worktree !== undefined) next.worktree = paramValues.worktree;
    for (const parameter of profile?.parameters ?? []) {
      if (parameter.default !== undefined) {
        next[profileParameterID(parameter.name)] = parameter.default;
      }
    }
    setParams(next);
  }

  // FR-A4: a dynamic-options param with zero materialized options
  // renders the empty-state body AND suppresses every subsequent param
  // + the submit-trigger Enter handler the final field would carry.
  // new-session project uses TargetPicker (GitHub may still be available).
  let stopIdx = params.length;
  for (let i = 0; i < params.length; i++) {
    const p = params[i];
    if (p === undefined) continue;
    if (p.id === "project" && selectedToolId === "new-session") continue;
    const opts = materialized[i] ?? null;
    if (p.kind === "dynamic-options" && opts !== null && opts.length === 0) {
      stopIdx = i + 1;
      break;
    }
  }

  return (
    <form
      className="palette-param-select"
      aria-label="palette parameters"
      onSubmit={(e) => {
        // The explicit confirm button (type=submit) routes here. Store
        // submit() re-validates disabledReason and routes errors.
        e.preventDefault();
        if (composing || submitting) return;
        void submit(ctx);
      }}
    >
      {paramCursor === 1 && selectedTarget && (
        <div className="palette-project-summary" data-testid="palette-project-summary">
          <span>
            {selectedTarget.source === "repository"
              ? `${selectedTarget.repository.owner}/${selectedTarget.repository.name}`
              : selectedProject?.serverLabel
                ? `${selectedProject.serverLabel} · ${selectedProject.path}`
                : selectedProject?.path}
          </span>
          <button type="button" onClick={() => moveCursor(-1)}>
            Change
          </button>
        </div>
      )}
      {paramCursor === 1 && resolvedExecutionServerId && (
        <ExecutionServerSelector
          servers={snapshot.serverOutcomes}
          value={resolvedExecutionServerId}
          disabled={submitting}
          onChange={(serverId) => setParam("executionServerId", serverId)}
        />
      )}
      {params.slice(0, stopIdx).map((param, idx) => {
        if (idx !== paramCursor) return null;
        const isCurrent = idx === paramCursor;
        const value = paramValues[param.id];
        const options = materialized[idx] ?? null;

        const isCommand = param.id === "command";
        const isProject = param.id === "project";
        const chips = null;

        // Categorized project+repository picker replaces the plain project listbox.
        // Always mount for new-session so GitHub category works even when projects are empty.
        if (isProject && selectedToolId === "new-session") {
          return (
            <div key={param.id} className="palette-param-text-group">
              <TargetPicker
                focused={isCurrent}
                disabled={submitting}
                projects={snapshot.projects}
                value={value}
                onSelectProject={(v) => selectProject(v)}
                onSelectRepository={selectRepository}
                onEnter={advanceOrSubmit}
                onAnnounce={announcePreparation}
                executionServers={snapshot.serverOutcomes}
                inputRef={projectInputRef}
              />
            </div>
          );
        }

        // FR-A4: dynamic-options with zero materialized options renders
        // the empty-state body. ParamEmptyState carries no key handlers
        // so Esc still bubbles to the palette shell. The message is
        // materializeKey-specific so the user sees "add a project" vs.
        // "configure commands" instead of one misleading line.
        if (param.kind === "dynamic-options" && options !== null && options.length === 0) {
          const emptyMessage =
            param.materializeKey === "commands"
              ? "No commands configured - add [session].commands in settings.toml"
              : "No projects available - add a project first";
          return (
            <fieldset
              key={param.id}
              className={`palette-param ${isCurrent ? "focused" : ""}`}
              data-param-id={param.id}
              aria-label={param.label}
            >
              <div className="palette-param-label">{param.label}</div>
              <ParamEmptyState message={emptyMessage} />
            </fieldset>
          );
        }

        if (options !== null) {
          // Listbox (static-options / dynamic-options with N>=1).
          return (
            <div key={param.id} className="palette-param-text-group">
              <ParamListbox
                paramId={param.id}
                label={param.label}
                options={options}
                value={value}
                focused={isCurrent}
                disabled={submitting}
                composing={composing}
                onSelect={(v: unknown) => {
                  if (isProject) selectProject(v);
                  else if (isCommand) selectCommand(v);
                  else setParam(param.id, v);
                }}
                onEnter={advanceOrSubmit}
                onCompositionStart={() => setComposing(true)}
                onCompositionEnd={() => setComposing(false)}
                inputRef={isCommand ? commandInputRef : isProject ? projectInputRef : undefined}
              />
              {chips}
            </div>
          );
        }
        // Free-form text input (param.kind === 'text').
        return (
          <div key={param.id} className="palette-param-text-group">
            <ParamTextInput
              ref={isCommand ? commandInputRef : undefined}
              paramId={param.id}
              label={param.label}
              value={typeof value === "string" ? value : ""}
              focused={isCurrent}
              disabled={submitting}
              composing={composing}
              onChange={(v) => setParam(param.id, v)}
              onEnter={advanceOrSubmit}
              onCompositionStart={() => setComposing(true)}
              onCompositionEnd={() => setComposing(false)}
            />
            {chips}
          </div>
        );
      })}
      {paramCursor === 1 && selectedProfile && (
        <ProfileArgs
          parameters={selectedProfile.parameters}
          values={paramValues}
          disabled={submitting}
          onChange={setParam}
        />
      )}
      {paramCursor === 1 && selectedProfile && (
        <div className="palette-actions" data-testid="palette-actions">
          <fieldset className="palette-actions__options" aria-label="session options">
            {showWorktreeToggle && (
              <ChipSwitch
                hintKey="W"
                label="worktree"
                checked={worktreeOn}
                onToggle={toggleWorktree}
                disabled={submitting}
                composing={composing}
                testId="worktree"
              />
            )}
          </fieldset>
          <button
            ref={submitRef}
            type="submit"
            className="palette-submit"
            data-testid="palette-submit"
            disabled={submitting}
          >
            {submitting ? "Working…" : (tool.label ?? "Confirm")}
          </button>
        </div>
      )}
    </form>
  );
}
