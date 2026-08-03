import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ExecutionServerSelector, defaultExecutionServerId } from "./ExecutionServerSelector";

const servers = [
  {
    serverId: "srv-a",
    serverLabel: "Alpha",
    isDefaultExecution: false,
    state: "connected" as const,
  },
  {
    serverId: "srv-b",
    serverLabel: "Beta",
    isDefaultExecution: true,
    state: "connected" as const,
  },
  {
    serverId: "srv-c",
    serverLabel: "Offline",
    isDefaultExecution: false,
    state: "unavailable" as const,
  },
];

describe("ExecutionServerSelector", () => {
  it("chooses the configured default before the project server", () => {
    expect(defaultExecutionServerId(servers, null, "srv-a")).toBe("srv-b");
    expect(defaultExecutionServerId(servers, "srv-a", "srv-b")).toBe("srv-a");
  });

  it("keeps the server list hidden until Change and selects a connected server", () => {
    const onChange = vi.fn();
    render(
      <ExecutionServerSelector
        servers={servers}
        value="srv-b"
        disabled={false}
        onChange={onChange}
      />,
    );

    expect(screen.getByTestId("execution-server-current").textContent).toBe("Beta");
    expect(screen.queryByRole("button", { name: /Alpha/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Offline/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    expect((screen.getByRole("button", { name: /Alpha/ }) as HTMLButtonElement).disabled).toBe(
      false,
    );
    expect((screen.getByRole("button", { name: /Offline/ }) as HTMLButtonElement).disabled).toBe(
      true,
    );
    fireEvent.click(screen.getByRole("button", { name: /Alpha/ }));
    expect(onChange).toHaveBeenCalledWith("srv-a");
  });

  it("keeps the configured default visible when its connection is unavailable", () => {
    const serversWithUnavailableDefault = servers.map((server) =>
      server.serverId === "srv-b" ? { ...server, state: "unavailable" as const } : server,
    );
    render(
      <ExecutionServerSelector
        servers={serversWithUnavailableDefault}
        value="srv-b"
        disabled={false}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByTestId("execution-server-current").textContent).toBe("Beta");
    expect(screen.queryByRole("button", { name: /Alpha/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Beta/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Offline/ })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    expect(screen.getByRole("button", { name: /Alpha/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Beta/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Offline/ })).toBeTruthy();
  });

  it.each([
    ["unavailable", "connection refused"],
    ["incompatible_protocol", "client protocol-3, server protocol-1"],
  ] as const)(
    "shows a configured %s default without exposing health details in the palette",
    (state, reason) => {
      render(
        <ExecutionServerSelector
          servers={[
            {
              serverId: "srv-default",
              serverLabel: "Configured Default",
              isDefaultExecution: true,
              state,
              reason,
            },
            {
              serverId: "srv-peer",
              serverLabel: "Healthy Peer",
              state: "connected",
            },
          ]}
          value="srv-default"
          disabled={false}
          onChange={vi.fn()}
        />,
      );

      const selector = screen.getByTestId("execution-server-selector");
      expect(screen.getByTestId("execution-server-current").textContent).toBe("Configured Default");
      expect(screen.getByRole("button", { name: "Change" })).toBeTruthy();
      expect(screen.queryByRole("button", { name: "Healthy Peer" })).toBeNull();
      expect(selector.textContent).not.toContain(reason);
      expect(selector.textContent).not.toMatch(/unavailable|incompatible|error/i);
    },
  );

  it("lists every configured server after Change but disables unavailable outcomes", () => {
    const onChange = vi.fn();
    render(
      <ExecutionServerSelector
        servers={[
          {
            serverId: "srv-connected",
            serverLabel: "Connected",
            state: "connected",
          },
          {
            serverId: "srv-unavailable",
            serverLabel: "Configured Offline",
            state: "unavailable",
            reason: "connection refused",
          },
          {
            serverId: "srv-incompatible",
            serverLabel: "Configured Old",
            state: "incompatible_protocol",
            reason: "protocol mismatch",
          },
        ]}
        value="srv-connected"
        disabled={false}
        onChange={onChange}
      />,
    );

    expect(screen.queryByRole("button", { name: "Configured Offline" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Configured Old" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Change" }));
    expect(
      (screen.getByRole("button", { name: "Configured Offline" }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(
      (screen.getByRole("button", { name: "Configured Old" }) as HTMLButtonElement).disabled,
    ).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Connected" }));
    expect(onChange).toHaveBeenCalledWith("srv-connected");
  });
});
