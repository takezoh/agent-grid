import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { loadOrCreateDesktopConfig } from "../src/main/desktop-config.js";

// Contract: contract-client-config-parity, contract-config-entry-admission,
// contract-server-naming-prerequisite
// (adr-20260814-shared-go-client-config-module,
// adr-20260814-server-display-name-naming-contract).
//
// This is the TypeScript reader of the shared expected-outcome corpus. The Go
// reader is clients/goclient/clientconfig/corpus_test.go and the C# reader is
// AgentGrid.Shell.Core.Tests/Configuration/ConfigCorpusTests.cs. All three read
// the same file and assert the same record, which is what makes "the three
// loaders agree" a mechanical property instead of a claim in a README.
//
// It replaces an assertion that each shared fixture merely threw — three
// loaders that each throw can still reject the same document for different
// reasons and accept it with different results.

type CorpusServer = {
  index: number;
  display_name: string;
  admission: "usable" | "disabled" | "configuration_invalid";
  enabled?: boolean;
  default_execution?: boolean;
  canonical_url?: string;
  server_id?: string;
  rule?: string;
};

type CorpusCase = {
  name: string;
  rules: string[];
  why?: string;
  document: string;
  expected: { decision: "load" | "stop"; servers?: CorpusServer[] };
};

const corpus = JSON.parse(
  fs.readFileSync(path.resolve("../config/fixtures/servers-corpus.json"), "utf8"),
) as { cases: CorpusCase[] };

const directories: string[] = [];

function seed(document: string): string {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "agent-grid-corpus-"));
  directories.push(directory);
  // The desktop loader reads four documents and creates the ones that are
  // absent; seeding servers.json after that leaves only the document under
  // test differing from the defaults.
  loadOrCreateDesktopConfig(directory);
  fs.writeFileSync(path.join(directory, "servers.json"), document);
  return directory;
}

afterEach(() => {
  for (const directory of directories.splice(0)) {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});

describe("shared client configuration corpus", () => {
  it("carries cases", () => {
    expect(corpus.cases.length).toBeGreaterThan(0);
  });

  it.each(corpus.cases.map((testCase) => [testCase.name, testCase] as const))(
    "reaches the recorded outcome for %s",
    (_name, testCase) => {
      const directory = seed(testCase.document);

      if (testCase.expected.decision === "stop") {
        expect(
          () => loadOrCreateDesktopConfig(directory),
          `corpus records a file-level stop for rules ${testCase.rules.join(", ")}`,
        ).toThrow();
        return;
      }

      const config = loadOrCreateDesktopConfig(directory);
      const expected = testCase.expected.servers ?? [];
      expect(config.servers).toHaveLength(expected.length);
      config.servers.forEach((server, position) => {
        const want = expected[position]!;
        expect(server.index).toBe(want.index);
        expect(server.display_name).toBe(want.display_name);
        expect(server.admission).toBe(want.admission);
        if (want.admission === "configuration_invalid") {
          expect(server.violation?.rule).toBe(want.rule);
          return;
        }
        expect(server.enabled).toBe(want.enabled);
        expect(server.default_execution).toBe(want.default_execution);
        expect(server.url).toBe(want.canonical_url);
        expect(server.id).toBe(want.server_id);
      });
    },
  );
});
