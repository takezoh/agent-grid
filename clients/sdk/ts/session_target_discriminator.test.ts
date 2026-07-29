/**
 * Executable SessionTarget discriminator contract for the generated TS SDK.
 * Invoked via `npm test` under clients/sdk (and the repository flag-day gate).
 */
import assert from "node:assert/strict";
import { after, describe, it } from "node:test";
import { isSessionTarget } from "./generated/repository.ts";

// Stable success sentinel consumed by run-session-target-conformance.sh.
// Printed only after the suite completes; runner also requires exit 0.
const SESSION_TARGET_CONFORMANCE_SENTINEL =
  "AG_SDK_SESSION_TARGET_CONFORMANCE_OK:ts";

after(() => {
  console.log(SESSION_TARGET_CONFORMANCE_SENTINEL);
});

describe("isSessionTarget discriminator", () => {
  it("accepts valid project target", () => {
    assert.equal(
      isSessionTarget({
        source: "project",
        project: { server_id: "srv-a", project: "/repo/demo" },
      }),
      true,
    );
  });

  it("accepts valid repository target", () => {
    assert.equal(
      isSessionTarget({
        source: "repository",
        repository: {
          repository_id: "42",
          owner: "acme",
          name: "widgets",
        },
      }),
      true,
    );
  });

  it("rejects collapsed project+repository payload", () => {
    assert.equal(
      isSessionTarget({
        source: "project",
        project: { server_id: "srv-a", project: "/repo/demo" },
        repository: {
          repository_id: "42",
          owner: "acme",
          name: "widgets",
        },
      }),
      false,
    );
  });

  it("rejects mixed discriminator (source repository with project body)", () => {
    assert.equal(
      isSessionTarget({
        source: "repository",
        project: { server_id: "srv-a", project: "/repo/demo" },
      }),
      false,
    );
  });

  it("rejects mixed discriminator (source project with repository body only)", () => {
    assert.equal(
      isSessionTarget({
        source: "project",
        repository: {
          repository_id: "42",
          owner: "acme",
          name: "widgets",
        },
      }),
      false,
    );
  });

  it("rejects missing source and non-objects", () => {
    assert.equal(isSessionTarget(null), false);
    assert.equal(isSessionTarget(undefined), false);
    assert.equal(isSessionTarget({}), false);
    assert.equal(isSessionTarget({ source: "other" }), false);
  });

  it("rejects empty required identity fields", () => {
    assert.equal(
      isSessionTarget({
        source: "project",
        project: { server_id: "", project: "/repo/demo" },
      }),
      false,
    );
    assert.equal(
      isSessionTarget({
        source: "project",
        project: { server_id: "srv-a", project: "" },
      }),
      false,
    );
    assert.equal(
      isSessionTarget({
        source: "repository",
        repository: { repository_id: "", owner: "acme", name: "widgets" },
      }),
      false,
    );
    assert.equal(
      isSessionTarget({
        source: "repository",
        repository: { repository_id: "42", owner: "", name: "widgets" },
      }),
      false,
    );
    assert.equal(
      isSessionTarget({
        source: "repository",
        repository: { repository_id: "42", owner: "acme", name: "" },
      }),
      false,
    );
  });
});
