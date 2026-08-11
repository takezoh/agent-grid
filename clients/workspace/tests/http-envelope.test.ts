import { describe, expect, it } from "vitest";
import {
  buildResponseEnvelope,
  isNullBodyStatus,
  NULL_BODY_STATUSES,
} from "../src/shared/http-envelope.js";

describe("null-body status set", () => {
  // The constant is subordinate to the platform. If a runtime ever disagrees
  // with this list — accepting a body where we forbid one, or rejecting one
  // where we allow it — this fails and forces the list to be updated instead of
  // letting the envelope and the Response constructor drift apart silently.
  it("matches what this runtime's Response constructor enforces", () => {
    for (const status of NULL_BODY_STATUSES) {
      expect(() => new Response("", { status }), `status ${status} should reject a body`).toThrow();
    }
    for (const status of [200, 201, 400, 500]) {
      expect(
        () => new Response("", { status }),
        `status ${status} should accept a body`,
      ).not.toThrow();
    }
  });

  it("builds a bodyless Response for every null-body status a fetch can carry", () => {
    // 101 is in the set because the Fetch Standard says so, but `Response`
    // refuses to represent it at all (its status range starts at 200), so a
    // null body cannot be demonstrated for it. Restricting this assertion to
    // the representable members keeps it about the invariant rather than about
    // the constructor's range check.
    for (const status of [...NULL_BODY_STATUSES].filter((s) => s >= 200)) {
      expect(new Response(null, { status }).status).toBe(status);
    }
  });

  it("covers the statuses the fleet routes actually return", () => {
    // 204: DELETE /api/sessions/{id}, POST .../messages/read
    // 304: GET /api/sessions/{id}/{kind} when not modified
    expect(isNullBodyStatus(204)).toBe(true);
    expect(isNullBodyStatus(304)).toBe(true);
    expect(isNullBodyStatus(200)).toBe(false);
  });
});

describe("buildResponseEnvelope", () => {
  it("reports no body for a null-body status", async () => {
    const envelope = await buildResponseEnvelope(new Response(null, { status: 204 }));

    expect(envelope.body).toBeNull();
    expect(envelope.status).toBe(204);
  });

  it("keeps an empty 200 body as an empty string, not as absent", async () => {
    // "" and null are different facts: one is a body that happens to be empty,
    // the other is the absence of a body. Collapsing them is the defect.
    const envelope = await buildResponseEnvelope(new Response("", { status: 200 }));

    expect(envelope.body).toBe("");
  });

  it("drops a body that an upstream wrongly attached to a 204", async () => {
    // The status is the authority, not whether bytes arrived. Forwarding these
    // bytes would produce an envelope the renderer cannot rebuild.
    const misbehaving = {
      status: 204,
      headers: new Headers(),
      text: async () => "unexpected",
    } as unknown as Response;

    const envelope = await buildResponseEnvelope(misbehaving);

    expect(envelope.body).toBeNull();
  });

  it("preserves headers and body for an ordinary response", async () => {
    const envelope = await buildResponseEnvelope(
      new Response('{"ok":true}', {
        status: 200,
        headers: { "content-type": "application/json", etag: "e1" },
      }),
    );

    expect(envelope.body).toBe('{"ok":true}');
    expect(envelope.headers["content-type"]).toBe("application/json");
    expect(envelope.headers.etag).toBe("e1");
  });

  it("preserves the etag on a 304 so transcript polling can still resume", async () => {
    const envelope = await buildResponseEnvelope(
      new Response(null, { status: 304, headers: { etag: "e2" } }),
    );

    expect(envelope.body).toBeNull();
    expect(envelope.headers.etag).toBe("e2");
  });
});
