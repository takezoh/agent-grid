export interface SessionStatus {
    changed_at?: Date;
    failure:     Failure;
    human_input: boolean;
    lifecycle:   Lifecycle;
    signal?:     Signal;
}

export type Failure = "none" | "failed";

export type Lifecycle = "active" | "idle" | "stopped";

export interface Signal {
    epoch:    string;
    kind:     "bell";
    sequence: number;
}
