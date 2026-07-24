/**
 * agent-grid:// URI shapes adopted from plans/remote-control-mobile-session-deep-link.md
 * (FR-P1-09).
 */
export interface DeepLinks {
    /**
     * Session id or ApprovalRequest id.
     */
    id: string;
    /**
     * Path kind: agent-grid://session/<id> or agent-grid://approval/<id>.
     */
    kind:   Kind;
    scheme: "agent-grid";
    /**
     * Full URI form for round-trip helpers.
     */
    uri?: string;
}

/**
 * Path kind: agent-grid://session/<id> or agent-grid://approval/<id>.
 */
export type Kind = "session" | "approval";
