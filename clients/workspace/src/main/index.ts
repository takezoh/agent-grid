/**
 * Electron main entry (Windows production). On Linux CI the pure modules are
 * exercised by vitest without launching Electron.
 *
 * Single-instance lock; control-endpoint + window-registry wiring.
 */

import { ControlEndpoint, defaultControlPath } from "./control-endpoint.js";
import { DaemonConfigResolver } from "./daemon-config.js";
import { FileStateStore, defaultWorkspaceStatePath } from "./file-state-store.js";
import {
  WindowRegistry,
  type StateStore,
  type WindowFactory,
  type WindowHandle,
} from "./window-registry.js";

export interface MainBootstrapOptions {
  tokenPath: string;
  baseUrl: string;
  webOrigin: string;
  controlPath?: string;
  factory: WindowFactory;
  /** Override layout persistence (default: FileStateStore under APPDATA). */
  stateStore?: StateStore | null;
  statePath?: string;
}

export async function bootstrapMain(opts: MainBootstrapOptions): Promise<{
  registry: WindowRegistry;
  endpoint: ControlEndpoint;
  config: DaemonConfigResolver;
  stop: () => Promise<void>;
}> {
  const config = new DaemonConfigResolver({
    tokenPath: opts.tokenPath,
    baseUrl: opts.baseUrl,
    webOrigin: opts.webOrigin,
  });
  const store: StateStore | null =
    opts.stateStore === undefined
      ? new FileStateStore(opts.statePath ?? defaultWorkspaceStatePath())
      : opts.stateStore;
  const registry = new WindowRegistry(opts.factory, store);
  const endpoint = new ControlEndpoint({
    path: opts.controlPath ?? defaultControlPath(),
    registry,
  });
  await endpoint.start();
  return {
    registry,
    endpoint,
    config,
    stop: async () => {
      await endpoint.stop();
    },
  };
}

/** Test/fake window handle. */
export function createMemoryFactory(): {
  factory: WindowFactory;
  created: string[];
} {
  const created: string[] = [];
  const factory: WindowFactory = {
    create(sessionId: string): WindowHandle {
      created.push(sessionId);
      let destroyed = false;
      let bounds = { x: 0, y: 0, width: 1200, height: 800 };
      return {
        id: sessionId,
        focus() {},
        close() {
          destroyed = true;
        },
        isDestroyed() {
          return destroyed;
        },
        getBounds() {
          return { ...bounds };
        },
        setBounds(b) {
          bounds = { ...b };
        },
      };
    },
  };
  return { factory, created };
}
