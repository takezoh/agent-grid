/**
 * Commands clients send for approval/question resolution.
 */
export interface Commands {
    approval_id?:        string;
    client_instance_id?: string;
    decision?:           Decision;
    session_id:          string;
    /**
     * Free-text answer only; structured objects are rejected at the wire layer.
     */
    answer?:      string;
    question_id?: string;
}

export type Decision = "accept" | "deny";
