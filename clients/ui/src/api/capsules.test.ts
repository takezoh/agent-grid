import { describe, expect, it, vi } from "vitest";
import { sessionKey } from "../fleet/session-protocol";
import { makeCapsulesApi } from "./capsules";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("makeCapsulesApi", () => {
  it("merges capsules across servers and qualifies fleet paths", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/host/servers/alpha/")) {
        return jsonResponse({
          resume_capsules: [
            {
              session_id: "s1",
              project: "/repo/a",
              objective: "ship it",
              generated_at: "2026-08-04T00:00:00Z",
            },
          ],
        });
      }
      return jsonResponse({ resume_capsules: [] });
    });

    const api = makeCapsulesApi(fetchFn as typeof fetch);
    const result = await api.listAll([
      { serverId: "alpha", serverLabel: "Alpha" },
      { serverId: "beta" },
    ]);

    expect(fetchFn).toHaveBeenCalledWith(
      "/host/servers/alpha/api/resume-capsules",
      expect.anything(),
    );
    expect(fetchFn).toHaveBeenCalledWith(
      "/host/servers/beta/api/resume-capsules",
      expect.anything(),
    );
    expect(result.errors).toEqual([]);
    expect(result.capsules).toHaveLength(1);
    const capsule = result.capsules[0];
    expect(capsule?.serverId).toBe("alpha");
    expect(capsule?.serverLabel).toBe("Alpha");
    expect(capsule?.fleetSessionKey).toBe(sessionKey({ serverId: "alpha", sessionId: "s1" }));
  });

  it("collects per-server errors without failing the whole list", async () => {
    const fetchFn = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/host/servers/down/")) {
        return jsonResponse({ error: "boom" }, 503);
      }
      return jsonResponse({
        resume_capsules: [
          { session_id: "s2", project: "/repo/b", generated_at: "2026-08-04T01:00:00Z" },
        ],
      });
    });

    const api = makeCapsulesApi(fetchFn as typeof fetch);
    const result = await api.listAll([{ serverId: "up" }, { serverId: "down" }]);

    expect(result.capsules).toHaveLength(1);
    expect(result.errors).toEqual([{ serverId: "down", message: "HTTP 503" }]);
  });

  it("sorts capsules newest first", async () => {
    const fetchFn = vi.fn(async () =>
      jsonResponse({
        resume_capsules: [
          { session_id: "old", project: "/a", generated_at: "2026-08-01T00:00:00Z" },
          { session_id: "new", project: "/b", generated_at: "2026-08-04T00:00:00Z" },
        ],
      }),
    );
    const api = makeCapsulesApi(fetchFn as typeof fetch);
    const result = await api.listAll([{ serverId: "alpha" }]);
    expect(result.capsules.map((c) => c.session_id)).toEqual(["new", "old"]);
  });

  it("posts the wrong-flag to the session-scoped endpoint", async () => {
    const fetchFn = vi.fn(async () => jsonResponse({ flagged: true }));
    const api = makeCapsulesApi(fetchFn as typeof fetch);

    await api.flag(sessionKey({ serverId: "alpha", sessionId: "s1" }), true);

    expect(fetchFn).toHaveBeenCalledWith(
      "/host/servers/alpha/api/sessions/s1/resume-capsule/flag",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ flagged: true }),
      }),
    );
  });

  it("rejects flagging with an invalid fleet key", async () => {
    const fetchFn = vi.fn();
    const api = makeCapsulesApi(fetchFn as typeof fetch);
    await expect(api.flag("not-a-fleet-key", true)).rejects.toThrow("invalid fleet session key");
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it("throws on a failed flag response", async () => {
    const fetchFn = vi.fn(async () => jsonResponse({}, 404));
    const api = makeCapsulesApi(fetchFn as typeof fetch);
    await expect(
      api.flag(sessionKey({ serverId: "alpha", sessionId: "s1" }), false),
    ).rejects.toThrow("HTTP 404");
  });
});
