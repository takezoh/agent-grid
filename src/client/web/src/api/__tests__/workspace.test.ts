import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WorkspaceApiError, makeWorkspaceApi } from "../workspace";

function jsonResponse(value: unknown, status: number): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

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
      "s1",
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
      api.save("s1", "x.txt", { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" }, "x"),
    ).rejects.toSatisfy((err: unknown) => WorkspaceApiError.isHandleStale(err));
  });

  it("maps invalid_handle (400)", async () => {
    const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: "invalid_handle" }, 400));
    const api = makeWorkspaceApi(fetchFn);
    await expect(
      api.getFile("s1", "x.txt", {
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
        "s1",
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
      api.save("s1", "x.txt", { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" }, "x"),
    ).rejects.toSatisfy((err: unknown) => WorkspaceApiError.isOversizeBody(err));
  });

  it("maps audit_emit_failed (500)", async () => {
    const fetchFn = vi.fn().mockResolvedValue(jsonResponse({ error: "audit_emit_failed" }, 500));
    const api = makeWorkspaceApi(fetchFn);
    await expect(
      api.save("s1", "x.txt", { sessionId: "s1", frameGeneration: 1, resolvedRootPath: "/w" }, "x"),
    ).rejects.toSatisfy((err: unknown) => WorkspaceApiError.isAuditEmitFailed(err));
  });
});
