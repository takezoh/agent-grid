import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTranscriptStore } from "../store/transcripts";
import type { LogTab } from "../wire/server";
import { LogTabs, kindOfTab } from "./LogTabs";

function makeFetch(status: number, body: string): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(body, {
      status,
      headers: { "Content-Type": "text/plain" },
    }),
  ) as unknown as typeof fetch;
}

const nopFetch = makeFetch(204, "");

const TABS: LogTab[] = [
  { label: "TRANSCRIPT", path: "/sessions/s1/x.transcript", kind: "text" },
  { label: "EVENT-LOG", path: "/sessions/s1/x.event-log", kind: "text" },
];

describe("LogTabs", () => {
  beforeEach(() => {
    useTranscriptStore.getState().reset();
    vi.clearAllMocks();
  });

  it("TestRendersNullWhenNoTabs: returns null when tabs is empty", () => {
    const { container } = render(
      <LogTabs tabs={[]} sessionId="s1" bearerToken="tok" fetchFn={nopFetch} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("TestRendersTabsList: renders a button for each tab", () => {
    render(<LogTabs tabs={TABS} sessionId="s1" bearerToken="tok" fetchFn={nopFetch} />);
    const buttons = screen.getAllByRole("tab");
    expect(buttons).toHaveLength(2);
    expect(buttons[0]?.textContent).toBe("TRANSCRIPT");
    expect(buttons[1]?.textContent).toBe("EVENT-LOG");
  });

  // Regression 2026-06-24: real-driver shaped tabs (TRANSCRIPT + EVENTS = label
  // "EVENTS" / path "<sid>.log") must render both buttons even though only the
  // EVENTS path uses the .log suffix that kindOfTab newly resolves. This pins
  // the "tabs disappear from the UI" regression at the component layer.
  it("TestRendersRealDriverEventsAndTranscriptTabs: TRANSCRIPT + EVENTS (real driver shape) both render", () => {
    const realDriverTabs: LogTab[] = [
      { label: "TRANSCRIPT", path: "/sessions/s1/x.transcript", kind: "text" },
      { label: "EVENTS", path: "/var/log/agent-reactor/s1.log", kind: "text" },
    ];
    render(<LogTabs tabs={realDriverTabs} sessionId="s1" bearerToken="tok" fetchFn={nopFetch} />);
    const buttons = screen.getAllByRole("tab");
    expect(buttons).toHaveLength(2);
    expect(buttons[0]?.textContent).toBe("TRANSCRIPT");
    expect(buttons[1]?.textContent).toBe("EVENTS");
  });

  it("TestShowsBufferLines: lines in store appear inside <pre>", async () => {
    useTranscriptStore.getState().appendBackfill("s1", "transcript", ["alpha", "beta"], 10);

    render(<LogTabs tabs={TABS} sessionId="s1" bearerToken="tok" fetchFn={nopFetch} />);

    const pre = await screen.findByRole("tabpanel").then((p) => p.querySelector("pre"));
    expect(pre?.textContent).toBe("alpha\nbeta");
  });

  it("TestSwitchesTab: clicking a tab changes the active panel", async () => {
    useTranscriptStore.getState().appendBackfill("s1", "transcript", ["transcript-line"], 5);
    useTranscriptStore.getState().appendBackfill("s1", "event-log", ["event-line"], 5);

    render(<LogTabs tabs={TABS} sessionId="s1" bearerToken="tok" fetchFn={nopFetch} />);

    // Initially on first tab (transcript)
    let pre = document.querySelector("pre");
    expect(pre?.textContent).toBe("transcript-line");

    // Switch to second tab (event-log)
    const [, eventLogTab] = screen.getAllByRole("tab");
    act(() => {
      fireEvent.click(eventLogTab as HTMLElement);
    });

    pre = document.querySelector("pre");
    expect(pre?.textContent).toBe("event-line");
  });

  it("TestSuppressInfo: suppressInfo=true hides INFO tab content", () => {
    const infoTabs: LogTab[] = [{ label: "INFO", path: "/x.transcript", kind: "text" }];

    render(
      <LogTabs tabs={infoTabs} sessionId="s1" bearerToken="tok" fetchFn={nopFetch} suppressInfo />,
    );

    // tabpanel should be empty (suppressed)
    const panel = screen.getByRole("tabpanel");
    expect(panel.querySelector("pre")).toBeNull();
    // No text content from lines
    expect(panel.textContent).toBe("");
  });

  it("TestSuppressInfoFalse: suppressInfo=false shows INFO tab content normally", async () => {
    useTranscriptStore.getState().appendBackfill("s1", "transcript", ["info-line"], 5);

    const infoTabs: LogTab[] = [{ label: "INFO", path: "/x.transcript", kind: "text" }];

    render(
      <LogTabs
        tabs={infoTabs}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        suppressInfo={false}
      />,
    );

    const pre = await screen.findByRole("tabpanel").then((p) => p.querySelector("pre"));
    expect(pre?.textContent).toBe("info-line");
  });

  it("TestBottomPinned: content area has role=tabpanel with scroll container ref", () => {
    render(<LogTabs tabs={TABS} sessionId="s1" bearerToken="tok" fetchFn={nopFetch} />);

    // The tabpanel exists and wraps a pre element
    const panel = screen.getByRole("tabpanel");
    expect(panel).toBeDefined();
    expect(panel.className).toContain("log-tab-content");
  });

  it("TestRestBackfillHybrid: fetch 200 → store backfilled, then appendLine simulates WS tail", async () => {
    const fetchFn = makeFetch(200, "line1\nline2\n");

    render(<LogTabs tabs={TABS} sessionId="s1" bearerToken="tok" fetchFn={fetchFn} />);

    // Wait for REST backfill to populate store and re-render
    await act(async () => {
      await vi.waitFor(() => {
        const buf = useTranscriptStore.getState().buffers["s1:transcript"];
        expect(buf?.lines).toEqual(["line1", "line2"]);
      });
    });

    // Simulate WS tail arriving after backfill
    act(() => {
      useTranscriptStore.getState().appendLine("s1", "transcript", "line3");
    });

    const pre = document.querySelector("pre");
    expect(pre?.textContent).toBe("line1\nline2\nline3");
  });
});

describe("kindOfTab", () => {
  // FR-004 regression: existing path-suffix cases must remain unchanged
  it("TestKindOfTabHelper: detects transcript by path suffix", () => {
    expect(kindOfTab({ label: "TRANSCRIPT", path: "/x.transcript", kind: "text" })).toBe(
      "transcript",
    );
  });

  it("TestKindOfTabHelper: detects event-log by .event-log path suffix (FR-004)", () => {
    expect(kindOfTab({ label: "LOGS", path: "/x.event-log", kind: "text" })).toBe("event-log");
  });

  // FR-004 regression: existing label-fallback cases must remain unchanged
  it("TestKindOfTabHelper: detects transcript by label fallback (FR-004)", () => {
    expect(kindOfTab({ label: "transcript", path: "/x.txt", kind: "text" })).toBe("transcript");
  });

  it("TestKindOfTabHelper: detects event-log by exact label fallback (FR-004)", () => {
    expect(kindOfTab({ label: "event-log", path: "/x.txt", kind: "text" })).toBe("event-log");
  });

  it("TestKindOfTabHelper: returns null for unknown kind", () => {
    expect(kindOfTab({ label: "INFO", path: "/x.json", kind: "text" })).toBeNull();
  });

  // FR-001: path ending in .log or .jsonl → event-log
  it("TestKindOfTabHelper: detects event-log by .log path suffix (FR-001)", () => {
    expect(kindOfTab({ label: "ANYTHING", path: "/sessions/abc/abc.log", kind: "text" })).toBe(
      "event-log",
    );
  });

  it("TestKindOfTabHelper: detects event-log by .jsonl path suffix (FR-001)", () => {
    expect(kindOfTab({ label: "ANYTHING", path: "/sessions/abc/abc.jsonl", kind: "text" })).toBe(
      "event-log",
    );
  });

  // FR-002: label lowercased includes "events" or "event-log" → event-log
  it("TestKindOfTabHelper: detects event-log when label includes events (FR-002)", () => {
    expect(kindOfTab({ label: "EVENTS", path: "/x.txt", kind: "text" })).toBe("event-log");
  });

  it("TestKindOfTabHelper: detects event-log when label includes event-log substring (FR-002)", () => {
    expect(kindOfTab({ label: "My-event-log-tab", path: "/x.txt", kind: "text" })).toBe(
      "event-log",
    );
  });

  // FR-003: real driver EVENTS tab (label=EVENTS, path=<sid>.log) → event-log
  it("TestKindOfTabHelper: real driver EVENTS tab resolves to event-log (FR-003)", () => {
    expect(
      kindOfTab({ label: "EVENTS", path: "/sessions/sess-123/sess-123.log", kind: "text" }),
    ).toBe("event-log");
  });
});
