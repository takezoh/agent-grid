import * as net from "node:net";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { ControlEndpoint } from "../src/main/control-endpoint.js";
import { WindowRegistry, type WindowFactory, type WindowHandle } from "../src/main/window-registry.js";

function memFactory(): WindowFactory {
  return {
    create(sessionId: string): WindowHandle {
      let destroyed = false;
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
          return { x: 0, y: 0, width: 1, height: 1 };
        },
        setBounds() {},
      };
    },
  };
}

function sockPath(): string {
  return path.join(os.tmpdir(), `ag-ws-ctrl-${process.pid}-${Math.random().toString(16).slice(2)}.sock`);
}

async function sendLine(sockPath: string, line: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const sock = net.createConnection(sockPath, () => {
      sock.write(`${line}\n`);
    });
    let buf = "";
    sock.setEncoding("utf8");
    sock.on("data", (c) => {
      buf += c;
      if (buf.includes("\n")) {
        sock.end();
        resolve(buf.trim());
      }
    });
    sock.on("error", reject);
  });
}

describe("control-endpoint", () => {
  let endpoint: ControlEndpoint | null = null;

  afterEach(async () => {
    await endpoint?.stop();
    endpoint = null;
  });

  it("openSession creates exactly one window; unknown field rejected without killing conn", async () => {
    const reg = new WindowRegistry(memFactory());
    const p = sockPath();
    endpoint = new ControlEndpoint({ path: p, registry: reg });
    await endpoint.start();

    const bad = await sendLine(p, '{"op":"openSession","id":"s1","extra":true}');
    expect(JSON.parse(bad).ok).toBe(false);
    expect(JSON.parse(bad).error).toMatch(/unknown field/);
    expect(reg.openCount).toBe(0);

    // Connection path still works for next valid line (new connection each sendLine).
    const ok1 = await sendLine(p, '{"op":"openSession","id":"s1"}');
    expect(JSON.parse(ok1).ok).toBe(true);
    const ok2 = await sendLine(p, '{"op":"openSession","id":"s1"}');
    expect(JSON.parse(ok2).ok).toBe(true);
    expect(reg.openCount).toBe(1);
  });
});
