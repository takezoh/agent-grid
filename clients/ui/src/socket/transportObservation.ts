export type PublicCorrelation = {
  clientInstanceID: string;
  connectionGeneration: number;
  clientRevision: number;
};

export type TransportObservation =
  | { kind: "observing"; correlation: PublicCorrelation; deadlineMs: number }
  | { kind: "observed_remote"; correlation: PublicCorrelation }
  | { kind: "publication_replaced"; correlation: PublicCorrelation; nextRevision: number }
  | { kind: "delivery_timeout"; correlation: PublicCorrelation }
  | { kind: "socket_closed"; correlation: PublicCorrelation };

export type TransportObservationEvent =
  | { type: "authoritative_terminal"; correlation: PublicCorrelation }
  | { type: "publication_replace"; nextRevision: number }
  | { type: "deadline"; nowMs: number }
  | { type: "socket_close" };

export function observeTransport(
  correlation: PublicCorrelation,
  deadlineMs: number,
): TransportObservation {
  return { kind: "observing", correlation, deadlineMs };
}

/**
 * Reduces the browser-owned transport namespace. Remote terminal authority is
 * matched by the public correlation tuple; accepted/waiting statuses are not
 * events in this namespace and therefore cannot clear the watchdog.
 */
export function reduceTransportObservation(
  current: TransportObservation,
  event: TransportObservationEvent,
): TransportObservation {
  if (current.kind !== "observing") return current;

  switch (event.type) {
    case "authoritative_terminal":
      if (!sameCorrelation(current.correlation, event.correlation)) return current;
      return { kind: "observed_remote", correlation: current.correlation };
    case "publication_replace":
      return {
        kind: "publication_replaced",
        correlation: current.correlation,
        nextRevision: event.nextRevision,
      };
    case "deadline":
      return event.nowMs >= current.deadlineMs
        ? { kind: "delivery_timeout", correlation: current.correlation }
        : current;
    case "socket_close":
      return { kind: "socket_closed", correlation: current.correlation };
  }
}

export function sameCorrelation(a: PublicCorrelation, b: PublicCorrelation): boolean {
  return (
    a.clientInstanceID === b.clientInstanceID &&
    a.connectionGeneration === b.connectionGeneration &&
    a.clientRevision === b.clientRevision
  );
}
