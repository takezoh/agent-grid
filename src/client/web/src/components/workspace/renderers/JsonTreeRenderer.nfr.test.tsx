import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { JsonTreeRenderer } from "./JsonTreeRenderer";

describe("JsonTreeRenderer NFR-002", () => {
  it("5 MiB JSON reaches first paint within 1500ms", async () => {
    const pad = "x".repeat(5 * 1024 * 1024);
    const source = `{"payload":"${pad}","meta":{"version":1}}`;
    expect(source.length).toBeGreaterThan(5 * 1024 * 1024);

    const start = performance.now();
    render(<JsonTreeRenderer source={source} />);
    await waitFor(
      () => {
        expect(screen.getByTestId("json-tree-renderer")).toBeTruthy();
      },
      { timeout: 1500 },
    );
    const elapsed = performance.now() - start;
    expect(elapsed).toBeLessThanOrEqual(1500);
  });
});
