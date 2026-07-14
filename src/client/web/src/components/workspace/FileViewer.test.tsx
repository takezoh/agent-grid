import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FileViewer } from "./FileViewer";

describe("FileViewer", () => {
  it("shows metadata placeholder for binary files", () => {
    render(
      <FileViewer
        file={{
          path: "img.png",
          size: 1024,
          is_binary: true,
          content_type: "image/png",
        }}
      />,
    );
    expect(screen.getByTestId("metadata-placeholder")).toBeTruthy();
    expect(screen.getByText("image/png")).toBeTruthy();
  });

  it("verify-vim-mutation-integration: mutation keys do not change viewer HTML", () => {
    const { container } = render(
      <FileViewer
        file={{
          path: "a.txt",
          size: 20,
          is_binary: false,
          content: "line1\nline2\nline3\nline4\nline5\n",
        }}
      />,
    );
    const before = container.innerHTML;
    const viewer = screen.getByTestId("file-viewer");
    fireEvent.keyDown(viewer, { key: "i" });
    fireEvent.keyDown(viewer, { key: "d" });
    fireEvent.keyDown(viewer, { key: "d" });
    fireEvent.keyDown(viewer, { key: ":" });
    fireEvent.keyDown(viewer, { key: "w" });
    expect(container.innerHTML).toBe(before);
  });

  it("renders .env content verbatim without masking", () => {
    const secret = "API_KEY=super-secret-value\nDB_PASS=hunter2";
    render(
      <FileViewer
        file={{
          path: ".env",
          size: secret.length,
          is_binary: false,
          content: secret,
        }}
      />,
    );
    expect(screen.getByText(/super-secret-value/)).toBeTruthy();
    expect(screen.getByText(/hunter2/)).toBeTruthy();
  });

  it("uses virtualized source for large files", () => {
    const big = `${"x".repeat(100)}\n`.repeat(20_000);
    render(
      <FileViewer
        file={{
          path: "big.txt",
          size: big.length,
          is_binary: false,
          content: big,
        }}
      />,
    );
    expect(screen.getByTestId("virtualized-source")).toBeTruthy();
  });
});
