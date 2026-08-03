import { describe, expect, it } from "vitest";
import { parseRepositoryCreatePayload } from "../src/main/repository-create.js";

describe("repository create payload", () => {
  it("accepts only the versioned command-profile contract", () => {
    expect(() =>
      parseRepositoryCreatePayload({ repository_id: "42", owner: "octo", name: "repo" }),
    ).toThrow(/versioned command profile/);
    expect(
      parseRepositoryCreatePayload({
        repository_id: "42",
        owner: "octo",
        name: "repo",
        version: 2,
        profile_id: "codex",
        profile_revision: "revision-codex",
        parameters: { model: "gpt-5.6" },
      }),
    ).toMatchObject({
      version: 2,
      profile_id: "codex",
      profile_revision: "revision-codex",
    });
  });
});
