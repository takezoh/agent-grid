import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MarkdownRenderer } from "./MarkdownRenderer";

describe("MarkdownRenderer", () => {
  it("renders structured markdown for clean input", () => {
    render(<MarkdownRenderer source={"# Title\n\n- item\n\n```js\nx\n```"} />);
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy();
  });

  it("verify-markdown-sanitization-fixture: forbidden remote image falls back with banner", () => {
    render(<MarkdownRenderer source={"![x](http://evil.example/x.png)\n\n# Title"} />);
    expect(screen.getByText(/sanitization rejected unsafe content/i)).toBeTruthy();
    expect(screen.getByText(/http:\/\/evil\.example/)).toBeTruthy();
    expect(document.querySelector("img[src^='http']")).toBeNull();
  });
});
