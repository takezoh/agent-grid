import { useState } from "react";
import type { ServerConfigOutcome } from "../../api/sessions";

export interface ExecutionServerSelectorProps {
  servers: readonly ServerConfigOutcome[];
  value: string;
  disabled: boolean;
  onChange: (serverId: string) => void;
}

export function defaultExecutionServerId(
  servers: readonly ServerConfigOutcome[],
  current?: string | null,
  projectServerId?: string | null,
): string | null {
  if (current && servers.some((server) => server.serverId === current)) return current;
  return (
    servers.find((server) => server.isDefaultExecution)?.serverId ??
    servers.find((server) => server.serverId === projectServerId)?.serverId ??
    servers[0]?.serverId ??
    null
  );
}

export function ExecutionServerSelector(props: ExecutionServerSelectorProps): JSX.Element | null {
  const { servers, value, disabled, onChange } = props;
  const [changing, setChanging] = useState(false);
  const selected = servers.find((server) => server.serverId === value);
  if (!selected) return null;

  return (
    <section className="palette-execution-server" data-testid="execution-server-selector">
      <div className="palette-execution-server__summary">
        <span>Execution Server</span>
        <strong data-testid="execution-server-current" data-server-state={selected.state}>
          {selected.serverLabel}
        </strong>
        <button
          type="button"
          disabled={disabled || servers.length < 2}
          aria-pressed={changing}
          aria-controls="execution-server-options"
          onClick={() => setChanging((value) => !value)}
        >
          Change
        </button>
      </div>
      {changing ? (
        <ul
          id="execution-server-options"
          className="palette-execution-server__options is-changing"
          aria-label="Execution Servers"
        >
          {servers.map((server) => (
            <li key={server.serverId}>
              <button
                type="button"
                aria-current={server.serverId === value ? "true" : undefined}
                data-server-state={server.state}
                disabled={disabled || server.state !== "connected"}
                onClick={() => {
                  onChange(server.serverId);
                  setChanging(false);
                }}
              >
                {server.serverLabel}
                {server.isDefaultExecution ? <span>Default</span> : null}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
