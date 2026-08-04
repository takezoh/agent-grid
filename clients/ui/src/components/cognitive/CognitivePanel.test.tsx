import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { sessionKey } from "../../fleet/session-protocol";
import { useDaemonStore } from "../../store/daemon";
import type { SessionInfo } from "../../wire/server";
import { CognitivePanel } from "./CognitivePanel";

const FLEET_KEY = sessionKey({ serverId: "test-server", sessionId: "s1" });

function makeSession(overrides: Partial<SessionInfo> = {}): SessionInfo {
  return {
    id: FLEET_KEY,
    upstreamSessionId: "s1",
    serverId: "test-server",
    serverLabel: "Test Server",
    project: "/workspace/agent-grid",
    command: "claude",
    created_at: "2026-08-04T09:00:00Z",
    status: { lifecycle: "active", human_input: false, failure: "none" },
    view: { card: { title: "Login fix" } },
    ...overrides,
  } as SessionInfo;
}

function capsuleBody(overrides: Record<string, unknown> = {}) {
  return {
    resume_capsules: [
      {
        session_id: "s1",
        project: "/workspace/agent-grid",
        title: "Login fix",
        objective: "Ship the login fix",
        current_state: "Root cause identified",
        next_action: "Write the regression test",
        unresolved: ["Flaky CI"],
        generated_at: "2026-08-04T10:00:00Z",
        source_turns: 4,
        ...overrides,
      },
    ],
  };
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  useDaemonStore.setState({ sessions: [makeSession()] });
});

describe("CognitivePanel", () => {
  it("renders capsule cards with objective, state, next action and provenance", async () => {
    const fetchFn = vi.fn(async () => jsonResponse(capsuleBody()));
    render(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);

    expect(await screen.findByText("Ship the login fix")).not.toBeNull();
    expect(screen.getByText("Root cause identified")).not.toBeNull();
    expect(screen.getByText("Write the regression test")).not.toBeNull();
    expect(screen.getByText("Flaky CI")).not.toBeNull();
    expect(screen.getByText("4 turns")).not.toBeNull();
    expect(screen.queryByText("Stale")).toBeNull();
    // No completion fields in the fixture → the completion block is hidden.
    expect(screen.queryByTestId("capsule-completion")).toBeNull();
  });

  it("renders the completion report sections when present", async () => {
    const fetchFn = vi.fn(async () =>
      jsonResponse(
        capsuleBody({
          changed: ["Fixed the token refresh path"],
          verified: ["Unit tests pass"],
          remaining: ["Add the regression test"],
          attention: "Review the fix before release",
        }),
      ),
    );
    render(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);

    const completion = await screen.findByTestId("capsule-completion");
    expect(completion.textContent).toContain("Changed");
    expect(completion.textContent).toContain("Fixed the token refresh path");
    expect(completion.textContent).toContain("Verified");
    expect(completion.textContent).toContain("Unit tests pass");
    expect(completion.textContent).toContain("Remaining");
    expect(completion.textContent).toContain("Add the regression test");
    expect(completion.textContent).toContain("Your attention");
    expect(completion.textContent).toContain("Review the fix before release");
  });

  it("marks the capsule stale when session activity is newer than generation", async () => {
    useDaemonStore.setState({
      sessions: [
        makeSession({
          status: {
            lifecycle: "active",
            human_input: false,
            failure: "none",
            changed_at: "2026-08-04T11:00:00Z",
          },
        }),
      ],
    });
    const fetchFn = vi.fn(async () => jsonResponse(capsuleBody()));
    render(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);

    expect(await screen.findByText("Stale")).not.toBeNull();
  });

  it("posts the wrong-flag and toggles the card state", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/resume-capsule/flag")) {
        return jsonResponse({ flagged: true });
      }
      return jsonResponse(capsuleBody());
    });
    render(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);

    fireEvent.click(await screen.findByRole("button", { name: "Mark as wrong" }));

    await waitFor(() => {
      expect(screen.getByText("Flagged wrong")).not.toBeNull();
    });
    expect(screen.getByRole("button", { name: "Clear wrong-flag" })).not.toBeNull();
    expect(fetchFn).toHaveBeenCalledWith(
      "/host/servers/test-server/api/sessions/s1/resume-capsule/flag",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("shows the empty state with the configuration hint", async () => {
    const fetchFn = vi.fn(async () => jsonResponse({ resume_capsules: [] }));
    render(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);

    expect(await screen.findByText("No resume capsules yet.")).not.toBeNull();
    expect(screen.getByText(/capsule_command/)).not.toBeNull();
  });

  it("reports unreachable servers without failing the panel", async () => {
    const fetchFn = vi.fn(async () => jsonResponse({}, 503));
    render(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);

    expect(await screen.findByText(/Some servers could not be reached/)).not.toBeNull();
    expect(screen.getByText(/test-server/)).not.toBeNull();
  });

  it("does not fetch while inactive and fetches once activated", async () => {
    const fetchFn = vi.fn(async () => jsonResponse(capsuleBody()));
    const { rerender } = render(
      <CognitivePanel active={false} fetchFn={fetchFn as typeof fetch} />,
    );
    expect(fetchFn).not.toHaveBeenCalled();

    rerender(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);
    expect(await screen.findByText("Ship the login fix")).not.toBeNull();
  });

  it("refetches on manual refresh", async () => {
    const fetchFn = vi.fn(async () => jsonResponse(capsuleBody()));
    render(<CognitivePanel active fetchFn={fetchFn as typeof fetch} />);
    await screen.findByText("Ship the login fix");
    const callsBefore = fetchFn.mock.calls.length;

    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));

    await waitFor(() => {
      expect(fetchFn.mock.calls.length).toBeGreaterThan(callsBefore);
    });
  });
});
