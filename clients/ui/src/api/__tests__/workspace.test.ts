import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { makeCapsulesApi } from "../capsules";
import { UnresolvedSessionError, makeSessionsApi } from "../sessions";
import { fetchSessionFile } from "../transcripts";
import { WorkspaceApiError, makeWorkspaceApi } from "../workspace";

function jsonResponse(value: unknown, status: number): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Sessions are addressed by fleet key; "local" maps to the same-origin /api
// routes, so the URLs asserted below are unchanged. pinned.sessionId stays
// bare on purpose — handle_session is a workspace handle, not a route.
const SESSION = JSON.stringify(["local", "s1"]);

describe("workspace save", () => {
  beforeEach(() => {
    window.location.hash = "";
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("PUTs raw body with If-Unmodified-Since on success", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ updated_mtime: "Mon, 02 Jan 2024 00:00:00 GMT", path: "foo.ts" }, 200),
      );
    const api = makeWorkspaceApi(fetchFn);
    const resp = await api.save(
      SESSION,
      "foo.ts",
      { sessionId: "s1", frameGeneration: 2, resolvedRootPath: "/workspace" },
      "hello",
      "Mon, 01 Jan 2024 00:00:00 GMT",
    );
    expect(resp.updated_mtime).toBe("Mon, 02 Jan 2024 00:00:00 GMT");
    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/sessions/s1/workspace/file");
    expect(url).toContain("path=foo.ts");
    expect(url).toContain("handle_session=s1");
    expect(url).toContain("root=%2Fworkspace");
    expect(init.method).toBe("PUT");
    expect(init.body).toBe("hello");
    expect((init.headers as Record<string, string>)["If-Unmodified-Since"]).toBe(
      "Mon, 01 Jan 2024 00:00:00 GMT",
    );
  });

  it("maps handle_stale (409)", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValue(jsonResponse({ error: "handle_stale", frame_generation: 3 }, 409));
    const api = makeWorkspaceApi(fetchFn);
    await expect(
      api.save(
        SESSION,
        "x.txt",
        { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" },
        "x",
      ),
    ).rejects.toSatisfy((err: unknown) => WorkspaceApiError.isHandleStale(err));
  });

  it("maps invalid_handle (400)", async () => {
    const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: "invalid_handle" }, 400));
    const api = makeWorkspaceApi(fetchFn);
    await expect(
      api.getFile(SESSION, "x.txt", {
        sessionId: "s1",
        frameGeneration: 1,
        resolvedRootPath: "/w",
      }),
    ).rejects.toMatchObject({ code: "invalid_handle", status: 400 });
  });

  it("maps precondition_failed (412)", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(
          { error: "precondition_failed", current_mtime: "Mon, 03 Jan 2024 00:00:00 GMT" },
          412,
        ),
      );
    const api = makeWorkspaceApi(fetchFn);
    await expect(
      api.save(
        SESSION,
        "x.txt",
        { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" },
        "x",
        "old",
      ),
    ).rejects.toSatisfy((err: unknown) => WorkspaceApiError.isPreconditionFailed(err));
  });

  it("maps oversize_body (413)", async () => {
    const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: "oversize_body" }, 413));
    const api = makeWorkspaceApi(fetchFn);
    await expect(
      api.save(
        SESSION,
        "x.txt",
        { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" },
        "x",
      ),
    ).rejects.toSatisfy((err: unknown) => WorkspaceApiError.isOversizeBody(err));
  });

  it("maps audit_emit_failed (500)", async () => {
    const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: "audit_emit_failed" }, 500));
    const api = makeWorkspaceApi(fetchFn);
    await expect(
      api.save(
        SESSION,
        "x.txt",
        { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" },
        "x",
      ),
    ).rejects.toSatisfy((err: unknown) => WorkspaceApiError.isAuditEmitFailed(err));
  });

  it("covers every read operation with qualified handle parameters", async () => {
    const fetchFn = vi.fn().mockImplementation(() => jsonResponse({ outcome: "ok" }, 200));
    const api = makeWorkspaceApi(fetchFn);
    const pinned = { sessionId: "s1", frameGeneration: 2, resolvedRootPath: "/workspace" };

    // The upstream id still needs percent-encoding in the path; it just
    // arrives inside a fleet key now.
    await api.getRootHandle(JSON.stringify(["local", "s/1"]));
    await api.getTree(SESSION, "", pinned);
    await api.getTree(SESSION, "src", pinned);
    await api.getDiff(SESSION, "", pinned);
    await api.getDiff(SESSION, "src/a.ts", pinned);

    expect(fetchFn).toHaveBeenCalledTimes(5);
    expect(String(fetchFn.mock.calls[0]?.[0])).toContain("/sessions/s%2F1/");
    expect(String(fetchFn.mock.calls[2]?.[0])).toContain("path=src");
    expect(String(fetchFn.mock.calls[4]?.[0])).toContain("path=src%2Fa.ts");
  });

  it("attaches bearer auth and maps an untyped unreadable failure", async () => {
    window.location.hash = "#token=secret";
    const response = {
      ok: false,
      status: 502,
      statusText: "Bad Gateway",
      text: vi.fn().mockRejectedValue(new Error("unreadable")),
    } as unknown as Response;
    const fetchFn = vi.fn().mockResolvedValue(response);

    await expect(makeWorkspaceApi(fetchFn).getRootHandle(SESSION)).rejects.toMatchObject({
      code: "http_error",
      status: 502,
    });
    expect(fetchFn).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: { Authorization: "Bearer secret" },
      }),
    );
  });

  it("maps an untyped save failure", async () => {
    const fetchFn = vi.fn().mockResolvedValue(new Response("nope", { status: 418 }));
    await expect(
      makeWorkspaceApi(fetchFn).save(
        SESSION,
        "x",
        { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" },
        "body",
      ),
    ).rejects.toMatchObject({ code: "http_error", status: 418 });
  });

  it("maps a save failure even when its body cannot be read", async () => {
    const response = {
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      text: vi.fn().mockRejectedValue(new Error("unreadable")),
    } as unknown as Response;
    await expect(
      makeWorkspaceApi(vi.fn().mockResolvedValue(response)).save(
        SESSION,
        "x",
        { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" },
        "body",
      ),
    ).rejects.toMatchObject({ code: "http_error", status: 500 });
  });
});

// contract-http-route-resolution: one policy for an id that cannot be routed.
//
// The three session-scoped modules used to disagree about the same input.
// transcripts.ts and workspace.ts passed a bare id through to a `/api/...` URL
// that no shipping host serves — web-host exposes only `/` and `/host/...`, the
// gateway serves no SPA, and the dev server has no `/api` proxy — while
// capsules.ts rejected it with a message about the input's shape. Both now
// raise the same failure, named for the outcome, before any request goes out.
//
// sessions.ts keeps its default-server fallback: "which server when the id does
// not say" is a fleet configuration question rather than an id-resolution one,
// so it stays out of the resolver (design decision-2).
describe("session route resolution across api modules", () => {
  it("issues no request and reports one failure for an id it cannot route", async () => {
    const fetchFn = vi.fn();

    // The failure must not be observable as Error("network"): doFetch maps any
    // non-Abort throw to that sentinel, so a resolver that threw from inside
    // the request path would recreate the misdiagnosis this change removes.
    await expect(
      makeWorkspaceApi(fetchFn as unknown as typeof fetch).getRootHandle("not-a-fleet-key"),
    ).rejects.toThrow("unresolved session");
    await expect(
      fetchSessionFile("not-a-fleet-key", "transcript", 0, {
        fetchFn: fetchFn as unknown as typeof fetch,
        bearerToken: "tok",
      }),
    ).rejects.toThrow("unresolved session");
    await expect(
      makeCapsulesApi(fetchFn as unknown as typeof fetch).flag("not-a-fleet-key", true),
    ).rejects.toThrow("unresolved session");

    expect(fetchFn).not.toHaveBeenCalled();
  });

  it("reports the same failure type from every converged module", async () => {
    const fetchFn = vi.fn();
    const failures = await Promise.all(
      [
        makeWorkspaceApi(fetchFn as unknown as typeof fetch).getRootHandle("bare"),
        fetchSessionFile("bare", "transcript", 0, {
          fetchFn: fetchFn as unknown as typeof fetch,
          bearerToken: "tok",
        }),
        makeCapsulesApi(fetchFn as unknown as typeof fetch).flag("bare", true),
      ].map((p) => p.then(() => null).catch((e: unknown) => e)),
    );

    for (const failure of failures) {
      expect(failure).toBeInstanceOf(UnresolvedSessionError);
    }
  });

  it("still routes a bare id through the configured default server for session CRUD", async () => {
    // test-setup pins ?server=test-server, so the fallback resolves here. The
    // point of the assertion is the boundary: the fallback belongs to
    // sessions.ts, not to the resolver, and it is the only module that has one.
    const fetchFn = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));

    await makeSessionsApi(fetchFn as unknown as typeof fetch).deleteSession("bare");

    expect(fetchFn).toHaveBeenCalledTimes(1);
    expect(String(fetchFn.mock.calls[0]?.[0])).toContain("/sessions/bare");
  });
});
