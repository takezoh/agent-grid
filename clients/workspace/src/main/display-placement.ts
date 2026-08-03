import type {
  StateStore,
  WindowBounds,
  WindowHandle,
  WorkspaceDisplayState,
} from "./window-registry.js";

export interface ScreenDisplayLike {
  id: number;
  label: string;
  bounds: WindowBounds;
  workArea: WindowBounds;
}

export interface DisplayScreenLike {
  getAllDisplays(): ScreenDisplayLike[];
  getPrimaryDisplay(): ScreenDisplayLike;
  getDisplayMatching(bounds: WindowBounds): ScreenDisplayLike;
  dipToScreenPoint?(point: { x: number; y: number }): { x: number; y: number };
}

export interface DisplayAnchor {
  x: number;
  y: number;
}

export interface DisplayChoice {
  id: string;
  label: string;
  primary: boolean;
  anchor: DisplayAnchor;
}

export function listDisplayChoices(screen: DisplayScreenLike): DisplayChoice[] {
  const primaryId = screen.getPrimaryDisplay().id;
  return screen.getAllDisplays().map((display, index) => {
    const center = {
      x: Math.round(display.bounds.x + display.bounds.width / 2),
      y: Math.round(display.bounds.y + display.bounds.height / 2),
    };
    const anchor = screen.dipToScreenPoint?.(center) ?? center;
    return {
      id: String(display.id),
      label: display.label.trim() || `Display ${index + 1}`,
      primary: display.id === primaryId,
      anchor: { x: Math.round(anchor.x), y: Math.round(anchor.y) },
    };
  });
}

export class WorkspaceDisplayController {
  constructor(
    private readonly store: StateStore,
    private readonly screen: DisplayScreenLike,
  ) {}

  initialBounds(
    requested: WindowBounds | undefined,
    size: { width: number; height: number },
  ): WindowBounds | undefined {
    const saved = this.store.load()?.last_display;
    if (!saved) return requested;
    const target = resolveSavedDisplay(saved, this.screen);
    if (requested && String(this.screen.getDisplayMatching(requested).id) === String(target.id)) {
      return clampToWorkArea(requested, target.workArea);
    }
    return centeredBounds(requested ?? size, target.workArea);
  }

  track(window: Pick<WindowHandle, "isDestroyed" | "getBounds"> & {
    on?(event: "moved", listener: () => void): void;
  }): void {
    const capture = () => {
      if (window.isDestroyed()) return;
      const display = this.screen.getDisplayMatching(window.getBounds());
      const state = this.store.load() ?? {
        schema_version: 3 as const,
        windows: {},
        last_display: null,
      };
      state.last_display = {
        id: String(display.id),
        bounds: { ...display.bounds },
      };
      this.store.save(state);
    };
    capture();
    window.on?.("moved", capture);
  }
}

export function resolveSavedDisplay(
  saved: WorkspaceDisplayState,
  screen: DisplayScreenLike,
): ScreenDisplayLike {
  const displays = screen.getAllDisplays();
  const byId = displays.find((display) => String(display.id) === saved.id);
  if (byId) return byId;
  let best: ScreenDisplayLike | null = null;
  let bestArea = 0;
  for (const display of displays) {
    const area = intersectionArea(saved.bounds, display.bounds);
    if (area > bestArea) {
      best = display;
      bestArea = area;
    }
  }
  return best ?? screen.getPrimaryDisplay();
}

export function centeredBounds(
  size: { width: number; height: number },
  workArea: WindowBounds,
): WindowBounds {
  const width = Math.min(Math.max(1, size.width), workArea.width);
  const height = Math.min(Math.max(1, size.height), workArea.height);
  return {
    x: workArea.x + Math.round((workArea.width - width) / 2),
    y: workArea.y + Math.round((workArea.height - height) / 2),
    width,
    height,
  };
}

function clampToWorkArea(bounds: WindowBounds, workArea: WindowBounds): WindowBounds {
  const width = Math.min(Math.max(1, bounds.width), workArea.width);
  const height = Math.min(Math.max(1, bounds.height), workArea.height);
  return {
    x: Math.min(Math.max(bounds.x, workArea.x), workArea.x + workArea.width - width),
    y: Math.min(Math.max(bounds.y, workArea.y), workArea.y + workArea.height - height),
    width,
    height,
  };
}

function intersectionArea(a: WindowBounds, b: WindowBounds): number {
  const width = Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x));
  const height = Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y));
  return width * height;
}
