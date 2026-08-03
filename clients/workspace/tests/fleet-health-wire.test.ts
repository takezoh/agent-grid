import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  decodeFleetHealthSnapshotV1,
  encodeFleetHealthSnapshotV1,
} from "../src/main/server-health.js";

const uiFixture = join(
  dirname(fileURLToPath(import.meta.url)),
  "../../ui/src/fleet/fixtures/fleet-health-v1.json",
);
const invalidFixture = join(
  dirname(fileURLToPath(import.meta.url)),
  "../../ui/src/fleet/fixtures/fleet-health-v1-invalid.json",
);

describe("Workspace fleet health wire codec", () => {
  it("deep-equals the complete decoded observation tree and canonical re-encoding", () => {
    const raw = JSON.parse(readFileSync(uiFixture, "utf8"));
    const decoded = decodeFleetHealthSnapshotV1(raw);
    // Full tree equality — not just counts/identity/redaction.
    expect(decoded).toEqual(raw);
    expect(encodeFleetHealthSnapshotV1(decoded)).toEqual(decoded);
    expect(decoded.servers.map((server) => server.observation)).toEqual(
      raw.servers.map((server: { observation: unknown }) => server.observation),
    );
    // reason / prior / state fields preserved for every server.
    for (const server of decoded.servers) {
      const source = raw.servers.find(
        (entry: { serverId: string }) => entry.serverId === server.serverId,
      );
      expect(server.observation).toEqual(source.observation);
    }
    expect(JSON.stringify(decoded)).not.toMatch(/Bearer |ticket=|token=/i);
  });

  it("rejects invalid unions and unknown versions", () => {
    const catalog = JSON.parse(readFileSync(invalidFixture, "utf8")) as {
      cases: Array<{ name: string; payload: unknown }>;
    };
    for (const testCase of catalog.cases) {
      expect(() => decodeFleetHealthSnapshotV1(testCase.payload), testCase.name).toThrow();
    }
  });
});
