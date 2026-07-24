/**
 * Capability negotiation declaration for bundled vs remote clients (FR-P1-03, FR-P1-04).
 */
export interface Capabilities {
    /**
     * Two-axis compatibility policy skeleton.
     */
    axis?: Axis;
    /**
     * Feature flags the peer supports (e.g. approval.respond).
     */
    capabilities: string[];
    /**
     * Daemon/client contract version. Bundled clients match this string and skip per-capability
     * negotiation.
     */
    protocolVersion: string;
}

/**
 * Two-axis compatibility policy skeleton.
 */
export type Axis = "bundled" | "remote";
