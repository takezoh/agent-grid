/**
 * TypeScript SDK entry.
 * - generated/* : quicktype models from protocol/*.schema.json (do not edit)
 * - src/*       : hand-written transport + deep-link helpers
 */

export type { Events, Approval, Question, Decision, Status, K } from "../generated/events.js";
export type { Commands } from "../generated/commands.js";
export type { Capabilities } from "../generated/capabilities.js";
export type { DeepLinks } from "../generated/deep-links.js";
export type { Notifications } from "../generated/notifications.js";

export {
  parseDeepLink,
  constructDeepLink,
  type DeepLink,
  type DeepLinkKind,
} from "./deepLinks.js";

export {
  GatewayTransport,
  type TransportConfig,
  type WsTicket,
} from "./transport.js";
