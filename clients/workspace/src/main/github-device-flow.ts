import type { GitHubAction, GitHubActionResult } from "./repository-lease.js";

type GitHubDispatch = (action: GitHubAction) => Promise<GitHubActionResult>;
type TimerHandle = ReturnType<typeof setTimeout>;

export interface GitHubDeviceFlowTimers {
  setTimeout(callback: () => void, delayMs: number): TimerHandle;
  clearTimeout(handle: TimerHandle): void;
}

const defaultTimers: GitHubDeviceFlowTimers = {
  setTimeout: (callback, delayMs) => setTimeout(callback, delayMs),
  clearTimeout: (handle) => clearTimeout(handle),
};

/**
 * Owns Device Flow progress in Electron's main process. Renderer visibility,
 * Settings window lifetime, and browser focus must not control authentication.
 */
export class GitHubDeviceFlowController {
  private timer: TimerHandle | null = null;
  private generation = 0;

  constructor(
    private readonly dispatch: GitHubDispatch,
    private readonly timers: GitHubDeviceFlowTimers = defaultTimers,
    private readonly tickMs = 1_000,
  ) {}

  async action(action: GitHubAction): Promise<GitHubActionResult> {
    if (action === "start") {
      this.stop();
      const result = await this.dispatch(action);
      if (result.account.state === "authorizing") this.schedule(this.generation);
      return result;
    }
    if (action === "cancel") this.stop();
    return this.dispatch(action);
  }

  stop(): void {
    this.generation += 1;
    if (this.timer !== null) this.timers.clearTimeout(this.timer);
    this.timer = null;
  }

  private schedule(generation: number): void {
    this.timer = this.timers.setTimeout(() => {
      this.timer = null;
      void this.poll(generation);
    }, this.tickMs);
  }

  private async poll(generation: number): Promise<void> {
    if (generation !== this.generation) return;
    try {
      const result = await this.dispatch("poll");
      if (generation === this.generation && result.account.state === "authorizing") {
        this.schedule(generation);
      }
    } catch {
      // A transport failure is projected by Account on the next status read.
      // Avoid an unhandled rejection and end this flow generation.
      this.stop();
    }
  }
}
