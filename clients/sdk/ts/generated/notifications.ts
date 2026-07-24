/**
 * Notification payload skeleton for Phase 0/1 (policy details in
 * contracts/notification-policy.md).
 */
export interface Notifications {
    body?:      string;
    deep_link?: string;
    kind:       Kind;
    session_id: string;
    title?:     string;
}

export type Kind = "approval_pending" | "question_pending" | "agent_notification";
