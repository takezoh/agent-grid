import { describe, expect, it } from "vitest";
import {
  CAPTION_BUTTONS_RESERVE,
  TITLE_BAR_HEIGHT,
  TITLE_BAR_MENUS,
  buildTitleBarHtml,
  isTitleBarMenuId,
  layoutChrome,
  resolveChromeTheme,
  themeSourceFor,
  titleBarDataUrl,
  titleBarOverlayFor,
  windowDisplayTitle,
} from "../src/main/window-chrome.js";

describe("chrome theme", () => {
  it("maps explicit themes and seeds system/default from the OS preference", () => {
    expect(resolveChromeTheme("dark", false)).toBe("dark");
    expect(resolveChromeTheme("light", true)).toBe("light");
    expect(resolveChromeTheme("system", true)).toBe("dark");
    expect(resolveChromeTheme("system", false)).toBe("light");
    expect(resolveChromeTheme("default", true)).toBe("dark");
    expect(resolveChromeTheme(undefined, false)).toBe("light");
  });

  it("pins native menus to explicit choices and keeps 'system' on the OS", () => {
    // Native popup menus follow nativeTheme.themeSource. "system" (and any
    // unknown payload) must stay "system": pinning it would freeze
    // prefers-color-scheme and the SPA's system-mode resolution could never
    // see the real OS theme again.
    expect(themeSourceFor("dark")).toBe("dark");
    expect(themeSourceFor("light")).toBe("light");
    expect(themeSourceFor("system")).toBe("system");
    expect(themeSourceFor(undefined)).toBe("system");
    expect(themeSourceFor("default")).toBe("system");
    expect(themeSourceFor(42)).toBe("system");
  });

  it("overlay colors mirror the SPA theme tokens at title-bar height", () => {
    expect(titleBarOverlayFor("dark")).toEqual({
      color: "#0b0c0e",
      symbolColor: "#e8eaee",
      height: TITLE_BAR_HEIGHT,
    });
    expect(titleBarOverlayFor("light")).toEqual({
      color: "#fafafa",
      symbolColor: "#17181c",
      height: TITLE_BAR_HEIGHT,
    });
  });
});

describe("chrome layout", () => {
  it("splits the window into the title-bar strip and the SPA area", () => {
    expect(layoutChrome({ width: 800, height: 600 })).toEqual({
      titleBar: { x: 0, y: 0, width: 800, height: TITLE_BAR_HEIGHT },
      content: { x: 0, y: TITLE_BAR_HEIGHT, width: 800, height: 600 - TITLE_BAR_HEIGHT },
    });
  });

  it("degrades gracefully on tiny or empty windows", () => {
    expect(layoutChrome({ width: 100, height: 20 })).toEqual({
      titleBar: { x: 0, y: 0, width: 100, height: 20 },
      content: { x: 0, y: 20, width: 100, height: 0 },
    });
    expect(layoutChrome({ width: -5, height: -5 })).toEqual({
      titleBar: { x: 0, y: 0, width: 0, height: 0 },
      content: { x: 0, y: 0, width: 0, height: 0 },
    });
  });
});

describe("title-bar document", () => {
  it("renders every menu-bar label as a no-drag button on a draggable strip", () => {
    const html = buildTitleBarHtml("dark", "Agent Grid — local");
    expect(html).toContain("-webkit-app-region:drag");
    expect(html).toContain("-webkit-app-region:no-drag");
    for (const menu of TITLE_BAR_MENUS) {
      expect(html).toContain(`data-menu="${menu.id}">${menu.label}</button>`);
    }
    expect(html).toContain("Agent Grid — local");
    expect(html).toContain(`width:${CAPTION_BUTTONS_RESERVE}px`);
  });

  it("escapes markup in the window title", () => {
    const html = buildTitleBarHtml("light", '<img src=x onerror="pwn"> & "q"');
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;img src=x onerror=&quot;pwn&quot;&gt; &amp; &quot;q&quot;");
  });

  it("seeds the initial data-theme and both palettes for later switches", () => {
    const dark = buildTitleBarHtml("dark", "t");
    expect(dark).toContain('<html data-theme="dark">');
    expect(dark).toContain('html[data-theme="light"] body');
    expect(buildTitleBarHtml("light", "t")).toContain('<html data-theme="light">');
  });

  it("packs the document into a data: URL", () => {
    const url = titleBarDataUrl("dark", "t");
    expect(url.startsWith("data:text/html;charset=utf-8,")).toBe(true);
    expect(decodeURIComponent(url)).toContain('data-menu="file"');
  });
});

describe("menu-bar contract", () => {
  it("accepts only declared menu ids from the untrusted IPC payload", () => {
    expect(isTitleBarMenuId("file")).toBe(true);
    expect(isTitleBarMenuId("edit")).toBe(true);
    expect(isTitleBarMenuId("help")).toBe(false);
    expect(isTitleBarMenuId(42)).toBe(false);
    expect(isTitleBarMenuId(undefined)).toBe(false);
  });
});

describe("window titles", () => {
  it("labels home and session windows distinctly", () => {
    expect(windowDisplayTitle("local")).toBe("Agent Grid — local");
    expect(windowDisplayTitle("local", "s1")).toBe("local · s1");
  });
});
