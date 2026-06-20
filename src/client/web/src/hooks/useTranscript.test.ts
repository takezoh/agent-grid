import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTranscriptStore } from "../store/transcripts";
import { useTranscript } from "./useTranscript";

function makeFetch(status: number, body: string): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(body, {
      status,
      headers: { "Content-Type": "text/plain" },
    }),
  ) as unknown as typeof fetch;
}

describe("useTranscript", () => {
  beforeEach(() => {
    useTranscriptStore.getState().reset();
  });

  it("TestBackfillsOnMount: fetch 200 body 'a\\nb\\n' → buffer ['a','b']", async () => {
    const fetchFn = makeFetch(200, "a\nb\n");

    const { unmount } = renderHook(() =>
      useTranscript({
        sessionId: "s1",
        kind: "transcript",
        bearerToken: "tok",
        fetchFn,
      }),
    );

    // Wait for the async effect to settle
    await vi.waitFor(() => {
      const buf = useTranscriptStore.getState().buffers["s1:transcript"];
      expect(buf?.lines).toEqual(["a", "b"]);
    });

    unmount();
  });

  it("TestEmptySkipsAppend: fetch 204 → buffer remains empty", async () => {
    const fetchFn = makeFetch(204, "");

    const { unmount } = renderHook(() =>
      useTranscript({
        sessionId: "s2",
        kind: "event-log",
        bearerToken: "tok",
        fetchFn,
      }),
    );

    await vi.waitFor(() => {
      expect(fetchFn).toHaveBeenCalledOnce();
    });

    expect(useTranscriptStore.getState().buffers["s2:event-log"]).toBeUndefined();

    unmount();
  });

  it("TestCancellationOnUnmount: unmount before resolve → store not written", async () => {
    let resolveResponse!: (r: Response) => void;
    const pendingFetch = vi.fn().mockReturnValue(
      new Promise<Response>((res) => {
        resolveResponse = res;
      }),
    ) as unknown as typeof fetch;

    const { unmount } = renderHook(() =>
      useTranscript({
        sessionId: "s3",
        kind: "transcript",
        bearerToken: "tok",
        fetchFn: pendingFetch,
      }),
    );

    // Unmount before fetch resolves
    unmount();

    // Now resolve the fetch — cancelled flag should prevent store write
    resolveResponse(new Response("line1\n", { status: 200 }));

    // Give any microtasks time to run
    await new Promise((r) => setTimeout(r, 10));

    expect(useTranscriptStore.getState().buffers["s3:transcript"]).toBeUndefined();
  });
});
