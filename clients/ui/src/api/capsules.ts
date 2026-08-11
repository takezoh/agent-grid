// Resume-capsule API client (cognitive workspace Phase 0).
//
// Capsules are listed per execution server (`GET /api/resume-capsules`) and
// merged into one fleet-wide list. The caller supplies the server set —
// normally derived from the sessions already present in the daemon store,
// which is sound because a capsule only exists for a listed session.

import { readBearerTokenFromHash } from "../auth";
import { makeFleetProvider } from "../fleet/provider";
import { fleetSessionIdFor, resolveSessionRoute } from "../fleet/session-identity";
import { UnresolvedSessionError } from "./sessions";

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
  /** Completion report (plan §18): changed / verified / remaining / attention. */
  changed?: string[];
  verified?: string[];
  remaining?: string[];
  attention?: string;
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
                fleetSessionKey: fleetSessionIdFor({ serverId, sessionId: capsule.session_id }),
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
      const route = resolveSessionRoute(fleetSessionKey);
      // Was "invalid fleet session key" — a description of the input's shape,
      // which invited callers to branch on the form. The shared failure names
      // the outcome instead.
      if (!route.ok) throw new UnresolvedSessionError(fleetSessionKey);
      const resp = await fleet.request(
        route.serverId,
        `/api/sessions/${encodeURIComponent(route.upstreamSessionId)}/resume-capsule/flag`,
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
