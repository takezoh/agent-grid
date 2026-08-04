// Resume-capsule API client (cognitive workspace Phase 0).
//
// Capsules are listed per execution server (`GET /api/resume-capsules`) and
// merged into one fleet-wide list. The caller supplies the server set —
// normally derived from the sessions already present in the daemon store,
// which is sound because a capsule only exists for a listed session.

import { readBearerTokenFromHash } from "../auth";
import { makeFleetProvider } from "../fleet/provider";
import { parseSessionKey, sessionKey } from "../fleet/session-protocol";

/** Wire shape from the Go gateway (snake_case mirrors proto.ResumeCapsuleInfo). */
export type ResumeCapsuleInfo = {
  session_id: string;
  project: string;
  workspace?: string;
  title?: string;
  objective?: string;
  current_state?: string;
  next_action?: string;
  unresolved?: string[];
  generated_at: string;
  source_turns?: number;
  flagged_wrong?: boolean;
  last_error?: string;
};

type ListResponse = {
  resume_capsules?: ResumeCapsuleInfo[];
};

/** Capsule enriched with fleet identity for cross-server rendering. */
export type FleetResumeCapsule = ResumeCapsuleInfo & {
  serverId: string;
  serverLabel?: string;
  /** Compound fleet session key — joins with daemon-store session ids. */
  fleetSessionKey: string;
};

export type CapsuleServerRef = {
  serverId: string;
  serverLabel?: string;
};

export type CapsuleListResult = {
  capsules: FleetResumeCapsule[];
  errors: { serverId: string; message: string }[];
};

export interface CapsulesApi {
  listAll(servers: CapsuleServerRef[]): Promise<CapsuleListResult>;
  flag(fleetSessionKey: string, flagged: boolean): Promise<void>;
}

function authHeaders(): Record<string, string> {
  const token = readBearerTokenFromHash();
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
}

export function makeCapsulesApi(fetchFn: typeof fetch = fetch): CapsulesApi {
  const fleet = makeFleetProvider(fetchFn);

  return {
    async listAll(servers) {
      const capsules: FleetResumeCapsule[] = [];
      const errors: { serverId: string; message: string }[] = [];
      await Promise.all(
        servers.map(async ({ serverId, serverLabel }) => {
          try {
            const resp = await fleet.request(serverId, "/api/resume-capsules", {
              headers: authHeaders(),
            });
            if (!resp.ok) {
              throw new Error(`HTTP ${resp.status}`);
            }
            const body = (await resp.json()) as ListResponse;
            for (const capsule of body.resume_capsules ?? []) {
              capsules.push({
                ...capsule,
                serverId,
                serverLabel,
                fleetSessionKey: sessionKey({ serverId, sessionId: capsule.session_id }),
              });
            }
          } catch (err: unknown) {
            errors.push({
              serverId,
              message: err instanceof Error ? err.message : String(err),
            });
          }
        }),
      );
      capsules.sort((a, b) => (a.generated_at < b.generated_at ? 1 : -1));
      return { capsules, errors };
    },

    async flag(fleetSessionKey, flagged) {
      const ref = parseSessionKey(fleetSessionKey);
      if (!ref) {
        throw new Error("invalid fleet session key");
      }
      const resp = await fleet.request(
        ref.serverId,
        `/api/sessions/${encodeURIComponent(ref.sessionId)}/resume-capsule/flag`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json", ...authHeaders() },
          body: JSON.stringify({ flagged }),
        },
      );
      if (!resp.ok) {
        throw new Error(`HTTP ${resp.status}`);
      }
    },
  };
}
