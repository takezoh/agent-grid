import { describe, expect, it, vi } from "vitest";
import type { DaemonConfigResolver } from "../src/main/daemon-config.js";
import type { AppearanceConfig } from "../src/main/desktop-config.js";
import {
  createElectronWindowFactory,
  sessionPageUrl,
  type ChromeRect,
  type ElectronBaseWindowConstructor,
  type ElectronFactoryOptions,
  type ElectronWebContentsViewConstructor,
  type ElectronWebContentsViewLike,
  type SettingsWindowHandle,
} from "../src/main/electron-window-factory.js";
import { TITLE_BAR_HEIGHT, titleBarOverlayFor } from "../src/main/window-chrome.js";

// ─── fakes ───────────────────────────────────────────────────────────────────

function appearance(theme: AppearanceConfig["theme"]): AppearanceConfig {
  return { theme, density: "comfortable", font_scale: 1 };
}

interface Recorded {
  windowOptions: Record<string, unknown>[];
  viewOptions: Record<string, unknown>[];
  viewBounds: ChromeRect[][];
  viewLoads: string[][];
  viewScripts: string[][];
  childViews: ElectronWebContentsViewLike[];
  resizeListeners: (() => void)[];
  settingsHandle?: SettingsWindowHandle;
}

function makeFakes(contentBounds: ChromeRect): {
  BaseWindow: ElectronBaseWindowConstructor;
  WebContentsView: ElectronWebContentsViewConstructor;
  recorded: Recorded;
} {
  const recorded: Recorded = {
    windowOptions: [],
    viewOptions: [],
    viewBounds: [],
    viewLoads: [],
    viewScripts: [],
    childViews: [],
    resizeListeners: [],
  };
  let nextWebContentsId = 1;

  class FakeWebContentsView {
    bounds: ChromeRect[] = [];
    loads: string[] = [];
    scripts: string[] = [];
    webContents: {
      id: number;
      once(event: string, listener: () => void): void;
      executeJavaScript(code: string): Promise<unknown>;
      loadURL(url: string): Promise<void>;
    };

    constructor(options: Record<string, unknown>) {
      recorded.viewOptions.push(options);
      recorded.viewBounds.push(this.bounds);
      recorded.viewLoads.push(this.loads);
      recorded.viewScripts.push(this.scripts);
      const loads = this.loads;
      const scripts = this.scripts;
      this.webContents = {
        id: nextWebContentsId++,
        once() {},
        executeJavaScript: (code: string) => {
          scripts.push(code);
          return Promise.resolve();
        },
        loadURL: (url: string) => {
          loads.push(url);
          return Promise.resolve();
        },
      };
    }

    setBounds(b: ChromeRect): void {
      this.bounds.push(b);
    }
  }

  class FakeBaseWindow {
    contentView = {
      addChildView: (view: ElectronWebContentsViewLike) => {
        recorded.childViews.push(view);
      },
    };

    constructor(options: Record<string, unknown>) {
      recorded.windowOptions.push(options);
    }

    focus(): void {}
    close(): void {}
    isDestroyed(): boolean {
      return false;
    }
    getBounds(): ChromeRect {
      return contentBounds;
    }
    setBounds(): void {}
    getContentBounds(): ChromeRect {
      return contentBounds;
    }
    on(event: string, listener: () => void): void {
      if (event === "resize") recorded.resizeListeners.push(listener);
    }
  }

  return {
    BaseWindow: FakeBaseWindow as unknown as ElectronBaseWindowConstructor,
    WebContentsView: FakeWebContentsView as unknown as ElectronWebContentsViewConstructor,
    recorded,
  };
}

function createHome(
  overrides: Partial<ElectronFactoryOptions>,
  resolveImpl: DaemonConfigResolver["resolve"] = () =>
    Promise.resolve({ baseUrl: "http://127.0.0.1:1", token: "t" }),
): Recorded {
  const { BaseWindow, WebContentsView, recorded } = makeFakes({
    x: 0,
    y: 0,
    width: 900,
    height: 700,
  });
  const factory = createElectronWindowFactory({
    config: { resolve: resolveImpl } as unknown as DaemonConfigResolver,
    BaseWindow,
    WebContentsView,
    preloadPath: "preload.cjs",
    titleBarPreloadPath: "titlebar-preload.cjs",
    uiEntryUrl: "file:///ui/index.html",
    ...overrides,
  });
  factory.createHome("srv");
  return recorded;
}

function createSettings(
  overrides: Partial<ElectronFactoryOptions> = {},
  section: "connections" | "github" = "connections",
): Recorded {
  const { BaseWindow, WebContentsView, recorded } = makeFakes({
    x: 0,
    y: 0,
    width: 900,
    height: 700,
  });
  const factory = createElectronWindowFactory({
    config: { resolve: () => Promise.resolve({ baseUrl: "http://127.0.0.1:1", token: "t" }) } as unknown as DaemonConfigResolver,
    BaseWindow,
    WebContentsView,
    preloadPath: "preload.cjs",
    titleBarPreloadPath: "titlebar-preload.cjs",
    uiEntryUrl: "file:///ui/index.html",
    ...overrides,
  });
  recorded.settingsHandle = factory.createSettings(section);
  return recorded;
}

// ─── VSCode-style window chrome ──────────────────────────────────────────────

describe("frameless window chrome", () => {
  it("creates a frameless window with a Window Controls Overlay", () => {
    const recorded = createHome({ appearance: appearance("dark") });
    expect(recorded.windowOptions).toHaveLength(1);
    const options = recorded.windowOptions[0]!;
    expect(options.titleBarStyle).toBe("hidden");
    expect(options.titleBarOverlay).toEqual(titleBarOverlayFor("dark"));
    expect(options.backgroundColor).toBe("#0b0c0e");
  });

  it("light appearance paints the light overlay and background", () => {
    const recorded = createHome({ appearance: appearance("light") });
    expect(recorded.windowOptions[0]!.titleBarOverlay).toEqual(titleBarOverlayFor("light"));
    expect(recorded.windowOptions[0]!.backgroundColor).toBe("#fafafa");
  });

  it("system appearance consults the injected OS probe (default dark)", () => {
    const light = createHome({
      appearance: appearance("system"),
      systemPrefersDark: () => false,
    });
    expect(light.windowOptions[0]!.titleBarOverlay).toEqual(titleBarOverlayFor("light"));
    const fallback = createHome({});
    expect(fallback.windowOptions[0]!.titleBarOverlay).toEqual(titleBarOverlayFor("dark"));
  });

  it("stacks the title-bar view above the SPA view and splits the bounds", () => {
    const recorded = createHome({});
    expect(recorded.childViews).toHaveLength(2);
    // First constructed view is the title bar, second the SPA content.
    const titleBarBounds = recorded.viewBounds[0]!;
    const contentBounds = recorded.viewBounds[1]!;
    expect(titleBarBounds[titleBarBounds.length - 1]).toEqual({
      x: 0,
      y: 0,
      width: 900,
      height: TITLE_BAR_HEIGHT,
    });
    expect(contentBounds[contentBounds.length - 1]).toEqual({
      x: 0,
      y: TITLE_BAR_HEIGHT,
      width: 900,
      height: 700 - TITLE_BAR_HEIGHT,
    });
    // Window resizes re-run the same layout.
    expect(recorded.resizeListeners).toHaveLength(1);
  });

  it("loads the menu-bar document in the title bar and the SPA in the content view", async () => {
    const recorded = createHome({});
    await Promise.resolve();
    const titleBarUrl = recorded.viewLoads[0]![0]!;
    expect(titleBarUrl.startsWith("data:text/html")).toBe(true);
    expect(decodeURIComponent(titleBarUrl)).toContain('data-menu="file"');
    expect(recorded.viewLoads[1]).toContain("file:///ui/index.html");
  });

  it("keeps SPA isolation: partition + preload on content, menu preload on title bar", () => {
    const recorded = createHome({});
    const titleBarPrefs = recorded.viewOptions[0]!.webPreferences as Record<string, unknown>;
    const contentPrefs = recorded.viewOptions[1]!.webPreferences as Record<string, unknown>;
    expect(titleBarPrefs.preload).toBe("titlebar-preload.cjs");
    expect(titleBarPrefs.partition).toBeUndefined();
    expect(contentPrefs.preload).toBe("preload.cjs");
    expect(contentPrefs.partition).toBe("persist:agent-grid-srv");
  });

  it("hands the chrome handles to the entry for menu/theme IPC", () => {
    const handles: unknown[] = [];
    createHome({ registerWindowChrome: (h) => handles.push(h) });
    expect(handles).toHaveLength(1);
    const chrome = handles[0] as {
      titleBarWebContentsId: number;
      contentWebContentsId: number;
    };
    expect(chrome.titleBarWebContentsId).not.toBe(chrome.contentWebContentsId);
  });

  it("resolves daemon auth for the home window so the shared SPA can connect", async () => {
    const resolve = vi.fn().mockResolvedValue({ baseUrl: "http://127.0.0.1:1", token: "t" });
    const registered: unknown[] = [];
    const recorded = createHome(
      {
        registerRendererInfo: (_webContentsId, info) => registered.push(info),
      },
      resolve,
    );

    await Promise.resolve();
    await Promise.resolve();

    expect(resolve).toHaveBeenCalledWith("srv");
    expect(recorded.viewLoads[1]).toContain("file:///ui/index.html");
    expect(registered).toEqual([
      {
        hosted: false,
        baseUrl: "http://127.0.0.1:1",
        token: "t",
      },
    ]);
  });

  it("loads the settings document in a dedicated settings window", async () => {
    const recorded = createSettings();
    await Promise.resolve();
    expect(recorded.viewLoads[1]![0]).toMatch(/^data:text\/html/);
    expect(decodeURIComponent(recorded.viewLoads[1]![0]!)).toContain("These entries are not a switcher");
  });

  it("opens GitHub Settings directly and can retarget an existing window", async () => {
    const recorded = createSettings({}, "github");
    await Promise.resolve();
    expect(recorded.viewLoads[1]![0]).toMatch(/#github$/);
    recorded.settingsHandle?.showSection("github");
    expect(recorded.viewScripts[1]?.at(-1)).toContain("#github");
  });

  it("does not resolve daemon auth for the dedicated settings window", async () => {
    const resolve = vi.fn().mockResolvedValue({ baseUrl: "http://127.0.0.1:1", token: "t" });
    createSettings({ config: { resolve } as unknown as DaemonConfigResolver });
    await Promise.resolve();
    expect(resolve).not.toHaveBeenCalled();
  });
});

// ─── existing URL contract (kept close to the factory) ───────────────────────

describe("sessionPageUrl", () => {
  it("marks hosted session pages and rejects non-file entries", () => {
    expect(sessionPageUrl("file:///ui/index.html", "s1")).toBe(
      "file:///ui/index.html?hosted=1&session=s1",
    );
    expect(() => sessionPageUrl("http://example.com/index.html", "s1")).toThrow();
  });
});

describe("session windows", () => {
  it("keep loading the shared SPA and resolve daemon config", async () => {
    const resolve = vi.fn().mockResolvedValue({ baseUrl: "http://127.0.0.1:1", token: "t" });
    const { BaseWindow, WebContentsView, recorded } = makeFakes({
      x: 0,
      y: 0,
      width: 900,
      height: 700,
    });
    const factory = createElectronWindowFactory({
      config: { resolve } as unknown as DaemonConfigResolver,
      BaseWindow,
      WebContentsView,
      preloadPath: "preload.cjs",
      titleBarPreloadPath: "titlebar-preload.cjs",
      uiEntryUrl: "file:///ui/index.html",
    });

    factory.create({ serverId: "srv", sessionId: "sess-1" });
    await Promise.resolve();
    await Promise.resolve();

    expect(resolve).toHaveBeenCalledWith("srv");
    expect(recorded.viewLoads[1]).toContain("file:///ui/index.html?hosted=1&session=sess-1");
  });
});
