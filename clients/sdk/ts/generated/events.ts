/**
 * WS/event frames the daemon pushes to clients. Discriminated by k on the lifecycle surface.
 */
export interface Events {
    approval?: Approval;
    k:         K;
    question?: Question;
}

export interface Approval {
    command?:                      string;
    created_at?:                   Date;
    decision?:                     Decision;
    default_decision?:             Decision;
    expires_at?:                   Date;
    frame_id?:                     string;
    id:                            string;
    kind?:                         Kind;
    path?:                         string;
    reason?:                       string;
    resolution_reason?:            ApprovalResolutionReason;
    resolving_client_instance_id?: string;
    session_id:                    string;
    status:                        Status;
}

export type Decision = "accept" | "deny";

export type Kind = "command" | "file_change";

export type ApprovalResolutionReason = "client" | "expired" | "cancelled" | "auto";

export type Status = "pending" | "resolved" | "expired" | "cancelled";

export type K = "ar" | "ax" | "qr" | "qx";

export interface Question {
    /**
     * Free-text only (HumanInputRequest.free_text).
     */
    answer?:                       string;
    created_at?:                   Date;
    expires_at?:                   Date;
    frame_id?:                     string;
    id:                            string;
    prompt?:                       string;
    resolution_reason?:            QuestionResolutionReason;
    resolving_client_instance_id?: string;
    session_id:                    string;
    status:                        Status;
}

export type QuestionResolutionReason = "client" | "expired" | "cancelled";
