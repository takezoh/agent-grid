import type { AccountProjection } from "./github-host.js";

export type ExternalUrlOpener = (url: string) => Promise<void>;
export type AuthorizationCodeWriter = (code: string) => void;

export function githubAuthorizationUrl(account: AccountProjection): string | null {
  if (account.state !== "authorizing" || !account.verification_uri) return null;
  try {
    const url = new URL(account.verification_uri);
    if (url.protocol !== "https:" || url.hostname !== "github.com") return null;
    if (url.pathname !== "/login/device") return null;
    return url.toString();
  } catch {
    return null;
  }
}

export async function openGitHubAuthorization(
  account: AccountProjection,
  openExternal: ExternalUrlOpener,
): Promise<void> {
  const url = githubAuthorizationUrl(account);
  if (!url) throw new Error("GitHub authorization URL is unavailable");
  await openExternal(url);
}

/** Copies the short-lived user code and waits for the OS browser handoff. */
export async function handoffGitHubAuthorization(
  account: AccountProjection,
  writeCode: AuthorizationCodeWriter,
  openExternal: ExternalUrlOpener,
): Promise<boolean> {
  try {
    if (account.user_code) writeCode(account.user_code);
  } catch {
    // Clipboard availability must not prevent browser authorization.
  }
  try {
    await openGitHubAuthorization(account, openExternal);
    return true;
  } catch {
    return false;
  }
}
