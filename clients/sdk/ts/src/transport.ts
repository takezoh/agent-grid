/**
 * Hand-written thin REST transport for the TS SDK.
 * Routes come from protocol/openapi.yaml (REST-binding annex only).
 * Models come from ../generated (quicktype).
 */

import type { Capabilities } from "../generated/capabilities.js";

export interface TransportConfig {
  /** Gateway base URL, e.g. http://127.0.0.1:8787 */
  baseUrl: string;
  /** Bearer token; omitted in no-auth local mode. */
  token?: string;
  /** Ephemeral client-instance-id from /api/ws-ticket (FR-P0-12). */
  clientInstanceId?: string;
  fetchImpl?: typeof fetch;
}

export interface WsTicket {
  ticket: string;
  client_instance_id: string;
}

export class GatewayTransport {
  private readonly baseUrl: string;
  private readonly token?: string;
  private clientInstanceId?: string;
  private readonly fetchImpl: typeof fetch;

  constructor(cfg: TransportConfig) {
    this.baseUrl = cfg.baseUrl.replace(/\/$/, "");
    this.token = cfg.token;
    this.clientInstanceId = cfg.clientInstanceId;
    this.fetchImpl = cfg.fetchImpl ?? fetch;
  }

  getClientInstanceId(): string | undefined {
    return this.clientInstanceId;
  }

  async mintWsTicket(): Promise<WsTicket> {
    const body = await this.request<WsTicket>("POST", "/api/ws-ticket");
    this.clientInstanceId = body.client_instance_id;
    return body;
  }

  async getCapabilities(): Promise<Capabilities> {
    return this.request<Capabilities>("GET", "/api/capabilities");
  }

  async respondApproval(
    sessionId: string,
    approvalId: string,
    decision: "accept" | "deny",
  ): Promise<void> {
    await this.request("POST", `/api/sessions/${encodeURIComponent(sessionId)}/approvals/${encodeURIComponent(approvalId)}`, {
      decision,
      client_instance_id: this.clientInstanceId,
    });
  }

  async respondQuestion(
    sessionId: string,
    questionId: string,
    answer: string,
  ): Promise<void> {
    await this.request("POST", `/api/sessions/${encodeURIComponent(sessionId)}/questions/${encodeURIComponent(questionId)}`, {
      answer,
      client_instance_id: this.clientInstanceId,
    });
  }

  private async request<T>(
    method: string,
    path: string,
    jsonBody?: unknown,
  ): Promise<T> {
    const headers: Record<string, string> = {
      Accept: "application/json",
    };
    if (this.token) {
      headers.Authorization = `Bearer ${this.token}`;
    }
    if (this.clientInstanceId) {
      headers["X-Client-Instance-ID"] = this.clientInstanceId;
    }
    let body: string | undefined;
    if (jsonBody !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(jsonBody);
    }
    const res = await this.fetchImpl(`${this.baseUrl}${path}`, {
      method,
      headers,
      body,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`gateway ${method} ${path}: ${res.status} ${text}`);
    }
    if (res.status === 204) {
      return undefined as T;
    }
    return (await res.json()) as T;
  }
}
