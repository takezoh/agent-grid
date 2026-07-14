import { render } from "@testing-library/react";
import { createElement } from "react";
import ReactMarkdown from "react-markdown";
import { describe, expect, it } from "vitest";
import { containsForbiddenMarkdownTokens, rehypeSanitizePlugin } from "./markdownSanitizer";

const MALICIOUS = `<script>alert(1)</script>
[x](javascript:alert(1))
<img src="http://evil.example/x" onerror="alert(1)">
<p onclick="alert(1)">click</p>`;

describe("markdownSanitizer", () => {
  it("verify-markdown-sanitization-fixture: no forbidden tokens in DOM", () => {
    const { container } = render(
      createElement(ReactMarkdown, { rehypePlugins: [rehypeSanitizePlugin] }, MALICIOUS),
    );
    expect(containsForbiddenMarkdownTokens(container)).toBe(false);
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("[onclick]")).toBeNull();
  });
});
