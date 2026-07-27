export type FleetPresentationState =
  | "connected"
  | "unavailable"
  | "incompatible_protocol"
  | "disabled";

export interface FleetPresentationOutcome {
  state: FleetPresentationState;
  reason?: string;
}

export function fleetOutcomeLabel(outcome: FleetPresentationOutcome): string | null {
  if (outcome.state === "connected" || outcome.state === "disabled") return null;
  const label = outcome.state === "incompatible_protocol" ? "Incompatible protocol" : "Unavailable";
  return outcome.reason ? `${label} — ${outcome.reason}` : label;
}

export function summarizeFleet(outcomes: readonly FleetPresentationOutcome[]): string | null {
  const enabled = outcomes.filter((outcome) => outcome.state !== "disabled");
  const incompatible = enabled.filter((outcome) => outcome.state === "incompatible_protocol");
  const unavailable = enabled.filter((outcome) => outcome.state === "unavailable");
  if (incompatible.length === 0 && unavailable.length === 0) return null;

  const summary = (count: number, state: string): string => {
    if (count === enabled.length) return `all servers ${state}`;
    return `${count}/${enabled.length} servers ${state}`;
  };
  const parts: string[] = [];
  if (incompatible.length > 0) parts.push(summary(incompatible.length, "incompatible"));
  if (unavailable.length > 0) parts.push(summary(unavailable.length, "unavailable"));
  return parts.join("; ");
}
