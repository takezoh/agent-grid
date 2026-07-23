import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { JsonTreeRenderer } from "./JsonTreeRenderer";

describe("JsonTreeRenderer", () => {
  it("renders collapsible tree for valid JSON", async () => {
    render(<JsonTreeRenderer source={'{"a":1,"b":[2]}'} />);
    await waitFor(() => expect(screen.getByTestId("json-tree-renderer")).toBeTruthy());
    expect(screen.getByText(/a/)).toBeTruthy();
  });

  it("fallback for invalid JSON", async () => {
    render(<JsonTreeRenderer source="{not-json}" />);
    await waitFor(() => expect(screen.getByText(/Invalid JSON/i)).toBeTruthy());
  });
});
